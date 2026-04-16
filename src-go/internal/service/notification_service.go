package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	eventbus "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
)

// NotificationRepository defines persistence for notifications.
type NotificationRepository interface {
	Create(ctx context.Context, n *model.Notification) error
	ListByTarget(ctx context.Context, targetID uuid.UUID, unreadOnly bool, limit int) ([]*model.Notification, error)
	MarkRead(ctx context.Context, id uuid.UUID) error
	MarkAllRead(ctx context.Context, targetID uuid.UUID) error
}

type NotificationService struct {
	repo NotificationRepository
	hub  *ws.Hub
	bus  eventbus.Publisher
}

func NewNotificationService(repo NotificationRepository, hub *ws.Hub, bus eventbus.Publisher) *NotificationService {
	return &NotificationService{repo: repo, hub: hub, bus: bus}
}

// Create stores a notification and broadcasts it via WebSocket.
func (s *NotificationService) Create(ctx context.Context, targetID uuid.UUID, ntype, title, body, data string) (*model.Notification, error) {
	n := &model.Notification{
		ID:       uuid.New(),
		TargetID: targetID,
		Type:     ntype,
		Title:    title,
		Body:     body,
		Data:     data,
		IsRead:   false,
	}
	if err := s.repo.Create(ctx, n); err != nil {
		return nil, fmt.Errorf("create notification: %w", err)
	}

	_ = eventbus.PublishLegacy(ctx, s.bus, ws.EventNotification, "", n.ToDTO())
	return n, nil
}

// List returns notifications for a target.
func (s *NotificationService) List(ctx context.Context, targetID uuid.UUID, unreadOnly bool, limit int) ([]*model.Notification, error) {
	return s.repo.ListByTarget(ctx, targetID, unreadOnly, limit)
}

// MarkRead marks a single notification as read.
func (s *NotificationService) MarkRead(ctx context.Context, id uuid.UUID) error {
	return s.repo.MarkRead(ctx, id)
}

// MarkAllRead marks all notifications as read for a target.
func (s *NotificationService) MarkAllRead(ctx context.Context, targetID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, targetID)
}
