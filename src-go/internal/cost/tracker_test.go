package cost_test

import (
	"testing"

	"github.com/react-go-quick-starter/server/internal/cost"
)

func TestTrackerRecordAccumulatesAndThresholds(t *testing.T) {
	tracker := cost.NewTracker()

	if action := tracker.Record("task-1", 10, 20, 5, 4.0, 1, 10.0); action != cost.ActionNone {
		t.Fatalf("first record action = %v, want %v", action, cost.ActionNone)
	}
	if action := tracker.Record("task-1", 2, 3, 1, 4.0, 2, 10.0); action != cost.ActionWarn {
		t.Fatalf("second record action = %v, want %v", action, cost.ActionWarn)
	}
	if action := tracker.Record("task-1", 1, 1, 0, 2.0, 1, 10.0); action != cost.ActionKill {
		t.Fatalf("third record action = %v, want %v", action, cost.ActionKill)
	}

	got := tracker.Get("task-1")
	if got.InputTokens != 13 {
		t.Errorf("InputTokens = %d, want 13", got.InputTokens)
	}
	if got.OutputTokens != 24 {
		t.Errorf("OutputTokens = %d, want 24", got.OutputTokens)
	}
	if got.CacheReadTokens != 6 {
		t.Errorf("CacheReadTokens = %d, want 6", got.CacheReadTokens)
	}
	if got.CostUsd != 10.0 {
		t.Errorf("CostUsd = %v, want 10", got.CostUsd)
	}
	if got.TurnCount != 4 {
		t.Errorf("TurnCount = %d, want 4", got.TurnCount)
	}
}

func TestTrackerRecordWithoutBudgetStillStoresUsage(t *testing.T) {
	tracker := cost.NewTracker()

	if action := tracker.Record("task-2", 7, 11, 3, 1.5, 2, 0); action != cost.ActionNone {
		t.Fatalf("Record() action = %v, want %v", action, cost.ActionNone)
	}

	got := tracker.Get("task-2")
	if got.InputTokens != 7 || got.OutputTokens != 11 || got.CacheReadTokens != 3 {
		t.Fatalf("unexpected token totals: %+v", got)
	}
	if got.CostUsd != 1.5 {
		t.Errorf("CostUsd = %v, want 1.5", got.CostUsd)
	}
	if got.TurnCount != 2 {
		t.Errorf("TurnCount = %d, want 2", got.TurnCount)
	}
}

func TestTrackerResetAndAllReturnSnapshots(t *testing.T) {
	tracker := cost.NewTracker()
	tracker.Record("task-1", 1, 2, 0, 0.5, 1, 10)
	tracker.Record("task-2", 3, 4, 1, 0.75, 1, 10)

	all := tracker.All()
	if len(all) != 2 {
		t.Fatalf("len(All()) = %d, want 2", len(all))
	}

	snapshot := all["task-1"]
	snapshot.CostUsd = 999
	all["task-1"] = snapshot

	if got := tracker.Get("task-1"); got.CostUsd != 0.5 {
		t.Fatalf("tracker state was mutated through All() snapshot: %+v", got)
	}

	tracker.Reset("task-1")

	if got := tracker.Get("task-1"); got != (cost.TaskCost{}) {
		t.Fatalf("Get() after Reset = %+v, want zero value", got)
	}
	if len(tracker.All()) != 1 {
		t.Fatalf("len(All()) after Reset = %d, want 1", len(tracker.All()))
	}
}
