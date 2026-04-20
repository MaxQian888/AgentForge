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

type CustomFieldRepository struct {
	db *gorm.DB
}

func NewCustomFieldRepository(db *gorm.DB) *CustomFieldRepository {
	return &CustomFieldRepository{db: db}
}

func (r *CustomFieldRepository) CreateDefinition(ctx context.Context, definition *model.CustomFieldDefinition) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	record := newCustomFieldDefinitionRecord(definition)
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create custom field definition: %w", err)
	}
	return nil
}

func (r *CustomFieldRepository) GetDefinition(ctx context.Context, id uuid.UUID) (*model.CustomFieldDefinition, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record customFieldDefinitionRecord
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get custom field definition: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *CustomFieldRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.CustomFieldDefinition, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []customFieldDefinitionRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND deleted_at IS NULL", projectID).
		Order("sort_order ASC, created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list custom field definitions: %w", err)
	}
	result := make([]*model.CustomFieldDefinition, 0, len(records))
	for i := range records {
		result = append(result, records[i].toModel())
	}
	return result, nil
}

func (r *CustomFieldRepository) UpdateDefinition(ctx context.Context, definition *model.CustomFieldDefinition) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	result := r.db.WithContext(ctx).
		Model(&customFieldDefinitionRecord{}).
		Where("id = ? AND deleted_at IS NULL", definition.ID).
		Updates(map[string]any{
			"name":       definition.Name,
			"field_type": definition.FieldType,
			"options":    newJSONText(definition.Options, "[]"),
			"sort_order": definition.SortOrder,
			"required":   definition.Required,
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("update custom field definition: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *CustomFieldRepository) DeleteDefinition(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&customFieldDefinitionRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now})
	if result.Error != nil {
		return fmt.Errorf("delete custom field definition: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *CustomFieldRepository) SetValue(ctx context.Context, value *model.CustomFieldValue) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	record := newCustomFieldValueRecord(value)
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "task_id"}, {Name: "field_def_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).
		Create(record).Error; err != nil {
		return fmt.Errorf("set custom field value: %w", err)
	}
	return nil
}

func (r *CustomFieldRepository) ClearValue(ctx context.Context, taskID uuid.UUID, fieldDefID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Delete(&customFieldValueRecord{}, "task_id = ? AND field_def_id = ?", taskID, fieldDefID).Error; err != nil {
		return fmt.Errorf("clear custom field value: %w", err)
	}
	return nil
}

func (r *CustomFieldRepository) ListValuesByTask(ctx context.Context, taskID uuid.UUID) ([]*model.CustomFieldValue, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []customFieldValueRecord
	if err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list custom field values: %w", err)
	}
	result := make([]*model.CustomFieldValue, 0, len(records))
	for i := range records {
		result = append(result, records[i].toModel())
	}
	return result, nil
}
