// Package trigger implements trigger-node materialization for workflow definitions.
package trigger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/react-go-quick-starter/server/internal/employee"
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

// DAGDefinitionResolver is the minimum read-side dependency Registrar needs
// to validate that a DAG target workflow exists in the definitions store.
// *repository.WorkflowDefinitionRepository satisfies this structurally via
// its GetByID method.
type DAGDefinitionResolver interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowDefinition, error)
}

// PluginTargetResolver is the minimum read-side dependency Registrar needs
// to validate that a plugin target refers to an enabled workflow plugin
// with an executable process mode. Returns the plugin record or an error
// explaining why it cannot be resolved.
type PluginTargetResolver interface {
	GetByID(ctx context.Context, pluginID string) (*model.PluginRecord, error)
}

// DisabledReason codes identify why a trigger was persisted with
// enabled=false during registrar sync. These are machine-readable so that
// both the sync response and downstream observability consumers can
// discriminate retriable from permanent failures.
const (
	DisabledReasonDAGWorkflowMissing       = "dag_workflow_not_found"
	DisabledReasonDAGWorkflowInactive      = "dag_workflow_inactive"
	DisabledReasonPluginMissing            = "plugin_not_found"
	DisabledReasonPluginDisabled           = "plugin_disabled"
	DisabledReasonPluginNotWorkflow        = "plugin_not_workflow_kind"
	DisabledReasonPluginNotExecutable      = "plugin_process_not_executable"
	DisabledReasonPluginMissingID          = "plugin_target_missing_plugin_id"
	DisabledReasonActingEmployeeMissing    = "acting_employee_not_found"
	DisabledReasonActingEmployeeCross      = "acting_employee_cross_project"
	DisabledReasonActingEmployeeArchived   = "acting_employee_archived"
)

// SyncOutcome reports the result of materializing one trigger node.
// Successful rows carry TargetKind + the resolved identifier; unresolved
// rows carry DisabledReason with a machine-readable code.
type SyncOutcome struct {
	TriggerID      uuid.UUID               `json:"triggerId"`
	TargetKind     model.TriggerTargetKind `json:"targetKind"`
	Enabled        bool                    `json:"enabled"`
	DisabledReason string                  `json:"disabledReason,omitempty"`
}

// triggerNodeConfig is the typed shape of a trigger node's Config JSON.
type triggerNodeConfig struct {
	Source                 string         `json:"source"`
	TargetKind             string         `json:"target_kind,omitempty"`
	PluginID               string         `json:"plugin_id,omitempty"`
	ActingEmployeeID       string         `json:"acting_employee_id,omitempty"`
	IM                     map[string]any `json:"im,omitempty"`
	Schedule               map[string]any `json:"schedule,omitempty"`
	InputMapping           map[string]any `json:"input_mapping,omitempty"`
	IdempotencyKeyTemplate string         `json:"idempotency_key_template,omitempty"`
	DedupeWindowSeconds    int            `json:"dedupe_window_seconds,omitempty"`
	Enabled                *bool          `json:"enabled,omitempty"`
}

// ActingEmployeeValidator is the author-time employee guard used during
// registrar sync. Satisfied in production by *employee.AttributionGuard.
// When unset, the registrar accepts acting_employee_id values without
// cross-project validation (backwards-compatible default).
type ActingEmployeeValidator interface {
	ValidateForProject(ctx context.Context, employeeID uuid.UUID, projectID uuid.UUID) error
}

// Registrar materializes trigger-node subscriptions from a workflow
// definition into rows in workflow_triggers. It is called from the
// workflow-save path (Task 15) so every save reconciles the set of
// enabled triggers with the current DAG.
//
// When DAG or plugin target resolvers are wired, the registrar validates
// that the referenced workflow / plugin is currently executable and
// persists unresolvable targets with enabled=false + disabled_reason.
// Unwired resolvers skip validation (backwards-compatible default).
type Registrar struct {
	repo              TriggerRepository
	dagResolver       DAGDefinitionResolver
	pluginResolver    PluginTargetResolver
	attributionGuard  ActingEmployeeValidator
}

// NewRegistrar returns a new Registrar backed by repo.
func NewRegistrar(repo TriggerRepository) *Registrar {
	return &Registrar{repo: repo}
}

// WithDAGResolver wires the DAG definitions resolver used to validate DAG
// target triggers. Returns the same Registrar for chaining.
func (r *Registrar) WithDAGResolver(resolver DAGDefinitionResolver) *Registrar {
	r.dagResolver = resolver
	return r
}

// WithPluginResolver wires the plugin registry resolver used to validate
// plugin target triggers. Returns the same Registrar for chaining.
func (r *Registrar) WithPluginResolver(resolver PluginTargetResolver) *Registrar {
	r.pluginResolver = resolver
	return r
}

// WithAttributionGuard wires the author-time acting-employee guard used to
// validate that an acting_employee_id on a trigger node config belongs to the
// same project as the workflow being synced. Returns the same Registrar for
// chaining.
func (r *Registrar) WithAttributionGuard(guard ActingEmployeeValidator) *Registrar {
	r.attributionGuard = guard
	return r
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
//
// The returned SyncOutcome slice mirrors the upserted rows so callers can
// surface per-trigger disabled reasons. Rows that were deleted (stale) are
// not reported.
func (r *Registrar) SyncFromDefinition(
	ctx context.Context,
	workflowID, projectID uuid.UUID,
	nodes []model.WorkflowNode,
	createdBy *uuid.UUID,
) ([]SyncOutcome, error) {
	keepSet := make(map[uuid.UUID]struct{})
	outcomes := make([]SyncOutcome, 0)

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
			return outcomes, fmt.Errorf("trigger node %q has unsupported source %q", node.ID, cfg.Source)
		}

		// Resolve target_kind, defaulting to DAG when omitted.
		targetKind := model.TriggerTargetKind(strings.TrimSpace(cfg.TargetKind))
		if targetKind == "" {
			targetKind = model.TriggerTargetDAG
		}
		if targetKind != model.TriggerTargetDAG && targetKind != model.TriggerTargetPlugin {
			return outcomes, fmt.Errorf("trigger node %q has unsupported target_kind %q", node.ID, targetKind)
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
			return outcomes, fmt.Errorf("trigger node %q: marshal sub-config: %w", node.ID, err)
		}

		// InputMapping defaults to {}.
		inputMapping := cfg.InputMapping
		if inputMapping == nil {
			inputMapping = map[string]any{}
		}
		inputMappingBytes, err := json.Marshal(inputMapping)
		if err != nil {
			return outcomes, fmt.Errorf("trigger node %q: marshal input_mapping: %w", node.ID, err)
		}

		// Enabled defaults to true when pointer is nil.
		requestedEnabled := true
		if cfg.Enabled != nil {
			requestedEnabled = *cfg.Enabled
		}

		tr := &model.WorkflowTrigger{
			ProjectID:              projectID,
			Source:                 model.TriggerSource(cfg.Source),
			TargetKind:             targetKind,
			Config:                 json.RawMessage(configBytes),
			InputMapping:           json.RawMessage(inputMappingBytes),
			IdempotencyKeyTemplate: cfg.IdempotencyKeyTemplate,
			DedupeWindowSeconds:    cfg.DedupeWindowSeconds,
			Enabled:                requestedEnabled,
			// Spec 1C §6.2: rows materialized from a DAG trigger node are
			// owned by the registrar and may be reaped on re-save. The
			// distinct 'manual' value is used for FE-authored rows and is
			// asserted to never be touched by the cleanup loop below.
			CreatedVia: model.TriggerCreatedViaDAGNode,
			CreatedBy:  createdBy,
		}

		// Parse and stamp the optional acting_employee_id. An invalid UUID is
		// treated the same as an unknown employee at validateTarget time.
		if strings.TrimSpace(cfg.ActingEmployeeID) != "" {
			if parsed, parseErr := uuid.Parse(cfg.ActingEmployeeID); parseErr == nil {
				empRef := parsed
				tr.ActingEmployeeID = &empRef
			} else {
				// Invalid UUID: record the reference in a disabled form so the
				// author gets a structured reason rather than a silent drop.
				tr.Enabled = false
				tr.DisabledReason = DisabledReasonActingEmployeeMissing
			}
		}

		// Wire the identifier field matching the declared target engine.
		switch targetKind {
		case model.TriggerTargetDAG:
			workflowRef := workflowID
			tr.WorkflowID = &workflowRef
		case model.TriggerTargetPlugin:
			tr.PluginID = strings.TrimSpace(cfg.PluginID)
		}

		// Resolve the target against the declared engine. If unresolvable,
		// persist the row as enabled=false with a structured reason. Skip
		// when a prior check already disabled this row.
		if tr.Enabled {
			reason := r.validateTarget(ctx, tr)
			if reason != "" {
				tr.Enabled = false
				tr.DisabledReason = reason
			}
		}

		if err := r.repo.Upsert(ctx, tr); err != nil {
			return outcomes, fmt.Errorf("trigger node %q: upsert: %w", node.ID, err)
		}

		keepSet[tr.ID] = struct{}{}
		outcomes = append(outcomes, SyncOutcome{
			TriggerID:      tr.ID,
			TargetKind:     tr.TargetKind,
			Enabled:        tr.Enabled,
			DisabledReason: tr.DisabledReason,
		})
	}

	// Delete rows that are no longer represented in the current DAG.
	// Spec 1C §6.2: scope cleanup to created_via='dag_node' rows only —
	// 'manual' rows are owned by the FE CRUD and must survive a DAG re-save.
	existing, err := r.repo.ListByWorkflow(ctx, workflowID)
	if err != nil {
		return outcomes, fmt.Errorf("sync triggers: list existing: %w", err)
	}

	for _, row := range existing {
		if row.CreatedVia == model.TriggerCreatedViaManual {
			// Manual rows are owned by the FE CRUD; never reaped by a sync.
			continue
		}
		if _, keep := keepSet[row.ID]; keep {
			continue
		}
		if err := r.repo.Delete(ctx, row.ID); err != nil {
			return outcomes, fmt.Errorf("sync triggers: delete stale dag_node row %s: %w", row.ID, err)
		}
	}

	return outcomes, nil
}

// validateTarget returns a DisabledReason* code if the trigger's declared
// target cannot currently be dispatched, or "" when the target resolves
// cleanly (or when no resolver is wired — validation is opt-in).
func (r *Registrar) validateTarget(ctx context.Context, tr *model.WorkflowTrigger) string {
	if reason := r.validateEngineTarget(ctx, tr); reason != "" {
		return reason
	}
	if reason := r.validateActingEmployee(ctx, tr); reason != "" {
		return reason
	}
	return ""
}

// validateEngineTarget checks the engine-side identifier (workflow_id /
// plugin_id) resolves to a currently-executable target.
func (r *Registrar) validateEngineTarget(ctx context.Context, tr *model.WorkflowTrigger) string {
	switch tr.TargetKind {
	case model.TriggerTargetDAG:
		if r.dagResolver == nil || tr.WorkflowID == nil {
			return ""
		}
		def, err := r.dagResolver.GetByID(ctx, *tr.WorkflowID)
		if err != nil || def == nil {
			return DisabledReasonDAGWorkflowMissing
		}
		if def.Status != model.WorkflowDefStatusActive {
			return DisabledReasonDAGWorkflowInactive
		}
		return ""
	case model.TriggerTargetPlugin:
		if tr.PluginID == "" {
			return DisabledReasonPluginMissingID
		}
		if r.pluginResolver == nil {
			return ""
		}
		record, err := r.pluginResolver.GetByID(ctx, tr.PluginID)
		if err != nil || record == nil {
			return DisabledReasonPluginMissing
		}
		if record.Kind != model.PluginKindWorkflow || record.Spec.Workflow == nil {
			return DisabledReasonPluginNotWorkflow
		}
		if record.LifecycleState == model.PluginStateDisabled {
			return DisabledReasonPluginDisabled
		}
		if record.LifecycleState != model.PluginStateEnabled && record.LifecycleState != model.PluginStateActive {
			return DisabledReasonPluginNotExecutable
		}
		return ""
	default:
		return ""
	}
}

// validateActingEmployee consults the attribution guard to check that the
// optional acting_employee_id on this trigger belongs to the same project as
// the workflow. Returns a DisabledReason* code if the guard rejects the
// reference, "" otherwise. When no guard is wired (default), returns "".
func (r *Registrar) validateActingEmployee(ctx context.Context, tr *model.WorkflowTrigger) string {
	if tr.ActingEmployeeID == nil || r.attributionGuard == nil {
		return ""
	}
	err := r.attributionGuard.ValidateForProject(ctx, *tr.ActingEmployeeID, tr.ProjectID)
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, employee.ErrEmployeeCrossProject):
		return DisabledReasonActingEmployeeCross
	case errors.Is(err, employee.ErrEmployeeArchived):
		return DisabledReasonActingEmployeeArchived
	case errors.Is(err, employee.ErrEmployeeNotFound):
		return DisabledReasonActingEmployeeMissing
	default:
		return DisabledReasonActingEmployeeMissing
	}
}
