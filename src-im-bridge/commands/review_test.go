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

func TestReviewCommand_EmptyArgsShowsUsage(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterReviewCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/review",
	})

	if len(platform.replies) != 1 || platform.replies[0] != reviewUsage {
		t.Fatalf("replies = %v", platform.replies)
	}
}

func TestReviewCommand_TriggerReviewRepliesWithCard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/reviews/trigger" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-IM-Source"); got != "slack" {
			t.Fatalf("X-IM-Source = %q, want slack", got)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["prUrl"] != "https://github.com/org/repo/pull/42" {
			t.Fatalf("prUrl = %q", body["prUrl"])
		}
		if body["projectId"] != "proj" {
			t.Fatalf("projectId = %q", body["projectId"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(&client.Review{
			ID:             "review-12345678",
			PRURL:          "https://github.com/org/repo/pull/42",
			Status:         "pending",
			RiskLevel:      "medium",
			Summary:        "需要关注安全问题",
			Recommendation: "建议修改鉴权逻辑",
			CostUSD:        0.35,
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskCardPlatform{}
	engine := core.NewEngine(platform)
	RegisterReviewCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/review https://github.com/org/repo/pull/42",
	})

	// First reply is a processing hint (card or text), followed by the review card.
	// With CardSender, both the processing hint and the review result may render
	// as cards, so we check the last card has review content.
	if len(platform.cards) == 0 {
		t.Fatalf("expected at least 1 card, got 0; replies = %v", platform.replies)
	}
	card := platform.cards[len(platform.cards)-1]
	if !strings.Contains(card.Title, "代码审查") {
		t.Fatalf("card title = %q, want review card", card.Title)
	}
	if len(card.Fields) < 3 {
		t.Fatalf("fields = %+v", card.Fields)
	}
}

func TestReviewCommand_TriggerReviewRepliesWithTextWithoutCardSupport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(&client.Review{
			ID:     "review-12345678",
			PRURL:  "https://github.com/org/repo/pull/42",
			Status: "pending",
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterReviewCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/review https://github.com/org/repo/pull/42",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[1], "已创建代码审查") {
		t.Fatalf("reply = %q", platform.replies[1])
	}
	if !strings.Contains(platform.replies[1], "代码审查 #review-1") {
		t.Fatalf("reply = %q", platform.replies[1])
	}
}

func TestReviewCommand_TriggerReviewFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"bad request"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterReviewCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/review https://github.com/org/repo/pull/42",
	})

	// Expect processing hint + error reply.
	if len(platform.replies) < 2 {
		t.Fatalf("replies count = %d, want >= 2; replies = %v", len(platform.replies), platform.replies)
	}
	lastReply := platform.replies[len(platform.replies)-1]
	if !strings.Contains(lastReply, "bad request") && !strings.Contains(lastReply, "触发审查失败") {
		t.Fatalf("reply = %q, want error content", lastReply)
	}
}

func TestReviewCommand_StatusRequiresID(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterReviewCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/review status",
	})

	if len(platform.replies) != 1 || platform.replies[0] != "用法: /review status <review-id>" {
		t.Fatalf("replies = %v", platform.replies)
	}
}

func TestReviewCommand_StatusRepliesWithCard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/reviews/review-123" {
			t.Fatalf("path = %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&client.Review{
			ID:             "review-12345678",
			PRURL:          "https://github.com/org/repo/pull/42",
			Status:         "completed",
			RiskLevel:      "low",
			Summary:        "代码质量良好",
			Recommendation: "可以合并",
			CostUSD:        0.50,
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskCardPlatform{}
	engine := core.NewEngine(platform)
	RegisterReviewCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/review status review-123",
	})

	if len(platform.cards) != 1 {
		t.Fatalf("cards len = %d, want 1", len(platform.cards))
	}
	card := platform.cards[0]
	if card.Title != "代码审查 #review-1" {
		t.Fatalf("card title = %q", card.Title)
	}
	// Fields: PR, 状态, 风险等级, 摘要, 建议, 费用
	if len(card.Fields) != 6 {
		t.Fatalf("fields len = %d, want 6, fields = %+v", len(card.Fields), card.Fields)
	}
}

func TestReviewCommand_StatusFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterReviewCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/review status review-notfound",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "获取审查失败") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
}

func TestReviewCommand_StatusSuggestsFollowUpTasksFromFindings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&client.Review{
			ID:             "review-12345678",
			PRURL:          "https://github.com/org/repo/pull/42",
			Status:         "completed",
			RiskLevel:      "high",
			Summary:        "发现 2 个问题",
			Recommendation: "request_changes",
			Findings: []client.ReviewFinding{
				{Severity: "high", Message: "Missing auth guard", File: "src-go/internal/service/review_service.go"},
				{Severity: "medium", Message: "Rename unclear variable"},
			},
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterReviewCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/review status review-123",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	for _, want := range []string{"后续任务建议", "/task create 修复审查问题", "Missing auth guard"} {
		if !strings.Contains(platform.replies[0], want) {
			t.Fatalf("reply = %q, want substring %q", platform.replies[0], want)
		}
	}
}

func TestReviewCommand_DeepCallsStandaloneEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/reviews/trigger" {
			t.Fatalf("path = %s", r.URL.Path)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["trigger"] != "manual" {
			t.Fatalf("trigger = %q, want manual", body["trigger"])
		}
		if body["prUrl"] != "https://github.com/org/repo/pull/55" {
			t.Fatalf("prUrl = %q", body["prUrl"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&client.Review{
			ID:             "review-55555555",
			TaskID:         "",
			PRURL:          "https://github.com/org/repo/pull/55",
			Status:         "pending",
			RiskLevel:      "low",
			Recommendation: "approve",
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterReviewCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/review deep https://github.com/org/repo/pull/55",
	})

	if len(platform.replies) < 2 {
		t.Fatalf("replies count = %d, want >= 2; replies = %v", len(platform.replies), platform.replies)
	}
	if !strings.Contains(platform.replies[0], "深度审查") {
		t.Fatalf("first reply = %q, want processing hint", platform.replies[0])
	}
	lastReply := platform.replies[len(platform.replies)-1]
	if !strings.Contains(lastReply, "review-5555") && !strings.Contains(lastReply, "approve") {
		t.Fatalf("last reply = %q, want review content", lastReply)
	}
}

func TestReviewCommand_ApproveAndRequestChangesSubcommands(t *testing.T) {
	calls := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/reviews/review-1/approve":
			_ = json.NewEncoder(w).Encode(&client.Review{
				ID:             "review-1",
				Status:         "completed",
				PRURL:          "https://github.com/org/repo/pull/1",
				Recommendation: "approve",
			})
		case "/api/v1/reviews/review-1/request-changes":
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request-changes body: %v", err)
			}
			if payload["comment"] != "Need more tests" {
				t.Fatalf("comment = %q", payload["comment"])
			}
			_ = json.NewEncoder(w).Encode(&client.Review{
				ID:             "review-1",
				Status:         "completed",
				PRURL:          "https://github.com/org/repo/pull/1",
				Recommendation: "request_changes",
			})
		default:
			t.Fatalf("unexpected path = %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterReviewCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/review approve review-1",
	})
	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/review request-changes review-1 Need more tests",
	})

	if len(calls) != 2 {
		t.Fatalf("calls = %v", calls)
	}
	if calls[0] != "POST /api/v1/reviews/review-1/approve" {
		t.Fatalf("first call = %q", calls[0])
	}
	if calls[1] != "POST /api/v1/reviews/review-1/request-changes" {
		t.Fatalf("second call = %q", calls[1])
	}
}

func TestBuildReviewCard_AddsPendingHumanButtonsOnly(t *testing.T) {
	pendingCard := buildReviewCard(&client.Review{
		ID:             "review-pending",
		PRURL:          "https://example.test/pr/2",
		Status:         "pending_human",
		RiskLevel:      "high",
		Recommendation: "approve",
	})
	if len(pendingCard.Buttons) != 3 {
		t.Fatalf("pending buttons = %+v, want 3", pendingCard.Buttons)
	}
	if pendingCard.Buttons[1].Action != "act:approve:review-pending" {
		t.Fatalf("approve action = %q", pendingCard.Buttons[1].Action)
	}
	if pendingCard.Buttons[2].Action != "act:request-changes:review-pending" {
		t.Fatalf("request-changes action = %q", pendingCard.Buttons[2].Action)
	}

	completedCard := buildReviewCard(&client.Review{
		ID:             "review-complete",
		PRURL:          "https://example.test/pr/3",
		Status:         "completed",
		RiskLevel:      "low",
		Recommendation: "approve",
	})
	if len(completedCard.Buttons) != 1 {
		t.Fatalf("completed buttons = %+v, want details only", completedCard.Buttons)
	}
}

func TestBuildReviewCard_OmitsEmptyOptionalFields(t *testing.T) {
	card := buildReviewCard(&client.Review{
		ID:        "review-12345678",
		PRURL:     "https://example.test/pr/1",
		Status:    "pending",
		RiskLevel: "medium",
	})

	if card.Title != "代码审查 #review-1" {
		t.Fatalf("title = %q", card.Title)
	}
	if len(card.Fields) != 3 {
		t.Fatalf("fields = %+v", card.Fields)
	}
	if len(card.Buttons) != 1 || card.Buttons[0].Action != "link:/reviews/review-12345678" {
		t.Fatalf("buttons = %+v", card.Buttons)
	}
}

func TestBuildReviewCard_IncludesFollowUpTaskSuggestionsForCompletedFindings(t *testing.T) {
	card := buildReviewCard(&client.Review{
		ID:             "review-12345678",
		PRURL:          "https://example.test/pr/1",
		Status:         "completed",
		RiskLevel:      "high",
		Recommendation: "request_changes",
		Findings: []client.ReviewFinding{
			{Severity: "high", Message: "Missing auth guard"},
		},
	})

	found := false
	for _, field := range card.Fields {
		if field.Label == "后续任务" {
			found = true
			if !strings.Contains(field.Value, "/task create 修复审查问题") {
				t.Fatalf("field.Value = %q", field.Value)
			}
		}
	}
	if !found {
		t.Fatalf("fields = %+v", card.Fields)
	}
}
