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

// WorkflowTrigger is the materialized form of a trigger-node subscription.
// Rows are upserted when a workflow definition is saved (see trigger.Registrar.SyncFromDefinition)
// and consulted at runtime by trigger.EventRouter and the scheduler adapter.
type WorkflowTrigger struct {
	ID                     uuid.UUID       `db:"id" json:"id"`
	WorkflowID             uuid.UUID       `db:"workflow_id" json:"workflowId"`
	ProjectID              uuid.UUID       `db:"project_id" json:"projectId"`
	Source                 TriggerSource   `db:"source" json:"source"`
	Config                 json.RawMessage `db:"config" json:"config"`
	InputMapping           json.RawMessage `db:"input_mapping" json:"inputMapping"`
	IdempotencyKeyTemplate string          `db:"idempotency_key_template" json:"idempotencyKeyTemplate,omitempty"`
	DedupeWindowSeconds    int             `db:"dedupe_window_seconds" json:"dedupeWindowSeconds"`
	Enabled                bool            `db:"enabled" json:"enabled"`
	CreatedBy              *uuid.UUID      `db:"created_by" json:"createdBy,omitempty"`
	CreatedAt              time.Time       `db:"created_at" json:"createdAt"`
	UpdatedAt              time.Time       `db:"updated_at" json:"updatedAt"`
}
