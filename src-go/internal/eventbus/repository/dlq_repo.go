package repository

import (
	"context"
	"encoding/json"
	"time"

	eb "github.com/react-go-quick-starter/server/internal/eventbus"
	"gorm.io/gorm"
)

type DeadLetterRepository struct {
	db *gorm.DB
}

func NewDeadLetterRepository(db *gorm.DB) *DeadLetterRepository {
	return &DeadLetterRepository{db: db}
}

type dlqRow struct {
	ID          uint      `gorm:"column:id;primaryKey;autoIncrement"`
	EventID     string    `gorm:"column:event_id"`
	Envelope    []byte    `gorm:"column:envelope;type:jsonb"`
	LastError   string    `gorm:"column:last_error"`
	RetryCount  int       `gorm:"column:retry_count"`
	FirstSeenAt time.Time `gorm:"column:first_seen_at"`
	LastSeenAt  time.Time `gorm:"column:last_seen_at"`
}

func (dlqRow) TableName() string { return "events_dead_letter" }

func (r *DeadLetterRepository) Record(ctx context.Context, e *eb.Event, err error, retries int) error {
	env, jerr := json.Marshal(e)
	if jerr != nil {
		return jerr
	}
	now := time.Now()
	row := dlqRow{
		EventID:     e.ID,
		Envelope:    env,
		LastError:   err.Error(),
		RetryCount:  retries,
		FirstSeenAt: now,
		LastSeenAt:  now,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}
