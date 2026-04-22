package scheduler

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/internal/ws"
)

type schedulerEventCapture struct {
	mu    sync.Mutex
	types []string
	jobs  []*model.ScheduledJob
	runs  []*model.ScheduledJobRun
}

func (c *schedulerEventCapture) BroadcastSchedulerEvent(eventType string, job *model.ScheduledJob, run *model.ScheduledJobRun) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.types = append(c.types, eventType)
	c.jobs = append(c.jobs, job)
	c.runs = append(c.runs, run)
}

func (c *schedulerEventCapture) snapshotTypes() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.types))
	copy(out, c.types)
	return out
}

func (c *schedulerEventCapture) snapshotRuns() []*model.ScheduledJobRun {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*model.ScheduledJobRun, len(c.runs))
	copy(out, c.runs)
	return out
}

func (c *schedulerEventCapture) snapshotJobs() []*model.ScheduledJob {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*model.ScheduledJob, len(c.jobs))
	copy(out, c.jobs)
	return out
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
	types := events.snapshotTypes()
	if len(types) != 1 || types[0] != ws.EventSchedulerJobUpdated {
		t.Fatalf("events.types = %v, want scheduler job update event", types)
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
	types := events.snapshotTypes()
	runs := events.snapshotRuns()
	if len(types) != 2 {
		t.Fatalf("len(events.types) = %d, want 2 lifecycle events", len(types))
	}
	if types[0] != ws.EventSchedulerRunStarted {
		t.Fatalf("events.types[0] = %q, want run started", types[0])
	}
	if types[1] != ws.EventSchedulerRunCompleted {
		t.Fatalf("events.types[1] = %q, want run completed", types[1])
	}
	if runs[1] == nil || runs[1].Status != model.ScheduledJobRunStatusSucceeded {
		t.Fatalf("events.runs[1] = %+v, want completed run payload", runs[1])
	}
}

func TestService_PauseAndResumeJobProjectTruthfulControlState(t *testing.T) {
	ctx := context.Background()
	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
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

	paused := callSchedulerJobMethod(t, service, "PauseJob", ctx, job.JobKey)
	if paused.ControlState != model.ScheduledJobControlStatePaused {
		t.Fatalf("paused.ControlState = %q, want paused", paused.ControlState)
	}
	if paused.Enabled {
		t.Fatal("paused.Enabled = true, want false after pause")
	}
	if paused.NextRunAt != nil {
		t.Fatalf("paused.NextRunAt = %v, want nil while paused", paused.NextRunAt)
	}

	resumed := callSchedulerJobMethod(t, service, "ResumeJob", ctx, job.JobKey)
	if resumed.ControlState != model.ScheduledJobControlStateActive {
		t.Fatalf("resumed.ControlState = %q, want active", resumed.ControlState)
	}
	if !resumed.Enabled {
		t.Fatal("resumed.Enabled = false, want true after resume")
	}
	if resumed.NextRunAt == nil || !resumed.NextRunAt.After(now) {
		t.Fatalf("resumed.NextRunAt = %v, want recalculated next due time after %v", resumed.NextRunAt, now)
	}
}

func TestService_ListJobsProjectsOperatorMetadata(t *testing.T) {
	ctx := context.Background()
	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	next := now.Add(5 * time.Minute)
	job := &model.ScheduledJob{
		JobKey:        "cost-reconcile",
		Name:          "Cost reconcile",
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
	if err := runRepo.Create(ctx, &model.ScheduledJobRun{
		RunID:         "active-run",
		JobKey:        job.JobKey,
		TriggerSource: model.ScheduledJobTriggerManual,
		Status:        model.ScheduledJobRunStatusRunning,
		StartedAt:     now.Add(-30 * time.Second),
		Metrics:       "{}",
	}); err != nil {
		t.Fatalf("runRepo.Create() error = %v", err)
	}

	service := NewService(jobRepo, runRepo)
	service.now = func() time.Time { return now }

	jobs, err := service.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
	got := jobs[0]
	if got.ControlState != model.ScheduledJobControlStateActive {
		t.Fatalf("got.ControlState = %q, want active", got.ControlState)
	}
	if got.ActiveRun == nil || got.ActiveRun.RunID != "active-run" {
		t.Fatalf("got.ActiveRun = %+v, want active run summary", got.ActiveRun)
	}
	if len(got.SupportedActions) == 0 {
		t.Fatal("got.SupportedActions is empty, want operator action metadata")
	}
	if got.ConfigMetadata == nil || !got.ConfigMetadata.Editable {
		t.Fatalf("got.ConfigMetadata = %+v, want editable config metadata", got.ConfigMetadata)
	}
	if len(got.UpcomingRuns) == 0 {
		t.Fatal("got.UpcomingRuns is empty, want schedule preview")
	}
}

func TestService_GetStatsIncludesOperatorMetrics(t *testing.T) {
	ctx := context.Background()
	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)

	activeNext := now.Add(5 * time.Minute)
	pausedNext := now.Add(10 * time.Minute)
	activeJob := &model.ScheduledJob{
		JobKey:        "bridge-health-reconcile",
		Name:          "Bridge health reconcile",
		Scope:         model.ScheduledJobScopeSystem,
		Schedule:      "*/5 * * * *",
		Enabled:       true,
		ExecutionMode: model.ScheduledJobExecutionModeInProcess,
		OverlapPolicy: model.ScheduledJobOverlapSkip,
		NextRunAt:     &activeNext,
		Config:        "{}",
	}
	pausedJob := &model.ScheduledJob{
		JobKey:        "cost-reconcile",
		Name:          "Cost reconcile",
		Scope:         model.ScheduledJobScopeSystem,
		Schedule:      "0 * * * *",
		Enabled:       false,
		ExecutionMode: model.ScheduledJobExecutionModeInProcess,
		OverlapPolicy: model.ScheduledJobOverlapSkip,
		NextRunAt:     &pausedNext,
		Config:        "{}",
	}
	for _, job := range []*model.ScheduledJob{activeJob, pausedJob} {
		if err := jobRepo.Upsert(ctx, job); err != nil {
			t.Fatalf("jobRepo.Upsert(%s) error = %v", job.JobKey, err)
		}
	}

	successStart := now.Add(-2 * time.Hour)
	successEnd := successStart.Add(10 * time.Second)
	if err := runRepo.Create(ctx, &model.ScheduledJobRun{
		RunID:         "success-run",
		JobKey:        activeJob.JobKey,
		TriggerSource: model.ScheduledJobTriggerCron,
		Status:        model.ScheduledJobRunStatusRunning,
		StartedAt:     successStart,
		Metrics:       "{}",
	}); err != nil {
		t.Fatalf("runRepo.Create(success) error = %v", err)
	}
	if err := runRepo.Complete(ctx, "success-run", model.ScheduledJobRunStatusSucceeded, "ok", "", `{}`, successEnd); err != nil {
		t.Fatalf("runRepo.Complete(success) error = %v", err)
	}

	failedStart := now.Add(-90 * time.Minute)
	failedEnd := failedStart.Add(5 * time.Second)
	if err := runRepo.Create(ctx, &model.ScheduledJobRun{
		RunID:         "failed-run",
		JobKey:        activeJob.JobKey,
		TriggerSource: model.ScheduledJobTriggerManual,
		Status:        model.ScheduledJobRunStatusRunning,
		StartedAt:     failedStart,
		Metrics:       "{}",
	}); err != nil {
		t.Fatalf("runRepo.Create(failed) error = %v", err)
	}
	if err := runRepo.Complete(ctx, "failed-run", model.ScheduledJobRunStatusFailed, "boom", "boom", `{}`, failedEnd); err != nil {
		t.Fatalf("runRepo.Complete(failed) error = %v", err)
	}

	if err := runRepo.Create(ctx, &model.ScheduledJobRun{
		RunID:         "queued-run",
		JobKey:        activeJob.JobKey,
		TriggerSource: model.ScheduledJobTriggerManual,
		Status:        model.ScheduledJobRunStatusRunning,
		StartedAt:     now.Add(-48 * time.Hour),
		Metrics:       "{}",
	}); err != nil {
		t.Fatalf("runRepo.Create(queued) error = %v", err)
	}

	service := NewService(jobRepo, runRepo)
	service.now = func() time.Time { return now }

	stats, err := service.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats.PausedJobs != 1 {
		t.Fatalf("stats.PausedJobs = %d, want 1", stats.PausedJobs)
	}
	if stats.QueueDepth != 1 {
		t.Fatalf("stats.QueueDepth = %d, want 1", stats.QueueDepth)
	}
	if stats.SuccessfulRuns24h != 1 {
		t.Fatalf("stats.SuccessfulRuns24h = %d, want 1", stats.SuccessfulRuns24h)
	}
	if stats.AverageDurationMs != 7500 {
		t.Fatalf("stats.AverageDurationMs = %d, want 7500", stats.AverageDurationMs)
	}
	if stats.SuccessRate24h != 50 {
		t.Fatalf("stats.SuccessRate24h = %v, want 50", stats.SuccessRate24h)
	}
}

func TestService_CancelJobMarksLifecycleAndBroadcastsTransitions(t *testing.T) {
	ctx := context.Background()
	jobRepo := repository.NewScheduledJobRepository()
	runRepo := repository.NewScheduledJobRunRepository()
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	next := now.Add(5 * time.Minute)
	job := &model.ScheduledJob{
		JobKey:        "cost-reconcile",
		Name:          "Cost reconcile",
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

	started := make(chan struct{})
	done := make(chan *model.ScheduledJobRun, 1)
	service.RegisterHandler(job.JobKey, func(ctx context.Context, job *model.ScheduledJob, run *model.ScheduledJobRun) (*RunResult, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})

	go func() {
		run, err := service.TriggerManual(ctx, job.JobKey)
		if err != nil {
			t.Errorf("TriggerManual() error = %v", err)
			done <- nil
			return
		}
		done <- run
	}()

	<-started
	cancelRequested := callSchedulerRunMethod(t, service, "CancelJob", ctx, job.JobKey)
	if cancelRequested.Status != model.ScheduledJobRunStatusCancelRequested {
		t.Fatalf("cancelRequested.Status = %q, want cancel_requested", cancelRequested.Status)
	}

	finalRun := <-done
	if finalRun == nil {
		t.Fatal("finalRun = nil, want cancelled run")
	}
	if finalRun.Status != model.ScheduledJobRunStatusCancelled {
		t.Fatalf("finalRun.Status = %q, want cancelled", finalRun.Status)
	}

	storedRuns, err := runRepo.ListByJobKey(ctx, job.JobKey, 10)
	if err != nil {
		t.Fatalf("runRepo.ListByJobKey() error = %v", err)
	}
	if len(storedRuns) != 1 || storedRuns[0].Status != model.ScheduledJobRunStatusCancelled {
		t.Fatalf("storedRuns = %+v, want persisted cancelled run", storedRuns)
	}

	types := events.snapshotTypes()
	if len(types) != 3 {
		t.Fatalf("events.types = %v, want 3 lifecycle events", types)
	}
	if types[1] != "scheduler.run.cancel_requested" {
		t.Fatalf("events.types[1] = %q, want cancel-requested event", types[1])
	}
	if types[2] != ws.EventSchedulerRunCompleted {
		t.Fatalf("events.types[2] = %q, want completion event", types[2])
	}
}

func stringPointer(value string) *string {
	return &value
}

func callSchedulerJobMethod(t *testing.T, service *Service, methodName string, ctx context.Context, jobKey string) *model.ScheduledJob {
	t.Helper()

	method := reflect.ValueOf(service).MethodByName(methodName)
	if !method.IsValid() {
		t.Fatalf("Service is missing method %s", methodName)
	}

	results := method.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(jobKey),
	})
	if len(results) != 2 {
		t.Fatalf("%s returned %d results, want 2", methodName, len(results))
	}

	if !results[1].IsNil() {
		t.Fatalf("%s() error = %v", methodName, results[1].Interface())
	}

	job, ok := results[0].Interface().(*model.ScheduledJob)
	if !ok {
		t.Fatalf("%s() first result = %T, want *model.ScheduledJob", methodName, results[0].Interface())
	}
	return job
}

func callSchedulerRunMethod(t *testing.T, service *Service, methodName string, ctx context.Context, jobKey string) *model.ScheduledJobRun {
	t.Helper()

	method := reflect.ValueOf(service).MethodByName(methodName)
	if !method.IsValid() {
		t.Fatalf("Service is missing method %s", methodName)
	}

	results := method.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(jobKey),
	})
	if len(results) != 2 {
		t.Fatalf("%s returned %d results, want 2", methodName, len(results))
	}

	if !results[1].IsNil() {
		t.Fatalf("%s() error = %v", methodName, results[1].Interface())
	}

	run, ok := results[0].Interface().(*model.ScheduledJobRun)
	if !ok {
		t.Fatalf("%s() first result = %T, want *model.ScheduledJobRun", methodName, results[0].Interface())
	}
	return run
}
