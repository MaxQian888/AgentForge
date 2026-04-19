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
	ID                     uuid.UUID       `json:"id"`
	WorkflowID             uuid.UUID       `json:"workflowId"`
	ProjectID              uuid.UUID       `json:"projectId"`
	Source                 TriggerSource   `json:"source"`
	Config                 json.RawMessage `json:"config"`
	InputMapping           json.RawMessage `json:"inputMapping"`
	IdempotencyKeyTemplate string          `json:"idempotencyKeyTemplate,omitempty"`
	DedupeWindowSeconds    int             `json:"dedupeWindowSeconds"`
	Enabled                bool            `json:"enabled"`
	CreatedBy              *uuid.UUID      `json:"createdBy,omitempty"`
	CreatedAt              time.Time       `json:"createdAt"`
	UpdatedAt              time.Time       `json:"updatedAt"`
}
