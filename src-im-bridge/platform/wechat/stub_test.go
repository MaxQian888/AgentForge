package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestStub_Name(t *testing.T) {
	stub := NewStub("0")
	if stub.Name() != "wechat-stub" {
		t.Fatalf("Name = %q", stub.Name())
	}
}

func TestStub_Metadata(t *testing.T) {
	stub := NewStub("0")
	metadata := stub.Metadata()
	if metadata.Source != "wechat" {
		t.Fatalf("Source = %q", metadata.Source)
	}
	if metadata.Capabilities.ReadinessTier != core.ReadinessTierTextFirst {
		t.Fatalf("ReadinessTier = %q", metadata.Capabilities.ReadinessTier)
	}
	if metadata.Capabilities.CommandSurface != core.CommandSurfaceMixed {
		t.Fatalf("CommandSurface = %q", metadata.Capabilities.CommandSurface)
	}
}

func TestStub_StartStop(t *testing.T) {
	stub := NewStub("0")
	if err := stub.Start(func(p core.Platform, msg *core.Message) {}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if err := stub.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
}

func TestStub_MessageFlow(t *testing.T) {
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
	if got.Platform != "wechat" {
		t.Fatalf("Platform = %q", got.Platform)
	}
	if got.ChatID != "wechat-chat" || got.UserID != "wechat-user" {
		t.Fatalf("message = %+v", got)
	}
	if got.UserName != "WeChat User" {
		t.Fatalf("UserName = %q", got.UserName)
	}

	// Verify reply is recorded
	if err := stub.Reply(context.Background(), replyContext{OpenID: "wechat-user", ChatID: "wechat-chat"}, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if len(stub.replies) != 1 || stub.replies[0].Content != "reply text" {
		t.Fatalf("replies = %+v", stub.replies)
	}
}

func TestStub_ReplyContextFromTarget(t *testing.T) {
	stub := NewStub("0")

	if stub.ReplyContextFromTarget(nil) != nil {
		t.Fatal("expected nil for nil target")
	}

	replyCtx := stub.ReplyContextFromTarget(&core.ReplyTarget{ChatID: "chat-1", UserID: "user-1"})
	ctx, ok := replyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyCtx)
	}
	if ctx.ChatID != "chat-1" || ctx.OpenID != "user-1" {
		t.Fatalf("reply context = %+v", ctx)
	}
}

func TestStub_InvalidJSONReturnsBadRequest(t *testing.T) {
	stub := NewStub("0")

	req, err := http.NewRequest(http.MethodPost, "/test/message", bytes.NewBufferString("{"))
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}

	rr := testRecorder{}
	stub.handleTestMessage(&rr, req)

	if rr.code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.code, http.StatusBadRequest)
	}
}

func TestStub_HTTPHandlersExposeAndClearReplies(t *testing.T) {
	stub := NewStub("0")
	stub.replies = append(stub.replies, stubReply{ChatID: "chat-1", Content: "hello"})

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

func TestStub_FormattedTextSender(t *testing.T) {
	stub := NewStub("0")

	if err := stub.SendFormattedText(context.Background(), "chat-1", &core.FormattedText{
		Content: "hello",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("SendFormattedText error: %v", err)
	}
	if len(stub.replies) != 1 || stub.replies[0].Content != "hello" {
		t.Fatalf("replies = %+v", stub.replies)
	}

	// nil message returns error
	if err := stub.SendFormattedText(context.Background(), "chat-1", nil); err == nil {
		t.Fatal("expected error for nil formatted text")
	}
}

func TestChatIDFromReplyContext_ExtractsTargetFromVariousTypes(t *testing.T) {
	if got := chatIDFromReplyContext(replyContext{ChatID: "chat-1", OpenID: "user-1"}); got != "chat-1" {
		t.Fatalf("replyContext = %q", got)
	}
	if got := chatIDFromReplyContext(&replyContext{OpenID: "user-2"}); got != "user-2" {
		t.Fatalf("*replyContext = %q", got)
	}
	if got := chatIDFromReplyContext(&core.Message{ChatID: "chat-3"}); got != "chat-3" {
		t.Fatalf("*Message = %q", got)
	}
	if got := chatIDFromReplyContext(&core.ReplyTarget{ChannelID: "channel-4"}); got != "channel-4" {
		t.Fatalf("*ReplyTarget = %q", got)
	}
	if got := chatIDFromReplyContext("invalid"); got != "" {
		t.Fatalf("invalid = %q", got)
	}
	if got := chatIDFromReplyContext((*replyContext)(nil)); got != "" {
		t.Fatalf("nil *replyContext = %q", got)
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
