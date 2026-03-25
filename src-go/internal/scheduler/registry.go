package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/robfig/cron/v3"
)

type CatalogEntry struct {
	JobKey        string
	Name          string
	Scope         model.ScheduledJobScope
	Schedule      string
	Enabled       bool
	ExecutionMode model.ScheduledJobExecutionMode
	OverlapPolicy model.ScheduledJobOverlapPolicy
	Config        string
}

type Registry struct {
	repo    *repository.ScheduledJobRepository
	catalog []CatalogEntry
	now     func() time.Time
}

func NewRegistry(repo *repository.ScheduledJobRepository, catalog []CatalogEntry) *Registry {
	return &Registry{
		repo:    repo,
		catalog: append([]CatalogEntry(nil), catalog...),
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func (r *Registry) Reconcile(ctx context.Context) ([]*model.ScheduledJob, error) {
	if r.repo == nil {
		return nil, fmt.Errorf("scheduled job repository is required")
	}

	now := r.now().UTC()
	jobs := make([]*model.ScheduledJob, 0, len(r.catalog))
	for _, entry := range r.catalog {
		job, err := r.reconcileEntry(ctx, entry, now)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (r *Registry) reconcileEntry(ctx context.Context, entry CatalogEntry, now time.Time) (*model.ScheduledJob, error) {
	if err := ValidateSchedule(entry.Schedule); err != nil {
		return nil, fmt.Errorf("validate schedule for %s: %w", entry.JobKey, err)
	}

	existing, err := r.repo.GetByKey(ctx, entry.JobKey)
	if err != nil && err != repository.ErrNotFound {
		return nil, fmt.Errorf("load existing job %s: %w", entry.JobKey, err)
	}

	job := &model.ScheduledJob{
		JobKey:         entry.JobKey,
		Name:           entry.Name,
		Scope:          entry.Scope,
		Schedule:       entry.Schedule,
		Enabled:        entry.Enabled,
		ExecutionMode:  entry.ExecutionMode,
		OverlapPolicy:  entry.OverlapPolicy,
		Config:         defaultJSON(entry.Config),
	}

	if existing != nil {
		job.Enabled = existing.Enabled
		job.Schedule = existing.Schedule
		job.LastRunStatus = existing.LastRunStatus
		job.LastRunAt = existing.LastRunAt
		job.LastRunSummary = existing.LastRunSummary
		job.LastError = existing.LastError
		job.Config = defaultJSON(existing.Config)
		job.CreatedAt = existing.CreatedAt
	}

	if err := ValidateSchedule(job.Schedule); err != nil {
		return nil, fmt.Errorf("validate effective schedule for %s: %w", entry.JobKey, err)
	}

	nextRunAt := NextRunAt(job.Schedule, now)
	job.NextRunAt = nextRunAt

	if err := r.repo.Upsert(ctx, job); err != nil {
		return nil, fmt.Errorf("upsert scheduled job %s: %w", entry.JobKey, err)
	}
	return job, nil
}

func ValidateSchedule(schedule string) error {
	if schedule == "" {
		return fmt.Errorf("schedule is required")
	}
	_, err := cron.ParseStandard(schedule)
	if err != nil {
		return err
	}
	return nil
}

func NextRunAt(schedule string, from time.Time) *time.Time {
	spec, err := cron.ParseStandard(schedule)
	if err != nil {
		return nil
	}
	next := spec.Next(from.UTC())
	if next.IsZero() {
		return nil
	}
	return &next
}

func defaultJSON(value string) string {
	if value == "" {
		return "{}"
	}
	return value
}
