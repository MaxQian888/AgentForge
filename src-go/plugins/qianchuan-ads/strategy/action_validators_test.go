package strategy

import (
	"strings"
	"testing"
)

func TestValidateAction(t *testing.T) {
	cases := []struct {
		name      string
		action    StrategyAction
		wantError string // substring; empty = expect nil error
	}{
		{
			name:   "adjust_bid pct ok",
			action: StrategyAction{Type: "adjust_bid", Params: map[string]any{"pct": float64(5)}},
		},
		{
			name:   "adjust_bid to ok",
			action: StrategyAction{Type: "adjust_bid", Params: map[string]any{"to": float64(2.5)}},
		},
		{
			name:      "adjust_bid both rejected",
			action:    StrategyAction{Type: "adjust_bid", Params: map[string]any{"pct": float64(5), "to": float64(2)}},
			wantError: "exactly one of pct or to",
		},
		{
			name:      "adjust_bid neither rejected",
			action:    StrategyAction{Type: "adjust_bid", Params: map[string]any{}},
			wantError: "exactly one of pct or to",
		},
		{
			name:      "adjust_bid pct zero rejected",
			action:    StrategyAction{Type: "adjust_bid", Params: map[string]any{"pct": float64(0)}},
			wantError: "non-zero",
		},
		{
			name:      "adjust_bid pct out of range",
			action:    StrategyAction{Type: "adjust_bid", Params: map[string]any{"pct": float64(150)}},
			wantError: "pct must be in",
		},
		{
			name:      "adjust_bid to negative",
			action:    StrategyAction{Type: "adjust_bid", Params: map[string]any{"to": float64(-1)}},
			wantError: "to must be positive",
		},
		{
			name:   "adjust_budget ok",
			action: StrategyAction{Type: "adjust_budget", Params: map[string]any{"to": float64(100)}},
		},
		{
			name:      "adjust_budget invalid",
			action:    StrategyAction{Type: "adjust_budget", Params: map[string]any{}},
			wantError: "exactly one of pct or to",
		},
		{
			name:   "pause_ad empty params ok",
			action: StrategyAction{Type: "pause_ad", Params: nil},
		},
		{
			name:      "pause_ad with params rejected",
			action:    StrategyAction{Type: "pause_ad", Params: map[string]any{"x": 1}},
			wantError: "must have empty params",
		},
		{
			name:   "resume_ad empty params ok",
			action: StrategyAction{Type: "resume_ad", Params: map[string]any{}},
		},
		{
			name:      "resume_ad with params rejected",
			action:    StrategyAction{Type: "resume_ad", Params: map[string]any{"x": 1}},
			wantError: "must have empty params",
		},
		{
			name:   "apply_material ok",
			action: StrategyAction{Type: "apply_material", Params: map[string]any{"material_id": "abc-123"}},
		},
		{
			name:      "apply_material missing material_id",
			action:    StrategyAction{Type: "apply_material", Params: map[string]any{}},
			wantError: "material_id",
		},
		{
			name:      "apply_material empty material_id",
			action:    StrategyAction{Type: "apply_material", Params: map[string]any{"material_id": "  "}},
			wantError: "material_id",
		},
		{
			name:   "notify_im ok",
			action: StrategyAction{Type: "notify_im", Params: map[string]any{"channel": "default", "template": "hi"}},
		},
		{
			name:      "notify_im missing channel",
			action:    StrategyAction{Type: "notify_im", Params: map[string]any{"template": "hi"}},
			wantError: "channel",
		},
		{
			name:      "notify_im missing template",
			action:    StrategyAction{Type: "notify_im", Params: map[string]any{"channel": "default"}},
			wantError: "template",
		},
		{
			name:   "record_event ok",
			action: StrategyAction{Type: "record_event", Params: map[string]any{"event_name": "tick"}},
		},
		{
			name:      "record_event missing event_name",
			action:    StrategyAction{Type: "record_event", Params: map[string]any{}},
			wantError: "event_name",
		},
		{
			name:      "unknown action type",
			action:    StrategyAction{Type: "do_evil", Params: map[string]any{}},
			wantError: "unknown action type",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAction(tc.action)
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantError)
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantError)
			}
			// Errors must include the action type for FE display.
			if !strings.Contains(err.Error(), tc.action.Type) && tc.action.Type != "" {
				t.Errorf("error should mention action type %q: %v", tc.action.Type, err)
			}
		})
	}
}
