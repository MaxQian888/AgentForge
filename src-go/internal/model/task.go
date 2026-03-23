package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Task struct {
	ID             uuid.UUID  `db:"id"`
	ProjectID      uuid.UUID  `db:"project_id"`
	ParentID       *uuid.UUID `db:"parent_id"`
	SprintID       *uuid.UUID `db:"sprint_id"`
	Title          string     `db:"title"`
	Description    string     `db:"description"`
	Status         string     `db:"status"`
	Priority       string     `db:"priority"`
	AssigneeID     *uuid.UUID `db:"assignee_id"`
	AssigneeType   string     `db:"assignee_type"` // "human" or "agent"
	ReporterID     *uuid.UUID `db:"reporter_id"`
	Labels         []string   `db:"labels"`
	BudgetUsd      float64    `db:"budget_usd"`
	SpentUsd       float64    `db:"spent_usd"`
	AgentBranch    string     `db:"agent_branch"`
	AgentWorktree  string     `db:"agent_worktree"`
	AgentSessionID string     `db:"agent_session_id"`
	PRUrl          string     `db:"pr_url"`
	PRNumber       int        `db:"pr_number"`
	BlockedBy      []string   `db:"blocked_by"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	CompletedAt    *time.Time `db:"completed_at"`
}

// Task status constants.
const (
	TaskStatusInbox            = "inbox"
	TaskStatusTriaged          = "triaged"
	TaskStatusAssigned         = "assigned"
	TaskStatusInProgress       = "in_progress"
	TaskStatusInReview         = "in_review"
	TaskStatusChangesRequested = "changes_requested"
	TaskStatusDone             = "done"
	TaskStatusCancelled        = "cancelled"
	TaskStatusBlocked          = "blocked"
	TaskStatusBudgetExceeded   = "budget_exceeded"
)

// validTransitions defines allowed status transitions.
var validTransitions = map[string][]string{
	TaskStatusInbox:            {TaskStatusTriaged, TaskStatusCancelled},
	TaskStatusTriaged:          {TaskStatusAssigned, TaskStatusCancelled},
	TaskStatusAssigned:         {TaskStatusInProgress, TaskStatusCancelled},
	TaskStatusInProgress:       {TaskStatusInReview, TaskStatusBlocked, TaskStatusCancelled, TaskStatusBudgetExceeded},
	TaskStatusInReview:         {TaskStatusDone, TaskStatusChangesRequested, TaskStatusCancelled},
	TaskStatusChangesRequested: {TaskStatusInProgress, TaskStatusCancelled},
	TaskStatusBlocked:          {TaskStatusInProgress, TaskStatusCancelled},
	TaskStatusBudgetExceeded:   {TaskStatusInProgress, TaskStatusCancelled},
}

// ValidateTransition checks whether a status transition is allowed.
func ValidateTransition(from, to string) error {
	allowed, ok := validTransitions[from]
	if !ok {
		return fmt.Errorf("unknown status: %s", from)
	}
	for _, s := range allowed {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("invalid transition from %s to %s", from, to)
}

type TaskDTO struct {
	ID             string   `json:"id"`
	ProjectID      string   `json:"projectId"`
	ParentID       *string  `json:"parentId,omitempty"`
	SprintID       *string  `json:"sprintId,omitempty"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Status         string   `json:"status"`
	Priority       string   `json:"priority"`
	AssigneeID     *string  `json:"assigneeId,omitempty"`
	AssigneeType   string   `json:"assigneeType"`
	ReporterID     *string  `json:"reporterId,omitempty"`
	Labels         []string `json:"labels"`
	BudgetUsd      float64  `json:"budgetUsd"`
	SpentUsd       float64  `json:"spentUsd"`
	AgentBranch    string   `json:"agentBranch"`
	AgentWorktree  string   `json:"agentWorktree"`
	AgentSessionID string   `json:"agentSessionId"`
	PRUrl          string   `json:"prUrl"`
	PRNumber       int      `json:"prNumber"`
	BlockedBy      []string `json:"blockedBy"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
	CompletedAt    *string  `json:"completedAt,omitempty"`
}

type CreateTaskRequest struct {
	Title       string   `json:"title" validate:"required,min=1,max=200"`
	Description string   `json:"description"`
	Priority    string   `json:"priority" validate:"required,oneof=critical high medium low"`
	ParentID    *string  `json:"parentId"`
	SprintID    *string  `json:"sprintId"`
	Labels      []string `json:"labels"`
	BudgetUsd   float64  `json:"budgetUsd"`
}

type UpdateTaskRequest struct {
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	Priority    *string  `json:"priority"`
	SprintID    *string  `json:"sprintId"`
	Labels      []string `json:"labels"`
	BudgetUsd   *float64 `json:"budgetUsd"`
}

type TaskListQuery struct {
	Status     string `json:"status"`
	AssigneeID string `json:"assigneeId"`
	SprintID   string `json:"sprintId"`
	Priority   string `json:"priority"`
	Search     string `json:"search"`
	Page       int    `json:"page"`
	Limit      int    `json:"limit"`
	Sort       string `json:"sort"`
}

type TransitionRequest struct {
	Status string `json:"status" validate:"required"`
	Reason string `json:"reason"`
}

type AssignRequest struct {
	AssigneeID   string `json:"assigneeId" validate:"required"`
	AssigneeType string `json:"assigneeType" validate:"required,oneof=human agent"`
}

func (t *Task) ToDTO() TaskDTO {
	dto := TaskDTO{
		ID:             t.ID.String(),
		ProjectID:      t.ProjectID.String(),
		Title:          t.Title,
		Description:    t.Description,
		Status:         t.Status,
		Priority:       t.Priority,
		AssigneeType:   t.AssigneeType,
		Labels:         t.Labels,
		BudgetUsd:      t.BudgetUsd,
		SpentUsd:       t.SpentUsd,
		AgentBranch:    t.AgentBranch,
		AgentWorktree:  t.AgentWorktree,
		AgentSessionID: t.AgentSessionID,
		PRUrl:          t.PRUrl,
		PRNumber:       t.PRNumber,
		BlockedBy:      t.BlockedBy,
		CreatedAt:      t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      t.UpdatedAt.Format(time.RFC3339),
	}
	if t.ParentID != nil {
		s := t.ParentID.String()
		dto.ParentID = &s
	}
	if t.SprintID != nil {
		s := t.SprintID.String()
		dto.SprintID = &s
	}
	if t.AssigneeID != nil {
		s := t.AssigneeID.String()
		dto.AssigneeID = &s
	}
	if t.ReporterID != nil {
		s := t.ReporterID.String()
		dto.ReporterID = &s
	}
	if t.CompletedAt != nil {
		s := t.CompletedAt.Format(time.RFC3339)
		dto.CompletedAt = &s
	}
	return dto
}
