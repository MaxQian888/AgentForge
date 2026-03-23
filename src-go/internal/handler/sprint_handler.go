package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

type SprintHandler struct {
	repo *repository.SprintRepository
}

func NewSprintHandler(repo *repository.SprintRepository) *SprintHandler {
	return &SprintHandler{repo: repo}
}

func (h *SprintHandler) Create(c echo.Context) error {
	req := new(model.CreateSprintRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	startDate, err := time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid start date format"})
	}
	endDate, err := time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid end date format"})
	}

	projectID := appMiddleware.GetProjectID(c)
	sprint := &model.Sprint{
		ID:             uuid.New(),
		ProjectID:      projectID,
		Name:           req.Name,
		StartDate:      startDate,
		EndDate:        endDate,
		Status:         model.SprintStatusPlanning,
		TotalBudgetUsd: req.TotalBudgetUsd,
	}
	if err := h.repo.Create(c.Request().Context(), sprint); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create sprint"})
	}
	return c.JSON(http.StatusCreated, sprint.ToDTO())
}

func (h *SprintHandler) List(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	sprints, err := h.repo.ListByProject(c.Request().Context(), projectID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list sprints"})
	}
	dtos := make([]model.SprintDTO, 0, len(sprints))
	for _, s := range sprints {
		dtos = append(dtos, s.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}
