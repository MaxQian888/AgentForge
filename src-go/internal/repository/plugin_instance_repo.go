package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
)

type PluginInstanceRepository struct {
	db        DBTX
	mu        sync.RWMutex
	snapshots map[string]*model.PluginInstanceSnapshot
}

func NewPluginInstanceRepository(db ...DBTX) *PluginInstanceRepository {
	var conn DBTX
	if len(db) > 0 {
		conn = db[0]
	}
	return &PluginInstanceRepository{
		db:        conn,
		snapshots: make(map[string]*model.PluginInstanceSnapshot),
	}
}

func (r *PluginInstanceRepository) UpsertCurrent(ctx context.Context, snapshot *model.PluginInstanceSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("plugin instance snapshot is required")
	}
	if snapshot.PluginID == "" {
		return fmt.Errorf("plugin instance snapshot plugin_id is required")
	}
	now := time.Now().UTC()
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = now
	}
	snapshot.UpdatedAt = now

	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.snapshots[snapshot.PluginID] = clonePluginInstanceSnapshot(snapshot)
		return nil
	}

	runtimeMetadata, err := json.Marshal(snapshot.RuntimeMetadata)
	if err != nil {
		return fmt.Errorf("marshal plugin instance runtime metadata: %w", err)
	}

	query := `
		INSERT INTO plugin_instances (
			plugin_id, project_id, runtime_host, lifecycle_state, resolved_source_path,
			runtime_metadata, restart_count, last_health_at, last_error, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (plugin_id) DO UPDATE SET
			project_id = EXCLUDED.project_id,
			runtime_host = EXCLUDED.runtime_host,
			lifecycle_state = EXCLUDED.lifecycle_state,
			resolved_source_path = EXCLUDED.resolved_source_path,
			runtime_metadata = EXCLUDED.runtime_metadata,
			restart_count = EXCLUDED.restart_count,
			last_health_at = EXCLUDED.last_health_at,
			last_error = EXCLUDED.last_error,
			updated_at = EXCLUDED.updated_at
	`

	_, err = r.db.Exec(
		ctx,
		query,
		snapshot.PluginID,
		nullablePluginInstanceString(snapshot.ProjectID),
		snapshot.RuntimeHost,
		snapshot.LifecycleState,
		nullablePluginInstanceString(snapshot.ResolvedSourcePath),
		runtimeMetadata,
		snapshot.RestartCount,
		snapshot.LastHealthAt,
		nullablePluginInstanceString(snapshot.LastError),
		snapshot.CreatedAt,
		snapshot.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert plugin instance snapshot: %w", err)
	}
	return nil
}

func (r *PluginInstanceRepository) GetCurrentByPluginID(ctx context.Context, pluginID string) (*model.PluginInstanceSnapshot, error) {
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		snapshot, ok := r.snapshots[pluginID]
		if !ok {
			return nil, ErrNotFound
		}
		return clonePluginInstanceSnapshot(snapshot), nil
	}

	query := `
		SELECT plugin_id, COALESCE(project_id, ''), runtime_host, lifecycle_state, COALESCE(resolved_source_path, ''),
			runtime_metadata, restart_count, last_health_at, COALESCE(last_error, ''), created_at, updated_at
		FROM plugin_instances
		WHERE plugin_id = $1
	`
	var (
		snapshot        model.PluginInstanceSnapshot
		runtimeMetadata []byte
	)
	err := r.db.QueryRow(ctx, query, pluginID).Scan(
		&snapshot.PluginID,
		&snapshot.ProjectID,
		&snapshot.RuntimeHost,
		&snapshot.LifecycleState,
		&snapshot.ResolvedSourcePath,
		&runtimeMetadata,
		&snapshot.RestartCount,
		&snapshot.LastHealthAt,
		&snapshot.LastError,
		&snapshot.CreatedAt,
		&snapshot.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get plugin instance snapshot: %w", err)
	}
	if len(runtimeMetadata) > 0 && string(runtimeMetadata) != "null" {
		snapshot.RuntimeMetadata = &model.PluginRuntimeMetadata{}
		if err := json.Unmarshal(runtimeMetadata, snapshot.RuntimeMetadata); err != nil {
			return nil, fmt.Errorf("decode plugin instance runtime metadata: %w", err)
		}
	}
	return &snapshot, nil
}

func (r *PluginInstanceRepository) DeleteByPluginID(ctx context.Context, pluginID string) error {
	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		if _, ok := r.snapshots[pluginID]; !ok {
			return ErrNotFound
		}
		delete(r.snapshots, pluginID)
		return nil
	}

	result, err := r.db.Exec(ctx, `DELETE FROM plugin_instances WHERE plugin_id = $1`, pluginID)
	if err != nil {
		return fmt.Errorf("delete plugin instance snapshot: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func clonePluginInstanceSnapshot(snapshot *model.PluginInstanceSnapshot) *model.PluginInstanceSnapshot {
	if snapshot == nil {
		return nil
	}
	cloned := *snapshot
	if snapshot.LastHealthAt != nil {
		ts := *snapshot.LastHealthAt
		cloned.LastHealthAt = &ts
	}
	if snapshot.RuntimeMetadata != nil {
		metadata := *snapshot.RuntimeMetadata
		cloned.RuntimeMetadata = &metadata
	}
	return &cloned
}

func nullablePluginInstanceString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
