package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create(ctx context.Context, n *model.Notification) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newNotificationRecord(n)).Error; err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

func (r *NotificationRepository) ListByTarget(ctx context.Context, targetID uuid.UUID, unreadOnly bool, limit int) ([]*model.Notification, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	if limit <= 0 {
		limit = 50
	}

	query := r.db.WithContext(ctx).Where("target_id = ?", targetID)
	if unreadOnly {
		query = query.Where("is_read = ?", false)
	}

	var records []notificationRecord
	if err := applyPagination(query.Order("created_at DESC"), limit, 0).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	notifications := make([]*model.Notification, 0, len(records))
	for i := range records {
		notifications = append(notifications, records[i].toModel())
	}
	return notifications, nil
}

func (r *NotificationRepository) MarkRead(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).
		Model(&notificationRecord{}).
		Where("id = ?", id).
		Update("is_read", true).
		Error; err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	return nil
}

func (r *NotificationRepository) MarkSent(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).
		Model(&notificationRecord{}).
		Where("id = ?", id).
		Update("sent", true).
		Error; err != nil {
		return fmt.Errorf("mark notification sent: %w", err)
	}
	return nil
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, targetID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).
		Model(&notificationRecord{}).
		Where("target_id = ? AND is_read = ?", targetID, false).
		Update("is_read", true).
		Error; err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}
