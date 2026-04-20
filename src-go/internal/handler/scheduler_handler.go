package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentforge/server/internal/i18n"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/internal/scheduler"
	"github.com/labstack/echo/v4"
)

type SchedulerService interface {
	GetJob(ctx context.Context, jobKey string) (*model.ScheduledJob, error)
	ListJobs(ctx context.Context) ([]*model.ScheduledJob, error)
	ListRuns(ctx context.Context, jobKey string, limit int) ([]*model.ScheduledJobRun, error)
	ListFilteredRuns(ctx context.Context, filters model.ScheduledJobRunFilters) ([]*model.ScheduledJobRun, error)
	GetStats(ctx context.Context) (*model.SchedulerStats, error)
	UpdateJob(ctx context.Context, jobKey string, input scheduler.UpdateJobInput) (*model.ScheduledJob, error)
	TriggerManual(ctx context.Context, jobKey string) (*model.ScheduledJobRun, error)
	TriggerCron(ctx context.Context, jobKey string) (*model.ScheduledJobRun, error)
	PauseJob(ctx context.Context, jobKey string) (*model.ScheduledJob, error)
	ResumeJob(ctx context.Context, jobKey string) (*model.ScheduledJob, error)
	CancelJob(ctx context.Context, jobKey string) (*model.ScheduledJobRun, error)
	DeleteTerminalRuns(ctx context.Context, policy model.ScheduledJobRunCleanupPolicy) (int64, error)
}

type SchedulerHandler struct {
	service SchedulerService
}

func NewSchedulerHandler(service SchedulerService) *SchedulerHandler {
	return &SchedulerHandler{service: service}
}

func (h *SchedulerHandler) GetJob(c echo.Context) error {
	job, err := h.service.GetJob(c.Request().Context(), c.Param("jobKey"))
	if err != nil {
		return h.writeSchedulerError(c, err, "failed to get scheduled job")
	}
	return c.JSON(http.StatusOK, job)
}

func (h *SchedulerHandler) GetStats(c echo.Context) error {
	stats, err := h.service.GetStats(c.Request().Context())
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToGetSchedulerStats)
	}
	return c.JSON(http.StatusOK, stats)
}

func (h *SchedulerHandler) ListJobs(c echo.Context) error {
	jobs, err := h.service.ListJobs(c.Request().Context())
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListScheduledJobs)
	}
	return c.JSON(http.StatusOK, jobs)
}

func (h *SchedulerHandler) ListRuns(c echo.Context) error {
	filters, err := schedulerRunFiltersFromRequest(c)
	if err != nil {
		return err
	}

	runs, err := h.service.ListFilteredRuns(c.Request().Context(), filters)
	if err != nil {
		return h.writeSchedulerError(c, err, "failed to list scheduled job runs")
	}
	return c.JSON(http.StatusOK, runs)
}

func (h *SchedulerHandler) UpdateJob(c echo.Context) error {
	req := new(model.UpdateScheduledJobRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}

	job, err := h.service.UpdateJob(c.Request().Context(), c.Param("jobKey"), scheduler.UpdateJobInput{
		Enabled:  req.Enabled,
		Schedule: req.Schedule,
	})
	if err != nil {
		return h.writeSchedulerError(c, err, "failed to update scheduled job")
	}
	return c.JSON(http.StatusOK, job)
}

func (h *SchedulerHandler) TriggerManual(c echo.Context) error {
	run, err := h.service.TriggerManual(c.Request().Context(), c.Param("jobKey"))
	if err != nil {
		return h.writeSchedulerError(c, err, "failed to trigger scheduled job")
	}
	return c.JSON(http.StatusOK, run)
}

func (h *SchedulerHandler) TriggerCron(c echo.Context) error {
	run, err := h.service.TriggerCron(c.Request().Context(), c.Param("jobKey"))
	if err != nil {
		return h.writeSchedulerError(c, err, "failed to trigger scheduled job")
	}
	return c.JSON(http.StatusOK, run)
}

func (h *SchedulerHandler) PauseJob(c echo.Context) error {
	job, err := h.service.PauseJob(c.Request().Context(), c.Param("jobKey"))
	if err != nil {
		return h.writeSchedulerError(c, err, "failed to pause scheduled job")
	}
	return c.JSON(http.StatusOK, job)
}

func (h *SchedulerHandler) ResumeJob(c echo.Context) error {
	job, err := h.service.ResumeJob(c.Request().Context(), c.Param("jobKey"))
	if err != nil {
		return h.writeSchedulerError(c, err, "failed to resume scheduled job")
	}
	return c.JSON(http.StatusOK, job)
}

func (h *SchedulerHandler) CleanupRuns(c echo.Context) error {
	req := new(model.ScheduledJobRunCleanupPolicy)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	req.JobKey = c.Param("jobKey")
	deleted, err := h.service.DeleteTerminalRuns(c.Request().Context(), *req)
	if err != nil {
		return h.writeSchedulerError(c, err, "failed to cleanup scheduled job runs")
	}
	return c.JSON(http.StatusOK, map[string]int64{"deleted": deleted})
}

func (h *SchedulerHandler) CancelJob(c echo.Context) error {
	run, err := h.service.CancelJob(c.Request().Context(), c.Param("jobKey"))
	if err != nil {
		return h.writeSchedulerError(c, err, "failed to cancel scheduled job")
	}
	return c.JSON(http.StatusOK, run)
}

func (h *SchedulerHandler) GetPreview(c echo.Context) error {
	job, err := h.service.GetJob(c.Request().Context(), c.Param("jobKey"))
	if err != nil {
		return h.writeSchedulerError(c, err, "failed to get scheduled job preview")
	}
	return c.JSON(http.StatusOK, map[string]any{"upcomingRuns": job.UpcomingRuns})
}

func (h *SchedulerHandler) GetConfigMetadata(c echo.Context) error {
	job, err := h.service.GetJob(c.Request().Context(), c.Param("jobKey"))
	if err != nil {
		return h.writeSchedulerError(c, err, "failed to get scheduled job config metadata")
	}
	return c.JSON(http.StatusOK, job.ConfigMetadata)
}

func (h *SchedulerHandler) writeSchedulerError(c echo.Context, err error, fallback string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return localizedError(c, http.StatusNotFound, i18n.MsgScheduledJobNotFound)
	case strings.Contains(err.Error(), "validate schedule"):
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	default:
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: fallback})
	}
}

func schedulerRunFiltersFromRequest(c echo.Context) (model.ScheduledJobRunFilters, error) {
	filters := model.ScheduledJobRunFilters{
		JobKey: c.Param("jobKey"),
		Limit:  20,
	}
	if raw := strings.TrimSpace(c.QueryParam("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return filters, localizedError(c, http.StatusBadRequest, i18n.MsgInvalidLimit)
		}
		filters.Limit = parsed
	}
	if raw := strings.TrimSpace(c.QueryParam("status")); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			filters.Statuses = append(filters.Statuses, model.ScheduledJobRunStatus(trimmed))
		}
	}
	if raw := strings.TrimSpace(c.QueryParam("triggerSource")); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			filters.TriggerSources = append(filters.TriggerSources, model.ScheduledJobTriggerSource(trimmed))
		}
	}
	if raw := strings.TrimSpace(c.QueryParam("since")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filters, c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid since"})
		}
		filters.StartedAfter = &parsed
	}
	if raw := strings.TrimSpace(c.QueryParam("before")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filters, c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid before"})
		}
		filters.StartedBefore = &parsed
	}
	return filters, nil
}
