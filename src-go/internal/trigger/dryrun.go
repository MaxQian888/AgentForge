// Package trigger — dryrun.go provides Spec 1C's "test event without
// dispatching" surface. The CRUD service calls DryRun to preview whether a
// sample event would match the trigger's filters and what input the engine
// would receive, without ever invoking the engine adapter or touching the
// idempotency store.
//
// Lives in the trigger package (not service) to avoid an import cycle:
// internal/trigger already depends on internal/service via engines.go, so
// the inverse direction is not allowed. The CRUD service consumes this
// function as plain trigger.DryRun.
package trigger

import (
	"github.com/agentforge/server/internal/model"
)

// DryRunResult is the structured outcome of a single dry-run invocation.
// JSON tags mirror what the FE trigger-test modal expects.
type DryRunResult struct {
	Matched       bool           `json:"matched"`
	WouldDispatch bool           `json:"would_dispatch"`
	RenderedInput map[string]any `json:"rendered_input,omitempty"`
	SkipReason    string         `json:"skip_reason,omitempty"`
}

// DryRun evaluates the trigger's matchers and input mapping against a
// sample event payload. It is engine- and idempotency-store-free by
// construction so callers can preview safely. The trigger's Enabled flag
// is honored so a disabled row reports would_dispatch=false even when its
// filters match.
func DryRun(tr *model.WorkflowTrigger, event map[string]any) *DryRunResult {
	if tr == nil {
		return &DryRunResult{SkipReason: "trigger_nil"}
	}
	ev := Event{Source: tr.Source, Data: event}
	if !MatchesTrigger(tr, ev) {
		return &DryRunResult{Matched: false, WouldDispatch: false, SkipReason: "no_match"}
	}
	mapped, mapErr := RenderInputMapping(tr.InputMapping, event)
	if mapErr != nil {
		return &DryRunResult{
			Matched:       true,
			WouldDispatch: false,
			SkipReason:    "mapping_error: " + mapErr.Error(),
		}
	}
	if !tr.Enabled {
		return &DryRunResult{
			Matched:       true,
			WouldDispatch: false,
			RenderedInput: mapped,
			SkipReason:    "trigger_disabled",
		}
	}
	return &DryRunResult{Matched: true, WouldDispatch: true, RenderedInput: mapped}
}
