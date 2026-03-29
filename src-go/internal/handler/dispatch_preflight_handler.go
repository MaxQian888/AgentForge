package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
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
}

type DispatchPreflightHandler struct {
	tasks   dispatchPreflightTaskReader
	members dispatchPreflightMemberReader
	budget  service.DispatchBudgetChecker
	pool    dispatchPreflightPoolStatsProvider
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

	if member.Type != model.MemberTypeAgent {
		return c.JSON(http.StatusOK, PreflightResponse{
			AdmissionLikely:     false,
			DispatchOutcomeHint: model.DispatchStatusSkipped,
		})
	}
	if !member.IsActive {
		return c.JSON(http.StatusOK, PreflightResponse{
			AdmissionLikely:     false,
			DispatchOutcomeHint: model.DispatchStatusBlocked,
			BudgetBlocked: &DispatchPreflightBudgetState{
				Scope:   "member",
				Message: "dispatch target is not an active agent member",
			},
		})
	}

	response := PreflightResponse{}
	if h.budget != nil {
		budgetResult, budgetErr := h.budget.CheckBudget(c.Request().Context(), task.ProjectID, task.SprintID, task.BudgetUsd)
		if budgetErr == nil && budgetResult != nil {
			scope := budgetResult.Scope
			if scope == "" {
				scope = inferPreflightBudgetScope(budgetResult.Reason + " " + budgetResult.WarningMessage)
			}
			if !budgetResult.Allowed {
				response.BudgetBlocked = &DispatchPreflightBudgetState{
					Scope:   scope,
					Message: budgetResult.Reason,
				}
				response.DispatchOutcomeHint = model.DispatchStatusBlocked
				return c.JSON(http.StatusOK, response)
			}
			if budgetResult.Warning {
				response.BudgetWarning = &DispatchPreflightBudgetState{
					Scope:   scope,
					Message: budgetResult.WarningMessage,
				}
			}
		}
	}

	stats := h.pool.PoolStats(c.Request().Context())
	response.PoolActive = intPtr(stats.Active)
	response.PoolAvailable = intPtr(stats.Available)
	response.PoolQueued = intPtr(stats.Queued)
	response.AdmissionLikely = stats.Available > 0
	if response.AdmissionLikely {
		response.DispatchOutcomeHint = model.DispatchStatusStarted
	} else {
		response.DispatchOutcomeHint = model.DispatchStatusQueued
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
