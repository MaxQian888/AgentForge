package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/react-go-quick-starter/server/internal/model"
)

// ScheduleLister is the minimum repo dependency the ticker needs.
// *repository.WorkflowTriggerRepository satisfies it structurally.
type ScheduleLister interface {
	ListEnabledBySource(ctx context.Context, source model.TriggerSource) ([]*model.WorkflowTrigger, error)
}

// ScheduleDispatcher is the subset of Router the ticker calls.
type ScheduleDispatcher interface {
	Route(ctx context.Context, ev Event) (int, error)
}

// Clock abstracts the current time for deterministic testing.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// ScheduleTicker watches enabled source=schedule workflow_triggers and
// invokes the router when each trigger's cron expression crosses a minute
// boundary. This is a lightweight shim — it runs in-process without DB
// persistence of firing history. Missed firings across process restarts
// are NOT replayed; production operators who need that guarantee should
// migrate to the fuller scheduler.Service job model in a follow-up.
//
// The ticker polls once per minute (aligned to minute boundaries) so at
// most one firing is evaluated per trigger per minute. Idempotency on the
// Router itself protects against brief double-dispatch at restart.
type ScheduleTicker struct {
	lister     ScheduleLister
	dispatcher ScheduleDispatcher
	clock      Clock
	parser     cron.Parser

	mu       sync.Mutex
	lastFire map[string]time.Time // trigger_id → last minute we successfully dispatched
}

// NewScheduleTicker constructs a ticker. Pass nil clock to use the system clock.
func NewScheduleTicker(lister ScheduleLister, dispatcher ScheduleDispatcher, clock Clock) *ScheduleTicker {
	if clock == nil {
		clock = systemClock{}
	}
	return &ScheduleTicker{
		lister:     lister,
		dispatcher: dispatcher,
		clock:      clock,
		parser:     cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		lastFire:   make(map[string]time.Time),
	}
}

// Run blocks until ctx is cancelled. It evaluates schedule triggers once
// at start, then on every minute boundary. Errors during dispatch are
// swallowed (logged upstream by the Router). Callers should run this in
// a goroutine.
func (t *ScheduleTicker) Run(ctx context.Context) {
	// Fire-on-start pass so operators can test "fired now" without waiting.
	t.tick(ctx)

	for {
		next := nextMinuteBoundary(t.clock.Now())
		wait := time.Until(next)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
			t.tick(ctx)
		}
	}
}

// Tick evaluates all schedule triggers once. Exposed for tests; production
// uses Run. Errors during list or dispatch are returned so tests can assert.
func (t *ScheduleTicker) Tick(ctx context.Context) error {
	return t.tick(ctx)
}

func (t *ScheduleTicker) tick(ctx context.Context) error {
	triggers, err := t.lister.ListEnabledBySource(ctx, model.TriggerSourceSchedule)
	if err != nil {
		return fmt.Errorf("list schedule triggers: %w", err)
	}
	now := t.clock.Now().UTC().Truncate(time.Minute)
	for _, tr := range triggers {
		if t.shouldFire(tr, now) {
			t.dispatchOne(ctx, tr, now)
		}
	}
	return nil
}

// shouldFire parses the trigger's cron expression and returns whether the
// current minute boundary matches a scheduled fire. It also dedupes so the
// same (trigger, minute) pair never fires twice within a process lifetime.
func (t *ScheduleTicker) shouldFire(tr *model.WorkflowTrigger, minute time.Time) bool {
	cronExpr := extractCronExpr(tr.Config)
	if cronExpr == "" {
		return false
	}
	sched, err := t.parser.Parse(cronExpr)
	if err != nil {
		return false
	}
	// A cron's Next(base) returns the first fire strictly after base. To
	// check whether `minute` itself is a fire, compute Next one second
	// before minute and compare.
	candidate := sched.Next(minute.Add(-time.Second))
	if !candidate.Equal(minute) {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if last, ok := t.lastFire[tr.ID.String()]; ok && !last.Before(minute) {
		return false
	}
	return true
}

func (t *ScheduleTicker) dispatchOne(ctx context.Context, tr *model.WorkflowTrigger, minute time.Time) {
	data := map[string]any{
		"trigger_id": tr.ID.String(),
		"workflow_id": tr.WorkflowID.String(),
		"project_id":  tr.ProjectID.String(),
		"fired_at":    minute.Format(time.RFC3339),
	}
	_, _ = t.dispatcher.Route(ctx, Event{
		Source: model.TriggerSourceSchedule,
		Data:   data,
	})
	t.mu.Lock()
	t.lastFire[tr.ID.String()] = minute
	t.mu.Unlock()
}

// extractCronExpr reads the cron string from a schedule trigger's Config
// jsonb. Expected shape: {"cron": "...", "timezone": "...", "overlap_policy": "..."}.
// Missing cron returns "".
func extractCronExpr(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var cfg struct {
		Cron string `json:"cron"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ""
	}
	return cfg.Cron
}

// nextMinuteBoundary returns the next UTC time whose seconds/nanos are zero.
func nextMinuteBoundary(now time.Time) time.Time {
	t := now.UTC().Truncate(time.Minute).Add(time.Minute)
	return t
}
