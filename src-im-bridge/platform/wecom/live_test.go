package wecom

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/im-bridge/core"
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
	if message.ReplyTarget.ChatID != "chat-1" {
		t.Fatalf("ChatID = %q", message.ReplyTarget.ChatID)
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
	calls []responseReplyCall
	err   error
}

type responseReplyCall struct {
	ResponseURL string
	Content     string
}

func (f *fakeResponseReplier) ReplyText(ctx context.Context, responseURL string, content string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, responseReplyCall{ResponseURL: responseURL, Content: content})
	return nil
}

type fakeDirectSender struct {
	calls []directSendCall
	err   error
}

type directSendCall struct {
	Target  directSendTarget
	Content string
}

func (f *fakeDirectSender) SendText(ctx context.Context, target directSendTarget, content string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, directSendCall{Target: target, Content: content})
	return nil
}

type staticTokenProvider string

func (s staticTokenProvider) AccessToken(ctx context.Context) (string, error) {
	if strings.TrimSpace(string(s)) == "" {
		return "", errors.New("missing token")
	}
	return string(s), nil
}
