package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
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

	if len(platform.replies) != 1 || platform.replies[0] != "用法: /agent list|spawn|run|logs <参数>" {
		t.Fatalf("replies = %v", platform.replies)
	}
}

func TestAgentCommand_ListRepliesWithPoolStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/agents/pool" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-IM-Source"); got != "slack" {
			t.Fatalf("X-IM-Source = %q, want slack", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&client.PoolStatus{ActiveAgents: 2, MaxAgents: 8})
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

	if len(platform.replies) != 1 || platform.replies[0] != "Agent 池状态: 2/8 活跃" {
		t.Fatalf("replies = %v", platform.replies)
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
