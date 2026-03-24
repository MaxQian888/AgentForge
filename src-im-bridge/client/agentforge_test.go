package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/platform/discord"
	"github.com/agentforge/im-bridge/platform/telegram"
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
	if gotPath != "/api/v1/tasks" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotBody["title"] != "Bridge rollout" || gotBody["description"] != "desc" || gotBody["project_id"] != "proj" {
		t.Fatalf("body = %+v", gotBody)
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
	var gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	if gotQuery != "project_id=proj&status=triaged" {
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

func TestGetCostStats_ParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CostStats{
			TotalUsd:   11.1,
			BudgetUsd:  20.2,
			DailyUsd:   1.1,
			WeeklyUsd:  4.4,
			MonthlyUsd: 11.1,
		})
	}))
	defer server.Close()

	client := NewAgentForgeClient(server.URL, "proj", "secret")

	stats, err := client.GetCostStats(context.Background())
	if err != nil {
		t.Fatalf("GetCostStats error: %v", err)
	}
	if stats.TotalUsd != 11.1 || stats.BudgetUsd != 20.2 {
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
