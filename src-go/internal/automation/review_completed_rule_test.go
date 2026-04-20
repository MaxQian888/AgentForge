package automation

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type fakeCardSender struct {
	cards []Card
}

func (f *fakeCardSender) SendActionCard(_ context.Context, _ map[string]any, card Card) error {
	f.cards = append(f.cards, card)
	return nil
}

type fakeCorrMinter struct {
	minted []CorrelationInput
}

func (f *fakeCorrMinter) Mint(_ context.Context, input CorrelationInput) (string, error) {
	f.minted = append(f.minted, input)
	return "tok-" + input.ActionID, nil
}

func baseEvent() ReviewCompletedEvent {
	return ReviewCompletedEvent{
		ReviewID:      uuid.New(),
		ProjectID:     uuid.New(),
		HeadBranch:    "feature/foo",
		IMReplyTarget: map[string]any{"chat_id": "C1"},
		Findings: []FindingSummary{
			{
				ID:             uuid.New(),
				Severity:       "high",
				Message:        "Use-after-free in handler loop.",
				Suggestion:     "Add nil check before dereference.",
				SuggestedPatch: "--- a/foo\n+++ b/foo\n@@ -1 +1 @@\n-x\n+y\n",
				File:           "internal/handler.go",
				Line:           42,
				Sources:        []string{"plugin.security"},
			},
		},
	}
}

func TestRule_SkipsFixBranches(t *testing.T) {
	sender := &fakeCardSender{}
	corr := &fakeCorrMinter{}
	rule := NewReviewCompletedRule(sender, corr)

	event := baseEvent()
	event.HeadBranch = "fix/abc/def"

	err := rule.Handle(context.Background(), event, ProjectConfig{AutomationDecision: "auto_send", AutoCardThreshold: "medium"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.cards) != 0 {
		t.Errorf("expected 0 cards, got %d", len(sender.cards))
	}
}

func TestRule_EmitsCardForActionableFinding(t *testing.T) {
	sender := &fakeCardSender{}
	corr := &fakeCorrMinter{}
	rule := NewReviewCompletedRule(sender, corr)

	event := baseEvent()
	err := rule.Handle(context.Background(), event, ProjectConfig{AutomationDecision: "auto_send", AutoCardThreshold: "medium"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(sender.cards))
	}
	card := sender.cards[0]
	if card.Fields["severity"] != "high" {
		t.Errorf("severity = %q", card.Fields["severity"])
	}
	if card.Fields["file"] != "internal/handler.go:42" {
		t.Errorf("file = %q", card.Fields["file"])
	}
	if len(card.Actions) != 3 {
		t.Errorf("actions = %d, want 3", len(card.Actions))
	}
}

func TestRule_SkipsBelowThreshold(t *testing.T) {
	sender := &fakeCardSender{}
	corr := &fakeCorrMinter{}
	rule := NewReviewCompletedRule(sender, corr)

	event := baseEvent()
	event.Findings[0].Severity = "low"

	err := rule.Handle(context.Background(), event, ProjectConfig{AutomationDecision: "auto_send", AutoCardThreshold: "medium"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.cards) != 0 {
		t.Errorf("expected 0 cards for low severity, got %d", len(sender.cards))
	}
}

func TestRule_SkipsNonActionable(t *testing.T) {
	sender := &fakeCardSender{}
	corr := &fakeCorrMinter{}
	rule := NewReviewCompletedRule(sender, corr)

	event := baseEvent()
	event.Findings[0].Suggestion = ""
	event.Findings[0].SuggestedPatch = ""

	err := rule.Handle(context.Background(), event, ProjectConfig{AutomationDecision: "auto_send", AutoCardThreshold: "medium"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.cards) != 0 {
		t.Errorf("expected 0 cards for non-actionable, got %d", len(sender.cards))
	}
}

func TestRule_AutomationDecisionManualOnly(t *testing.T) {
	sender := &fakeCardSender{}
	corr := &fakeCorrMinter{}
	rule := NewReviewCompletedRule(sender, corr)

	event := baseEvent()
	err := rule.Handle(context.Background(), event, ProjectConfig{AutomationDecision: "manual_only", AutoCardThreshold: "medium"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.cards) != 0 {
		t.Errorf("expected 0 cards for manual_only, got %d", len(sender.cards))
	}
}

func TestRule_MintsCorrelationWithNullExecutionID(t *testing.T) {
	sender := &fakeCardSender{}
	corr := &fakeCorrMinter{}
	rule := NewReviewCompletedRule(sender, corr)

	event := baseEvent()
	err := rule.Handle(context.Background(), event, ProjectConfig{AutomationDecision: "auto_send", AutoCardThreshold: "medium"})
	if err != nil {
		t.Fatal(err)
	}

	// Two callback actions: apply + dismiss
	callbackCount := 0
	for _, m := range corr.minted {
		if m.ExecutionID != nil {
			t.Errorf("expected nil ExecutionID, got %v", m.ExecutionID)
		}
		if m.NodeID != "(automation)" {
			t.Errorf("NodeID = %q, want (automation)", m.NodeID)
		}
		callbackCount++
	}
	if callbackCount != 2 {
		t.Errorf("callback correlations = %d, want 2", callbackCount)
	}
}
