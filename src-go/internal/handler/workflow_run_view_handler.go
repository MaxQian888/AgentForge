package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// WorkflowRunViewServiceInterface narrows the WorkflowRunViewService methods
// the handler uses, so tests can supply a stub.
type WorkflowRunViewServiceInterface interface {
	ListRuns(ctx context.Context, projectID uuid.UUID, filter service.UnifiedRunListFilter, cursor string, limit int) (*service.UnifiedRunListResult, error)
	GetRun(ctx context.Context, projectID uuid.UUID, engine string, runID uuid.UUID) (*service.UnifiedRunDetail, error)
}

// WorkflowRunViewHandler mounts the cross-engine workflow-run list/detail
// endpoints consumed by the workflow workspace UI.
type WorkflowRunViewHandler struct {
	svc WorkflowRunViewServiceInterface
}

// NewWorkflowRunViewHandler constructs the handler. Pass a nil service only in
// tests that exercise the param-validation paths.
func NewWorkflowRunViewHandler(svc WorkflowRunViewServiceInterface) *WorkflowRunViewHandler {
	return &WorkflowRunViewHandler{svc: svc}
}

// List handles `GET /api/v1/projects/:pid/workflow-runs`.
func (h *WorkflowRunViewHandler) List(c echo.Context) error {
	if h.svc == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "workflow run view service unavailable"})
	}
	projectID := appMiddleware.GetProjectID(c)
	q := c.QueryParams()

	filter := service.UnifiedRunListFilter{
		Engine:          strings.ToLower(strings.TrimSpace(q.Get("engine"))),
		Statuses:        splitCSV(q["status"]),
		TriggeredByKind: strings.ToLower(strings.TrimSpace(q.Get("triggeredByKind"))),
	}
	if filter.Engine != "" && filter.Engine != service.UnifiedRunEngineDAG && filter.Engine != service.UnifiedRunEnginePlugin {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid engine value"})
	}
	if acting := strings.TrimSpace(q.Get("actingEmployeeId")); acting != "" {
		id, err := uuid.Parse(acting)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid actingEmployeeId"})
		}
		filter.ActingEmployeeID = &id
	}
	if trig := strings.TrimSpace(q.Get("triggerId")); trig != "" {
		id, err := uuid.Parse(trig)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid triggerId"})
		}
		filter.TriggerID = &id
	}
	if after := strings.TrimSpace(q.Get("startedAfter")); after != "" {
		t, err := time.Parse(time.RFC3339Nano, after)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid startedAfter"})
		}
		filter.StartedAfter = &t
	}
	if before := strings.TrimSpace(q.Get("startedBefore")); before != "" {
		t, err := time.Parse(time.RFC3339Nano, before)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid startedBefore"})
		}
		filter.StartedBefore = &t
	}

	limit := 50
	if lim := strings.TrimSpace(q.Get("limit")); lim != "" {
		parsed, err := strconv.Atoi(lim)
		if err != nil || parsed <= 0 {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid limit"})
		}
		limit = parsed
	}
	cursor := strings.TrimSpace(q.Get("cursor"))

	result, err := h.svc.ListRuns(c.Request().Context(), projectID, filter, cursor, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

// Detail handles `GET /api/v1/projects/:pid/workflow-runs/:engine/:id`.
func (h *WorkflowRunViewHandler) Detail(c echo.Context) error {
	if h.svc == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "workflow run view service unavailable"})
	}
	projectID := appMiddleware.GetProjectID(c)
	engine := strings.ToLower(strings.TrimSpace(c.Param("engine")))
	if engine != service.UnifiedRunEngineDAG && engine != service.UnifiedRunEnginePlugin {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid engine"})
	}
	runID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid run id"})
	}

	detail, err := h.svc.GetRun(c.Request().Context(), projectID, engine, runID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "run not found"})
		}
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, detail)
}

// splitCSV accepts both repeated query-param values and comma-joined values
// for convenience (status=running&status=paused OR status=running,paused).
// Empty strings are dropped.
func splitCSV(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}
