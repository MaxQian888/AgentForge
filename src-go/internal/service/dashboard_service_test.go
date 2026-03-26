package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type fakeDashboardRepo struct {
	configs       map[uuid.UUID]*model.DashboardConfig
	widgetsByDash map[uuid.UUID][]*model.DashboardWidget
	createdConfig *model.DashboardConfig
	updatedConfig *model.DashboardConfig
	deletedConfig uuid.UUID
	savedWidget   *model.DashboardWidget
	deletedWidget uuid.UUID
}

func (f *fakeDashboardRepo) CreateConfig(_ context.Context, config *model.DashboardConfig) error {
	f.createdConfig = config
	if f.configs == nil {
		f.configs = map[uuid.UUID]*model.DashboardConfig{}
	}
	f.configs[config.ID] = config
	return nil
}

func (f *fakeDashboardRepo) GetConfig(_ context.Context, id uuid.UUID) (*model.DashboardConfig, error) {
	return f.configs[id], nil
}

func (f *fakeDashboardRepo) ListByProject(_ context.Context, projectID uuid.UUID) ([]*model.DashboardConfig, error) {
	result := make([]*model.DashboardConfig, 0)
	for _, config := range f.configs {
		if config.ProjectID == projectID {
			result = append(result, config)
		}
	}
	return result, nil
}

func (f *fakeDashboardRepo) UpdateConfig(_ context.Context, config *model.DashboardConfig) error {
	f.updatedConfig = config
	f.configs[config.ID] = config
	return nil
}

func (f *fakeDashboardRepo) DeleteConfig(_ context.Context, id uuid.UUID) error {
	f.deletedConfig = id
	return nil
}

func (f *fakeDashboardRepo) SaveWidget(_ context.Context, widget *model.DashboardWidget) error {
	f.savedWidget = widget
	f.widgetsByDash[widget.DashboardID] = append(f.widgetsByDash[widget.DashboardID], widget)
	return nil
}

func (f *fakeDashboardRepo) DeleteWidget(_ context.Context, id uuid.UUID) error {
	f.deletedWidget = id
	return nil
}

func (f *fakeDashboardRepo) ListWidgetsByDashboard(_ context.Context, dashboardID uuid.UUID) ([]*model.DashboardWidget, error) {
	return append([]*model.DashboardWidget(nil), f.widgetsByDash[dashboardID]...), nil
}

func TestDashboardServiceCreateUpdateAndDeleteConfig(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeDashboardRepo{
		configs:       map[uuid.UUID]*model.DashboardConfig{},
		widgetsByDash: map[uuid.UUID][]*model.DashboardWidget{},
	}
	service := NewDashboardService(repo)

	config := &model.DashboardConfig{
		ProjectID: projectID,
		Name:      "Sprint Overview",
		Layout:    `[]`,
		CreatedBy: uuid.New(),
	}
	if err := service.CreateDashboard(context.Background(), config); err != nil {
		t.Fatalf("CreateDashboard() error = %v", err)
	}
	if config.ID == uuid.Nil {
		t.Fatal("expected CreateDashboard to assign an ID")
	}

	config.Name = "Updated Overview"
	if err := service.UpdateDashboard(context.Background(), config); err != nil {
		t.Fatalf("UpdateDashboard() error = %v", err)
	}
	if repo.updatedConfig == nil || repo.updatedConfig.Name != "Updated Overview" {
		t.Fatalf("unexpected updated config: %+v", repo.updatedConfig)
	}
	if err := service.DeleteDashboard(context.Background(), config.ID); err != nil {
		t.Fatalf("DeleteDashboard() error = %v", err)
	}
	if repo.deletedConfig != config.ID {
		t.Fatalf("repo.deletedConfig = %s, want %s", repo.deletedConfig, config.ID)
	}
}

func TestDashboardServiceSaveAndListWidgets(t *testing.T) {
	dashboardID := uuid.New()
	repo := &fakeDashboardRepo{
		configs:       map[uuid.UUID]*model.DashboardConfig{},
		widgetsByDash: map[uuid.UUID][]*model.DashboardWidget{},
	}
	service := NewDashboardService(repo)

	widget := &model.DashboardWidget{
		DashboardID: dashboardID,
		WidgetType:  model.DashboardWidgetThroughputChart,
		Config:      `{"range":"30d"}`,
		Position:    `{"x":0,"y":0,"w":6,"h":4}`,
	}
	if err := service.SaveWidget(context.Background(), widget); err != nil {
		t.Fatalf("SaveWidget() error = %v", err)
	}
	if widget.ID == uuid.Nil {
		t.Fatal("expected SaveWidget to assign an ID")
	}

	widgets, err := service.ListWidgets(context.Background(), dashboardID)
	if err != nil {
		t.Fatalf("ListWidgets() error = %v", err)
	}
	if len(widgets) != 1 {
		t.Fatalf("len(widgets) = %d, want 1", len(widgets))
	}
	if err := service.DeleteWidget(context.Background(), widget.ID); err != nil {
		t.Fatalf("DeleteWidget() error = %v", err)
	}
	if repo.deletedWidget != widget.ID {
		t.Fatalf("repo.deletedWidget = %s, want %s", repo.deletedWidget, widget.ID)
	}
}
