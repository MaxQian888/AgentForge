package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type Handler func(context.Context, *model.ScheduledJob, *model.ScheduledJobRun) (*RunResult, error)

type RunResult struct {
	Summary string
	Metrics string
}

type UpdateJobInput struct {
	Enabled  *bool
	Schedule *string
}

type EventBroadcaster interface {
	BroadcastSchedulerEvent(eventType string, job *model.ScheduledJob, run *model.ScheduledJobRun)
}

type Service struct {
	jobRepo     *repository.ScheduledJobRepository
	runRepo     *repository.ScheduledJobRunRepository
	handlers    map[string]Handler
	broadcaster EventBroadcaster
	now         func() time.Time
}

func NewService(jobRepo *repository.ScheduledJobRepository, runRepo *repository.ScheduledJobRunRepository) *Service {
	return &Service{
		jobRepo:  jobRepo,
		runRepo:  runRepo,
		handlers: make(map[string]Handler),
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) RegisterHandler(jobKey string, handler Handler) {
	if jobKey == "" || handler == nil {
		return
	}
	s.handlers[jobKey] = handler
}

func (s *Service) SetBroadcaster(broadcaster EventBroadcaster) {
	s.broadcaster = broadcaster
}

func (s *Service) RunDue(ctx context.Context) ([]*model.ScheduledJobRun, error) {
	if s.jobRepo == nil || s.runRepo == nil {
		return nil, fmt.Errorf("scheduler repositories are required")
	}

	jobs, err := s.jobRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list scheduled jobs: %w", err)
	}

	now := s.now().UTC()
	runs := make([]*model.ScheduledJobRun, 0)
	for _, job := range jobs {
		if !job.Enabled {
			continue
		}
		if job.ExecutionMode != model.ScheduledJobExecutionModeInProcess {
			continue
		}
		if !s.isDue(job, now) {
			continue
		}

		run, started, err := s.executeJob(ctx, job, model.ScheduledJobTriggerCron, now)
		if err != nil {
			return nil, err
		}
		if started {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func (s *Service) TriggerManual(ctx context.Context, jobKey string) (*model.ScheduledJobRun, error) {
	return s.trigger(ctx, jobKey, model.ScheduledJobTriggerManual)
}

func (s *Service) TriggerCron(ctx context.Context, jobKey string) (*model.ScheduledJobRun, error) {
	return s.trigger(ctx, jobKey, model.ScheduledJobTriggerCron)
}

func (s *Service) trigger(ctx context.Context, jobKey string, trigger model.ScheduledJobTriggerSource) (*model.ScheduledJobRun, error) {
	if s.jobRepo == nil || s.runRepo == nil {
		return nil, fmt.Errorf("scheduler repositories are required")
	}

	job, err := s.jobRepo.GetByKey(ctx, jobKey)
	if err != nil {
		return nil, err
	}
	run, _, err := s.executeJob(ctx, job, trigger, s.now().UTC())
	return run, err
}

func (s *Service) GetJob(ctx context.Context, jobKey string) (*model.ScheduledJob, error) {
	if s.jobRepo == nil {
		return nil, fmt.Errorf("scheduled job repository is required")
	}
	return s.jobRepo.GetByKey(ctx, jobKey)
}

func (s *Service) ListJobs(ctx context.Context) ([]*model.ScheduledJob, error) {
	if s.jobRepo == nil {
		return nil, fmt.Errorf("scheduled job repository is required")
	}
	return s.jobRepo.List(ctx)
}

func (s *Service) ListRuns(ctx context.Context, jobKey string, limit int) ([]*model.ScheduledJobRun, error) {
	if s.runRepo == nil {
		return nil, fmt.Errorf("scheduled job run repository is required")
	}
	return s.runRepo.ListByJobKey(ctx, jobKey, limit)
}

func (s *Service) GetStats(ctx context.Context) (*model.SchedulerStats, error) {
	if s.jobRepo == nil || s.runRepo == nil {
		return nil, fmt.Errorf("scheduler repositories are required")
	}
	stats, err := s.jobRepo.Stats(ctx)
	if err != nil {
		return nil, err
	}
	since := s.now().UTC().Add(-24 * time.Hour)
	totalRuns, failedRuns, activeRuns, err := s.runRepo.CountByStatus(ctx, since)
	if err != nil {
		return nil, err
	}
	stats.ActiveRuns = activeRuns
	stats.TotalRuns24h = totalRuns
	stats.FailedRuns24h = failedRuns
	return stats, nil
}

func (s *Service) DeleteOldRuns(ctx context.Context, retentionDays int) (int64, error) {
	if s.runRepo == nil {
		return 0, fmt.Errorf("scheduled job run repository is required")
	}
	if retentionDays <= 0 {
		retentionDays = 30
	}
	before := s.now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	return s.runRepo.DeleteOlderThan(ctx, before)
}

func (s *Service) UpdateJob(ctx context.Context, jobKey string, input UpdateJobInput) (*model.ScheduledJob, error) {
	if s.jobRepo == nil {
		return nil, fmt.Errorf("scheduled job repository is required")
	}

	job, err := s.jobRepo.GetByKey(ctx, jobKey)
	if err != nil {
		return nil, err
	}

	if input.Schedule != nil {
		schedule := *input.Schedule
		if err := ValidateSchedule(schedule); err != nil {
			return nil, fmt.Errorf("validate schedule: %w", err)
		}
		job.Schedule = schedule
	}
	if input.Enabled != nil {
		job.Enabled = *input.Enabled
	}
	if job.Enabled {
		job.NextRunAt = NextRunAt(job.Schedule, s.now().UTC())
	} else {
		job.NextRunAt = nil
	}

	if err := s.jobRepo.Upsert(ctx, job); err != nil {
		return nil, fmt.Errorf("update scheduled job %s: %w", jobKey, err)
	}
	s.broadcast(ws.EventSchedulerJobUpdated, job, nil)
	return job, nil
}

func (s *Service) isDue(job *model.ScheduledJob, now time.Time) bool {
	if job == nil || job.NextRunAt == nil {
		return false
	}
	return !job.NextRunAt.After(now)
}

func (s *Service) executeJob(ctx context.Context, job *model.ScheduledJob, trigger model.ScheduledJobTriggerSource, now time.Time) (*model.ScheduledJobRun, bool, error) {
	if job == nil {
		return nil, false, fmt.Errorf("scheduled job is required")
	}

	if job.OverlapPolicy == model.ScheduledJobOverlapSkip {
		active, err := s.runRepo.HasActiveRun(ctx, job.JobKey)
		if err != nil {
			return nil, false, fmt.Errorf("check active scheduled job run for %s: %w", job.JobKey, err)
		}
		if active {
			return nil, false, nil
		}
	}

	run := &model.ScheduledJobRun{
		JobKey:        job.JobKey,
		TriggerSource: trigger,
		Status:        model.ScheduledJobRunStatusRunning,
		StartedAt:     now,
		Metrics:       "{}",
	}
	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, false, fmt.Errorf("create scheduled job run for %s: %w", job.JobKey, err)
	}
	s.broadcast(ws.EventSchedulerRunStarted, job, run)

	result, execErr := s.runHandler(ctx, job, run)
	if err := s.finalizeRun(ctx, job, run, result, execErr, now); err != nil {
		return nil, false, err
	}
	return run, true, nil
}

func (s *Service) runHandler(ctx context.Context, job *model.ScheduledJob, run *model.ScheduledJobRun) (*RunResult, error) {
	handler, ok := s.handlers[job.JobKey]
	if !ok {
		return nil, fmt.Errorf("scheduled job handler not registered for %s", job.JobKey)
	}
	return handler(ctx, job, run)
}

func (s *Service) finalizeRun(ctx context.Context, job *model.ScheduledJob, run *model.ScheduledJobRun, result *RunResult, execErr error, finishedAt time.Time) error {
	status := model.ScheduledJobRunStatusSucceeded
	summary := ""
	errorMessage := ""
	metrics := "{}"
	if result != nil {
		summary = result.Summary
		if result.Metrics != "" {
			metrics = result.Metrics
		}
	}
	if execErr != nil {
		status = model.ScheduledJobRunStatusFailed
		errorMessage = execErr.Error()
		if summary == "" {
			summary = execErr.Error()
		}
	}

	if err := s.runRepo.Complete(ctx, run.RunID, status, summary, errorMessage, metrics, finishedAt); err != nil {
		return fmt.Errorf("complete scheduled job run %s: %w", run.RunID, err)
	}

	run.Status = status
	run.Summary = summary
	run.ErrorMessage = errorMessage
	run.Metrics = metrics
	run.FinishedAt = &finishedAt
	run.UpdatedAt = finishedAt

	job.LastRunStatus = status
	job.LastRunAt = &finishedAt
	job.LastRunSummary = summary
	job.LastError = errorMessage
	job.NextRunAt = NextRunAt(job.Schedule, finishedAt)
	if err := s.jobRepo.Upsert(ctx, job); err != nil {
		return fmt.Errorf("update scheduled job %s after run completion: %w", job.JobKey, err)
	}
	s.broadcast(ws.EventSchedulerRunCompleted, job, run)
	return nil
}

func (s *Service) broadcast(eventType string, job *model.ScheduledJob, run *model.ScheduledJobRun) {
	if s == nil || s.broadcaster == nil {
		return
	}
	s.broadcaster.BroadcastSchedulerEvent(eventType, job, run)
}
