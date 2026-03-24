package handler

import (
	"net/http"

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
