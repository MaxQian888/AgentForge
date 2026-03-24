package repository

import (
	"context"
	"fmt"
	"sync"

	"github.com/react-go-quick-starter/server/internal/model"
)

type PluginRegistryRepository struct {
	mu      sync.RWMutex
	records map[string]*model.PluginRecord
}

func NewPluginRegistryRepository() *PluginRegistryRepository {
	return &PluginRegistryRepository{
		records: make(map[string]*model.PluginRecord),
	}
}

func (r *PluginRegistryRepository) Save(_ context.Context, record *model.PluginRecord) error {
	if record == nil {
		return fmt.Errorf("plugin record is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.records[record.Metadata.ID] = clonePluginRecord(record)
	return nil
}

func (r *PluginRegistryRepository) GetByID(_ context.Context, pluginID string) (*model.PluginRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, ok := r.records[pluginID]
	if !ok {
		return nil, ErrNotFound
	}

	return clonePluginRecord(record), nil
}

func (r *PluginRegistryRepository) List(_ context.Context, filter model.PluginFilter) ([]*model.PluginRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	records := make([]*model.PluginRecord, 0, len(r.records))
	for _, record := range r.records {
		if filter.Kind != "" && record.Kind != filter.Kind {
			continue
		}
		if filter.LifecycleState != "" && record.LifecycleState != filter.LifecycleState {
			continue
		}
		records = append(records, clonePluginRecord(record))
	}

	return records, nil
}

func clonePluginRecord(record *model.PluginRecord) *model.PluginRecord {
	if record == nil {
		return nil
	}
	cloned := *record
	if record.LastHealthAt != nil {
		ts := *record.LastHealthAt
		cloned.LastHealthAt = &ts
	}
	if record.Metadata.Tags != nil {
		cloned.Metadata.Tags = append([]string(nil), record.Metadata.Tags...)
	}
	if record.Spec.Args != nil {
		cloned.Spec.Args = append([]string(nil), record.Spec.Args...)
	}
	if record.Spec.Capabilities != nil {
		cloned.Spec.Capabilities = append([]string(nil), record.Spec.Capabilities...)
	}
	if record.RuntimeMetadata != nil {
		metadata := *record.RuntimeMetadata
		cloned.RuntimeMetadata = &metadata
	}
	if record.Permissions.Network != nil {
		network := *record.Permissions.Network
		network.Domains = append([]string(nil), network.Domains...)
		cloned.Permissions.Network = &network
	}
	if record.Permissions.Filesystem != nil {
		fs := *record.Permissions.Filesystem
		fs.AllowedPaths = append([]string(nil), fs.AllowedPaths...)
		cloned.Permissions.Filesystem = &fs
	}
	return &cloned
}
