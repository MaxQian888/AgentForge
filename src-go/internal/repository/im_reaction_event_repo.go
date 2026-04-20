package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IMReactionEventRepository struct {
	db *gorm.DB
}

func NewIMReactionEventRepository(db *gorm.DB) *IMReactionEventRepository {
	return &IMReactionEventRepository{db: db}
}

type imReactionEventRecord struct {
	ID         uuid.UUID `gorm:"column:id;primaryKey"`
	Platform   string    `gorm:"column:platform"`
	ChatID     string    `gorm:"column:chat_id"`
	MessageID  string    `gorm:"column:message_id"`
	UserID     string    `gorm:"column:user_id"`
	Emoji      string    `gorm:"column:emoji"`
	EventType  string    `gorm:"column:event_type"`
	RawPayload []byte    `gorm:"column:raw_payload;type:jsonb"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (imReactionEventRecord) TableName() string { return "im_reaction_events" }

func (r *IMReactionEventRepository) Record(ctx context.Context, event *model.IMReactionEvent) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if event == nil {
		return fmt.Errorf("im_reaction_event is required")
	}
	if event.EventType != model.IMReactionEventTypeCreated && event.EventType != model.IMReactionEventTypeDeleted {
		return fmt.Errorf("invalid event_type %q", event.EventType)
	}
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	raw := event.RawPayload
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	record := imReactionEventRecord{
		ID:         event.ID,
		Platform:   event.Platform,
		ChatID:     event.ChatID,
		MessageID:  event.MessageID,
		UserID:     event.UserID,
		Emoji:      event.Emoji,
		EventType:  event.EventType,
		RawPayload: raw,
		CreatedAt:  event.CreatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("record im reaction event: %w", err)
	}
	return nil
}

func (r *IMReactionEventRepository) ListByMessage(ctx context.Context, messageID string, limit int) ([]model.IMReactionEvent, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 100
	}
	var records []imReactionEventRecord
	if err := r.db.WithContext(ctx).
		Where("message_id = ?", messageID).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list im reaction events by message: %w", err)
	}
	out := make([]model.IMReactionEvent, 0, len(records))
	for _, rec := range records {
		out = append(out, model.IMReactionEvent{
			ID:         rec.ID,
			Platform:   rec.Platform,
			ChatID:     rec.ChatID,
			MessageID:  rec.MessageID,
			UserID:     rec.UserID,
			Emoji:      rec.Emoji,
			EventType:  rec.EventType,
			RawPayload: rec.RawPayload,
			CreatedAt:  rec.CreatedAt,
		})
	}
	return out, nil
}
