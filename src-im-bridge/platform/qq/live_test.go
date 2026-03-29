package qq

import (
	"context"
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestNewLive_RequiresWSURL(t *testing.T) {
	if _, err := NewLive("", "token"); err == nil {
		t.Fatal("expected missing ws url to fail")
	}
}

func TestLive_NormalizeMessageEventPreservesReplyTargetContext(t *testing.T) {
	message, err := normalizeIncomingEvent(incomingEvent{
		PostType:    "message",
		MessageType: "group",
		MessageID:   1001,
		GroupID:     2002,
		UserID:      3003,
		RawMessage:  "/help",
		Sender: senderInfo{
			Nickname: "QQ User",
		},
	})
	if err != nil {
		t.Fatalf("normalizeIncomingEvent error: %v", err)
	}
	if message.Platform != "qq" {
		t.Fatalf("Platform = %q", message.Platform)
	}
	if message.ReplyTarget == nil || message.ReplyTarget.ChatID != "2002" || message.ReplyTarget.MessageID != "1001" {
		t.Fatalf("ReplyTarget = %+v", message.ReplyTarget)
	}
}

func TestLive_ReplyAndSendDispatchOneBotActions(t *testing.T) {
	transport := &fakeTransport{}
	live, err := NewLive(
		"ws://127.0.0.1:3001/onebot/v11/ws",
		"qq-token",
		WithTransport(transport),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.Reply(context.Background(), replyContext{ChatID: "2002", MessageID: "1001"}, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := live.Send(context.Background(), "user:3003", "send text"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if len(transport.calls) != 2 {
		t.Fatalf("calls = %+v", transport.calls)
	}
	if transport.calls[0].Action != "send_group_msg" {
		t.Fatalf("first action = %+v", transport.calls[0])
	}
	if transport.calls[1].Action != "send_private_msg" {
		t.Fatalf("second action = %+v", transport.calls[1])
	}
}

func TestLive_MetadataDeclaresQQCapabilities(t *testing.T) {
	live, err := NewLive(
		"ws://127.0.0.1:3001/onebot/v11/ws",
		"qq-token",
		WithTransport(&fakeTransport{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	metadata := live.Metadata()
	if metadata.Source != "qq" {
		t.Fatalf("source = %q", metadata.Source)
	}
	if metadata.Capabilities.RequiresPublicCallback {
		t.Fatal("expected qq live transport to avoid public callback requirement")
	}
	if !metadata.Capabilities.SupportsSlashCommands {
		t.Fatal("expected qq slash-style commands")
	}
}

type fakeTransport struct {
	calls []transportCall
}

type transportCall struct {
	Action string
	Params map[string]any
}

func (f *fakeTransport) Start(ctx context.Context, handler func(context.Context, incomingEvent) error) error {
	return nil
}

func (f *fakeTransport) Stop(ctx context.Context) error {
	return nil
}

func (f *fakeTransport) SendAction(ctx context.Context, action string, params map[string]any) error {
	cloned := make(map[string]any, len(params))
	for key, value := range params {
		cloned[key] = value
	}
	f.calls = append(f.calls, transportCall{Action: action, Params: cloned})
	return nil
}

func TestLive_SendStructuredFallsBackToRenderableText(t *testing.T) {
	transport := &fakeTransport{}
	live, err := NewLive(
		"ws://127.0.0.1:3001/onebot/v11/ws",
		"qq-token",
		WithTransport(transport),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	err = live.SendStructured(context.Background(), "group:2002", &core.StructuredMessage{
		Title: "Review Ready",
		Body:  "Choose the next step.",
		Actions: []core.StructuredAction{
			{ID: "act:approve:review-1", Label: "Approve", Style: core.ActionStylePrimary},
		},
	})
	if err != nil {
		t.Fatalf("SendStructured error: %v", err)
	}
	if len(transport.calls) != 1 {
		t.Fatalf("calls = %+v", transport.calls)
	}
	message, _ := transport.calls[0].Params["message"].(string)
	if !strings.Contains(message, "Review Ready") || !strings.Contains(message, "Approve") {
		t.Fatalf("message = %q", message)
	}
}
