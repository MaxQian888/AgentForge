package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/agentforge/im-bridge/core"
)

func TestStub_MapsInboundMessageToCoreMessage(t *testing.T) {
	stub := NewStub("0")

	var got *core.Message
	if err := stub.Start(func(p core.Platform, msg *core.Message) {
		got = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer stub.Stop()

	payload := map[string]any{
		"content":   "/task list",
		"user_id":   "u-1",
		"user_name": "Slack User",
		"chat_id":   "c-1",
		"is_group":  true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "/test/message", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}

	rr := testRecorder{}
	stub.handleTestMessage(&rr, req)

	if got == nil {
		t.Fatal("expected handler to receive message")
	}
	if got.Platform != "slack-stub" {
		t.Fatalf("Platform = %q, want slack-stub", got.Platform)
	}
	if got.SessionKey != "slack-stub:c-1:u-1" {
		t.Fatalf("SessionKey = %q", got.SessionKey)
	}
	if got.Content != "/task list" {
		t.Fatalf("Content = %q", got.Content)
	}
}

func TestStub_SendStoresReply(t *testing.T) {
	stub := NewStub("0")
	if err := stub.Send(context.Background(), "chat-1", "hello"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if len(stub.replies) != 1 || stub.replies[0].Content != "hello" {
		t.Fatalf("replies = %+v", stub.replies)
	}
}

func TestStub_ReplyUsesMessageChatID(t *testing.T) {
	stub := NewStub("0")

	if err := stub.Reply(context.Background(), &core.Message{ChatID: "chat-2"}, "hello"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if len(stub.replies) != 1 || stub.replies[0].ChatID != "chat-2" {
		t.Fatalf("replies = %+v", stub.replies)
	}
}

func TestStub_LogsNativeAndFormattedReplies(t *testing.T) {
	stub := NewStub("0")

	message, err := core.NewSlackBlockKitMessage([]map[string]any{
		{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": "*Build* ready",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewSlackBlockKitMessage error: %v", err)
	}

	if err := stub.SendNative(context.Background(), "chat-1", message); err != nil {
		t.Fatalf("SendNative error: %v", err)
	}
	if err := stub.SendFormattedText(context.Background(), "chat-1", &core.FormattedText{
		Content: "*bold*",
		Format:  core.TextFormatSlackMrkdwn,
	}); err != nil {
		t.Fatalf("SendFormattedText error: %v", err)
	}

	if len(stub.replies) != 2 {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if stub.replies[0].NativeSurface != core.NativeSurfaceSlackBlockKit {
		t.Fatalf("native reply = %+v", stub.replies[0])
	}
	if stub.replies[1].Format != string(core.TextFormatSlackMrkdwn) {
		t.Fatalf("formatted reply = %+v", stub.replies[1])
	}
}

func TestStub_DeliverEnvelopeSupportsNativeStructuredAndFormatted(t *testing.T) {
	stub := NewStub("0")

	native, err := core.NewSlackBlockKitMessage([]map[string]any{
		{
			"type": "section",
			"text": map[string]any{"type": "mrkdwn", "text": "*Build* ready"},
		},
	})
	if err != nil {
		t.Fatalf("NewSlackBlockKitMessage error: %v", err)
	}
	receipt, err := core.DeliverEnvelope(context.Background(), stub, stub.Metadata(), "chat-1", &core.DeliveryEnvelope{Native: native})
	if err != nil {
		t.Fatalf("DeliverEnvelope native error: %v", err)
	}
	if receipt.Type != "native" {
		t.Fatalf("native receipt = %+v", receipt)
	}

	receipt, err = core.DeliverEnvelope(context.Background(), stub, stub.Metadata(), "chat-1", &core.DeliveryEnvelope{
		Structured: &core.StructuredMessage{
			Sections: []core.StructuredSection{{
				Type: core.StructuredSectionTypeText,
				TextSection: &core.TextSection{
					Body: "Build ready",
				},
			}},
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope structured error: %v", err)
	}
	if receipt.Type != "structured" {
		t.Fatalf("structured receipt = %+v", receipt)
	}

	receipt, err = core.DeliverEnvelope(context.Background(), stub, stub.Metadata(), "chat-1", &core.DeliveryEnvelope{
		Content: "build *status*",
		Metadata: map[string]string{
			"text_format": string(core.TextFormatSlackMrkdwn),
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope formatted error: %v", err)
	}
	if receipt.Type != "text" {
		t.Fatalf("formatted receipt = %+v", receipt)
	}
	if len(stub.replies) != 3 {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if stub.replies[0].NativeSurface != core.NativeSurfaceSlackBlockKit || stub.replies[2].Format != string(core.TextFormatSlackMrkdwn) {
		t.Fatalf("replies = %+v", stub.replies)
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

var _ = time.Now
