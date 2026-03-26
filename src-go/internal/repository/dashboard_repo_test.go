package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestNewDashboardRepository(t *testing.T) {
	repo := NewDashboardRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil DashboardRepository")
	}
}

func TestDashboardRepositoryRoundTripConfigsAndWidgets(t *testing.T) {
	ctx := context.Background()
	repo := NewDashboardRepository(openFoundationRepoTestDB(t, &dashboardConfigRecord{}, &dashboardWidgetRecord{}))

	projectID := uuid.New()
	userID := uuid.New()
	now := time.Date(2026, 3, 26, 17, 0, 0, 0, time.UTC)

	config := &model.DashboardConfig{
		ID:        uuid.New(),
		ProjectID: projectID,
		Name:      "Sprint Overview",
		Layout:    `[{"i":"throughput","x":0,"y":0,"w":6,"h":4}]`,
		CreatedBy: userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateConfig(ctx, config); err != nil {
		t.Fatalf("CreateConfig() error = %v", err)
	}

	widget := &model.DashboardWidget{
		ID:          uuid.New(),
		DashboardID: config.ID,
		WidgetType:  model.DashboardWidgetThroughputChart,
		Config:      `{"range":"30d"}`,
		Position:    `{"x":0,"y":0,"w":6,"h":4}`,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := repo.SaveWidget(ctx, widget); err != nil {
		t.Fatalf("SaveWidget() error = %v", err)
	}

	configs, err := repo.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("len(configs) = %d, want 1", len(configs))
	}

	widgets, err := repo.ListWidgetsByDashboard(ctx, config.ID)
	if err != nil {
		t.Fatalf("ListWidgetsByDashboard() error = %v", err)
	}
	if len(widgets) != 1 {
		t.Fatalf("len(widgets) = %d, want 1", len(widgets))
	}
	if widgets[0].WidgetType != model.DashboardWidgetThroughputChart {
		t.Fatalf("widgets[0].WidgetType = %q", widgets[0].WidgetType)
	}
}
