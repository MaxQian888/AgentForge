package repository

import (
	"context"
	"fmt"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReviewAggregationRepository struct {
	db *gorm.DB
}

func NewReviewAggregationRepository(db *gorm.DB) *ReviewAggregationRepository {
	return &ReviewAggregationRepository{db: db}
}

func (r *ReviewAggregationRepository) Create(ctx context.Context, agg *model.ReviewAggregation) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newReviewAggregationRecord(agg)).Error; err != nil {
		return fmt.Errorf("create review aggregation: %w", err)
	}
	return nil
}

func (r *ReviewAggregationRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.ReviewAggregation, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record reviewAggregationRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get review aggregation: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *ReviewAggregationRepository) GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.ReviewAggregation, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []reviewAggregationRecord
	if err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list review aggregations by task: %w", err)
	}

	results := make([]*model.ReviewAggregation, 0, len(records))
	for i := range records {
		results = append(results, records[i].toModel())
	}
	return results, nil
}

func (r *ReviewAggregationRepository) Update(ctx context.Context, agg *model.ReviewAggregation) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).
		Model(&reviewAggregationRecord{}).
		Where("id = ?", agg.ID).
		Updates(map[string]any{
			"overall_risk":   agg.OverallRisk,
			"recommendation": agg.Recommendation,
			"findings":       newJSONText(agg.Findings, "[]"),
			"summary":        agg.Summary,
			"metrics":        newJSONText(agg.Metrics, "{}"),
			"human_decision": agg.HumanDecision,
			"human_reviewer": agg.HumanReviewer,
			"human_comment":  agg.HumanComment,
			"decided_at":     agg.DecidedAt,
			"total_cost_usd": agg.TotalCostUsd,
		}).
		Error; err != nil {
		return fmt.Errorf("update review aggregation: %w", err)
	}
	return nil
}
