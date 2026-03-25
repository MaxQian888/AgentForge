package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type schedulerEventCapture struct {
	types []string
	jobs  []*model.ScheduledJob
	runs  []*model.ScheduledJobRun
}

func (c *schedulerEventCapture) BroadcastSchedulerEvent(eventType string, job *model.ScheduledJob, run *model.ScheduledJobRun) {
	c.types = append(c.types, eventType)
	c.jobs = append(c.jobs, job)
	c.runs = append(c.runs, run)
}

func TestService_RunDueExecutesEligibleJobsAndFinalizesState(t *testing.T) {
	ctx := context.Background()
	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	dueAt := now.Add(-1 * time.Minute)
	job := &model.ScheduledJob{
		JobKey:        "task-progress-detector",
		Name:          "Task progress detector",
		Scope:         model.ScheduledJobScopeSystem,
		Schedule:      "*/5 * * * *",
		Enabled:       true,
		ExecutionMode: model.ScheduledJobExecutionModeInProcess,
		OverlapPolicy: model.ScheduledJobOverlapSkip,
		NextRunAt:     &dueAt,
		Config:        "{}",
	}
	if err := jobRepo.Upsert(ctx, job); err != nil {
		t.Fatalf("jobRepo.Upsert() error = %v", err)
	}

	service := NewService(jobRepo, runRepo)
	service.now = func() time.Time { return now }
	service.RegisterHandler(job.JobKey, func(ctx context.Context, job *model.ScheduledJob, run *model.ScheduledJobRun) (*RunResult, error) {
		return &RunResult{
			Summary: "checked 12 tasks",
			Metrics: `{"checkedTasks":12}`,
		}, nil
	})

	runs, err := service.RunDue(ctx)
	if err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if runs[0].TriggerSource != model.ScheduledJobTriggerCron {
		t.Fatalf("runs[0].TriggerSource = %q, want cron", runs[0].TriggerSource)
	}
	if runs[0].Status != model.ScheduledJobRunStatusSucceeded {
		t.Fatalf("runs[0].Status = %q, want succeeded", runs[0].Status)
	}
	if runs[0].Metrics != `{"checkedTasks":12}` {
		t.Fatalf("runs[0].Metrics = %q, want persisted metrics", runs[0].Metrics)
	}

	storedJob, err := jobRepo.GetByKey(ctx, job.JobKey)
	if err != nil {
		t.Fatalf("jobRepo.GetByKey() error = %v", err)
	}
	if storedJob.LastRunStatus != model.ScheduledJobRunStatusSucceeded {
		t.Fatalf("storedJob.LastRunStatus = %q, want succeeded", storedJob.LastRunStatus)
	}
	if storedJob.LastRunSummary != "checked 12 tasks" {
		t.Fatalf("storedJob.LastRunSummary = %q, want execution summary", storedJob.LastRunSummary)
	}
	if storedJob.NextRunAt == nil || !storedJob.NextRunAt.After(now) {
		t.Fatalf("storedJob.NextRunAt = %v, want advanced next due time after %v", storedJob.NextRunAt, now)
	}
}

func TestService_RunDueSkipsSingletonJobsWithActiveRun(t *testing.T) {
	ctx := context.Background()
	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	dueAt := now.Add(-1 * time.Minute)
	job := &model.ScheduledJob{
		JobKey:        "task-progress-detector",
		Name:          "Task progress detector",
		Scope:         model.ScheduledJobScopeSystem,
		Schedule:      "*/5 * * * *",
		Enabled:       true,
		ExecutionMode: model.ScheduledJobExecutionModeInProcess,
		OverlapPolicy: model.ScheduledJobOverlapSkip,
		NextRunAt:     &dueAt,
		Config:        "{}",
	}
	if err := jobRepo.Upsert(ctx, job); err != nil {
		t.Fatalf("jobRepo.Upsert() error = %v", err)
	}
	if err := runRepo.Create(ctx, &model.ScheduledJobRun{
		RunID:         "active-run",
		JobKey:        job.JobKey,
		TriggerSource: model.ScheduledJobTriggerCron,
		Status:        model.ScheduledJobRunStatusRunning,
		StartedAt:     now.Add(-30 * time.Second),
		Metrics:       "{}",
	}); err != nil {
		t.Fatalf("runRepo.Create() error = %v", err)
	}

	calls := 0
	service := NewService(jobRepo, runRepo)
	service.now = func() time.Time { return now }
	service.RegisterHandler(job.JobKey, func(ctx context.Context, job *model.ScheduledJob, run *model.ScheduledJobRun) (*RunResult, error) {
		calls++
		return &RunResult{Summary: "should not run", Metrics: "{}"}, nil
	})

	runs, err := service.RunDue(ctx)
	if err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("len(runs) = %d, want 0 because active singleton run blocks overlap", len(runs))
	}
	if calls != 0 {
		t.Fatalf("handler calls = %d, want 0", calls)
	}
}

func TestService_RunDueSkipsOSRegisteredJobs(t *testing.T) {
	ctx := context.Background()
	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	dueAt := now.Add(-1 * time.Minute)
	job := &model.ScheduledJob{
		JobKey:        "task-progress-detector",
		Name:          "Task progress detector",
		Scope:         model.ScheduledJobScopeSystem,
		Schedule:      "*/5 * * * *",
		Enabled:       true,
		ExecutionMode: model.ScheduledJobExecutionModeOSRegistered,
		OverlapPolicy: model.ScheduledJobOverlapSkip,
		NextRunAt:     &dueAt,
		Config:        "{}",
	}
	if err := jobRepo.Upsert(ctx, job); err != nil {
		t.Fatalf("jobRepo.Upsert() error = %v", err)
	}

	calls := 0
	service := NewService(jobRepo, runRepo)
	service.now = func() time.Time { return now }
	service.RegisterHandler(job.JobKey, func(ctx context.Context, job *model.ScheduledJob, run *model.ScheduledJobRun) (*RunResult, error) {
		calls++
		return &RunResult{Summary: "should not run", Metrics: "{}"}, nil
	})

	runs, err := service.RunDue(ctx)
	if err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("len(runs) = %d, want 0 for os_registered jobs", len(runs))
	}
	if calls != 0 {
		t.Fatalf("handler calls = %d, want 0", calls)
	}
}

func TestService_TriggerManualExecutesImmediatelyEvenWhenNotDue(t *testing.T) {
	ctx := context.Background()
	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	future := now.Add(1 * time.Hour)
	job := &model.ScheduledJob{
		JobKey:        "bridge-health-reconcile",
		Name:          "Bridge health reconcile",
		Scope:         model.ScheduledJobScopeSystem,
		Schedule:      "0 * * * *",
		Enabled:       false,
		ExecutionMode: model.ScheduledJobExecutionModeInProcess,
		OverlapPolicy: model.ScheduledJobOverlapSkip,
		NextRunAt:     &future,
		Config:        "{}",
	}
	if err := jobRepo.Upsert(ctx, job); err != nil {
		t.Fatalf("jobRepo.Upsert() error = %v", err)
	}

	service := NewService(jobRepo, runRepo)
	service.now = func() time.Time { return now }
	service.RegisterHandler(job.JobKey, func(ctx context.Context, job *model.ScheduledJob, run *model.ScheduledJobRun) (*RunResult, error) {
		return &RunResult{
			Summary: "checked bridge state",
			Metrics: `{"bridges":3}`,
		}, nil
	})

	run, err := service.TriggerManual(ctx, job.JobKey)
	if err != nil {
		t.Fatalf("TriggerManual() error = %v", err)
	}
	if run.TriggerSource != model.ScheduledJobTriggerManual {
		t.Fatalf("run.TriggerSource = %q, want manual", run.TriggerSource)
	}
	if run.Status != model.ScheduledJobRunStatusSucceeded {
		t.Fatalf("run.Status = %q, want succeeded", run.Status)
	}
}

func TestService_TriggerManualFinalizesFailedRuns(t *testing.T) {
	ctx := context.Background()
	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	future := now.Add(5 * time.Minute)
	job := &model.ScheduledJob{
		JobKey:        "cost-reconcile",
		Name:          "Cost reconcile",
		Scope:         model.ScheduledJobScopeSystem,
		Schedule:      "*/5 * * * *",
		Enabled:       true,
		ExecutionMode: model.ScheduledJobExecutionModeInProcess,
		OverlapPolicy: model.ScheduledJobOverlapSkip,
		NextRunAt:     &future,
		Config:        "{}",
	}
	if err := jobRepo.Upsert(ctx, job); err != nil {
		t.Fatalf("jobRepo.Upsert() error = %v", err)
	}

	service := NewService(jobRepo, runRepo)
	service.now = func() time.Time { return now }
	service.RegisterHandler(job.JobKey, func(ctx context.Context, job *model.ScheduledJob, run *model.ScheduledJobRun) (*RunResult, error) {
		return nil, errors.New("budget drift query failed")
	})

	run, err := service.TriggerManual(ctx, job.JobKey)
	if err != nil {
		t.Fatalf("TriggerManual() error = %v", err)
	}
	if run.Status != model.ScheduledJobRunStatusFailed {
		t.Fatalf("run.Status = %q, want failed", run.Status)
	}
	if run.ErrorMessage != "budget drift query failed" {
		t.Fatalf("run.ErrorMessage = %q, want failure message", run.ErrorMessage)
	}

	storedJob, err := jobRepo.GetByKey(ctx, job.JobKey)
	if err != nil {
		t.Fatalf("jobRepo.GetByKey() error = %v", err)
	}
	if storedJob.LastRunStatus != model.ScheduledJobRunStatusFailed {
		t.Fatalf("storedJob.LastRunStatus = %q, want failed", storedJob.LastRunStatus)
	}
	if storedJob.LastError != "budget drift query failed" {
		t.Fatalf("storedJob.LastError = %q, want failure message", storedJob.LastError)
	}
}

func TestService_TriggerCronExecutesThroughCanonicalRunLifecycle(t *testing.T) {
	ctx := context.Background()
	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	future := now.Add(1 * time.Hour)
	job := &model.ScheduledJob{
		JobKey:        "bridge-health-reconcile",
		Name:          "Bridge health reconcile",
		Scope:         model.ScheduledJobScopeSystem,
		Schedule:      "0 * * * *",
		Enabled:       true,
		ExecutionMode: model.ScheduledJobExecutionModeOSRegistered,
		OverlapPolicy: model.ScheduledJobOverlapSkip,
		NextRunAt:     &future,
		Config:        "{}",
	}
	if err := jobRepo.Upsert(ctx, job); err != nil {
		t.Fatalf("jobRepo.Upsert() error = %v", err)
	}

	service := NewService(jobRepo, runRepo)
	service.now = func() time.Time { return now }
	service.RegisterHandler(job.JobKey, func(ctx context.Context, job *model.ScheduledJob, run *model.ScheduledJobRun) (*RunResult, error) {
		return &RunResult{Summary: "bridge checked", Metrics: `{"bridges":1}`}, nil
	})

	run, err := service.TriggerCron(ctx, job.JobKey)
	if err != nil {
		t.Fatalf("TriggerCron() error = %v", err)
	}
	if run.TriggerSource != model.ScheduledJobTriggerCron {
		t.Fatalf("run.TriggerSource = %q, want cron", run.TriggerSource)
	}
	if run.Status != model.ScheduledJobRunStatusSucceeded {
		t.Fatalf("run.Status = %q, want succeeded", run.Status)
	}
}

func TestService_UpdateJobRejectsInvalidScheduleAndPreservesExistingValue(t *testing.T) {
	ctx := context.Background()
	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	next := now.Add(5 * time.Minute)
	job := &model.ScheduledJob{
		JobKey:        "task-progress-detector",
		Name:          "Task progress detector",
		Scope:         model.ScheduledJobScopeSystem,
		Schedule:      "*/5 * * * *",
		Enabled:       true,
		ExecutionMode: model.ScheduledJobExecutionModeInProcess,
		OverlapPolicy: model.ScheduledJobOverlapSkip,
		NextRunAt:     &next,
		Config:        "{}",
	}
	if err := jobRepo.Upsert(ctx, job); err != nil {
		t.Fatalf("jobRepo.Upsert() error = %v", err)
	}

	service := NewService(jobRepo, runRepo)
	service.now = func() time.Time { return now }
	input := UpdateJobInput{
		Schedule: stringPointer("not-a-valid-schedule"),
	}

	if _, err := service.UpdateJob(ctx, job.JobKey, input); err == nil {
		t.Fatal("UpdateJob() error = nil, want invalid schedule error")
	}

	storedJob, err := jobRepo.GetByKey(ctx, job.JobKey)
	if err != nil {
		t.Fatalf("jobRepo.GetByKey() error = %v", err)
	}
	if storedJob.Schedule != "*/5 * * * *" {
		t.Fatalf("storedJob.Schedule = %q, want preserved existing schedule", storedJob.Schedule)
	}
}

func TestService_UpdateJobAppliesSupportedScheduleAndEnableStateChanges(t *testing.T) {
	ctx := context.Background()
	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	next := now.Add(5 * time.Minute)
	job := &model.ScheduledJob{
		JobKey:        "task-progress-detector",
		Name:          "Task progress detector",
		Scope:         model.ScheduledJobScopeSystem,
		Schedule:      "*/5 * * * *",
		Enabled:       true,
		ExecutionMode: model.ScheduledJobExecutionModeInProcess,
		OverlapPolicy: model.ScheduledJobOverlapSkip,
		NextRunAt:     &next,
		Config:        "{}",
	}
	if err := jobRepo.Upsert(ctx, job); err != nil {
		t.Fatalf("jobRepo.Upsert() error = %v", err)
	}

	service := NewService(jobRepo, runRepo)
	service.now = func() time.Time { return now }
	enabled := false
	updated, err := service.UpdateJob(ctx, job.JobKey, UpdateJobInput{
		Enabled:  &enabled,
		Schedule: stringPointer("0 * * * *"),
	})
	if err != nil {
		t.Fatalf("UpdateJob() error = %v", err)
	}
	if updated.Enabled {
		t.Fatal("updated.Enabled = true, want false")
	}
	if updated.Schedule != "0 * * * *" {
		t.Fatalf("updated.Schedule = %q, want updated schedule", updated.Schedule)
	}
	if updated.NextRunAt != nil {
		t.Fatalf("updated.NextRunAt = %v, want nil when disabled", updated.NextRunAt)
	}
}

func TestService_UpdateJobBroadcastsLifecycleChanges(t *testing.T) {
	ctx := context.Background()
	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	next := now.Add(5 * time.Minute)
	job := &model.ScheduledJob{
		JobKey:        "task-progress-detector",
		Name:          "Task progress detector",
		Scope:         model.ScheduledJobScopeSystem,
		Schedule:      "*/5 * * * *",
		Enabled:       true,
		ExecutionMode: model.ScheduledJobExecutionModeInProcess,
		OverlapPolicy: model.ScheduledJobOverlapSkip,
		NextRunAt:     &next,
		Config:        "{}",
	}
	if err := jobRepo.Upsert(ctx, job); err != nil {
		t.Fatalf("jobRepo.Upsert() error = %v", err)
	}

	events := &schedulerEventCapture{}
	service := NewService(jobRepo, runRepo)
	service.now = func() time.Time { return now }
	service.SetBroadcaster(events)
	enabled := false

	if _, err := service.UpdateJob(ctx, job.JobKey, UpdateJobInput{Enabled: &enabled}); err != nil {
		t.Fatalf("UpdateJob() error = %v", err)
	}
	if len(events.types) != 1 || events.types[0] != ws.EventSchedulerJobUpdated {
		t.Fatalf("events.types = %v, want scheduler job update event", events.types)
	}
}

func TestService_TriggerManualBroadcastsRunLifecycle(t *testing.T) {
	ctx := context.Background()
	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	future := now.Add(5 * time.Minute)
	job := &model.ScheduledJob{
		JobKey:        "cost-reconcile",
		Name:          "Cost reconcile",
		Scope:         model.ScheduledJobScopeSystem,
		Schedule:      "*/5 * * * *",
		Enabled:       true,
		ExecutionMode: model.ScheduledJobExecutionModeInProcess,
		OverlapPolicy: model.ScheduledJobOverlapSkip,
		NextRunAt:     &future,
		Config:        "{}",
	}
	if err := jobRepo.Upsert(ctx, job); err != nil {
		t.Fatalf("jobRepo.Upsert() error = %v", err)
	}

	events := &schedulerEventCapture{}
	service := NewService(jobRepo, runRepo)
	service.now = func() time.Time { return now }
	service.SetBroadcaster(events)
	service.RegisterHandler(job.JobKey, func(ctx context.Context, job *model.ScheduledJob, run *model.ScheduledJobRun) (*RunResult, error) {
		return &RunResult{Summary: "costs reconciled", Metrics: `{"projects":2}`}, nil
	})

	run, err := service.TriggerManual(ctx, job.JobKey)
	if err != nil {
		t.Fatalf("TriggerManual() error = %v", err)
	}
	if run.Status != model.ScheduledJobRunStatusSucceeded {
		t.Fatalf("run.Status = %q, want succeeded", run.Status)
	}
	if len(events.types) != 2 {
		t.Fatalf("len(events.types) = %d, want 2 lifecycle events", len(events.types))
	}
	if events.types[0] != ws.EventSchedulerRunStarted {
		t.Fatalf("events.types[0] = %q, want run started", events.types[0])
	}
	if events.types[1] != ws.EventSchedulerRunCompleted {
		t.Fatalf("events.types[1] = %q, want run completed", events.types[1])
	}
	if events.runs[1] == nil || events.runs[1].Status != model.ScheduledJobRunStatusSucceeded {
		t.Fatalf("events.runs[1] = %+v, want completed run payload", events.runs[1])
	}
}

func stringPointer(value string) *string {
	return &value
}
