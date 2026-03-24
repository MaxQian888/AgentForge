package slack

import (
	"context"
	"errors"
	"testing"

	"github.com/agentforge/im-bridge/core"
	goslack "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func TestLive_StartAcknowledgesSlashCommandBeforeDispatch(t *testing.T) {
	runner := &fakeSocketRunner{}
	messages := &fakeSlackMessageClient{}

	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(messages))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	var order []string
	var got *core.Message
	if err := live.Start(func(p core.Platform, msg *core.Message) {
		order = append(order, "handler")
		got = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	err = runner.dispatch(context.Background(), socketEnvelope{
		Type: socketEnvelopeSlashCommand,
		Ack: func(payload any) error {
			order = append(order, "ack")
			return nil
		},
		SlashCommand: &goslack.SlashCommand{
			Command:     "/task",
			Text:        "list",
			UserID:      "U123",
			UserName:    "alice",
			ChannelID:   "C456",
			ChannelName: "ops",
		},
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if len(order) != 2 || order[0] != "ack" || order[1] != "handler" {
		t.Fatalf("order = %v, want ack before handler", order)
	}
	if got == nil {
		t.Fatal("expected normalized slash command message")
	}
	if got.Platform != "slack" {
		t.Fatalf("Platform = %q, want slack", got.Platform)
	}
	if got.SessionKey != "slack:C456:U123" {
		t.Fatalf("SessionKey = %q", got.SessionKey)
	}
	if got.Content != "/task list" {
		t.Fatalf("Content = %q", got.Content)
	}
	replyCtx, ok := got.ReplyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyCtx type = %T, want replyContext", got.ReplyCtx)
	}
	if replyCtx.ChannelID != "C456" || replyCtx.ResponseURL != "" {
		t.Fatalf("ReplyCtx = %+v", replyCtx)
	}
	if got.ReplyTarget == nil || got.ReplyTarget.ChannelID != "C456" || !got.ReplyTarget.UseReply {
		t.Fatalf("ReplyTarget = %+v", got.ReplyTarget)
	}
}

func TestLive_StartNormalizesAppMentionEvent(t *testing.T) {
	runner := &fakeSocketRunner{}
	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(&fakeSlackMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	var got *core.Message
	if err := live.Start(func(p core.Platform, msg *core.Message) {
		got = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	err = runner.dispatch(context.Background(), socketEnvelope{
		Type: socketEnvelopeEventsAPI,
		Ack:  func(payload any) error { return nil },
		EventsAPI: &slackevents.EventsAPIEvent{
			Type: slackevents.CallbackEvent,
			InnerEvent: slackevents.EventsAPIInnerEvent{
				Type: "app_mention",
				Data: &slackevents.AppMentionEvent{
					Type:            "app_mention",
					User:            "U123",
					Text:            "<@U999> status",
					Channel:         "C456",
					TimeStamp:       "1700000000.123456",
					ThreadTimeStamp: "1700000000.100000",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if got == nil {
		t.Fatal("expected normalized app mention")
	}
	if got.Content != "@AgentForge status" {
		t.Fatalf("Content = %q, want @AgentForge status", got.Content)
	}
	replyCtx, ok := got.ReplyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyCtx type = %T, want replyContext", got.ReplyCtx)
	}
	if replyCtx.ThreadTS != "1700000000.100000" {
		t.Fatalf("ReplyCtx = %+v", replyCtx)
	}
	if got.ReplyTarget == nil || got.ReplyTarget.ChannelID != "C456" || got.ReplyTarget.ThreadID != "1700000000.100000" {
		t.Fatalf("ReplyTarget = %+v", got.ReplyTarget)
	}
}

func TestLive_ReplySendAndSendCardUseSlackMessageClient(t *testing.T) {
	runner := &fakeSocketRunner{}
	messages := &fakeSlackMessageClient{}

	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(messages))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	replyCtx := replyContext{ChannelID: "C111", ThreadTS: "1700000000.100000"}
	if err := live.Reply(context.Background(), replyCtx, "hello"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := live.Send(context.Background(), "C222", "broadcast"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	card := core.NewCard().
		SetTitle("Task Update").
		AddField("Status", "Done").
		AddPrimaryButton("Open", "link:https://example.test/task/1")
	if err := live.SendCard(context.Background(), "C333", card); err != nil {
		t.Fatalf("SendCard error: %v", err)
	}

	if len(messages.posts) != 3 {
		t.Fatalf("posts = %+v", messages.posts)
	}
	if messages.posts[0].ChannelID != "C111" || messages.posts[0].ThreadTS != "1700000000.100000" || messages.posts[0].Text != "hello" {
		t.Fatalf("reply post = %+v", messages.posts[0])
	}
	if messages.posts[1].ChannelID != "C222" || messages.posts[1].Text != "broadcast" {
		t.Fatalf("send post = %+v", messages.posts[1])
	}
	if len(messages.posts[2].Blocks) == 0 {
		t.Fatalf("card post = %+v", messages.posts[2])
	}
}

func TestLive_StopReturnsRunnerError(t *testing.T) {
	stopErr := errors.New("stop failed")
	runner := &fakeSocketRunner{stopErr: stopErr}

	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(&fakeSlackMessageClient{}))
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

func TestLive_MetadataDeclaresDeferredRichSlackCapabilities(t *testing.T) {
	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(&fakeSocketRunner{}), WithMessageClient(&fakeSlackMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	metadata := live.Metadata()
	if metadata.Source != "slack" {
		t.Fatalf("Source = %q, want slack", metadata.Source)
	}
	if !metadata.Capabilities.SupportsDeferredReply {
		t.Fatal("expected deferred reply capability")
	}
	if !metadata.Capabilities.SupportsRichMessages {
		t.Fatal("expected rich-message capability")
	}
}

type fakeSocketRunner struct {
	handler func(context.Context, socketEnvelope) error
	stopErr error
}

func (r *fakeSocketRunner) Start(ctx context.Context, handler func(context.Context, socketEnvelope) error) error {
	r.handler = handler
	return nil
}

func (r *fakeSocketRunner) Stop(context.Context) error {
	return r.stopErr
}

func (r *fakeSocketRunner) dispatch(ctx context.Context, envelope socketEnvelope) error {
	if r.handler == nil {
		return errors.New("handler not registered")
	}
	return r.handler(ctx, envelope)
}

type fakeSlackMessageClient struct {
	posts []slackOutgoingMessage
}

func (c *fakeSlackMessageClient) PostMessage(ctx context.Context, message slackOutgoingMessage) error {
	c.posts = append(c.posts, message)
	return nil
}
