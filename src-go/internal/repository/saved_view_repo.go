package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SavedViewRepository struct {
	db *gorm.DB
}

func NewSavedViewRepository(db *gorm.DB) *SavedViewRepository {
	return &SavedViewRepository{db: db}
}

func (r *SavedViewRepository) Create(ctx context.Context, view *model.SavedView) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	record := newSavedViewRecord(view)
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create saved view: %w", err)
	}
	return nil
}

func (r *SavedViewRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.SavedView, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record savedViewRecord
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get saved view: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *SavedViewRepository) ListByProject(ctx context.Context, projectID uuid.UUID, userID uuid.UUID, roles []string) ([]*model.SavedView, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []savedViewRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND deleted_at IS NULL", projectID).
		Order("is_default DESC, created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list saved views: %w", err)
	}
	result := make([]*model.SavedView, 0, len(records))
	for i := range records {
		view := records[i].toModel()
		if view.IsAccessibleTo(userID, roles) {
			result = append(result, view)
		}
	}
	return result, nil
}

func (r *SavedViewRepository) Update(ctx context.Context, view *model.SavedView) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	result := r.db.WithContext(ctx).
		Model(&savedViewRecord{}).
		Where("id = ? AND deleted_at IS NULL", view.ID).
		Updates(map[string]any{
			"name":        view.Name,
			"owner_id":    cloneUUIDPointer(view.OwnerID),
			"is_default":  view.IsDefault,
			"shared_with": newJSONText(view.SharedWith, "{}"),
			"config":      newJSONText(view.Config, "{}"),
			"updated_at":  time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("update saved view: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SavedViewRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&savedViewRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now, "is_default": false})
	if result.Error != nil {
		return fmt.Errorf("delete saved view: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SavedViewRepository) SetDefault(ctx context.Context, projectID uuid.UUID, viewID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&savedViewRecord{}).
			Where("project_id = ? AND deleted_at IS NULL", projectID).
			Updates(map[string]any{"is_default": false, "updated_at": time.Now().UTC()}).Error; err != nil {
			return fmt.Errorf("clear saved view defaults: %w", err)
		}

		result := tx.Model(&savedViewRecord{}).
			Where("id = ? AND project_id = ? AND deleted_at IS NULL", viewID, projectID).
			Updates(map[string]any{"is_default": true, "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return fmt.Errorf("set saved view default: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}
