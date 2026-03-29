package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/scheduler"
)

type schedulerServiceMock struct {
	jobs           []*model.ScheduledJob
	runs           []*model.ScheduledJobRun
	updatedJob     *model.ScheduledJob
	triggered      *model.ScheduledJobRun
	updateInput    scheduler.UpdateJobInput
	updateKey      string
	runKey         string
	runLimit       int
	triggerKey     string
	triggerCronKey string
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
	if svc.runKey != "task-progress-detector" || svc.runLimit != 5 {
		t.Fatalf("service received jobKey=%q limit=%d, want task-progress-detector and 5", svc.runKey, svc.runLimit)
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
