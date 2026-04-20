package handler

import (
	"context"
	"errors"
	"net/http"

	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type budgetQueryService interface {
	GetProjectBudgetSummary(ctx context.Context, projectID uuid.UUID) (*model.ProjectBudgetSummary, error)
	GetSprintBudgetDetail(ctx context.Context, sprintID uuid.UUID) (*model.SprintBudgetDetail, error)
}

type BudgetQueryHandler struct {
	service budgetQueryService
}

func NewBudgetQueryHandler(service budgetQueryService) *BudgetQueryHandler {
	return &BudgetQueryHandler{service: service}
}

func (h *BudgetQueryHandler) ProjectSummary(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	summary, err := h.service.GetProjectBudgetSummary(c.Request().Context(), projectID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, summary)
}

func (h *BudgetQueryHandler) SprintDetail(c echo.Context) error {
	sprintID, err := uuid.Parse(c.Param("sid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid sprint id"})
	}

	detail, err := h.service.GetSprintBudgetDetail(c.Request().Context(), sprintID)
	if err != nil {
		if errors.Is(err, service.ErrBudgetSprintNotFound) {
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, detail)
}
