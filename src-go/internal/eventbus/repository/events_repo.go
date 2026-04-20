package repository

import (
	"context"
	"encoding/json"
	"fmt"

	eb "github.com/agentforge/server/internal/eventbus"
	"gorm.io/gorm"
)

type EventsRepository struct {
	db *gorm.DB
}

func NewEventsRepository(db *gorm.DB) *EventsRepository {
	return &EventsRepository{db: db}
}

// eventRow maps to the events table.
type eventRow struct {
	ID         string  `gorm:"column:id;primaryKey"`
	Type       string  `gorm:"column:type"`
	Source     string  `gorm:"column:source"`
	Target     string  `gorm:"column:target"`
	Visibility string  `gorm:"column:visibility"`
	Payload    []byte  `gorm:"column:payload;type:jsonb"`
	Metadata   []byte  `gorm:"column:metadata;type:jsonb"`
	ProjectID  *string `gorm:"column:project_id"`
	OccurredAt int64   `gorm:"column:occurred_at"`
}

func (eventRow) TableName() string { return "events" }

func (r *EventsRepository) Insert(ctx context.Context, e *eb.Event) error {
	meta, err := json.Marshal(e.Metadata)
	if err != nil {
		return fmt.Errorf("encode metadata: %w", err)
	}
	row := eventRow{
		ID:         e.ID,
		Type:       e.Type,
		Source:     e.Source,
		Target:     e.Target,
		Visibility: string(e.Visibility),
		Payload:    []byte(e.Payload),
		Metadata:   meta,
		OccurredAt: e.Timestamp,
	}
	if pid := eb.GetString(e, eb.MetaProjectID); pid != "" {
		row.ProjectID = &pid
	}
	result := r.db.WithContext(ctx).Create(&row)
	if result.Error != nil {
		// Ignore duplicate key (ON CONFLICT DO NOTHING equivalent)
		return result.Error
	}
	return nil
}

func (r *EventsRepository) FindByID(ctx context.Context, id string) (*eb.Event, error) {
	var rr eventRow
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&rr).Error; err != nil {
		return nil, err
	}
	e := &eb.Event{
		ID:         rr.ID,
		Type:       rr.Type,
		Source:     rr.Source,
		Target:     rr.Target,
		Visibility: eb.Visibility(rr.Visibility),
		Payload:    json.RawMessage(rr.Payload),
		Timestamp:  rr.OccurredAt,
	}
	if len(rr.Metadata) > 0 {
		_ = json.Unmarshal(rr.Metadata, &e.Metadata)
	}
	return e, nil
}
