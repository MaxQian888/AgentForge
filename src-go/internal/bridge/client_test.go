package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientExecuteUsesCanonicalBridgeContract(t *testing.T) {
	t.Parallel()

	var (
		gotPath   string
		gotMethod string
		gotBody   map[string]any
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"session_id": "session-123",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	response, err := client.Execute(context.Background(), ExecuteRequest{
		TaskID:         "task-123",
		SessionID:      "session-123",
		Prompt:         "Implement the OpenSpec change.",
		WorktreePath:   "D:/Project/AgentForge",
		BranchName:     "agent/task-123",
		SystemPrompt:   "You are a bridge runtime.",
		MaxTurns:       12,
		BudgetUSD:      5,
		AllowedTools:   []string{"Read", "Edit"},
		PermissionMode: "default",
	})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/bridge/execute" {
		t.Fatalf("expected /bridge/execute, got %s", gotPath)
	}
	if response.SessionID != "session-123" {
		t.Fatalf("expected session-123, got %s", response.SessionID)
	}
	if gotBody["task_id"] != "task-123" {
		t.Fatalf("expected task_id to be encoded in snake_case, got %#v", gotBody)
	}
	if gotBody["worktree_path"] != "D:/Project/AgentForge" {
		t.Fatalf("expected worktree_path in request body, got %#v", gotBody)
	}
	if gotBody["permission_mode"] != "default" {
		t.Fatalf("expected permission_mode in request body, got %#v", gotBody)
	}
}

func TestClientCancelUsesCanonicalBridgeContract(t *testing.T) {
	t.Parallel()

	var (
		gotPath string
		gotBody map[string]any
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.Cancel(context.Background(), "task-123", "user requested stop"); err != nil {
		t.Fatalf("Cancel() error: %v", err)
	}

	if gotPath != "/bridge/cancel" {
		t.Fatalf("expected /bridge/cancel, got %s", gotPath)
	}
	if gotBody["task_id"] != "task-123" || gotBody["reason"] != "user requested stop" {
		t.Fatalf("expected snake_case cancel payload, got %#v", gotBody)
	}
}

func TestClientHealthAndStatusUseBridgeRoutes(t *testing.T) {
	t.Parallel()

	var (
		statusPath string
		healthPath string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bridge/status/task-123":
			statusPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"task_id":          "task-123",
				"state":            "running",
				"turn_number":      3,
				"last_tool":        "Read",
				"last_activity_ms": 1234567890,
				"spent_usd":        0.12,
			})
		case "/bridge/health":
			healthPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"SERVING"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	status, err := client.GetStatus(context.Background(), "task-123")
	if err != nil {
		t.Fatalf("GetStatus() error: %v", err)
	}
	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("Health() error: %v", err)
	}

	if statusPath != "/bridge/status/task-123" {
		t.Fatalf("expected canonical status route, got %s", statusPath)
	}
	if healthPath != "/bridge/health" {
		t.Fatalf("expected canonical health route, got %s", healthPath)
	}
	if status.State != "running" || status.TurnNumber != 3 || status.LastTool != "Read" {
		t.Fatalf("unexpected status response: %#v", status)
	}
}
