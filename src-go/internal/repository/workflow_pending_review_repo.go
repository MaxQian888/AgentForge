package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type workflowPendingReviewRecord struct {
	ID          uuid.UUID  `gorm:"column:id;primaryKey"`
	ExecutionID uuid.UUID  `gorm:"column:execution_id"`
	NodeID      string     `gorm:"column:node_id"`
	ProjectID   uuid.UUID  `gorm:"column:project_id"`
	ReviewerID  *uuid.UUID `gorm:"column:reviewer_id"`
	Prompt      string     `gorm:"column:prompt"`
	Context     rawJSON    `gorm:"column:context;type:jsonb"`
	Decision    string     `gorm:"column:decision"`
	Comment     string     `gorm:"column:comment"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	ResolvedAt  *time.Time `gorm:"column:resolved_at"`
}

func (workflowPendingReviewRecord) TableName() string { return "workflow_pending_reviews" }

func (r *workflowPendingReviewRecord) toModel() *model.WorkflowPendingReview {
	if r == nil {
		return nil
	}
	return &model.WorkflowPendingReview{
		ID:          r.ID,
		ExecutionID: r.ExecutionID,
		NodeID:      r.NodeID,
		ProjectID:   r.ProjectID,
		ReviewerID:  r.ReviewerID,
		Prompt:      r.Prompt,
		Context:     r.Context.Bytes("{}"),
		Decision:    r.Decision,
		Comment:     r.Comment,
		CreatedAt:   r.CreatedAt,
		ResolvedAt:  r.ResolvedAt,
	}
}

type WorkflowPendingReviewRepository struct {
	db *gorm.DB
}

func NewWorkflowPendingReviewRepository(db *gorm.DB) *WorkflowPendingReviewRepository {
	return &WorkflowPendingReviewRepository{db: db}
}

func (r *WorkflowPendingReviewRepository) Create(ctx context.Context, review *model.WorkflowPendingReview) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	record := &workflowPendingReviewRecord{
		ID:          review.ID,
		ExecutionID: review.ExecutionID,
		NodeID:      review.NodeID,
		ProjectID:   review.ProjectID,
		ReviewerID:  review.ReviewerID,
		Prompt:      review.Prompt,
		Context:     newRawJSON(review.Context, "{}"),
		Decision:    review.Decision,
		Comment:     review.Comment,
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create workflow pending review: %w", err)
	}
	return nil
}

func (r *WorkflowPendingReviewRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowPendingReview, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record workflowPendingReviewRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get workflow pending review: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *WorkflowPendingReviewRepository) ListPendingByProject(ctx context.Context, projectID uuid.UUID) ([]*model.WorkflowPendingReview, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []workflowPendingReviewRecord
	if err := r.db.WithContext(ctx).Where("project_id = ? AND decision = ?", projectID, model.ReviewDecisionPending).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list pending reviews: %w", err)
	}
	result := make([]*model.WorkflowPendingReview, len(records))
	for i := range records {
		result[i] = records[i].toModel()
	}
	return result, nil
}

func (r *WorkflowPendingReviewRepository) Resolve(ctx context.Context, id uuid.UUID, decision, comment string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"decision":    decision,
		"comment":     comment,
		"resolved_at": now,
	}
	result := r.db.WithContext(ctx).Model(&workflowPendingReviewRecord{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("resolve workflow pending review: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *WorkflowPendingReviewRepository) FindPendingByExecutionAndNode(ctx context.Context, executionID uuid.UUID, nodeID string) (*model.WorkflowPendingReview, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record workflowPendingReviewRecord
	if err := r.db.WithContext(ctx).Where("execution_id = ? AND node_id = ? AND decision = ?", executionID, nodeID, model.ReviewDecisionPending).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("find pending review: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}
