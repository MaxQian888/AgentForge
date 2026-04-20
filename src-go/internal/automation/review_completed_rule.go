// Package automation implements event-driven automation rules that fire
// in response to domain events (e.g. review.completed).
package automation

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Severity ordering for threshold checks.
var severityOrder = map[string]int{
	"critical": 4,
	"high":     3,
	"medium":   2,
	"low":      1,
	"info":     0,
}

func severityAtLeast(actual, threshold string) bool {
	return severityOrder[actual] >= severityOrder[threshold]
}

// ReviewCompletedEvent carries the data the rule needs.
type ReviewCompletedEvent struct {
	ReviewID       uuid.UUID
	ProjectID      uuid.UUID
	HeadBranch     string
	IMReplyTarget  map[string]any
	Findings       []FindingSummary
}

// FindingSummary is the subset of a finding the rule evaluates.
type FindingSummary struct {
	ID             uuid.UUID
	Severity       string
	Message        string
	Suggestion     string
	SuggestedPatch string
	File           string
	Line           int
	Sources        []string
}

// ProjectConfig carries project-level automation settings.
type ProjectConfig struct {
	AutomationDecision string // "auto_send" | "manual_only"
	AutoCardThreshold  string // severity threshold (e.g. "medium")
}

// CardAction represents an action button on an IM card.
type CardAction struct {
	ID    string
	Type  string // "callback" | "url"
	Label string
	URL   string // only for type "url"
}

// Card is the provider-neutral card payload sent to the IM bridge.
type Card struct {
	Title   string
	Summary string
	Fields  map[string]string
	Actions []CardAction
}

// CorrelationInput for minting correlation tokens.
type CorrelationInput struct {
	ExecutionID *uuid.UUID
	NodeID      string
	ActionID    string
	Payload     map[string]any
}

// CorrelationMinter mints correlation tokens for card action callbacks.
type CorrelationMinter interface {
	Mint(ctx context.Context, input CorrelationInput) (token string, err error)
}

// CardSender sends action cards to IM channels.
type CardSender interface {
	SendActionCard(ctx context.Context, replyTarget map[string]any, card Card) error
}

// ReviewCompletedRule evaluates review.completed events and emits
// interactive IM cards for actionable findings above threshold.
type ReviewCompletedRule struct {
	sender CardSender
	corr   CorrelationMinter
}

// NewReviewCompletedRule creates a new rule instance.
func NewReviewCompletedRule(sender CardSender, corr CorrelationMinter) *ReviewCompletedRule {
	return &ReviewCompletedRule{sender: sender, corr: corr}
}

// Handle evaluates the event. Returns nil if the rule doesn't apply.
func (r *ReviewCompletedRule) Handle(ctx context.Context, event ReviewCompletedEvent, project ProjectConfig) error {
	// Skip fix branches to prevent infinite loops (§9 boundary policy)
	if strings.HasPrefix(event.HeadBranch, "fix/") {
		return nil
	}

	// Respect project automation setting
	if project.AutomationDecision == "manual_only" {
		return nil
	}

	threshold := project.AutoCardThreshold
	if threshold == "" {
		threshold = "medium"
	}

	for _, f := range event.Findings {
		if !severityAtLeast(f.Severity, threshold) {
			continue
		}
		if f.SuggestedPatch == "" && f.Suggestion == "" {
			continue
		}

		card := r.buildCard(f, event.ReviewID)

		// Mint correlation tokens for callback actions
		for i, action := range card.Actions {
			if action.Type != "callback" {
				continue
			}
			tok, err := r.corr.Mint(ctx, CorrelationInput{
				ExecutionID: nil, // null — automation branch
				NodeID:      "(automation)",
				ActionID:    action.ID,
				Payload: map[string]any{
					"finding_id": f.ID.String(),
					"action":     action.ID,
				},
			})
			if err != nil {
				return fmt.Errorf("mint correlation for action %s: %w", action.ID, err)
			}
			card.Actions[i].URL = tok // token used as callback reference
		}

		if err := r.sender.SendActionCard(ctx, event.IMReplyTarget, card); err != nil {
			return fmt.Errorf("send card for finding %s: %w", f.ID, err)
		}
	}
	return nil
}

func (r *ReviewCompletedRule) buildCard(f FindingSummary, reviewID uuid.UUID) Card {
	msg := f.Message
	if len(msg) > 60 {
		msg = msg[:60]
	}

	summary := f.Suggestion
	if summary == "" {
		summary = f.SuggestedPatch
	}
	if len(summary) > 240 {
		summary = summary[:240]
	}

	source := ""
	if len(f.Sources) > 0 {
		source = f.Sources[0]
	}

	fileLine := f.File
	if f.Line > 0 {
		fileLine = fmt.Sprintf("%s:%d", f.File, f.Line)
	}

	return Card{
		Title:   fmt.Sprintf("Apply fix? \u2014 %s", msg),
		Summary: summary,
		Fields: map[string]string{
			"file":     fileLine,
			"severity": f.Severity,
			"source":   source,
		},
		Actions: []CardAction{
			{ID: "apply", Type: "callback", Label: "Apply"},
			{ID: "dismiss", Type: "callback", Label: "Dismiss"},
			{ID: "view", Type: "url", Label: "Open in AgentForge",
				URL: fmt.Sprintf("/reviews/%s#%s", reviewID.String(), f.ID.String())},
		},
	}
}
