package notify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentforge/im-bridge/core"
)

// testPlatform is the minimal Platform stub for attachment/reaction tests.
type testPlatform struct{ name string }

func (p *testPlatform) Name() string                                           { return p.name }
func (p *testPlatform) Start(_ core.MessageHandler) error                      { return nil }
func (p *testPlatform) Reply(_ context.Context, _ any, _ string) error         { return nil }
func (p *testPlatform) Send(_ context.Context, _ string, _ string) error       { return nil }
func (p *testPlatform) Stop() error                                            { return nil }

func newAttachmentTestReceiver(t *testing.T) *Receiver {
	t.Helper()
	r := NewReceiver(&testPlatform{name: "slack"}, "0")
	store, err := NewStagingStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStagingStore: %v", err)
	}
	r.SetStagingStore(store)
	return r
}

func TestReceiver_HandleUploadAttachment_MultipartReturnsStagedID(t *testing.T) {
	r := newAttachmentTestReceiver(t)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "fix.patch")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := io.Copy(fw, strings.NewReader("diff --git a b")); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if err := mw.WriteField("kind", "patch"); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/im/attachments", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.handleUploadAttachment(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["kind"] != "patch" {
		t.Fatalf("kind = %v", resp["kind"])
	}
	if id, _ := resp["id"].(string); id == "" {
		t.Fatalf("id missing: %v", resp)
	}
}

func TestReceiver_ResolveAttachments_StageBase64Payload(t *testing.T) {
	r := newAttachmentTestReceiver(t)
	r.auditSalt = "testsalt"

	data := base64.StdEncoding.EncodeToString([]byte("hello"))
	got, err := r.resolveAttachments([]AttachmentRequest{{
		Kind:       "report",
		Filename:   "x.md",
		DataBase64: data,
	}})
	if err != nil {
		t.Fatalf("resolveAttachments: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d attachments", len(got))
	}
	if got[0].Kind != core.AttachmentKindReport {
		t.Fatalf("kind = %q", got[0].Kind)
	}
	if got[0].ContentRef == "" {
		t.Fatalf("content ref missing: %+v", got[0])
	}
	if got[0].SizeBytes != 5 {
		t.Fatalf("size = %d, want 5", got[0].SizeBytes)
	}
}

type capturingReactionSink struct {
	mu     sync.Mutex
	events []ReactionEvent
}

func (c *capturingReactionSink) RecordReaction(_ context.Context, event ReactionEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
	return nil
}

func TestReceiver_DispatchReaction_ForwardsToSink(t *testing.T) {
	r := NewReceiver(&testPlatform{name: "slack"}, "0")
	sink := &capturingReactionSink{}
	r.SetReactionSink(sink)

	event := ReactionEvent{
		Platform:  "slack",
		ChatID:    "C1",
		MessageID: "M1",
		UserID:    "U1",
		EmojiCode: core.ReactionDone,
		RawEmoji:  "white_check_mark",
		ReactedAt: time.Now().UTC(),
	}
	if err := r.dispatchReaction(context.Background(), event); err != nil {
		t.Fatalf("dispatchReaction: %v", err)
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.events) != 1 {
		t.Fatalf("events = %d, want 1", len(sink.events))
	}
	if sink.events[0].EmojiCode != core.ReactionDone {
		t.Fatalf("emoji code = %q", sink.events[0].EmojiCode)
	}
}
