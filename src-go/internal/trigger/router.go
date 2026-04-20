package trigger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/employee"
	"github.com/agentforge/server/internal/model"
)

// ListRepository is the read side of workflow_triggers that Router needs.
type ListRepository interface {
	ListEnabledBySource(ctx context.Context, source model.TriggerSource) ([]*model.WorkflowTrigger, error)
}

// Event is the source-agnostic payload the Router consumes.
type Event struct {
	Source model.TriggerSource
	// Data is the event payload (IM command args, cron $now, webhook body).
	// Template references like `{{$event.pr_url}}` resolve into this map.
	Data map[string]any
}

// OutcomeStatus is the normalized status of a dispatch attempt, shared across
// trigger sources so IM, schedule, and future sources emit the same shape.
type OutcomeStatus string

const (
	// OutcomeStarted: the adapter started a workflow run successfully.
	OutcomeStarted OutcomeStatus = "started"
	// OutcomeSkippedIdempotent: the idempotency key collided inside the
	// dedupe window; no run was started.
	OutcomeSkippedIdempotent OutcomeStatus = "skipped_idempotent"
	// OutcomeFailedUnknownTarget: the trigger declared a target kind for
	// which no adapter is registered.
	OutcomeFailedUnknownTarget OutcomeStatus = "failed_unknown_target"
	// OutcomeFailedMapping: input_mapping templating failed to render.
	OutcomeFailedMapping OutcomeStatus = "failed_mapping"
	// OutcomeFailedEngineStart: the engine adapter returned an error when
	// attempting to start the run.
	OutcomeFailedEngineStart OutcomeStatus = "failed_engine_start"
	// OutcomeFailedIdempotencyStore: the idempotency store returned an
	// error; the trigger was skipped defensively.
	OutcomeFailedIdempotencyStore OutcomeStatus = "failed_idempotency_store"
	// OutcomeFailedActingEmployee: the trigger declared an acting_employee_id
	// that failed dispatch-time validation (e.g. archived or deleted).
	// The idempotency key is NOT consumed so re-dispatch is possible once the
	// author re-binds the trigger.
	OutcomeFailedActingEmployee OutcomeStatus = "failed_acting_employee"
)

// Outcome is the structured result of dispatching a single matched trigger.
// The key invariant: every matched trigger produces exactly one Outcome,
// regardless of success or engine kind.
type Outcome struct {
	TriggerID  uuid.UUID               `json:"triggerId"`
	TargetKind model.TriggerTargetKind `json:"targetKind"`
	Status     OutcomeStatus           `json:"status"`
	RunID      *uuid.UUID              `json:"runId,omitempty"`
	Reason     string                  `json:"reason,omitempty"`
}

// imTriggerConfig is the typed shape of a trigger's Config JSON for IM source.
type imTriggerConfig struct {
	Platform      string   `json:"platform"`
	Command       string   `json:"command"`
	MatchRegex    string   `json:"match_regex"`
	ChatAllowlist []string `json:"chat_allowlist"`
}

// AttributionValidator is the dispatch-time employee guard the Router consults
// when a trigger row carries a non-nil acting_employee_id. Satisfied in
// production by *employee.AttributionGuard; tests may substitute a fake.
// Left unset when attribution validation is disabled (e.g. unit tests that
// predate the guard).
type AttributionValidator interface {
	ValidateNotArchived(ctx context.Context, employeeID uuid.UUID) error
}

// Router dispatches an incoming Event to every matching, enabled trigger
// and invokes the target engine registered for the trigger's TargetKind.
type Router struct {
	repo    ListRepository
	engines map[model.TriggerTargetKind]TargetEngine
	idem    IdempotencyStore
	guard   AttributionValidator
}

// NewRouter returns a new Router backed by the provided dependencies. Each
// entry in engines keys its adapter by TargetEngine.Kind(); duplicate Kinds
// produce a single adapter keyed by the last entry (caller responsibility).
func NewRouter(repo ListRepository, idem IdempotencyStore, engines ...TargetEngine) *Router {
	registry := make(map[model.TriggerTargetKind]TargetEngine, len(engines))
	for _, eng := range engines {
		if eng == nil {
			continue
		}
		registry[eng.Kind()] = eng
	}
	return &Router{repo: repo, engines: registry, idem: idem}
}

// WithAttributionGuard attaches a dispatch-time guard that validates each
// trigger's acting_employee_id (if any) before the engine adapter is called.
// When the guard returns an error, the Router emits
// OutcomeFailedActingEmployee and does NOT consume the idempotency key so the
// operator can re-bind the trigger and retry.
func (r *Router) WithAttributionGuard(guard AttributionValidator) *Router {
	r.guard = guard
	return r
}

// RegisterEngine attaches or replaces the adapter for a TargetEngine.Kind().
// This is useful when an engine is constructed after the Router (e.g. the
// plugin runtime becomes available later in wiring). Callers that supply all
// engines at construction time do not need this.
func (r *Router) RegisterEngine(engine TargetEngine) {
	if engine == nil {
		return
	}
	if r.engines == nil {
		r.engines = make(map[model.TriggerTargetKind]TargetEngine)
	}
	r.engines[engine.Kind()] = engine
}

// Route returns the number of executions started and the last non-idempotent
// error observed across the dispatched triggers. Use RouteWithOutcomes for
// per-trigger structured outcomes.
func (r *Router) Route(ctx context.Context, ev Event) (int, error) {
	outcomes, err := r.RouteWithOutcomes(ctx, ev)
	started := 0
	for _, out := range outcomes {
		if out.Status == OutcomeStarted {
			started++
		}
	}
	return started, err
}

// RouteWithOutcomes evaluates every enabled trigger for ev.Source against
// the event's match filter, applies idempotency + input mapping, and
// dispatches through the registered TargetEngine. Errors on individual
// triggers do not abort the batch; each matched trigger yields exactly one
// Outcome in the returned slice. The returned error is the last engine or
// idempotency-store error encountered (nil on full success) — outcomes
// remain the authoritative per-trigger record.
func (r *Router) RouteWithOutcomes(ctx context.Context, ev Event) ([]Outcome, error) {
	triggers, err := r.repo.ListEnabledBySource(ctx, ev.Source)
	if err != nil {
		return nil, err
	}

	var lastErr error
	outcomes := make([]Outcome, 0, len(triggers))

	for _, trigger := range triggers {
		if !MatchesTrigger(trigger, ev) {
			continue
		}

		targetKind := trigger.TargetKind
		if targetKind == "" {
			targetKind = model.TriggerTargetDAG
		}

		// Step 0: Attribution guard (pre-idempotency so the key is not consumed
		// on guard failure; operators can re-bind and retry).
		if trigger.ActingEmployeeID != nil && r.guard != nil {
			if guardErr := r.guard.ValidateNotArchived(ctx, *trigger.ActingEmployeeID); guardErr != nil {
				reason := guardErr.Error()
				if errors.Is(guardErr, employee.ErrEmployeeArchived) {
					reason = fmt.Sprintf("acting employee %s is archived", trigger.ActingEmployeeID)
				} else if errors.Is(guardErr, employee.ErrEmployeeNotFound) {
					reason = fmt.Sprintf("acting employee %s not found", trigger.ActingEmployeeID)
				}
				outcomes = append(outcomes, Outcome{
					TriggerID:  trigger.ID,
					TargetKind: targetKind,
					Status:     OutcomeFailedActingEmployee,
					Reason:     reason,
				})
				continue
			}
		}

		// Step a: Idempotency check (engine-agnostic per Decision 4).
		if trigger.IdempotencyKeyTemplate != "" && trigger.DedupeWindowSeconds > 0 {
			rendered := renderTemplate(trigger.IdempotencyKeyTemplate, ev.Data)
			var key string
			if rendered != nil {
				key = fmt.Sprint(rendered)
			}
			seen, idemErr := r.idem.SeenWithin(ctx, key, time.Duration(trigger.DedupeWindowSeconds)*time.Second)
			if idemErr != nil {
				lastErr = idemErr
				outcomes = append(outcomes, Outcome{
					TriggerID:  trigger.ID,
					TargetKind: targetKind,
					Status:     OutcomeFailedIdempotencyStore,
					Reason:     idemErr.Error(),
				})
				continue
			}
			if seen {
				outcomes = append(outcomes, Outcome{
					TriggerID:  trigger.ID,
					TargetKind: targetKind,
					Status:     OutcomeSkippedIdempotent,
					Reason:     "idempotency key already seen within dedupe window",
				})
				continue
			}
		}

		// Step b: Input mapping.
		seed, mappingErr := RenderInputMapping(trigger.InputMapping, ev.Data)
		if mappingErr != nil {
			lastErr = mappingErr
			outcomes = append(outcomes, Outcome{
				TriggerID:  trigger.ID,
				TargetKind: targetKind,
				Status:     OutcomeFailedMapping,
				Reason:     mappingErr.Error(),
			})
			continue
		}

		// Step c: Engine lookup + dispatch.
		engine, ok := r.engines[targetKind]
		if !ok {
			outcomes = append(outcomes, Outcome{
				TriggerID:  trigger.ID,
				TargetKind: targetKind,
				Status:     OutcomeFailedUnknownTarget,
				Reason:     fmt.Sprintf("no adapter registered for target_kind=%q", targetKind),
			})
			continue
		}

		run, execErr := engine.Start(ctx, trigger, seed)
		if execErr != nil {
			lastErr = execErr
			outcomes = append(outcomes, Outcome{
				TriggerID:  trigger.ID,
				TargetKind: targetKind,
				Status:     OutcomeFailedEngineStart,
				Reason:     execErr.Error(),
			})
			continue
		}

		runID := run.RunID
		outcomes = append(outcomes, Outcome{
			TriggerID:  trigger.ID,
			TargetKind: run.Engine,
			Status:     OutcomeStarted,
			RunID:      &runID,
		})
	}

	return outcomes, lastErr
}

// MatchesTrigger returns true if ev satisfies the trigger's filter conditions.
// Exported in Spec 1C so the trigger CRUD service's dry-run endpoint can
// reuse the exact same matching logic the live router uses.
func MatchesTrigger(trigger *model.WorkflowTrigger, ev Event) bool {
	switch trigger.Source {
	case model.TriggerSourceIM:
		return matchesIMTrigger(trigger, ev)
	case model.TriggerSourceSchedule:
		// Schedule triggers always match; the cron dispatcher pre-filters by trigger.
		return true
	default:
		return false
	}
}

// matchesIMTrigger checks the IM-specific filter conditions.
func matchesIMTrigger(trigger *model.WorkflowTrigger, ev Event) bool {
	if len(trigger.Config) == 0 {
		return true
	}

	var cfg imTriggerConfig
	if err := json.Unmarshal(trigger.Config, &cfg); err != nil {
		return false
	}

	// Platform filter.
	if cfg.Platform != "" {
		platform, _ := ev.Data["platform"].(string)
		if platform != cfg.Platform {
			return false
		}
	}

	// Command filter.
	if cfg.Command != "" {
		command, _ := ev.Data["command"].(string)
		if command != cfg.Command {
			return false
		}
	}

	// Regex filter on content.
	if cfg.MatchRegex != "" {
		content, _ := ev.Data["content"].(string)
		re, err := regexp.Compile(cfg.MatchRegex)
		if err != nil {
			return false
		}
		if !re.MatchString(content) {
			return false
		}
	}

	// Chat allowlist filter.
	if len(cfg.ChatAllowlist) > 0 {
		chatID, _ := ev.Data["chat_id"].(string)
		found := false
		for _, allowed := range cfg.ChatAllowlist {
			if chatID == allowed {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// templateExpr matches template expressions like `{{ $event.some.path }}`.
var templateExpr = regexp.MustCompile(`\{\{\s*\$event\.([a-zA-Z0-9_.]+)\s*\}\}`)

// renderTemplate renders a template string against event data.
// If the entire trimmed string is a single template expression, it returns
// the resolved value preserving its native type.
// Otherwise it performs string substitution, stringifying each resolved value.
// Unresolvable paths render as empty string in embedded mode or nil in whole-template mode.
// Malformed templates (unbalanced braces) are returned unchanged.
func renderTemplate(tmpl string, data map[string]any) any {
	trimmed := strings.TrimSpace(tmpl)
	if m := templateExpr.FindStringSubmatch(trimmed); m != nil && m[0] == trimmed {
		// Whole-template reference: preserve native type.
		return lookupPath(data, m[1])
	}
	// Embedded: stringify each match.
	return templateExpr.ReplaceAllStringFunc(tmpl, func(match string) string {
		m := templateExpr.FindStringSubmatch(match)
		if m == nil {
			return ""
		}
		v := lookupPath(data, m[1])
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	})
}

// lookupPath traverses root following dotted path segments.
// Numeric segments index into arrays. Returns nil if any segment is unresolvable.
func lookupPath(root map[string]any, path string) any {
	var cur any = root
	for _, seg := range strings.Split(path, ".") {
		switch v := cur.(type) {
		case map[string]any:
			cur = v[seg]
		case []any:
			var idx int
			if _, err := fmt.Sscanf(seg, "%d", &idx); err != nil || idx < 0 || idx >= len(v) {
				return nil
			}
			cur = v[idx]
		default:
			return nil
		}
		if cur == nil {
			return nil
		}
	}
	return cur
}

// RenderInputMapping renders each string value in the mapping as a template,
// passing through non-string values unchanged. Exported in Spec 1C so the
// trigger CRUD service's dry-run endpoint can preview the input the engine
// would receive without actually dispatching.
func RenderInputMapping(mappingRaw json.RawMessage, data map[string]any) (map[string]any, error) {
	if len(mappingRaw) == 0 {
		return map[string]any{}, nil
	}
	var mapping map[string]any
	if err := json.Unmarshal(mappingRaw, &mapping); err != nil {
		return nil, err
	}
	out := make(map[string]any, len(mapping))
	for k, v := range mapping {
		if s, ok := v.(string); ok {
			out[k] = renderTemplate(s, data)
		} else {
			out[k] = v
		}
	}
	return out, nil
}
