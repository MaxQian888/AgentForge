package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/internal/scheduler"
	"github.com/labstack/echo/v4"
)

type schedulerServiceMock struct {
	jobs           []*model.ScheduledJob
	runs           []*model.ScheduledJobRun
	filteredRuns   []*model.ScheduledJobRun
	updatedJob     *model.ScheduledJob
	pausedJob      *model.ScheduledJob
	resumedJob     *model.ScheduledJob
	triggered      *model.ScheduledJobRun
	cancelledRun   *model.ScheduledJobRun
	updateInput    scheduler.UpdateJobInput
	updateKey      string
	runKey         string
	runLimit       int
	runFilters     model.ScheduledJobRunFilters
	triggerKey     string
	triggerCronKey string
	pauseKey       string
	resumeKey      string
	cancelKey      string
	cleanupPolicy  model.ScheduledJobRunCleanupPolicy
	cleaned        int64
	err            error
}

func (m *schedulerServiceMock) ListJobs(_ context.Context) ([]*model.ScheduledJob, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.jobs, nil
}

func (m *schedulerServiceMock) GetStats(_ context.Context) (*model.SchedulerStats, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &model.SchedulerStats{}, nil
}

func (m *schedulerServiceMock) GetJob(_ context.Context, jobKey string) (*model.ScheduledJob, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, job := range m.jobs {
		if job.JobKey == jobKey {
			return job, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *schedulerServiceMock) ListRuns(_ context.Context, jobKey string, limit int) ([]*model.ScheduledJobRun, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.runKey = jobKey
	m.runLimit = limit
	return m.runs, nil
}

func (m *schedulerServiceMock) ListFilteredRuns(_ context.Context, filters model.ScheduledJobRunFilters) ([]*model.ScheduledJobRun, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.runFilters = filters
	if m.filteredRuns != nil {
		return m.filteredRuns, nil
	}
	return m.runs, nil
}

func (m *schedulerServiceMock) UpdateJob(_ context.Context, jobKey string, input scheduler.UpdateJobInput) (*model.ScheduledJob, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.updateKey = jobKey
	m.updateInput = input
	return m.updatedJob, nil
}

func (m *schedulerServiceMock) TriggerManual(_ context.Context, jobKey string) (*model.ScheduledJobRun, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.triggerKey = jobKey
	return m.triggered, nil
}

func (m *schedulerServiceMock) TriggerCron(_ context.Context, jobKey string) (*model.ScheduledJobRun, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.triggerCronKey = jobKey
	return m.triggered, nil
}

func (m *schedulerServiceMock) PauseJob(_ context.Context, jobKey string) (*model.ScheduledJob, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.pauseKey = jobKey
	return m.pausedJob, nil
}

func (m *schedulerServiceMock) ResumeJob(_ context.Context, jobKey string) (*model.ScheduledJob, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.resumeKey = jobKey
	return m.resumedJob, nil
}

func (m *schedulerServiceMock) DeleteTerminalRuns(_ context.Context, policy model.ScheduledJobRunCleanupPolicy) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	m.cleanupPolicy = policy
	return m.cleaned, nil
}

func (m *schedulerServiceMock) CancelJob(_ context.Context, jobKey string) (*model.ScheduledJobRun, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.cancelKey = jobKey
	return m.cancelledRun, nil
}

func TestSchedulerHandler_ListJobs(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	svc := &schedulerServiceMock{
		jobs: []*model.ScheduledJob{
			{
				JobKey:        "task-progress-detector",
				Name:          "Task progress detector",
				Scope:         model.ScheduledJobScopeSystem,
				Schedule:      "*/5 * * * *",
				Enabled:       true,
				ExecutionMode: model.ScheduledJobExecutionModeInProcess,
				OverlapPolicy: model.ScheduledJobOverlapSkip,
				LastRunStatus: model.ScheduledJobRunStatusSucceeded,
				LastRunAt:     &now,
				Config:        "{}",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/jobs", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := NewSchedulerHandler(svc).ListJobs(c); err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("rec.Code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"jobKey":"task-progress-detector"`) {
		t.Fatalf("response body = %s, want scheduler job payload", rec.Body.String())
	}
}

func TestSchedulerHandler_ListRunsHonorsLimit(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	svc := &schedulerServiceMock{
		runs: []*model.ScheduledJobRun{
			{
				RunID:         "run-1",
				JobKey:        "task-progress-detector",
				TriggerSource: model.ScheduledJobTriggerManual,
				Status:        model.ScheduledJobRunStatusSucceeded,
				StartedAt:     now,
				FinishedAt:    &now,
				Metrics:       "{}",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/jobs/task-progress-detector/runs?limit=5", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("jobKey")
	c.SetParamValues("task-progress-detector")

	if err := NewSchedulerHandler(svc).ListRuns(c); err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("rec.Code = %d, want 200", rec.Code)
	}
	if svc.runFilters.JobKey != "task-progress-detector" || svc.runFilters.Limit != 5 {
		t.Fatalf("service received filters=%+v, want task-progress-detector and limit 5", svc.runFilters)
	}
}

func TestSchedulerHandler_UpdateJob(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	enabled := false
	svc := &schedulerServiceMock{
		updatedJob: &model.ScheduledJob{
			JobKey:        "task-progress-detector",
			Name:          "Task progress detector",
			Scope:         model.ScheduledJobScopeSystem,
			Schedule:      "0 * * * *",
			Enabled:       enabled,
			ExecutionMode: model.ScheduledJobExecutionModeInProcess,
			OverlapPolicy: model.ScheduledJobOverlapSkip,
			Config:        "{}",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/scheduler/jobs/task-progress-detector", strings.NewReader(`{"enabled":false,"schedule":"0 * * * *"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("jobKey")
	c.SetParamValues("task-progress-detector")

	if err := NewSchedulerHandler(svc).UpdateJob(c); err != nil {
		t.Fatalf("UpdateJob() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("rec.Code = %d, want 200", rec.Code)
	}
	if svc.updateKey != "task-progress-detector" {
		t.Fatalf("svc.updateKey = %q, want task-progress-detector", svc.updateKey)
	}
	if svc.updateInput.Enabled == nil || *svc.updateInput.Enabled {
		t.Fatalf("svc.updateInput.Enabled = %v, want false", svc.updateInput.Enabled)
	}
	if svc.updateInput.Schedule == nil || *svc.updateInput.Schedule != "0 * * * *" {
		t.Fatalf("svc.updateInput.Schedule = %v, want updated cron", svc.updateInput.Schedule)
	}
}

func TestSchedulerHandler_UpdateJobReturnsValidationError(t *testing.T) {
	svc := &schedulerServiceMock{err: errors.New("validate schedule: expected exactly 5 fields")}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/scheduler/jobs/task-progress-detector", strings.NewReader(`{"schedule":"bad"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("jobKey")
	c.SetParamValues("task-progress-detector")

	if err := NewSchedulerHandler(svc).UpdateJob(c); err != nil {
		t.Fatalf("UpdateJob() error = %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("rec.Code = %d, want 422", rec.Code)
	}
}

func TestSchedulerHandler_TriggerManual(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	svc := &schedulerServiceMock{
		triggered: &model.ScheduledJobRun{
			RunID:         "run-1",
			JobKey:        "task-progress-detector",
			TriggerSource: model.ScheduledJobTriggerManual,
			Status:        model.ScheduledJobRunStatusSucceeded,
			StartedAt:     now,
			FinishedAt:    &now,
			Metrics:       "{}",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/jobs/task-progress-detector/trigger", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("jobKey")
	c.SetParamValues("task-progress-detector")

	if err := NewSchedulerHandler(svc).TriggerManual(c); err != nil {
		t.Fatalf("TriggerManual() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("rec.Code = %d, want 200", rec.Code)
	}
	if svc.triggerKey != "task-progress-detector" {
		t.Fatalf("svc.triggerKey = %q, want task-progress-detector", svc.triggerKey)
	}
}

func TestSchedulerHandler_TriggerCron(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	svc := &schedulerServiceMock{
		triggered: &model.ScheduledJobRun{
			RunID:         "run-1",
			JobKey:        "task-progress-detector",
			TriggerSource: model.ScheduledJobTriggerCron,
			Status:        model.ScheduledJobRunStatusSucceeded,
			StartedAt:     now,
			FinishedAt:    &now,
			Metrics:       "{}",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/internal/scheduler/jobs/task-progress-detector/trigger", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/internal/scheduler/jobs/:jobKey/trigger")
	c.SetParamNames("jobKey")
	c.SetParamValues("task-progress-detector")

	if err := NewSchedulerHandler(svc).TriggerCron(c); err != nil {
		t.Fatalf("TriggerCron() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("rec.Code = %d, want 200", rec.Code)
	}
	if svc.triggerCronKey != "task-progress-detector" {
		t.Fatalf("svc.triggerCronKey = %q, want task-progress-detector", svc.triggerCronKey)
	}
}

func TestSchedulerHandler_ReturnsNotFoundForMissingJob(t *testing.T) {
	svc := &schedulerServiceMock{err: repository.ErrNotFound}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/jobs/missing/trigger", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("jobKey")
	c.SetParamValues("missing")

	if err := NewSchedulerHandler(svc).TriggerManual(c); err != nil {
		t.Fatalf("TriggerManual() error = %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("rec.Code = %d, want 404", rec.Code)
	}
}

func TestSchedulerHandler_PauseJob(t *testing.T) {
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	svc := &schedulerServiceMock{
		pausedJob: &model.ScheduledJob{
			JobKey:       "task-progress-detector",
			Enabled:      false,
			ControlState: model.ScheduledJobControlStatePaused,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/jobs/task-progress-detector/pause", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("jobKey")
	c.SetParamValues("task-progress-detector")

	callHandlerMethod(t, NewSchedulerHandler(svc), "PauseJob", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("rec.Code = %d, want 200", rec.Code)
	}
	if svc.pauseKey != "task-progress-detector" {
		t.Fatalf("svc.pauseKey = %q, want task-progress-detector", svc.pauseKey)
	}
}

func TestSchedulerHandler_ResumeJob(t *testing.T) {
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	svc := &schedulerServiceMock{
		resumedJob: &model.ScheduledJob{
			JobKey:       "task-progress-detector",
			Enabled:      true,
			ControlState: model.ScheduledJobControlStateActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/jobs/task-progress-detector/resume", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("jobKey")
	c.SetParamValues("task-progress-detector")

	callHandlerMethod(t, NewSchedulerHandler(svc), "ResumeJob", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("rec.Code = %d, want 200", rec.Code)
	}
	if svc.resumeKey != "task-progress-detector" {
		t.Fatalf("svc.resumeKey = %q, want task-progress-detector", svc.resumeKey)
	}
}

func TestSchedulerHandler_ListRunsSupportsFilters(t *testing.T) {
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	svc := &schedulerServiceMock{
		filteredRuns: []*model.ScheduledJobRun{
			{
				RunID:         "run-1",
				JobKey:        "task-progress-detector",
				TriggerSource: model.ScheduledJobTriggerCron,
				Status:        model.ScheduledJobRunStatusFailed,
				StartedAt:     now,
				FinishedAt:    &now,
				Metrics:       "{}",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/jobs/task-progress-detector/runs?status=failed&triggerSource=cron&limit=7&since=2026-04-13T08:00:00Z&before=2026-04-13T11:00:00Z", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("jobKey")
	c.SetParamValues("task-progress-detector")

	if err := NewSchedulerHandler(svc).ListRuns(c); err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("rec.Code = %d, want 200", rec.Code)
	}
	if svc.runFilters.JobKey != "task-progress-detector" {
		t.Fatalf("svc.runFilters.JobKey = %q, want task-progress-detector", svc.runFilters.JobKey)
	}
	if len(svc.runFilters.Statuses) != 1 || svc.runFilters.Statuses[0] != model.ScheduledJobRunStatusFailed {
		t.Fatalf("svc.runFilters.Statuses = %v, want [failed]", svc.runFilters.Statuses)
	}
	if len(svc.runFilters.TriggerSources) != 1 || svc.runFilters.TriggerSources[0] != model.ScheduledJobTriggerCron {
		t.Fatalf("svc.runFilters.TriggerSources = %v, want [cron]", svc.runFilters.TriggerSources)
	}
	if svc.runFilters.Limit != 7 {
		t.Fatalf("svc.runFilters.Limit = %d, want 7", svc.runFilters.Limit)
	}
	if svc.runFilters.StartedAfter == nil || svc.runFilters.StartedBefore == nil {
		t.Fatalf("svc.runFilters = %+v, want parsed since/before filters", svc.runFilters)
	}
}

func TestSchedulerHandler_CleanupRuns(t *testing.T) {
	svc := &schedulerServiceMock{cleaned: 2}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/jobs/task-progress-detector/runs/cleanup", strings.NewReader(`{"retainRecent":1}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("jobKey")
	c.SetParamValues("task-progress-detector")

	callHandlerMethod(t, NewSchedulerHandler(svc), "CleanupRuns", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("rec.Code = %d, want 200", rec.Code)
	}
	if svc.cleanupPolicy.JobKey != "task-progress-detector" {
		t.Fatalf("svc.cleanupPolicy.JobKey = %q, want task-progress-detector", svc.cleanupPolicy.JobKey)
	}
	if svc.cleanupPolicy.RetainRecent != 1 {
		t.Fatalf("svc.cleanupPolicy.RetainRecent = %d, want 1", svc.cleanupPolicy.RetainRecent)
	}
}

func TestSchedulerHandler_CancelJob(t *testing.T) {
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	svc := &schedulerServiceMock{
		cancelledRun: &model.ScheduledJobRun{
			RunID:         "run-1",
			JobKey:        "task-progress-detector",
			TriggerSource: model.ScheduledJobTriggerManual,
			Status:        model.ScheduledJobRunStatusCancelRequested,
			StartedAt:     now,
			Metrics:       "{}",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/jobs/task-progress-detector/cancel", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("jobKey")
	c.SetParamValues("task-progress-detector")

	callHandlerMethod(t, NewSchedulerHandler(svc), "CancelJob", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("rec.Code = %d, want 200", rec.Code)
	}
	if svc.cancelKey != "task-progress-detector" {
		t.Fatalf("svc.cancelKey = %q, want task-progress-detector", svc.cancelKey)
	}
}

func TestSchedulerHandler_GetPreview(t *testing.T) {
	now := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)
	svc := &schedulerServiceMock{
		jobs: []*model.ScheduledJob{
			{
				JobKey: "task-progress-detector",
				UpcomingRuns: []model.ScheduledJobOccurrence{
					{RunAt: now.Add(5 * time.Minute)},
					{RunAt: now.Add(10 * time.Minute)},
				},
			},
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/jobs/task-progress-detector/preview", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("jobKey")
	c.SetParamValues("task-progress-detector")

	callHandlerMethod(t, NewSchedulerHandler(svc), "GetPreview", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("rec.Code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "runAt") {
		t.Fatalf("response body = %s, want preview payload", rec.Body.String())
	}
}

func TestSchedulerHandler_GetConfigMetadata(t *testing.T) {
	svc := &schedulerServiceMock{
		jobs: []*model.ScheduledJob{
			{
				JobKey: "task-progress-detector",
				ConfigMetadata: &model.ScheduledJobConfigMetadata{
					SchemaVersion: "v1",
					Editable:      true,
				},
			},
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/jobs/task-progress-detector/config-metadata", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("jobKey")
	c.SetParamValues("task-progress-detector")

	callHandlerMethod(t, NewSchedulerHandler(svc), "GetConfigMetadata", c)
	if rec.Code != http.StatusOK {
		t.Fatalf("rec.Code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "schemaVersion") {
		t.Fatalf("response body = %s, want config metadata payload", rec.Body.String())
	}
}

func callHandlerMethod(t *testing.T, handler *SchedulerHandler, methodName string, c echo.Context) {
	t.Helper()
	method := reflect.ValueOf(handler).MethodByName(methodName)
	if !method.IsValid() {
		t.Fatalf("SchedulerHandler is missing method %s", methodName)
	}
	results := method.Call([]reflect.Value{reflect.ValueOf(c)})
	if len(results) != 1 {
		t.Fatalf("%s returned %d results, want 1", methodName, len(results))
	}
	if errValue := results[0].Interface(); errValue != nil {
		t.Fatalf("%s() error = %v", methodName, errValue)
	}
}
