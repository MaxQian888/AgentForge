package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type PluginHandler struct {
	service *service.PluginService
}

func NewPluginHandler(service *service.PluginService) *PluginHandler {
	return &PluginHandler{service: service}
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
		Path string `json:"path"`
	}
	if err := c.Bind(&req); err != nil || req.Path == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "path is required"})
	}

	record, err := h.service.RegisterLocalPath(c.Request().Context(), req.Path)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusCreated, record)
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
