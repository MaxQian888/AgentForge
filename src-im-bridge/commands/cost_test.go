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

func TestCostCommand_RepliesWithTextWithoutCardSupport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/proj/costs" {
			t.Fatalf("path = %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&client.CostStats{
			TotalUsd:   12.34,
			BudgetUsd:  50,
			DailyUsd:   1.2,
			WeeklyUsd:  4.5,
			MonthlyUsd: 12.34,
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterCostCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/cost",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "总费用: $12.34 / $50.00") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
}

func TestCostCommand_RepliesWithCardWhenSupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&client.CostStats{
			TotalUsd:   12.34,
			BudgetUsd:  50,
			DailyUsd:   1.2,
			WeeklyUsd:  4.5,
			MonthlyUsd: 12.34,
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskCardPlatform{}
	engine := core.NewEngine(platform)
	RegisterCostCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/cost",
	})

	if len(platform.cards) != 1 {
		t.Fatalf("cards len = %d, want 1", len(platform.cards))
	}
	if platform.cards[0].Title != "费用统计" {
		t.Fatalf("card title = %q", platform.cards[0].Title)
	}
	if len(platform.cards[0].Fields) != 5 {
		t.Fatalf("fields = %+v", platform.cards[0].Fields)
	}
}

func TestCostCommand_RepliesWithFailureWhenAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterCostCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/cost",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "获取费用统计失败") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
}
