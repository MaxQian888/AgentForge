package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type MilestoneRepository struct {
	db *gorm.DB
}

func NewMilestoneRepository(db *gorm.DB) *MilestoneRepository {
	return &MilestoneRepository{db: db}
}

func (r *MilestoneRepository) Create(ctx context.Context, milestone *model.Milestone) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	record := newMilestoneRecord(milestone)
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create milestone: %w", err)
	}
	return nil
}

func (r *MilestoneRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Milestone, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record milestoneRecord
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get milestone: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *MilestoneRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Milestone, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []milestoneRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND deleted_at IS NULL", projectID).
		Order("target_date ASC, created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list milestones: %w", err)
	}
	result := make([]*model.Milestone, 0, len(records))
	for i := range records {
		result = append(result, records[i].toModel())
	}
	return result, nil
}

func (r *MilestoneRepository) Update(ctx context.Context, milestone *model.Milestone) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	result := r.db.WithContext(ctx).
		Model(&milestoneRecord{}).
		Where("id = ? AND deleted_at IS NULL", milestone.ID).
		Updates(map[string]any{
			"name":        milestone.Name,
			"target_date": cloneTimePointer(milestone.TargetDate),
			"status":      milestone.Status,
			"description": milestone.Description,
			"updated_at":  time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("update milestone: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MilestoneRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&milestoneRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now})
	if result.Error != nil {
		return fmt.Errorf("delete milestone: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MilestoneRepository) GetWithMetrics(ctx context.Context, id uuid.UUID) (*model.Milestone, model.MilestoneMetrics, error) {
	milestone, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, model.MilestoneMetrics{}, err
	}

	var totalTasks int64
	if err := r.db.WithContext(ctx).Model(&taskRecord{}).Where("milestone_id = ?", id).Count(&totalTasks).Error; err != nil {
		return nil, model.MilestoneMetrics{}, fmt.Errorf("count milestone tasks: %w", err)
	}
	var completedTasks int64
	if err := r.db.WithContext(ctx).Model(&taskRecord{}).Where("milestone_id = ? AND status = ?", id, model.TaskStatusDone).Count(&completedTasks).Error; err != nil {
		return nil, model.MilestoneMetrics{}, fmt.Errorf("count completed milestone tasks: %w", err)
	}
	var totalSprints int64
	if err := r.db.WithContext(ctx).Model(&sprintRecord{}).Where("milestone_id = ?", id).Count(&totalSprints).Error; err != nil {
		return nil, model.MilestoneMetrics{}, fmt.Errorf("count milestone sprints: %w", err)
	}

	return milestone, model.BuildMilestoneMetrics(int(totalTasks), int(completedTasks), int(totalSprints)), nil
}
