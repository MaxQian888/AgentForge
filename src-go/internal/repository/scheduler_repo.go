package repository

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ScheduledJobRepository struct {
	db   *gorm.DB
	mu   sync.RWMutex
	jobs map[string]*model.ScheduledJob
}

func NewScheduledJobRepository(db ...*gorm.DB) *ScheduledJobRepository {
	var conn *gorm.DB
	if len(db) > 0 {
		conn = db[0]
	}
	return &ScheduledJobRepository{
		db:   conn,
		jobs: make(map[string]*model.ScheduledJob),
	}
}

func (r *ScheduledJobRepository) Upsert(ctx context.Context, job *model.ScheduledJob) error {
	if job == nil {
		return fmt.Errorf("scheduled job is required")
	}
	if job.JobKey == "" {
		return fmt.Errorf("scheduled job key is required")
	}
	if job.Scope == "" {
		job.Scope = model.ScheduledJobScopeSystem
	}
	if job.ExecutionMode == "" {
		job.ExecutionMode = model.ScheduledJobExecutionModeInProcess
	}
	if job.OverlapPolicy == "" {
		job.OverlapPolicy = model.ScheduledJobOverlapSkip
	}
	if job.Config == "" {
		job.Config = "{}"
	}

	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		if job.CreatedAt.IsZero() {
			job.CreatedAt = time.Now().UTC()
		}
		job.UpdatedAt = time.Now().UTC()
		r.jobs[job.JobKey] = cloneScheduledJob(job)
		return nil
	}

	row := newScheduledJobRecord(job)
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "job_key"}},
			UpdateAll: true,
		}).
		Create(row).Error; err != nil {
		return fmt.Errorf("upsert scheduled job: %w", err)
	}
	return nil
}

func (r *ScheduledJobRepository) GetByKey(ctx context.Context, jobKey string) (*model.ScheduledJob, error) {
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		job, ok := r.jobs[jobKey]
		if !ok {
			return nil, ErrNotFound
		}
		return cloneScheduledJob(job), nil
	}

	var row scheduledJobRecord
	if err := r.db.WithContext(ctx).Where("job_key = ?", jobKey).Take(&row).Error; err != nil {
		return nil, normalizeRepositoryError(err)
	}
	return row.toModel(), nil
}

func (r *ScheduledJobRepository) Stats(ctx context.Context) (*model.SchedulerStats, error) {
	jobs, err := r.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("stats: %w", err)
	}
	stats := &model.SchedulerStats{TotalJobs: len(jobs)}
	for _, job := range jobs {
		if job.Enabled {
			stats.EnabledJobs++
		} else {
			stats.DisabledJobs++
		}
		if job.LastRunStatus == model.ScheduledJobRunStatusFailed {
			stats.FailedJobs++
		}
	}
	return stats, nil
}

func (r *ScheduledJobRepository) List(ctx context.Context) ([]*model.ScheduledJob, error) {
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		jobs := make([]*model.ScheduledJob, 0, len(r.jobs))
		for _, job := range r.jobs {
			jobs = append(jobs, cloneScheduledJob(job))
		}
		slices.SortFunc(jobs, func(a, b *model.ScheduledJob) int {
			if a.JobKey < b.JobKey {
				return -1
			}
			if a.JobKey > b.JobKey {
				return 1
			}
			return 0
		})
		return jobs, nil
	}

	var rows []scheduledJobRecord
	if err := r.db.WithContext(ctx).Order("job_key ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list scheduled jobs: %w", err)
	}

	jobs := make([]*model.ScheduledJob, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, row.toModel())
	}
	return jobs, nil
}

type ScheduledJobRunRepository struct {
	db   *gorm.DB
	mu   sync.RWMutex
	runs []*model.ScheduledJobRun
}

func NewScheduledJobRunRepository(db ...*gorm.DB) *ScheduledJobRunRepository {
	var conn *gorm.DB
	if len(db) > 0 {
		conn = db[0]
	}
	return &ScheduledJobRunRepository{
		db:   conn,
		runs: make([]*model.ScheduledJobRun, 0),
	}
}

func (r *ScheduledJobRunRepository) Create(ctx context.Context, run *model.ScheduledJobRun) error {
	if run == nil {
		return fmt.Errorf("scheduled job run is required")
	}
	if run.RunID == "" {
		run.RunID = uuid.NewString()
	}
	if run.JobKey == "" {
		return fmt.Errorf("scheduled job run job_key is required")
	}
	if run.Status == "" {
		run.Status = model.ScheduledJobRunStatusRunning
	}
	if run.Metrics == "" {
		run.Metrics = "{}"
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = run.StartedAt
	}
	run.UpdatedAt = run.StartedAt

	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.runs = append(r.runs, cloneScheduledJobRun(run))
		return nil
	}

	if err := r.db.WithContext(ctx).Create(newScheduledJobRunRecord(run)).Error; err != nil {
		return fmt.Errorf("create scheduled job run: %w", err)
	}
	return nil
}

func (r *ScheduledJobRunRepository) Complete(ctx context.Context, runID string, status model.ScheduledJobRunStatus, summary string, errorMessage string, metrics string, finishedAt time.Time) error {
	if runID == "" {
		return fmt.Errorf("scheduled job run id is required")
	}
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	}
	if metrics == "" {
		metrics = "{}"
	}

	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, run := range r.runs {
			if run.RunID != runID {
				continue
			}
			run.Status = status
			run.Summary = summary
			run.ErrorMessage = errorMessage
			run.Metrics = metrics
			run.FinishedAt = cloneTimePointer(&finishedAt)
			run.UpdatedAt = finishedAt
			return nil
		}
		return ErrNotFound
	}

	result := r.db.WithContext(ctx).
		Model(&scheduledJobRunRecord{}).
		Where("run_id = ?", runID).
		Updates(map[string]any{
			"status":        status,
			"summary":       optionalSchedulerString(summary),
			"error_message": optionalSchedulerString(errorMessage),
			"metrics":       newJSONText(metrics, "{}"),
			"finished_at":   finishedAt,
			"updated_at":    finishedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("complete scheduled job run: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ScheduledJobRunRepository) ListByJobKey(ctx context.Context, jobKey string, limit int) ([]*model.ScheduledJobRun, error) {
	if limit <= 0 {
		limit = 20
	}

	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		runs := make([]*model.ScheduledJobRun, 0, limit)
		for i := len(r.runs) - 1; i >= 0; i-- {
			run := r.runs[i]
			if run.JobKey != jobKey {
				continue
			}
			runs = append(runs, cloneScheduledJobRun(run))
			if len(runs) == limit {
				break
			}
		}
		return runs, nil
	}

	var rows []scheduledJobRunRecord
	if err := r.db.WithContext(ctx).
		Where("job_key = ?", jobKey).
		Order("started_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list scheduled job runs: %w", err)
	}

	runs := make([]*model.ScheduledJobRun, 0, len(rows))
	for _, row := range rows {
		runs = append(runs, row.toModel())
	}
	return runs, nil
}

func (r *ScheduledJobRunRepository) CountByStatus(ctx context.Context, since time.Time) (total int, failed int, active int, err error) {
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		for _, run := range r.runs {
			if !run.Status.IsTerminal() {
				active++
			}
			if run.StartedAt.Before(since) {
				continue
			}
			total++
			if run.Status == model.ScheduledJobRunStatusFailed {
				failed++
			}
		}
		return total, failed, active, nil
	}

	type result struct {
		Status string
		Count  int
	}

	var results []result
	if dbErr := r.db.WithContext(ctx).
		Model(&scheduledJobRunRecord{}).
		Select("status, COUNT(*) as count").
		Where("started_at >= ?", since).
		Group("status").
		Scan(&results).Error; dbErr != nil {
		return 0, 0, 0, fmt.Errorf("count runs by status: %w", dbErr)
	}
	for _, row := range results {
		total += row.Count
		if model.ScheduledJobRunStatus(row.Status) == model.ScheduledJobRunStatusFailed {
			failed += row.Count
		}
	}

	var activeCount int64
	if dbErr := r.db.WithContext(ctx).
		Model(&scheduledJobRunRecord{}).
		Where("status IN ?", []model.ScheduledJobRunStatus{
			model.ScheduledJobRunStatusPending,
			model.ScheduledJobRunStatusRunning,
		}).
		Count(&activeCount).Error; dbErr != nil {
		return 0, 0, 0, fmt.Errorf("count active runs: %w", dbErr)
	}
	active = int(activeCount)

	return total, failed, active, nil
}

func (r *ScheduledJobRunRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		kept := make([]*model.ScheduledJobRun, 0, len(r.runs))
		deleted := int64(0)
		for _, run := range r.runs {
			if run.StartedAt.Before(before) && run.Status.IsTerminal() {
				deleted++
				continue
			}
			kept = append(kept, run)
		}
		r.runs = kept
		return deleted, nil
	}

	result := r.db.WithContext(ctx).
		Where("started_at < ? AND status IN ?", before, []model.ScheduledJobRunStatus{
			model.ScheduledJobRunStatusSucceeded,
			model.ScheduledJobRunStatusFailed,
			model.ScheduledJobRunStatusSkipped,
		}).
		Delete(&scheduledJobRunRecord{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete old scheduled job runs: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *ScheduledJobRunRepository) HasActiveRun(ctx context.Context, jobKey string) (bool, error) {
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		for _, run := range r.runs {
			if run.JobKey == jobKey && !run.Status.IsTerminal() {
				return true, nil
			}
		}
		return false, nil
	}

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&scheduledJobRunRecord{}).
		Where("job_key = ? AND status IN ?", jobKey, []model.ScheduledJobRunStatus{
			model.ScheduledJobRunStatusPending,
			model.ScheduledJobRunStatusRunning,
		}).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check active scheduled job run: %w", err)
	}
	return count > 0, nil
}

func cloneScheduledJob(job *model.ScheduledJob) *model.ScheduledJob {
	if job == nil {
		return nil
	}
	cloned := *job
	cloned.LastRunAt = cloneTimePointer(job.LastRunAt)
	cloned.NextRunAt = cloneTimePointer(job.NextRunAt)
	return &cloned
}

func cloneScheduledJobRun(run *model.ScheduledJobRun) *model.ScheduledJobRun {
	if run == nil {
		return nil
	}
	cloned := *run
	cloned.FinishedAt = cloneTimePointer(run.FinishedAt)
	cloned.ComputeDuration()
	return &cloned
}

func optionalSchedulerString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
