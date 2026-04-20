// Package imcards implements the card-action correlation store and routing
// logic for interactive IM card buttons dispatched by workflow im_send nodes.
//
// Coupling note: the string-comparison fallback for
// "wait_event: target node is not waiting" is intentional to keep the
// imcards package free of a backward import on nodetypes. If the constant
// ever changes, the router test must also change.
package imcards

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Router-level errors. Each maps to a stable HTTP code via the handler
// so the IM Bridge can render the right toast to the end user.
var (
	ErrCardActionExpired   = errors.New("card_action: expired")
	ErrCardActionConsumed  = errors.New("card_action: consumed")
	ErrExecutionNotWaiting = errors.New("card_action: execution_not_waiting")
)

// RouteOutcome reports what the router did, for both audit and HTTP body.
type RouteOutcome string

const (
	OutcomeResumed              RouteOutcome = "resumed"
	OutcomeFallback             RouteOutcome = "fallback_triggered"
	OutcomeAutomationDispatched RouteOutcome = "automation_dispatched"
)

// CorrelationsStore is the narrow interface the router consumes from
// CorrelationsRepo. Tests substitute a stub.
type CorrelationsStore interface {
	Lookup(ctx context.Context, token uuid.UUID) (*Correlation, error)
	MarkConsumed(ctx context.Context, token uuid.UUID) error
}

// WaitEventResumer is the narrow contract the router calls when a token
// is matched. Implemented by *nodetypes.WaitEventResumer.
type WaitEventResumer interface {
	Resume(ctx context.Context, executionID uuid.UUID, nodeID string, payload map[string]any) error
}

// FallbackTriggerRouter handles the "no token match" branch by treating
// the click as a brand-new IM event so trigger_handler can dispatch to
// any matching workflow trigger.
type FallbackTriggerRouter interface {
	RouteAsIMEvent(ctx context.Context, event map[string]any) error
}

// AuditSink records router outcomes.
type AuditSink interface {
	Record(ctx context.Context, kind string, payload map[string]any) error
}

// AutomationActionHandler dispatches actions for correlations with no
// execution_id (null). These originate from automation rules rather than
// active workflow executions.
type AutomationActionHandler interface {
	Decide(ctx context.Context, findingID uuid.UUID, action string, actor string) error
}

// Router is the central card-action decision point.
type Router struct {
	Correlations CorrelationsStore
	Resumer      WaitEventResumer
	Fallback     FallbackTriggerRouter
	Audit        AuditSink
	Automation   AutomationActionHandler
	Now          func() time.Time // override for tests
}

// RouteInput is the structured input forwarded by the HTTP handler.
type RouteInput struct {
	Token       uuid.UUID
	ActionID    string
	Value       map[string]any
	ReplyTarget map[string]any
	UserID      string
	TenantID    string
}

// RouteResult describes what the router did so the handler can surface it
// to the IM Bridge for toast rendering.
type RouteResult struct {
	Outcome     RouteOutcome
	ExecutionID uuid.UUID // zero when fallback
	NodeID      string    // empty when fallback
}

// Route is the single entry point. Branches:
//  1. Token missing in store           -> fallback to trigger router
//  2. Token consumed                   -> ErrCardActionConsumed (409)
//  3. Token past expires_at            -> ErrCardActionExpired (410)
//  4. Resumer reports not-waiting      -> ErrExecutionNotWaiting (409)
//  5. Otherwise: Resume + MarkConsumed -> OutcomeResumed (200)
func (r *Router) Route(ctx context.Context, in RouteInput) (*RouteResult, error) {
	if r == nil || r.Correlations == nil {
		return nil, fmt.Errorf("router not configured")
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}

	corr, err := r.Correlations.Lookup(ctx, in.Token)
	if errors.Is(err, ErrCorrelationNotFound) {
		// Fallback path: synthesize an IM event so the existing trigger
		// router can match the action_id as a free-form command.
		if r.Fallback != nil {
			ev := map[string]any{
				"actionId":    in.ActionID,
				"value":       in.Value,
				"userId":      in.UserID,
				"tenantId":    in.TenantID,
				"replyTarget": in.ReplyTarget,
			}
			if err := r.Fallback.RouteAsIMEvent(ctx, ev); err != nil {
				return nil, fmt.Errorf("fallback route: %w", err)
			}
		}
		r.audit(ctx, "card_action_fallback", map[string]any{
			"token":    in.Token.String(),
			"actionId": in.ActionID,
			"userId":   in.UserID,
		})
		return &RouteResult{Outcome: OutcomeFallback}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup: %w", err)
	}

	if corr.ConsumedAt != nil {
		return nil, ErrCardActionConsumed
	}
	if corr.ExpiresAt.Before(now) {
		return nil, ErrCardActionExpired
	}

	// Automation branch: correlation with nil ExecutionID was minted by
	// an automation rule (e.g. review_completed_rule). Dispatch to the
	// AutomationActionHandler instead of the wait_event resumer.
	if corr.ExecutionID == uuid.Nil && r.Automation != nil {
		// Extract finding_id from correlation payload
		findingIDStr, _ := corr.Payload["finding_id"].(string)
		findingID, _ := uuid.Parse(findingIDStr)
		actionID, _ := corr.Payload["action"].(string)
		if actionID == "" {
			actionID = in.ActionID
		}
		if err := r.Automation.Decide(ctx, findingID, actionID, in.UserID); err != nil {
			return nil, fmt.Errorf("automation dispatch: %w", err)
		}
		if err := r.Correlations.MarkConsumed(ctx, in.Token); err != nil {
			r.audit(ctx, "card_action_mark_consumed_failed", map[string]any{
				"token": in.Token.String(), "error": err.Error(),
			})
		}
		r.audit(ctx, "card_action_automation", map[string]any{
			"token":     in.Token.String(),
			"actionId":  actionID,
			"userId":    in.UserID,
			"findingId": findingIDStr,
		})
		return &RouteResult{Outcome: OutcomeAutomationDispatched}, nil
	}

	// Build the payload visible to the wait_event node.
	payload := map[string]any{
		"action_id": corr.ActionID,
		"value":     in.Value,
		"user_id":   in.UserID,
		"tenant_id": in.TenantID,
	}

	if err := r.Resumer.Resume(ctx, corr.ExecutionID, corr.NodeID, payload); err != nil {
		if err.Error() == "wait_event: target node is not waiting" {
			return nil, ErrExecutionNotWaiting
		}
		return nil, fmt.Errorf("resume: %w", err)
	}

	if err := r.Correlations.MarkConsumed(ctx, in.Token); err != nil {
		r.audit(ctx, "card_action_mark_consumed_failed", map[string]any{
			"token": in.Token.String(), "error": err.Error(),
		})
	}

	r.audit(ctx, "card_action_routed", map[string]any{
		"token":       in.Token.String(),
		"actionId":    in.ActionID,
		"userId":      in.UserID,
		"executionId": corr.ExecutionID.String(),
	})
	return &RouteResult{
		Outcome:     OutcomeResumed,
		ExecutionID: corr.ExecutionID,
		NodeID:      corr.NodeID,
	}, nil
}

func (r *Router) audit(ctx context.Context, kind string, payload map[string]any) {
	if r.Audit == nil {
		return
	}
	_ = r.Audit.Record(ctx, kind, payload)
}
