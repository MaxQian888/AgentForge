package scheduler

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
)

// TestRunLoop_TickAssignsTraceID verifies that RunLoop emits a trace_id log
// line on each tick when the incoming ctx carries no existing trace_id.
func TestRunLoop_TickAssignsTraceID(t *testing.T) {
	var buf bytes.Buffer
	old := log.StandardLogger().Out
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(old) })

	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	dueAt := now.Add(-1 * time.Minute)

	job := &model.ScheduledJob{
		JobKey:        "test-trace-job",
		Name:          "Trace test job",
		Scope:         model.ScheduledJobScopeSystem,
		Schedule:      "*/5 * * * *",
		Enabled:       true,
		ExecutionMode: model.ScheduledJobExecutionModeInProcess,
		OverlapPolicy: model.ScheduledJobOverlapSkip,
		NextRunAt:     &dueAt,
		Config:        "{}",
	}
	if err := jobRepo.Upsert(context.Background(), job); err != nil {
		t.Fatalf("jobRepo.Upsert() error = %v", err)
	}

	svc := NewService(jobRepo, runRepo)
	svc.now = func() time.Time { return now }
	svc.RegisterHandler(job.JobKey, func(_ context.Context, _ *model.ScheduledJob, _ *model.ScheduledJobRun) (*RunResult, error) {
		return &RunResult{Summary: "ok", Metrics: `{}`}, nil
	})

	// Run the loop with a very short interval; cancel after first tick fires.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	RunLoop(ctx, 10*time.Millisecond, svc)

	out := buf.String()
	if !strings.Contains(out, "trace_id=tr_") && !strings.Contains(out, `"trace_id":"tr_`) {
		t.Fatalf("scheduler tick did not emit a trace_id; log output:\n%s", out)
	}
}
