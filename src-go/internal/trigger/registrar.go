// Package trigger implements trigger-node materialization for workflow definitions.
package trigger

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/react-go-quick-starter/server/internal/model"
)

// TriggerRepository is the subset of *repository.WorkflowTriggerRepository
// that Registrar needs. Using an interface here keeps Registrar unit-testable
// without a real DB.
type TriggerRepository interface {
	Upsert(ctx context.Context, t *model.WorkflowTrigger) error
	ListByWorkflow(ctx context.Context, workflowID uuid.UUID) ([]*model.WorkflowTrigger, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// triggerNodeConfig is the typed shape of a trigger node's Config JSON.
type triggerNodeConfig struct {
	Source                 string         `json:"source"`
	IM                     map[string]any `json:"im,omitempty"`
	Schedule               map[string]any `json:"schedule,omitempty"`
	InputMapping           map[string]any `json:"input_mapping,omitempty"`
	IdempotencyKeyTemplate string         `json:"idempotency_key_template,omitempty"`
	DedupeWindowSeconds    int            `json:"dedupe_window_seconds,omitempty"`
	Enabled                *bool          `json:"enabled,omitempty"`
}

// Registrar materializes trigger-node subscriptions from a workflow
// definition into rows in workflow_triggers. It is called from the
// workflow-save path (Task 15) so every save reconciles the set of
// enabled triggers with the current DAG.
type Registrar struct {
	repo TriggerRepository
}

// NewRegistrar returns a new Registrar backed by repo.
func NewRegistrar(repo TriggerRepository) *Registrar {
	return &Registrar{repo: repo}
}

// SyncFromDefinition reconciles persisted workflow_triggers for workflowID
// against the trigger nodes in `nodes`. For each trigger node whose config
// declares source=im|schedule, a WorkflowTrigger row is upserted. Rows whose
// config no longer matches any current trigger node are deleted.
//
// `nodes` is the same slice that lives inside a WorkflowDefinition after
// `json.Unmarshal(def.Nodes, &nodes)`. The caller is responsible for that
// unmarshal; this function only accepts the typed slice.
//
// `createdBy` is stamped on any newly-inserted row; nil is acceptable for
// system-initiated syncs (e.g., template seed).
func (r *Registrar) SyncFromDefinition(
	ctx context.Context,
	workflowID, projectID uuid.UUID,
	nodes []model.WorkflowNode,
	createdBy *uuid.UUID,
) error {
	keepSet := make(map[uuid.UUID]struct{})

	for _, node := range nodes {
		if node.Type != model.NodeTypeTrigger {
			continue
		}

		// Skip nodes with empty config — bare manual triggers.
		if len(node.Config) == 0 {
			continue
		}

		var cfg triggerNodeConfig
		if err := json.Unmarshal(node.Config, &cfg); err != nil {
			// Unmarshal failure means misconfigured or non-subscription node; skip silently.
			continue
		}

		// Skip manual / empty source — normal.
		if cfg.Source == "" || cfg.Source == "manual" {
			continue
		}

		// Only "im" and "schedule" are supported in v1.
		if cfg.Source != string(model.TriggerSourceIM) && cfg.Source != string(model.TriggerSourceSchedule) {
			return fmt.Errorf("trigger node %q has unsupported source %q", node.ID, cfg.Source)
		}

		// Select the per-source sub-config.
		var subConfig map[string]any
		switch model.TriggerSource(cfg.Source) {
		case model.TriggerSourceIM:
			subConfig = cfg.IM
		case model.TriggerSourceSchedule:
			subConfig = cfg.Schedule
		}
		if subConfig == nil {
			subConfig = map[string]any{}
		}

		configBytes, err := json.Marshal(subConfig)
		if err != nil {
			return fmt.Errorf("trigger node %q: marshal sub-config: %w", node.ID, err)
		}

		// InputMapping defaults to {}.
		inputMapping := cfg.InputMapping
		if inputMapping == nil {
			inputMapping = map[string]any{}
		}
		inputMappingBytes, err := json.Marshal(inputMapping)
		if err != nil {
			return fmt.Errorf("trigger node %q: marshal input_mapping: %w", node.ID, err)
		}

		// Enabled defaults to true when pointer is nil.
		enabled := true
		if cfg.Enabled != nil {
			enabled = *cfg.Enabled
		}

		tr := &model.WorkflowTrigger{
			WorkflowID:             workflowID,
			ProjectID:              projectID,
			Source:                 model.TriggerSource(cfg.Source),
			Config:                 json.RawMessage(configBytes),
			InputMapping:           json.RawMessage(inputMappingBytes),
			IdempotencyKeyTemplate: cfg.IdempotencyKeyTemplate,
			DedupeWindowSeconds:    cfg.DedupeWindowSeconds,
			Enabled:                enabled,
			CreatedBy:              createdBy,
		}

		if err := r.repo.Upsert(ctx, tr); err != nil {
			return fmt.Errorf("trigger node %q: upsert: %w", node.ID, err)
		}

		keepSet[tr.ID] = struct{}{}
	}

	// Delete rows that are no longer represented in the current DAG.
	existing, err := r.repo.ListByWorkflow(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("sync triggers: list existing: %w", err)
	}

	for _, row := range existing {
		if _, keep := keepSet[row.ID]; keep {
			continue
		}
		if err := r.repo.Delete(ctx, row.ID); err != nil {
			return fmt.Errorf("sync triggers: delete stale row %s: %w", row.ID, err)
		}
	}

	return nil
}
