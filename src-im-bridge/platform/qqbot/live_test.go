package qqbot

import (
	"context"
	"encoding/json"
	"github.com/agentforge/im-bridge/core"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewLive_RequiresCredentialsAndCallbackSettings(t *testing.T) {
	if _, err := NewLive("", "secret", "9080", "/callback"); err == nil {
		t.Fatal("expected missing app id to fail")
	}
	if _, err := NewLive("1024", "", "9080", "/callback"); err == nil {
		t.Fatal("expected missing app secret to fail")
	}
	if _, err := NewLive("1024", "secret", "", "/callback"); err == nil {
		t.Fatal("expected missing callback port to fail")
	}
}

func TestLive_NormalizeInboundMessagePreservesReplyTargetContext(t *testing.T) {
	message, err := normalizeInboundPayload(webhookPayload{
		Type: "GROUP_AT_MESSAGE_CREATE",
		Data: inboundMessage{
			ID:          "evt-1",
			Content:     "@AgentForge /help",
			GroupOpenID: "group-openid",
			Author: inboundAuthor{
				UserOpenID: "user-openid",
				Username:   "QQ Bot User",
			},
		},
	})
	if err != nil {
		t.Fatalf("normalizeInboundPayload error: %v", err)
	}
	if message.Platform != "qqbot" {
		t.Fatalf("Platform = %q", message.Platform)
	}
	if message.ReplyTarget == nil || message.ReplyTarget.ChatID != "group-openid" || message.ReplyTarget.MessageID != "evt-1" {
		t.Fatalf("ReplyTarget = %+v", message.ReplyTarget)
	}
}

func TestLive_ReplyPrefersReplyContextAndFallsBackToDirectSend(t *testing.T) {
	sender := &fakeSender{}
	live, err := NewLive(
		"1024",
		"secret",
		"9080",
		"/callback",
		WithSender(sender),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.Reply(context.Background(), replyContext{ChatID: "group-openid", MessageID: "evt-1"}, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := live.Send(context.Background(), "user:user-openid", "send text"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if len(sender.calls) != 2 {
		t.Fatalf("sender calls = %+v", sender.calls)
	}
	if sender.calls[0].Target.GroupOpenID != "group-openid" || sender.calls[0].Target.MessageID != "evt-1" {
		t.Fatalf("first target = %+v", sender.calls[0].Target)
	}
	if sender.calls[1].Target.UserOpenID != "user-openid" {
		t.Fatalf("second target = %+v", sender.calls[1].Target)
	}
}

func TestLive_SendStructuredRendersAsMarkdownNative(t *testing.T) {
	sender := &fakeSender{}
	live, err := NewLive(
		"1024",
		"secret",
		"9080",
		"/callback",
		WithSender(sender),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	err = live.SendStructured(context.Background(), "group:group-openid", &core.StructuredMessage{
		Title: "Review Ready",
		Body:  "Choose the next step.",
		Actions: []core.StructuredAction{
			{ID: "act:approve:review-1", Label: "Approve", Style: core.ActionStylePrimary},
		},
	})
	if err != nil {
		t.Fatalf("SendStructured error: %v", err)
	}
	// Structured messages now render as markdown and go through the native path
	if len(sender.messageCalls) != 1 {
		t.Fatalf("messageCalls = %+v", sender.messageCalls)
	}
	if sender.messageCalls[0].Payload["msg_type"] != 2 {
		t.Fatalf("payload = %+v", sender.messageCalls[0].Payload)
	}
	md, _ := sender.messageCalls[0].Payload["markdown"].(map[string]any)
	content, _ := md["content"].(string)
	if !strings.Contains(content, "Review Ready") || !strings.Contains(content, "Approve") {
		t.Fatalf("markdown content = %q", content)
	}
}

func TestLive_MetadataDeclaresQQBotCallbackCapabilities(t *testing.T) {
	live, err := NewLive(
		"1024",
		"secret",
		"9080",
		"/callback",
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	metadata := live.Metadata()
	if metadata.Source != "qqbot" {
		t.Fatalf("source = %q", metadata.Source)
	}
	if !metadata.Capabilities.RequiresPublicCallback {
		t.Fatal("expected public callback requirement")
	}
	if !metadata.Capabilities.SupportsSlashCommands {
		t.Fatal("expected slash command capability")
	}
	if len(metadata.Rendering.NativeSurfaces) != 1 || metadata.Rendering.NativeSurfaces[0] != core.NativeSurfaceQQBotMarkdown {
		t.Fatalf("NativeSurfaces = %+v", metadata.Rendering.NativeSurfaces)
	}
}

func TestLive_SendNativeUsesQQBotMarkdownPayload(t *testing.T) {
	sender := &fakeSender{}
	live, err := NewLive(
		"1024",
		"secret",
		"9080",
		"/callback",
		WithSender(sender),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	message, err := core.NewQQBotMarkdownMessage(
		"## Review Ready",
		[][]core.QQBotKeyboardButton{{
			{Label: "Open", URL: "https://example.test/reviews/1"},
		}},
	)
	if err != nil {
		t.Fatalf("NewQQBotMarkdownMessage error: %v", err)
	}

	if err := live.SendNative(context.Background(), "group:group-openid", message); err != nil {
		t.Fatalf("SendNative error: %v", err)
	}
	if len(sender.messageCalls) != 1 {
		t.Fatalf("messageCalls = %+v", sender.messageCalls)
	}
	if sender.messageCalls[0].Payload["msg_type"] != 2 {
		t.Fatalf("payload = %+v", sender.messageCalls[0].Payload)
	}
}

type fakeSender struct {
	calls        []sendCall
	messageCalls []sendMessageCall
}

type sendCall struct {
	Target  messageTarget
	Content string
}

type sendMessageCall struct {
	Target  messageTarget
	Payload map[string]any
}

func (f *fakeSender) SendText(ctx context.Context, target messageTarget, content string) error {
	f.calls = append(f.calls, sendCall{Target: target, Content: content})
	return nil
}

func (f *fakeSender) SendMessage(ctx context.Context, target messageTarget, payload map[string]any) error {
	f.messageCalls = append(f.messageCalls, sendMessageCall{Target: target, Payload: payload})
	return nil
}

type staticTokenProvider string

func (s staticTokenProvider) AccessToken(ctx context.Context) (string, error) {
	return string(s), nil
}

func TestLive_SendFormattedTextUsesMarkdownWhenQQBotMD(t *testing.T) {
	sender := &fakeSender{}
	live, err := NewLive(
		"1024",
		"secret",
		"9080",
		"/callback",
		WithSender(sender),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.SendFormattedText(context.Background(), "group:group-openid", &core.FormattedText{
		Content: "## Review Ready",
		Format:  core.TextFormatQQBotMD,
	}); err != nil {
		t.Fatalf("SendFormattedText error: %v", err)
	}
	if len(sender.messageCalls) != 1 {
		t.Fatalf("messageCalls = %+v", sender.messageCalls)
	}
	if sender.messageCalls[0].Payload["msg_type"] != 2 {
		t.Fatalf("payload = %+v", sender.messageCalls[0].Payload)
	}

	// plain text format should fall back to SendText
	if err := live.SendFormattedText(context.Background(), "group:group-openid", &core.FormattedText{
		Content: "plain text",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("SendFormattedText plain error: %v", err)
	}
	if len(sender.calls) != 1 || sender.calls[0].Content != "plain text" {
		t.Fatalf("calls = %+v", sender.calls)
	}

	if err := live.SendFormattedText(context.Background(), "group:group-openid", nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("nil message error = %v", err)
	}
}

func TestLive_ReplyFormattedTextUsesMarkdownWhenQQBotMD(t *testing.T) {
	sender := &fakeSender{}
	live, err := NewLive(
		"1024",
		"secret",
		"9080",
		"/callback",
		WithSender(sender),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.ReplyFormattedText(context.Background(), replyContext{ChatID: "group-openid", MessageID: "evt-1", IsGroup: true}, &core.FormattedText{
		Content: "## Reply Ready",
		Format:  core.TextFormatQQBotMD,
	}); err != nil {
		t.Fatalf("ReplyFormattedText error: %v", err)
	}
	if len(sender.messageCalls) != 1 {
		t.Fatalf("messageCalls = %+v", sender.messageCalls)
	}
	if sender.messageCalls[0].Payload["msg_type"] != 2 {
		t.Fatalf("payload = %+v", sender.messageCalls[0].Payload)
	}

	if err := live.ReplyFormattedText(context.Background(), replyContext{ChatID: "group-openid"}, nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("nil message error = %v", err)
	}
}

func TestLive_UpdateFormattedTextDelegatesToReply(t *testing.T) {
	sender := &fakeSender{}
	live, err := NewLive(
		"1024",
		"secret",
		"9080",
		"/callback",
		WithSender(sender),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.UpdateFormattedText(context.Background(), replyContext{ChatID: "group-openid", MessageID: "evt-1", IsGroup: true}, &core.FormattedText{
		Content: "## Update Ready",
		Format:  core.TextFormatQQBotMD,
	}); err != nil {
		t.Fatalf("UpdateFormattedText error: %v", err)
	}
	if len(sender.messageCalls) != 1 {
		t.Fatalf("messageCalls = %+v", sender.messageCalls)
	}
}

func TestLive_SendStructuredRendersAsMarkdown(t *testing.T) {
	sender := &fakeSender{}
	live, err := NewLive(
		"1024",
		"secret",
		"9080",
		"/callback",
		WithSender(sender),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	err = live.SendStructured(context.Background(), "group:group-openid", &core.StructuredMessage{
		Sections: []core.StructuredSection{
			{
				Type:        core.StructuredSectionTypeText,
				TextSection: &core.TextSection{Body: "Review Ready"},
			},
			{
				Type: core.StructuredSectionTypeFields,
				FieldsSection: &core.FieldsSection{
					Fields: []core.StructuredField{
						{Label: "Status", Value: "Open"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SendStructured error: %v", err)
	}
	// Should send via markdown native path
	if len(sender.messageCalls) != 1 {
		t.Fatalf("messageCalls = %+v", sender.messageCalls)
	}
	if sender.messageCalls[0].Payload["msg_type"] != 2 {
		t.Fatalf("payload = %+v", sender.messageCalls[0].Payload)
	}
}

func TestLive_OptionsIdentityAndReplyContextHelpers(t *testing.T) {
	live, err := NewLive(
		"1024",
		"secret",
		"9080",
		"callback",
		WithAPIBase("https://api.example.test/"),
		WithTokenBase("https://token.example.test/"),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if live.Name() != "qqbot-live" {
		t.Fatalf("Name = %q", live.Name())
	}
	if live.apiBase != "https://api.example.test" {
		t.Fatalf("apiBase = %q", live.apiBase)
	}
	if live.tokenBase != "https://token.example.test" {
		t.Fatalf("tokenBase = %q", live.tokenBase)
	}
	if !reflect.DeepEqual(live.CallbackPaths(), []string{"/callback"}) {
		t.Fatalf("CallbackPaths = %+v", live.CallbackPaths())
	}
	if live.ReplyContextFromTarget(nil) != nil {
		t.Fatal("expected nil reply target to stay nil")
	}

	replyAny := live.ReplyContextFromTarget(&core.ReplyTarget{
		ConversationID: "group-openid",
		UserID:         "user-openid",
		MessageID:      "evt-1",
	})
	reply, ok := replyAny.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyAny)
	}
	if reply.ChatID != "group-openid" || reply.UserID != "user-openid" || reply.MessageID != "evt-1" || !reply.IsGroup {
		t.Fatalf("reply = %+v", reply)
	}
}

func TestQQBotLiveHelpers_ParseTargetsAndEventTime(t *testing.T) {
	if got := parseTarget(""); got.GroupOpenID != "" || got.UserOpenID != "" {
		t.Fatalf("parseTarget(empty) = %+v", got)
	}
	if got := parseTarget("user:user-openid"); got.UserOpenID != "user-openid" || got.GroupOpenID != "" {
		t.Fatalf("parseTarget(user) = %+v", got)
	}
	if got := parseTarget("group:group-openid"); got.GroupOpenID != "group-openid" || got.UserOpenID != "" {
		t.Fatalf("parseTarget(group) = %+v", got)
	}
	if got := parseTarget("group-openid"); got.GroupOpenID != "group-openid" {
		t.Fatalf("parseTarget(default) = %+v", got)
	}

	if got := targetFromReply(replyContext{ChatID: "group-openid", MessageID: "evt-1", IsGroup: true}); got.GroupOpenID != "group-openid" || got.MessageID != "evt-1" {
		t.Fatalf("targetFromReply(group) = %+v", got)
	}
	if got := targetFromReply(replyContext{UserID: "user-openid", MessageID: "evt-2"}); got.UserOpenID != "user-openid" || got.MessageID != "evt-2" {
		t.Fatalf("targetFromReply(user) = %+v", got)
	}
	if got := targetFromReply(replyContext{}); got != (messageTarget{}) {
		t.Fatalf("targetFromReply(empty) = %+v", got)
	}

	if got := parseEventTime("2026-03-30T10:20:30Z"); !got.Equal(time.Date(2026, 3, 30, 10, 20, 30, 0, time.UTC)) {
		t.Fatalf("parseEventTime(valid) = %v", got)
	}
	before := time.Now()
	got := parseEventTime("bad-time")
	after := time.Now()
	if got.Before(before) || got.After(after.Add(time.Second)) {
		t.Fatalf("parseEventTime(invalid) = %v, want near now", got)
	}
}

func TestLive_ReplyNativeUsesReplyTargetsAndRejectsMissingTargets(t *testing.T) {
	sender := &fakeSender{}
	live, err := NewLive(
		"1024",
		"secret",
		"9080",
		"/callback",
		WithSender(sender),
		WithAccessTokenProvider(staticTokenProvider("token")),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	message, err := core.NewQQBotMarkdownMessage("## Review Ready", nil)
	if err != nil {
		t.Fatalf("NewQQBotMarkdownMessage error: %v", err)
	}

	if err := live.ReplyNative(context.Background(), replyContext{ChatID: "group-openid", MessageID: "evt-1", IsGroup: true}, message); err != nil {
		t.Fatalf("ReplyNative error: %v", err)
	}
	if len(sender.messageCalls) != 1 || sender.messageCalls[0].Target.GroupOpenID != "group-openid" || sender.messageCalls[0].Target.MessageID != "evt-1" {
		t.Fatalf("messageCalls = %+v", sender.messageCalls)
	}

	if err := live.ReplyNative(context.Background(), replyContext{}, message); err == nil || !strings.Contains(err.Error(), "requires group or user target") {
		t.Fatalf("missing target error = %v", err)
	}
}

func TestCachedAccessTokenProvider_CachesAndValidatesResponses(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/app/getAppAccessToken" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["appId"] != "1024" || body["clientSecret"] != "secret" {
			t.Fatalf("body = %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "token-123",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	provider := &cachedAccessTokenProvider{
		appID:     "1024",
		appSecret: "secret",
		tokenBase: server.URL,
		client:    server.Client(),
	}

	token, err := provider.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken first error: %v", err)
	}
	if token != "token-123" {
		t.Fatalf("token = %q", token)
	}
	token, err = provider.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken second error: %v", err)
	}
	if token != "token-123" {
		t.Fatalf("cached token = %q", token)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}

	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusBadGateway)
	}))
	defer errorServer.Close()
	provider = &cachedAccessTokenProvider{
		appID:     "1024",
		appSecret: "secret",
		tokenBase: errorServer.URL,
		client:    errorServer.Client(),
	}
	if _, err := provider.AccessToken(context.Background()); err == nil || !strings.Contains(err.Error(), "qqbot access token request failed") {
		t.Fatalf("error response = %v", err)
	}

	missingTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"expires_in":120}`))
	}))
	defer missingTokenServer.Close()
	provider = &cachedAccessTokenProvider{
		appID:     "1024",
		appSecret: "secret",
		tokenBase: missingTokenServer.URL,
		client:    missingTokenServer.Client(),
	}
	if _, err := provider.AccessToken(context.Background()); err == nil || !strings.Contains(err.Error(), "missing access_token") {
		t.Fatalf("missing token error = %v", err)
	}
}

func TestAPISender_SendTextAndMessage(t *testing.T) {
	requests := make([]map[string]any, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "QQBot token-123" {
			t.Fatalf("Authorization = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		body["_path"] = r.URL.Path
		requests = append(requests, body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := &apiSender{
		apiBase:       server.URL,
		tokenProvider: staticTokenProvider("token-123"),
		client:        server.Client(),
	}

	if err := sender.SendText(context.Background(), messageTarget{GroupOpenID: "group-openid", MessageID: "evt-1"}, " hello "); err != nil {
		t.Fatalf("SendText error: %v", err)
	}
	if err := sender.SendMessage(context.Background(), messageTarget{UserOpenID: "user-openid"}, map[string]any{"msg_type": 2, "markdown": map[string]any{"content": "## Ready"}}); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("requests = %+v", requests)
	}
	if requests[0]["_path"] != "/v2/groups/group-openid/messages" || requests[0]["content"] != "hello" || requests[0]["msg_type"] != float64(0) || requests[0]["msg_id"] != "evt-1" {
		t.Fatalf("first request = %+v", requests[0])
	}
	if requests[1]["_path"] != "/v2/users/user-openid/messages" || requests[1]["msg_type"] != float64(2) {
		t.Fatalf("second request = %+v", requests[1])
	}
}

func TestAPISender_RejectsBadInputsAndServerErrors(t *testing.T) {
	sender := &apiSender{
		apiBase:       "https://api.example.test",
		tokenProvider: staticTokenProvider("token-123"),
		client:        http.DefaultClient,
	}

	if err := sender.SendMessage(context.Background(), messageTarget{}, map[string]any{"msg_type": 2}); err == nil || !strings.Contains(err.Error(), "requires group_openid or user_openid") {
		t.Fatalf("empty target error = %v", err)
	}
	if err := sender.SendMessage(context.Background(), messageTarget{GroupOpenID: "group-openid"}, nil); err == nil || !strings.Contains(err.Error(), "payload is required") {
		t.Fatalf("nil payload error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream boom", http.StatusBadGateway)
	}))
	defer server.Close()
	sender = &apiSender{
		apiBase:       server.URL,
		tokenProvider: staticTokenProvider("token-123"),
		client:        server.Client(),
	}
	if err := sender.SendMessage(context.Background(), messageTarget{GroupOpenID: "group-openid"}, map[string]any{"msg_type": 2}); err == nil || !strings.Contains(err.Error(), "qqbot send failed: upstream boom") {
		t.Fatalf("server error = %v", err)
	}
}
