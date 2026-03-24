package telegram

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentforge/im-bridge/core"
)

func TestLive_StartNormalizesSlashCommandUpdate(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	var gotPlatform core.Platform
	var gotMessage *core.Message
	if err := live.Start(func(p core.Platform, msg *core.Message) {
		gotPlatform = p
		gotMessage = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	err = runner.dispatch(context.Background(), update{
		Message: &message{
			MessageID: 42,
			Date:      time.Unix(1_700_000_000, 0).Unix(),
			Text:      "/task@agentforge_bot list",
			From: &user{
				ID:        1001,
				Username:  "alice",
				FirstName: "Alice",
			},
			Chat: &chat{
				ID:    -2001,
				Title: "Ops",
				Type:  "group",
			},
		},
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if gotPlatform != live {
		t.Fatalf("platform = %#v, want live platform", gotPlatform)
	}
	if gotMessage == nil {
		t.Fatal("expected normalized message")
	}
	if gotMessage.Platform != "telegram" {
		t.Fatalf("Platform = %q", gotMessage.Platform)
	}
	if gotMessage.SessionKey != "telegram:-2001:1001" {
		t.Fatalf("SessionKey = %q", gotMessage.SessionKey)
	}
	if gotMessage.Content != "/task list" {
		t.Fatalf("Content = %q", gotMessage.Content)
	}
	replyCtx, ok := gotMessage.ReplyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyCtx type = %T, want replyContext", gotMessage.ReplyCtx)
	}
	if replyCtx.ChatID != -2001 || replyCtx.MessageID != 42 {
		t.Fatalf("ReplyCtx = %+v", replyCtx)
	}
	if !gotMessage.IsGroup {
		t.Fatal("expected group update to be marked as group")
	}
}

func TestLive_ReplyAndSendUseTelegramSender(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	replyCtx := replyContext{ChatID: 12345, MessageID: 99}
	if err := live.Reply(context.Background(), replyCtx, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := live.Send(context.Background(), "-4001", "broadcast"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if len(sender.calls) != 2 {
		t.Fatalf("calls = %+v", sender.calls)
	}
	if sender.calls[0].ChatID != 12345 || sender.calls[0].ReplyToMessageID != 99 || sender.calls[0].Text != "reply text" {
		t.Fatalf("reply call = %+v", sender.calls[0])
	}
	if sender.calls[1].ChatID != -4001 || sender.calls[1].ReplyToMessageID != 0 || sender.calls[1].Text != "broadcast" {
		t.Fatalf("send call = %+v", sender.calls[1])
	}
}

func TestLive_MetadataDeclaresTelegramCapabilities(t *testing.T) {
	live, err := NewLive("bot-token", WithUpdateRunner(&fakeUpdateRunner{}), WithSender(&fakeSender{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	metadata := live.Metadata()
	if metadata.Source != "telegram" {
		t.Fatalf("Source = %q", metadata.Source)
	}
	if metadata.Capabilities.SupportsRichMessages {
		t.Fatal("expected telegram live transport to use text fallback for rich notifications")
	}
	if !metadata.Capabilities.SupportsSlashCommands {
		t.Fatal("expected slash command capability")
	}
	if !metadata.Capabilities.SupportsMentions {
		t.Fatal("expected mention capability")
	}
}

func TestNormalizeIncomingUpdateRejectsUnsupportedPayload(t *testing.T) {
	_, err := normalizeIncomingUpdate(update{})
	if err == nil {
		t.Fatal("expected update without message payload to fail")
	}
}

func TestNormalizeUpdateModeRejectsWebhookWhenLongPollingChosen(t *testing.T) {
	if err := validateUpdateMode("longpoll", "https://example.test/webhook"); err == nil {
		t.Fatal("expected long polling with webhook url to fail")
	}
}

func TestLive_StopReturnsRunnerError(t *testing.T) {
	stopErr := errors.New("stop failed")
	runner := &fakeUpdateRunner{stopErr: stopErr}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(&fakeSender{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	if err := live.Start(func(p core.Platform, msg *core.Message) {}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if err := live.Stop(); !errors.Is(err, stopErr) {
		t.Fatalf("Stop error = %v, want %v", err, stopErr)
	}
}

type fakeUpdateRunner struct {
	handler func(context.Context, update) error
	stopErr error
}

func (r *fakeUpdateRunner) Start(ctx context.Context, handler func(context.Context, update) error) error {
	r.handler = handler
	return nil
}

func (r *fakeUpdateRunner) Stop(context.Context) error {
	return r.stopErr
}

func (r *fakeUpdateRunner) dispatch(ctx context.Context, incoming update) error {
	if r.handler == nil {
		return errors.New("handler not registered")
	}
	return r.handler(ctx, incoming)
}

type sendCall struct {
	ChatID           int64
	ReplyToMessageID int
	Text             string
}

type fakeSender struct {
	calls []sendCall
}

func (s *fakeSender) SendText(ctx context.Context, chatID int64, replyToMessageID int, text string) error {
	s.calls = append(s.calls, sendCall{
		ChatID:           chatID,
		ReplyToMessageID: replyToMessageID,
		Text:             text,
	})
	return nil
}
