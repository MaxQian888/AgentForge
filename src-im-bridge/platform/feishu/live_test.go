package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/agentforge/im-bridge/core"
	larkcallback "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestLive_StartNormalizesInboundMessageFromLongConnection(t *testing.T) {
	runner := &fakeEventRunner{}
	sender := &fakeMessageClient{}

	live, err := NewLive("app-id", "app-secret", WithEventRunner(runner), WithMessageClient(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	var gotPlatform core.Platform
	var got *core.Message
	if err := live.Start(func(p core.Platform, msg *core.Message) {
		gotPlatform = p
		got = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	if !runner.started {
		t.Fatal("expected event runner to start")
	}

	err = runner.dispatch(context.Background(), newMessageReceiveEvent(
		"msg-1",
		"chat-1",
		"group",
		`{"text":"@_user_1 hello"}`,
		"1700000000123",
		"ou_user_1",
		[]*larkim.MentionEvent{
			newMentionEvent("@_user_1", "AgentForge"),
		},
	))
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if gotPlatform != live {
		t.Fatalf("platform = %#v, want live platform", gotPlatform)
	}
	if got == nil {
		t.Fatal("expected normalized message")
	}
	if got.Platform != "feishu" {
		t.Fatalf("Platform = %q, want feishu", got.Platform)
	}
	if got.SessionKey != "feishu:chat-1:ou_user_1" {
		t.Fatalf("SessionKey = %q", got.SessionKey)
	}
	if got.Content != "@AgentForge hello" {
		t.Fatalf("Content = %q, want @AgentForge hello", got.Content)
	}
	replyCtx, ok := got.ReplyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyCtx type = %T, want replyContext", got.ReplyCtx)
	}
	if replyCtx.MessageID != "msg-1" || replyCtx.ChatID != "chat-1" {
		t.Fatalf("ReplyCtx = %+v", replyCtx)
	}
	if got.ReplyTarget == nil || got.ReplyTarget.ChatID != "chat-1" || got.ReplyTarget.MessageID != "msg-1" {
		t.Fatalf("ReplyTarget = %+v", got.ReplyTarget)
	}
	if !got.IsGroup {
		t.Fatal("expected group message")
	}
}

func TestLive_ReplySendAndCardFallbackUseFeishuMessageAPI(t *testing.T) {
	runner := &fakeEventRunner{}
	sender := &fakeMessageClient{}

	live, err := NewLive("app-id", "app-secret", WithEventRunner(runner), WithMessageClient(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	replyCtx := replyContext{MessageID: "msg-1", ChatID: "chat-1"}
	if err := live.Reply(context.Background(), replyCtx, "hello"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := live.Send(context.Background(), "chat-2", "broadcast"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	card := core.NewCard().
		SetTitle("Task Update").
		AddField("Status", "Done").
		AddPrimaryButton("Open", "link:https://example.test/task/1")
	if err := live.SendCard(context.Background(), "chat-3", card); err != nil {
		t.Fatalf("SendCard error: %v", err)
	}
	if err := live.ReplyCard(context.Background(), replyCtx, core.NewCard().SetTitle("Reply Card")); err != nil {
		t.Fatalf("ReplyCard error: %v", err)
	}

	if len(sender.replyCalls) != 2 {
		t.Fatalf("replyCalls = %+v", sender.replyCalls)
	}
	if sender.replyCalls[0].MessageID != "msg-1" || sender.replyCalls[0].MsgType != larkim.MsgTypeText {
		t.Fatalf("first reply call = %+v", sender.replyCalls[0])
	}

	replyText := decodeJSONMap(t, sender.replyCalls[0].Content)
	if replyText["text"] != "hello" {
		t.Fatalf("reply text payload = %+v", replyText)
	}

	if len(sender.sendCalls) != 2 {
		t.Fatalf("sendCalls = %+v", sender.sendCalls)
	}
	if sender.sendCalls[0].ReceiveID != "chat-2" || sender.sendCalls[0].ReceiveIDType != larkim.ReceiveIdTypeChatId {
		t.Fatalf("first send call = %+v", sender.sendCalls[0])
	}
	sendText := decodeJSONMap(t, sender.sendCalls[0].Content)
	if sendText["text"] != "broadcast" {
		t.Fatalf("send text payload = %+v", sendText)
	}

	if sender.sendCalls[1].MsgType != larkim.MsgTypeInteractive {
		t.Fatalf("interactive send call = %+v", sender.sendCalls[1])
	}
	if sender.replyCalls[1].MsgType != larkim.MsgTypeInteractive {
		t.Fatalf("interactive reply call = %+v", sender.replyCalls[1])
	}

	sendCardPayload := decodeJSONMap(t, sender.sendCalls[1].Content)
	if sendCardPayload["config"] == nil || sendCardPayload["header"] == nil {
		t.Fatalf("interactive payload = %+v", sendCardPayload)
	}
}

func TestLive_HTTPCallbackHandlerRequiresExplicitRegistration(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	if live.HTTPCallbackHandler() != nil {
		t.Fatal("expected no HTTP callback handler by default")
	}

	live, err = NewLive(
		"app-id",
		"app-secret",
		WithEventRunner(&fakeEventRunner{}),
		WithMessageClient(&fakeMessageClient{}),
		WithLegacyCardCallbackHandler("verification-token", "encrypt-key", func(ctx context.Context, event *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error) {
			return &larkcallback.CardActionTriggerResponse{}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewLive with callback handler error: %v", err)
	}
	if live.HTTPCallbackHandler() == nil {
		t.Fatal("expected explicit HTTP callback handler")
	}
}

func TestLive_StopReturnsRunnerError(t *testing.T) {
	stopErr := errors.New("stop failed")
	runner := &fakeEventRunner{stopErr: stopErr}
	live, err := NewLive("app-id", "app-secret", WithEventRunner(runner), WithMessageClient(&fakeMessageClient{}))
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

func TestLive_MetadataDeclaresFeishuCapabilities(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	metadata := live.Metadata()
	if metadata.Source != "feishu" {
		t.Fatalf("Source = %q, want feishu", metadata.Source)
	}
	if !metadata.Capabilities.SupportsRichMessages {
		t.Fatal("expected rich-message capability")
	}
	if metadata.Capabilities.RequiresPublicCallback {
		t.Fatal("expected long-connection mode to avoid requiring a public callback by default")
	}
}

type fakeEventRunner struct {
	started bool
	stopped bool
	handler func(context.Context, *larkim.P2MessageReceiveV1) error
	stopErr error
}

func (r *fakeEventRunner) Start(ctx context.Context, handler func(context.Context, *larkim.P2MessageReceiveV1) error) error {
	r.started = true
	r.handler = handler
	return nil
}

func (r *fakeEventRunner) Stop(context.Context) error {
	r.stopped = true
	return r.stopErr
}

func (r *fakeEventRunner) dispatch(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if r.handler == nil {
		return errors.New("handler not registered")
	}
	return r.handler(ctx, event)
}

type fakeMessageClient struct {
	sendCalls  []fakeSendCall
	replyCalls []fakeReplyCall
}

type fakeSendCall struct {
	ReceiveIDType string
	ReceiveID     string
	MsgType       string
	Content       string
}

type fakeReplyCall struct {
	MessageID string
	MsgType   string
	Content   string
}

func (c *fakeMessageClient) Send(ctx context.Context, receiveIDType, receiveID, msgType, content string) error {
	c.sendCalls = append(c.sendCalls, fakeSendCall{
		ReceiveIDType: receiveIDType,
		ReceiveID:     receiveID,
		MsgType:       msgType,
		Content:       content,
	})
	return nil
}

func (c *fakeMessageClient) Reply(ctx context.Context, messageID, msgType, content string) error {
	c.replyCalls = append(c.replyCalls, fakeReplyCall{
		MessageID: messageID,
		MsgType:   msgType,
		Content:   content,
	})
	return nil
}

func newMessageReceiveEvent(messageID, chatID, chatType, content, createTime, senderOpenID string, mentions []*larkim.MentionEvent) *larkim.P2MessageReceiveV1 {
	return &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: stringPtr(senderOpenID),
				},
			},
			Message: &larkim.EventMessage{
				MessageId:   stringPtr(messageID),
				ChatId:      stringPtr(chatID),
				ChatType:    stringPtr(chatType),
				MessageType: stringPtr(larkim.MsgTypeText),
				Content:     stringPtr(content),
				CreateTime:  stringPtr(createTime),
				Mentions:    mentions,
			},
		},
	}
}

func newMentionEvent(key, name string) *larkim.MentionEvent {
	return &larkim.MentionEvent{
		Key:  stringPtr(key),
		Name: stringPtr(name),
	}
}

func decodeJSONMap(t *testing.T, raw string) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("unmarshal JSON payload %q: %v", raw, err)
	}
	return decoded
}

func stringPtr(value string) *string { return &value }

var _ http.Handler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
