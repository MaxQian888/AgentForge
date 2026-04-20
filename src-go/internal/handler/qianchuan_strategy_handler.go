package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

// QianchuanStrategyAssignService is the narrow contract for assigning a
// strategy to a binding + materializing the schedule trigger.
type QianchuanStrategyAssignService interface {
	AssignStrategy(ctx context.Context, in StrategyAssignInput) (*StrategyAssignResult, error)
	UnassignStrategy(ctx context.Context, bindingID uuid.UUID) error
}

// StrategyAssignInput carries the assignment request parameters.
type StrategyAssignInput struct {
	BindingID        uuid.UUID
	StrategyID       uuid.UUID
	ScheduleOverride string
	ProjectID        uuid.UUID
	ActorID          uuid.UUID
}

// StrategyAssignResult is returned on successful assignment.
type StrategyAssignResult struct {
	TriggerID uuid.UUID `json:"triggerId"`
	Cron      string    `json:"cron"`
}

// QianchuanStrategyHandler handles strategy assignment endpoints.
type QianchuanStrategyHandler struct {
	service QianchuanStrategyAssignService
}

// NewQianchuanStrategyHandler wires the handler.
func NewQianchuanStrategyHandler(svc QianchuanStrategyAssignService) *QianchuanStrategyHandler {
	return &QianchuanStrategyHandler{service: svc}
}

// RegisterFlat registers per-binding strategy endpoints.
func (h *QianchuanStrategyHandler) RegisterFlat(e *echo.Echo) {
	g := e.Group("/api/v1/qianchuan/bindings")
	g.POST("/:id/strategy", h.Assign, appMiddleware.Require(appMiddleware.ActionQianchuanBindingUpdate))
	g.DELETE("/:id/strategy", h.Unassign, appMiddleware.Require(appMiddleware.ActionQianchuanBindingUpdate))
}

type assignStrategyRequest struct {
	StrategyID       string `json:"strategy_id" validate:"required"`
	ScheduleOverride string `json:"schedule_override,omitempty"`
}

func (h *QianchuanStrategyHandler) Assign(c echo.Context) error {
	bindingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid binding id"})
	}

	var req assignStrategyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	strategyID, err := uuid.Parse(req.StrategyID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid strategy_id"})
	}

	projectID := appMiddleware.GetProjectID(c)
	actorPtr, _ := claimsUserID(c)
	actorID := uuid.Nil
	if actorPtr != nil {
		actorID = *actorPtr
	}

	result, err := h.service.AssignStrategy(c.Request().Context(), StrategyAssignInput{
		BindingID:        bindingID,
		StrategyID:       strategyID,
		ScheduleOverride: req.ScheduleOverride,
		ProjectID:        projectID,
		ActorID:          actorID,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

func (h *QianchuanStrategyHandler) Unassign(c echo.Context) error {
	bindingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid binding id"})
	}

	if err := h.service.UnassignStrategy(c.Request().Context(), bindingID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}

// ── Strategy assignment service (default impl) ──────────────────────────

// StrategyAssignServiceDeps bundles the repos needed by the default service.
type StrategyAssignServiceDeps struct {
	TriggerRepo    StrategyTriggerRepo
	StrategyLoader StrategySpecLoader
	DefRepo        StrategyDefLookup
}

// StrategyTriggerRepo is the persistence contract for trigger rows.
type StrategyTriggerRepo interface {
	Create(ctx context.Context, tr *model.WorkflowTrigger) error
	Update(ctx context.Context, tr *model.WorkflowTrigger) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByConfigField(ctx context.Context, field, value string) (*model.WorkflowTrigger, error)
}

// StrategySpecLoader loads a strategy's parsed spec to extract schedule.
type StrategySpecLoader interface {
	Load(ctx context.Context, id uuid.UUID) (json.RawMessage, error)
}

// StrategyDefLookup resolves the system workflow definition ID by name.
type StrategyDefLookup interface {
	GetByName(ctx context.Context, name string) (*model.WorkflowDefinition, error)
}
