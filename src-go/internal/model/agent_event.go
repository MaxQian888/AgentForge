package model

import (
	"time"

	"github.com/google/uuid"
)

// AgentEvent represents a persistent lifecycle event for an agent run.
type AgentEvent struct {
	ID         uuid.UUID `db:"id"`
	RunID      uuid.UUID `db:"run_id"`
	TaskID     uuid.UUID `db:"task_id"`
	ProjectID  uuid.UUID `db:"project_id"`
	EventType  string    `db:"event_type"`
	Payload    string    `db:"payload"`
	OccurredAt time.Time `db:"occurred_at"`
	CreatedAt  time.Time `db:"created_at"`
}

const (
	AgentEventSpawn          = "spawn"
	AgentEventRunning        = "running"
	AgentEventPaused         = "paused"
	AgentEventResumed        = "resumed"
	AgentEventCompleted      = "completed"
	AgentEventFailed         = "failed"
	AgentEventCancelled      = "cancelled"
	AgentEventBudgetExceeded = "budget_exceeded"
	AgentEventCostUpdate     = "cost_update"
	AgentEventBudgetWarning  = "budget_warning"
)

// AgentEventDTO is the JSON representation of an agent event.
type AgentEventDTO struct {
	ID         string `json:"id"`
	RunID      string `json:"runId"`
	TaskID     string `json:"taskId"`
	ProjectID  string `json:"projectId"`
	EventType  string `json:"eventType"`
	Payload    string `json:"payload,omitempty"`
	OccurredAt string `json:"occurredAt"`
}

func (e *AgentEvent) ToDTO() AgentEventDTO {
	return AgentEventDTO{
		ID:         e.ID.String(),
		RunID:      e.RunID.String(),
		TaskID:     e.TaskID.String(),
		ProjectID:  e.ProjectID.String(),
		EventType:  e.EventType,
		Payload:    e.Payload,
		OccurredAt: e.OccurredAt.Format(time.RFC3339),
	}
}
