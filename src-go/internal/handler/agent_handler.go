package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

type AgentHandler struct {
	repo *repository.AgentRunRepository
}

func NewAgentHandler(repo *repository.AgentRunRepository) *AgentHandler {
	return &AgentHandler{repo: repo}
}

type SpawnAgentRequest struct {
	TaskID   string `json:"taskId" validate:"required"`
	MemberID string `json:"memberId" validate:"required"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

func (h *AgentHandler) Spawn(c echo.Context) error {
	req := new(SpawnAgentRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	taskID, err := uuid.Parse(req.TaskID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid task ID"})
	}
	memberID, err := uuid.Parse(req.MemberID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid member ID"})
	}

	run := &model.AgentRun{
		ID:        uuid.New(),
		TaskID:    taskID,
		MemberID:  memberID,
		Status:    model.AgentRunStatusStarting,
		Provider:  req.Provider,
		Model:     req.Model,
		StartedAt: time.Now().UTC(),
	}

	if err := h.repo.Create(c.Request().Context(), run); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create agent run"})
	}
	return c.JSON(http.StatusCreated, run.ToDTO())
}

func (h *AgentHandler) List(c echo.Context) error {
	runs, err := h.repo.ListActive(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list agent runs"})
	}
	dtos := make([]model.AgentRunDTO, 0, len(runs))
	for _, r := range runs {
		dtos = append(dtos, r.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *AgentHandler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid agent run ID"})
	}
	run, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "agent run not found"})
	}
	return c.JSON(http.StatusOK, run.ToDTO())
}

func (h *AgentHandler) Pause(c echo.Context) error {
	return h.updateStatus(c, model.AgentRunStatusPaused)
}

func (h *AgentHandler) Resume(c echo.Context) error {
	return h.updateStatus(c, model.AgentRunStatusRunning)
}

func (h *AgentHandler) Kill(c echo.Context) error {
	return h.updateStatus(c, model.AgentRunStatusCancelled)
}

func (h *AgentHandler) updateStatus(c echo.Context, status string) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid agent run ID"})
	}
	if err := h.repo.UpdateStatus(c.Request().Context(), id, status); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to update agent run status"})
	}
	run, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to fetch agent run"})
	}
	return c.JSON(http.StatusOK, run.ToDTO())
}
