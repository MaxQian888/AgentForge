package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type AgentRuntimeService interface {
	Spawn(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error)
	ListActive(ctx context.Context) ([]*model.AgentRun, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRun, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	Cancel(ctx context.Context, id uuid.UUID, reason string) error
}

type AgentHandler struct {
	service    AgentRuntimeService
	dispatcher agentTaskDispatcher
}

func NewAgentHandler(service AgentRuntimeService) *AgentHandler {
	return &AgentHandler{service: service}
}

type agentTaskDispatcher interface {
	Spawn(ctx context.Context, input service.DispatchSpawnInput) (*model.TaskDispatchResponse, error)
}

func (h *AgentHandler) WithDispatcher(dispatcher agentTaskDispatcher) *AgentHandler {
	h.dispatcher = dispatcher
	return h
}

type SpawnAgentRequest struct {
	TaskID   string `json:"taskId" validate:"required"`
	MemberID string `json:"memberId"`
	Runtime  string `json:"runtime"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	RoleID   string `json:"roleId"`
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
	if h.dispatcher != nil {
		var memberID *uuid.UUID
		if strings.TrimSpace(req.MemberID) != "" {
			parsedMemberID, err := uuid.Parse(req.MemberID)
			if err != nil {
				return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid member ID"})
			}
			memberID = &parsedMemberID
		}

		result, err := h.dispatcher.Spawn(c.Request().Context(), service.DispatchSpawnInput{
			TaskID:    taskID,
			MemberID:  memberID,
			Runtime:   req.Runtime,
			Provider:  req.Provider,
			Model:     req.Model,
			RoleID:    req.RoleID,
			BudgetUSD: 0,
		})
		if err != nil {
			switch {
			case errors.Is(err, service.ErrAgentTaskNotFound), errors.Is(err, service.ErrAgentProjectNotFound):
				return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
			default:
				return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: "failed to start agent run"})
			}
		}

		statusCode := http.StatusOK
		if result.Dispatch.Status == model.DispatchStatusStarted {
			statusCode = http.StatusCreated
		}
		return c.JSON(statusCode, result)
	}

	memberID, err := uuid.Parse(req.MemberID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid member ID"})
	}

	run, err := h.service.Spawn(c.Request().Context(), taskID, memberID, req.Runtime, req.Provider, req.Model, 0, req.RoleID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAgentAlreadyRunning):
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
		case errors.Is(err, service.ErrAgentWorktreeUnavailable):
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
		case errors.Is(err, service.ErrAgentRoleNotFound):
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		case errors.Is(err, service.ErrAgentTaskNotFound), errors.Is(err, service.ErrAgentProjectNotFound):
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		default:
			return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: "failed to start agent run"})
		}
	}

	return c.JSON(http.StatusCreated, run.ToDTO())
}

func (h *AgentHandler) List(c echo.Context) error {
	runs, err := h.service.ListActive(c.Request().Context())
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
	run, err := h.service.GetByID(c.Request().Context(), id)
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
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid agent run ID"})
	}

	if err := h.service.Cancel(c.Request().Context(), id, "killed_by_user"); err != nil {
		switch {
		case errors.Is(err, service.ErrAgentNotFound):
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		case errors.Is(err, service.ErrAgentNotRunning):
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
		default:
			return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: "failed to cancel agent run"})
		}
	}

	run, err := h.service.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to fetch agent run"})
	}
	return c.JSON(http.StatusOK, run.ToDTO())
}

func (h *AgentHandler) updateStatus(c echo.Context, status string) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid agent run ID"})
	}
	if err := h.service.UpdateStatus(c.Request().Context(), id, status); err != nil {
		switch {
		case errors.Is(err, service.ErrAgentNotFound):
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		case errors.Is(err, service.ErrAgentNotRunning):
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
		default:
			return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: "failed to update agent run status"})
		}
	}
	run, err := h.service.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to fetch agent run"})
	}
	return c.JSON(http.StatusOK, run.ToDTO())
}
