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
		Runtime:        "opencode",
		Provider:       "anthropic",
		Model:          "claude-sonnet-4-5",
		Prompt:         "Implement the OpenSpec change.",
		WorktreePath:   "D:/Project/AgentForge",
		BranchName:     "agent/task-123",
		SystemPrompt:   "You are a bridge runtime.",
		MaxTurns:       12,
		BudgetUSD:      5,
		AllowedTools:   []string{"Read", "Edit"},
		PermissionMode: "default",
		RoleConfig: &RoleConfig{
			RoleID:         "frontend-developer",
			Name:           "Frontend Developer",
			Role:           "Senior Frontend Developer",
			Goal:           "Build reliable UI",
			Backstory:      "A frontend specialist",
			SystemPrompt:   "You build safe UI.",
			AllowedTools:   []string{"Read", "Edit"},
			MaxBudgetUsd:   5,
			MaxTurns:       20,
			PermissionMode: "default",
		},
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
	if gotBody["provider"] != "anthropic" || gotBody["model"] != "claude-sonnet-4-5" {
		t.Fatalf("expected provider/model in request body, got %#v", gotBody)
	}
	if gotBody["runtime"] != "opencode" {
		t.Fatalf("expected runtime in request body, got %#v", gotBody)
	}
	if gotBody["worktree_path"] != "D:/Project/AgentForge" {
		t.Fatalf("expected worktree_path in request body, got %#v", gotBody)
	}
	if gotBody["permission_mode"] != "default" {
		t.Fatalf("expected permission_mode in request body, got %#v", gotBody)
	}
	roleConfig, ok := gotBody["role_config"].(map[string]any)
	if !ok {
		t.Fatalf("expected role_config payload, got %#v", gotBody["role_config"])
	}
	if roleConfig["role_id"] != "frontend-developer" {
		t.Fatalf("expected role_id in normalized role_config, got %#v", roleConfig)
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

func TestClientPauseAndResumeUseCanonicalBridgeContract(t *testing.T) {
	t.Parallel()

	var (
		pausePath  string
		pauseBody  map[string]any
		resumePath string
		resumeBody map[string]any
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		switch r.URL.Path {
		case "/bridge/pause":
			pausePath = r.URL.Path
			if err := json.NewDecoder(r.Body).Decode(&pauseBody); err != nil {
				t.Fatalf("decode pause request body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true,"session_id":"session-123","status":"paused"}`))
		case "/bridge/resume":
			resumePath = r.URL.Path
			if err := json.NewDecoder(r.Body).Decode(&resumeBody); err != nil {
				t.Fatalf("decode resume request body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"session_id":"session-123","resumed":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	pauseResp, err := client.Pause(context.Background(), "task-123", "user requested pause")
	if err != nil {
		t.Fatalf("Pause() error: %v", err)
	}
	resumeResp, err := client.Resume(context.Background(), ExecuteRequest{
		TaskID:         "task-123",
		SessionID:      "session-123",
		Prompt:         "Resume task-123",
		WorktreePath:   "D:/Project/AgentForge",
		BranchName:     "agent/task-123",
		SystemPrompt:   "",
		MaxTurns:       8,
		BudgetUSD:      2,
		AllowedTools:   []string{"Read"},
		PermissionMode: "default",
		Runtime:        "codex",
	})
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}

	if pausePath != "/bridge/pause" {
		t.Fatalf("expected /bridge/pause, got %s", pausePath)
	}
	if pauseBody["task_id"] != "task-123" || pauseBody["reason"] != "user requested pause" {
		t.Fatalf("expected snake_case pause payload, got %#v", pauseBody)
	}
	if pauseResp.SessionID != "session-123" || pauseResp.Status != "paused" {
		t.Fatalf("unexpected pause response: %#v", pauseResp)
	}

	if resumePath != "/bridge/resume" {
		t.Fatalf("expected /bridge/resume, got %s", resumePath)
	}
	if resumeBody["task_id"] != "task-123" || resumeBody["session_id"] != "session-123" {
		t.Fatalf("expected snake_case resume payload, got %#v", resumeBody)
	}
	if resumeResp.SessionID != "session-123" || !resumeResp.Resumed {
		t.Fatalf("unexpected resume response: %#v", resumeResp)
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
				"runtime":          "codex",
				"provider":         "openai",
				"model":            "gpt-5-codex",
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
	if status.Runtime != "codex" || status.Provider != "openai" || status.Model != "gpt-5-codex" {
		t.Fatalf("expected runtime identity in status response, got %#v", status)
	}
}

func TestClientGetRuntimeCatalogUsesBridgeRoute(t *testing.T) {
	t.Parallel()

	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"default_runtime": "claude_code",
			"runtimes": []map[string]any{
				{
					"key":                  "claude_code",
					"default_provider":     "anthropic",
					"compatible_providers": []string{"anthropic"},
					"default_model":        "claude-sonnet-4-5",
					"available":            true,
					"diagnostics":          []map[string]any{},
				},
				{
					"key":                  "codex",
					"default_provider":     "openai",
					"compatible_providers": []string{"openai", "codex"},
					"default_model":        "gpt-5-codex",
					"available":            false,
					"diagnostics": []map[string]any{
						{
							"code":     "missing_executable",
							"message":  "Executable not found for runtime codex",
							"blocking": true,
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	catalog, err := client.GetRuntimeCatalog(context.Background())
	if err != nil {
		t.Fatalf("GetRuntimeCatalog() error: %v", err)
	}

	if gotPath != "/bridge/runtimes" {
		t.Fatalf("expected canonical runtime catalog route, got %s", gotPath)
	}
	if catalog.DefaultRuntime != "claude_code" {
		t.Fatalf("default runtime = %s, want claude_code", catalog.DefaultRuntime)
	}
	if len(catalog.Runtimes) != 2 {
		t.Fatalf("runtime count = %d, want 2", len(catalog.Runtimes))
	}
	if catalog.Runtimes[1].DefaultProvider != "openai" {
		t.Fatalf("codex default provider = %s, want openai", catalog.Runtimes[1].DefaultProvider)
	}
}

func TestClientDecomposeIncludesProviderAndModelWhenSpecified(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"summary": "Decomposed",
			"subtasks": []map[string]any{
				{
					"title":       "One",
					"description": "Two",
					"priority":    "high",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.DecomposeTask(context.Background(), DecomposeRequest{
		TaskID:      "task-123",
		Title:       "Bridge",
		Description: "Break the task down",
		Priority:    "high",
		Provider:    "openai",
		Model:       "gpt-5",
	})
	if err != nil {
		t.Fatalf("DecomposeTask() error: %v", err)
	}

	if gotBody["provider"] != "openai" {
		t.Fatalf("expected provider in decompose payload, got %#v", gotBody)
	}
	if gotBody["model"] != "gpt-5" {
		t.Fatalf("expected model in decompose payload, got %#v", gotBody)
	}
}
