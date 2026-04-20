package handler

import (
	"context"
	"net/http"

	"github.com/agentforge/server/internal/i18n"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type dashboardCrudService interface {
	CreateDashboard(ctx context.Context, config *model.DashboardConfig) error
	UpdateDashboard(ctx context.Context, config *model.DashboardConfig) error
	DeleteDashboard(ctx context.Context, id uuid.UUID) error
	ListDashboards(ctx context.Context, projectID uuid.UUID) ([]*model.DashboardConfig, error)
	GetDashboard(ctx context.Context, id uuid.UUID) (*model.DashboardConfig, error)
	SaveWidget(ctx context.Context, widget *model.DashboardWidget) error
	DeleteWidget(ctx context.Context, id uuid.UUID) error
	ListWidgets(ctx context.Context, dashboardID uuid.UUID) ([]*model.DashboardWidget, error)
}

type dashboardDataService interface {
	WidgetData(ctx context.Context, projectID uuid.UUID, widgetType string, configRaw string) (map[string]any, error)
}

type DashboardHandler struct {
	crud dashboardCrudService
	data dashboardDataService
}

func NewDashboardHandler(crud dashboardCrudService, data dashboardDataService) *DashboardHandler {
	return &DashboardHandler{crud: crud, data: data}
}

func (h *DashboardHandler) List(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	configs, err := h.crud.ListDashboards(c.Request().Context(), projectID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListDashboards)
	}
	dtos := make([]model.DashboardConfigDTO, 0, len(configs))
	for _, config := range configs {
		widgets, widgetsErr := h.crud.ListWidgets(c.Request().Context(), config.ID)
		if widgetsErr != nil {
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListWidgets)
		}
		widgetDTOs := make([]model.DashboardWidgetDTO, 0, len(widgets))
		for _, widget := range widgets {
			widgetDTOs = append(widgetDTOs, widget.ToDTO())
		}
		dtos = append(dtos, config.ToDTO(widgetDTOs))
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *DashboardHandler) Create(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	userID, err := claimsUserID(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgAuthRequired)
	}
	req := new(model.CreateDashboardRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	config := &model.DashboardConfig{
		ProjectID: projectID,
		Name:      req.Name,
		Layout:    string(req.Layout),
		CreatedBy: *userID,
	}
	if err := h.crud.CreateDashboard(c.Request().Context(), config); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateDashboard)
	}
	return c.JSON(http.StatusCreated, config.ToDTO(nil))
}

func (h *DashboardHandler) Update(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	dashboardID, err := uuid.Parse(c.Param("did"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidDashboardID)
	}
	config, err := h.crud.GetDashboard(c.Request().Context(), dashboardID)
	if err != nil || config == nil || config.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgDashboardNotFound)
	}
	req := new(model.UpdateDashboardRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if req.Name != nil {
		config.Name = *req.Name
	}
	if len(req.Layout) > 0 {
		config.Layout = string(req.Layout)
	}
	if err := h.crud.UpdateDashboard(c.Request().Context(), config); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateDashboard)
	}
	return c.JSON(http.StatusOK, config.ToDTO(nil))
}

func (h *DashboardHandler) Delete(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	dashboardID, err := uuid.Parse(c.Param("did"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidDashboardID)
	}
	config, err := h.crud.GetDashboard(c.Request().Context(), dashboardID)
	if err != nil || config == nil || config.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgDashboardNotFound)
	}
	if err := h.crud.DeleteDashboard(c.Request().Context(), dashboardID); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteDashboard)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "dashboard deleted"})
}

func (h *DashboardHandler) SaveWidget(c echo.Context) error {
	dashboardID, err := uuid.Parse(c.Param("did"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidDashboardID)
	}
	req := new(model.SaveDashboardWidgetRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	widget := &model.DashboardWidget{
		DashboardID: dashboardID,
		WidgetType:  req.WidgetType,
		Config:      string(req.Config),
		Position:    string(req.Position),
	}
	if req.ID != nil {
		if parsed, parseErr := uuid.Parse(*req.ID); parseErr == nil {
			widget.ID = parsed
		}
	}
	if err := h.crud.SaveWidget(c.Request().Context(), widget); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToSaveWidget)
	}
	return c.JSON(http.StatusOK, widget.ToDTO())
}

func (h *DashboardHandler) ListWidgets(c echo.Context) error {
	dashboardID, err := uuid.Parse(c.Param("did"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidDashboardID)
	}
	widgets, err := h.crud.ListWidgets(c.Request().Context(), dashboardID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListWidgets)
	}
	dtos := make([]model.DashboardWidgetDTO, 0, len(widgets))
	for _, widget := range widgets {
		dtos = append(dtos, widget.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *DashboardHandler) DeleteWidget(c echo.Context) error {
	widgetID, err := uuid.Parse(c.Param("wid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidWidgetID)
	}
	if err := h.crud.DeleteWidget(c.Request().Context(), widgetID); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteWidget)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "widget deleted"})
}

func (h *DashboardHandler) WidgetData(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	payload, err := h.data.WidgetData(c.Request().Context(), projectID, c.Param("type"), c.QueryParam("config"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, payload)
}
