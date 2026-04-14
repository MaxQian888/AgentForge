package wecom

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentforge/im-bridge/core"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewLive_RequiresCredentialsAndCallbackSettings(t *testing.T) {
	if _, err := NewLive("", "1000002", "secret", "token", "9080", "/callback"); err == nil {
		t.Fatal("expected missing corp id to fail")
	}
	if _, err := NewLive("corp-id", "", "secret", "token", "9080", "/callback"); err == nil {
		t.Fatal("expected missing agent id to fail")
	}
	if _, err := NewLive("corp-id", "1000002", "", "token", "9080", "/callback"); err == nil {
		t.Fatal("expected missing agent secret to fail")
	}
	if _, err := NewLive("corp-id", "1000002", "secret", "", "9080", "/callback"); err == nil {
		t.Fatal("expected missing callback token to fail")
	}
	if _, err := NewLive("corp-id", "1000002", "secret", "token", "", "/callback"); err == nil {
		t.Fatal("expected missing callback port to fail")
	}
}

func TestLive_NormalizeInboundMessagePreservesReplyTargetContext(t *testing.T) {
	message, err := normalizeInboundMessage(callbackMessage{
		MsgID:       "msg-1",
		ChatID:      "chat-1",
		ChatType:    "group",
		ResponseURL: "https://work.weixin.qq.com/response",
		MsgType:     "text",
		Text: callbackText{
			Content: "@AgentForge /help",
		},
		From: callbackSender{
			UserID: "zhangsan",
		},
		CreatedAt: time.Unix(1710000000, 0).UnixMilli(),
	})
	if err != nil {
		t.Fatalf("normalizeInboundMessage error: %v", err)
	}
	if message.Platform != "wecom" {
		t.Fatalf("Platform = %q", message.Platform)
	}
	if message.SessionKey != "wecom:chat-1:zhangsan" {
		t.Fatalf("SessionKey = %q", message.SessionKey)
	}
	if message.Content != "@AgentForge /help" {
		t.Fatalf("Content = %q", message.Content)
	}
	if message.ReplyTarget == nil {
		t.Fatal("expected reply target")
	}
	if message.ReplyTarget.SessionWebhook != "https://work.weixin.qq.com/response" {
		t.Fatalf("SessionWebhook = %q", message.ReplyTarget.SessionWebhook)
	}
	if message.ReplyTarget.ResponseURL != "https://work.weixin.qq.com/response" {
		t.Fatalf("ResponseURL = %q", message.ReplyTarget.ResponseURL)
	}
	if message.ReplyTarget.ChatID != "chat-1" {
		t.Fatalf("ChatID = %q", message.ReplyTarget.ChatID)
	}
	if message.ReplyTarget.ConversationID != "chat-1" {
		t.Fatalf("ConversationID = %q", message.ReplyTarget.ConversationID)
	}
	if message.ReplyTarget.ProgressMode != string(core.AsyncUpdateSessionWebhook) {
		t.Fatalf("ProgressMode = %q", message.ReplyTarget.ProgressMode)
	}
	if !message.ReplyTarget.UseReply {
		t.Fatalf("ReplyTarget = %+v", message.ReplyTarget)
	}
}

func TestLive_ReplyPrefersResponseURLAndFallsBackToDirectSend(t *testing.T) {
	replier := &fakeResponseReplier{}
	sender := &fakeDirectSender{}
	live, err := NewLive(
		"corp-id",
		"1000002",
		"agent-secret",
		"callback-token",
		"9080",
		"/callback",
		WithResponseReplier(replier),
		WithDirectSender(sender),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.Reply(context.Background(), replyContext{ResponseURL: "https://work.weixin.qq.com/response", ChatID: "chat-1"}, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if len(replier.calls) != 1 {
		t.Fatalf("replier calls = %+v", replier.calls)
	}

	if err := live.Send(context.Background(), "chat-2", "send text"); err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("sender calls = %+v", sender.calls)
	}
	if sender.calls[0].Target.ChatID != "chat-2" {
		t.Fatalf("target = %+v", sender.calls[0].Target)
	}
}

func TestLive_SendStructuredFallsBackToRenderableText(t *testing.T) {
	sender := &fakeDirectSender{}
	live, err := NewLive(
		"corp-id",
		"1000002",
		"agent-secret",
		"callback-token",
		"9080",
		"/callback",
		WithDirectSender(sender),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	err = live.SendStructured(context.Background(), "chat-2", &core.StructuredMessage{
		Title: "Review Ready",
		Body:  "Choose the next step.",
		Actions: []core.StructuredAction{
			{ID: "act:approve:review-1", Label: "Approve", Style: core.ActionStylePrimary},
			{URL: "https://example.test/reviews/1", Label: "Open"},
		},
	})
	if err != nil {
		t.Fatalf("SendStructured error: %v", err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("sender calls = %+v", sender.calls)
	}
	if !strings.Contains(sender.calls[0].Content, "Review Ready") || !strings.Contains(sender.calls[0].Content, "Approve") {
		t.Fatalf("content = %q", sender.calls[0].Content)
	}
}

func TestLive_MetadataDeclaresWeComCallbackAndRenderingCapabilities(t *testing.T) {
	live, err := NewLive(
		"corp-id",
		"1000002",
		"agent-secret",
		"callback-token",
		"9080",
		"/callback",
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	metadata := live.Metadata()
	if metadata.Source != "wecom" {
		t.Fatalf("source = %q", metadata.Source)
	}
	if !metadata.Capabilities.RequiresPublicCallback {
		t.Fatal("expected public callback requirement")
	}
	if metadata.Capabilities.ActionCallbackMode != core.ActionCallbackWebhook {
		t.Fatalf("action callback mode = %q", metadata.Capabilities.ActionCallbackMode)
	}
	if metadata.Rendering.StructuredSurface == core.StructuredSurfaceNone {
		t.Fatalf("rendering = %+v", metadata.Rendering)
	}
	if len(metadata.Rendering.NativeSurfaces) != 1 || metadata.Rendering.NativeSurfaces[0] != core.NativeSurfaceWeComCard {
		t.Fatalf("NativeSurfaces = %+v", metadata.Rendering.NativeSurfaces)
	}
}

func TestLive_SendNativeUsesWeComCardPayload(t *testing.T) {
	replier := &fakeResponseReplier{}
	sender := &fakeDirectSender{}
	live, err := NewLive(
		"corp-id",
		"1000002",
		"agent-secret",
		"callback-token",
		"9080",
		"/callback",
		WithResponseReplier(replier),
		WithDirectSender(sender),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	message, err := core.NewWeComCardMessage(
		core.WeComCardTypeNews,
		"Review Ready",
		"Choose the next step.",
		"https://example.test/reviews/1",
		[]core.WeComArticle{{
			Title:       "Review Ready",
			Description: "Choose the next step.",
			URL:         "https://example.test/reviews/1",
		}},
		nil,
	)
	if err != nil {
		t.Fatalf("NewWeComCardMessage error: %v", err)
	}

	if err := live.SendNative(context.Background(), "chat-2", message); err != nil {
		t.Fatalf("SendNative error: %v", err)
	}
	if len(sender.messageCalls) != 1 {
		t.Fatalf("messageCalls = %+v", sender.messageCalls)
	}
	if sender.messageCalls[0].Payload["msgtype"] != "news" {
		t.Fatalf("payload = %+v", sender.messageCalls[0].Payload)
	}

	if err := live.ReplyNative(context.Background(), replyContext{ResponseURL: "https://work.weixin.qq.com/response"}, message); err != nil {
		t.Fatalf("ReplyNative error: %v", err)
	}
	if len(replier.messageCalls) != 1 || replier.messageCalls[0].Payload["msgtype"] != "news" {
		t.Fatalf("reply payload = %+v", replier.messageCalls)
	}
}

func TestRenderStructuredSectionsBuildsWeComNewsPayload(t *testing.T) {
	payload := renderStructuredSections([]core.StructuredSection{
		{
			Type: core.StructuredSectionTypeText,
			TextSection: &core.TextSection{
				Body: "Review Ready",
			},
		},
		{
			Type: core.StructuredSectionTypeImage,
			ImageSection: &core.ImageSection{
				URL: "https://example.test/review.png",
			},
		},
		{
			Type: core.StructuredSectionTypeFields,
			FieldsSection: &core.FieldsSection{
				Fields: []core.StructuredField{{Label: "Status", Value: "pending"}},
			},
		},
		{
			Type: core.StructuredSectionTypeActions,
			ActionsSection: &core.ActionsSection{
				Actions: []core.StructuredAction{{Label: "Open", URL: "https://example.test/reviews/1"}},
			},
		},
	})

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if string(raw) == "" {
		t.Fatal("expected serializable payload")
	}
	if payload.CardType != core.WeComCardTypeNews || len(payload.Articles) != 1 {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Articles[0].URL != "https://example.test/reviews/1" || payload.Articles[0].PicURL != "https://example.test/review.png" {
		t.Fatalf("article = %+v", payload.Articles[0])
	}
}

func TestLive_CallbackPathsIncludeConfiguredEndpoint(t *testing.T) {
	live, err := NewLive(
		"corp-id",
		"1000002",
		"agent-secret",
		"callback-token",
		"9080",
		"/callback",
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	paths := live.CallbackPaths()
	if len(paths) != 1 || paths[0] != "/callback" {
		t.Fatalf("CallbackPaths = %+v", paths)
	}
}

type fakeResponseReplier struct {
	calls        []responseReplyCall
	messageCalls []responseMessageCall
	err          error
}

type responseReplyCall struct {
	ResponseURL string
	Content     string
}

type responseMessageCall struct {
	ResponseURL string
	Payload     map[string]any
}

func (f *fakeResponseReplier) ReplyText(ctx context.Context, responseURL string, content string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, responseReplyCall{ResponseURL: responseURL, Content: content})
	return nil
}

func (f *fakeResponseReplier) ReplyMessage(ctx context.Context, responseURL string, payload map[string]any) error {
	if f.err != nil {
		return f.err
	}
	f.messageCalls = append(f.messageCalls, responseMessageCall{ResponseURL: responseURL, Payload: payload})
	return nil
}

type fakeDirectSender struct {
	calls        []directSendCall
	messageCalls []directSendMessageCall
	err          error
}

type directSendCall struct {
	Target  directSendTarget
	Content string
}

type directSendMessageCall struct {
	Target  directSendTarget
	Payload map[string]any
}

func (f *fakeDirectSender) SendText(ctx context.Context, target directSendTarget, content string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, directSendCall{Target: target, Content: content})
	return nil
}

func (f *fakeDirectSender) SendMessage(ctx context.Context, target directSendTarget, payload map[string]any) error {
	if f.err != nil {
		return f.err
	}
	f.messageCalls = append(f.messageCalls, directSendMessageCall{Target: target, Payload: payload})
	return nil
}

type staticTokenProvider string

func (s staticTokenProvider) AccessToken(ctx context.Context) (string, error) {
	if strings.TrimSpace(string(s)) == "" {
		return "", errors.New("missing token")
	}
	return string(s), nil
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestLive_ReplyContextAndFallbackHelpers(t *testing.T) {
	live, err := NewLive(
		"corp-id",
		"1000002",
		"agent-secret",
		"callback-token",
		"9080",
		"/callback",
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if live.Name() != "wecom-live" {
		t.Fatalf("Name = %q", live.Name())
	}
	if live.ReplyContextFromTarget(nil) != nil {
		t.Fatal("expected nil reply target to stay nil")
	}

	replyAny := live.ReplyContextFromTarget(&core.ReplyTarget{
		SessionWebhook: " https://work.weixin.qq.com/response ",
		ConversationID: "chat-1",
		UserID:         "zhangsan",
	})
	reply, ok := replyAny.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyAny)
	}
	if reply.ResponseURL != "https://work.weixin.qq.com/response" || reply.ChatID != "chat-1" || reply.UserID != "zhangsan" {
		t.Fatalf("reply = %+v", reply)
	}

	if got := renderStructuredFallback(nil); got != "" {
		t.Fatalf("renderStructuredFallback(nil) = %q", got)
	}
	if got := renderStructuredFallback(&core.StructuredMessage{Title: "Review Ready", Body: "Ship it"}); !strings.Contains(got, "Review Ready") || strings.Contains(got, "WeCom richer card update is unavailable") {
		t.Fatalf("renderStructuredFallback(no actions) = %q", got)
	}
	if got := renderStructuredFallback(&core.StructuredMessage{
		Title: "Review Ready",
		Body:  "Ship it",
		Actions: []core.StructuredAction{
			{Label: "Open", URL: "https://example.test/reviews/1"},
		},
	}); !strings.Contains(got, "WeCom richer card update is unavailable") {
		t.Fatalf("renderStructuredFallback(actions) = %q", got)
	}

	sender := &fakeDirectSender{}
	replier := &fakeResponseReplier{}
	live, err = NewLive(
		"corp-id",
		"1000002",
		"agent-secret",
		"callback-token",
		"9080",
		"/callback",
		WithDirectSender(sender),
		WithResponseReplier(replier),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive second error: %v", err)
	}
	if err := live.ReplyStructured(context.Background(), replyContext{
		ResponseURL: "https://work.weixin.qq.com/response",
		ChatID:      "chat-1",
	}, &core.StructuredMessage{
		Sections: []core.StructuredSection{{
			Type: core.StructuredSectionTypeText,
			TextSection: &core.TextSection{
				Body: "Build ready",
			},
		}},
	}); err != nil {
		t.Fatalf("ReplyStructured error: %v", err)
	}
	if len(replier.messageCalls) != 1 || replier.messageCalls[0].Payload["msgtype"] == nil {
		t.Fatalf("replier.messageCalls = %+v", replier.messageCalls)
	}
}

func TestWeComHelpers_ParseTargetsReplyContextAndTime(t *testing.T) {
	testCases := []struct {
		name     string
		incoming callbackMessage
		wantErr  string
	}{
		{
			name: "unsupported message type",
			incoming: callbackMessage{
				MsgType: "image",
				Text:    callbackText{Content: "ignored"},
				ChatID:  "chat-1",
				From:    callbackSender{UserID: "zhangsan"},
			},
			wantErr: "unsupported wecom message type",
		},
		{
			name: "missing content",
			incoming: callbackMessage{
				MsgType: "text",
				ChatID:  "chat-1",
				From:    callbackSender{UserID: "zhangsan"},
			},
			wantErr: "missing text content",
		},
		{
			name: "missing chat",
			incoming: callbackMessage{
				MsgType: "text",
				Text:    callbackText{Content: "hello"},
				From:    callbackSender{UserID: "zhangsan"},
			},
			wantErr: "missing chat id",
		},
		{
			name: "missing sender",
			incoming: callbackMessage{
				MsgType: "text",
				Text:    callbackText{Content: "hello"},
				ChatID:  "chat-1",
			},
			wantErr: "missing sender user id",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := normalizeInboundMessage(tc.incoming); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}

	raw := replyContext{ResponseURL: "https://work.weixin.qq.com/response", ChatID: "chat-1", UserID: "zhangsan"}
	if got := toReplyContext(raw); got != raw {
		t.Fatalf("toReplyContext(raw) = %+v", got)
	}
	if got := toReplyContext(&replyContext{ChatID: "chat-2", UserID: "lisi"}); got.ChatID != "chat-2" || got.UserID != "lisi" {
		t.Fatalf("toReplyContext(pointer) = %+v", got)
	}
	msg := &core.Message{ChatID: "chat-3", UserID: "wangwu", ThreadID: "thread-1"}
	if got := toReplyContext(msg); got.ChatID != "chat-3" || got.UserID != "wangwu" {
		t.Fatalf("toReplyContext(message) = %+v", got)
	}
	target := &core.ReplyTarget{SessionWebhook: "https://work.weixin.qq.com/hook", ConversationID: "chat-4", UserID: "zhaoliu"}
	if got := toReplyContext(target); got.ResponseURL != "https://work.weixin.qq.com/hook" || got.ChatID != "chat-4" || got.UserID != "zhaoliu" {
		t.Fatalf("toReplyContext(target) = %+v", got)
	}
	if got := toReplyContext("invalid"); got != (replyContext{}) {
		t.Fatalf("toReplyContext(invalid) = %+v", got)
	}

	if got := parseDirectSendTarget(""); got != (directSendTarget{}) {
		t.Fatalf("parseDirectSendTarget(empty) = %+v", got)
	}
	if got := parseDirectSendTarget("user:zhangsan"); got.UserID != "zhangsan" || got.ChatID != "" {
		t.Fatalf("parseDirectSendTarget(user) = %+v", got)
	}
	if got := parseDirectSendTarget("chat-1"); got.ChatID != "chat-1" || got.UserID != "" {
		t.Fatalf("parseDirectSendTarget(chat) = %+v", got)
	}

	if got := firstNonEmpty(" ", "chat-1", "chat-2"); got != "chat-1" {
		t.Fatalf("firstNonEmpty = %q", got)
	}

	if got := parseEventTime(1710000000000); !got.Equal(time.UnixMilli(1710000000000)) {
		t.Fatalf("parseEventTime(valid) = %v", got)
	}
	before := time.Now()
	got := parseEventTime(0)
	after := time.Now()
	if got.Before(before) || got.After(after.Add(time.Second)) {
		t.Fatalf("parseEventTime(zero) = %v, want near now", got)
	}
}

func TestCachedAccessTokenProviderAndHTTPSenders(t *testing.T) {
	tokenRequests := 0
	tokenClient := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		tokenRequests++
		if req.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", req.Method)
		}
		if !strings.Contains(req.URL.RawQuery, "corpid=corp-id") || !strings.Contains(req.URL.RawQuery, "corpsecret=agent-secret") {
			t.Fatalf("query = %s", req.URL.RawQuery)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok","access_token":"token-123","expires_in":120}`)),
			Header:     make(http.Header),
		}, nil
	})}
	provider := &cachedAccessTokenProvider{
		corpID:      "corp-id",
		agentSecret: "agent-secret",
		client:      tokenClient,
	}

	token, err := provider.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken first error: %v", err)
	}
	if token != "token-123" {
		t.Fatalf("token = %q", token)
	}
	if _, err := provider.AccessToken(context.Background()); err != nil {
		t.Fatalf("AccessToken cached error: %v", err)
	}
	if tokenRequests != 1 {
		t.Fatalf("tokenRequests = %d, want 1", tokenRequests)
	}

	errorProvider := &cachedAccessTokenProvider{
		corpID:      "corp-id",
		agentSecret: "agent-secret",
		client: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"errcode":40013,"errmsg":"invalid corpid"}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	if _, err := errorProvider.AccessToken(context.Background()); err == nil || !strings.Contains(err.Error(), "wecom gettoken failed") {
		t.Fatalf("errorProvider err = %v", err)
	}

	responseBodies := make([]map[string]any, 0, 2)
	responseClient := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode response replier body: %v", err)
		}
		responseBodies = append(responseBodies, body)
		if strings.Contains(req.URL.String(), "bad") {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("upstream boom")),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`ok`)),
			Header:     make(http.Header),
		}, nil
	})}
	replier := &httpResponseReplier{client: responseClient}
	if err := replier.ReplyText(context.Background(), "https://work.weixin.qq.com/response", "hello"); err != nil {
		t.Fatalf("ReplyText error: %v", err)
	}
	if err := replier.ReplyMessage(context.Background(), "https://work.weixin.qq.com/bad", map[string]any{"msgtype": "text"}); err == nil || !strings.Contains(err.Error(), "wecom response_url reply failed: upstream boom") {
		t.Fatalf("ReplyMessage error = %v", err)
	}
	if len(responseBodies) == 0 || responseBodies[0]["msgtype"] != "text" {
		t.Fatalf("responseBodies = %+v", responseBodies)
	}

	sendBodies := make([]map[string]any, 0, 2)
	sender := &apiDirectSender{
		agentID:       "1000002",
		tokenProvider: staticTokenProvider("token-123"),
		client: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", req.Method)
			}
			if !strings.Contains(req.URL.RawQuery, "access_token=token-123") {
				t.Fatalf("query = %s", req.URL.RawQuery)
			}
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("decode direct send body: %v", err)
			}
			sendBodies = append(sendBodies, body)
			if body["chatid"] == "bad-chat" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"errcode":82001,"errmsg":"send denied"}`)),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok"}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	if err := sender.SendText(context.Background(), directSendTarget{ChatID: "chat-1"}, "hello"); err != nil {
		t.Fatalf("SendText error: %v", err)
	}
	if err := sender.SendMessage(context.Background(), directSendTarget{UserID: "zhangsan"}, map[string]any{"msgtype": "textcard", "textcard": map[string]any{"title": "Review Ready"}}); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if len(sendBodies) != 2 {
		t.Fatalf("sendBodies = %+v", sendBodies)
	}
	if sendBodies[0]["chatid"] != "chat-1" || sendBodies[0]["agentid"] != "1000002" || sendBodies[0]["safe"] != float64(0) {
		t.Fatalf("first send body = %+v", sendBodies[0])
	}
	if sendBodies[1]["touser"] != "zhangsan" {
		t.Fatalf("second send body = %+v", sendBodies[1])
	}
	if err := sender.SendMessage(context.Background(), directSendTarget{}, map[string]any{"msgtype": "text"}); err == nil || !strings.Contains(err.Error(), "requires chat id or user id") {
		t.Fatalf("empty target error = %v", err)
	}
	if err := sender.SendMessage(context.Background(), directSendTarget{ChatID: "chat-1"}, nil); err == nil || !strings.Contains(err.Error(), "payload is required") {
		t.Fatalf("nil payload error = %v", err)
	}
	if err := sender.SendMessage(context.Background(), directSendTarget{ChatID: "bad-chat"}, map[string]any{"msgtype": "text"}); err == nil || !strings.Contains(err.Error(), "wecom direct send failed: send denied") {
		t.Fatalf("upstream err = %v", err)
	}
}

func TestLive_SendFormattedTextUsesMarkdownPayloadForWeComMD(t *testing.T) {
	sender := &fakeDirectSender{}
	replier := &fakeResponseReplier{}
	live, err := NewLive(
		"corp-id", "1000002", "agent-secret", "callback-token", "9080", "/callback",
		WithDirectSender(sender),
		WithResponseReplier(replier),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	// SendFormattedText with WeComMD format should use markdown msgtype
	if err := live.SendFormattedText(context.Background(), "chat-1", &core.FormattedText{
		Content: "**bold text**",
		Format:  core.TextFormatWeComMD,
	}); err != nil {
		t.Fatalf("SendFormattedText error: %v", err)
	}
	if len(sender.messageCalls) != 1 {
		t.Fatalf("messageCalls = %+v", sender.messageCalls)
	}
	if sender.messageCalls[0].Payload["msgtype"] != "markdown" {
		t.Fatalf("msgtype = %v", sender.messageCalls[0].Payload["msgtype"])
	}

	// SendFormattedText with plain text should fall back to Send
	sender.messageCalls = nil
	if err := live.SendFormattedText(context.Background(), "chat-2", &core.FormattedText{
		Content: "plain text",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("SendFormattedText plain error: %v", err)
	}
	if len(sender.calls) != 1 || sender.calls[0].Content != "plain text" {
		t.Fatalf("sender.calls = %+v", sender.calls)
	}

	// nil message returns error
	if err := live.SendFormattedText(context.Background(), "chat-1", nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("nil message error = %v", err)
	}
}

func TestLive_ReplyFormattedTextUsesResponseURLOrDirectSend(t *testing.T) {
	sender := &fakeDirectSender{}
	replier := &fakeResponseReplier{}
	live, err := NewLive(
		"corp-id", "1000002", "agent-secret", "callback-token", "9080", "/callback",
		WithDirectSender(sender),
		WithResponseReplier(replier),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	// Reply with response URL
	if err := live.ReplyFormattedText(context.Background(), replyContext{
		ResponseURL: "https://work.weixin.qq.com/response",
		ChatID:      "chat-1",
	}, &core.FormattedText{
		Content: "**markdown reply**",
		Format:  core.TextFormatWeComMD,
	}); err != nil {
		t.Fatalf("ReplyFormattedText error: %v", err)
	}
	if len(replier.messageCalls) != 1 || replier.messageCalls[0].Payload["msgtype"] != "markdown" {
		t.Fatalf("replier.messageCalls = %+v", replier.messageCalls)
	}

	// Reply without response URL falls back to direct send
	if err := live.ReplyFormattedText(context.Background(), replyContext{
		ChatID: "chat-2",
	}, &core.FormattedText{
		Content: "**direct markdown**",
		Format:  core.TextFormatWeComMD,
	}); err != nil {
		t.Fatalf("ReplyFormattedText direct error: %v", err)
	}
	if len(sender.messageCalls) != 1 || sender.messageCalls[0].Payload["msgtype"] != "markdown" {
		t.Fatalf("sender.messageCalls = %+v", sender.messageCalls)
	}

	// nil message returns error
	if err := live.ReplyFormattedText(context.Background(), replyContext{ChatID: "chat-1"}, nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("nil message error = %v", err)
	}

	// Missing target returns error
	if err := live.ReplyFormattedText(context.Background(), replyContext{}, &core.FormattedText{
		Content: "no target",
		Format:  core.TextFormatWeComMD,
	}); err == nil || !strings.Contains(err.Error(), "requires response url, chat id, or user id") {
		t.Fatalf("missing target error = %v", err)
	}
}

func TestLive_UpdateFormattedTextDelegatesToReply(t *testing.T) {
	replier := &fakeResponseReplier{}
	live, err := NewLive(
		"corp-id", "1000002", "agent-secret", "callback-token", "9080", "/callback",
		WithResponseReplier(replier),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.UpdateFormattedText(context.Background(), replyContext{
		ResponseURL: "https://work.weixin.qq.com/response",
	}, &core.FormattedText{
		Content: "**updated**",
		Format:  core.TextFormatWeComMD,
	}); err != nil {
		t.Fatalf("UpdateFormattedText error: %v", err)
	}
	if len(replier.messageCalls) != 1 || replier.messageCalls[0].Payload["msgtype"] != "markdown" {
		t.Fatalf("replier.messageCalls = %+v", replier.messageCalls)
	}
}

func TestLive_SendCardAndReplyCardUseTextCardPayload(t *testing.T) {
	sender := &fakeDirectSender{}
	replier := &fakeResponseReplier{}
	live, err := NewLive(
		"corp-id", "1000002", "agent-secret", "callback-token", "9080", "/callback",
		WithDirectSender(sender),
		WithResponseReplier(replier),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	card := core.NewCard().
		SetTitle("Review Ready").
		AddField("Status", "pending").
		AddField("Author", "zhangsan").
		AddPrimaryButton("Open", "link:https://example.test/reviews/1")

	// SendCard
	if err := live.SendCard(context.Background(), "chat-1", card); err != nil {
		t.Fatalf("SendCard error: %v", err)
	}
	if len(sender.messageCalls) != 1 {
		t.Fatalf("sender.messageCalls = %+v", sender.messageCalls)
	}
	if sender.messageCalls[0].Payload["msgtype"] != "textcard" {
		t.Fatalf("msgtype = %v", sender.messageCalls[0].Payload["msgtype"])
	}
	textcard, ok := sender.messageCalls[0].Payload["textcard"].(map[string]any)
	if !ok {
		t.Fatalf("textcard type = %T", sender.messageCalls[0].Payload["textcard"])
	}
	if textcard["title"] != "Review Ready" {
		t.Fatalf("title = %v", textcard["title"])
	}
	if textcard["url"] != "https://example.test/reviews/1" {
		t.Fatalf("url = %v", textcard["url"])
	}

	// ReplyCard via response URL
	if err := live.ReplyCard(context.Background(), replyContext{
		ResponseURL: "https://work.weixin.qq.com/response",
		ChatID:      "chat-1",
	}, card); err != nil {
		t.Fatalf("ReplyCard error: %v", err)
	}
	if len(replier.messageCalls) != 1 || replier.messageCalls[0].Payload["msgtype"] != "textcard" {
		t.Fatalf("replier.messageCalls = %+v", replier.messageCalls)
	}

	// ReplyCard via direct send (no response URL)
	sender.messageCalls = nil
	if err := live.ReplyCard(context.Background(), replyContext{ChatID: "chat-2"}, card); err != nil {
		t.Fatalf("ReplyCard direct error: %v", err)
	}
	if len(sender.messageCalls) != 1 || sender.messageCalls[0].Payload["msgtype"] != "textcard" {
		t.Fatalf("sender direct messageCalls = %+v", sender.messageCalls)
	}

	// nil card returns error
	if err := live.SendCard(context.Background(), "chat-1", nil); err == nil || !strings.Contains(err.Error(), "card is required") {
		t.Fatalf("nil card error = %v", err)
	}
	if err := live.ReplyCard(context.Background(), replyContext{ChatID: "chat-1"}, nil); err == nil || !strings.Contains(err.Error(), "card is required") {
		t.Fatalf("nil reply card error = %v", err)
	}

	// Missing target returns error
	if err := live.ReplyCard(context.Background(), replyContext{}, card); err == nil || !strings.Contains(err.Error(), "requires response url, chat id, or user id") {
		t.Fatalf("missing target error = %v", err)
	}
}

func TestLive_MetadataIncludesWeComMDInSupportedFormats(t *testing.T) {
	live, err := NewLive("corp-id", "1000002", "agent-secret", "callback-token", "9080", "/callback")
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	metadata := live.Metadata()
	found := false
	for _, format := range metadata.Rendering.SupportedFormats {
		if format == core.TextFormatWeComMD {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("SupportedFormats = %+v, want TextFormatWeComMD", metadata.Rendering.SupportedFormats)
	}
}
