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
	audit      AgentAuditEmitter
}

// AgentAuditEmitter is the narrow contract the handler uses to record
// audit events on the success path. *service.AuditService satisfies it.
type AgentAuditEmitter interface {
	RecordEvent(ctx context.Context, event *model.AuditEvent) error
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

// WithAudit wires the audit service so successful Spawn calls emit a
// business-level event with the dispatch outcome and key request fields.
// Without this seam the only audit record is the RBAC-layer "allowed"
// event from middleware/rbac.go.
func (h *AgentHandler) WithAudit(audit AgentAuditEmitter) *AgentHandler {
	h.audit = audit
	return h
}

type SpawnAgentRequest struct {
	TaskID       string  `json:"taskId" validate:"required"`
	MemberID     string  `json:"memberId"`
	Runtime      string  `json:"runtime"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	RoleID       string  `json:"roleId"`
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

		// Plumb the initiating user from JWT claims into the service-layer
		// Caller. Required by the typed contract introduced for RBAC §3;
		// audit emission references this identity.
		callerID, err := claimsUserID(c)
		if err != nil || callerID == nil {
			return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
		}
		result, err := h.dispatcher.Spawn(c.Request().Context(), service.DispatchSpawnInput{
			TaskID:    taskID,
			MemberID:  memberID,
			Runtime:   req.Runtime,
			Provider:  req.Provider,
			Model:     req.Model,
			RoleID:    req.RoleID,
			BudgetUSD: req.MaxBudgetUsd,
			Caller:    service.Caller{UserID: *callerID},
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
		// Handler-level audit emission. The RBAC middleware already
		// recorded the "allowed" attempt; this row carries the actual
		// business outcome so /audit-events answers "what happened" not
		// just "who tried."
		h.emitSpawnAudit(c, *callerID, taskID, memberID, req, result)
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
		case errors.Is(err, service.ErrAgentBridgeUnavailable):
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "bridge_unavailable"})
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

// emitSpawnAudit records a business-level audit event for a successful
// agent spawn. Best-effort: a failure here is logged inside the audit
// service and never blocks the response.
//
// The payload focuses on the authoritative outcome — which task, which
// member, what dispatch status. Sanitization happens inside RecordEvent.
func (h *AgentHandler) emitSpawnAudit(
	c echo.Context,
	callerID uuid.UUID,
	taskID uuid.UUID,
	memberID *uuid.UUID,
	req *SpawnAgentRequest,
	result *model.TaskDispatchResponse,
) {
	if h.audit == nil || result == nil {
		return
	}
	payload := map[string]any{
		"outcome":        "spawned",
		"taskId":         taskID.String(),
		"runtime":        req.Runtime,
		"provider":       req.Provider,
		"model":          req.Model,
		"roleId":         req.RoleID,
		"dispatchStatus": string(result.Dispatch.Status),
		"dispatchReason": result.Dispatch.Reason,
	}
	if memberID != nil {
		payload["memberId"] = memberID.String()
	}
	caller := callerID
	event := &model.AuditEvent{
		ProjectID:           parseProjectIDOrNil(result.Task.ProjectID),
		ActorUserID:         &caller,
		ActionID:            "agent.spawn",
		ResourceType:        model.AuditResourceTypeTask,
		ResourceID:          taskID.String(),
		PayloadSnapshotJSON: service.SanitizeAuditPayload(payload),
	}
	_ = h.audit.RecordEvent(c.Request().Context(), event)
}

// parseProjectIDOrNil parses the projectId string carried on the
// dispatch response. Returns Nil on parse failure; the audit service
// will reject rows with Nil project_id, surfacing the issue rather
// than silently landing orphaned events.
func parseProjectIDOrNil(s string) uuid.UUID {
	if id, err := uuid.Parse(s); err == nil {
		return id
	}
	return uuid.Nil
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
