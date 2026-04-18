package service

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type fakeAuditWriter struct {
	mu       sync.Mutex
	stored   []*model.AuditEvent
	failOnce atomic.Int32 // counts forced failures still pending
	err      error
}

func (f *fakeAuditWriter) Insert(_ context.Context, event *model.AuditEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOnce.Load() > 0 {
		f.failOnce.Add(-1)
		return f.err
	}
	f.stored = append(f.stored, event)
	return nil
}

func (f *fakeAuditWriter) snapshot() []*model.AuditEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*model.AuditEvent(nil), f.stored...)
}

func TestAuditSink_DedupsRBACDeniedWithinWindow(t *testing.T) {
	w := &fakeAuditWriter{}
	sink := NewAuditSink(w, AuditSinkConfig{
		QueueCapacity: 16,
		DedupWindow:   1 * time.Second,
	})
	sink.Start(context.Background())
	defer sink.Stop(2 * time.Second)

	actor := uuid.New()
	resourceID := "task-123"

	// Three identical rbac_denied events; only the first should persist.
	for i := 0; i < 3; i++ {
		sink.Enqueue(context.Background(), &model.AuditEvent{
			ID:           uuid.New(),
			ProjectID:    uuid.New(),
			ActorUserID:  &actor,
			ActionID:     "task.dispatch",
			ResourceType: model.AuditResourceTypeAuth,
			ResourceID:   resourceID,
			OccurredAt:   time.Now().UTC(),
		})
	}

	// Allow the worker to drain.
	waitFor(t, 2*time.Second, func() bool {
		return len(w.snapshot()) >= 1
	})
	time.Sleep(100 * time.Millisecond) // give it a moment to process duplicates

	stored := w.snapshot()
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored event after dedup; got %d", len(stored))
	}
}

func TestAuditSink_DoesNotDedupNonAuthEvents(t *testing.T) {
	w := &fakeAuditWriter{}
	sink := NewAuditSink(w, AuditSinkConfig{
		QueueCapacity: 16,
		DedupWindow:   5 * time.Second,
	})
	sink.Start(context.Background())
	defer sink.Stop(2 * time.Second)

	for i := 0; i < 3; i++ {
		sink.Enqueue(context.Background(), &model.AuditEvent{
			ID:           uuid.New(),
			ProjectID:    uuid.New(),
			ActionID:     "task.update",
			ResourceType: model.AuditResourceTypeTask,
			OccurredAt:   time.Now().UTC(),
		})
	}

	waitFor(t, 2*time.Second, func() bool {
		return len(w.snapshot()) >= 3
	})

	stored := w.snapshot()
	if len(stored) != 3 {
		t.Fatalf("expected 3 stored events (no dedup for non-auth); got %d", len(stored))
	}
}

func TestAuditSink_QueueFullSpillsToDisk(t *testing.T) {
	w := &fakeAuditWriter{}
	tmpDir := t.TempDir()
	sink := NewAuditSink(w, AuditSinkConfig{
		QueueCapacity: 1, // tiny queue forces overflow
		SpillFilePath: tmpDir + "/spill.jsonl",
	})
	// Don't Start the sink — that way the queue fills immediately.

	first := &model.AuditEvent{ID: uuid.New(), ActionID: "task.create", ResourceType: model.AuditResourceTypeTask}
	second := &model.AuditEvent{ID: uuid.New(), ActionID: "task.update", ResourceType: model.AuditResourceTypeTask}

	sink.Enqueue(context.Background(), first)
	sink.Enqueue(context.Background(), second) // queue full → spill

	// Spill happens synchronously in Enqueue.
	info, err := readSpillFile(tmpDir + "/spill.jsonl")
	if err != nil {
		t.Fatalf("read spill: %v", err)
	}
	if info == 0 {
		t.Errorf("expected spill file to contain at least one event; got 0 lines")
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("waitFor: condition not met within %s", timeout)
}

func readSpillFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, b := range data {
		if b == '\n' {
			count++
		}
	}
	return count, nil
}
