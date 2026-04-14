package wechat

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/im-bridge/core"
)

func TestNewLive_Validation(t *testing.T) {
	if _, err := NewLive("", "secret", "token"); err == nil {
		t.Fatal("expected missing app id to fail")
	}
	if _, err := NewLive("app-id", "", "token"); err == nil {
		t.Fatal("expected missing app secret to fail")
	}
	if _, err := NewLive("app-id", "secret", ""); err == nil {
		t.Fatal("expected missing callback token to fail")
	}
	// Valid credentials should succeed
	live, err := NewLive("app-id", "secret", "token")
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	if live.Name() != "wechat-live" {
		t.Fatalf("Name = %q", live.Name())
	}
}

func TestCallbackVerification(t *testing.T) {
	token := "test-token"
	timestamp := "1234567890"
	nonce := "nonce123"

	// Compute expected signature
	strs := []string{token, timestamp, nonce}
	combined := ""
	// Sort manually for the test
	if strs[0] > strs[1] {
		strs[0], strs[1] = strs[1], strs[0]
	}
	if strs[1] > strs[2] {
		strs[1], strs[2] = strs[2], strs[1]
	}
	if strs[0] > strs[1] {
		strs[0], strs[1] = strs[1], strs[0]
	}
	combined = strings.Join(strs, "")
	hash := sha1.New()
	hash.Write([]byte(combined))
	expectedSig := fmt.Sprintf("%x", hash.Sum(nil))

	if !verifySignature(token, timestamp, nonce, expectedSig) {
		t.Fatal("expected valid signature to pass verification")
	}
	if verifySignature(token, timestamp, nonce, "invalid-signature") {
		t.Fatal("expected invalid signature to fail verification")
	}
}

func TestCallbackXMLParsing(t *testing.T) {
	xmlBody := `<xml>
		<ToUserName><![CDATA[gh_test123]]></ToUserName>
		<FromUserName><![CDATA[oUser123]]></FromUserName>
		<CreateTime>1710000000</CreateTime>
		<MsgType><![CDATA[text]]></MsgType>
		<Content><![CDATA[/task list]]></Content>
		<MsgId>1234567890</MsgId>
	</xml>`

	var msg callbackXMLMessage
	if err := xml.Unmarshal([]byte(xmlBody), &msg); err != nil {
		t.Fatalf("XML unmarshal error: %v", err)
	}

	if msg.ToUserName != "gh_test123" {
		t.Fatalf("ToUserName = %q", msg.ToUserName)
	}
	if msg.FromUserName != "oUser123" {
		t.Fatalf("FromUserName = %q", msg.FromUserName)
	}
	if msg.CreateTime != 1710000000 {
		t.Fatalf("CreateTime = %d", msg.CreateTime)
	}
	if msg.MsgType != "text" {
		t.Fatalf("MsgType = %q", msg.MsgType)
	}
	if msg.Content != "/task list" {
		t.Fatalf("Content = %q", msg.Content)
	}
	if msg.MsgId != 1234567890 {
		t.Fatalf("MsgId = %d", msg.MsgId)
	}
}

func TestAccessTokenCaching(t *testing.T) {
	tokenRequests := 0
	live, err := NewLive("app-id", "secret", "token")
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	live.httpClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		tokenRequests++
		if !strings.Contains(req.URL.String(), "grant_type=client_credential") {
			t.Fatalf("unexpected request: %s", req.URL.String())
		}
		if !strings.Contains(req.URL.RawQuery, "appid=app-id") || !strings.Contains(req.URL.RawQuery, "secret=secret") {
			t.Fatalf("query = %s", req.URL.RawQuery)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok","access_token":"token-abc","expires_in":120}`)),
			Header:     make(http.Header),
		}, nil
	})}

	token, err := live.getAccessToken(context.Background())
	if err != nil {
		t.Fatalf("getAccessToken first error: %v", err)
	}
	if token != "token-abc" {
		t.Fatalf("token = %q", token)
	}

	// Second call should use cache
	token2, err := live.getAccessToken(context.Background())
	if err != nil {
		t.Fatalf("getAccessToken cached error: %v", err)
	}
	if token2 != "token-abc" {
		t.Fatalf("cached token = %q", token2)
	}
	if tokenRequests != 1 {
		t.Fatalf("tokenRequests = %d, want 1", tokenRequests)
	}
}

func TestAccessTokenError(t *testing.T) {
	live, err := NewLive("app-id", "secret", "token")
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	live.httpClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"errcode":40013,"errmsg":"invalid appid"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	if _, err := live.getAccessToken(context.Background()); err == nil || !strings.Contains(err.Error(), "wechat gettoken failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestLive_MetadataDeclaresWeChatCapabilities(t *testing.T) {
	live, err := NewLive("app-id", "secret", "token")
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	metadata := live.Metadata()
	if metadata.Source != "wechat" {
		t.Fatalf("source = %q", metadata.Source)
	}
	if metadata.Capabilities.ReadinessTier != "text_first" {
		t.Fatalf("ReadinessTier = %q", metadata.Capabilities.ReadinessTier)
	}
	if metadata.Rendering.MaxTextLength != 2048 {
		t.Fatalf("MaxTextLength = %d", metadata.Rendering.MaxTextLength)
	}
}

func TestLive_ReplyContextAndHelpers(t *testing.T) {
	live, err := NewLive("app-id", "secret", "token")
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if live.ReplyContextFromTarget(nil) != nil {
		t.Fatal("expected nil for nil target")
	}

	replyAny := live.ReplyContextFromTarget(&core.ReplyTarget{
		ConversationID: "chat-1",
		UserID:         "user-1",
	})
	reply, ok := replyAny.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyAny)
	}
	if reply.OpenID != "user-1" || reply.ChatID != "chat-1" {
		t.Fatalf("reply = %+v", reply)
	}
}

func TestLive_CallbackPaths(t *testing.T) {
	live, err := NewLive("app-id", "secret", "token")
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	paths := live.CallbackPaths()
	if len(paths) != 1 || paths[0] != "/wechat/callback" {
		t.Fatalf("CallbackPaths = %+v", paths)
	}
}

func TestLive_WithOptions(t *testing.T) {
	live, err := NewLive("app-id", "secret", "token",
		WithCallbackPort("9090"),
		WithCallbackPath("/custom/callback"),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	if live.callbackPort != "9090" {
		t.Fatalf("callbackPort = %q", live.callbackPort)
	}
	if live.callbackPath != "/custom/callback" {
		t.Fatalf("callbackPath = %q", live.callbackPath)
	}
}

func TestLive_SendCustomMessage(t *testing.T) {
	var sentBodies []map[string]any
	live, err := NewLive("app-id", "secret", "token")
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	live.httpClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/cgi-bin/token") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok","access_token":"token-xyz","expires_in":7200}`)),
				Header:     make(http.Header),
			}, nil
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode send body: %v", err)
		}
		sentBodies = append(sentBodies, body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	if err := live.Send(context.Background(), "oUser123", "hello"); err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if len(sentBodies) != 1 || sentBodies[0]["touser"] != "oUser123" {
		t.Fatalf("sentBodies = %+v", sentBodies)
	}
}

func TestToReplyContext_VariousTypes(t *testing.T) {
	raw := replyContext{OpenID: "user-1", ChatID: "chat-1"}
	if got := toReplyContext(raw); got != raw {
		t.Fatalf("toReplyContext(raw) = %+v", got)
	}
	if got := toReplyContext(&replyContext{OpenID: "user-2"}); got.OpenID != "user-2" {
		t.Fatalf("toReplyContext(pointer) = %+v", got)
	}
	if got := toReplyContext((*replyContext)(nil)); got != (replyContext{}) {
		t.Fatalf("toReplyContext(nil pointer) = %+v", got)
	}
	if got := toReplyContext("invalid"); got != (replyContext{}) {
		t.Fatalf("toReplyContext(invalid) = %+v", got)
	}
}

func TestParseEventTime(t *testing.T) {
	if got := parseEventTime(1710000000); !got.Equal(time.Unix(1710000000, 0)) {
		t.Fatalf("parseEventTime(valid) = %v", got)
	}
	before := time.Now()
	got := parseEventTime(0)
	after := time.Now()
	if got.Before(before) || got.After(after.Add(time.Second)) {
		t.Fatalf("parseEventTime(zero) = %v, want near now", got)
	}
}

func TestRenderStructuredFallback(t *testing.T) {
	if got := renderStructuredFallback(nil); got != "" {
		t.Fatalf("renderStructuredFallback(nil) = %q", got)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

