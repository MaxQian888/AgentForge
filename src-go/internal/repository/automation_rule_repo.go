package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AutomationRuleRepository struct {
	db *gorm.DB
}

func NewAutomationRuleRepository(db *gorm.DB) *AutomationRuleRepository {
	return &AutomationRuleRepository{db: db}
}

func (r *AutomationRuleRepository) Create(ctx context.Context, rule *model.AutomationRule) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	record := newAutomationRuleRecord(rule)
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create automation rule: %w", err)
	}
	return nil
}

func (r *AutomationRuleRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AutomationRule, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record automationRuleRecord
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get automation rule: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *AutomationRuleRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.AutomationRule, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []automationRuleRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND deleted_at IS NULL", projectID).
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list automation rules: %w", err)
	}
	result := make([]*model.AutomationRule, 0, len(records))
	for i := range records {
		result = append(result, records[i].toModel())
	}
	return result, nil
}

func (r *AutomationRuleRepository) ListByProjectAndEvent(ctx context.Context, projectID uuid.UUID, eventType string) ([]*model.AutomationRule, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := r.db.WithContext(ctx).Where("project_id = ? AND deleted_at IS NULL", projectID)
	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	var records []automationRuleRecord
	if err := query.Order("created_at ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list automation rules by event: %w", err)
	}
	result := make([]*model.AutomationRule, 0, len(records))
	for i := range records {
		result = append(result, records[i].toModel())
	}
	return result, nil
}

func (r *AutomationRuleRepository) Update(ctx context.Context, rule *model.AutomationRule) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	result := r.db.WithContext(ctx).
		Model(&automationRuleRecord{}).
		Where("id = ? AND deleted_at IS NULL", rule.ID).
		Updates(map[string]any{
			"name":       rule.Name,
			"enabled":    rule.Enabled,
			"event_type": rule.EventType,
			"conditions": newJSONText(rule.Conditions, "[]"),
			"actions":    newJSONText(rule.Actions, "[]"),
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("update automation rule: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AutomationRuleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&automationRuleRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now, "enabled": false})
	if result.Error != nil {
		return fmt.Errorf("delete automation rule: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
