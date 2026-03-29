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
	AgentRunID *string              `db:"agent_run_id" json:"agentRunId,omitempty"`
	CreatedAt  time.Time            `db:"created_at" json:"createdAt"`
	UpdatedAt  time.Time            `db:"updated_at" json:"updatedAt"`
}
