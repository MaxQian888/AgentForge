package model

import (
	"time"

	"github.com/google/uuid"
)

type WorkflowRunStatus string

const (
	WorkflowRunStatusPending   WorkflowRunStatus = "pending"
	WorkflowRunStatusRunning   WorkflowRunStatus = "running"
	WorkflowRunStatusCompleted WorkflowRunStatus = "completed"
	WorkflowRunStatusFailed    WorkflowRunStatus = "failed"
	WorkflowRunStatusCancelled WorkflowRunStatus = "cancelled"
	WorkflowRunStatusPaused    WorkflowRunStatus = "paused"
)

type WorkflowStepRunStatus string

const (
	WorkflowStepRunStatusPending              WorkflowStepRunStatus = "pending"
	WorkflowStepRunStatusRunning              WorkflowStepRunStatus = "running"
	WorkflowStepRunStatusCompleted            WorkflowStepRunStatus = "completed"
	WorkflowStepRunStatusFailed               WorkflowStepRunStatus = "failed"
	WorkflowStepRunStatusSkipped              WorkflowStepRunStatus = "skipped"
	// WorkflowStepRunStatusAwaitingSubWorkflow parks the step while a DAG
	// child runs. Introduced by bridge-legacy-to-dag-invocation so the step
	// router's `workflow` action can invoke a DAG child and wait for its
	// terminal state before advancing the parent plugin run. The run-level
	// status transitions to WorkflowRunStatusPaused for the duration.
	WorkflowStepRunStatusAwaitingSubWorkflow WorkflowStepRunStatus = "awaiting_sub_workflow"
)

type WorkflowStepAttempt struct {
	Attempt     int                   `json:"attempt"`
	Status      WorkflowStepRunStatus `json:"status"`
	Output      map[string]any        `json:"output,omitempty"`
	Error       string                `json:"error,omitempty"`
	StartedAt   time.Time             `json:"started_at"`
	CompletedAt *time.Time            `json:"completed_at,omitempty"`
}

type WorkflowStepRun struct {
	StepID      string                `json:"step_id"`
	RoleID      string                `json:"role_id"`
	Action      WorkflowActionType    `json:"action"`
	Status      WorkflowStepRunStatus `json:"status"`
	Input       map[string]any        `json:"input,omitempty"`
	Output      map[string]any        `json:"output,omitempty"`
	RetryCount  int                   `json:"retry_count"`
	Error       string                `json:"error,omitempty"`
	Attempts    []WorkflowStepAttempt `json:"attempts,omitempty"`
	StartedAt   *time.Time            `json:"started_at,omitempty"`
	CompletedAt *time.Time            `json:"completed_at,omitempty"`
}

type WorkflowPluginRun struct {
	ID               uuid.UUID           `json:"id"`
	PluginID         string              `json:"plugin_id"`
	// ProjectID scopes the run to its originating project. Populated when the
	// run is started in response to a trigger (trigger rows carry ProjectID);
	// runs started through non-project-scoped entry points leave this zero.
	// Used by the unified workflow-run view to enforce project scope at the
	// service layer (bridge-unified-run-view).
	ProjectID        uuid.UUID           `json:"project_id,omitempty"`
	Process          WorkflowProcessMode `json:"process"`
	Status           WorkflowRunStatus   `json:"status"`
	Trigger          map[string]any      `json:"trigger,omitempty"`
	CurrentStepID    string              `json:"current_step_id,omitempty"`
	Steps            []WorkflowStepRun   `json:"steps,omitempty"`
	ActingEmployeeID *uuid.UUID          `json:"acting_employee_id,omitempty"`
	TriggerID        *uuid.UUID          `json:"trigger_id,omitempty"`
	Error            string              `json:"error,omitempty"`
	StartedAt        time.Time           `json:"started_at"`
	CompletedAt      *time.Time          `json:"completed_at,omitempty"`
}
