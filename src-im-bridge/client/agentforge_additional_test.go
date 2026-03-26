package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
