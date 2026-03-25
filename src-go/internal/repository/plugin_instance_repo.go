package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PluginInstanceRepository struct {
	db        *gorm.DB
	mu        sync.RWMutex
	snapshots map[string]*model.PluginInstanceSnapshot
}

func NewPluginInstanceRepository(db ...*gorm.DB) *PluginInstanceRepository {
	var conn *gorm.DB
	if len(db) > 0 {
		conn = db[0]
	}
	return &PluginInstanceRepository{
		db:        conn,
		snapshots: make(map[string]*model.PluginInstanceSnapshot),
	}
}

func (r *PluginInstanceRepository) WithDB(db *gorm.DB) PluginInstanceDBBinder {
	if r == nil {
		return NewPluginInstanceRepository(db)
	}
	rebound := NewPluginInstanceRepository(db)
	if db == nil {
		rebound.snapshots = r.snapshots
	}
	return rebound
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

	row := &pluginInstanceRecordModel{
		PluginID:           snapshot.PluginID,
		ProjectID:          optionalPluginInstanceString(snapshot.ProjectID),
		RuntimeHost:        string(snapshot.RuntimeHost),
		LifecycleState:     string(snapshot.LifecycleState),
		ResolvedSourcePath: optionalPluginInstanceString(snapshot.ResolvedSourcePath),
		RuntimeMetadata:    newRawJSON(runtimeMetadata, "null"),
		RestartCount:       snapshot.RestartCount,
		LastHealthAt:       cloneTimePointer(snapshot.LastHealthAt),
		LastError:          optionalPluginInstanceString(snapshot.LastError),
		CreatedAt:          snapshot.CreatedAt,
		UpdatedAt:          snapshot.UpdatedAt,
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "plugin_id"}},
			UpdateAll: true,
		}).
		Create(row).Error; err != nil {
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

	var row pluginInstanceRecordModel
	if err := r.db.WithContext(ctx).Where("plugin_id = ?", pluginID).Take(&row).Error; err != nil {
		return nil, fmt.Errorf("get plugin instance snapshot: %w", normalizeRepositoryError(err))
	}
	return row.toSnapshot()
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

	result := r.db.WithContext(ctx).Delete(&pluginInstanceRecordModel{}, "plugin_id = ?", pluginID)
	if result.Error != nil {
		return fmt.Errorf("delete plugin instance snapshot: %w", result.Error)
	}
	if result.RowsAffected == 0 {
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
		cloned.RuntimeMetadata = clonePluginRuntimeMetadata(snapshot.RuntimeMetadata)
	}
	return &cloned
}

func nullablePluginInstanceString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func optionalPluginInstanceString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (r *pluginInstanceRecordModel) toSnapshot() (*model.PluginInstanceSnapshot, error) {
	if r == nil {
		return nil, nil
	}

	snapshot := &model.PluginInstanceSnapshot{
		PluginID:           r.PluginID,
		ProjectID:          valueOrEmpty(r.ProjectID),
		RuntimeHost:        model.PluginRuntimeHost(r.RuntimeHost),
		LifecycleState:     model.PluginLifecycleState(r.LifecycleState),
		ResolvedSourcePath: valueOrEmpty(r.ResolvedSourcePath),
		RestartCount:       r.RestartCount,
		LastHealthAt:       cloneTimePointer(r.LastHealthAt),
		LastError:          valueOrEmpty(r.LastError),
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}

	if payload := r.RuntimeMetadata.Bytes("null"); len(payload) > 0 && string(payload) != "null" {
		snapshot.RuntimeMetadata = &model.PluginRuntimeMetadata{}
		if err := json.Unmarshal(payload, snapshot.RuntimeMetadata); err != nil {
			return nil, fmt.Errorf("decode plugin instance runtime metadata: %w", err)
		}
	}
	return snapshot, nil
}
