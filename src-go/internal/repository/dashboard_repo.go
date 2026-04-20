package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DashboardRepository struct {
	db *gorm.DB
}

func NewDashboardRepository(db *gorm.DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

func (r *DashboardRepository) CreateConfig(ctx context.Context, config *model.DashboardConfig) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	record := newDashboardConfigRecord(config)
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create dashboard config: %w", err)
	}
	return nil
}

func (r *DashboardRepository) GetConfig(ctx context.Context, id uuid.UUID) (*model.DashboardConfig, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record dashboardConfigRecord
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get dashboard config: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *DashboardRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.DashboardConfig, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []dashboardConfigRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND deleted_at IS NULL", projectID).
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list dashboards by project: %w", err)
	}
	result := make([]*model.DashboardConfig, 0, len(records))
	for i := range records {
		result = append(result, records[i].toModel())
	}
	return result, nil
}

func (r *DashboardRepository) UpdateConfig(ctx context.Context, config *model.DashboardConfig) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	result := r.db.WithContext(ctx).
		Model(&dashboardConfigRecord{}).
		Where("id = ? AND deleted_at IS NULL", config.ID).
		Updates(map[string]any{
			"name":       config.Name,
			"layout":     newJSONText(config.Layout, "[]"),
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("update dashboard config: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *DashboardRepository) DeleteConfig(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&dashboardConfigRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now})
	if result.Error != nil {
		return fmt.Errorf("delete dashboard config: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *DashboardRepository) SaveWidget(ctx context.Context, widget *model.DashboardWidget) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	record := newDashboardWidgetRecord(widget)
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"widget_type", "config", "position", "updated_at"}),
		}).
		Create(record).Error; err != nil {
		return fmt.Errorf("save dashboard widget: %w", err)
	}
	return nil
}

func (r *DashboardRepository) DeleteWidget(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Delete(&dashboardWidgetRecord{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete dashboard widget: %w", err)
	}
	return nil
}

func (r *DashboardRepository) ListWidgetsByDashboard(ctx context.Context, dashboardID uuid.UUID) ([]*model.DashboardWidget, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []dashboardWidgetRecord
	if err := r.db.WithContext(ctx).
		Where("dashboard_id = ?", dashboardID).
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list dashboard widgets: %w", err)
	}
	result := make([]*model.DashboardWidget, 0, len(records))
	for i := range records {
		result = append(result, records[i].toModel())
	}
	return result, nil
}
