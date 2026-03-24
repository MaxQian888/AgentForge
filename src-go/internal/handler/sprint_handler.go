package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type SprintHandler struct {
	repo     sprintRepository
	taskRepo sprintTaskRepository
	hub      *ws.Hub
	now      func() time.Time
}

type sprintRepository interface {
	Create(ctx context.Context, sprint *model.Sprint) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Sprint, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Sprint, error)
	Update(ctx context.Context, sprint *model.Sprint) error
}

type sprintTaskRepository interface {
	ListBySprint(ctx context.Context, projectID uuid.UUID, sprintID uuid.UUID) ([]*model.Task, error)
}

func NewSprintHandler(repo sprintRepository, taskRepo ...sprintTaskRepository) *SprintHandler {
	handler := &SprintHandler{
		repo: repo,
		now:  time.Now,
	}
	if len(taskRepo) > 0 {
		handler.taskRepo = taskRepo[0]
	}
	return handler
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

func (h *SprintHandler) Metrics(c echo.Context) error {
	sprintID, err := uuid.Parse(c.Param("sid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid sprint ID"})
	}
	if h.taskRepo == nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "sprint metrics unavailable"})
	}

	projectID := appMiddleware.GetProjectID(c)
	sprint, err := h.repo.GetByID(c.Request().Context(), sprintID)
	if err != nil || sprint.ProjectID != projectID {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "sprint not found"})
	}

	tasks, err := h.taskRepo.ListBySprint(c.Request().Context(), projectID, sprintID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to load sprint metrics"})
	}

	return c.JSON(http.StatusOK, model.BuildSprintMetricsDTO(sprint, tasks, h.now()))
}

func (h *SprintHandler) Burndown(c echo.Context) error {
	sprintID, err := uuid.Parse(c.Param("sid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid sprint ID"})
	}

	sprint, err := h.repo.GetByID(c.Request().Context(), sprintID)
	if err != nil {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "sprint not found"})
	}

	if h.taskRepo == nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "sprint burndown unavailable"})
	}

	tasks, err := h.taskRepo.ListBySprint(c.Request().Context(), sprint.ProjectID, sprintID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to load burndown data"})
	}

	metrics := model.BuildSprintMetricsDTO(sprint, tasks, h.now())
	return c.JSON(http.StatusOK, metrics)
}

func (h *SprintHandler) WithHub(hub *ws.Hub) *SprintHandler {
	h.hub = hub
	return h
}

func (h *SprintHandler) Update(c echo.Context) error {
	sprintID, err := uuid.Parse(c.Param("sid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid sprint ID"})
	}

	projectID := appMiddleware.GetProjectID(c)
	sprint, err := h.repo.GetByID(c.Request().Context(), sprintID)
	if err != nil || sprint.ProjectID != projectID {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "sprint not found"})
	}

	req := new(model.UpdateSprintRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}

	oldStatus := sprint.Status
	transitioned := false

	if req.Status != nil && *req.Status != oldStatus {
		if err := model.ValidateSprintTransition(oldStatus, *req.Status); err != nil {
			return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
		}
		sprint.Status = *req.Status
		transitioned = true
	}
	if req.Name != nil {
		sprint.Name = *req.Name
	}
	if req.StartDate != nil {
		parsed, err := time.Parse(time.RFC3339, *req.StartDate)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid start date format"})
		}
		sprint.StartDate = parsed
	}
	if req.EndDate != nil {
		parsed, err := time.Parse(time.RFC3339, *req.EndDate)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid end date format"})
		}
		sprint.EndDate = parsed
	}
	if req.TotalBudgetUsd != nil {
		sprint.TotalBudgetUsd = *req.TotalBudgetUsd
	}

	if err := h.repo.Update(c.Request().Context(), sprint); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to update sprint"})
	}

	dto := sprint.ToDTO()
	if h.hub != nil {
		eventType := ws.EventSprintUpdated
		if transitioned {
			eventType = ws.EventSprintTransitioned
		}
		h.hub.BroadcastEvent(&ws.Event{
			Type:      eventType,
			ProjectID: projectID.String(),
			Payload: map[string]interface{}{
				"sprint":    dto,
				"oldStatus": oldStatus,
			},
		})
	}

	return c.JSON(http.StatusOK, dto)
}
