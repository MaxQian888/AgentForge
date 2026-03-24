package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/react-go-quick-starter/server/internal/model"
)

type PluginRegistryRepository struct {
	db      DBTX
	mu      sync.RWMutex
	records map[string]*model.PluginRecord
}

func NewPluginRegistryRepository(db ...DBTX) *PluginRegistryRepository {
	var conn DBTX
	if len(db) > 0 {
		conn = db[0]
	}
	return &PluginRegistryRepository{
		db:      conn,
		records: make(map[string]*model.PluginRecord),
	}
}

func (r *PluginRegistryRepository) Save(ctx context.Context, record *model.PluginRecord) error {
	if record == nil {
		return fmt.Errorf("plugin record is required")
	}
	if r.db != nil {
		manifest, err := json.Marshal(record.PluginManifest)
		if err != nil {
			return fmt.Errorf("marshal plugin manifest: %w", err)
		}
		runtimeMetadata, err := json.Marshal(record.RuntimeMetadata)
		if err != nil {
			return fmt.Errorf("marshal plugin runtime metadata: %w", err)
		}
		query := `
			INSERT INTO plugins (
				plugin_id, kind, name, version, description, tags, manifest,
				source_type, source_path, runtime, lifecycle_state, runtime_host,
				last_health_at, last_error, restart_count, resolved_source_path,
				runtime_metadata, created_at, updated_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,NOW(),NOW())
			ON CONFLICT (plugin_id) DO UPDATE SET
				kind = EXCLUDED.kind,
				name = EXCLUDED.name,
				version = EXCLUDED.version,
				description = EXCLUDED.description,
				tags = EXCLUDED.tags,
				manifest = EXCLUDED.manifest,
				source_type = EXCLUDED.source_type,
				source_path = EXCLUDED.source_path,
				runtime = EXCLUDED.runtime,
				lifecycle_state = EXCLUDED.lifecycle_state,
				runtime_host = EXCLUDED.runtime_host,
				last_health_at = EXCLUDED.last_health_at,
				last_error = EXCLUDED.last_error,
				restart_count = EXCLUDED.restart_count,
				resolved_source_path = EXCLUDED.resolved_source_path,
				runtime_metadata = EXCLUDED.runtime_metadata,
				updated_at = NOW()
		`
		_, err = r.db.Exec(
			ctx,
			query,
			record.Metadata.ID,
			record.Kind,
			record.Metadata.Name,
			record.Metadata.Version,
			nullablePluginString(record.Metadata.Description),
			record.Metadata.Tags,
			manifest,
			record.Source.Type,
			nullablePluginString(record.Source.Path),
			record.Spec.Runtime,
			record.LifecycleState,
			record.RuntimeHost,
			record.LastHealthAt,
			nullablePluginString(record.LastError),
			record.RestartCount,
			nullablePluginString(record.ResolvedSourcePath),
			runtimeMetadata,
		)
		if err != nil {
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
		query := `
			SELECT manifest, lifecycle_state, runtime_host, last_health_at, COALESCE(last_error, ''),
				restart_count, COALESCE(resolved_source_path, ''), runtime_metadata
			FROM plugins
			WHERE plugin_id = $1
		`
		var (
			manifest        []byte
			record          model.PluginRecord
			runtimeMetadata []byte
		)
		if err := r.db.QueryRow(ctx, query, pluginID).Scan(
			&manifest,
			&record.LifecycleState,
			&record.RuntimeHost,
			&record.LastHealthAt,
			&record.LastError,
			&record.RestartCount,
			&record.ResolvedSourcePath,
			&runtimeMetadata,
		); err != nil {
			return nil, ErrNotFound
		}
		if err := json.Unmarshal(manifest, &record.PluginManifest); err != nil {
			return nil, fmt.Errorf("decode plugin manifest: %w", err)
		}
		if len(runtimeMetadata) > 0 && string(runtimeMetadata) != "null" {
			record.RuntimeMetadata = &model.PluginRuntimeMetadata{}
			if err := json.Unmarshal(runtimeMetadata, record.RuntimeMetadata); err != nil {
				return nil, fmt.Errorf("decode plugin runtime metadata: %w", err)
			}
		}
		return &record, nil
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
		query := `
			SELECT manifest, lifecycle_state, runtime_host, last_health_at, COALESCE(last_error, ''),
				restart_count, COALESCE(resolved_source_path, ''), runtime_metadata
			FROM plugins
			WHERE ($1 = '' OR kind = $1) AND ($2 = '' OR lifecycle_state = $2)
			ORDER BY plugin_id
		`
		rows, err := r.db.Query(ctx, query, filter.Kind, filter.LifecycleState)
		if err != nil {
			return nil, fmt.Errorf("list plugin records: %w", err)
		}
		defer rows.Close()

		records := make([]*model.PluginRecord, 0)
		for rows.Next() {
			var (
				record          model.PluginRecord
				manifest        []byte
				runtimeMetadata []byte
			)
			if err := rows.Scan(
				&manifest,
				&record.LifecycleState,
				&record.RuntimeHost,
				&record.LastHealthAt,
				&record.LastError,
				&record.RestartCount,
				&record.ResolvedSourcePath,
				&runtimeMetadata,
			); err != nil {
				return nil, fmt.Errorf("scan plugin record: %w", err)
			}
			if err := json.Unmarshal(manifest, &record.PluginManifest); err != nil {
				return nil, fmt.Errorf("decode plugin manifest: %w", err)
			}
			if len(runtimeMetadata) > 0 && string(runtimeMetadata) != "null" {
				record.RuntimeMetadata = &model.PluginRuntimeMetadata{}
				if err := json.Unmarshal(runtimeMetadata, record.RuntimeMetadata); err != nil {
					return nil, fmt.Errorf("decode plugin runtime metadata: %w", err)
				}
			}
			if !matchesPluginFilter(&record, filter) {
				continue
			}
			records = append(records, &record)
		}
		return records, rows.Err()
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
		result, err := r.db.Exec(ctx, `DELETE FROM plugins WHERE plugin_id = $1`, pluginID)
		if err != nil {
			return fmt.Errorf("delete plugin record: %w", err)
		}
		if result.RowsAffected() == 0 {
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
		metadata := *record.RuntimeMetadata
		cloned.RuntimeMetadata = &metadata
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
