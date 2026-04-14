package qq

import (
	"context"
	"encoding/json"
	"github.com/agentforge/im-bridge/core"
	"strings"
	"testing"
	"time"
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
	if message.ReplyTarget.ConversationID != "2002" {
		t.Fatalf("ConversationID = %q", message.ReplyTarget.ConversationID)
	}
	if message.ReplyTarget.ProgressMode != string(core.AsyncUpdateReply) {
		t.Fatalf("ProgressMode = %q", message.ReplyTarget.ProgressMode)
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

type trackingTransport struct {
	started bool
	stopped bool
}

func (t *trackingTransport) Start(ctx context.Context, handler func(context.Context, incomingEvent) error) error {
	t.started = true
	return nil
}

func (t *trackingTransport) Stop(ctx context.Context) error {
	t.stopped = true
	return nil
}

func (t *trackingTransport) SendAction(ctx context.Context, action string, params map[string]any) error {
	return nil
}

func TestLive_NameReplyContextAndLifecycle(t *testing.T) {
	transport := &trackingTransport{}
	live, err := NewLive(
		"ws://127.0.0.1:3001/onebot/v11/ws",
		"qq-token",
		WithTransport(transport),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if live.Name() != "qq-live" {
		t.Fatalf("Name = %q", live.Name())
	}
	if live.ReplyContextFromTarget(nil) != nil {
		t.Fatal("expected nil reply target to stay nil")
	}

	replyAny := live.ReplyContextFromTarget(&core.ReplyTarget{
		ChannelID: "group-1",
		UserID:    "user-1",
		MessageID: "msg-1",
	})
	reply, ok := replyAny.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyAny)
	}
	if reply.ChatID != "group-1" || reply.UserID != "user-1" || reply.MessageID != "msg-1" || !reply.IsGroup {
		t.Fatalf("reply = %+v", reply)
	}

	if err := live.Start(nil); err == nil {
		t.Fatal("expected nil handler to fail")
	}
	if err := live.Start(func(p core.Platform, msg *core.Message) {}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if !transport.started {
		t.Fatal("expected transport Start to be called")
	}

	if err := live.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
	if !transport.stopped {
		t.Fatal("expected transport Stop to be called")
	}
}

func TestLive_SendFormattedTextDelegatesToPlainTextSend(t *testing.T) {
	transport := &fakeTransport{}
	live, err := NewLive(
		"ws://127.0.0.1:3001/onebot/v11/ws",
		"qq-token",
		WithTransport(transport),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.SendFormattedText(context.Background(), "group:2002", &core.FormattedText{
		Content: "hello formatted",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("SendFormattedText error: %v", err)
	}
	if len(transport.calls) != 1 {
		t.Fatalf("calls = %+v", transport.calls)
	}
	message, _ := transport.calls[0].Params["message"].(string)
	if message != "hello formatted" {
		t.Fatalf("message = %q", message)
	}

	if err := live.SendFormattedText(context.Background(), "group:2002", nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("nil message error = %v", err)
	}
}

func TestLive_ReplyFormattedTextDelegatesToPlainTextReply(t *testing.T) {
	transport := &fakeTransport{}
	live, err := NewLive(
		"ws://127.0.0.1:3001/onebot/v11/ws",
		"qq-token",
		WithTransport(transport),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.ReplyFormattedText(context.Background(), replyContext{ChatID: "2002", MessageID: "1001", IsGroup: true}, &core.FormattedText{
		Content: "reply formatted",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("ReplyFormattedText error: %v", err)
	}
	if len(transport.calls) != 1 {
		t.Fatalf("calls = %+v", transport.calls)
	}
	message, _ := transport.calls[0].Params["message"].(string)
	if !strings.Contains(message, "reply formatted") {
		t.Fatalf("message = %q", message)
	}

	if err := live.ReplyFormattedText(context.Background(), replyContext{ChatID: "2002"}, nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("nil message error = %v", err)
	}
}

func TestLive_UpdateFormattedTextDelegatesToReply(t *testing.T) {
	transport := &fakeTransport{}
	live, err := NewLive(
		"ws://127.0.0.1:3001/onebot/v11/ws",
		"qq-token",
		WithTransport(transport),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.UpdateFormattedText(context.Background(), replyContext{ChatID: "2002", IsGroup: true}, &core.FormattedText{
		Content: "update formatted",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("UpdateFormattedText error: %v", err)
	}
	if len(transport.calls) != 1 {
		t.Fatalf("calls = %+v", transport.calls)
	}
}

func TestQQLiveHelpers_ParseTargetsPayloadAndTime(t *testing.T) {
	if got := parseTarget(""); got.action != "" || got.id != "" {
		t.Fatalf("parseTarget(empty) = %+v", got)
	}
	if got := parseTarget("user:3003"); got.action != "send_private_msg" || got.paramName != "user_id" || got.id != "3003" {
		t.Fatalf("parseTarget(user) = %+v", got)
	}
	if got := parseTarget("group:2002"); got.action != "send_group_msg" || got.paramName != "group_id" || got.id != "2002" {
		t.Fatalf("parseTarget(group) = %+v", got)
	}
	if got := parseTarget("2002"); got.action != "send_group_msg" || got.id != "2002" {
		t.Fatalf("parseTarget(default) = %+v", got)
	}

	if got := messageTargetFromReply(replyContext{ChatID: "2002", MessageID: "msg-1", IsGroup: true}); got.action != "send_group_msg" || got.id != "2002" {
		t.Fatalf("messageTargetFromReply(group) = %+v", got)
	}
	if got := messageTargetFromReply(replyContext{UserID: "3003", MessageID: "msg-2"}); got.action != "send_private_msg" || got.id != "3003" {
		t.Fatalf("messageTargetFromReply(user) = %+v", got)
	}
	if got := messageTargetFromReply(replyContext{}); got.action != "" || got.id != "" {
		t.Fatalf("messageTargetFromReply(empty) = %+v", got)
	}

	if got := renderMessagePayload(json.RawMessage(`"plain text"`)); got != "plain text" {
		t.Fatalf("renderMessagePayload(string) = %q", got)
	}
	if got := renderMessagePayload(json.RawMessage(`[{"type":"text","data":{"text":"hello"}},{"type":"image","data":{"url":"skip"}},{"type":"text","data":{"text":" world"}}]`)); got != "hello world" {
		t.Fatalf("renderMessagePayload(segments) = %q", got)
	}
	if got := renderMessagePayload(json.RawMessage(`{"bad":true}`)); got != "" {
		t.Fatalf("renderMessagePayload(invalid) = %q", got)
	}

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
