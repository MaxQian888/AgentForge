package handler

import (
	"net/http"

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
	runs, err := h.repo.ListActive(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to get cost stats"})
	}

	var totalCost float64
	var totalInput, totalOutput, totalCacheRead int64
	var totalTurns int
	for _, r := range runs {
		totalCost += r.CostUsd
		totalInput += r.InputTokens
		totalOutput += r.OutputTokens
		totalCacheRead += r.CacheReadTokens
		totalTurns += r.TurnCount
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"totalCostUsd":    totalCost,
		"totalInputTokens":  totalInput,
		"totalOutputTokens": totalOutput,
		"totalCacheReadTokens": totalCacheRead,
		"totalTurns":       totalTurns,
		"activeAgents":     len(runs),
	})
}
