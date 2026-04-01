package client

import (
	"context"
	"encoding/json"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/platform/discord"
	"github.com/agentforge/im-bridge/platform/qq"
	"github.com/agentforge/im-bridge/platform/qqbot"
	"github.com/agentforge/im-bridge/platform/telegram"
	"github.com/agentforge/im-bridge/platform/wecom"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestWithSource_NormalizesHeaderValue(t *testing.T) {
	var gotSource string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSource = r.Header.Get("X-IM-Source")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret").WithSource("slack-stub")

	resp, err := client.doRequest(context.Background(), http.MethodGet, "/", nil)
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	resp.Body.Close()

	if gotSource != "slack" {
		t.Fatalf("X-IM-Source = %q, want slack", gotSource)
	}
}

func TestDecomposeTask_CallsEndpointAndParsesResponse(t *testing.T) {
	const taskID = "task-123"

	type responseBody struct {
		ParentTask Task                    `json:"parentTask"`
		Summary    string                  `json:"summary"`
		Subtasks   []TaskDecompositionItem `json:"subtasks"`
	}

	var gotMethod string
	var gotPath string
	var gotAuth string
	var gotSource string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotSource = r.Header.Get("X-IM-Source")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responseBody{
			ParentTask: Task{ID: taskID, Title: "Refactor bridge", Status: "triaged", Priority: "high"},
			Summary:    "拆成 2 个子任务，先补接口再补命令。",
			Subtasks: []TaskDecompositionItem{
				{ID: "child-1", Title: "实现 API client", Status: "inbox", Priority: "high"},
				{ID: "child-2", Title: "实现 IM 命令", Status: "inbox", Priority: "medium"},
			},
		})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret").WithSource("slack-stub")

	result, err := client.DecomposeTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("DecomposeTask error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/api/v1/tasks/"+taskID+"/decompose" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotSource != "slack" {
		t.Fatalf("X-IM-Source = %q, want slack", gotSource)
	}
	if result.ParentTask.ID != taskID {
		t.Fatalf("parent task id = %q, want %q", result.ParentTask.ID, taskID)
	}
	if result.Summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if len(result.Subtasks) != 2 {
		t.Fatalf("subtasks len = %d, want 2", len(result.Subtasks))
	}
	if result.Subtasks[0].Title != "实现 API client" {
		t.Fatalf("first subtask title = %q", result.Subtasks[0].Title)
	}
}

func TestDecomposeTaskViaBridge_LoadsTaskAndCallsAIEndpoint(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/tasks/task-123":
			_ = json.NewEncoder(w).Encode(Task{
				ID:          "task-123",
				ProjectID:   "proj",
				Title:       "Bridge rollout",
				Description: "Break bridge work down",
				Priority:    "high",
				Status:      "triaged",
			})
		case "/api/v1/ai/decompose":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode bridge body: %v", err)
			}
			if body["task_id"] != "task-123" || body["title"] != "Bridge rollout" || body["provider"] != "openai" || body["model"] != "gpt-5" {
				t.Fatalf("bridge body = %+v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"summary": "Bridge summary",
				"subtasks": []map[string]any{
					{"title": "API client", "description": "Expose API", "priority": "high", "executionMode": "agent"},
					{"title": "IM command", "description": "Reply in chat", "priority": "medium", "executionMode": "human"},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	result, err := client.DecomposeTaskViaBridge(context.Background(), "task-123", "openai", "gpt-5")
	if err != nil {
		t.Fatalf("DecomposeTaskViaBridge error: %v", err)
	}

	if result.ParentTask.ID != "task-123" || result.Summary != "Bridge summary" || len(result.Subtasks) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if !reflect.DeepEqual(calls, []string{
		"GET /api/v1/tasks/task-123",
		"POST /api/v1/ai/decompose",
	}) {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestCreateTask_SendsProjectPayloadAndParsesResponse(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Task{ID: "task-1", Title: "Bridge rollout"})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	task, err := client.CreateTask(context.Background(), "Bridge rollout", "desc")
	if err != nil {
		t.Fatalf("CreateTask error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/api/v1/projects/proj/tasks" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotBody["title"] != "Bridge rollout" || gotBody["description"] != "desc" {
		t.Fatalf("body = %+v", gotBody)
	}
	if _, exists := gotBody["project_id"]; exists {
		t.Fatalf("body should not send legacy project_id field: %+v", gotBody)
	}
	if task.ID != "task-1" || task.Title != "Bridge rollout" {
		t.Fatalf("task = %+v", task)
	}
}

func TestCreateTask_ReturnsDecodeErrorForInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{"))
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	_, err := client.CreateTask(context.Background(), "Bridge rollout", "desc")
	if err == nil {
		t.Fatal("expected CreateTask to fail")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("err = %v", err)
	}
}

func TestListTasks_AppendsStatusFilterAndParsesTasks(t *testing.T) {
	var gotPath string
	var gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Task{{ID: "task-1", Status: "triaged", Title: "Bridge rollout"}})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	tasks, err := client.ListTasks(context.Background(), "triaged")
	if err != nil {
		t.Fatalf("ListTasks error: %v", err)
	}

	if gotPath != "/api/v1/projects/proj/tasks" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotQuery != "status=triaged" {
		t.Fatalf("query = %q", gotQuery)
	}
	if len(tasks) != 1 || tasks[0].ID != "task-1" {
		t.Fatalf("tasks = %+v", tasks)
	}
}

func TestListTasks_ReturnsDecodeErrorForInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{"))
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	_, err := client.ListTasks(context.Background(), "")
	if err == nil {
		t.Fatal("expected ListTasks to fail")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("err = %v", err)
	}
}

func TestBridgeToolsClientMethods_CallExpectedEndpoints(t *testing.T) {
	calls := make([]string, 0, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bridge/tools":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tools": []map[string]any{
					{"plugin_id": "web-search", "name": "search", "description": "Search repos"},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/bridge/tools/install":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode install body: %v", err)
			}
			if body["manifest_url"] != "https://registry.example.com/web-search.yaml" {
				t.Fatalf("install body = %+v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"metadata":        map[string]any{"id": "web-search", "name": "Web Search", "version": "1.0.0"},
				"lifecycle_state": "active",
				"restart_count":   0,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/bridge/tools/uninstall":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode uninstall body: %v", err)
			}
			if body["plugin_id"] != "web-search" {
				t.Fatalf("uninstall body = %+v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"metadata":        map[string]any{"id": "web-search", "name": "Web Search", "version": "1.0.0"},
				"lifecycle_state": "disabled",
				"restart_count":   0,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/bridge/tools/web-search/restart":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"metadata":        map[string]any{"id": "web-search", "name": "Web Search", "version": "1.0.0"},
				"lifecycle_state": "active",
				"restart_count":   1,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	tools, err := client.ListBridgeTools(context.Background())
	if err != nil {
		t.Fatalf("ListBridgeTools error: %v", err)
	}
	if len(tools) != 1 || tools[0].PluginID != "web-search" || tools[0].Name != "search" {
		t.Fatalf("tools = %+v", tools)
	}

	installed, err := client.InstallBridgeTool(context.Background(), "https://registry.example.com/web-search.yaml")
	if err != nil {
		t.Fatalf("InstallBridgeTool error: %v", err)
	}
	if installed.Metadata.ID != "web-search" || installed.LifecycleState != "active" {
		t.Fatalf("installed = %+v", installed)
	}

	uninstalled, err := client.UninstallBridgeTool(context.Background(), "web-search")
	if err != nil {
		t.Fatalf("UninstallBridgeTool error: %v", err)
	}
	if uninstalled.LifecycleState != "disabled" {
		t.Fatalf("uninstalled = %+v", uninstalled)
	}

	restarted, err := client.RestartBridgeTool(context.Background(), "web-search")
	if err != nil {
		t.Fatalf("RestartBridgeTool error: %v", err)
	}
	if restarted.RestartCount != 1 {
		t.Fatalf("restarted = %+v", restarted)
	}

	if !reflect.DeepEqual(calls, []string{
		"GET /api/v1/bridge/tools",
		"POST /api/v1/bridge/tools/install",
		"POST /api/v1/bridge/tools/uninstall",
		"POST /api/v1/bridge/tools/web-search/restart",
	}) {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestGetTask_ReturnsAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	_, err := client.GetTask(context.Background(), "task-404")
	if err == nil {
		t.Fatal("expected GetTask to fail")
	}
	if !strings.Contains(err.Error(), "API error 404") {
		t.Fatalf("err = %v", err)
	}
}

func TestGetTask_ParsesSuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tasks/task-1" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Task{ID: "task-1", Title: "Bridge rollout"})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	task, err := client.GetTask(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("GetTask error: %v", err)
	}
	if task.ID != "task-1" || task.Title != "Bridge rollout" {
		t.Fatalf("task = %+v", task)
	}
}

func TestAssignTask_SendsCanonicalPayloadAndParsesDispatchResult(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TaskDispatchResponse{
			Task: Task{ID: "task-1"},
			Dispatch: DispatchOutcome{
				Status: "started",
				Run:    &AgentRun{ID: "run-1", TaskID: "task-1", Status: "running"},
			},
		})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	result, err := client.AssignTask(context.Background(), "task-1", "member-1", "agent")
	if err != nil {
		t.Fatalf("AssignTask error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/api/v1/tasks/task-1/assign" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotBody["assigneeId"] != "member-1" || gotBody["assigneeType"] != "agent" {
		t.Fatalf("body = %+v", gotBody)
	}
	if result.Dispatch.Status != "started" || result.Dispatch.Run == nil || result.Dispatch.Run.ID != "run-1" {
		t.Fatalf("result = %+v", result)
	}
}

func TestSpawnAgent_CallsEndpointAndParsesDispatchResult(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TaskDispatchResponse{
			Task: Task{ID: "task-1"},
			Dispatch: DispatchOutcome{
				Status: "started",
				Run:    &AgentRun{ID: "run-1", TaskID: "task-1", Status: "starting"},
			},
		})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	result, err := client.SpawnAgent(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("SpawnAgent error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/api/v1/agents/spawn" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotBody["taskId"] != "task-1" {
		t.Fatalf("body = %+v", gotBody)
	}
	if result.Dispatch.Status != "started" || result.Dispatch.Run == nil || result.Dispatch.Run.ID != "run-1" {
		t.Fatalf("result = %+v", result)
	}
}

func TestSpawnAgent_ReturnsAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "spawn failed", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	_, err := client.SpawnAgent(context.Background(), "task-1")
	if err == nil {
		t.Fatal("expected SpawnAgent to fail")
	}
	if !strings.Contains(err.Error(), "API error 502") {
		t.Fatalf("err = %v", err)
	}
}

func TestGetAgentPoolStatus_ParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(PoolStatus{ActiveAgents: 3, MaxAgents: 10})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	status, err := client.GetAgentPoolStatus(context.Background())
	if err != nil {
		t.Fatalf("GetAgentPoolStatus error: %v", err)
	}
	if status.ActiveAgents != 3 || status.MaxAgents != 10 {
		t.Fatalf("status = %+v", status)
	}
}

func TestBridgeRuntimeStatusMethods_ParseResponses(t *testing.T) {
	calls := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/bridge/pool":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active":         2,
				"max":            6,
				"warm_total":     1,
				"warm_available": 1,
				"degraded":       false,
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
		case "/api/v1/bridge/runtimes":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"default_runtime": "codex",
				"runtimes": []map[string]any{
					{"key": "codex", "label": "Codex", "default_provider": "openai", "default_model": "gpt-5-codex", "available": true},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	pool, err := client.GetBridgePoolStatus(context.Background())
	if err != nil {
		t.Fatalf("GetBridgePoolStatus error: %v", err)
	}
	if pool.Active != 2 || pool.Max != 6 {
		t.Fatalf("pool = %+v", pool)
	}

	health, err := client.GetBridgeHealth(context.Background())
	if err != nil {
		t.Fatalf("GetBridgeHealth error: %v", err)
	}
	if health.Status != "ready" || health.Pool.Active != 2 {
		t.Fatalf("health = %+v", health)
	}

	runtimes, err := client.GetBridgeRuntimes(context.Background())
	if err != nil {
		t.Fatalf("GetBridgeRuntimes error: %v", err)
	}
	if runtimes.DefaultRuntime != "codex" || len(runtimes.Runtimes) != 1 || runtimes.Runtimes[0].Key != "codex" {
		t.Fatalf("runtimes = %+v", runtimes)
	}

	if !reflect.DeepEqual(calls, []string{
		"GET /api/v1/bridge/pool",
		"GET /api/v1/bridge/health",
		"GET /api/v1/bridge/runtimes",
	}) {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestBridgeAITextMethods_CallExpectedEndpoints(t *testing.T) {
	calls := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
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
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	generated, err := client.GenerateTaskAI(context.Background(), "Write a summary", "", "gpt-5")
	if err != nil {
		t.Fatalf("GenerateTaskAI error: %v", err)
	}
	if generated.Text != "Summary output" || generated.Usage.OutputTokens != 8 {
		t.Fatalf("generated = %+v", generated)
	}

	classified, err := client.ClassifyTaskAI(context.Background(), "show sprint status", []string{"sprint_view", "task_list"})
	if err != nil {
		t.Fatalf("ClassifyTaskAI error: %v", err)
	}
	if classified.Intent != "sprint_view" || classified.Confidence != 0.95 {
		t.Fatalf("classified = %+v", classified)
	}

	if !reflect.DeepEqual(calls, []string{
		"POST /api/v1/ai/generate",
		"POST /api/v1/ai/classify-intent",
	}) {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestGetCostStats_ParsesResponse(t *testing.T) {
	var gotPath string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"totalCostUsd": 11.1,
			"budgetSummary": map[string]any{
				"allocated": 20.2,
			},
			"periodRollups": map[string]any{
				"today": map[string]any{
					"costUsd": 1.1,
				},
				"last7Days": map[string]any{
					"costUsd": 4.4,
				},
				"last30Days": map[string]any{
					"costUsd": 11.1,
				},
			},
		})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	stats, err := client.GetCostStats(context.Background())
	if err != nil {
		t.Fatalf("GetCostStats error: %v", err)
	}
	if gotPath != "/api/v1/stats/cost" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotQuery != "projectId=proj" {
		t.Fatalf("query = %q", gotQuery)
	}
	if stats.TotalUsd != 11.1 || stats.BudgetUsd != 20.2 {
		t.Fatalf("stats = %+v", stats)
	}
	if stats.DailyUsd != 1.1 || stats.WeeklyUsd != 4.4 || stats.MonthlyUsd != 11.1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestGetCostStats_ReturnsAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "costs failed", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	_, err := client.GetCostStats(context.Background())
	if err == nil {
		t.Fatal("expected GetCostStats to fail")
	}
	if !strings.Contains(err.Error(), "API error 502") {
		t.Fatalf("err = %v", err)
	}
}

func TestSendNLU_SendsIntentPayloadAndParsesReply(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"reply": "task created"})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	reply, err := client.SendNLU(context.Background(), "create task", "user-1")
	if err != nil {
		t.Fatalf("SendNLU error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/api/v1/intent" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotBody["text"] != "create task" || gotBody["user_id"] != "user-1" || gotBody["project_id"] != "proj" {
		t.Fatalf("body = %+v", gotBody)
	}
	if reply != "task created" {
		t.Fatalf("reply = %q", reply)
	}
}

func TestClassifyMentionIntent_SendsCandidatesAndContext(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TaskAIClassifyResponse{
			Intent:     "help",
			Command:    "/help",
			Args:       "",
			Confidence: 0.91,
		})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	reply, err := client.ClassifyMentionIntent(context.Background(), MentionIntentRequest{
		Text:       "@AgentForge 帮我看看帮助",
		UserID:     "user-1",
		Candidates: []string{"help", "task_list"},
		Context: map[string]any{
			"history": []string{"上一条消息"},
		},
	})
	if err != nil {
		t.Fatalf("ClassifyMentionIntent error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/api/v1/ai/classify-intent" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotBody["text"] != "@AgentForge 帮我看看帮助" || gotBody["user_id"] != "user-1" {
		t.Fatalf("body = %+v", gotBody)
	}
	candidates, ok := gotBody["candidates"].([]any)
	if !ok || len(candidates) != 2 {
		t.Fatalf("candidates = %#v", gotBody["candidates"])
	}
	contextValue, ok := gotBody["context"].(map[string]any)
	if !ok {
		t.Fatalf("context = %#v", gotBody["context"])
	}
	if reply.Intent != "help" || contextValue["history"] == nil {
		t.Fatalf("reply=%+v context=%#v", reply, contextValue)
	}
}

func TestHandleIMAction_SendsCanonicalPayloadAndParsesReplyTarget(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(IMActionResponse{
			Result: "Approved",
			ReplyTarget: &core.ReplyTarget{
				Platform:          "slack",
				ChannelID:         "C123",
				ThreadID:          "thread-1",
				PreferredRenderer: "blocks",
			},
			Metadata: map[string]string{
				"source": "block_actions",
			},
		})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret").WithSource("slack").WithBridgeContext("bridge-slack-1", &core.ReplyTarget{
		Platform:  "slack",
		ChannelID: "C123",
		ThreadID:  "thread-1",
	})

	resp, err := client.HandleIMAction(context.Background(), IMActionRequest{
		Action:    "approve",
		EntityID:  "review-1",
		ChannelID: "C123",
		UserID:    "U123",
		Metadata: map[string]string{
			"source": "block_actions",
		},
	})
	if err != nil {
		t.Fatalf("HandleIMAction error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/api/v1/im/action" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotBody["platform"] != "slack" {
		t.Fatalf("platform = %v", gotBody["platform"])
	}
	if gotBody["bridgeId"] != "bridge-slack-1" {
		t.Fatalf("bridgeId = %v", gotBody["bridgeId"])
	}
	replyTarget, ok := gotBody["replyTarget"].(map[string]any)
	if !ok {
		t.Fatalf("replyTarget = %#v", gotBody["replyTarget"])
	}
	if replyTarget["threadId"] != "thread-1" {
		t.Fatalf("threadId = %v", replyTarget["threadId"])
	}
	if resp.ReplyTarget == nil || resp.ReplyTarget.PreferredRenderer != "blocks" {
		t.Fatalf("ReplyTarget = %+v", resp.ReplyTarget)
	}
	if resp.Metadata["source"] != "block_actions" {
		t.Fatalf("Metadata = %+v", resp.Metadata)
	}
}

func TestHandleIMAction_ParsesCanonicalActionOutcome(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(IMActionResponse{
			Result:  "Task task-1 was dispatched and agent run run-1 started.",
			Success: true,
			Status:  "started",
			Task: &Task{
				ID:    "task-1",
				Title: "Bridge rollout",
			},
			Dispatch: &DispatchOutcome{
				Status: "started",
				Run:    &AgentRun{ID: "run-1", TaskID: "task-1", Status: "running"},
			},
			Metadata: map[string]string{
				"action_status": "started",
			},
		})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")
	resp, err := client.HandleIMAction(context.Background(), IMActionRequest{
		Platform:  "slack",
		Action:    "assign-agent",
		EntityID:  "task-1",
		ChannelID: "C123",
	})
	if err != nil {
		t.Fatalf("HandleIMAction error: %v", err)
	}

	if resp.Status != "started" {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.Task == nil || resp.Task.ID != "task-1" {
		t.Fatalf("task = %+v", resp.Task)
	}
	if resp.Dispatch == nil || resp.Dispatch.Run == nil || resp.Dispatch.Run.ID != "run-1" {
		t.Fatalf("dispatch = %+v", resp.Dispatch)
	}
}

func TestWithSource_LeavesExistingSourceWhenNormalizationReturnsEmpty(t *testing.T) {
	client := NewAgentForgeClient("http://example.test", "proj", "secret").WithSource("slack-stub")

	scoped := client.WithSource("   ")
	if scoped.imSource != "slack" {
		t.Fatalf("imSource = %q, want slack", scoped.imSource)
	}
}

func TestWithPlatform_UsesTelegramMetadataSource(t *testing.T) {
	assertPlatformHeader(t, telegram.NewStub("0"), "telegram")
}

func TestWithPlatform_UsesDiscordMetadataSource(t *testing.T) {
	assertPlatformHeader(t, discord.NewStub("0"), "discord")
}

func TestWithPlatform_UsesWeComMetadataSource(t *testing.T) {
	assertPlatformHeader(t, wecom.NewStub("0"), "wecom")
}

func TestWithPlatform_UsesQQMetadataSource(t *testing.T) {
	assertPlatformHeader(t, qq.NewStub("0"), "qq")
}

func TestWithPlatform_UsesQQBotMetadataSource(t *testing.T) {
	assertPlatformHeader(t, qqbot.NewStub("0"), "qqbot")
}

func TestProjectScopedOperations_RequireConfiguredProjectScope(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "", "secret")
	checks := []struct {
		name string
		call func() error
	}{
		{
			name: "members",
			call: func() error {
				_, err := client.ListProjectMembers(context.Background())
				return err
			},
		},
		{
			name: "queue list",
			call: func() error {
				_, err := client.ListQueueEntries(context.Background(), "queued")
				return err
			},
		},
		{
			name: "queue cancel",
			call: func() error {
				_, err := client.CancelQueueEntry(context.Background(), "entry-1", "manual_cancel")
				return err
			},
		},
		{
			name: "memory search",
			call: func() error {
				_, err := client.SearchProjectMemory(context.Background(), "release", 5)
				return err
			},
		},
		{
			name: "memory note",
			call: func() error {
				_, err := client.StoreProjectMemoryNote(context.Background(), "operator-note", "Remember to reuse Codex")
				return err
			},
		},
	}

	for _, tc := range checks {
		err := tc.call()
		if err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
		if !strings.Contains(err.Error(), "project scope is not configured") {
			t.Fatalf("%s: err = %v", tc.name, err)
		}
	}

	if requestCount != 0 {
		t.Fatalf("requestCount = %d, want 0", requestCount)
	}
}

func TestDoRequest_RejectsUnmarshalableBody(t *testing.T) {
	client := NewAgentForgeClient("http://example.test", "proj", "secret")

	_, err := client.doRequest(context.Background(), http.MethodPost, "/bad", make(chan int))
	if err == nil {
		t.Fatal("expected doRequest to fail")
	}
	if !strings.Contains(err.Error(), "marshal body") {
		t.Fatalf("err = %v", err)
	}
}

func TestTriggerStandaloneDeepReview_SendsManualTriggerPayload(t *testing.T) {
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/reviews/trigger" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Review{
			ID:             "review-standalone",
			PRURL:          "https://github.com/org/repo/pull/9",
			Status:         "pending",
			Recommendation: "approve",
		})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj-1", "secret")
	review, err := client.TriggerStandaloneDeepReview(context.Background(), "https://github.com/org/repo/pull/9")
	if err != nil {
		t.Fatalf("TriggerStandaloneDeepReview error: %v", err)
	}
	if gotBody["trigger"] != "manual" {
		t.Fatalf("trigger = %q, want manual", gotBody["trigger"])
	}
	if gotBody["projectId"] != "proj-1" {
		t.Fatalf("projectId = %q, want proj-1", gotBody["projectId"])
	}
	if review.ID != "review-standalone" {
		t.Fatalf("review = %+v", review)
	}
}

func TestApproveReview_CallsApproveEndpoint(t *testing.T) {
	var gotPath string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Review{
			ID:             "review-1",
			Status:         "completed",
			Recommendation: "approve",
		})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")
	review, err := client.ApproveReview(context.Background(), "review-1", "LGTM")
	if err != nil {
		t.Fatalf("ApproveReview error: %v", err)
	}
	if gotPath != "/api/v1/reviews/review-1/approve" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["comment"] != "LGTM" {
		t.Fatalf("comment = %q", gotBody["comment"])
	}
	if review.Recommendation != "approve" {
		t.Fatalf("review = %+v", review)
	}
}

func TestRequestChangesReview_CallsRequestChangesEndpoint(t *testing.T) {
	var gotPath string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Review{
			ID:             "review-2",
			Status:         "completed",
			Recommendation: "request_changes",
		})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")
	review, err := client.RequestChangesReview(context.Background(), "review-2", "Need more tests")
	if err != nil {
		t.Fatalf("RequestChangesReview error: %v", err)
	}
	if gotPath != "/api/v1/reviews/review-2/request-changes" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["comment"] != "Need more tests" {
		t.Fatalf("comment = %q", gotBody["comment"])
	}
	if review.Recommendation != "request_changes" {
		t.Fatalf("review = %+v", review)
	}
}

func assertPlatformHeader(t *testing.T, platform core.Platform, want string) {
	t.Helper()

	var gotSource string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSource = r.Header.Get("X-IM-Source")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret").WithPlatform(platform)
	resp, err := client.doRequest(context.Background(), http.MethodGet, "/", nil)
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	resp.Body.Close()

	if gotSource != want {
		t.Fatalf("X-IM-Source = %q, want %q", gotSource, want)
	}
}

func TestListProjectMembers_ParsesMembers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj-1/members" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Member{{ID: "member-1", Name: "Alice", Type: "agent", IsActive: true}})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj-1", "secret")
	members, err := client.ListProjectMembers(context.Background())
	if err != nil {
		t.Fatalf("ListProjectMembers error: %v", err)
	}
	if len(members) != 1 || members[0].Name != "Alice" {
		t.Fatalf("members = %+v", members)
	}
}

func TestTriggerReviewAndGetReview_UseReviewEndpoints(t *testing.T) {
	var gotTriggerBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/reviews/trigger":
			if err := json.NewDecoder(r.Body).Decode(&gotTriggerBody); err != nil {
				t.Fatalf("decode trigger body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(Review{ID: "review-1", PRURL: "https://example.test/pr/1", Status: "pending"})
		case "/api/v1/reviews/review-1":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(Review{ID: "review-1", Status: "completed", Summary: "Looks good"})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj-1", "secret")

	review, err := client.TriggerReview(context.Background(), "https://example.test/pr/1")
	if err != nil {
		t.Fatalf("TriggerReview error: %v", err)
	}
	if gotTriggerBody["projectId"] != "proj-1" || gotTriggerBody["prUrl"] != "https://example.test/pr/1" {
		t.Fatalf("trigger body = %+v", gotTriggerBody)
	}
	if review.ID != "review-1" || review.Status != "pending" {
		t.Fatalf("review = %+v", review)
	}

	loaded, err := client.GetReview(context.Background(), "review-1")
	if err != nil {
		t.Fatalf("GetReview error: %v", err)
	}
	if loaded.Status != "completed" || loaded.Summary != "Looks good" {
		t.Fatalf("loaded review = %+v", loaded)
	}
}

func TestGetCurrentSprint_ReturnsFirstActiveSprintAndErrorsWhenEmpty(t *testing.T) {
	activeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Sprint{
			{ID: "sprint-1", Name: "Sprint 1", Status: "active"},
			{ID: "sprint-2", Name: "Sprint 2", Status: "planned"},
		})
	}))
	defer activeServer.Close()

	clientWithSprint := NewAgentForgeClient(activeServer.URL, "proj-1", "secret")

	sprint, err := clientWithSprint.GetCurrentSprint(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentSprint error: %v", err)
	}
	if sprint.ID != "sprint-1" {
		t.Fatalf("sprint = %+v", sprint)
	}

	emptyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Sprint{})
	}))
	defer emptyServer.Close()

	emptyClient := NewAgentForgeClient(emptyServer.URL, "proj-1", "secret")
	if _, err := emptyClient.GetCurrentSprint(context.Background()); err == nil || !strings.Contains(err.Error(), "no active sprint found") {
		t.Fatalf("err = %v", err)
	}
}

func TestGetSprintBurndown_ParsesMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sprints/sprint-1/burndown" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SprintMetrics{
			Sprint:         Sprint{ID: "sprint-1"},
			RemainingTasks: 3,
			Burndown:       []BurndownPoint{{Date: "2026-03-25", RemainingTasks: 3}},
		})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj-1", "secret")
	metrics, err := client.GetSprintBurndown(context.Background(), "sprint-1")
	if err != nil {
		t.Fatalf("GetSprintBurndown error: %v", err)
	}
	if metrics.RemainingTasks != 3 || len(metrics.Burndown) != 1 {
		t.Fatalf("metrics = %+v", metrics)
	}
}

func TestQuickAgentRun_ComposesCreateAndSpawn(t *testing.T) {
	var calls []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/projects/proj-1/tasks":
			_ = json.NewEncoder(w).Encode(Task{ID: "task-1", Title: "Bridge rollout"})
		case "/api/v1/agents/spawn":
			_ = json.NewEncoder(w).Encode(TaskDispatchResponse{
				Task:     Task{ID: "task-1"},
				Dispatch: DispatchOutcome{Status: "started", Run: &AgentRun{ID: "run-1"}},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj-1", "secret")
	result, err := client.QuickAgentRun(context.Background(), "Bridge rollout")
	if err != nil {
		t.Fatalf("QuickAgentRun error: %v", err)
	}
	if len(calls) != 2 || calls[0] != "/api/v1/projects/proj-1/tasks" || calls[1] != "/api/v1/agents/spawn" {
		t.Fatalf("calls = %+v", calls)
	}
	if result.Dispatch.Run == nil || result.Dispatch.Run.ID != "run-1" {
		t.Fatalf("result = %+v", result)
	}
}

func TestQuickAgentRun_WrapsCreateFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "create failed", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj-1", "secret")
	_, err := client.QuickAgentRun(context.Background(), "Bridge rollout")
	if err == nil || !strings.Contains(err.Error(), "create task") {
		t.Fatalf("err = %v", err)
	}
}

func TestGetAgentLogs_ParsesEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents/run-1/logs" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]AgentLogEntry{{Timestamp: "now", Type: "info", Content: "started"}})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj-1", "secret")
	logs, err := client.GetAgentLogs(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("GetAgentLogs error: %v", err)
	}
	if len(logs) != 1 || logs[0].Content != "started" {
		t.Fatalf("logs = %+v", logs)
	}
}

func TestGetAgentRunAndLifecycleActions_UseCanonicalEndpoints(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AgentRunSummary{
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

	client := NewAgentForgeClient(server.URL, "proj-1", "secret")

	run, err := client.GetAgentRun(context.Background(), "run-123")
	if err != nil {
		t.Fatalf("GetAgentRun error: %v", err)
	}
	if run.TaskTitle != "Bridge rollout" || run.Status != "paused" {
		t.Fatalf("run = %+v", run)
	}

	for _, action := range []struct {
		name string
		call func(context.Context, string) (*AgentRunSummary, error)
	}{
		{name: "pause", call: client.PauseAgentRun},
		{name: "resume", call: client.ResumeAgentRun},
		{name: "kill", call: client.KillAgentRun},
	} {
		got, err := action.call(context.Background(), "run-123")
		if err != nil {
			t.Fatalf("%s error: %v", action.name, err)
		}
		if got.ID != "run-123" {
			t.Fatalf("%s returned %+v", action.name, got)
		}
	}

	wantCalls := []string{
		"GET /api/v1/agents/run-123",
		"POST /api/v1/agents/run-123/pause",
		"POST /api/v1/agents/run-123/resume",
		"POST /api/v1/agents/run-123/kill",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %+v, want %+v", calls, wantCalls)
	}
}

func TestTransitionTaskStatus_UsesTransitionEndpoint(t *testing.T) {
	var gotPath string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Task{
			ID:       "task-123",
			Title:    "Bridge rollout",
			Status:   "done",
			Priority: "high",
		})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj-1", "secret")
	task, err := client.TransitionTaskStatus(context.Background(), "task-123", "done")
	if err != nil {
		t.Fatalf("TransitionTaskStatus error: %v", err)
	}
	if gotPath != "/api/v1/tasks/task-123/transition" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["status"] != "done" {
		t.Fatalf("body = %+v", gotBody)
	}
	if task.Status != "done" {
		t.Fatalf("task = %+v", task)
	}
}

func TestQueueAndMemoryEndpoints_ParseCanonicalResponses(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/proj-1/queue":
			_ = json.NewEncoder(w).Encode([]QueueEntry{
				{EntryID: "entry-1", TaskID: "task-1", MemberID: "member-1", Status: "queued", Priority: 20, Reason: "agent pool is at capacity"},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/projects/proj-1/queue/entry-1":
			_ = json.NewEncoder(w).Encode(QueueEntry{
				EntryID: "entry-1", TaskID: "task-1", MemberID: "member-1", Status: "cancelled", Priority: 20, Reason: "manual_cancel",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/proj-1/memory":
			_ = json.NewEncoder(w).Encode([]MemoryEntry{
				{ID: "mem-1", Key: "release-plan", Content: "Coordinate deployment in phases", Category: "semantic", Scope: "project"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/projects/proj-1/memory":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if payload["scope"] != "project" || payload["category"] != "operator_note" {
				t.Fatalf("payload = %+v", payload)
			}
			_ = json.NewEncoder(w).Encode(MemoryEntry{
				ID: "mem-2", Key: "operator-note", Content: "Remember to reuse Codex", Category: "operator_note", Scope: "project",
			})
		default:
			t.Fatalf("unexpected request: %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj-1", "secret")

	queue, err := client.ListQueueEntries(context.Background(), "queued")
	if err != nil {
		t.Fatalf("ListQueueEntries error: %v", err)
	}
	if len(queue) != 1 || queue[0].EntryID != "entry-1" {
		t.Fatalf("queue = %+v", queue)
	}

	cancelled, err := client.CancelQueueEntry(context.Background(), "entry-1", "manual_cancel")
	if err != nil {
		t.Fatalf("CancelQueueEntry error: %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("cancelled = %+v", cancelled)
	}

	memories, err := client.SearchProjectMemory(context.Background(), "release", 5)
	if err != nil {
		t.Fatalf("SearchProjectMemory error: %v", err)
	}
	if len(memories) != 1 || memories[0].ID != "mem-1" {
		t.Fatalf("memories = %+v", memories)
	}

	created, err := client.StoreProjectMemoryNote(context.Background(), "operator-note", "Remember to reuse Codex")
	if err != nil {
		t.Fatalf("StoreProjectMemoryNote error: %v", err)
	}
	if created.Category != "operator_note" || created.Scope != "project" {
		t.Fatalf("created = %+v", created)
	}

	wantCalls := []string{
		"GET /api/v1/projects/proj-1/queue?status=queued",
		"DELETE /api/v1/projects/proj-1/queue/entry-1?reason=manual_cancel",
		"GET /api/v1/projects/proj-1/memory?limit=5&q=release",
		"POST /api/v1/projects/proj-1/memory?",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %+v, want %+v", calls, wantCalls)
	}
}

func TestGetSprintBurndown_PropagatesAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "burndown failed", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj-1", "secret")
	_, err := client.GetSprintBurndown(context.Background(), "sprint-1")
	if err == nil || !strings.Contains(err.Error(), "API error 502") {
		t.Fatalf("err = %v", err)
	}
}
