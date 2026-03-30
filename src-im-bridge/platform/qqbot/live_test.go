package qqbot

import (
	"context"
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
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

func TestLive_SendStructuredFallsBackToRenderableText(t *testing.T) {
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
	if len(sender.calls) != 1 {
		t.Fatalf("sender calls = %+v", sender.calls)
	}
	if !strings.Contains(sender.calls[0].Content, "Review Ready") || !strings.Contains(sender.calls[0].Content, "Approve") {
		t.Fatalf("content = %q", sender.calls[0].Content)
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
