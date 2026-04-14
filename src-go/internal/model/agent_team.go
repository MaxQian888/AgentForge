package model

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	TeamStatusPending   = "pending"
	TeamStatusPlanning  = "planning"
	TeamStatusExecuting = "executing"
	TeamStatusReviewing = "reviewing"
	TeamStatusCompleted = "completed"
	TeamStatusFailed    = "failed"
	TeamStatusCancelled = "cancelled"
)

const (
	TeamRolePlanner  = "planner"
	TeamRoleCoder    = "coder"
	TeamRoleReviewer = "reviewer"
)

type AgentTeam struct {
	ID                    uuid.UUID  `db:"id"`
	ProjectID             uuid.UUID  `db:"project_id"`
	TaskID                uuid.UUID  `db:"task_id"`
	Name                  string     `db:"name"`
	Status                string     `db:"status"`
	Strategy              string     `db:"strategy"`
	PlannerRunID          *uuid.UUID `db:"planner_run_id"`
	ReviewerRunID         *uuid.UUID `db:"reviewer_run_id"`
	TotalBudgetUsd        float64    `db:"total_budget_usd"`
	TotalSpentUsd         float64    `db:"total_spent_usd"`
	Config                string     `db:"config"`
	ErrorMessage          string     `db:"error_message"`
	WorkflowExecutionID   *uuid.UUID `db:"workflow_execution_id"`
	CreatedAt             time.Time  `db:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at"`
}

type AgentTeamDTO struct {
	ID             string  `json:"id"`
	ProjectID      string  `json:"projectId"`
	TaskID         string  `json:"taskId"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	Strategy       string  `json:"strategy"`
	Runtime        string  `json:"runtime"`
	Provider       string  `json:"provider"`
	Model          string  `json:"model"`
	PlannerRunID   *string `json:"plannerRunId,omitempty"`
	ReviewerRunID  *string `json:"reviewerRunId,omitempty"`
	TotalBudgetUsd float64 `json:"totalBudgetUsd"`
	TotalSpentUsd  float64 `json:"totalSpentUsd"`
	ErrorMessage   string  `json:"errorMessage"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

type AgentTeamSummaryDTO struct {
	AgentTeamDTO
	TaskTitle      string        `json:"taskTitle"`
	PlannerStatus  string        `json:"plannerStatus"`
	ReviewerStatus string        `json:"reviewerStatus"`
	CoderRuns      []AgentRunDTO `json:"coderRuns"`
	CoderTotal     int           `json:"coderTotal"`
	CoderCompleted int           `json:"coderCompleted"`
}

type UpdateTeamRequest struct {
	Name           *string  `json:"name"`
	TotalBudgetUsd *float64 `json:"totalBudgetUsd"`
}

func (t *AgentTeam) ToDTO() AgentTeamDTO {
	dto := AgentTeamDTO{
		ID:             t.ID.String(),
		ProjectID:      t.ProjectID.String(),
		TaskID:         t.TaskID.String(),
		Name:           t.Name,
		Status:         t.Status,
		Strategy:       t.Strategy,
		TotalBudgetUsd: t.TotalBudgetUsd,
		TotalSpentUsd:  t.TotalSpentUsd,
		ErrorMessage:   t.ErrorMessage,
		CreatedAt:      t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      t.UpdatedAt.Format(time.RFC3339),
	}
	selection := t.CodingAgentSelection()
	dto.Runtime = selection.Runtime
	dto.Provider = selection.Provider
	dto.Model = selection.Model
	if t.PlannerRunID != nil {
		s := t.PlannerRunID.String()
		dto.PlannerRunID = &s
	}
	if t.ReviewerRunID != nil {
		s := t.ReviewerRunID.String()
		dto.ReviewerRunID = &s
	}
	return dto
}

func (t *AgentTeam) CodingAgentSelection() CodingAgentSelection {
	trimmed := strings.TrimSpace(t.Config)
	if trimmed == "" {
		return CodingAgentSelection{}
	}

	var selection CodingAgentSelection
	if err := json.Unmarshal([]byte(trimmed), &selection); err != nil {
		return CodingAgentSelection{}
	}
	return selection
}

func IsTerminalTeamStatus(status string) bool {
	switch status {
	case TeamStatusCompleted, TeamStatusFailed, TeamStatusCancelled:
		return true
	}
	return false
}
