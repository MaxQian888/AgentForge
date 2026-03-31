package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/react-go-quick-starter/server/internal/model"
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
	req := ExecuteRequest{
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
			LoadedSkills: []model.RoleExecutionSkill{
				{
					Path:         "skills/react",
					Label:        "React",
					Description:  "React UI implementation guidance",
					Instructions: "Prefer server-safe React composition.",
					DisplayName:  "React Workspace",
					AvailableParts: []string{
						"agents",
						"references",
					},
					Source:     "repo-local",
					SourceRoot: "skills",
					Origin:     "direct",
				},
			},
			AvailableSkills: []model.RoleExecutionSkill{
				{
					Path:        "skills/testing",
					Label:       "Testing",
					Description: "Regression-oriented test guidance",
					Source:      "repo-local",
					SourceRoot:  "skills",
					Origin:      "direct",
				},
			},
			SkillDiagnostics: []model.RoleExecutionSkillDiagnostic{},
		},
	}
	setStringField(t, &req, "TeamID", "team-123")
	setStringField(t, &req, "TeamRole", "planner")
	setStringSliceField(t, req.RoleConfig, "Tools", []string{"github-tool", "web-search"})
	setStringField(t, req.RoleConfig, "KnowledgeContext", "docs/PRD.md\nshared://design-guidelines")
	setStringSliceField(t, req.RoleConfig, "OutputFilters", []string{"no_credentials", "no_pii"})

	response, err := client.Execute(context.Background(), req)
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
	if gotBody["team_id"] != "team-123" || gotBody["team_role"] != "planner" {
		t.Fatalf("expected team context in execute payload, got %#v", gotBody)
	}
	if !reflect.DeepEqual(roleConfig["tools"], []any{"github-tool", "web-search"}) {
		t.Fatalf("expected bridge tool ids in role_config, got %#v", roleConfig["tools"])
	}
	if roleConfig["knowledge_context"] != "docs/PRD.md\nshared://design-guidelines" {
		t.Fatalf("expected knowledge_context in role_config, got %#v", roleConfig["knowledge_context"])
	}
	if !reflect.DeepEqual(roleConfig["output_filters"], []any{"no_credentials", "no_pii"}) {
		t.Fatalf("expected output_filters in role_config, got %#v", roleConfig["output_filters"])
	}
	loadedSkills, ok := roleConfig["loaded_skills"].([]any)
	if !ok || len(loadedSkills) != 1 {
		t.Fatalf("expected loaded_skills in role_config, got %#v", roleConfig["loaded_skills"])
	}
	loadedSkill, ok := loadedSkills[0].(map[string]any)
	if !ok {
		t.Fatalf("expected loaded skill object, got %#v", loadedSkills[0])
	}
	if loadedSkill["display_name"] != "React Workspace" {
		t.Fatalf("expected loaded skill display_name, got %#v", loadedSkill)
	}
	availableSkills, ok := roleConfig["available_skills"].([]any)
	if !ok || len(availableSkills) != 1 {
		t.Fatalf("expected available_skills in role_config, got %#v", roleConfig["available_skills"])
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
				"role_id":          "frontend-developer",
				"team_id":          "team-123",
				"team_role":        "coder",
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
	assertStructFieldString(t, status, "RoleID", "frontend-developer")
	assertStructFieldString(t, status, "TeamID", "team-123")
	assertStructFieldString(t, status, "TeamRole", "coder")
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
					"model_options":        []string{"claude-sonnet-4-5", "claude-opus-4-1"},
					"available":            true,
					"diagnostics":          []map[string]any{},
					"supported_features":   []string{"structured_output", "interrupt"},
				},
				{
					"key":                  "codex",
					"default_provider":     "openai",
					"compatible_providers": []string{"openai", "codex"},
					"default_model":        "gpt-5-codex",
					"model_options":        []string{"gpt-5-codex", "o3"},
					"available":            false,
					"diagnostics": []map[string]any{
						{
							"code":     "missing_executable",
							"message":  "Executable not found for runtime codex",
							"blocking": true,
						},
					},
					"supported_features": []string{"reasoning", "fork"},
				},
				{
					"key":                  "cursor",
					"default_provider":     "cursor",
					"compatible_providers": []string{"cursor"},
					"default_model":        "claude-sonnet-4-20250514",
					"model_options":        []string{"claude-sonnet-4-20250514", "gpt-4o"},
					"available":            true,
					"diagnostics":          []map[string]any{},
					"supported_features":   []string{"progress", "reasoning"},
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
	if len(catalog.Runtimes) != 3 {
		t.Fatalf("runtime count = %d, want 3", len(catalog.Runtimes))
	}
	if catalog.Runtimes[1].DefaultProvider != "openai" {
		t.Fatalf("codex default provider = %s, want openai", catalog.Runtimes[1].DefaultProvider)
	}
	if !reflect.DeepEqual(catalog.Runtimes[1].ModelOptions, []string{"gpt-5-codex", "o3"}) {
		t.Fatalf("codex model options = %#v, want gpt-5-codex/o3", catalog.Runtimes[1].ModelOptions)
	}
	if !reflect.DeepEqual(catalog.Runtimes[2].SupportedFeatures, []string{"progress", "reasoning"}) {
		t.Fatalf("cursor supported features = %#v, want progress/reasoning", catalog.Runtimes[2].SupportedFeatures)
	}
}

func TestClientGetHealthUsesCanonicalBridgeRoute(t *testing.T) {
	t.Parallel()

	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":        "SERVING",
			"active_agents": 2,
			"max_agents":    5,
			"uptime_ms":     12345,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	health, err := client.GetHealth(context.Background())
	if err != nil {
		t.Fatalf("GetHealth() error: %v", err)
	}

	if gotPath != "/bridge/health" {
		t.Fatalf("expected canonical health route, got %s", gotPath)
	}
	if health.Status != "SERVING" || health.ActiveAgents != 2 || health.MaxAgents != 5 || health.UptimeMS != 12345 {
		t.Fatalf("unexpected health response: %#v", health)
	}
}

func TestClientGetPoolSummaryUsesCanonicalBridgeRoute(t *testing.T) {
	t.Parallel()

	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"active":            1,
			"max":               3,
			"warm_total":        1,
			"warm_available":    0,
			"warm_reuse_hits":   2,
			"cold_starts":       4,
			"last_reconcile_at": 1742896800000,
			"degraded":          false,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	summary, err := client.GetPoolSummary(context.Background())
	if err != nil {
		t.Fatalf("GetPoolSummary() error: %v", err)
	}

	if gotPath != "/bridge/pool" {
		t.Fatalf("expected canonical pool route, got %s", gotPath)
	}
	if summary.Active != 1 || summary.Max != 3 || summary.WarmReuseHits != 2 {
		t.Fatalf("unexpected pool summary: %+v", summary)
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

func TestClientDecomposeIncludesContextWhenSpecified(t *testing.T) {
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
					"title":         "One",
					"description":   "Two",
					"priority":      "high",
					"executionMode": "agent",
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
		Context: map[string]any{
			"relevantFiles": []string{"src-go/internal/server/routes.go"},
			"waveMode":      true,
		},
	})
	if err != nil {
		t.Fatalf("DecomposeTask() error: %v", err)
	}

	contextValue, ok := gotBody["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context object in decompose payload, got %#v", gotBody["context"])
	}
	if contextValue["waveMode"] != true {
		t.Fatalf("unexpected context payload: %#v", contextValue)
	}
}

func TestClientListToolsUsesCanonicalBridgeRoute(t *testing.T) {
	t.Parallel()

	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tools": []map[string]any{
				{
					"plugin_id":   "web-search",
					"name":        "search",
					"description": "Search repos",
					"input_schema": map[string]any{
						"type": "object",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools() error: %v", err)
	}

	if gotPath != "/bridge/tools" {
		t.Fatalf("expected canonical tools route, got %s", gotPath)
	}
	if len(result.Tools) != 1 || result.Tools[0].PluginID != "web-search" || result.Tools[0].Name != "search" {
		t.Fatalf("unexpected tools response: %#v", result)
	}
}

func TestClientInstallToolUsesCanonicalBridgeRoute(t *testing.T) {
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sampleToolPluginRecord("web-search", "active", 0))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	record, err := client.InstallTool(context.Background(), sampleToolPluginManifest("web-search"))
	if err != nil {
		t.Fatalf("InstallTool() error: %v", err)
	}

	if gotPath != "/bridge/tools/install" {
		t.Fatalf("expected canonical install route, got %s", gotPath)
	}
	manifest, ok := gotBody["manifest"].(map[string]any)
	if !ok || manifest["kind"] != string(model.PluginKindTool) {
		t.Fatalf("expected manifest wrapper payload, got %#v", gotBody)
	}
	if record.Metadata.ID != "web-search" || record.LifecycleState != model.PluginStateActive {
		t.Fatalf("unexpected installed record: %#v", record)
	}
}

func TestClientUninstallToolUsesCanonicalBridgeRoute(t *testing.T) {
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sampleToolPluginRecord("web-search", "disabled", 0))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	record, err := client.UninstallTool(context.Background(), "web-search")
	if err != nil {
		t.Fatalf("UninstallTool() error: %v", err)
	}

	if gotPath != "/bridge/tools/uninstall" {
		t.Fatalf("expected canonical uninstall route, got %s", gotPath)
	}
	if gotBody["plugin_id"] != "web-search" {
		t.Fatalf("expected plugin_id uninstall payload, got %#v", gotBody)
	}
	if record.Metadata.ID != "web-search" || record.LifecycleState != model.PluginStateDisabled {
		t.Fatalf("unexpected uninstalled record: %#v", record)
	}
}

func TestClientRestartToolUsesCanonicalBridgeRoute(t *testing.T) {
	t.Parallel()

	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sampleToolPluginRecord("web-search", "active", 1))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	record, err := client.RestartTool(context.Background(), "web-search")
	if err != nil {
		t.Fatalf("RestartTool() error: %v", err)
	}

	if gotPath != "/bridge/tools/web-search/restart" {
		t.Fatalf("expected canonical restart route, got %s", gotPath)
	}
	if record.Metadata.ID != "web-search" || record.RestartCount != 1 {
		t.Fatalf("unexpected restarted record: %#v", record)
	}
}

func TestClientGenerateUsesCanonicalBridgeContract(t *testing.T) {
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

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"text": "Sample response",
			"usage": map[string]any{
				"input_tokens":  12,
				"output_tokens": 8,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.Generate(context.Background(), GenerateRequest{
		Prompt:       "How would you review this dashboard change?",
		SystemPrompt: "You are a design lead.",
		Provider:     "openai",
		Model:        "gpt-5",
		MaxTokens:    256,
		Temperature:  0.2,
	})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if gotPath != "/bridge/generate" {
		t.Fatalf("expected /bridge/generate, got %s", gotPath)
	}
	if gotBody["prompt"] != "How would you review this dashboard change?" {
		t.Fatalf("expected prompt in request body, got %#v", gotBody)
	}
	if gotBody["system_prompt"] != "You are a design lead." {
		t.Fatalf("expected system_prompt in request body, got %#v", gotBody)
	}
	if gotBody["provider"] != "openai" || gotBody["model"] != "gpt-5" {
		t.Fatalf("expected provider/model in request body, got %#v", gotBody)
	}
	if result.Text != "Sample response" || result.Usage.InputTokens != 12 || result.Usage.OutputTokens != 8 {
		t.Fatalf("unexpected generate result: %+v", result)
	}
}

func TestClientClassifyIntentUsesCanonicalBridgeContract(t *testing.T) {
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

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"intent":     "task_assign",
			"command":    "/task assign",
			"args":       "task-123 Alice",
			"confidence": 0.92,
			"reply":      "准备分配任务",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.ClassifyIntent(context.Background(), ClassifyIntentRequest{
		Text:      "把 task-123 分配给 Alice",
		UserID:    "user-123",
		ProjectID: "project-123",
	})
	if err != nil {
		t.Fatalf("ClassifyIntent() error: %v", err)
	}

	if gotPath != "/bridge/classify-intent" {
		t.Fatalf("expected /bridge/classify-intent, got %s", gotPath)
	}
	if gotBody["text"] != "把 task-123 分配给 Alice" || gotBody["user_id"] != "user-123" || gotBody["project_id"] != "project-123" {
		t.Fatalf("unexpected classify intent payload: %#v", gotBody)
	}
	if result.Intent != "task_assign" || result.Command != "/task assign" || result.Confidence != 0.92 {
		t.Fatalf("unexpected classify intent result: %+v", result)
	}
}

func setStringField(t *testing.T, target any, fieldName, value string) {
	t.Helper()
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		t.Fatalf("target must be a non-nil pointer, got %T", target)
	}
	field := rv.Elem().FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("expected field %s on %T", fieldName, target)
	}
	if !field.CanSet() || field.Kind() != reflect.String {
		t.Fatalf("field %s on %T is not settable string", fieldName, target)
	}
	field.SetString(value)
}

func setStringSliceField(t *testing.T, target any, fieldName string, values []string) {
	t.Helper()
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		t.Fatalf("target must be a non-nil pointer, got %T", target)
	}
	field := rv.Elem().FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("expected field %s on %T", fieldName, target)
	}
	if !field.CanSet() || field.Kind() != reflect.Slice {
		t.Fatalf("field %s on %T is not settable slice", fieldName, target)
	}
	slice := reflect.MakeSlice(field.Type(), len(values), len(values))
	for index, value := range values {
		slice.Index(index).SetString(value)
	}
	field.Set(slice)
}

func assertStructFieldString(t *testing.T, target any, fieldName, want string) {
	t.Helper()
	rv := reflect.ValueOf(target)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	field := rv.FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("expected field %s on %T", fieldName, target)
	}
	if field.Kind() != reflect.String {
		t.Fatalf("field %s on %T is not a string", fieldName, target)
	}
	if got := field.String(); got != want {
		t.Fatalf("%s = %q, want %q", fieldName, got, want)
	}
}

func TestClientReviewUsesCanonicalBridgeContract(t *testing.T) {
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

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"risk_level":     "low",
			"findings":       []map[string]any{},
			"summary":        "Deep review completed",
			"recommendation": "approve",
			"cost_usd":       0.15,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Review(context.Background(), ReviewRequest{
		ReviewID:     "review-123",
		TaskID:       "task-123",
		PRURL:        "https://github.com/acme/project/pull/12",
		PRNumber:     12,
		Title:        "Review plugin selection",
		Description:  "Ensures selected plugins are forwarded",
		Diff:         "diff --git a/src/review.ts b/src/review.ts",
		Dimensions:   []string{"logic", "security"},
		TriggerEvent: "pull_request.updated",
		ChangedFiles: []string{"src/review.ts"},
		ReviewPlugins: []ReviewPluginRequest{
			{
				PluginID:     "review.typescript",
				Name:         "TypeScript Review",
				Entrypoint:   "review:run",
				SourceType:   "npm",
				Events:       []string{"pull_request.updated"},
				FilePatterns: []string{"src/**/*.ts"},
				OutputFormat: "findings/v1",
			},
		},
	})
	if err != nil {
		t.Fatalf("Review() error: %v", err)
	}

	if gotPath != "/bridge/review" {
		t.Fatalf("expected /bridge/review, got %s", gotPath)
	}
	if gotBody["trigger_event"] != "pull_request.updated" {
		t.Fatalf("expected trigger_event in request body, got %#v", gotBody)
	}
	changedFiles, ok := gotBody["changed_files"].([]any)
	if !ok || len(changedFiles) != 1 || changedFiles[0] != "src/review.ts" {
		t.Fatalf("expected changed_files payload, got %#v", gotBody["changed_files"])
	}
	reviewPlugins, ok := gotBody["review_plugins"].([]any)
	if !ok || len(reviewPlugins) != 1 {
		t.Fatalf("expected review_plugins payload, got %#v", gotBody["review_plugins"])
	}
	plugin, ok := reviewPlugins[0].(map[string]any)
	if !ok || plugin["plugin_id"] != "review.typescript" || plugin["output_format"] != "findings/v1" {
		t.Fatalf("unexpected review plugin payload: %#v", reviewPlugins[0])
	}
}

func TestClientRefreshToolPluginMCPSurfaceUsesCanonicalBridgeContract(t *testing.T) {
	t.Parallel()

	var (
		gotPath   string
		gotMethod string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"metadata": map[string]any{
				"id": "repo-search",
			},
			"lifecycle_state": "active",
			"runtime_host":    "ts-bridge",
			"restart_count":   1,
			"runtime_metadata": map[string]any{
				"mcp": map[string]any{
					"transport":         "stdio",
					"last_discovery_at": "2026-03-25T10:00:00Z",
					"tool_count":        2,
					"resource_count":    1,
					"prompt_count":      1,
				},
			},
			"mcp_capability_snapshot": map[string]any{
				"transport":         "stdio",
				"last_discovery_at": "2026-03-25T10:00:00Z",
				"tool_count":        2,
				"resource_count":    1,
				"prompt_count":      1,
				"tools": []map[string]any{
					{"name": "search", "description": "Search code"},
				},
				"resources": []map[string]any{
					{"uri": "file://README.md", "name": "README"},
				},
				"prompts": []map[string]any{
					{"name": "summarize", "description": "Summarize repository"},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	surface, err := client.RefreshToolPluginMCPSurface(context.Background(), "repo-search")
	if err != nil {
		t.Fatalf("RefreshToolPluginMCPSurface() error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/bridge/plugins/repo-search/mcp/refresh" {
		t.Fatalf("expected MCP refresh route, got %s", gotPath)
	}
	if surface.PluginID != "repo-search" || surface.Snapshot.Transport != "stdio" {
		t.Fatalf("unexpected MCP surface header: %+v", surface)
	}
	if len(surface.Snapshot.Tools) != 1 || surface.Snapshot.Tools[0].Name != "search" {
		t.Fatalf("unexpected MCP tools: %+v", surface.Snapshot.Tools)
	}
	if surface.RuntimeMetadata == nil || surface.RuntimeMetadata.MCP == nil || surface.RuntimeMetadata.MCP.ToolCount != 2 {
		t.Fatalf("expected runtime metadata summary, got %+v", surface.RuntimeMetadata)
	}
}

func TestClientInvokeToolPluginMCPToolUsesCanonicalBridgeContract(t *testing.T) {
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"plugin_id": "repo-search",
			"operation": "call_tool",
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "found 3 files"},
				},
				"isError": false,
				"structuredContent": map[string]any{
					"count": 3,
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.InvokeToolPluginMCPTool(context.Background(), "repo-search", "search", map[string]any{
		"query": "bridge client",
	})
	if err != nil {
		t.Fatalf("InvokeToolPluginMCPTool() error: %v", err)
	}

	if gotPath != "/bridge/plugins/repo-search/mcp/tools/call" {
		t.Fatalf("expected MCP tool call route, got %s", gotPath)
	}
	if gotBody["tool_name"] != "search" {
		t.Fatalf("expected tool_name in request body, got %#v", gotBody)
	}
	args, ok := gotBody["arguments"].(map[string]any)
	if !ok || args["query"] != "bridge client" {
		t.Fatalf("expected arguments payload, got %#v", gotBody)
	}
	if result.PluginID != "repo-search" || result.Operation != "call_tool" || result.Result.IsError {
		t.Fatalf("unexpected tool call result: %+v", result)
	}
}

func TestClientReadToolPluginMCPResourceUsesCanonicalBridgeContract(t *testing.T) {
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"plugin_id": "repo-search",
			"operation": "read_resource",
			"result": map[string]any{
				"contents": []map[string]any{
					{"uri": "file://README.md", "mimeType": "text/markdown", "text": "# README"},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.ReadToolPluginMCPResource(context.Background(), "repo-search", "file://README.md")
	if err != nil {
		t.Fatalf("ReadToolPluginMCPResource() error: %v", err)
	}

	if gotPath != "/bridge/plugins/repo-search/mcp/resources/read" {
		t.Fatalf("expected MCP resource route, got %s", gotPath)
	}
	if gotBody["uri"] != "file://README.md" {
		t.Fatalf("expected uri in request body, got %#v", gotBody)
	}
	if result.PluginID != "repo-search" || result.Operation != "read_resource" || len(result.Result.Contents) != 1 {
		t.Fatalf("unexpected resource result: %+v", result)
	}
}

func TestClientGetToolPluginMCPPromptUsesCanonicalBridgeContract(t *testing.T) {
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"plugin_id": "repo-search",
			"operation": "get_prompt",
			"result": map[string]any{
				"description": "Repository summary prompt",
				"messages": []map[string]any{
					{
						"role": "user",
						"content": map[string]any{
							"type": "text",
							"text": "Summarize repo-search",
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.GetToolPluginMCPPrompt(context.Background(), "repo-search", "summarize", map[string]string{
		"topic": "repo-search",
	})
	if err != nil {
		t.Fatalf("GetToolPluginMCPPrompt() error: %v", err)
	}

	if gotPath != "/bridge/plugins/repo-search/mcp/prompts/get" {
		t.Fatalf("expected MCP prompt route, got %s", gotPath)
	}
	if gotBody["name"] != "summarize" {
		t.Fatalf("expected prompt name in request body, got %#v", gotBody)
	}
	args, ok := gotBody["arguments"].(map[string]any)
	if !ok || args["topic"] != "repo-search" {
		t.Fatalf("expected prompt arguments in request body, got %#v", gotBody)
	}
	if result.PluginID != "repo-search" || result.Operation != "get_prompt" || result.Result.Description != "Repository summary prompt" {
		t.Fatalf("unexpected prompt result: %+v", result)
	}
}

func sampleToolPluginManifest(id string) model.PluginManifest {
	return model.PluginManifest{
		APIVersion: "agentforge/v1",
		Kind:       model.PluginKindTool,
		Metadata: model.PluginMetadata{
			ID:      id,
			Name:    "Web Search",
			Version: "1.0.0",
		},
		Spec: model.PluginSpec{
			Runtime:   model.PluginRuntimeMCP,
			Transport: "stdio",
			Command:   "node",
			Args:      []string{"index.js"},
		},
		Permissions: model.PluginPermissions{},
		Source: model.PluginSource{
			Type: model.PluginSourceLocal,
		},
	}
}

func sampleToolPluginRecord(id string, lifecycle string, restartCount int) map[string]any {
	return map[string]any{
		"apiVersion": "agentforge/v1",
		"kind":       string(model.PluginKindTool),
		"metadata": map[string]any{
			"id":      id,
			"name":    "Web Search",
			"version": "1.0.0",
		},
		"spec": map[string]any{
			"runtime":   string(model.PluginRuntimeMCP),
			"transport": "stdio",
			"command":   "node",
			"args":      []string{"index.js"},
		},
		"permissions": map[string]any{},
		"source": map[string]any{
			"type": string(model.PluginSourceLocal),
		},
		"lifecycle_state": lifecycle,
		"runtime_host":    string(model.PluginHostTSBridge),
		"restart_count":   restartCount,
	}
}
