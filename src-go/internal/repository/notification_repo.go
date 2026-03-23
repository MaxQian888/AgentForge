package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type NotificationRepository struct {
	db DBTX
}

func NewNotificationRepository(db DBTX) *NotificationRepository {
	return &NotificationRepository{db: db}
}

const notificationColumns = `id, target_id, type, title, body, data, is_read, created_at`

func (r *NotificationRepository) Create(ctx context.Context, n *model.Notification) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `
		INSERT INTO notifications (id, target_id, type, title, body, data, is_read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`
	_, err := r.db.Exec(ctx, query, n.ID, n.TargetID, n.Type, n.Title, n.Body, n.Data, n.IsRead)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

func (r *NotificationRepository) ListByTarget(ctx context.Context, targetID uuid.UUID, unreadOnly bool, limit int) ([]*model.Notification, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	query := `SELECT ` + notificationColumns + ` FROM notifications WHERE target_id = $1`
	if unreadOnly {
		query += ` AND is_read = false`
	}
	if limit <= 0 {
		limit = 50
	}
	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT %d`, limit)

	rows, err := r.db.Query(ctx, query, targetID)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*model.Notification
	for rows.Next() {
		n := &model.Notification{}
		if err := rows.Scan(&n.ID, &n.TargetID, &n.Type, &n.Title, &n.Body, &n.Data, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		notifications = append(notifications, n)
	}
	return notifications, rows.Err()
}

func (r *NotificationRepository) MarkRead(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	_, err := r.db.Exec(ctx, `UPDATE notifications SET is_read = true WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	return nil
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, targetID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	_, err := r.db.Exec(ctx, `UPDATE notifications SET is_read = true WHERE target_id = $1 AND is_read = false`, targetID)
	if err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}
