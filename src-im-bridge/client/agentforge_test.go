package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		ParentTask Task                   `json:"parentTask"`
		Summary    string                 `json:"summary"`
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
