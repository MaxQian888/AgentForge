package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type TriggerSource string

const (
	TriggerSourceIM       TriggerSource = "im"
	TriggerSourceSchedule TriggerSource = "schedule"
)

// TriggerTargetKind names the execution engine that handles a trigger dispatch.
// "dag" fires a DAG workflow execution (workflow_executions); "plugin" fires a
// legacy workflow plugin run (workflow_plugin_run).
type TriggerTargetKind string

const (
	TriggerTargetDAG    TriggerTargetKind = "dag"
	TriggerTargetPlugin TriggerTargetKind = "plugin"
)

// TriggerCreatedVia distinguishes how a workflow_triggers row was authored.
// 'dag_node' rows are upserted by the registrar from a workflow definition's
// trigger node and may be replaced when the DAG is re-saved. 'manual' rows
// are authored via the trigger CRUD API (Spec 1C) and are NEVER touched by
// the registrar's merge pass.
type TriggerCreatedVia string

const (
	TriggerCreatedViaDAGNode TriggerCreatedVia = "dag_node"
	TriggerCreatedViaManual  TriggerCreatedVia = "manual"
)

// WorkflowTrigger is the materialized form of a trigger-node subscription.
// Rows are upserted when a workflow definition is saved (see trigger.Registrar.SyncFromDefinition)
// and consulted at runtime by trigger.EventRouter and the scheduler adapter.
//
// Exactly one of WorkflowID / PluginID is populated, discriminated by TargetKind:
//   - TargetKind == TriggerTargetDAG    → WorkflowID set, PluginID empty
//   - TargetKind == TriggerTargetPlugin → PluginID set, WorkflowID nil
type WorkflowTrigger struct {
	ID                     uuid.UUID         `db:"id" json:"id"`
	WorkflowID             *uuid.UUID        `db:"workflow_id" json:"workflowId,omitempty"`
	PluginID               string            `db:"plugin_id" json:"pluginId,omitempty"`
	ProjectID              uuid.UUID         `db:"project_id" json:"projectId"`
	Source                 TriggerSource     `db:"source" json:"source"`
	TargetKind             TriggerTargetKind `db:"target_kind" json:"targetKind"`
	Config                 json.RawMessage   `db:"config" json:"config"`
	InputMapping           json.RawMessage   `db:"input_mapping" json:"inputMapping"`
	IdempotencyKeyTemplate string            `db:"idempotency_key_template" json:"idempotencyKeyTemplate,omitempty"`
	DedupeWindowSeconds    int               `db:"dedupe_window_seconds" json:"dedupeWindowSeconds"`
	Enabled                bool              `db:"enabled" json:"enabled"`
	DisabledReason         string            `db:"disabled_reason" json:"disabledReason,omitempty"`
	CreatedVia             TriggerCreatedVia `db:"created_via"  json:"createdVia"`
	DisplayName            string            `db:"display_name" json:"displayName,omitempty"`
	Description            string            `db:"description"  json:"description,omitempty"`
	ActingEmployeeID       *uuid.UUID        `db:"acting_employee_id" json:"actingEmployeeId,omitempty"`
	CreatedBy              *uuid.UUID        `db:"created_by" json:"createdBy,omitempty"`
	CreatedAt              time.Time         `db:"created_at" json:"createdAt"`
	UpdatedAt              time.Time         `db:"updated_at" json:"updatedAt"`
}
