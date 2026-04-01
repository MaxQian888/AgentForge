package qq

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/agentforge/im-bridge/core"
	"net/http"
	"strings"
	"testing"
)

func TestStub_MetadataAndReplyContextDeclareQQBehavior(t *testing.T) {
	stub := NewStub("0")

	if stub.Name() != "qq-stub" {
		t.Fatalf("Name = %q", stub.Name())
	}

	metadata := stub.Metadata()
	if metadata.Source != "qq" {
		t.Fatalf("Source = %q", metadata.Source)
	}
	if !metadata.Capabilities.SupportsSlashCommands {
		t.Fatal("expected slash command capability")
	}

	replyCtx := stub.ReplyContextFromTarget(&core.ReplyTarget{ChatID: "2001", MessageID: "99"})
	ctx, ok := replyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyCtx)
	}
	if ctx.ChatID != "2001" || ctx.MessageID != "99" {
		t.Fatalf("reply context = %+v", ctx)
	}
}

func TestStub_MapsInboundMessageAndAppliesDefaults(t *testing.T) {
	stub := NewStub("0")

	var got *core.Message
	if err := stub.Start(func(p core.Platform, msg *core.Message) {
		got = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer stub.Stop()

	req, err := http.NewRequest(http.MethodPost, "/test/message", bytes.NewBufferString(`{"content":"/task list"}`))
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}

	rr := testRecorder{}
	stub.handleTestMessage(&rr, req)

	if got == nil {
		t.Fatal("expected handler to receive message")
	}
	if got.Platform != "qq-stub" {
		t.Fatalf("Platform = %q", got.Platform)
	}
	if got.UserID != "qq-user" || got.ChatID != "10001" {
		t.Fatalf("message = %+v", got)
	}
	if got.ReplyTarget == nil || got.ReplyTarget.ChatID != "10001" {
		t.Fatalf("ReplyTarget = %+v", got.ReplyTarget)
	}
}

func TestStub_ReplyAndSendStoreReplies(t *testing.T) {
	stub := NewStub("0")

	if err := stub.Reply(context.Background(), replyContext{ChatID: "1001"}, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := stub.Send(context.Background(), "group:1002", "send text"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if len(stub.replies) != 2 {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if stub.replies[0].ChatID != "1001" || stub.replies[0].Content != "reply text" {
		t.Fatalf("first reply = %+v", stub.replies[0])
	}
	if stub.replies[1].ChatID != "group:1002" || stub.replies[1].Content != "send text" {
		t.Fatalf("second reply = %+v", stub.replies[1])
	}
}

type testRecorder struct {
	header http.Header
	code   int
	buf    bytes.Buffer
}

func (r *testRecorder) Header() http.Header {
	if r.header == nil {
		r.header = make(http.Header)
	}
	return r.header
}

func (r *testRecorder) Write(data []byte) (int, error) { return r.buf.Write(data) }
func (r *testRecorder) WriteHeader(statusCode int)     { r.code = statusCode }

func TestStub_HTTPHandlersExposeAndClearReplies(t *testing.T) {
	stub := NewStub("0")
	stub.replies = append(stub.replies, stubReply{ChatID: "1001", Content: "hello"})

	getReq, err := http.NewRequest(http.MethodGet, "/test/replies", nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	getRec := testRecorder{}
	stub.handleGetReplies(&getRec, getReq)

	var replies []stubReply
	if err := json.Unmarshal(getRec.buf.Bytes(), &replies); err != nil {
		t.Fatalf("unmarshal replies: %v", err)
	}
	if len(replies) != 1 || replies[0].Content != "hello" {
		t.Fatalf("replies = %+v", replies)
	}

	clearReq, err := http.NewRequest(http.MethodDelete, "/test/replies", nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	clearRec := testRecorder{}
	stub.handleClearReplies(&clearRec, clearReq)

	if len(stub.replies) != 0 {
		t.Fatalf("replies = %+v", stub.replies)
	}
}

func TestStub_FormattedTextSendAndReplyStorePlainText(t *testing.T) {
	stub := NewStub("0")

	if err := stub.SendFormattedText(context.Background(), "group:1002", &core.FormattedText{
		Content: "formatted send",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("SendFormattedText error: %v", err)
	}
	if err := stub.ReplyFormattedText(context.Background(), replyContext{ChatID: "1001"}, &core.FormattedText{
		Content: "formatted reply",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("ReplyFormattedText error: %v", err)
	}
	if err := stub.UpdateFormattedText(context.Background(), replyContext{ChatID: "1001"}, &core.FormattedText{
		Content: "formatted update",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("UpdateFormattedText error: %v", err)
	}

	if len(stub.replies) != 3 {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if stub.replies[0].Content != "formatted send" {
		t.Fatalf("first reply = %+v", stub.replies[0])
	}
	if stub.replies[1].Content != "formatted reply" {
		t.Fatalf("second reply = %+v", stub.replies[1])
	}
	if stub.replies[2].Content != "formatted update" {
		t.Fatalf("third reply = %+v", stub.replies[2])
	}

	if err := stub.SendFormattedText(context.Background(), "group:1002", nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("nil message error = %v", err)
	}
	if err := stub.ReplyFormattedText(context.Background(), replyContext{ChatID: "1001"}, nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("nil reply message error = %v", err)
	}
}

func TestStub_SendStructuredAndHelperConversions(t *testing.T) {
	stub := NewStub("0")

	if err := stub.SendStructured(context.Background(), "group:2002", &core.StructuredMessage{
		Title: "Review Ready",
		Body:  "Choose the next step.",
		Actions: []core.StructuredAction{
			{ID: "act:approve:review-1", Label: "Approve"},
		},
	}); err != nil {
		t.Fatalf("SendStructured error: %v", err)
	}
	if len(stub.replies) != 1 || !strings.Contains(stub.replies[0].Content, "Review Ready") {
		t.Fatalf("replies = %+v", stub.replies)
	}

	raw := replyContext{ChatID: "2002", UserID: "3003", MessageID: "msg-1", IsGroup: true}
	if got := toReplyContext(raw); got != raw {
		t.Fatalf("toReplyContext(raw) = %+v", got)
	}
	if got := toReplyContext(&replyContext{ChatID: "2002", UserID: "3003", MessageID: "msg-2", IsGroup: true}); got.MessageID != "msg-2" {
		t.Fatalf("toReplyContext(pointer) = %+v", got)
	}

	msg := &core.Message{
		ChatID:      "2002",
		UserID:      "3003",
		IsGroup:     true,
		ReplyTarget: &core.ReplyTarget{MessageID: "msg-3"},
		ReplyCtx:    nil,
	}
	if got := toReplyContext(msg); got.ChatID != "2002" || got.UserID != "3003" || got.MessageID != "msg-3" || !got.IsGroup {
		t.Fatalf("toReplyContext(message) = %+v", got)
	}

	target := &core.ReplyTarget{ConversationID: "group-77", UserID: "user-1", MessageID: "msg-4"}
	if got := toReplyContext(target); got.ChatID != "group-77" || got.MessageID != "msg-4" || !got.IsGroup {
		t.Fatalf("toReplyContext(target) = %+v", got)
	}

	if got := toReplyContext("invalid"); got != (replyContext{}) {
		t.Fatalf("toReplyContext(invalid) = %+v", got)
	}
	if got := messageIDFromTarget(nil); got != "" {
		t.Fatalf("messageIDFromTarget(nil) = %q", got)
	}
	if got := messageIDFromTarget(&core.ReplyTarget{MessageID: " 99 "}); got != "99" {
		t.Fatalf("messageIDFromTarget(target) = %q", got)
	}
}
