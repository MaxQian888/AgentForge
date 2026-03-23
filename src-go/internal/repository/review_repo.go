package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type ReviewRepository struct {
	db DBTX
}

func NewReviewRepository(db DBTX) *ReviewRepository {
	return &ReviewRepository{db: db}
}

const reviewColumns = `id, task_id, reviewer_id, reviewer_type, status, recommendation, summary, comments, created_at, updated_at`

func (r *ReviewRepository) Create(ctx context.Context, review *model.Review) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `
		INSERT INTO reviews (id, task_id, reviewer_id, reviewer_type, status, recommendation, summary, comments, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	`
	_, err := r.db.Exec(ctx, query, review.ID, review.TaskID, review.ReviewerID,
		review.ReviewerType, review.Status, review.Recommendation, review.Summary, review.Comments)
	if err != nil {
		return fmt.Errorf("create review: %w", err)
	}
	return nil
}

func (r *ReviewRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + reviewColumns + ` FROM reviews WHERE id = $1`
	rev := &model.Review{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&rev.ID, &rev.TaskID, &rev.ReviewerID, &rev.ReviewerType,
		&rev.Status, &rev.Recommendation, &rev.Summary, &rev.Comments,
		&rev.CreatedAt, &rev.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get review by id: %w", err)
	}
	return rev, nil
}

func (r *ReviewRepository) GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.Review, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + reviewColumns + ` FROM reviews WHERE task_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("get reviews by task: %w", err)
	}
	defer rows.Close()

	var reviews []*model.Review
	for rows.Next() {
		rev := &model.Review{}
		if err := rows.Scan(
			&rev.ID, &rev.TaskID, &rev.ReviewerID, &rev.ReviewerType,
			&rev.Status, &rev.Recommendation, &rev.Summary, &rev.Comments,
			&rev.CreatedAt, &rev.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		reviews = append(reviews, rev)
	}
	return reviews, rows.Err()
}

func (r *ReviewRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, recommendation string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE reviews SET status = $1, recommendation = $2, updated_at = NOW() WHERE id = $3`
	_, err := r.db.Exec(ctx, query, status, recommendation, id)
	if err != nil {
		return fmt.Errorf("update review status: %w", err)
	}
	return nil
}
