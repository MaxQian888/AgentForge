package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
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

func TestStub_FormattedTextAndUpdateMessageMethods(t *testing.T) {
	stub := NewStub("0")
	msg := &core.Message{ChatID: "chat-1"}

	if err := stub.SendFormattedText(context.Background(), "chat-2", &core.FormattedText{
		Content: "formatted send",
		Format:  core.TextFormatLarkMD,
	}); err != nil {
		t.Fatalf("SendFormattedText error: %v", err)
	}

	if err := stub.ReplyFormattedText(context.Background(), msg, &core.FormattedText{
		Content: "formatted reply",
		Format:  core.TextFormatLarkMD,
	}); err != nil {
		t.Fatalf("ReplyFormattedText error: %v", err)
	}

	if err := stub.UpdateFormattedText(context.Background(), msg, &core.FormattedText{
		Content: "formatted update",
		Format:  core.TextFormatLarkMD,
	}); err != nil {
		t.Fatalf("UpdateFormattedText error: %v", err)
	}

	if err := stub.UpdateMessage(context.Background(), msg, "plain update"); err != nil {
		t.Fatalf("UpdateMessage error: %v", err)
	}

	if len(stub.replies) != 4 {
		t.Fatalf("replies = %d, want 4", len(stub.replies))
	}
	if stub.replies[0].ChatID != "chat-2" || stub.replies[0].Content != "formatted send" {
		t.Fatalf("replies[0] = %+v", stub.replies[0])
	}
	if stub.replies[1].ChatID != "chat-1" || stub.replies[1].Content != "formatted reply" {
		t.Fatalf("replies[1] = %+v", stub.replies[1])
	}
	if stub.replies[2].ChatID != "chat-1" || stub.replies[2].Content != "formatted update" {
		t.Fatalf("replies[2] = %+v", stub.replies[2])
	}
	if stub.replies[3].ChatID != "chat-1" || stub.replies[3].Content != "plain update" {
		t.Fatalf("replies[3] = %+v", stub.replies[3])
	}
}

func TestStub_FormattedTextNilMessageReturnsError(t *testing.T) {
	stub := NewStub("0")

	if err := stub.SendFormattedText(context.Background(), "chat-1", nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("SendFormattedText(nil) err = %v", err)
	}
	if err := stub.ReplyFormattedText(context.Background(), &core.Message{ChatID: "chat-1"}, nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("ReplyFormattedText(nil) err = %v", err)
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

func TestFeishuStub_HelperBranches(t *testing.T) {
	stub := NewStub("0")

	if stub.Name() != "feishu-stub" {
		t.Fatalf("Name = %q", stub.Name())
	}
	metadata := stub.Metadata()
	if metadata.Source != "feishu" || !metadata.Capabilities.SupportsRichMessages {
		t.Fatalf("Metadata = %+v", metadata)
	}
	if stub.ReplyContextFromTarget(nil) != nil {
		t.Fatal("expected nil reply target to stay nil")
	}
	replyAny := stub.ReplyContextFromTarget(&core.ReplyTarget{ChannelID: "chat-1"})
	msg, ok := replyAny.(*core.Message)
	if !ok || msg.ChatID != "chat-1" {
		t.Fatalf("ReplyContextFromTarget = %#v", replyAny)
	}

	native, err := stub.BuildNativeTextMessage("AgentForge Update", "hello **world**")
	if err != nil {
		t.Fatalf("BuildNativeTextMessage error: %v", err)
	}
	if native == nil || native.FeishuCard == nil {
		t.Fatalf("native = %+v", native)
	}

	if err := stub.Send(context.Background(), "chat-2", "broadcast"); err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if err := stub.SendNative(context.Background(), "chat-3", native); err != nil {
		t.Fatalf("SendNative error: %v", err)
	}
	if err := stub.ReplyNative(context.Background(), &core.ReplyTarget{ChatID: "chat-4"}, native); err != nil {
		t.Fatalf("ReplyNative error: %v", err)
	}
	if err := stub.UpdateNative(context.Background(), &core.ReplyTarget{ChannelID: "chat-5"}, native); err != nil {
		t.Fatalf("UpdateNative error: %v", err)
	}

	if len(stub.replies) != 1 || stub.replies[0].ChatID != "chat-2" {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if len(stub.native) != 3 {
		t.Fatalf("native replies = %+v", stub.native)
	}
	if stub.native[0].ChatID != "chat-3" || stub.native[1].ChatID != "chat-4" || stub.native[2].ChatID != "chat-5" || !stub.native[2].Updated {
		t.Fatalf("native replies = %+v", stub.native)
	}
}
