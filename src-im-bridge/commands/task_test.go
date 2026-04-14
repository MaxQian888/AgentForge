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

type taskCardPlatform struct {
	taskTestPlatform
	cards []*core.Card
}

func (p *taskTestPlatform) Name() string                                                  { return "slack-stub" }
func (p *taskTestPlatform) Start(handler core.MessageHandler) error                       { return nil }
func (p *taskTestPlatform) Stop() error                                                   { return nil }
func (p *taskTestPlatform) Send(ctx context.Context, chatID string, content string) error { return nil }
func (p *taskTestPlatform) Reply(ctx context.Context, replyCtx any, content string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.replies = append(p.replies, content)
	return nil
}
func (p *taskCardPlatform) SendCard(ctx context.Context, chatID string, card *core.Card) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cards = append(p.cards, card)
	return nil
}
func (p *taskCardPlatform) ReplyCard(ctx context.Context, replyCtx any, card *core.Card) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cards = append(p.cards, card)
	return nil
}

func TestTaskCommand_CreateRequiresTitle(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &taskCardPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task create",
	})

	if len(platform.replies) != 1 || platform.replies[0] != "用法: /task create <标题> [--priority <级别>] [--description <描述>]" {
		t.Fatalf("replies = %v", platform.replies)
	}
}

func TestTaskCommand_CreateRepliesWithCardWhenSupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/proj/tasks" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-IM-Source"); got != "slack" {
			t.Fatalf("X-IM-Source = %q, want slack", got)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["title"] != "Bridge rollout" {
			t.Fatalf("title = %q", body["title"])
		}
		if _, exists := body["project_id"]; exists {
			t.Fatalf("legacy project_id should not be sent: %+v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&client.Task{
			ID:           "task-123456",
			Title:        "Bridge rollout",
			Status:       "triaged",
			Priority:     "high",
			AssigneeName: "Agent Smith",
			SpentUsd:     1.25,
			BudgetUsd:    3.5,
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskCardPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task create Bridge rollout",
	})

	if len(platform.replies) != 0 {
		t.Fatalf("replies = %v, want no plain replies", platform.replies)
	}
	if len(platform.cards) != 1 {
		t.Fatalf("cards len = %d, want 1", len(platform.cards))
	}
	if platform.cards[0].Title != "任务 #task-123" {
		t.Fatalf("card title = %q", platform.cards[0].Title)
	}
	if len(platform.cards[0].Buttons) != 2 {
		t.Fatalf("buttons = %+v", platform.cards[0].Buttons)
	}
}

func TestTaskCommand_ListIncludesAssigneeAndCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/proj/tasks" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("status"); got != "triaged" {
			t.Fatalf("status = %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]client.Task{
			{ID: "task-123456", Title: "Bridge rollout", Status: "triaged", AssigneeName: "Alice"},
			{ID: "task-789012", Title: "CLI polish", Status: "inbox"},
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskCardPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task list triaged",
	})

	// With CardSender available, task list renders as a card via replyStructured.
	if len(platform.cards) == 1 {
		card := platform.cards[0]
		if !strings.Contains(card.Title, "任务列表") {
			t.Fatalf("card title = %q, want task list title", card.Title)
		}
		foundAlice := false
		for _, field := range card.Fields {
			if strings.Contains(field.Value, "Alice") {
				foundAlice = true
			}
		}
		if !foundAlice {
			t.Fatalf("card fields = %+v, want assignee Alice", card.Fields)
		}
	} else if len(platform.replies) == 1 {
		if !strings.Contains(platform.replies[0], "任务列表 (2)") {
			t.Fatalf("reply = %q", platform.replies[0])
		}
		if !strings.Contains(platform.replies[0], "(@Alice)") {
			t.Fatalf("reply = %q, want assignee mention", platform.replies[0])
		}
	} else {
		t.Fatalf("expected either 1 card or 1 reply, got cards=%d replies=%d", len(platform.cards), len(platform.replies))
	}
}

func TestTaskCommand_ListHandlesEmptyTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]client.Task{})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskCardPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task list",
	})

	if len(platform.replies) != 1 || platform.replies[0] != "暂无任务" {
		t.Fatalf("replies = %v", platform.replies)
	}
}

func TestTaskCommand_StatusRequiresTaskID(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &taskCardPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task status",
	})

	if len(platform.replies) != 1 || platform.replies[0] != "用法: /task status <task-id>" {
		t.Fatalf("replies = %v", platform.replies)
	}
}

func TestTaskCommand_StatusRepliesWithCardWhenSupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/tasks/task-123" {
			t.Fatalf("path = %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&client.Task{
			ID:           "task-123456",
			Title:        "Bridge rollout",
			Status:       "triaged",
			Priority:     "high",
			AssigneeName: "Alice",
			SpentUsd:     1.25,
			BudgetUsd:    3.5,
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskCardPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task status task-123",
	})

	if len(platform.cards) != 1 {
		t.Fatalf("cards len = %d, want 1", len(platform.cards))
	}
	card := platform.cards[0]
	if card.Title != "任务 #task-123" {
		t.Fatalf("card title = %q", card.Title)
	}
	if len(card.Fields) < 4 {
		t.Fatalf("fields = %+v", card.Fields)
	}
}

func TestTaskCommand_AssignRequiresTaskIDAndAssignee(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task assign task-123",
	})

	if len(platform.replies) != 1 || platform.replies[0] != "用法: /task assign <task-id> <assignee>" {
		t.Fatalf("replies = %v", platform.replies)
	}
}

func TestTaskCommand_AssignRepliesWithAssignee(t *testing.T) {
	memberPayload := []map[string]any{
		{"id": "member-1", "name": "Alice", "type": "agent", "isActive": true},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/proj/members":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(memberPayload)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks/task-123/assign":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["assigneeId"] != "member-1" || body["assigneeType"] != "agent" {
				t.Fatalf("assignment body = %+v", body)
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(&client.TaskDispatchResponse{
				Task: client.Task{ID: "task-123456"},
				Dispatch: client.DispatchOutcome{
					Status: "started",
					Run:    &client.AgentRun{ID: "run-123456", TaskID: "task-123456"},
				},
			})
			return
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task assign task-123 Alice",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "已将任务 #task-123 分配给 Alice，并启动 Agent #run-1234") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
}

func TestTaskCommand_DecomposeSuccess(t *testing.T) {
	t.Skip("legacy /task decompose path preserved only as reference; bridge-first coverage lives in dedicated tests")
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
	t.Skip("legacy /task decompose failure preserved only as reference; bridge-first fallback coverage lives in dedicated tests")
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

func TestTaskCommand_AIGenerateAndClassify(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/ai/generate":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode generate body: %v", err)
			}
			if body["prompt"] != "Write a summary" || body["model"] != "gpt-5" {
				t.Fatalf("generate body = %+v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"text": "Summary output",
				"usage": map[string]any{
					"input_tokens":  12,
					"output_tokens": 8,
				},
			})
		case "/api/v1/ai/classify-intent":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode classify body: %v", err)
			}
			if body["text"] != "show sprint status" {
				t.Fatalf("classify body = %+v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"intent":     "sprint_view",
				"command":    "/sprint status",
				"args":       "",
				"confidence": 0.95,
				"reply":      "Route to sprint status",
			})
		default:
			t.Fatalf("unexpected path = %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task ai generate --model gpt-5 Write a summary",
	})
	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task ai classify show sprint status sprint_view,task_list",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "Summary output") {
		t.Fatalf("generate reply = %q", platform.replies[0])
	}
	for _, want := range []string{"sprint_view", "0.95"} {
		if !strings.Contains(platform.replies[1], want) {
			t.Fatalf("classify reply = %q, want substring %q", platform.replies[1], want)
		}
	}
}

func TestTaskCommand_MoveTransitionsTaskStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/tasks/task-123/transition" {
			t.Fatalf("path = %s", r.URL.Path)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["status"] != "done" {
			t.Fatalf("body = %+v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&client.Task{
			ID:       "task-123",
			Title:    "Bridge rollout",
			Status:   "done",
			Priority: "high",
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task move task-123 done",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "done") || !strings.Contains(platform.replies[0], "task-123") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
}

func TestTaskCommand_TransitionAliasUsesCanonicalMoveFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tasks/task-456/transition" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&client.Task{
			ID:       "task-456",
			Title:    "Alias coverage",
			Status:   "in_progress",
			Priority: "medium",
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task transition task-456 in_progress",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "in_progress") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
}

func TestTaskCommand_CreateSupportsPriorityAndDescriptionFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/proj/tasks" {
			t.Fatalf("path = %s", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["title"] != "Bridge rollout" || body["priority"] != "high" || body["description"] != "Fix the bridge pipeline" {
			t.Fatalf("body = %+v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&client.Task{
			ID:           "task-654321",
			Title:        "Bridge rollout",
			Description:  "Fix the bridge pipeline",
			Status:       "inbox",
			Priority:     "high",
			AssigneeName: "",
			SpentUsd:     0,
			BudgetUsd:    0,
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskCardPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task create Bridge rollout --priority high --description Fix the bridge pipeline",
	})

	if len(platform.cards) != 1 {
		t.Fatalf("cards len = %d, want 1", len(platform.cards))
	}
	if platform.cards[0].Title != "任务 #task-654" {
		t.Fatalf("card title = %q", platform.cards[0].Title)
	}
}

func TestTaskCommand_DeleteRemovesTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/tasks/task-789" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "task deleted"})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task delete task-789",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "已删除任务 #task-789") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
}

func TestTaskHelpers_BuildCardShortIDResolveMemberAndDispatchReply(t *testing.T) {
	card := buildTaskCard(&client.Task{
		ID:           "task-12345678",
		ProjectID:    "project-1",
		Title:        "Bridge rollout",
		Description:  "Capture the bridge rollout details",
		Status:       "triaged",
		Priority:     "high",
		AssigneeName: "Alice",
		SpentUsd:     1.25,
		BudgetUsd:    3.5,
	})
	if card.Title != "任务 #task-123" {
		t.Fatalf("title = %q", card.Title)
	}
	if len(card.Fields) != 5 {
		t.Fatalf("fields = %+v", card.Fields)
	}
	if len(card.Buttons) != 4 || card.Buttons[0].Style != "primary" {
		t.Fatalf("buttons = %+v", card.Buttons)
	}
	if card.Buttons[1].Text != "保存为文档" || card.Buttons[2].Text != "创建跟进任务" {
		t.Fatalf("buttons = %+v", card.Buttons)
	}
	action, entityID, metadata, ok := core.ParseActionReferenceWithMetadata(card.Buttons[1].Action)
	if !ok || action != "save-as-doc" || entityID != "project-1" {
		t.Fatalf("doc action = %q", card.Buttons[1].Action)
	}
	if metadata["title"] != "Bridge rollout" || metadata["body"] != "Capture the bridge rollout details" {
		t.Fatalf("doc metadata = %+v", metadata)
	}
	action, entityID, metadata, ok = core.ParseActionReferenceWithMetadata(card.Buttons[2].Action)
	if !ok || action != "create-task" || entityID != "project-1" {
		t.Fatalf("task action = %q", card.Buttons[2].Action)
	}
	if metadata["title"] != "Follow up: Bridge rollout" || metadata["priority"] != "high" {
		t.Fatalf("task metadata = %+v", metadata)
	}

	if got := shortID("task-12345678"); got != "task-123" {
		t.Fatalf("shortID = %q", got)
	}
	if got := shortID("short"); got != "short" {
		t.Fatalf("shortID short = %q", got)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]client.Member{
			{ID: "member-1", Name: "Alice", Type: "agent", IsActive: true},
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	member, err := resolveProjectMember(context.Background(), apiClient, "alice")
	if err != nil {
		t.Fatalf("resolveProjectMember error: %v", err)
	}
	if member.ID != "member-1" {
		t.Fatalf("member = %+v", member)
	}
	if _, err := resolveProjectMember(context.Background(), apiClient, "Bob"); err == nil {
		t.Fatal("expected missing member to fail")
	}

	started := formatTaskDispatchReply(&client.TaskDispatchResponse{
		Task: client.Task{ID: "task-12345678"},
		Dispatch: client.DispatchOutcome{
			Status: "started",
			Run:    &client.AgentRun{ID: "run-12345678"},
		},
	}, "Alice")
	if !strings.Contains(started, "启动 Agent #run-1234") {
		t.Fatalf("started = %q", started)
	}

	blocked := formatTaskDispatchReply(&client.TaskDispatchResponse{
		Task: client.Task{ID: "task-12345678"},
		Dispatch: client.DispatchOutcome{
			Status: "blocked",
			Reason: "budget exceeded",
			GuardrailType: "budget",
			GuardrailScope: "project",
		},
	}, "Alice")
	if blocked != "已将任务 #task-123 分配给 Alice，但未启动 Agent：project budget blocked dispatch: budget exceeded" {
		t.Fatalf("blocked = %q", blocked)
	}

	queued := formatTaskDispatchReply(&client.TaskDispatchResponse{
		Task: client.Task{ID: "task-12345678"},
		Dispatch: client.DispatchOutcome{
			Status: "queued",
			Reason: "Bridge pool at capacity (2/2 active)",
			Queue: &client.QueueEntry{
				EntryID:             "entry-12345678",
				RecoveryDisposition: "recoverable",
			},
		},
	}, "Alice")
	if queued != "已将任务 #task-123 分配给 Alice，且仍在 Agent 队列 #entry-12 中等待恢复：Bridge pool at capacity (2/2 active)" {
		t.Fatalf("queued = %q", queued)
	}

	skipped := formatTaskDispatchReply(&client.TaskDispatchResponse{
		Task: client.Task{ID: "task-12345678"},
		Dispatch: client.DispatchOutcome{
			Status: "skipped",
			Reason: "task assigned to a human member",
		},
	}, "Alice")
	if skipped != "已将任务 #task-123 分配给 Alice，但本次未启动 Agent：task assigned to a human member" {
		t.Fatalf("skipped = %q", skipped)
	}
}
