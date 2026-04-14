package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
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
	activeRunMu sync.Mutex
	activeRuns  map[string]activeSchedulerRun
}

type activeSchedulerRun struct {
	runID  string
	cancel context.CancelFunc
}

func NewService(jobRepo *repository.ScheduledJobRepository, runRepo *repository.ScheduledJobRunRepository) *Service {
	return &Service{
		jobRepo:    jobRepo,
		runRepo:    runRepo,
		handlers:   make(map[string]Handler),
		now:        func() time.Time { return time.Now().UTC() },
		activeRuns: make(map[string]activeSchedulerRun),
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
	job, err := s.jobRepo.GetByKey(ctx, jobKey)
	if err != nil {
		return nil, err
	}
	return s.projectJob(ctx, job)
}

func (s *Service) ListJobs(ctx context.Context) ([]*model.ScheduledJob, error) {
	if s.jobRepo == nil {
		return nil, fmt.Errorf("scheduled job repository is required")
	}
	jobs, err := s.jobRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	projected := make([]*model.ScheduledJob, 0, len(jobs))
	for _, job := range jobs {
		projectedJob, projectErr := s.projectJob(ctx, job)
		if projectErr != nil {
			return nil, projectErr
		}
		projected = append(projected, projectedJob)
	}
	return projected, nil
}

func (s *Service) ListRuns(ctx context.Context, jobKey string, limit int) ([]*model.ScheduledJobRun, error) {
	if s.runRepo == nil {
		return nil, fmt.Errorf("scheduled job run repository is required")
	}
	return s.runRepo.ListByJobKey(ctx, jobKey, limit)
}

func (s *Service) ListFilteredRuns(ctx context.Context, filters model.ScheduledJobRunFilters) ([]*model.ScheduledJobRun, error) {
	if s.runRepo == nil {
		return nil, fmt.Errorf("scheduled job run repository is required")
	}
	return s.runRepo.ListFiltered(ctx, filters)
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
	stats.QueueDepth = activeRuns

	jobs, err := s.jobRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, job := range jobs {
		if !job.Enabled {
			stats.PausedJobs++
		}
		runs, listErr := s.runRepo.ListByJobKey(ctx, job.JobKey, 200)
		if listErr != nil {
			return nil, listErr
		}
		for _, run := range runs {
			if run == nil {
				continue
			}
			if run.Status == model.ScheduledJobRunStatusSucceeded && run.StartedAt.After(since) {
				stats.SuccessfulRuns24h++
			}
			if run.Status.IsTerminal() && run.StartedAt.After(since) {
				duration := runDurationMs(run)
				if duration > 0 {
					stats.AverageDurationMs += duration
				}
			}
		}
	}
	terminalRuns24h := stats.SuccessfulRuns24h + stats.FailedRuns24h
	if terminalRuns24h > 0 {
		stats.AverageDurationMs = stats.AverageDurationMs / int64(terminalRuns24h)
		stats.SuccessRate24h = float64(stats.SuccessfulRuns24h) * 100 / float64(terminalRuns24h)
	}
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

func (s *Service) DeleteTerminalRuns(ctx context.Context, policy model.ScheduledJobRunCleanupPolicy) (int64, error) {
	if s.runRepo == nil {
		return 0, fmt.Errorf("scheduled job run repository is required")
	}
	return s.runRepo.DeleteTerminalRuns(ctx, policy)
}

func (s *Service) CancelJob(ctx context.Context, jobKey string) (*model.ScheduledJobRun, error) {
	if s.runRepo == nil {
		return nil, fmt.Errorf("scheduled job run repository is required")
	}
	activeRun, ok := s.getActiveRun(jobKey)
	if !ok || activeRun.cancel == nil {
		return nil, fmt.Errorf("scheduled job %s has no active run to cancel", jobKey)
	}

	now := s.now().UTC()
	if err := s.runRepo.UpdateStatus(ctx, activeRun.runID, model.ScheduledJobRunStatusCancelRequested, "cancellation requested", "", now); err != nil {
		return nil, err
	}

	run, err := s.getRunByID(ctx, jobKey, activeRun.runID)
	if err != nil {
		return nil, err
	}
	run.Status = model.ScheduledJobRunStatusCancelRequested
	run.Summary = "cancellation requested"
	run.UpdatedAt = now
	activeRun.cancel()

	job, jobErr := s.jobRepo.GetByKey(ctx, jobKey)
	if jobErr == nil {
		s.broadcast(ws.EventSchedulerRunCancelRequested, job, run)
	}
	return run, nil
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
	return s.projectJob(ctx, job)
}

func (s *Service) PauseJob(ctx context.Context, jobKey string) (*model.ScheduledJob, error) {
	enabled := false
	return s.UpdateJob(ctx, jobKey, UpdateJobInput{Enabled: &enabled})
}

func (s *Service) ResumeJob(ctx context.Context, jobKey string) (*model.ScheduledJob, error) {
	enabled := true
	return s.UpdateJob(ctx, jobKey, UpdateJobInput{Enabled: &enabled})
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

	execCtx, cancel := context.WithCancel(ctx)
	s.setActiveRun(job.JobKey, activeSchedulerRun{runID: run.RunID, cancel: cancel})
	defer s.clearActiveRun(job.JobKey, run.RunID)

	result, execErr := s.runHandler(execCtx, job, run)
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
		if errors.Is(execErr, context.Canceled) {
			status = model.ScheduledJobRunStatusCancelled
			if summary == "" {
				summary = "cancelled"
			}
		} else {
			status = model.ScheduledJobRunStatusFailed
			errorMessage = execErr.Error()
			if summary == "" {
				summary = execErr.Error()
			}
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

func (s *Service) projectJob(ctx context.Context, job *model.ScheduledJob) (*model.ScheduledJob, error) {
	if job == nil {
		return nil, nil
	}

	projected := *job
	projected.ControlState = model.ScheduledJobControlStateActive
	if !projected.Enabled {
		projected.ControlState = model.ScheduledJobControlStatePaused
	}
	projected.ConfigMetadata = buildConfigMetadata(projected.JobKey)
	projected.UpcomingRuns = buildUpcomingRuns(projected.Schedule, s.now().UTC(), 3)

	activeRun, err := s.findActiveRun(ctx, projected.JobKey)
	if err != nil {
		return nil, err
	}
	projected.ActiveRun = activeRun
	projected.SupportedActions = buildSupportedActions(&projected, activeRun)

	return &projected, nil
}

func (s *Service) findActiveRun(ctx context.Context, jobKey string) (*model.ScheduledJobRunSummary, error) {
	if s.runRepo == nil {
		return nil, nil
	}
	runs, err := s.runRepo.ListByJobKey(ctx, jobKey, 20)
	if err != nil {
		return nil, err
	}
	for _, run := range runs {
		if run == nil || run.Status.IsTerminal() {
			continue
		}
		summary := &model.ScheduledJobRunSummary{
			RunID:         run.RunID,
			TriggerSource: run.TriggerSource,
			Status:        run.Status,
			StartedAt:     run.StartedAt,
			FinishedAt:    run.FinishedAt,
			DurationMs:    run.DurationMs,
			Summary:       run.Summary,
			ErrorMessage:  run.ErrorMessage,
		}
		if summary.DurationMs == nil {
			duration := runDurationMs(run)
			if duration > 0 {
				summary.DurationMs = &duration
			}
		}
		return summary, nil
	}
	return nil, nil
}

func buildSupportedActions(job *model.ScheduledJob, activeRun *model.ScheduledJobRunSummary) []model.ScheduledJobActionSupport {
	if job == nil {
		return nil
	}
	supported := []model.ScheduledJobActionSupport{
		{Action: model.ScheduledJobActionTrigger, Enabled: activeRun == nil, Reason: disabledReason(activeRun != nil, "job already running")},
		{Action: model.ScheduledJobActionUpdate, Enabled: true},
		{Action: model.ScheduledJobActionCleanup, Enabled: true},
	}
	if job.Enabled {
		supported = append(supported, model.ScheduledJobActionSupport{Action: model.ScheduledJobActionPause, Enabled: true})
	} else {
		supported = append(supported, model.ScheduledJobActionSupport{Action: model.ScheduledJobActionResume, Enabled: true})
	}
	if activeRun != nil {
		supported = append(supported, model.ScheduledJobActionSupport{Action: model.ScheduledJobActionCancel, Enabled: true})
	} else {
		supported = append(supported, model.ScheduledJobActionSupport{Action: model.ScheduledJobActionCancel, Enabled: false, Reason: "no active run"})
	}
	return supported
}

func buildConfigMetadata(jobKey string) *model.ScheduledJobConfigMetadata {
	return &model.ScheduledJobConfigMetadata{
		SchemaVersion: "v1",
		Editable:      true,
		Fields: []model.ScheduledJobConfigFieldDescriptor{
			{
				Key:      "schedule",
				Label:    "Schedule",
				Type:     model.ScheduledJobConfigFieldTypeString,
				Required: true,
				HelpText: fmt.Sprintf("Operator-facing schedule controls for %s remain built-in and validated by the backend.", jobKey),
			},
		},
	}
}

func buildUpcomingRuns(schedule string, from time.Time, limit int) []model.ScheduledJobOccurrence {
	if limit <= 0 {
		return nil
	}
	occurrences := make([]model.ScheduledJobOccurrence, 0, limit)
	cursor := from
	for len(occurrences) < limit {
		next := NextRunAt(schedule, cursor)
		if next == nil {
			break
		}
		occurrences = append(occurrences, model.ScheduledJobOccurrence{RunAt: *next})
		cursor = next.Add(time.Second)
	}
	return occurrences
}

func runDurationMs(run *model.ScheduledJobRun) int64 {
	if run == nil {
		return 0
	}
	if run.DurationMs != nil && *run.DurationMs > 0 {
		return *run.DurationMs
	}
	if run.FinishedAt == nil {
		return 0
	}
	return run.FinishedAt.Sub(run.StartedAt).Milliseconds()
}

func disabledReason(disabled bool, reason string) string {
	if disabled {
		return reason
	}
	return ""
}

func (s *Service) setActiveRun(jobKey string, run activeSchedulerRun) {
	s.activeRunMu.Lock()
	defer s.activeRunMu.Unlock()
	s.activeRuns[jobKey] = run
}

func (s *Service) getActiveRun(jobKey string) (activeSchedulerRun, bool) {
	s.activeRunMu.Lock()
	defer s.activeRunMu.Unlock()
	run, ok := s.activeRuns[jobKey]
	return run, ok
}

func (s *Service) clearActiveRun(jobKey string, runID string) {
	s.activeRunMu.Lock()
	defer s.activeRunMu.Unlock()
	active, ok := s.activeRuns[jobKey]
	if ok && active.runID == runID {
		delete(s.activeRuns, jobKey)
	}
}

func (s *Service) getRunByID(ctx context.Context, jobKey string, runID string) (*model.ScheduledJobRun, error) {
	runs, err := s.runRepo.ListByJobKey(ctx, jobKey, 50)
	if err != nil {
		return nil, err
	}
	for _, run := range runs {
		if run != nil && run.RunID == runID {
			return run, nil
		}
	}
	return nil, repository.ErrNotFound
}
