package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestStub_MapsInboundMessageAndAppliesDefaults(t *testing.T) {
	stub := NewStub("0")

	var got *core.Message
	if err := stub.Start(func(p core.Platform, msg *core.Message) {
		got = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer stub.Stop()

	req, err := http.NewRequest(http.MethodPost, "/test/message", bytes.NewBufferString(`{"content":"hello"}`))
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}

	rr := testRecorder{}
	stub.handleTestMessage(&rr, req)

	if got == nil {
		t.Fatal("expected handler to receive message")
	}
	if got.Platform != "feishu-stub" {
		t.Fatalf("Platform = %q, want feishu-stub", got.Platform)
	}
	if got.UserID != "test-user" || got.ChatID != "test-chat" {
		t.Fatalf("message = %+v", got)
	}
	if got.ReplyCtx != got {
		t.Fatal("expected reply context to point to message")
	}
}

func TestStub_ReplyAndCardMethodsStorePayloads(t *testing.T) {
	stub := NewStub("0")
	msg := &core.Message{ChatID: "chat-1"}

	if err := stub.Reply(context.Background(), msg, "hello"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := stub.SendCard(context.Background(), "chat-2", core.NewCard().SetTitle("send card")); err != nil {
		t.Fatalf("SendCard error: %v", err)
	}
	if err := stub.ReplyCard(context.Background(), msg, core.NewCard().SetTitle("reply card")); err != nil {
		t.Fatalf("ReplyCard error: %v", err)
	}

	if len(stub.replies) != 1 || stub.replies[0].ChatID != "chat-1" {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if len(stub.cards) != 2 {
		t.Fatalf("cards = %+v", stub.cards)
	}
	if stub.cards[0].ChatID != "chat-2" || stub.cards[1].ChatID != "chat-1" {
		t.Fatalf("cards = %+v", stub.cards)
	}
}

func TestStub_HTTPHandlersExposeAndClearRepliesAndCards(t *testing.T) {
	stub := NewStub("0")
	stub.replies = append(stub.replies, stubReply{ChatID: "chat-1", Content: "hello"})
	stub.cards = append(stub.cards, stubCardReply{ChatID: "chat-1", Card: core.NewCard().SetTitle("card title")})

	replyReq, err := http.NewRequest(http.MethodGet, "/test/replies", nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	replyRec := testRecorder{}
	stub.handleGetReplies(&replyRec, replyReq)
	if replyRec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content-type = %q", replyRec.Header().Get("Content-Type"))
	}

	var replies []stubReply
	if err := json.Unmarshal(replyRec.buf.Bytes(), &replies); err != nil {
		t.Fatalf("unmarshal replies: %v", err)
	}
	if len(replies) != 1 || replies[0].Content != "hello" {
		t.Fatalf("replies = %+v", replies)
	}

	cardReq, err := http.NewRequest(http.MethodGet, "/test/cards", nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	cardRec := testRecorder{}
	stub.handleGetCards(&cardRec, cardReq)

	var cards []stubCardReply
	if err := json.Unmarshal(cardRec.buf.Bytes(), &cards); err != nil {
		t.Fatalf("unmarshal cards: %v", err)
	}
	if len(cards) != 1 || cards[0].Card.Title != "card title" {
		t.Fatalf("cards = %+v", cards)
	}

	clearReq, err := http.NewRequest(http.MethodDelete, "/test/replies", nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	clearRec := testRecorder{}
	stub.handleClearReplies(&clearRec, clearReq)

	if len(stub.replies) != 0 || len(stub.cards) != 0 {
		t.Fatalf("replies = %+v, cards = %+v", stub.replies, stub.cards)
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
