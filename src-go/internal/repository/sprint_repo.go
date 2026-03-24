package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type SprintRepository struct {
	db *gorm.DB
}

func NewSprintRepository(db *gorm.DB) *SprintRepository {
	return &SprintRepository{db: db}
}

func (r *SprintRepository) Create(ctx context.Context, sprint *model.Sprint) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newSprintRecord(sprint)).Error; err != nil {
		return fmt.Errorf("create sprint: %w", err)
	}
	return nil
}

func (r *SprintRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Sprint, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record sprintRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get sprint by id: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *SprintRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Sprint, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []sprintRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("start_date DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list sprints: %w", err)
	}

	sprints := make([]*model.Sprint, 0, len(records))
	for i := range records {
		sprints = append(sprints, records[i].toModel())
	}
	return sprints, nil
}

func (r *SprintRepository) Update(ctx context.Context, sprint *model.Sprint) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	result := r.db.WithContext(ctx).
		Model(&sprintRecord{}).
		Where("id = ?", sprint.ID).
		Updates(map[string]any{
			"name":             sprint.Name,
			"start_date":       sprint.StartDate,
			"end_date":         sprint.EndDate,
			"status":           sprint.Status,
			"total_budget_usd": sprint.TotalBudgetUsd,
			"spent_usd":        sprint.SpentUsd,
		})
	if result.Error != nil {
		return fmt.Errorf("update sprint: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("update sprint: %w", ErrNotFound)
	}
	return nil
}

func (r *SprintRepository) GetActive(ctx context.Context, projectID uuid.UUID) (*model.Sprint, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record sprintRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND status = ?", projectID, "active").
		Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get active sprint: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}
