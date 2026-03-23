package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

type taskTestPlatform struct {
	mu      sync.Mutex
	replies []string
}

func (p *taskTestPlatform) Name() string { return "slack-stub" }
func (p *taskTestPlatform) Start(handler core.MessageHandler) error { return nil }
func (p *taskTestPlatform) Stop() error { return nil }
func (p *taskTestPlatform) Send(ctx context.Context, chatID string, content string) error { return nil }
func (p *taskTestPlatform) Reply(ctx context.Context, replyCtx any, content string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.replies = append(p.replies, content)
	return nil
}

func TestTaskCommand_DecomposeSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/tasks/task-123/decompose" {
			t.Fatalf("path = %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"parentTask": map[string]any{
				"id":       "task-123",
				"title":    "Bridge decomposition",
				"status":   "triaged",
				"priority": "high",
			},
			"summary": "拆成 2 个子任务，先打通 API，再补 IM 文案。",
			"subtasks": []map[string]any{
				{"id": "child-1", "title": "打通 API client", "status": "inbox", "priority": "high"},
				{"id": "child-2", "title": "补 IM 回显", "status": "inbox", "priority": "medium"},
			},
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task decompose task-123",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies len = %d, want 2, replies=%v", len(platform.replies), platform.replies)
	}
	if platform.replies[0] != "正在分解任务，请稍候..." {
		t.Fatalf("progress reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "任务分解完成") {
		t.Fatalf("final reply = %q, want completion text", platform.replies[1])
	}
	if !strings.Contains(platform.replies[1], "拆成 2 个子任务") {
		t.Fatalf("final reply = %q, want summary", platform.replies[1])
	}
	if !strings.Contains(platform.replies[1], "打通 API client") || !strings.Contains(platform.replies[1], "补 IM 回显") {
		t.Fatalf("final reply = %q, want subtask titles", platform.replies[1])
	}
}

func TestTaskCommand_DecomposeFailureExplainsNoSubtasksCreated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"invalid task decomposition"}`, http.StatusBadGateway)
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task decompose task-123",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies len = %d, want 2, replies=%v", len(platform.replies), platform.replies)
	}
	if platform.replies[0] != "正在分解任务，请稍候..." {
		t.Fatalf("progress reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "任务分解失败") {
		t.Fatalf("final reply = %q, want failure text", platform.replies[1])
	}
	if !strings.Contains(platform.replies[1], "未创建任何子任务") {
		t.Fatalf("final reply = %q, want no-subtasks explanation", platform.replies[1])
	}
}
