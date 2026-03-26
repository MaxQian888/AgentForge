package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type PageCommentRepository struct {
	db *gorm.DB
}

func NewPageCommentRepository(db *gorm.DB) *PageCommentRepository {
	return &PageCommentRepository{db: db}
}

func (r *PageCommentRepository) Create(ctx context.Context, comment *model.PageComment) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newPageCommentRecord(comment)).Error; err != nil {
		return fmt.Errorf("create page comment: %w", err)
	}
	return nil
}

func (r *PageCommentRepository) ListByPageID(ctx context.Context, pageID uuid.UUID) ([]*model.PageComment, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []pageCommentRecord
	if err := r.db.WithContext(ctx).
		Where("page_id = ? AND deleted_at IS NULL", pageID).
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list page comments: %w", err)
	}

	comments := make([]*model.PageComment, 0, len(records))
	for i := range records {
		comments = append(comments, records[i].toModel())
	}
	return comments, nil
}

func (r *PageCommentRepository) Update(ctx context.Context, comment *model.PageComment) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	result := r.db.WithContext(ctx).
		Model(&pageCommentRecord{}).
		Where("id = ? AND deleted_at IS NULL", comment.ID).
		Updates(map[string]any{
			"body":        comment.Body,
			"mentions":    newJSONText(comment.Mentions, "[]"),
			"resolved_at": comment.ResolvedAt,
			"updated_at":  comment.UpdatedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update page comment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PageCommentRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&pageCommentRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now})
	if result.Error != nil {
		return fmt.Errorf("soft delete page comment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
