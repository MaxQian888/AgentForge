package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	eventbus "github.com/agentforge/server/internal/eventbus"
	"github.com/agentforge/server/internal/i18n"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/ws"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type SprintHandler struct {
	repo     sprintRepository
	taskRepo sprintTaskRepository
	hub      *ws.Hub
	bus      eventbus.Publisher
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

func parseOptionalMilestoneID(value *string) (*uuid.UUID, error) {
	if value == nil {
		return nil, nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}

	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func (h *SprintHandler) Create(c echo.Context) error {
	req := new(model.CreateSprintRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	startDate, err := time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidStartDateFormat)
	}
	endDate, err := time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEndDateFormat)
	}
	milestoneID, err := parseOptionalMilestoneID(req.MilestoneID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidMilestoneID)
	}

	projectID := appMiddleware.GetProjectID(c)
	sprint := &model.Sprint{
		ID:             uuid.New(),
		ProjectID:      projectID,
		Name:           req.Name,
		StartDate:      startDate,
		EndDate:        endDate,
		MilestoneID:    milestoneID,
		Status:         model.SprintStatusPlanning,
		TotalBudgetUsd: req.TotalBudgetUsd,
	}
	if err := h.repo.Create(c.Request().Context(), sprint); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateSprint)
	}
	return c.JSON(http.StatusCreated, sprint.ToDTO())
}

func (h *SprintHandler) List(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	sprints, err := h.repo.ListByProject(c.Request().Context(), projectID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListSprints)
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidSprintID)
	}
	if h.taskRepo == nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgSprintMetricsUnavailable)
	}

	projectID := appMiddleware.GetProjectID(c)
	sprint, err := h.repo.GetByID(c.Request().Context(), sprintID)
	if err != nil || sprint.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgSprintNotFound)
	}

	tasks, err := h.taskRepo.ListBySprint(c.Request().Context(), projectID, sprintID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToLoadSprintMetrics)
	}

	return c.JSON(http.StatusOK, model.BuildSprintMetricsDTO(sprint, tasks, h.now()))
}

func (h *SprintHandler) Burndown(c echo.Context) error {
	sprintID, err := uuid.Parse(c.Param("sid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidSprintID)
	}

	sprint, err := h.repo.GetByID(c.Request().Context(), sprintID)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgSprintNotFound)
	}

	if h.taskRepo == nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgSprintBurndownUnavailable)
	}

	tasks, err := h.taskRepo.ListBySprint(c.Request().Context(), sprint.ProjectID, sprintID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToLoadBurndownData)
	}

	metrics := model.BuildSprintMetricsDTO(sprint, tasks, h.now())
	return c.JSON(http.StatusOK, metrics)
}

func (h *SprintHandler) WithHub(hub *ws.Hub) *SprintHandler {
	h.hub = hub
	return h
}

func (h *SprintHandler) WithBus(bus eventbus.Publisher) *SprintHandler {
	h.bus = bus
	return h
}

func (h *SprintHandler) Update(c echo.Context) error {
	sprintID, err := uuid.Parse(c.Param("sid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidSprintID)
	}

	projectID := appMiddleware.GetProjectID(c)
	sprint, err := h.repo.GetByID(c.Request().Context(), sprintID)
	if err != nil || sprint.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgSprintNotFound)
	}

	req := new(model.UpdateSprintRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
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
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidStartDateFormat)
		}
		sprint.StartDate = parsed
	}
	if req.EndDate != nil {
		parsed, err := time.Parse(time.RFC3339, *req.EndDate)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEndDateFormat)
		}
		sprint.EndDate = parsed
	}
	if req.MilestoneID != nil {
		parsed, err := parseOptionalMilestoneID(req.MilestoneID)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidMilestoneID)
		}
		sprint.MilestoneID = parsed
	}
	if req.TotalBudgetUsd != nil {
		sprint.TotalBudgetUsd = *req.TotalBudgetUsd
	}

	if err := h.repo.Update(c.Request().Context(), sprint); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateSprint)
	}

	dto := sprint.ToDTO()
	eventType := ws.EventSprintUpdated
	if transitioned {
		eventType = ws.EventSprintTransitioned
	}
	_ = eventbus.PublishLegacy(c.Request().Context(), h.bus, eventType, projectID.String(), map[string]interface{}{
		"sprint":    dto,
		"oldStatus": oldStatus,
	})

	return c.JSON(http.StatusOK, dto)
}
