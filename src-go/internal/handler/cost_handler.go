package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/model"
)

type costStatsService interface {
	ProjectSummary(ctx context.Context, projectID uuid.UUID) (*model.ProjectCostSummaryDTO, error)
	SprintSummary(ctx context.Context, sprintID uuid.UUID) (*model.CostSummaryDTO, error)
	ActiveSummary(ctx context.Context) (*model.CostSummaryDTO, error)
}

type CostHandler struct {
	service costStatsService
}

func NewCostHandler(service costStatsService) *CostHandler {
	return &CostHandler{service: service}
}

func (h *CostHandler) GetStats(c echo.Context) error {
	// Support filtering by projectId and sprintId query params
	projectIDStr := c.QueryParam("projectId")
	sprintIDStr := c.QueryParam("sprintId")

	if projectIDStr != "" {
		projectID, err := uuid.Parse(projectIDStr)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectIDParam)
		}
		summary, err := h.service.ProjectSummary(c.Request().Context(), projectID)
		if err != nil {
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToGetCostStats)
		}
		return c.JSON(http.StatusOK, summary)
	}

	if sprintIDStr != "" {
		sprintID, err := uuid.Parse(sprintIDStr)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidSprintIDParam)
		}
		summary, err := h.service.SprintSummary(c.Request().Context(), sprintID)
		if err != nil {
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToGetCostStats)
		}
		return c.JSON(http.StatusOK, summary)
	}

	// Default: aggregate across all active runs
	summary, err := h.service.ActiveSummary(c.Request().Context())
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToGetCostStats)
	}

	return c.JSON(http.StatusOK, summary)
}
