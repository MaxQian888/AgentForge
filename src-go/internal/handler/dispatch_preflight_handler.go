package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/agentforge/server/internal/i18n"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type dispatchPreflightTaskReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
}

type dispatchPreflightMemberReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Member, error)
}

type dispatchPreflightPoolStatsProvider interface {
	PoolStats(ctx context.Context) model.AgentPoolStatsDTO
}

type dispatchPreflightRunReader interface {
	GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.AgentRun, error)
}

type DispatchPreflightBudgetState struct {
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

type PreflightResponse struct {
	AdmissionLikely     bool                          `json:"admissionLikely"`
	BudgetWarning       *DispatchPreflightBudgetState `json:"budgetWarning,omitempty"`
	BudgetBlocked       *DispatchPreflightBudgetState `json:"budgetBlocked,omitempty"`
	PoolActive          *int                          `json:"poolActive,omitempty"`
	PoolAvailable       *int                          `json:"poolAvailable,omitempty"`
	PoolQueued          *int                          `json:"poolQueued,omitempty"`
	DispatchOutcomeHint string                        `json:"dispatchOutcomeHint"`
	GuardrailType       string                        `json:"guardrailType,omitempty"`
	GuardrailScope      string                        `json:"guardrailScope,omitempty"`
	Reason              string                        `json:"reason,omitempty"`
	Runtime             string                        `json:"runtime,omitempty"`
	Provider            string                        `json:"provider,omitempty"`
	Model               string                        `json:"model,omitempty"`
	RoleID              string                        `json:"roleId,omitempty"`
}

type DispatchPreflightHandler struct {
	tasks     dispatchPreflightTaskReader
	members   dispatchPreflightMemberReader
	roleStore memberRoleStore
	budget    service.DispatchBudgetChecker
	pool      dispatchPreflightPoolStatsProvider
	runs      dispatchPreflightRunReader
}

func NewDispatchPreflightHandler(
	tasks dispatchPreflightTaskReader,
	members dispatchPreflightMemberReader,
	budget service.DispatchBudgetChecker,
	pool dispatchPreflightPoolStatsProvider,
) *DispatchPreflightHandler {
	return &DispatchPreflightHandler{
		tasks:   tasks,
		members: members,
		budget:  budget,
		pool:    pool,
	}
}

func (h *DispatchPreflightHandler) WithRunReader(runs dispatchPreflightRunReader) *DispatchPreflightHandler {
	h.runs = runs
	return h
}

func (h *DispatchPreflightHandler) WithRoleStore(store memberRoleStore) *DispatchPreflightHandler {
	h.roleStore = store
	return h
}

func (h *DispatchPreflightHandler) Get(c echo.Context) error {
	taskID, err := uuid.Parse(c.QueryParam("taskId"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	memberID, err := uuid.Parse(c.QueryParam("memberId"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidMemberID)
	}

	projectID := appMiddleware.GetProjectID(c)
	task, err := h.tasks.GetByID(c.Request().Context(), taskID)
	if err != nil || task == nil || task.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgTaskNotFound)
	}
	member, err := h.members.GetByID(c.Request().Context(), memberID)
	if err != nil || member == nil || member.ProjectID != projectID {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "member not found"})
	}

	budgetUSD := 0.0
	if rawBudget := strings.TrimSpace(c.QueryParam("budgetUsd")); rawBudget != "" {
		parsedBudget, parseErr := strconv.ParseFloat(rawBudget, 64)
		if parseErr != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "budgetUsd must be a valid number"})
		}
		budgetUSD = parsedBudget
	}

	input := service.DispatchSpawnInput{
		TaskID:    task.ID,
		MemberID:  &member.ID,
		Runtime:   strings.TrimSpace(c.QueryParam("runtime")),
		Provider:  strings.TrimSpace(c.QueryParam("provider")),
		Model:     strings.TrimSpace(c.QueryParam("model")),
		RoleID:    strings.TrimSpace(c.QueryParam("roleId")),
		BudgetUSD: budgetUSD,
	}
	input.RoleID = service.ResolveEffectiveRoleID(input.RoleID, member)
	if h.roleStore != nil {
		if err := service.NewRoleReferenceGovernanceService(nil, nil, nil, nil).
			WithRoleStore(h.roleStore).
			ValidateRoleBinding(c.Request().Context(), input.RoleID); err != nil {
			return c.JSON(http.StatusOK, PreflightResponse{
				AdmissionLikely:     false,
				DispatchOutcomeHint: model.DispatchStatusBlocked,
				GuardrailType:       model.DispatchGuardrailTypeTarget,
				GuardrailScope:      "role",
				Reason:              err.Error(),
				Runtime:             input.Runtime,
				Provider:            input.Provider,
				Model:               input.Model,
				RoleID:              input.RoleID,
			})
		}
	}

	result := service.EvaluateDispatchPreflight(c.Request().Context(), task, member, input, h.budget, h.runs, h.pool)
	response := PreflightResponse{
		AdmissionLikely:     result.AdmissionLikely,
		DispatchOutcomeHint: result.Outcome.Status,
		GuardrailType:       result.Outcome.GuardrailType,
		GuardrailScope:      result.Outcome.GuardrailScope,
		Reason:              result.Outcome.Reason,
		Runtime:             result.Outcome.Runtime,
		Provider:            result.Outcome.Provider,
		Model:               result.Outcome.Model,
		RoleID:              result.Outcome.RoleID,
	}
	if result.Outcome.BudgetWarning != nil {
		response.BudgetWarning = &DispatchPreflightBudgetState{
			Scope:   result.Outcome.BudgetWarning.Scope,
			Message: result.Outcome.BudgetWarning.Message,
		}
	}
	if result.Outcome.Status == model.DispatchStatusBlocked && result.Outcome.GuardrailType == model.DispatchGuardrailTypeBudget {
		response.BudgetBlocked = &DispatchPreflightBudgetState{
			Scope:   result.Outcome.GuardrailScope,
			Message: result.Outcome.Reason,
		}
	}
	if result.PoolStats != nil && (result.Outcome.Status == model.DispatchStatusStarted || result.Outcome.Status == model.DispatchStatusQueued) {
		response.PoolActive = intPtr(result.PoolStats.Active)
		response.PoolAvailable = intPtr(result.PoolStats.Available)
		response.PoolQueued = intPtr(result.PoolStats.Queued)
	}

	return c.JSON(http.StatusOK, response)
}

func intPtr(value int) *int {
	return &value
}

func inferPreflightBudgetScope(text string) string {
	switch {
	case text == "":
		return ""
	case containsBudgetScope(text, "task"):
		return "task"
	case containsBudgetScope(text, "sprint"):
		return "sprint"
	case containsBudgetScope(text, "project"):
		return "project"
	default:
		return ""
	}
}

func containsBudgetScope(text string, scope string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(text)), scope+" budget")
}
