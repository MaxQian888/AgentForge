package repository

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
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

func (r *ScheduledJobRunRepository) UpdateStatus(ctx context.Context, runID string, status model.ScheduledJobRunStatus, summary string, errorMessage string, updatedAt time.Time) error {
	if runID == "" {
		return fmt.Errorf("scheduled job run id is required")
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
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
			run.UpdatedAt = updatedAt
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
			"updated_at":    updatedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update scheduled job run status: %w", result.Error)
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

func (r *ScheduledJobRunRepository) ListFiltered(ctx context.Context, filters model.ScheduledJobRunFilters) ([]*model.ScheduledJobRun, error) {
	if filters.Limit <= 0 {
		filters.Limit = 20
	}

	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		runs := make([]*model.ScheduledJobRun, 0, filters.Limit)
		for i := len(r.runs) - 1; i >= 0; i-- {
			run := r.runs[i]
			if !matchesScheduledJobRunFilters(run, filters) {
				continue
			}
			runs = append(runs, cloneScheduledJobRun(run))
			if len(runs) == filters.Limit {
				break
			}
		}
		return runs, nil
	}

	query := r.db.WithContext(ctx).Model(&scheduledJobRunRecord{}).Order("started_at DESC")
	if filters.JobKey != "" {
		query = query.Where("job_key = ?", filters.JobKey)
	}
	if len(filters.Statuses) > 0 {
		query = query.Where("status IN ?", filters.Statuses)
	}
	if len(filters.TriggerSources) > 0 {
		query = query.Where("trigger_source IN ?", filters.TriggerSources)
	}
	if filters.StartedAfter != nil {
		query = query.Where("started_at >= ?", *filters.StartedAfter)
	}
	if filters.StartedBefore != nil {
		query = query.Where("started_at < ?", *filters.StartedBefore)
	}

	var rows []scheduledJobRunRecord
	if err := query.Limit(filters.Limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list filtered scheduled job runs: %w", err)
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

func (r *ScheduledJobRunRepository) DeleteTerminalRuns(ctx context.Context, policy model.ScheduledJobRunCleanupPolicy) (int64, error) {
	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()

		protected := buildRetainedRunIDSet(r.runs, policy)
		kept := make([]*model.ScheduledJobRun, 0, len(r.runs))
		deleted := int64(0)
		for _, run := range r.runs {
			_, keepProtected := protected[run.RunID]
			if !matchesCleanupPolicy(run, policy) || keepProtected {
				kept = append(kept, run)
				continue
			}
			deleted++
		}
		r.runs = kept
		return deleted, nil
	}

	var rows []scheduledJobRunRecord
	query := r.db.WithContext(ctx).
		Model(&scheduledJobRunRecord{}).
		Where("status IN ?", []model.ScheduledJobRunStatus{
			model.ScheduledJobRunStatusSucceeded,
			model.ScheduledJobRunStatusFailed,
			model.ScheduledJobRunStatusSkipped,
			model.ScheduledJobRunStatusCancelled,
		}).
		Order("started_at DESC")
	if policy.JobKey != "" {
		query = query.Where("job_key = ?", policy.JobKey)
	}
	if policy.StartedBefore != nil {
		query = query.Where("started_at < ?", *policy.StartedBefore)
	}
	if err := query.Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("list terminal scheduled job runs for cleanup: %w", err)
	}

	if len(rows) == 0 {
		return 0, nil
	}

	retained := make(map[string]struct{}, policy.RetainRecent)
	if policy.RetainRecent > 0 {
		fullHistoryFilters := model.ScheduledJobRunFilters{
			JobKey: policy.JobKey,
			Limit:  1000,
			Statuses: []model.ScheduledJobRunStatus{
				model.ScheduledJobRunStatusSucceeded,
				model.ScheduledJobRunStatusFailed,
				model.ScheduledJobRunStatusSkipped,
				model.ScheduledJobRunStatusCancelled,
			},
		}
		fullHistory, err := r.ListFiltered(ctx, fullHistoryFilters)
		if err != nil {
			return 0, err
		}
		for _, run := range fullHistory {
			if policy.RetainRecent == 0 {
				break
			}
			retained[run.RunID] = struct{}{}
			policy.RetainRecent--
		}
	}

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if _, ok := retained[row.RunID]; ok {
			continue
		}
		ids = append(ids, row.RunID)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	result := r.db.WithContext(ctx).Where("run_id IN ?", ids).Delete(&scheduledJobRunRecord{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete terminal scheduled job runs: %w", result.Error)
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

func matchesScheduledJobRunFilters(run *model.ScheduledJobRun, filters model.ScheduledJobRunFilters) bool {
	if run == nil {
		return false
	}
	if filters.JobKey != "" && run.JobKey != filters.JobKey {
		return false
	}
	if len(filters.Statuses) > 0 && !slices.Contains(filters.Statuses, run.Status) {
		return false
	}
	if len(filters.TriggerSources) > 0 && !slices.Contains(filters.TriggerSources, run.TriggerSource) {
		return false
	}
	if filters.StartedAfter != nil && run.StartedAt.Before(*filters.StartedAfter) {
		return false
	}
	if filters.StartedBefore != nil && !run.StartedAt.Before(*filters.StartedBefore) {
		return false
	}
	return true
}

func matchesCleanupPolicy(run *model.ScheduledJobRun, policy model.ScheduledJobRunCleanupPolicy) bool {
	if run == nil || !run.Status.IsTerminal() {
		return false
	}
	if policy.JobKey != "" && run.JobKey != policy.JobKey {
		return false
	}
	if policy.StartedBefore != nil && !run.StartedAt.Before(*policy.StartedBefore) {
		return false
	}
	return true
}

func buildRetainedRunIDSet(runs []*model.ScheduledJobRun, policy model.ScheduledJobRunCleanupPolicy) map[string]struct{} {
	retained := make(map[string]struct{}, policy.RetainRecent)
	if policy.RetainRecent <= 0 {
		return retained
	}
	candidates := make([]*model.ScheduledJobRun, 0, len(runs))
	for _, run := range runs {
		if run == nil || !run.Status.IsTerminal() {
			continue
		}
		if policy.JobKey != "" && run.JobKey != policy.JobKey {
			continue
		}
		candidates = append(candidates, run)
	}
	slices.SortFunc(candidates, func(a, b *model.ScheduledJobRun) int {
		if a.StartedAt.After(b.StartedAt) {
			return -1
		}
		if a.StartedAt.Before(b.StartedAt) {
			return 1
		}
		return 0
	})
	for _, run := range candidates {
		if len(retained) >= policy.RetainRecent {
			break
		}
		retained[run.RunID] = struct{}{}
	}
	return retained
}

func optionalSchedulerString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
