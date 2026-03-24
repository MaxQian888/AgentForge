package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

type CostHandler struct {
	repo *repository.AgentRunRepository
}

func NewCostHandler(repo *repository.AgentRunRepository) *CostHandler {
	return &CostHandler{repo: repo}
}

func (h *CostHandler) GetStats(c echo.Context) error {
	// Support filtering by projectId and sprintId query params
	projectIDStr := c.QueryParam("projectId")
	sprintIDStr := c.QueryParam("sprintId")

	if projectIDStr != "" {
		projectID, err := uuid.Parse(projectIDStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid projectId"})
		}
		summary, err := h.repo.AggregateByProject(c.Request().Context(), projectID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to get cost stats"})
		}
		return c.JSON(http.StatusOK, summary)
	}

	if sprintIDStr != "" {
		sprintID, err := uuid.Parse(sprintIDStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid sprintId"})
		}
		runs, err := h.repo.ListBySprint(c.Request().Context(), sprintID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to get cost stats"})
		}
		return c.JSON(http.StatusOK, aggregateRuns(runs))
	}

	// Default: aggregate across all active runs
	runs, err := h.repo.ListActive(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to get cost stats"})
	}

	return c.JSON(http.StatusOK, aggregateRuns(runs))
}

func aggregateRuns(runs []*model.AgentRun) model.CostSummaryDTO {
	s := model.CostSummaryDTO{RunCount: len(runs)}
	for _, r := range runs {
		s.TotalCostUsd += r.CostUsd
		s.TotalInputTokens += r.InputTokens
		s.TotalOutputTokens += r.OutputTokens
		s.TotalCacheReadTokens += r.CacheReadTokens
		s.TotalTurns += r.TurnCount
	}
	return s
}
