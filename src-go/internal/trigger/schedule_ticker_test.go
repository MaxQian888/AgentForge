package trigger_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/trigger"
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
	return &model.WorkflowTrigger{
		ID:         uuid.New(),
		WorkflowID: uuid.New(),
		ProjectID:  uuid.New(),
		Source:     model.TriggerSourceSchedule,
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
	tr := &model.WorkflowTrigger{
		ID: uuid.New(), WorkflowID: uuid.New(), ProjectID: uuid.New(),
		Source: model.TriggerSourceSchedule, Config: cfg, Enabled: true,
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
