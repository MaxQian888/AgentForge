package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// WorkflowConfig stores per-project custom task transitions and automation triggers.
type WorkflowConfig struct {
	ID          uuid.UUID       `db:"id"`
	ProjectID   uuid.UUID       `db:"project_id"`
	Transitions json.RawMessage `db:"transitions"` // map[string][]string
	Triggers    json.RawMessage `db:"triggers"`    // []TaskWorkflowTrigger
	CreatedAt   time.Time       `db:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at"`
}

// TaskWorkflowTrigger defines an automation rule that fires on a status transition.
type TaskWorkflowTrigger struct {
	FromStatus string `json:"fromStatus"`
	ToStatus   string `json:"toStatus"`
	Action     string `json:"action"` // e.g. "auto_assign", "notify", "dispatch_agent"
	Config     any    `json:"config,omitempty"`
}

const (
	TaskWorkflowTriggerActionDispatchAgent  = "dispatch_agent"
	TaskWorkflowTriggerActionStartWorkflow  = "start_workflow"
	TaskWorkflowTriggerActionNotify         = "notify"
	TaskWorkflowTriggerActionAutoTransition = "auto_transition"
)

const (
	TaskWorkflowTriggerOutcomeStarted   = "started"
	TaskWorkflowTriggerOutcomeCompleted = "completed"
	TaskWorkflowTriggerOutcomeBlocked   = "blocked"
	TaskWorkflowTriggerOutcomeSkipped   = "skipped"
	TaskWorkflowTriggerOutcomeFailed    = "failed"
)

type TaskWorkflowTriggerOutcome struct {
	Action           string `json:"action"`
	Status           string `json:"status"`
	Reason           string `json:"reason,omitempty"`
	ReasonCode       string `json:"reasonCode,omitempty"`
	WorkflowPluginID string `json:"workflowPluginId,omitempty"`
	WorkflowRunID    string `json:"workflowRunId,omitempty"`
}

type WorkflowConfigDTO struct {
	ID          string                 `json:"id"`
	ProjectID   string                 `json:"projectId"`
	Transitions map[string][]string    `json:"transitions"`
	Triggers    []TaskWorkflowTrigger  `json:"triggers"`
	CreatedAt   string                 `json:"createdAt"`
	UpdatedAt   string                 `json:"updatedAt"`
}

type UpdateWorkflowRequest struct {
	Transitions map[string][]string   `json:"transitions"`
	Triggers    []TaskWorkflowTrigger `json:"triggers"`
}

func (w *WorkflowConfig) ToDTO() WorkflowConfigDTO {
	dto := WorkflowConfigDTO{
		ID:        w.ID.String(),
		ProjectID: w.ProjectID.String(),
		CreatedAt: w.CreatedAt.Format(time.RFC3339),
		UpdatedAt: w.UpdatedAt.Format(time.RFC3339),
	}

	dto.Transitions = make(map[string][]string)
	if len(w.Transitions) > 0 {
		_ = json.Unmarshal(w.Transitions, &dto.Transitions)
	}

	dto.Triggers = make([]TaskWorkflowTrigger, 0)
	if len(w.Triggers) > 0 {
		_ = json.Unmarshal(w.Triggers, &dto.Triggers)
	}

	return dto
}

// ParseTransitions returns the transitions map from the raw JSON.
func (w *WorkflowConfig) ParseTransitions() (map[string][]string, error) {
	if len(w.Transitions) == 0 {
		return nil, nil
	}
	var m map[string][]string
	err := json.Unmarshal(w.Transitions, &m)
	return m, err
}
