package service_test

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	applog "github.com/agentforge/server/internal/log"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/agentforge/server/internal/ws"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// syncBuffer wraps bytes.Buffer with a mutex so logrus writes from background
// goroutines and the test's String() reads do not race.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// TestAgentService_BackgroundGoroutineInheritsParentTrace verifies the
// inherit-or-generate trace pattern on the DAG completion goroutine site.
//
// When the caller ctx carries a trace_id:
//   - The goroutine must inherit it (no new trace generated, no log emitted).
//
// When the caller ctx has no trace_id:
//   - The goroutine must generate a fresh trace and emit
//     "trace.generated_for_background_job".
func TestAgentService_BackgroundGoroutineInheritsParentTrace(t *testing.T) {
	// DAGWorkflowService with all-nil repos: HandleAgentRunCompletion returns
	// immediately when mappingRepo == nil, so the goroutine is very fast.
	dagSvc := service.NewDAGWorkflowService(nil, nil, nil, nil, nil, nil, nil)

	setup := func(t *testing.T) (svc *service.AgentService, runID uuid.UUID) {
		t.Helper()
		taskID := uuid.New()
		runID = uuid.New()

		repo := newMockAgentRunRepo()
		repo.runs[runID] = &model.AgentRun{
			ID:     runID,
			TaskID: taskID,
			Status: model.AgentRunStatusRunning,
		}

		svc = service.NewAgentService(
			repo,
			nil,  // taskRepo — nil causes releaseTaskRuntime to return nil immediately
			nil,  // projectRepo
			ws.NewHub(),
			nil, // eventbus
			&mockAgentBridge{},
			&mockWorktreeManager{},
			nil,
		)
		svc.SetDAGWorkflowService(dagSvc)
		return svc, runID
	}

	waitForLog := func(t *testing.T, buf *syncBuffer, wantPresent bool, marker string) {
		t.Helper()
		deadline := time.Now().Add(500 * time.Millisecond)
		for time.Now().Before(deadline) {
			got := buf.String()
			if wantPresent && strings.Contains(got, marker) {
				return
			}
			if !wantPresent {
				// Give the goroutine time to run, then verify absence.
				time.Sleep(50 * time.Millisecond)
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		got := buf.String()
		if wantPresent && !strings.Contains(got, marker) {
			t.Errorf("expected log to contain %q but it did not; output:\n%s", marker, got)
		}
		if !wantPresent && strings.Contains(got, marker) {
			t.Errorf("expected log NOT to contain %q but it did; output:\n%s", marker, got)
		}
	}

	t.Run("inherits parent trace — no generation log", func(t *testing.T) {
		var buf syncBuffer
		old := log.StandardLogger().Out
		log.SetOutput(&buf)
		t.Cleanup(func() { log.SetOutput(old) })

		svc, runID := setup(t)

		const parentTrace = "tr_parent0000000000000000"
		ctx := applog.WithTrace(context.Background(), parentTrace)

		if err := svc.UpdateStatus(ctx, runID, model.AgentRunStatusCompleted); err != nil {
			t.Fatalf("UpdateStatus() error = %v", err)
		}

		// Wait long enough for the goroutine to run, then assert absence.
		waitForLog(t, &buf, false, "trace.generated_for_background_job")
	})

	t.Run("no parent trace — generates fresh trace and logs it", func(t *testing.T) {
		var buf syncBuffer
		old := log.StandardLogger().Out
		log.SetOutput(&buf)
		t.Cleanup(func() { log.SetOutput(old) })

		svc, runID := setup(t)

		// ctx carries no trace_id
		if err := svc.UpdateStatus(context.Background(), runID, model.AgentRunStatusCompleted); err != nil {
			t.Fatalf("UpdateStatus() error = %v", err)
		}

		// Wait up to 500ms for the goroutine to emit the log line.
		waitForLog(t, &buf, true, "trace.generated_for_background_job")
	})
}
