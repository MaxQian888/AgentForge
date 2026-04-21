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

// ListByTraceID returns all events whose metadata JSONB has "trace_id" equal to traceID.
// Ordered by occurred_at ASC. Returns at most limit rows (defaults to 10000 when <= 0).
func (r *EventsRepository) ListByTraceID(ctx context.Context, traceID string, limit int) ([]*eb.Event, error) {
	if limit <= 0 {
		limit = 10000
	}
	var rows []eventRow
	if err := r.db.WithContext(ctx).
		Where("metadata->>'trace_id' = ?", traceID).
		Order("occurred_at ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list events by trace: %w", err)
	}
	out := make([]*eb.Event, len(rows))
	for i := range rows {
		e := &eb.Event{
			ID:         rows[i].ID,
			Type:       rows[i].Type,
			Source:     rows[i].Source,
			Target:     rows[i].Target,
			Visibility: eb.Visibility(rows[i].Visibility),
			Payload:    json.RawMessage(rows[i].Payload),
			Timestamp:  rows[i].OccurredAt,
		}
		if len(rows[i].Metadata) > 0 {
			_ = json.Unmarshal(rows[i].Metadata, &e.Metadata)
		}
		out[i] = e
	}
	return out, nil
}
