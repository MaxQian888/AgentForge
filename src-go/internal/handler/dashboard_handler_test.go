package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/server/internal/handler"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type dashboardCrudServiceMock struct {
	configs       []*model.DashboardConfig
	widgets       []*model.DashboardWidget
	createdConfig *model.DashboardConfig
	savedWidget   *model.DashboardWidget
}

func (m *dashboardCrudServiceMock) CreateDashboard(_ context.Context, config *model.DashboardConfig) error {
	if config.ID == uuid.Nil {
		config.ID = uuid.New()
	}
	m.createdConfig = config
	return nil
}
func (m *dashboardCrudServiceMock) UpdateDashboard(_ context.Context, _ *model.DashboardConfig) error {
	return nil
}
func (m *dashboardCrudServiceMock) DeleteDashboard(_ context.Context, _ uuid.UUID) error { return nil }
func (m *dashboardCrudServiceMock) ListDashboards(_ context.Context, _ uuid.UUID) ([]*model.DashboardConfig, error) {
	return m.configs, nil
}
func (m *dashboardCrudServiceMock) GetDashboard(_ context.Context, id uuid.UUID) (*model.DashboardConfig, error) {
	for _, config := range m.configs {
		if config.ID == id {
			return config, nil
		}
	}
	return nil, nil
}
func (m *dashboardCrudServiceMock) SaveWidget(_ context.Context, widget *model.DashboardWidget) error {
	if widget.ID == uuid.Nil {
		widget.ID = uuid.New()
	}
	m.savedWidget = widget
	return nil
}
func (m *dashboardCrudServiceMock) DeleteWidget(_ context.Context, _ uuid.UUID) error { return nil }
func (m *dashboardCrudServiceMock) ListWidgets(_ context.Context, _ uuid.UUID) ([]*model.DashboardWidget, error) {
	return m.widgets, nil
}

type dashboardDataServiceMock struct{ payload map[string]any }

func (m *dashboardDataServiceMock) WidgetData(_ context.Context, _ uuid.UUID, _ string, _ string) (map[string]any, error) {
	return m.payload, nil
}

func TestDashboardHandlerCrudAndWidgetData(t *testing.T) {
	projectID := uuid.New()
	dashboardID := uuid.New()
	now := time.Now().UTC()
	crud := &dashboardCrudServiceMock{
		configs: []*model.DashboardConfig{{ID: dashboardID, ProjectID: projectID, Name: "Sprint Overview", Layout: `[]`, CreatedBy: uuid.New(), CreatedAt: now, UpdatedAt: now}},
		widgets: []*model.DashboardWidget{{ID: uuid.New(), DashboardID: dashboardID, WidgetType: model.DashboardWidgetThroughputChart, Config: `{}`, Position: `{}`, CreatedAt: now, UpdatedAt: now}},
	}
	data := &dashboardDataServiceMock{payload: map[string]any{"widgetType": model.DashboardWidgetThroughputChart, "points": []map[string]any{{"date": "2026-03-26", "count": 2}}}}

	e := echo.New()
	e.Validator = &customFieldValidator{validator: validator.New()}
	h := handler.NewDashboardHandler(crud, data)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/dashboards", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	listCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	if err := h.List(listCtx); err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusOK)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/dashboards", strings.NewReader(`{"name":"Overview","layout":[]}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	createCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	createCtx.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: uuid.New().String()})
	if err := h.Create(createCtx); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRec.Code, http.StatusCreated)
	}

	widgetReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/dashboard/widgets/"+model.DashboardWidgetThroughputChart+"?config=%7B%7D", nil)
	widgetRec := httptest.NewRecorder()
	widgetCtx := e.NewContext(widgetReq, widgetRec)
	widgetCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	widgetCtx.SetPath("/api/v1/projects/:pid/dashboard/widgets/:type")
	widgetCtx.SetParamNames("pid", "type")
	widgetCtx.SetParamValues(projectID.String(), model.DashboardWidgetThroughputChart)
	if err := h.WidgetData(widgetCtx); err != nil {
		t.Fatalf("WidgetData() error: %v", err)
	}
	if widgetRec.Code != http.StatusOK {
		t.Fatalf("widget status = %d, want %d", widgetRec.Code, http.StatusOK)
	}
}
