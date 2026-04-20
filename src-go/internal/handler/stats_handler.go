package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/agentforge/server/internal/i18n"
	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type statsService interface {
	Velocity(ctx context.Context, from, to time.Time, projectID *uuid.UUID) (*model.VelocityStatsDTO, error)
	AgentPerformance(ctx context.Context, from, to time.Time, projectID *uuid.UUID) (*model.AgentPerformanceDTO, error)
}

type StatsHandler struct {
	service statsService
}

func NewStatsHandler(svc statsService) *StatsHandler {
	return &StatsHandler{service: svc}
}

func (h *StatsHandler) Velocity(c echo.Context) error {
	from, to, projectID, err := parseStatsParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}

	stats, err := h.service.Velocity(c.Request().Context(), from, to, projectID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToGetVelocityStats)
	}
	return c.JSON(http.StatusOK, stats)
}

func (h *StatsHandler) AgentPerformance(c echo.Context) error {
	from, to, projectID, err := parseStatsParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}

	stats, err := h.service.AgentPerformance(c.Request().Context(), from, to, projectID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToGetAgentPerfStats)
	}
	return c.JSON(http.StatusOK, stats)
}

func parseStatsParams(c echo.Context) (time.Time, time.Time, *uuid.UUID, error) {
	fromStr := c.QueryParam("from")
	toStr := c.QueryParam("to")

	now := time.Now().UTC()
	from := now.AddDate(0, 0, -30)
	to := now

	if fromStr != "" {
		parsed, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, nil, err
		}
		from = parsed
	}
	if toStr != "" {
		parsed, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			return time.Time{}, time.Time{}, nil, err
		}
		to = parsed.Add(24*time.Hour - time.Nanosecond)
	}

	var projectID *uuid.UUID
	if pidStr := c.QueryParam("projectId"); pidStr != "" {
		parsed, err := uuid.Parse(pidStr)
		if err != nil {
			return time.Time{}, time.Time{}, nil, err
		}
		projectID = &parsed
	}

	return from, to, projectID, nil
}
