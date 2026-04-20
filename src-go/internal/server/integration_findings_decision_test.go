package server

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/automation"
	"github.com/react-go-quick-starter/server/internal/imcards"
)

// --- integration stubs ---

type intCardSender struct {
	cards []automation.Card
}

func (s *intCardSender) SendActionCard(_ context.Context, _ map[string]any, card automation.Card) error {
	s.cards = append(s.cards, card)
	return nil
}

type intCorrMinter struct {
	minted []automation.CorrelationInput
	tokens []string
}

func (s *intCorrMinter) Mint(_ context.Context, input automation.CorrelationInput) (string, error) {
	tok := uuid.New().String()
	s.minted = append(s.minted, input)
	s.tokens = append(s.tokens, tok)
	return tok, nil
}

type intAutomationHandler struct {
	calls []struct {
		FindingID uuid.UUID
		Action    string
		Actor     string
	}
}

func (h *intAutomationHandler) Decide(_ context.Context, findingID uuid.UUID, action, actor string) error {
	h.calls = append(h.calls, struct {
		FindingID uuid.UUID
		Action    string
		Actor     string
	}{findingID, action, actor})
	return nil
}

// TestIntegration_FindingsDecisionLoop verifies the end-to-end flow:
// review.completed → automation card → click Apply → spawn execution.
func TestIntegration_FindingsDecisionLoop(t *testing.T) {
	// Step 1: Create test fixtures
	findingID := uuid.New()
	reviewID := uuid.New()
	projectID := uuid.New()

	event := automation.ReviewCompletedEvent{
		ReviewID:      reviewID,
		ProjectID:     projectID,
		HeadBranch:    "feature/foo",
		IMReplyTarget: map[string]any{"chat_id": "C123"},
		Findings: []automation.FindingSummary{
			{
				ID:             findingID,
				Severity:       "high",
				Message:        "Null pointer dereference in handler.",
				Suggestion:     "Add nil check.",
				SuggestedPatch: "--- a/handler.go\n+++ b/handler.go\n@@ -10 +10 @@\n-x\n+if x != nil { x.Do() }\n",
				File:           "handler.go",
				Line:           10,
				Sources:        []string{"plugin.security"},
			},
		},
	}

	project := automation.ProjectConfig{
		AutomationDecision: "auto_send",
		AutoCardThreshold:  "medium",
	}

	// Step 2: Run the automation rule
	sender := &intCardSender{}
	corr := &intCorrMinter{}
	rule := automation.NewReviewCompletedRule(sender, corr)

	err := rule.Handle(context.Background(), event, project)
	if err != nil {
		t.Fatalf("rule.Handle: %v", err)
	}

	// Step 3: Verify card was sent
	if len(sender.cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(sender.cards))
	}

	// Step 4: Verify correlations minted with nil ExecutionID
	applyMints := 0
	for _, m := range corr.minted {
		if m.ExecutionID != nil {
			t.Errorf("expected nil ExecutionID, got %v", m.ExecutionID)
		}
		if m.Payload["finding_id"] != findingID.String() {
			t.Errorf("finding_id = %v, want %s", m.Payload["finding_id"], findingID.String())
		}
		if m.Payload["action"] == "apply" {
			applyMints++
		}
	}
	if applyMints != 1 {
		t.Errorf("expected 1 apply mint, got %d", applyMints)
	}

	// Step 5: Simulate user click by routing through card_action_router
	// Build a correlation as if it was stored
	token := uuid.New()
	stubCorr := &stubIntCorrelations{
		c: &imcards.Correlation{
			Token:       token,
			ExecutionID: uuid.Nil, // null → automation branch
			NodeID:      "(automation)",
			ActionID:    "apply",
			Payload:     map[string]any{"finding_id": findingID.String(), "action": "apply"},
			ExpiresAt:   time.Now().Add(time.Hour),
		},
	}

	autoHandler := &intAutomationHandler{}
	router := &imcards.Router{
		Correlations: stubCorr,
		Resumer:      &stubIntResumer{},
		Fallback:     nil,
		Audit:        &stubIntAudit{},
		Automation:   autoHandler,
	}

	result, err := router.Route(context.Background(), imcards.RouteInput{
		Token:    token,
		ActionID: "apply",
		UserID:   "user-123",
	})
	if err != nil {
		t.Fatalf("router.Route: %v", err)
	}
	if result.Outcome != imcards.OutcomeAutomationDispatched {
		t.Errorf("outcome = %s, want automation_dispatched", result.Outcome)
	}

	// Step 6: Verify automation handler was called
	if len(autoHandler.calls) != 1 {
		t.Fatalf("automation handler calls = %d, want 1", len(autoHandler.calls))
	}
	if autoHandler.calls[0].FindingID != findingID {
		t.Errorf("finding_id mismatch")
	}
	if autoHandler.calls[0].Action != "apply" {
		t.Errorf("action = %q, want apply", autoHandler.calls[0].Action)
	}
	if autoHandler.calls[0].Actor != "user-123" {
		t.Errorf("actor = %q, want user-123", autoHandler.calls[0].Actor)
	}
}

// integration test stubs for router
type stubIntCorrelations struct {
	c *imcards.Correlation
}

func (s *stubIntCorrelations) Lookup(_ context.Context, _ uuid.UUID) (*imcards.Correlation, error) {
	return s.c, nil
}
func (s *stubIntCorrelations) MarkConsumed(_ context.Context, _ uuid.UUID) error {
	return nil
}

type stubIntResumer struct{}

func (s *stubIntResumer) Resume(_ context.Context, _ uuid.UUID, _ string, _ map[string]any) error {
	return nil
}

type stubIntAudit struct{}

func (s *stubIntAudit) Record(_ context.Context, _ string, _ map[string]any) error {
	return nil
}
