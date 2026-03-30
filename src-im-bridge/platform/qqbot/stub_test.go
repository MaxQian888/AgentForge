package qqbot

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestStub_MetadataAndReplyContextDeclareQQBotBehavior(t *testing.T) {
	stub := NewStub("0")

	if stub.Name() != "qqbot-stub" {
		t.Fatalf("Name = %q", stub.Name())
	}

	metadata := stub.Metadata()
	if metadata.Source != "qqbot" {
		t.Fatalf("Source = %q", metadata.Source)
	}
	if !metadata.Capabilities.RequiresPublicCallback {
		t.Fatal("expected callback requirement")
	}

	replyCtx := stub.ReplyContextFromTarget(&core.ReplyTarget{ChatID: "group-openid", MessageID: "evt-1", UserID: "user-openid"})
	ctx, ok := replyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyCtx)
	}
	if ctx.ChatID != "group-openid" || ctx.MessageID != "evt-1" {
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
	if got.Platform != "qqbot-stub" {
		t.Fatalf("Platform = %q", got.Platform)
	}
	if got.UserID != "qqbot-user" || got.ChatID != "group-openid" {
		t.Fatalf("message = %+v", got)
	}
	if got.ReplyTarget == nil || got.ReplyTarget.ChatID != "group-openid" {
		t.Fatalf("ReplyTarget = %+v", got.ReplyTarget)
	}
}

func TestStub_ReplyAndSendStoreReplies(t *testing.T) {
	stub := NewStub("0")

	if err := stub.Reply(context.Background(), replyContext{ChatID: "group-openid", MessageID: "evt-1"}, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := stub.Send(context.Background(), "user:user-openid", "send text"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if len(stub.replies) != 2 {
		t.Fatalf("replies = %+v", stub.replies)
	}
}

func TestStub_LogsNativeReplies(t *testing.T) {
	stub := NewStub("0")

	message, err := core.NewQQBotMarkdownMessage(
		"## Review Ready",
		nil,
	)
	if err != nil {
		t.Fatalf("NewQQBotMarkdownMessage error: %v", err)
	}

	if err := stub.SendNative(context.Background(), "group:group-openid", message); err != nil {
		t.Fatalf("SendNative error: %v", err)
	}
	if len(stub.replies) != 1 || stub.replies[0].NativeSurface != core.NativeSurfaceQQBotMarkdown {
		t.Fatalf("replies = %+v", stub.replies)
	}
}

func TestStub_DeliverEnvelopeSupportsNativeAndStructured(t *testing.T) {
	stub := NewStub("0")

	native, err := core.NewQQBotMarkdownMessage("## Review Ready", nil)
	if err != nil {
		t.Fatalf("NewQQBotMarkdownMessage error: %v", err)
	}
	receipt, err := core.DeliverEnvelope(context.Background(), stub, stub.Metadata(), "group:group-openid", &core.DeliveryEnvelope{Native: native})
	if err != nil {
		t.Fatalf("DeliverEnvelope native error: %v", err)
	}
	if receipt.Type != "native" {
		t.Fatalf("native receipt = %+v", receipt)
	}

	receipt, err = core.DeliverEnvelope(context.Background(), stub, stub.Metadata(), "group:group-openid", &core.DeliveryEnvelope{
		Structured: &core.StructuredMessage{
			Sections: []core.StructuredSection{{
				Type: core.StructuredSectionTypeText,
				TextSection: &core.TextSection{
					Body: "Review Ready",
				},
			}},
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope structured error: %v", err)
	}
	if receipt.Type != "text" {
		t.Fatalf("structured receipt = %+v", receipt)
	}
	if len(stub.replies) != 2 || stub.replies[0].NativeSurface != core.NativeSurfaceQQBotMarkdown {
		t.Fatalf("replies = %+v", stub.replies)
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
	stub.replies = append(stub.replies, stubReply{ChatID: "group-openid", Content: "hello"})

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
