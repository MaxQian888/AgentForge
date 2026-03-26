package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	DashboardWidgetThroughputChart   = "throughput_chart"
	DashboardWidgetBurndown          = "burndown"
	DashboardWidgetBlockerCount      = "blocker_count"
	DashboardWidgetBudgetConsumption = "budget_consumption"
	DashboardWidgetAgentCost         = "agent_cost"
	DashboardWidgetReviewBacklog     = "review_backlog"
	DashboardWidgetTaskAging         = "task_aging"
	DashboardWidgetSLACompliance     = "sla_compliance"
)

type DashboardConfig struct {
	ID        uuid.UUID  `db:"id"`
	ProjectID uuid.UUID  `db:"project_id"`
	Name      string     `db:"name"`
	Layout    string     `db:"layout"`
	CreatedBy uuid.UUID  `db:"created_by"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

type DashboardWidget struct {
	ID          uuid.UUID `db:"id"`
	DashboardID uuid.UUID `db:"dashboard_id"`
	WidgetType  string    `db:"widget_type"`
	Config      string    `db:"config"`
	Position    string    `db:"position"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type DashboardConfigDTO struct {
	ID        string               `json:"id"`
	ProjectID string               `json:"projectId"`
	Name      string               `json:"name"`
	Layout    json.RawMessage      `json:"layout"`
	CreatedBy string               `json:"createdBy"`
	CreatedAt string               `json:"createdAt"`
	UpdatedAt string               `json:"updatedAt"`
	DeletedAt *string              `json:"deletedAt,omitempty"`
	Widgets   []DashboardWidgetDTO `json:"widgets,omitempty"`
}

type DashboardWidgetDTO struct {
	ID          string          `json:"id"`
	DashboardID string          `json:"dashboardId"`
	WidgetType  string          `json:"widgetType"`
	Config      json.RawMessage `json:"config"`
	Position    json.RawMessage `json:"position"`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
}

type CreateDashboardRequest struct {
	Name   string          `json:"name" validate:"required,min=1,max=120"`
	Layout json.RawMessage `json:"layout"`
}

type UpdateDashboardRequest struct {
	Name   *string         `json:"name"`
	Layout json.RawMessage `json:"layout"`
}

type SaveDashboardWidgetRequest struct {
	ID         *string         `json:"id"`
	WidgetType string          `json:"widgetType" validate:"required"`
	Config     json.RawMessage `json:"config"`
	Position   json.RawMessage `json:"position"`
}

func (d *DashboardConfig) ToDTO(widgets []DashboardWidgetDTO) DashboardConfigDTO {
	dto := DashboardConfigDTO{
		ID:        d.ID.String(),
		ProjectID: d.ProjectID.String(),
		Name:      d.Name,
		Layout:    normalizeJSONRawMessage(d.Layout, []byte("[]")),
		CreatedBy: d.CreatedBy.String(),
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
		UpdatedAt: d.UpdatedAt.Format(time.RFC3339),
		Widgets:   widgets,
	}
	if d.DeletedAt != nil {
		value := d.DeletedAt.Format(time.RFC3339)
		dto.DeletedAt = &value
	}
	return dto
}

func (w *DashboardWidget) ToDTO() DashboardWidgetDTO {
	return DashboardWidgetDTO{
		ID:          w.ID.String(),
		DashboardID: w.DashboardID.String(),
		WidgetType:  w.WidgetType,
		Config:      normalizeJSONRawMessage(w.Config, []byte("{}")),
		Position:    normalizeJSONRawMessage(w.Position, []byte("{}")),
		CreatedAt:   w.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   w.UpdatedAt.Format(time.RFC3339),
	}
}
