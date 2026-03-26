package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type WikiSpaceRepository struct {
	db *gorm.DB
}

func NewWikiSpaceRepository(db *gorm.DB) *WikiSpaceRepository {
	return &WikiSpaceRepository{db: db}
}

func (r *WikiSpaceRepository) Create(ctx context.Context, space *model.WikiSpace) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newWikiSpaceRecord(space)).Error; err != nil {
		return fmt.Errorf("create wiki space: %w", err)
	}
	return nil
}

func (r *WikiSpaceRepository) GetByProjectID(ctx context.Context, projectID uuid.UUID) (*model.WikiSpace, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record wikiSpaceRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND deleted_at IS NULL", projectID).
		Take(&record).
		Error; err != nil {
		return nil, normalizeRepositoryError(err)
	}
	return record.toModel(), nil
}

func (r *WikiSpaceRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.WikiSpace, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record wikiSpaceRecord
	if err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		Take(&record).
		Error; err != nil {
		return nil, normalizeRepositoryError(err)
	}
	return record.toModel(), nil
}

func (r *WikiSpaceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	result := r.db.WithContext(ctx).
		Model(&wikiSpaceRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": time.Now().UTC()})
	if result.Error != nil {
		return fmt.Errorf("delete wiki space: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
