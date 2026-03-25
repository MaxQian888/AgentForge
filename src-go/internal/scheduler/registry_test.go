package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

func TestRegistry_ReconcileMaterializesBuiltInJobs(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewScheduledJobRepository()
	registry := NewRegistry(repo, []CatalogEntry{
		{
			JobKey:        "task-progress-detector",
			Name:          "Task progress detector",
			Scope:         model.ScheduledJobScopeSystem,
			Schedule:      "*/5 * * * *",
			Enabled:       true,
			ExecutionMode: model.ScheduledJobExecutionModeInProcess,
			OverlapPolicy: model.ScheduledJobOverlapSkip,
		},
		{
			JobKey:        "worktree-garbage-collector",
			Name:          "Worktree garbage collector",
			Scope:         model.ScheduledJobScopeSystem,
			Schedule:      "0 * * * *",
			Enabled:       true,
			ExecutionMode: model.ScheduledJobExecutionModeInProcess,
			OverlapPolicy: model.ScheduledJobOverlapSkip,
		},
	})

	jobs, err := registry.Reconcile(ctx)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("len(jobs) = %d, want 2", len(jobs))
	}
	if jobs[0].JobKey != "task-progress-detector" {
		t.Fatalf("jobs[0].JobKey = %q, want task-progress-detector", jobs[0].JobKey)
	}
	if jobs[0].NextRunAt == nil {
		t.Fatal("jobs[0].NextRunAt = nil, want next due time")
	}
}

func TestRegistry_ReconcileRejectsInvalidSchedules(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewScheduledJobRepository()
	registry := NewRegistry(repo, []CatalogEntry{
		{
			JobKey:        "task-progress-detector",
			Name:          "Task progress detector",
			Scope:         model.ScheduledJobScopeSystem,
			Schedule:      "not-a-valid-schedule",
			Enabled:       true,
			ExecutionMode: model.ScheduledJobExecutionModeInProcess,
			OverlapPolicy: model.ScheduledJobOverlapSkip,
		},
	})

	if _, err := registry.Reconcile(ctx); err == nil {
		t.Fatal("Reconcile() error = nil, want invalid schedule error")
	}
}

func TestRegistry_ReconcilePreservesExistingRunMetadata(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewScheduledJobRepository()
	lastRunAt := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	existing := &model.ScheduledJob{
		JobKey:         "task-progress-detector",
		Name:           "Old name",
		Scope:          model.ScheduledJobScopeSystem,
		Schedule:       "*/10 * * * *",
		Enabled:        false,
		ExecutionMode:  model.ScheduledJobExecutionModeInProcess,
		OverlapPolicy:  model.ScheduledJobOverlapSkip,
		LastRunStatus:  model.ScheduledJobRunStatusFailed,
		LastRunAt:      &lastRunAt,
		LastRunSummary: "failed previously",
		LastError:      "boom",
		Config:         `{"manual":true}`,
	}
	if err := repo.Upsert(ctx, existing); err != nil {
		t.Fatalf("seed Upsert() error = %v", err)
	}

	registry := NewRegistry(repo, []CatalogEntry{
		{
			JobKey:        "task-progress-detector",
			Name:          "Task progress detector",
			Scope:         model.ScheduledJobScopeSystem,
			Schedule:      "*/5 * * * *",
			Enabled:       true,
			ExecutionMode: model.ScheduledJobExecutionModeInProcess,
			OverlapPolicy: model.ScheduledJobOverlapSkip,
		},
	})

	jobs, err := registry.Reconcile(ctx)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	job := jobs[0]
	if job.LastRunStatus != model.ScheduledJobRunStatusFailed {
		t.Fatalf("job.LastRunStatus = %q, want failed", job.LastRunStatus)
	}
	if job.LastRunAt == nil || !job.LastRunAt.Equal(lastRunAt) {
		t.Fatalf("job.LastRunAt = %v, want %v", job.LastRunAt, lastRunAt)
	}
	if job.LastRunSummary != "failed previously" {
		t.Fatalf("job.LastRunSummary = %q, want preserved summary", job.LastRunSummary)
	}
	if job.LastError != "boom" {
		t.Fatalf("job.LastError = %q, want boom", job.LastError)
	}
}

func TestBuiltInCatalog_UsesStableJobKeysAndSchedules(t *testing.T) {
	catalog := BuiltInCatalog(CatalogConfig{
		TaskProgressDetectorInterval: time.Hour,
		ExecutionMode:                model.ScheduledJobExecutionModeOSRegistered,
	})
	if len(catalog) < 4 {
		t.Fatalf("len(catalog) = %d, want builtin jobs", len(catalog))
	}
	if catalog[0].JobKey != "task-progress-detector" {
		t.Fatalf("catalog[0].JobKey = %q, want task-progress-detector", catalog[0].JobKey)
	}
	if catalog[0].Schedule != "0 * * * *" {
		t.Fatalf("catalog[0].Schedule = %q, want hourly schedule derived from config", catalog[0].Schedule)
	}
	if catalog[1].JobKey != "worktree-garbage-collector" {
		t.Fatalf("catalog[1].JobKey = %q, want worktree-garbage-collector", catalog[1].JobKey)
	}
	if catalog[0].ExecutionMode != model.ScheduledJobExecutionModeOSRegistered {
		t.Fatalf("catalog[0].ExecutionMode = %q, want os_registered", catalog[0].ExecutionMode)
	}
}
