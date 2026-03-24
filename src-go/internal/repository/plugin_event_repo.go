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
)

type PluginEventRepository struct {
	db     DBTX
	mu     sync.RWMutex
	events []*model.PluginEventRecord
}

func NewPluginEventRepository(db ...DBTX) *PluginEventRepository {
	var conn DBTX
	if len(db) > 0 {
		conn = db[0]
	}
	return &PluginEventRepository{db: conn, events: make([]*model.PluginEventRecord, 0)}
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

	query := `
		INSERT INTO plugin_events (
			id, plugin_id, event_type, event_source, lifecycle_state, summary, payload, created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`
	_, err = r.db.Exec(
		ctx,
		query,
		event.ID,
			event.PluginID,
			event.EventType,
			event.EventSource,
			nullablePluginLifecycleState(event.LifecycleState),
			nullablePluginString(event.Summary),
			payload,
			event.CreatedAt,
		)
	if err != nil {
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

	query := `
		SELECT id, plugin_id, event_type, event_source, COALESCE(lifecycle_state, ''), COALESCE(summary, ''),
			payload, created_at
		FROM plugin_events
		WHERE plugin_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, pluginID, limit)
	if err != nil {
		return nil, fmt.Errorf("list plugin events: %w", err)
	}
	defer rows.Close()

	events := make([]*model.PluginEventRecord, 0, limit)
	for rows.Next() {
		var (
			event   model.PluginEventRecord
			payload []byte
		)
		if err := rows.Scan(
			&event.ID,
			&event.PluginID,
			&event.EventType,
			&event.EventSource,
			&event.LifecycleState,
			&event.Summary,
			&payload,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan plugin event: %w", err)
		}
		if len(payload) > 0 && string(payload) != "null" {
			event.Payload = map[string]any{}
			if err := json.Unmarshal(payload, &event.Payload); err != nil {
				return nil, fmt.Errorf("decode plugin event payload: %w", err)
			}
		}
		events = append(events, &event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
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
