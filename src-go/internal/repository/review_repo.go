package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type ReviewRepository struct {
	db *gorm.DB
}

func NewReviewRepository(db *gorm.DB) *ReviewRepository {
	return &ReviewRepository{db: db}
}

func (r *ReviewRepository) Create(ctx context.Context, review *model.Review) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	rec, err := newReviewRecord(review)
	if err != nil {
		return fmt.Errorf("create review: %w", err)
	}

	if err := r.db.WithContext(ctx).Create(rec).Error; err != nil {
		return fmt.Errorf("create review: %w", err)
	}
	return nil
}

func (r *ReviewRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record reviewRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get review by id: %w", normalizeRepositoryError(err))
	}

	review, err := record.toModel()
	if err != nil {
		return nil, fmt.Errorf("get review by id: %w", err)
	}
	return review, nil
}

func (r *ReviewRepository) GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.Review, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []reviewRecord
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("get reviews by task: %w", err)
	}

	reviews := make([]*model.Review, 0, len(records))
	for i := range records {
		review, err := records[i].toModel()
		if err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		reviews = append(reviews, review)
	}
	return reviews, nil
}

func (r *ReviewRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	if err := r.db.WithContext(ctx).Model(&reviewRecord{}).Where("id = ?", id).Updates(map[string]any{
		"status":     status,
		"updated_at": gorm.Expr("NOW()"),
	}).Error; err != nil {
		return fmt.Errorf("update review status: %w", err)
	}
	return nil
}

func (r *ReviewRepository) UpdateResult(ctx context.Context, review *model.Review) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	rec, err := newReviewRecord(review)
	if err != nil {
		return fmt.Errorf("update review result: %w", err)
	}

	if err := r.db.WithContext(ctx).Model(&reviewRecord{}).Where("id = ?", review.ID).Updates(map[string]any{
		"status":             rec.Status,
		"risk_level":         rec.RiskLevel,
		"findings":           rec.Findings,
		"execution_metadata": rec.ExecutionMetadata,
		"summary":            rec.Summary,
		"recommendation":     rec.Recommendation,
		"cost_usd":           rec.CostUSD,
		"updated_at":         gorm.Expr("NOW()"),
	}).Error; err != nil {
		return fmt.Errorf("update review result: %w", err)
	}
	return nil
}
