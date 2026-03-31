package commands

import (
	"encoding/json"
	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestAgentCommand_RequiresSubcommand(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent",
	})

	if len(platform.replies) != 1 || platform.replies[0] != "用法: /agent status|runtimes|health|spawn|run|logs|pause|resume|kill <参数>" {
		t.Fatalf("replies = %v", platform.replies)
	}
}

func TestAgentCommand_ListRepliesWithPoolStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		switch r.URL.Path {
		case "/api/v1/agents/pool":
			if got := r.Header.Get("X-IM-Source"); got != "slack" {
				t.Fatalf("X-IM-Source = %q, want slack", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(&client.PoolStatus{ActiveAgents: 2, MaxAgents: 8})
		case "/api/v1/bridge/pool":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active":         1,
				"max":            4,
				"warm_total":     1,
				"warm_available": 1,
			})
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent list",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	for _, want := range []string{"2/8", "Bridge", "1/4"} {
		if !strings.Contains(platform.replies[0], want) {
			t.Fatalf("reply = %q, want substring %q", platform.replies[0], want)
		}
	}
}

func TestAgentCommand_RuntimesAndHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/bridge/runtimes":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"default_runtime": "codex",
				"runtimes": []map[string]any{
					{"key": "codex", "label": "Codex", "default_provider": "openai", "default_model": "gpt-5-codex", "available": true},
					{"key": "claude_code", "label": "Claude Code", "default_provider": "anthropic", "default_model": "claude-sonnet-4-5", "available": false},
				},
			})
		case "/api/v1/bridge/health":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "ready",
				"pool": map[string]any{
					"active":    2,
					"available": 4,
					"warm":      1,
				},
			})
		default:
			t.Fatalf("unexpected path = %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent runtimes",
	})
	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent health",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	for _, want := range []string{"codex", "claude_code", "gpt-5-codex"} {
		if !strings.Contains(platform.replies[0], want) {
			t.Fatalf("runtimes reply = %q, want substring %q", platform.replies[0], want)
		}
	}
	for _, want := range []string{"ready", "2", "4", "1"} {
		if !strings.Contains(platform.replies[1], want) {
			t.Fatalf("health reply = %q, want substring %q", platform.replies[1], want)
		}
	}
}

func TestAgentCommand_RequiresArgsForRuntimesOnlyViaSubcommandUsage(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent runtimes",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "获取 Bridge runtimes 失败") && !strings.Contains(platform.replies[0], "Bridge runtimes") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
}

func TestAgentCommand_StatusRepliesWithPoolStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		switch r.URL.Path {
		case "/api/v1/agents/pool":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(&client.PoolStatus{Active: 2, Max: 8, Available: 6, PausedResumable: 1, Queued: 3})
		case "/api/v1/bridge/pool":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active":         1,
				"max":            4,
				"warm_total":     1,
				"warm_available": 1,
			})
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent status",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	for _, want := range []string{"2/8", "排队", "可恢复"} {
		if !strings.Contains(platform.replies[0], want) {
			t.Fatalf("reply = %q, want substring %q", platform.replies[0], want)
		}
	}
}

func TestAgentCommand_StatusWithRunIDRepliesWithRunSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/agents/run-123" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&client.AgentRunSummary{
			ID:             "run-123",
			TaskID:         "task-123",
			TaskTitle:      "Bridge rollout",
			Status:         "paused",
			Runtime:        "codex",
			Provider:       "openai",
			Model:          "gpt-5-codex",
			CanResume:      true,
			LastActivityAt: "2026-03-31T12:00:00Z",
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent status run-123",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	for _, want := range []string{"Bridge rollout", "paused", "codex"} {
		if !strings.Contains(platform.replies[0], want) {
			t.Fatalf("reply = %q, want substring %q", platform.replies[0], want)
		}
	}
}

func TestAgentCommand_SpawnRequiresTaskID(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent spawn",
	})

	if len(platform.replies) != 1 || platform.replies[0] != "用法: /agent spawn <task-id>" {
		t.Fatalf("replies = %v", platform.replies)
	}
}

func TestAgentCommand_SpawnRepliesWithRunAndTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/agents/spawn" {
			t.Fatalf("path = %s", r.URL.Path)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["taskId"] != "task-123" {
			t.Fatalf("taskId = %q", body["taskId"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&client.TaskDispatchResponse{
			Task: client.Task{ID: "task-123456"},
			Dispatch: client.DispatchOutcome{
				Status: "started",
				Run:    &client.AgentRun{ID: "run-123456", TaskID: "task-123456"},
			},
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent spawn task-123",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "已启动 Agent #run-1234 执行任务 task-123") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
}

func TestFormatAgentLogs_HandlesEmptyAndLongLists(t *testing.T) {
	if got := formatAgentLogs(nil, "run-12345678"); got != "Agent #run-1234 暂无日志" {
		t.Fatalf("empty logs reply = %q", got)
	}

	logs := make([]client.AgentLogEntry, 16)
	for i := range logs {
		logs[i] = client.AgentLogEntry{Timestamp: "2026-03-25T00:00:00Z", Type: "info", Content: "log line"}
	}
	got := formatAgentLogs(logs, "run-12345678")
	if !strings.Contains(got, "Agent #run-1234 最近日志") {
		t.Fatalf("logs reply = %q", got)
	}
	if !strings.Contains(got, "... 还有 1 条日志") {
		t.Fatalf("logs reply = %q", got)
	}
}

func TestFormatAgentSpawnReply_CoversDispatchBranches(t *testing.T) {
	startedWithoutRun := formatAgentSpawnReply(&client.TaskDispatchResponse{
		Dispatch: client.DispatchOutcome{Status: "started"},
	}, "task-12345678")
	if startedWithoutRun != "已启动 Agent 执行任务 task-123" {
		t.Fatalf("startedWithoutRun = %q", startedWithoutRun)
	}

	blocked := formatAgentSpawnReply(&client.TaskDispatchResponse{
		Dispatch: client.DispatchOutcome{Status: "blocked", Reason: "budget exceeded"},
	}, "task-12345678")
	if blocked != "未启动 Agent：budget exceeded" {
		t.Fatalf("blocked = %q", blocked)
	}

	idle := formatAgentSpawnReply(&client.TaskDispatchResponse{
		Dispatch: client.DispatchOutcome{Status: "queued"},
	}, "task-12345678")
	if idle != "任务 task-123 当前未启动 Agent" {
		t.Fatalf("idle = %q", idle)
	}
}

func TestAgentCommand_RunRequiresPrompt(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent run",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "/agent run <") {
		t.Fatalf("usage reply = %q", platform.replies[0])
	}
}

func TestAgentCommand_RunCreatesTaskAndStartsAgent(t *testing.T) {
	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/api/v1/projects/proj/tasks":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			if body["title"] != "Bridge smoke" || body["description"] != "Bridge smoke" {
				t.Fatalf("create body = %+v", body)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(&client.Task{ID: "task-quick-123"})
		case "/api/v1/agents/spawn":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode spawn body: %v", err)
			}
			if body["taskId"] != "task-quick-123" {
				t.Fatalf("spawn body = %+v", body)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(&client.TaskDispatchResponse{
				Task: client.Task{ID: "task-quick-123"},
				Dispatch: client.DispatchOutcome{
					Status: "started",
					Run:    &client.AgentRun{ID: "run-quick-123", TaskID: "task-quick-123"},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent run Bridge smoke",
	})

	if len(requests) != 2 {
		t.Fatalf("requests = %v", requests)
	}
	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if strings.Contains(platform.replies[0], "run-quic") {
		t.Fatalf("progress reply should not contain run id: %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "run-quic") {
		t.Fatalf("final reply = %q", platform.replies[1])
	}
}

func TestAgentCommand_RunReportsFailuresAfterProgressReply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"create failed"}`, http.StatusBadGateway)
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent run Broken path",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[1], "create failed") {
		t.Fatalf("failure reply = %q", platform.replies[1])
	}
}

func TestAgentCommand_LogsRequiresRunID(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent logs",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "/agent logs <run-id>") {
		t.Fatalf("usage reply = %q", platform.replies[0])
	}
}

func TestAgentCommand_LogsRepliesWithEntriesAndFailures(t *testing.T) {
	step := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if step == 0 {
			step++
			if r.URL.Path != "/api/v1/agents/run-123/logs" {
				t.Fatalf("path = %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]client.AgentLogEntry{
				{Timestamp: "2026-03-25T00:00:00Z", Type: "info", Content: "bridge ready"},
				{Timestamp: "2026-03-25T00:00:05Z", Type: "info", Content: "delivery sent"},
			})
			return
		}
		http.Error(w, `{"message":"logs unavailable"}`, http.StatusBadGateway)
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent logs run-123",
	})
	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent logs run-456",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "bridge ready") || !strings.Contains(platform.replies[0], "delivery sent") {
		t.Fatalf("logs reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "logs unavailable") {
		t.Fatalf("failure reply = %q", platform.replies[1])
	}
}

func TestAgentCommand_UnknownSubcommandShowsUsage(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/agent noop something",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "/agent status|runtimes|health|spawn|run|logs|pause|resume|kill") {
		t.Fatalf("usage reply = %q", platform.replies[0])
	}
}

func TestAgentCommand_LifecycleControlsCallCanonicalEndpoints(t *testing.T) {
	calls := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&client.AgentRunSummary{
			ID:        "run-123",
			TaskID:    "task-123",
			TaskTitle: "Bridge rollout",
			Status:    "paused",
			Runtime:   "codex",
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterAgentCommands(engine, apiClient)

	for _, command := range []string{"/agent pause run-123", "/agent resume run-123", "/agent kill run-123"} {
		engine.HandleMessage(platform, &core.Message{
			Platform: "slack-stub",
			Content:  command,
		})
	}

	wantCalls := []string{
		"POST /api/v1/agents/run-123/pause",
		"POST /api/v1/agents/run-123/resume",
		"POST /api/v1/agents/run-123/kill",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %+v, want %+v", calls, wantCalls)
	}
}

func TestFormatAgentSpawnReply_HandlesBlockedWithoutReason(t *testing.T) {
	got := formatAgentSpawnReply(&client.TaskDispatchResponse{
		Dispatch: client.DispatchOutcome{Status: "blocked"},
	}, "task-12345678")

	if !strings.Contains(got, "Agent") {
		t.Fatalf("reply = %q", got)
	}
	if strings.Contains(got, "budget exceeded") {
		t.Fatalf("reply should not include stale reason: %q", got)
	}
}
