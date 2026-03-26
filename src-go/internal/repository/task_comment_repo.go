package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type TaskCommentRepository struct {
	db *gorm.DB
}

func NewTaskCommentRepository(db *gorm.DB) *TaskCommentRepository {
	return &TaskCommentRepository{db: db}
}

func (r *TaskCommentRepository) Create(ctx context.Context, comment *model.TaskComment) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	if comment.ID == uuid.Nil {
		comment.ID = uuid.New()
	}
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = now
	}
	if comment.UpdatedAt.IsZero() {
		comment.UpdatedAt = comment.CreatedAt
	}
	if err := r.db.WithContext(ctx).Create(newTaskCommentRecord(comment)).Error; err != nil {
		return fmt.Errorf("create task comment: %w", err)
	}
	return nil
}

func (r *TaskCommentRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.TaskComment, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record taskCommentRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get task comment by id: %w", normalizeRepositoryError(err))
	}
	comment, err := record.toModel()
	if err != nil {
		return nil, fmt.Errorf("scan task comment: %w", err)
	}
	return comment, nil
}

func (r *TaskCommentRepository) Update(ctx context.Context, comment *model.TaskComment) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	comment.UpdatedAt = time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&taskCommentRecord{}).
		Where("id = ?", comment.ID).
		Updates(map[string]any{
			"body":        comment.Body,
			"mentions":    newJSONText(mustMarshalStringSlice(comment.Mentions), "[]"),
			"resolved_at": cloneTimePointer(comment.ResolvedAt),
			"updated_at":  comment.UpdatedAt,
		}).
		Error; err != nil {
		return fmt.Errorf("update task comment: %w", err)
	}
	return nil
}

func (r *TaskCommentRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&taskCommentRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"deleted_at": now,
			"updated_at": now,
		}).
		Error; err != nil {
		return fmt.Errorf("soft delete task comment: %w", err)
	}
	return nil
}

func (r *TaskCommentRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]*model.TaskComment, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []taskCommentRecord
	if err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list task comments by task id: %w", err)
	}
	comments := make([]*model.TaskComment, 0, len(records))
	for i := range records {
		comment, err := records[i].toModel()
		if err != nil {
			return nil, fmt.Errorf("scan task comment: %w", err)
		}
		comments = append(comments, comment)
	}
	return comments, nil
}
