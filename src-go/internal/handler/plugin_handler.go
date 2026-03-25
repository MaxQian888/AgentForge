package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/service"
)

type PluginHandler struct {
	service  *service.PluginService
	workflow WorkflowExecutionRuntime
}

type WorkflowExecutionRuntime interface {
	Start(ctx context.Context, pluginID string, req service.WorkflowExecutionRequest) (*model.WorkflowPluginRun, error)
	GetRun(ctx context.Context, id uuid.UUID) (*model.WorkflowPluginRun, error)
	ListRuns(ctx context.Context, pluginID string, limit int) ([]*model.WorkflowPluginRun, error)
}

func NewPluginHandler(service *service.PluginService) *PluginHandler {
	return &PluginHandler{service: service}
}

func (h *PluginHandler) WithWorkflowExecution(workflow WorkflowExecutionRuntime) *PluginHandler {
	h.workflow = workflow
	return h
}

func (h *PluginHandler) DiscoverBuiltIns(c echo.Context) error {
	records, err := h.service.DiscoverBuiltIns(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, records)
}

func (h *PluginHandler) InstallLocal(c echo.Context) error {
	var req struct {
		Path    string              `json:"path"`
		EntryID string              `json:"entry_id"`
		Source  *model.PluginSource `json:"source"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid install request"})
	}

	var (
		record *model.PluginRecord
		err    error
	)
	switch {
	case req.EntryID != "":
		record, err = h.service.InstallCatalogEntry(c.Request().Context(), req.EntryID)
	case req.Path == "":
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "path or entry_id is required"})
	case req.Source == nil || req.Source.Type == "" || req.Source.Type == model.PluginSourceLocal:
		record, err = h.service.RegisterLocalPath(c.Request().Context(), req.Path)
	default:
		record, err = h.service.Install(c.Request().Context(), service.PluginInstallRequest{
			Path:   req.Path,
			Source: req.Source,
		})
	}
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusCreated, record)
}

func (h *PluginHandler) SearchCatalog(c echo.Context) error {
	entries, err := h.service.SearchCatalog(c.Request().Context(), c.QueryParam("q"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, entries)
}

func (h *PluginHandler) List(c echo.Context) error {
	records, err := h.service.List(c.Request().Context(), service.PluginListFilter{
		Kind:           model.PluginKind(c.QueryParam("kind")),
		LifecycleState: model.PluginLifecycleState(c.QueryParam("state")),
		SourceType:     model.PluginSourceType(c.QueryParam("source")),
		TrustState:     model.PluginTrustState(c.QueryParam("trust")),
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, records)
}

func (h *PluginHandler) Enable(c echo.Context) error {
	record, err := h.service.Enable(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, record)
}

func (h *PluginHandler) Disable(c echo.Context) error {
	record, err := h.service.Disable(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, record)
}

func (h *PluginHandler) Deactivate(c echo.Context) error {
	record, err := h.service.Deactivate(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, record)
}

func (h *PluginHandler) Activate(c echo.Context) error {
	record, err := h.service.Activate(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, record)
}

func (h *PluginHandler) Health(c echo.Context) error {
	record, err := h.service.CheckHealth(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, record)
}

func (h *PluginHandler) Restart(c echo.Context) error {
	record, err := h.service.Restart(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, record)
}

func (h *PluginHandler) Invoke(c echo.Context) error {
	var req struct {
		Operation string         `json:"operation"`
		Payload   map[string]any `json:"payload"`
	}
	if err := c.Bind(&req); err != nil || req.Operation == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "operation is required"})
	}
	if req.Payload == nil {
		req.Payload = map[string]any{}
	}

	result, err := h.service.Invoke(c.Request().Context(), c.Param("id"), req.Operation, req.Payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"plugin_id": c.Param("id"),
		"operation": req.Operation,
		"result":    result,
	})
}

func (h *PluginHandler) RefreshMCP(c echo.Context) error {
	result, err := h.service.RefreshMCP(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h *PluginHandler) CallMCPTool(c echo.Context) error {
	var req struct {
		ToolName  string         `json:"tool_name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := c.Bind(&req); err != nil || req.ToolName == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "tool_name is required"})
	}
	if req.Arguments == nil {
		req.Arguments = map[string]any{}
	}

	result, err := h.service.CallMCPTool(c.Request().Context(), c.Param("id"), req.ToolName, req.Arguments)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h *PluginHandler) ReadMCPResource(c echo.Context) error {
	var req struct {
		URI string `json:"uri"`
	}
	if err := c.Bind(&req); err != nil || req.URI == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "uri is required"})
	}

	result, err := h.service.ReadMCPResource(c.Request().Context(), c.Param("id"), req.URI)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h *PluginHandler) GetMCPPrompt(c echo.Context) error {
	var req struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "name is required"})
	}

	result, err := h.service.GetMCPPrompt(c.Request().Context(), c.Param("id"), req.Name, req.Arguments)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h *PluginHandler) Uninstall(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "plugin id required"})
	}
	if err := h.service.Uninstall(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "plugin uninstalled"})
}

func (h *PluginHandler) Update(c echo.Context) error {
	var req struct {
		Path   string              `json:"path"`
		Source *model.PluginSource `json:"source"`
	}
	if err := c.Bind(&req); err != nil || req.Path == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "path is required"})
	}
	record, err := h.service.Update(c.Request().Context(), c.Param("id"), service.PluginInstallRequest{
		Path:   req.Path,
		Source: req.Source,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, record)
}

func (h *PluginHandler) InstallCatalogEntry(c echo.Context) error {
	var req struct {
		EntryID string `json:"entry_id"`
	}
	if err := c.Bind(&req); err != nil || req.EntryID == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "entry_id is required"})
	}
	record, err := h.service.InstallCatalogEntry(c.Request().Context(), req.EntryID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusCreated, record)
}

func (h *PluginHandler) UpdateConfig(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "plugin id required"})
	}
	req := new(model.UpdatePluginConfigRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	rec, err := h.service.UpdateConfig(c.Request().Context(), id, req.Config)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, rec)
}

func (h *PluginHandler) Marketplace(c echo.Context) error {
	plugins, err := h.service.ListMarketplace(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list marketplace"})
	}
	return c.JSON(http.StatusOK, plugins)
}

func (h *PluginHandler) ListEvents(c echo.Context) error {
	limit := 20
	if raw := c.QueryParam("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "limit must be a positive integer"})
		}
		limit = parsed
	}

	events, err := h.service.ListEvents(c.Request().Context(), c.Param("id"), limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, events)
}

func (h *PluginHandler) StartWorkflowRun(c echo.Context) error {
	if h.workflow == nil {
		return c.JSON(http.StatusNotImplemented, model.ErrorResponse{Message: "workflow execution is not configured"})
	}
	var req service.WorkflowExecutionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid workflow execution request"})
	}
	run, err := h.workflow.Start(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusCreated, run)
}

func (h *PluginHandler) ListWorkflowRuns(c echo.Context) error {
	if h.workflow == nil {
		return c.JSON(http.StatusNotImplemented, model.ErrorResponse{Message: "workflow execution is not configured"})
	}
	limit := 20
	if raw := c.QueryParam("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "limit must be a positive integer"})
		}
		limit = parsed
	}
	runs, err := h.workflow.ListRuns(c.Request().Context(), c.Param("id"), limit)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, runs)
}

func (h *PluginHandler) GetWorkflowRun(c echo.Context) error {
	if h.workflow == nil {
		return c.JSON(http.StatusNotImplemented, model.ErrorResponse{Message: "workflow execution is not configured"})
	}
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "runId must be a valid UUID"})
	}
	run, err := h.workflow.GetRun(c.Request().Context(), runID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		}
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, run)
}

func (h *PluginHandler) SyncRuntimeState(c echo.Context) error {
	var update model.PluginRuntimeStatus
	if err := c.Bind(&update); err != nil || update.PluginID == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "plugin_id is required"})
	}

	record, err := h.service.ReportRuntimeState(c.Request().Context(), update.PluginID, update)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, record)
}
