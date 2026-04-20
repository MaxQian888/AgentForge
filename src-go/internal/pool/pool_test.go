package pool_test

import (
	"errors"
	"testing"

	"github.com/agentforge/server/internal/pool"
)

func TestPoolAcquireReleaseLifecycle(t *testing.T) {
	p := pool.NewPool(2)

	if err := p.Acquire("run-1", "task-1", "member-1"); err != nil {
		t.Fatalf("Acquire(run-1) error = %v", err)
	}
	if err := p.Acquire("run-2", "task-2", "member-2"); err != nil {
		t.Fatalf("Acquire(run-2) error = %v", err)
	}
	if err := p.Acquire("run-3", "task-3", "member-3"); !errors.Is(err, pool.ErrPoolFull) {
		t.Fatalf("Acquire(run-3) error = %v, want ErrPoolFull", err)
	}

	if got := p.ActiveCount(); got != 2 {
		t.Errorf("ActiveCount() = %d, want 2", got)
	}
	if got := p.Available(); got != 0 {
		t.Errorf("Available() = %d, want 0", got)
	}
	if !p.Has("run-1") || !p.Has("run-2") {
		t.Fatalf("Has() did not report active runs")
	}

	list := p.List()
	if len(list) != 2 {
		t.Fatalf("len(List()) = %d, want 2", len(list))
	}
	gotByRun := map[string]pool.AgentEntry{}
	for _, entry := range list {
		gotByRun[entry.RunID] = entry
	}
	if gotByRun["run-1"].TaskID != "task-1" || gotByRun["run-2"].MemberID != "member-2" {
		t.Fatalf("List() returned unexpected entries: %+v", gotByRun)
	}

	if err := p.Release("run-1"); err != nil {
		t.Fatalf("Release(run-1) error = %v", err)
	}
	if err := p.Release("run-1"); !errors.Is(err, pool.ErrNotInPool) {
		t.Fatalf("Release(run-1) second error = %v, want ErrNotInPool", err)
	}

	if p.Has("run-1") {
		t.Fatal("Has(run-1) = true after release, want false")
	}
	if got := p.ActiveCount(); got != 1 {
		t.Errorf("ActiveCount() after release = %d, want 1", got)
	}
	if got := p.Available(); got != 1 {
		t.Errorf("Available() after release = %d, want 1", got)
	}
}
