package trigger_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/trigger"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

type fakeScheduleLister struct {
	triggers []*model.WorkflowTrigger
	err      error
}

func (f *fakeScheduleLister) ListEnabledBySource(_ context.Context, _ model.TriggerSource) ([]*model.WorkflowTrigger, error) {
	return f.triggers, f.err
}

type fakeScheduleDispatcher struct {
	events []trigger.Event
}

func (f *fakeScheduleDispatcher) Route(_ context.Context, ev trigger.Event) (int, error) {
	f.events = append(f.events, ev)
	return 1, nil
}

func scheduleTrigger(cronExpr string) *model.WorkflowTrigger {
	cfg, _ := json.Marshal(map[string]any{"cron": cronExpr})
	wfID := uuid.New()
	return &model.WorkflowTrigger{
		ID:         uuid.New(),
		WorkflowID: &wfID,
		ProjectID:  uuid.New(),
		Source:     model.TriggerSourceSchedule,
		TargetKind: model.TriggerTargetDAG,
		Config:     cfg,
		Enabled:    true,
	}
}

func TestScheduleTicker_FiresOnMinuteMatch(t *testing.T) {
	// Cron "30 14 * * *" = every day at 14:30 UTC. Clock fixed at 14:30:00.
	clock := &fakeClock{t: time.Date(2026, 4, 19, 14, 30, 0, 0, time.UTC)}
	lister := &fakeScheduleLister{triggers: []*model.WorkflowTrigger{scheduleTrigger("30 14 * * *")}}
	dispatcher := &fakeScheduleDispatcher{}

	ticker := trigger.NewScheduleTicker(lister, dispatcher, clock)
	if err := ticker.Tick(context.Background()); err != nil {
		t.Fatalf("tick: %v", err)
	}
	if len(dispatcher.events) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(dispatcher.events))
	}
	if dispatcher.events[0].Source != model.TriggerSourceSchedule {
		t.Errorf("expected schedule source, got %s", dispatcher.events[0].Source)
	}
}

func TestScheduleTicker_SkipsWhenCronDoesNotMatch(t *testing.T) {
	// Cron fires at 14:30; current minute is 14:29.
	clock := &fakeClock{t: time.Date(2026, 4, 19, 14, 29, 0, 0, time.UTC)}
	lister := &fakeScheduleLister{triggers: []*model.WorkflowTrigger{scheduleTrigger("30 14 * * *")}}
	dispatcher := &fakeScheduleDispatcher{}

	ticker := trigger.NewScheduleTicker(lister, dispatcher, clock)
	if err := ticker.Tick(context.Background()); err != nil {
		t.Fatalf("tick: %v", err)
	}
	if len(dispatcher.events) != 0 {
		t.Fatalf("expected 0 dispatches, got %d", len(dispatcher.events))
	}
}

func TestScheduleTicker_DedupesSameMinute(t *testing.T) {
	// Two ticks at the same minute should only dispatch once.
	clock := &fakeClock{t: time.Date(2026, 4, 19, 14, 30, 0, 0, time.UTC)}
	lister := &fakeScheduleLister{triggers: []*model.WorkflowTrigger{scheduleTrigger("30 14 * * *")}}
	dispatcher := &fakeScheduleDispatcher{}

	ticker := trigger.NewScheduleTicker(lister, dispatcher, clock)
	_ = ticker.Tick(context.Background())
	_ = ticker.Tick(context.Background())
	if len(dispatcher.events) != 1 {
		t.Fatalf("dedupe failed: got %d dispatches", len(dispatcher.events))
	}
}

func TestScheduleTicker_DedupesConcurrentTicksInSameMinute(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 4, 19, 14, 30, 0, 0, time.UTC)}
	lister := &fakeScheduleLister{triggers: []*model.WorkflowTrigger{scheduleTrigger("30 14 * * *")}}
	dispatcher := &fakeScheduleDispatcher{}

	ticker := trigger.NewScheduleTicker(lister, dispatcher, clock)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ticker.Tick(context.Background()); err != nil {
				t.Errorf("tick: %v", err)
			}
		}()
	}
	wg.Wait()

	if len(dispatcher.events) != 1 {
		t.Fatalf("expected 1 dispatch across concurrent ticks, got %d", len(dispatcher.events))
	}
}

func TestScheduleTicker_InvalidCronSkipped(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 4, 19, 14, 30, 0, 0, time.UTC)}
	lister := &fakeScheduleLister{triggers: []*model.WorkflowTrigger{scheduleTrigger("not a cron")}}
	dispatcher := &fakeScheduleDispatcher{}

	ticker := trigger.NewScheduleTicker(lister, dispatcher, clock)
	if err := ticker.Tick(context.Background()); err != nil {
		t.Fatalf("tick: %v", err)
	}
	if len(dispatcher.events) != 0 {
		t.Fatalf("invalid cron should not dispatch; got %d", len(dispatcher.events))
	}
}

func TestScheduleTicker_EmptyCronSkipped(t *testing.T) {
	cfg, _ := json.Marshal(map[string]any{}) // no cron key
	wfID := uuid.New()
	tr := &model.WorkflowTrigger{
		ID: uuid.New(), WorkflowID: &wfID, ProjectID: uuid.New(),
		Source: model.TriggerSourceSchedule, TargetKind: model.TriggerTargetDAG, Config: cfg, Enabled: true,
	}
	clock := &fakeClock{t: time.Date(2026, 4, 19, 14, 30, 0, 0, time.UTC)}
	lister := &fakeScheduleLister{triggers: []*model.WorkflowTrigger{tr}}
	dispatcher := &fakeScheduleDispatcher{}

	ticker := trigger.NewScheduleTicker(lister, dispatcher, clock)
	_ = ticker.Tick(context.Background())
	if len(dispatcher.events) != 0 {
		t.Fatalf("empty cron should not dispatch; got %d", len(dispatcher.events))
	}
}

// --- Plan 3B §7.4: auth_expired binding gate ---

type fakeBindingChecker struct {
	active map[string]bool
}

func (f *fakeBindingChecker) IsBindingActive(_ context.Context, bindingID string) bool {
	return f.active[bindingID]
}

func TestScheduleTicker_SkipsWhenAuthExpired(t *testing.T) {
	bindingID := uuid.New().String()
	cfg, _ := json.Marshal(map[string]any{"cron": "* * * * *", "binding_id": bindingID})
	wfID := uuid.New()
	tr := &model.WorkflowTrigger{
		ID: uuid.New(), WorkflowID: &wfID, ProjectID: uuid.New(),
		Source: model.TriggerSourceSchedule, TargetKind: model.TriggerTargetDAG, Config: cfg, Enabled: true,
	}

	clock := &fakeClock{t: time.Date(2026, 4, 19, 14, 30, 0, 0, time.UTC)}
	lister := &fakeScheduleLister{triggers: []*model.WorkflowTrigger{tr}}
	dispatcher := &fakeScheduleDispatcher{}

	ticker := trigger.NewScheduleTicker(lister, dispatcher, clock)
	ticker.SetBindingChecker(&fakeBindingChecker{active: map[string]bool{bindingID: false}})

	_ = ticker.Tick(context.Background())
	if len(dispatcher.events) != 0 {
		t.Fatalf("expected dispatch to be skipped for auth_expired binding; got %d", len(dispatcher.events))
	}

	// With active binding, dispatch should proceed.
	dispatcher.events = nil
	ticker2 := trigger.NewScheduleTicker(lister, dispatcher, clock)
	ticker2.SetBindingChecker(&fakeBindingChecker{active: map[string]bool{bindingID: true}})
	_ = ticker2.Tick(context.Background())
	if len(dispatcher.events) != 1 {
		t.Fatalf("expected 1 dispatch for active binding; got %d", len(dispatcher.events))
	}
}
