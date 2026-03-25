package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestStub_MetadataAndReplyContextDeclareTelegramBehavior(t *testing.T) {
	stub := NewStub("0")

	if stub.Name() != "telegram-stub" {
		t.Fatalf("Name = %q", stub.Name())
	}

	metadata := stub.Metadata()
	if metadata.Source != "telegram" {
		t.Fatalf("Source = %q", metadata.Source)
	}
	if !metadata.Capabilities.SupportsSlashCommands {
		t.Fatal("expected slash command capability")
	}
	if !metadata.Capabilities.SupportsMentions {
		t.Fatal("expected mention capability")
	}

	replyCtx := stub.ReplyContextFromTarget(&core.ReplyTarget{ChannelID: "2001"})
	msg, ok := replyCtx.(*core.Message)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyCtx)
	}
	if msg.ChatID != "2001" {
		t.Fatalf("ReplyContextFromTarget chatID = %q", msg.ChatID)
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
	if got.Platform != "telegram-stub" {
		t.Fatalf("Platform = %q", got.Platform)
	}
	if got.UserID != "telegram-user" || got.ChatID != "123456" {
		t.Fatalf("message = %+v", got)
	}
	if got.SessionKey != "telegram-stub:123456:telegram-user" {
		t.Fatalf("SessionKey = %q", got.SessionKey)
	}
	if got.ReplyTarget == nil || got.ReplyTarget.ChatID != "123456" {
		t.Fatalf("ReplyTarget = %+v", got.ReplyTarget)
	}
	if !got.ReplyTarget.UseReply {
		t.Fatalf("expected reply target to prefer reply: %+v", got.ReplyTarget)
	}
}

func TestStub_ReplyAndSendStoreReplies(t *testing.T) {
	stub := NewStub("0")

	if err := stub.Reply(context.Background(), &core.Message{ChatID: "1001"}, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := stub.Send(context.Background(), "1002", "send text"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if len(stub.replies) != 2 {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if stub.replies[0].ChatID != "1001" || stub.replies[0].Content != "reply text" {
		t.Fatalf("first reply = %+v", stub.replies[0])
	}
	if stub.replies[1].ChatID != "1002" || stub.replies[1].Content != "send text" {
		t.Fatalf("second reply = %+v", stub.replies[1])
	}
}

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
