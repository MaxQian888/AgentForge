package commands

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

func TestTaskCommand_DecomposeBridgeFirstWithProviderAndModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/tasks/task-123":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":          "task-123",
				"projectId":   "proj",
				"title":       "Bridge decomposition",
				"description": "Break bridge work down",
				"status":      "triaged",
				"priority":    "high",
			})
		case "/api/v1/ai/decompose":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode bridge decompose body: %v", err)
			}
			if body["provider"] != "openai" || body["model"] != "gpt-5" {
				t.Fatalf("bridge decompose body = %+v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"summary": "bridge summary",
				"subtasks": []map[string]any{
					{"title": "API client", "description": "Expose API", "priority": "high", "executionMode": "agent"},
					{"title": "IM reply", "description": "Reply in chat", "priority": "medium", "executionMode": "human"},
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
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task decompose task-123 openai gpt-5",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[1], "bridge summary") {
		t.Fatalf("reply = %q", platform.replies[1])
	}
	for _, want := range []string{"API client", "IM reply"} {
		if !strings.Contains(platform.replies[1], want) {
			t.Fatalf("reply = %q, want substring %q", platform.replies[1], want)
		}
	}
	for _, want := range []string{"可继续执行", "/agent run API client"} {
		if !strings.Contains(platform.replies[1], want) {
			t.Fatalf("reply = %q, want substring %q", platform.replies[1], want)
		}
	}
}

func TestTaskCommand_DecomposeFallsBackToLegacyAPIWhenBridgeUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/tasks/task-123":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":          "task-123",
				"projectId":   "proj",
				"title":       "Bridge decomposition",
				"description": "Break bridge work down",
				"status":      "triaged",
				"priority":    "high",
			})
		case "/api/v1/ai/decompose":
			http.Error(w, `{"message":"bridge unavailable"}`, http.StatusServiceUnavailable)
		case "/api/v1/tasks/task-123/decompose":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"parentTask": map[string]any{
					"id":        "task-123",
					"projectId": "proj",
					"title":     "Bridge decomposition",
					"status":    "triaged",
					"priority":  "high",
				},
				"summary": "legacy fallback summary",
				"subtasks": []map[string]any{
					{"id": "child-1", "title": "legacy subtask", "status": "inbox", "priority": "high"},
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
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task decompose task-123",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	for _, want := range []string{"fallback", "legacy fallback summary", "legacy subtask", "/agent spawn child-1"} {
		if !strings.Contains(platform.replies[1], want) {
			t.Fatalf("reply = %q, want substring %q", platform.replies[1], want)
		}
	}
}

func TestTaskCommand_DecomposeFailureAfterBridgeAndFallbackExplainsNoSubtasksCreated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/tasks/task-123":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":          "task-123",
				"projectId":   "proj",
				"title":       "Bridge decomposition",
				"description": "Break bridge work down",
				"status":      "triaged",
				"priority":    "high",
			})
		case "/api/v1/ai/decompose":
			http.Error(w, `{"message":"bridge unavailable"}`, http.StatusServiceUnavailable)
		case "/api/v1/tasks/task-123/decompose":
			http.Error(w, `{"message":"invalid task decomposition"}`, http.StatusBadGateway)
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
		Content:  "/task decompose task-123",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[1], "未创建任何子任务") {
		t.Fatalf("reply = %q", platform.replies[1])
	}
}

func TestTaskCommand_DecomposeSkipsBridgeWhenCapabilityProbeFails(t *testing.T) {
	calls := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/tasks/task-123/decompose":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"parentTask": map[string]any{
					"id":        "task-123",
					"projectId": "proj",
					"title":     "Bridge decomposition",
					"status":    "triaged",
					"priority":  "high",
				},
				"summary": "legacy fallback summary",
				"subtasks": []map[string]any{
					{"id": "child-1", "title": "legacy subtask", "status": "inbox", "priority": "high"},
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
	engine.SetBridgeCapabilityProbe(core.BridgeCapabilityProbeFunc(func(ctx context.Context, capability core.BridgeCapability) error {
		if capability != core.BridgeCapabilityDecompose {
			t.Fatalf("capability = %q", capability)
		}
		return errors.New("bridge unavailable")
	}))
	RegisterTaskCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/task decompose task-123",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[1], "fallback") || !strings.Contains(platform.replies[1], "legacy fallback summary") {
		t.Fatalf("reply = %q", platform.replies[1])
	}
	if strings.Contains(strings.Join(calls, ","), "/api/v1/ai/decompose") {
		t.Fatalf("bridge decompose endpoint should not be called, calls = %v", calls)
	}
}
