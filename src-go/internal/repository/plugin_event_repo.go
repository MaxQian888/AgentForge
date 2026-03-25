package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type PluginEventRepository struct {
	db     *gorm.DB
	mu     sync.RWMutex
	events []*model.PluginEventRecord
}

func NewPluginEventRepository(db ...*gorm.DB) *PluginEventRepository {
	var conn *gorm.DB
	if len(db) > 0 {
		conn = db[0]
	}
	return &PluginEventRepository{db: conn, events: make([]*model.PluginEventRecord, 0)}
}

func (r *PluginEventRepository) WithDB(db *gorm.DB) PluginEventDBBinder {
	if r == nil {
		return NewPluginEventRepository(db)
	}
	rebound := NewPluginEventRepository(db)
	if db == nil {
		rebound.events = r.events
	}
	return rebound
}

func (r *PluginEventRepository) Append(ctx context.Context, event *model.PluginEventRecord) error {
	if event == nil {
		return fmt.Errorf("plugin event is required")
	}
	if event.PluginID == "" {
		return fmt.Errorf("plugin event plugin_id is required")
	}
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.events = append(r.events, clonePluginEventRecord(event))
		return nil
	}

	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal plugin event payload: %w", err)
	}

	row := &pluginEventRecordModel{
		ID:             event.ID,
		PluginID:       event.PluginID,
		EventType:      string(event.EventType),
		EventSource:    string(event.EventSource),
		LifecycleState: optionalPluginLifecycleState(event.LifecycleState),
		Summary:        optionalPluginSummary(event.Summary),
		Payload:        newRawJSON(payload, "{}"),
		CreatedAt:      event.CreatedAt,
	}

	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("append plugin event: %w", err)
	}
	return nil
}

func (r *PluginEventRepository) ListByPluginID(ctx context.Context, pluginID string, limit int) ([]*model.PluginEventRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		filtered := make([]*model.PluginEventRecord, 0, limit)
		for i := len(r.events) - 1; i >= 0; i-- {
			event := r.events[i]
			if event.PluginID != pluginID {
				continue
			}
			filtered = append(filtered, clonePluginEventRecord(event))
			if len(filtered) == limit {
				break
			}
		}
		return filtered, nil
	}

	var rows []pluginEventRecordModel
	if err := r.db.WithContext(ctx).
		Where("plugin_id = ?", pluginID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list plugin events: %w", err)
	}

	events := make([]*model.PluginEventRecord, 0, len(rows))
	for _, row := range rows {
		event, err := row.toEventRecord()
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func clonePluginEventRecord(event *model.PluginEventRecord) *model.PluginEventRecord {
	if event == nil {
		return nil
	}
	cloned := *event
	if event.Payload != nil {
		cloned.Payload = mapsClone(event.Payload)
	}
	return &cloned
}

func mapsClone(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		if nested, ok := value.(map[string]any); ok {
			dst[key] = mapsClone(nested)
			continue
		}
		if arr, ok := value.([]string); ok {
			dst[key] = slices.Clone(arr)
			continue
		}
		dst[key] = value
	}
	return dst
}

func optionalPluginLifecycleState(value model.PluginLifecycleState) *string {
	if value == "" {
		return nil
	}
	stringValue := string(value)
	return &stringValue
}

func optionalPluginSummary(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (r *pluginEventRecordModel) toEventRecord() (*model.PluginEventRecord, error) {
	if r == nil {
		return nil, nil
	}

	event := &model.PluginEventRecord{
		ID:             r.ID,
		PluginID:       r.PluginID,
		EventType:      model.PluginEventType(r.EventType),
		EventSource:    model.PluginEventSource(r.EventSource),
		LifecycleState: model.PluginLifecycleState(valueOrEmpty(r.LifecycleState)),
		Summary:        valueOrEmpty(r.Summary),
		CreatedAt:      r.CreatedAt,
	}

	if payload := r.Payload.Bytes("{}"); len(payload) > 0 && string(payload) != "null" {
		event.Payload = map[string]any{}
		if err := json.Unmarshal(payload, &event.Payload); err != nil {
			return nil, fmt.Errorf("decode plugin event payload: %w", err)
		}
	}
	return event, nil
}
