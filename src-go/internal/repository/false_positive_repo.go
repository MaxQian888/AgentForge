package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type FalsePositiveRepository struct {
	db *gorm.DB
}

func NewFalsePositiveRepository(db *gorm.DB) *FalsePositiveRepository {
	return &FalsePositiveRepository{db: db}
}

func (r *FalsePositiveRepository) Create(ctx context.Context, fp *model.FalsePositive) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newFalsePositiveRecord(fp)).Error; err != nil {
		return fmt.Errorf("create false positive: %w", err)
	}
	return nil
}

func (r *FalsePositiveRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.FalsePositive, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []falsePositiveRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("occurrences DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list false positives: %w", err)
	}

	results := make([]*model.FalsePositive, 0, len(records))
	for i := range records {
		results = append(results, records[i].toModel())
	}
	return results, nil
}

func (r *FalsePositiveRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Delete(&falsePositiveRecord{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete false positive: %w", err)
	}
	return nil
}

func (r *FalsePositiveRepository) IncrementOccurrences(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).
		Model(&falsePositiveRecord{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"occurrences": gorm.Expr("occurrences + 1"),
			"is_strong":   gorm.Expr("(occurrences + 1) >= 3"),
		}).
		Error; err != nil {
		return fmt.Errorf("increment false positive occurrences: %w", err)
	}
	return nil
}
