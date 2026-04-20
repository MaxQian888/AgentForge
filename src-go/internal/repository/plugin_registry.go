package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/agentforge/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PluginRegistryRepository struct {
	db      *gorm.DB
	mu      sync.RWMutex
	records map[string]*model.PluginRecord
}

func NewPluginRegistryRepository(db ...*gorm.DB) *PluginRegistryRepository {
	var conn *gorm.DB
	if len(db) > 0 {
		conn = db[0]
	}
	return &PluginRegistryRepository{
		db:      conn,
		records: make(map[string]*model.PluginRecord),
	}
}

func (r *PluginRegistryRepository) DB() *gorm.DB {
	if r == nil {
		return nil
	}
	return r.db
}

func (r *PluginRegistryRepository) WithDB(db *gorm.DB) PluginRegistryDBBinder {
	if r == nil {
		return NewPluginRegistryRepository(db)
	}
	rebound := NewPluginRegistryRepository(db)
	if db == nil {
		rebound.records = r.records
	}
	return rebound
}

func (r *PluginRegistryRepository) Save(ctx context.Context, record *model.PluginRecord) error {
	if record == nil {
		return fmt.Errorf("plugin record is required")
	}
	if r.db != nil {
		row, err := newPluginRecordModel(record)
		if err != nil {
			return err
		}
		if err := r.db.WithContext(ctx).
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "plugin_id"}},
				UpdateAll: true,
			}).
			Create(row).Error; err != nil {
			return fmt.Errorf("save plugin record: %w", err)
		}
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.records[record.Metadata.ID] = clonePluginRecord(record)
	return nil
}

func (r *PluginRegistryRepository) GetByID(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	if r.db != nil {
		var row pluginRecordModel
		if err := r.db.WithContext(ctx).Where("plugin_id = ?", pluginID).Take(&row).Error; err != nil {
			return nil, normalizeRepositoryError(err)
		}
		return row.toPluginRecord()
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	record, ok := r.records[pluginID]
	if !ok {
		return nil, ErrNotFound
	}

	return clonePluginRecord(record), nil
}

func (r *PluginRegistryRepository) List(ctx context.Context, filter model.PluginFilter) ([]*model.PluginRecord, error) {
	if r.db != nil {
		query := r.db.WithContext(ctx).Order("plugin_id ASC")
		if filter.Kind != "" {
			query = query.Where("kind = ?", string(filter.Kind))
		}
		if filter.LifecycleState != "" {
			query = query.Where("lifecycle_state = ?", string(filter.LifecycleState))
		}

		var rows []pluginRecordModel
		if err := query.Find(&rows).Error; err != nil {
			return nil, fmt.Errorf("list plugin records: %w", err)
		}

		records := make([]*model.PluginRecord, 0, len(rows))
		for _, row := range rows {
			record, err := row.toPluginRecord()
			if err != nil {
				return nil, err
			}
			if !matchesPluginFilter(record, filter) {
				continue
			}
			records = append(records, record)
		}
		return records, nil
	}

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
		if !matchesPluginFilter(record, filter) {
			continue
		}
		records = append(records, clonePluginRecord(record))
	}

	return records, nil
}

func (r *PluginRegistryRepository) Delete(ctx context.Context, pluginID string) error {
	if r.db != nil {
		result := r.db.WithContext(ctx).Delete(&pluginRecordModel{}, "plugin_id = ?", pluginID)
		if result.Error != nil {
			return fmt.Errorf("delete plugin record: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.records[pluginID]; !ok {
		return ErrNotFound
	}
	delete(r.records, pluginID)
	return nil
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
	if record.Spec.Workflow != nil {
		workflow := *record.Spec.Workflow
		if record.Spec.Workflow.Roles != nil {
			workflow.Roles = append([]model.WorkflowRoleBinding(nil), record.Spec.Workflow.Roles...)
		}
		if record.Spec.Workflow.Steps != nil {
			workflow.Steps = cloneWorkflowSteps(record.Spec.Workflow.Steps)
		}
		if record.Spec.Workflow.Triggers != nil {
			workflow.Triggers = append([]model.PluginWorkflowTrigger(nil), record.Spec.Workflow.Triggers...)
		}
		if record.Spec.Workflow.Limits != nil {
			limits := *record.Spec.Workflow.Limits
			workflow.Limits = &limits
		}
		cloned.Spec.Workflow = &workflow
	}
	if record.Spec.Review != nil {
		review := *record.Spec.Review
		review.Triggers.Events = append([]string(nil), record.Spec.Review.Triggers.Events...)
		review.Triggers.FilePatterns = append([]string(nil), record.Spec.Review.Triggers.FilePatterns...)
		cloned.Spec.Review = &review
	}
	if record.RuntimeMetadata != nil {
		cloned.RuntimeMetadata = clonePluginRuntimeMetadata(record.RuntimeMetadata)
	}
	if record.CurrentInstance != nil {
		cloned.CurrentInstance = clonePluginInstanceSnapshot(record.CurrentInstance)
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
	if record.Source.Trust != nil {
		trust := *record.Source.Trust
		if record.Source.Trust.VerifiedAt != nil {
			verifiedAt := *record.Source.Trust.VerifiedAt
			trust.VerifiedAt = &verifiedAt
		}
		if record.Source.Trust.ApprovedAt != nil {
			approvedAt := *record.Source.Trust.ApprovedAt
			trust.ApprovedAt = &approvedAt
		}
		cloned.Source.Trust = &trust
	}
	if record.Source.Release != nil {
		release := *record.Source.Release
		if record.Source.Release.PublishedAt != nil {
			publishedAt := *record.Source.Release.PublishedAt
			release.PublishedAt = &publishedAt
		}
		cloned.Source.Release = &release
	}
	return &cloned
}

func clonePluginRuntimeMetadata(metadata *model.PluginRuntimeMetadata) *model.PluginRuntimeMetadata {
	if metadata == nil {
		return nil
	}
	cloned := *metadata
	if metadata.MCP != nil {
		mcp := *metadata.MCP
		mcp.LastDiscoveryAt = clonePluginTimePointer(metadata.MCP.LastDiscoveryAt)
		if metadata.MCP.LatestInteraction != nil {
			interaction := *metadata.MCP.LatestInteraction
			interaction.At = clonePluginTimePointer(metadata.MCP.LatestInteraction.At)
			mcp.LatestInteraction = &interaction
		}
		cloned.MCP = &mcp
	}
	return &cloned
}

func clonePluginTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneWorkflowSteps(steps []model.WorkflowStepDefinition) []model.WorkflowStepDefinition {
	if steps == nil {
		return nil
	}
	cloned := make([]model.WorkflowStepDefinition, len(steps))
	for i, step := range steps {
		cloned[i] = step
		if step.Next != nil {
			cloned[i].Next = append([]string(nil), step.Next...)
		}
	}
	return cloned
}

func matchesPluginFilter(record *model.PluginRecord, filter model.PluginFilter) bool {
	if record == nil {
		return false
	}
	if filter.SourceType != "" && record.Source.Type != filter.SourceType {
		return false
	}
	if filter.TrustState != "" && pluginTrustState(record) != filter.TrustState {
		return false
	}
	return true
}

func pluginTrustState(record *model.PluginRecord) model.PluginTrustState {
	if record == nil || record.Source.Trust == nil || record.Source.Trust.Status == "" {
		return model.PluginTrustUnknown
	}
	return record.Source.Trust.Status
}

func nullablePluginString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullablePluginLifecycleState(value model.PluginLifecycleState) any {
	if value == "" {
		return nil
	}
	return value
}

func newPluginRecordModel(record *model.PluginRecord) (*pluginRecordModel, error) {
	if record == nil {
		return nil, fmt.Errorf("plugin record is required")
	}

	manifest, err := json.Marshal(record.PluginManifest)
	if err != nil {
		return nil, fmt.Errorf("marshal plugin manifest: %w", err)
	}
	runtimeMetadata, err := json.Marshal(record.RuntimeMetadata)
	if err != nil {
		return nil, fmt.Errorf("marshal plugin runtime metadata: %w", err)
	}

	return &pluginRecordModel{
		PluginID:           record.Metadata.ID,
		Kind:               string(record.Kind),
		Name:               record.Metadata.Name,
		Version:            record.Metadata.Version,
		Description:        cloneStringPointer(optionalPluginString(record.Metadata.Description)),
		Tags:               newStringList(record.Metadata.Tags),
		Manifest:           newRawJSON(manifest, "{}"),
		SourceType:         string(record.Source.Type),
		SourcePath:         cloneStringPointer(optionalPluginString(record.Source.Path)),
		Runtime:            string(record.Spec.Runtime),
		LifecycleState:     string(record.LifecycleState),
		RuntimeHost:        string(record.RuntimeHost),
		LastHealthAt:       cloneTimePointer(record.LastHealthAt),
		LastError:          cloneStringPointer(optionalPluginString(record.LastError)),
		RestartCount:       record.RestartCount,
		ResolvedSourcePath: cloneStringPointer(optionalPluginString(record.ResolvedSourcePath)),
		RuntimeMetadata:    newRawJSON(runtimeMetadata, "null"),
	}, nil
}

func (r *pluginRecordModel) toPluginRecord() (*model.PluginRecord, error) {
	if r == nil {
		return nil, nil
	}

	record := &model.PluginRecord{
		LifecycleState:     model.PluginLifecycleState(r.LifecycleState),
		RuntimeHost:        model.PluginRuntimeHost(r.RuntimeHost),
		LastHealthAt:       cloneTimePointer(r.LastHealthAt),
		LastError:          valueOrEmpty(r.LastError),
		RestartCount:       r.RestartCount,
		ResolvedSourcePath: valueOrEmpty(r.ResolvedSourcePath),
	}

	if err := json.Unmarshal(r.Manifest.Bytes("{}"), &record.PluginManifest); err != nil {
		return nil, fmt.Errorf("decode plugin manifest: %w", err)
	}
	if payload := r.RuntimeMetadata.Bytes("null"); len(payload) > 0 && string(payload) != "null" {
		record.RuntimeMetadata = &model.PluginRuntimeMetadata{}
		if err := json.Unmarshal(payload, record.RuntimeMetadata); err != nil {
			return nil, fmt.Errorf("decode plugin runtime metadata: %w", err)
		}
	}
	return record, nil
}

func optionalPluginString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
