package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type AgentRuntimeService interface {
	Spawn(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error)
	ListActive(ctx context.Context) ([]*model.AgentRun, error)
	ListSummaries(ctx context.Context) ([]model.AgentRunSummaryDTO, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRun, error)
	GetSummary(ctx context.Context, id uuid.UUID) (*model.AgentRunSummaryDTO, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	Cancel(ctx context.Context, id uuid.UUID, reason string) error
	PoolStats(ctx context.Context) model.AgentPoolStatsDTO
	GetLogs(ctx context.Context, id uuid.UUID) ([]model.AgentLogEntry, error)
	BridgeStatus() string
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
	MaxBudgetUsd float64 `json:"maxBudgetUsd"`
}

func (h *AgentHandler) Spawn(c echo.Context) error {
	req := new(SpawnAgentRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	if h.service != nil && h.service.BridgeStatus() == service.BridgeStatusDegraded {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "bridge_unavailable"})
	}

	taskID, err := uuid.Parse(req.TaskID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	if h.dispatcher != nil {
		var memberID *uuid.UUID
		if strings.TrimSpace(req.MemberID) != "" {
			parsedMemberID, err := uuid.Parse(req.MemberID)
			if err != nil {
				return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidMemberID)
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
			BudgetUSD: req.MaxBudgetUsd,
		})
		if err != nil {
			switch {
			case errors.Is(err, service.ErrAgentTaskNotFound), errors.Is(err, service.ErrAgentProjectNotFound):
				return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
			default:
				return localizedError(c, http.StatusBadGateway, i18n.MsgFailedToStartAgentRun)
			}
		}

		statusCode := http.StatusOK
		if result.Dispatch.Status == model.DispatchStatusStarted {
			statusCode = http.StatusCreated
		} else if result.Dispatch.Status == model.DispatchStatusQueued {
			statusCode = http.StatusAccepted
		}
		return c.JSON(statusCode, result)
	}

	memberID, err := uuid.Parse(req.MemberID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidMemberID)
	}

	run, err := h.service.Spawn(c.Request().Context(), taskID, memberID, req.Runtime, req.Provider, req.Model, req.MaxBudgetUsd, req.RoleID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAgentAlreadyRunning):
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
		case errors.Is(err, service.ErrAgentPoolFull):
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
		case errors.Is(err, service.ErrAgentWorktreeUnavailable):
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
		case errors.Is(err, service.ErrAgentRoleNotFound):
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		case errors.Is(err, service.ErrAgentTaskNotFound), errors.Is(err, service.ErrAgentProjectNotFound):
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		default:
			return localizedError(c, http.StatusBadGateway, i18n.MsgFailedToStartAgentRun)
		}
	}

	summary, summaryErr := h.service.GetSummary(c.Request().Context(), run.ID)
	if summaryErr == nil && summary != nil {
		return c.JSON(http.StatusCreated, summary)
	}
	return c.JSON(http.StatusCreated, run.ToDTO())
}

func (h *AgentHandler) List(c echo.Context) error {
	runs, err := h.service.ListSummaries(c.Request().Context())
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListAgentRuns)
	}
	return c.JSON(http.StatusOK, runs)
}

func (h *AgentHandler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidAgentRunID)
	}
	run, err := h.service.GetSummary(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgAgentRunNotFound)
	}
	return c.JSON(http.StatusOK, run)
}

func (h *AgentHandler) Pool(c echo.Context) error {
	return c.JSON(http.StatusOK, h.service.PoolStats(c.Request().Context()))
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidAgentRunID)
	}

	if err := h.service.Cancel(c.Request().Context(), id, "killed_by_user"); err != nil {
		switch {
		case errors.Is(err, service.ErrAgentNotFound):
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		case errors.Is(err, service.ErrAgentNotRunning):
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
		default:
			return localizedError(c, http.StatusBadGateway, i18n.MsgFailedToCancelAgentRun)
		}
	}

	run, err := h.service.GetByID(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToFetchAgentRun)
	}
	summary, summaryErr := h.service.GetSummary(c.Request().Context(), run.ID)
	if summaryErr == nil && summary != nil {
		return c.JSON(http.StatusOK, summary)
	}
	return c.JSON(http.StatusOK, run.ToDTO())
}

func (h *AgentHandler) Logs(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidAgentRunID)
	}
	logs, err := h.service.GetLogs(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrAgentNotFound) {
			return localizedError(c, http.StatusNotFound, i18n.MsgAgentRunNotFound)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToGetAgentLogs)
	}
	return c.JSON(http.StatusOK, logs)
}

func (h *AgentHandler) updateStatus(c echo.Context, status string) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidAgentRunID)
	}
	if h.service != nil && h.service.BridgeStatus() == service.BridgeStatusDegraded {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "bridge_unavailable"})
	}
	if err := h.service.UpdateStatus(c.Request().Context(), id, status); err != nil {
		switch {
		case errors.Is(err, service.ErrAgentNotFound):
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		case errors.Is(err, service.ErrAgentNotRunning):
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
		default:
			return localizedError(c, http.StatusBadGateway, i18n.MsgFailedToUpdateAgentRunStatus)
		}
	}
	run, err := h.service.GetByID(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToFetchAgentRun)
	}
	summary, summaryErr := h.service.GetSummary(c.Request().Context(), run.ID)
	if summaryErr == nil && summary != nil {
		return c.JSON(http.StatusOK, summary)
	}
	return c.JSON(http.StatusOK, run.ToDTO())
}
