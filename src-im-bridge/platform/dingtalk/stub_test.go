package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestStub_MapsMentionMessageToCoreMessage(t *testing.T) {
	stub := NewStub("0")

	var got *core.Message
	if err := stub.Start(func(p core.Platform, msg *core.Message) {
		got = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer stub.Stop()

	payload := map[string]any{
		"content":   "@AgentForge status",
		"user_id":   "u-2",
		"user_name": "DingTalk User",
		"chat_id":   "chat-2",
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
	if got.Platform != "dingtalk-stub" {
		t.Fatalf("Platform = %q, want dingtalk-stub", got.Platform)
	}
	if got.SessionKey != "dingtalk-stub:chat-2:u-2" {
		t.Fatalf("SessionKey = %q", got.SessionKey)
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
