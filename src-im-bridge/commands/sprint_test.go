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

func TestSprintCommand_EmptyArgsShowsUsage(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterSprintCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/sprint",
	})

	if len(platform.replies) != 1 || platform.replies[0] != sprintUsage {
		t.Fatalf("replies = %v", platform.replies)
	}
}

func TestSprintCommand_UnknownSubcommandShowsUsage(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterSprintCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/sprint unknown",
	})

	if len(platform.replies) != 1 || platform.replies[0] != sprintUsage {
		t.Fatalf("replies = %v", platform.replies)
	}
}

func TestSprintCommand_StatusRepliesWithCard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/proj/sprints" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("status"); got != "active" {
			t.Fatalf("status = %q, want active", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]client.Sprint{
			{
				ID:             "sprint-123",
				Name:           "Sprint 7",
				StartDate:      "2026-03-17",
				EndDate:        "2026-03-31",
				Status:         "active",
				TotalBudgetUsd: 100.0,
				SpentUsd:       42.5,
			},
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskCardPlatform{}
	engine := core.NewEngine(platform)
	RegisterSprintCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/sprint status",
	})

	if len(platform.cards) != 1 {
		t.Fatalf("cards len = %d, want 1", len(platform.cards))
	}
	card := platform.cards[0]
	if card.Title != "Sprint: Sprint 7" {
		t.Fatalf("card title = %q", card.Title)
	}
	if len(card.Fields) != 4 {
		t.Fatalf("fields len = %d, want 4, fields = %+v", len(card.Fields), card.Fields)
	}
}

func TestSprintCommand_StatusRepliesWithTextWithoutCardSupport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]client.Sprint{
			{
				ID:             "sprint-123",
				Name:           "Sprint 7",
				StartDate:      "2026-03-17",
				EndDate:        "2026-03-31",
				Status:         "active",
				TotalBudgetUsd: 100.0,
				SpentUsd:       42.5,
			},
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterSprintCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/sprint status",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "Sprint: Sprint 7") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[0], "$42.50 / $100.00") {
		t.Fatalf("reply = %q, want budget info", platform.replies[0])
	}
}

func TestSprintCommand_StatusNoActiveSprint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]client.Sprint{})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterSprintCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/sprint status",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "获取 Sprint 失败") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
}

func TestSprintCommand_BurndownRepliesWithText(t *testing.T) {
	reqCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/projects/proj/sprints":
			_ = json.NewEncoder(w).Encode([]client.Sprint{
				{
					ID:             "sprint-123",
					Name:           "Sprint 7",
					StartDate:      "2026-03-17",
					EndDate:        "2026-03-31",
					Status:         "active",
					TotalBudgetUsd: 100.0,
					SpentUsd:       42.5,
				},
			})
		case r.URL.Path == "/api/v1/sprints/sprint-123/burndown":
			reqCount++
			_ = json.NewEncoder(w).Encode(&client.SprintMetrics{
				Sprint: client.Sprint{
					ID:        "sprint-123",
					Name:      "Sprint 7",
					StartDate: "2026-03-17",
					EndDate:   "2026-03-31",
				},
				PlannedTasks:    10,
				CompletedTasks:  6,
				RemainingTasks:  4,
				CompletionRate:  0.6,
				VelocityPerWeek: 3.0,
				Burndown: []client.BurndownPoint{
					{Date: "2026-03-17", RemainingTasks: 10, CompletedTasks: 0},
					{Date: "2026-03-20", RemainingTasks: 7, CompletedTasks: 3},
					{Date: "2026-03-24", RemainingTasks: 4, CompletedTasks: 6},
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
	RegisterSprintCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/sprint burndown",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	reply := platform.replies[0]
	if !strings.Contains(reply, "Sprint: Sprint 7") {
		t.Fatalf("reply = %q, want sprint name", reply)
	}
	if !strings.Contains(reply, "6/10 任务 (60%)") {
		t.Fatalf("reply = %q, want progress info", reply)
	}
	if !strings.Contains(reply, "燃尽图") {
		t.Fatalf("reply = %q, want burndown chart", reply)
	}
	if !strings.Contains(reply, "█") {
		t.Fatalf("reply = %q, want bar chart characters", reply)
	}
	if reqCount != 1 {
		t.Fatalf("burndown API called %d times, want 1", reqCount)
	}
}

func TestSprintCommand_BurndownAPIFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/projects/proj/sprints":
			_ = json.NewEncoder(w).Encode([]client.Sprint{
				{ID: "sprint-123", Name: "Sprint 7", Status: "active"},
			})
		case r.URL.Path == "/api/v1/sprints/sprint-123/burndown":
			http.Error(w, "internal error", http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterSprintCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/sprint burndown",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "获取燃尽图失败") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
}
