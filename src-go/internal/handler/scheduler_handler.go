package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/scheduler"
)

type SchedulerService interface {
	GetJob(ctx context.Context, jobKey string) (*model.ScheduledJob, error)
	ListJobs(ctx context.Context) ([]*model.ScheduledJob, error)
	ListRuns(ctx context.Context, jobKey string, limit int) ([]*model.ScheduledJobRun, error)
	GetStats(ctx context.Context) (*model.SchedulerStats, error)
	UpdateJob(ctx context.Context, jobKey string, input scheduler.UpdateJobInput) (*model.ScheduledJob, error)
	TriggerManual(ctx context.Context, jobKey string) (*model.ScheduledJobRun, error)
	TriggerCron(ctx context.Context, jobKey string) (*model.ScheduledJobRun, error)
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
	limit := 20
	if raw := strings.TrimSpace(c.QueryParam("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidLimit)
		}
		limit = parsed
	}

	runs, err := h.service.ListRuns(c.Request().Context(), c.Param("jobKey"), limit)
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
