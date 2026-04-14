package model

import "time"

type AgentPoolQueueStatus string

const (
	AgentPoolQueueStatusQueued    AgentPoolQueueStatus = "queued"
	AgentPoolQueueStatusAdmitted  AgentPoolQueueStatus = "admitted"
	AgentPoolQueueStatusPromoted  AgentPoolQueueStatus = "promoted"
	AgentPoolQueueStatusFailed    AgentPoolQueueStatus = "failed"
	AgentPoolQueueStatusCancelled AgentPoolQueueStatus = "cancelled"
)

const (
	PriorityLow      = 0
	PriorityNormal   = 10
	PriorityHigh     = 20
	PriorityCritical = 30
)

const (
	QueueRecoveryDispositionPending     = "pending"
	QueueRecoveryDispositionRecoverable = "recoverable"
	QueueRecoveryDispositionTerminal    = "terminal"
	QueueRecoveryDispositionPromoted    = "promoted"
	QueueRecoveryDispositionCancelled   = "cancelled"
)

type AgentPoolQueueEntry struct {
	EntryID    string               `db:"entry_id" json:"entryId"`
	ProjectID  string               `db:"project_id" json:"projectId"`
	TaskID     string               `db:"task_id" json:"taskId"`
	MemberID   string               `db:"member_id" json:"memberId"`
	Status     AgentPoolQueueStatus `db:"status" json:"status"`
	Reason     string               `db:"reason" json:"reason"`
	Runtime    string               `db:"runtime" json:"runtime"`
	Provider   string               `db:"provider" json:"provider"`
	Model      string               `db:"model" json:"model"`
	RoleID     string               `db:"role_id" json:"roleId,omitempty"`
	Priority   int                  `db:"priority" json:"priority"`
	BudgetUSD  float64              `db:"budget_usd" json:"budgetUsd"`
	GuardrailType       string      `db:"guardrail_type" json:"guardrailType,omitempty"`
	GuardrailScope      string      `db:"guardrail_scope" json:"guardrailScope,omitempty"`
	RecoveryDisposition string      `db:"recovery_disposition" json:"recoveryDisposition,omitempty"`
	AgentRunID *string              `db:"agent_run_id" json:"agentRunId,omitempty"`
	CreatedAt  time.Time            `db:"created_at" json:"createdAt"`
	UpdatedAt  time.Time            `db:"updated_at" json:"updatedAt"`
}

type QueueEntryDTO struct {
	EntryID    string  `json:"entryId"`
	ProjectID  string  `json:"projectId"`
	TaskID     string  `json:"taskId"`
	MemberID   string  `json:"memberId"`
	Status     string  `json:"status"`
	Reason     string  `json:"reason"`
	Runtime    string  `json:"runtime"`
	Provider   string  `json:"provider"`
	Model      string  `json:"model"`
	RoleID     string  `json:"roleId,omitempty"`
	Priority   int     `json:"priority"`
	BudgetUSD  float64 `json:"budgetUsd"`
	GuardrailType       string  `json:"guardrailType,omitempty"`
	GuardrailScope      string  `json:"guardrailScope,omitempty"`
	RecoveryDisposition string  `json:"recoveryDisposition,omitempty"`
	AgentRunID *string `json:"agentRunId,omitempty"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
}

func (e *AgentPoolQueueEntry) ToDTO() QueueEntryDTO {
	if e == nil {
		return QueueEntryDTO{}
	}
	dto := QueueEntryDTO{
		EntryID:   e.EntryID,
		ProjectID: e.ProjectID,
		TaskID:    e.TaskID,
		MemberID:  e.MemberID,
		Status:    string(e.Status),
		Reason:    e.Reason,
		Runtime:   e.Runtime,
		Provider:  e.Provider,
		Model:     e.Model,
		RoleID:    e.RoleID,
		Priority:  e.Priority,
		BudgetUSD: e.BudgetUSD,
		GuardrailType:       e.GuardrailType,
		GuardrailScope:      e.GuardrailScope,
		RecoveryDisposition: e.RecoveryDisposition,
		CreatedAt: e.CreatedAt.Format(time.RFC3339),
		UpdatedAt: e.UpdatedAt.Format(time.RFC3339),
	}
	if e.AgentRunID != nil {
		value := *e.AgentRunID
		dto.AgentRunID = &value
	}
	return dto
}
