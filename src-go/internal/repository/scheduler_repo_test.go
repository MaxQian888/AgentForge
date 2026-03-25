package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

func TestScheduledJobRepository_UpsertAndListInMemory(t *testing.T) {
	ctx := context.Background()
	repo := NewScheduledJobRepository()
	now := time.Date(2026, 3, 25, 8, 0, 0, 0, time.UTC)
	next := now.Add(5 * time.Minute)

	job := &model.ScheduledJob{
		JobKey:         "task-progress-detector",
		Name:           "Task progress detector",
		Scope:          model.ScheduledJobScopeSystem,
		Schedule:       "*/5 * * * *",
		Enabled:        true,
		ExecutionMode:  model.ScheduledJobExecutionModeInProcess,
		OverlapPolicy:  model.ScheduledJobOverlapSkip,
		LastRunStatus:  model.ScheduledJobRunStatusSucceeded,
		LastRunAt:      &now,
		NextRunAt:      &next,
		LastRunSummary: "checked 12 tasks",
		Config:         `{"warningAfter":"2h"}`,
	}

	if err := repo.Upsert(ctx, job); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	stored, err := repo.GetByKey(ctx, job.JobKey)
	if err != nil {
		t.Fatalf("GetByKey() error = %v", err)
	}
	if stored.JobKey != job.JobKey {
		t.Fatalf("stored.JobKey = %q, want %q", stored.JobKey, job.JobKey)
	}
	if stored.LastRunSummary != job.LastRunSummary {
		t.Fatalf("stored.LastRunSummary = %q, want %q", stored.LastRunSummary, job.LastRunSummary)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if list[0].JobKey != job.JobKey {
		t.Fatalf("list[0].JobKey = %q, want %q", list[0].JobKey, job.JobKey)
	}
}

func TestScheduledJobRepository_UpsertPersistsThroughDatabase(t *testing.T) {
	repo := NewScheduledJobRepository(openSchedulerRepoTestDB(t))
	now := time.Date(2026, 3, 25, 8, 0, 0, 0, time.UTC)
	next := now.Add(5 * time.Minute)
	job := &model.ScheduledJob{
		JobKey:         "task-progress-detector",
		Name:           "Task progress detector",
		Scope:          model.ScheduledJobScopeSystem,
		Schedule:       "*/5 * * * *",
		Enabled:        true,
		ExecutionMode:  model.ScheduledJobExecutionModeInProcess,
		OverlapPolicy:  model.ScheduledJobOverlapSkip,
		LastRunStatus:  model.ScheduledJobRunStatusSucceeded,
		LastRunAt:      &now,
		NextRunAt:      &next,
		LastRunSummary: "checked 12 tasks",
		Config:         `{"warningAfter":"2h"}`,
	}

	if err := repo.Upsert(context.Background(), job); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	stored, err := repo.GetByKey(context.Background(), job.JobKey)
	if err != nil {
		t.Fatalf("GetByKey() error = %v", err)
	}
	if stored.LastRunSummary != job.LastRunSummary {
		t.Fatalf("stored.LastRunSummary = %q, want %q", stored.LastRunSummary, job.LastRunSummary)
	}
	if stored.LastError != "" {
		t.Fatalf("stored.LastError = %q, want empty", stored.LastError)
	}

	list, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || list[0].JobKey != job.JobKey {
		t.Fatalf("unexpected persisted jobs: %+v", list)
	}
}

func TestScheduledJobRunRepository_CreateCompleteAndDetectActiveRun(t *testing.T) {
	ctx := context.Background()
	repo := NewScheduledJobRunRepository()
	startedAt := time.Date(2026, 3, 25, 8, 0, 0, 0, time.UTC)

	run := &model.ScheduledJobRun{
		RunID:         "run-1",
		JobKey:        "task-progress-detector",
		TriggerSource: model.ScheduledJobTriggerCron,
		Status:        model.ScheduledJobRunStatusRunning,
		StartedAt:     startedAt,
		Summary:       "started",
	}

	if err := repo.Create(ctx, run); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	active, err := repo.HasActiveRun(ctx, run.JobKey)
	if err != nil {
		t.Fatalf("HasActiveRun() error = %v", err)
	}
	if !active {
		t.Fatal("HasActiveRun() = false, want true")
	}

	finishedAt := startedAt.Add(10 * time.Second)
	if err := repo.Complete(ctx, run.RunID, model.ScheduledJobRunStatusSucceeded, "checked 12 tasks", "", `{"checkedTasks":12}`, finishedAt); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	active, err = repo.HasActiveRun(ctx, run.JobKey)
	if err != nil {
		t.Fatalf("HasActiveRun() after completion error = %v", err)
	}
	if active {
		t.Fatal("HasActiveRun() after completion = true, want false")
	}

	runs, err := repo.ListByJobKey(ctx, run.JobKey, 10)
	if err != nil {
		t.Fatalf("ListByJobKey() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if runs[0].Status != model.ScheduledJobRunStatusSucceeded {
		t.Fatalf("runs[0].Status = %q, want %q", runs[0].Status, model.ScheduledJobRunStatusSucceeded)
	}
	if runs[0].FinishedAt == nil || !runs[0].FinishedAt.Equal(finishedAt) {
		t.Fatalf("runs[0].FinishedAt = %v, want %v", runs[0].FinishedAt, finishedAt)
	}
	if runs[0].Metrics != `{"checkedTasks":12}` {
		t.Fatalf("runs[0].Metrics = %q, want metrics payload", runs[0].Metrics)
	}
}

func TestScheduledJobRunRepository_PersistsLifecycleThroughDatabase(t *testing.T) {
	ctx := context.Background()
	repo := NewScheduledJobRunRepository(openSchedulerRepoTestDB(t))
	startedAt := time.Date(2026, 3, 25, 8, 0, 0, 0, time.UTC)

	run := &model.ScheduledJobRun{
		RunID:         "run-db-1",
		JobKey:        "task-progress-detector",
		TriggerSource: model.ScheduledJobTriggerManual,
		Status:        model.ScheduledJobRunStatusRunning,
		StartedAt:     startedAt,
		Summary:       "started from DB test",
	}
	if err := repo.Create(ctx, run); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	active, err := repo.HasActiveRun(ctx, run.JobKey)
	if err != nil {
		t.Fatalf("HasActiveRun() error = %v", err)
	}
	if !active {
		t.Fatal("HasActiveRun() = false, want true")
	}

	finishedAt := startedAt.Add(30 * time.Second)
	if err := repo.Complete(ctx, run.RunID, model.ScheduledJobRunStatusSucceeded, "checked 8 tasks", "", `{"checkedTasks":8}`, finishedAt); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	active, err = repo.HasActiveRun(ctx, run.JobKey)
	if err != nil {
		t.Fatalf("HasActiveRun() after completion error = %v", err)
	}
	if active {
		t.Fatal("HasActiveRun() after completion = true, want false")
	}

	runs, err := repo.ListByJobKey(ctx, run.JobKey, 10)
	if err != nil {
		t.Fatalf("ListByJobKey() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if runs[0].Status != model.ScheduledJobRunStatusSucceeded {
		t.Fatalf("runs[0].Status = %q, want %q", runs[0].Status, model.ScheduledJobRunStatusSucceeded)
	}
	if runs[0].FinishedAt == nil || !runs[0].FinishedAt.Equal(finishedAt) {
		t.Fatalf("runs[0].FinishedAt = %v, want %v", runs[0].FinishedAt, finishedAt)
	}
}

func openSchedulerRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	schema := []string{
		`CREATE TABLE scheduled_jobs (
			job_key TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			scope TEXT NOT NULL,
			schedule TEXT NOT NULL,
			enabled BOOLEAN NOT NULL,
			execution_mode TEXT NOT NULL,
			overlap_policy TEXT NOT NULL,
			last_run_status TEXT NOT NULL DEFAULT '',
			last_run_at DATETIME,
			next_run_at DATETIME,
			last_run_summary TEXT,
			last_error TEXT,
			config TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE scheduled_job_runs (
			run_id TEXT PRIMARY KEY,
			job_key TEXT NOT NULL,
			trigger_source TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at DATETIME NOT NULL,
			finished_at DATETIME,
			summary TEXT,
			error_message TEXT,
			metrics TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range schema {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create scheduler persistence schema: %v", err)
		}
	}

	return db
}
