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
)

type WorkflowStepRunStatus string

const (
	WorkflowStepRunStatusPending   WorkflowStepRunStatus = "pending"
	WorkflowStepRunStatusRunning   WorkflowStepRunStatus = "running"
	WorkflowStepRunStatusCompleted WorkflowStepRunStatus = "completed"
	WorkflowStepRunStatusFailed    WorkflowStepRunStatus = "failed"
	WorkflowStepRunStatusSkipped   WorkflowStepRunStatus = "skipped"
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
	ID            uuid.UUID           `json:"id"`
	PluginID      string              `json:"plugin_id"`
	Process       WorkflowProcessMode `json:"process"`
	Status        WorkflowRunStatus   `json:"status"`
	Trigger       map[string]any      `json:"trigger,omitempty"`
	CurrentStepID string              `json:"current_step_id,omitempty"`
	Steps         []WorkflowStepRun   `json:"steps,omitempty"`
	Error         string              `json:"error,omitempty"`
	StartedAt     time.Time           `json:"started_at"`
	CompletedAt   *time.Time          `json:"completed_at,omitempty"`
}
