package repository

import (
	"context"
	"fmt"
	"reflect"
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

func TestScheduledJobRunRepository_ListFilteredByStatusAndTriggerSource(t *testing.T) {
	ctx := context.Background()
	repo := NewScheduledJobRunRepository()
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)

	seedRunForFilterTest(t, ctx, repo, &model.ScheduledJobRun{
		RunID:         "success-manual",
		JobKey:        "task-progress-detector",
		TriggerSource: model.ScheduledJobTriggerManual,
		Status:        model.ScheduledJobRunStatusSucceeded,
		StartedAt:     now.Add(-2 * time.Hour),
	}, model.ScheduledJobRunStatusSucceeded, now.Add(-2*time.Hour+10*time.Second))
	seedRunForFilterTest(t, ctx, repo, &model.ScheduledJobRun{
		RunID:         "failed-cron",
		JobKey:        "task-progress-detector",
		TriggerSource: model.ScheduledJobTriggerCron,
		Status:        model.ScheduledJobRunStatusFailed,
		StartedAt:     now.Add(-90 * time.Minute),
	}, model.ScheduledJobRunStatusFailed, now.Add(-90*time.Minute+5*time.Second))
	seedRunForFilterTest(t, ctx, repo, &model.ScheduledJobRun{
		RunID:         "other-job",
		JobKey:        "cost-reconcile",
		TriggerSource: model.ScheduledJobTriggerCron,
		Status:        model.ScheduledJobRunStatusSucceeded,
		StartedAt:     now.Add(-30 * time.Minute),
	}, model.ScheduledJobRunStatusSucceeded, now.Add(-30*time.Minute+2*time.Second))

	filtered := callListFilteredRuns(t, repo, ctx, model.ScheduledJobRunFilters{
		JobKey:         "task-progress-detector",
		Statuses:       []model.ScheduledJobRunStatus{model.ScheduledJobRunStatusFailed},
		TriggerSources: []model.ScheduledJobTriggerSource{model.ScheduledJobTriggerCron},
		Limit:          10,
	})
	if len(filtered) != 1 {
		t.Fatalf("len(filtered) = %d, want 1", len(filtered))
	}
	if filtered[0].RunID != "failed-cron" {
		t.Fatalf("filtered[0].RunID = %q, want failed-cron", filtered[0].RunID)
	}
}

func TestScheduledJobRunRepository_DeleteTerminalRunsKeepsRecentAndActive(t *testing.T) {
	ctx := context.Background()
	repo := NewScheduledJobRunRepository()
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	cutoff := now.Add(-1 * time.Hour)

	seedRunForFilterTest(t, ctx, repo, &model.ScheduledJobRun{
		RunID:         "old-terminal",
		JobKey:        "task-progress-detector",
		TriggerSource: model.ScheduledJobTriggerCron,
		Status:        model.ScheduledJobRunStatusSucceeded,
		StartedAt:     now.Add(-4 * time.Hour),
	}, model.ScheduledJobRunStatusSucceeded, now.Add(-4*time.Hour+10*time.Second))
	seedRunForFilterTest(t, ctx, repo, &model.ScheduledJobRun{
		RunID:         "recent-terminal",
		JobKey:        "task-progress-detector",
		TriggerSource: model.ScheduledJobTriggerManual,
		Status:        model.ScheduledJobRunStatusSucceeded,
		StartedAt:     now.Add(-30 * time.Minute),
	}, model.ScheduledJobRunStatusSucceeded, now.Add(-30*time.Minute+4*time.Second))
	if err := repo.Create(ctx, &model.ScheduledJobRun{
		RunID:         "active-run",
		JobKey:        "task-progress-detector",
		TriggerSource: model.ScheduledJobTriggerManual,
		Status:        model.ScheduledJobRunStatusRunning,
		StartedAt:     now.Add(-5 * time.Minute),
		Metrics:       "{}",
	}); err != nil {
		t.Fatalf("repo.Create(active-run) error = %v", err)
	}

	deleted := callDeleteTerminalRuns(t, repo, ctx, model.ScheduledJobRunCleanupPolicy{
		JobKey:        "task-progress-detector",
		StartedBefore: &cutoff,
		RetainRecent:  1,
	})
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	remaining := callListFilteredRuns(t, repo, ctx, model.ScheduledJobRunFilters{
		JobKey: "task-progress-detector",
		Limit:  10,
	})
	if len(remaining) != 2 {
		t.Fatalf("len(remaining) = %d, want 2", len(remaining))
	}
	if remaining[0].RunID != "active-run" && remaining[1].RunID != "active-run" {
		t.Fatalf("remaining runs = %+v, want active-run to be preserved", remaining)
	}
	if remaining[0].RunID != "recent-terminal" && remaining[1].RunID != "recent-terminal" {
		t.Fatalf("remaining runs = %+v, want recent-terminal to be preserved", remaining)
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

func seedRunForFilterTest(t *testing.T, ctx context.Context, repo *ScheduledJobRunRepository, run *model.ScheduledJobRun, status model.ScheduledJobRunStatus, finishedAt time.Time) {
	t.Helper()
	if err := repo.Create(ctx, run); err != nil {
		t.Fatalf("repo.Create(%s) error = %v", run.RunID, err)
	}
	if err := repo.Complete(ctx, run.RunID, status, run.RunID, "", `{}`, finishedAt); err != nil {
		t.Fatalf("repo.Complete(%s) error = %v", run.RunID, err)
	}
}

func callListFilteredRuns(t *testing.T, repo *ScheduledJobRunRepository, ctx context.Context, filters model.ScheduledJobRunFilters) []*model.ScheduledJobRun {
	t.Helper()
	method := reflect.ValueOf(repo).MethodByName("ListFiltered")
	if !method.IsValid() {
		t.Fatal("ScheduledJobRunRepository is missing ListFiltered")
	}
	results := method.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(filters)})
	if len(results) != 2 {
		t.Fatalf("ListFiltered returned %d results, want 2", len(results))
	}
	if !results[1].IsNil() {
		t.Fatalf("ListFiltered() error = %v", results[1].Interface())
	}
	runs, ok := results[0].Interface().([]*model.ScheduledJobRun)
	if !ok {
		t.Fatalf("ListFiltered() first result = %T, want []*model.ScheduledJobRun", results[0].Interface())
	}
	return runs
}

func callDeleteTerminalRuns(t *testing.T, repo *ScheduledJobRunRepository, ctx context.Context, policy model.ScheduledJobRunCleanupPolicy) int64 {
	t.Helper()
	method := reflect.ValueOf(repo).MethodByName("DeleteTerminalRuns")
	if !method.IsValid() {
		t.Fatal("ScheduledJobRunRepository is missing DeleteTerminalRuns")
	}
	results := method.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(policy)})
	if len(results) != 2 {
		t.Fatalf("DeleteTerminalRuns returned %d results, want 2", len(results))
	}
	if !results[1].IsNil() {
		t.Fatalf("DeleteTerminalRuns() error = %v", results[1].Interface())
	}
	return results[0].Interface().(int64)
}
