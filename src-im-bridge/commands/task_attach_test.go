package commands

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

type fakeReplyPlatform struct {
	mu      sync.Mutex
	name    string
	replies []string
}

func (f *fakeReplyPlatform) Name() string                            { return f.name }
func (f *fakeReplyPlatform) Start(_ core.MessageHandler) error       { return nil }
func (f *fakeReplyPlatform) Stop() error                             { return nil }
func (f *fakeReplyPlatform) Send(_ context.Context, _, _ string) error { return nil }
func (f *fakeReplyPlatform) Reply(_ context.Context, _ any, content string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies = append(f.replies, content)
	return nil
}

func TestRegisterTaskCommands_AttachPostsToBackend(t *testing.T) {
	var captured struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.path = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		if len(body) > 0 {
			_ = json.Unmarshal(body, &captured.body)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret").WithSource("slack-live")

	engine := core.NewEngine(nil)
	RegisterTaskCommands(engine, apiClient)

	p := &fakeReplyPlatform{name: "slack"}
	msg := &core.Message{
		Platform:   "slack",
		ChatID:     "C1",
		UserID:     "U1",
		Content:    "/task attach task-123 staged-abc logs",
	}
	engine.HandleMessage(p, msg)

	if !strings.HasPrefix(captured.path, "/api/v1/tasks/") {
		t.Fatalf("unexpected path: %q", captured.path)
	}
	if captured.body["staged_id"] != "staged-abc" {
		t.Fatalf("staged_id = %v", captured.body["staged_id"])
	}
	if captured.body["kind"] != "logs" {
		t.Fatalf("kind = %v", captured.body["kind"])
	}
	if len(p.replies) == 0 || !strings.Contains(p.replies[len(p.replies)-1], "已为任务") {
		t.Fatalf("replies = %v", p.replies)
	}
}

func TestRegisterReviewCommands_ApproveReactionBindsShortcut(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/im/reactions/shortcuts" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret").WithSource("slack-live")

	engine := core.NewEngine(nil)
	RegisterReviewCommands(engine, apiClient)

	p := &fakeReplyPlatform{name: "slack"}
	msg := &core.Message{
		Platform: "slack",
		ChatID:   "C1",
		UserID:   "U1",
		Content:  "/review approve-reaction rev-9",
	}
	engine.HandleMessage(p, msg)

	if captured["review_id"] != "rev-9" {
		t.Fatalf("review_id = %v", captured["review_id"])
	}
	if captured["outcome"] != "approve" {
		t.Fatalf("outcome = %v", captured["outcome"])
	}
	if captured["emoji_code"] != "thumbs_up" {
		t.Fatalf("emoji_code = %v", captured["emoji_code"])
	}
}
