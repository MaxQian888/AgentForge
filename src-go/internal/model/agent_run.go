package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AgentRun struct {
	ID              uuid.UUID               `db:"id"`
	TaskID          uuid.UUID               `db:"task_id"`
	MemberID        uuid.UUID               `db:"member_id"`
	RoleID          string                  `db:"role_id"`
	EmployeeID      *uuid.UUID              `db:"employee_id"`
	Status          string                  `db:"status"`
	Runtime         string                  `db:"runtime"`
	Provider        string                  `db:"provider"`
	Model           string                  `db:"model"`
	InputTokens     int64                   `db:"input_tokens"`
	OutputTokens    int64                   `db:"output_tokens"`
	CacheReadTokens int64                   `db:"cache_read_tokens"`
	CostUsd         float64                 `db:"cost_usd"`
	TurnCount       int                     `db:"turn_count"`
	ErrorMessage    string                  `db:"error_message"`
	StartedAt       time.Time               `db:"started_at"`
	CompletedAt     *time.Time              `db:"completed_at"`
	CreatedAt       time.Time               `db:"created_at"`
	UpdatedAt       time.Time               `db:"updated_at"`
	TeamID           *uuid.UUID              `db:"team_id"`
	TeamRole         string                  `db:"team_role"`
	CostAccounting   *CostAccountingSnapshot `db:"cost_accounting"`
	StructuredOutput json.RawMessage         `db:"structured_output"`
}

const (
	AgentRunStatusStarting       = "starting"
	AgentRunStatusRunning        = "running"
	AgentRunStatusPaused         = "paused"
	AgentRunStatusCompleted      = "completed"
	AgentRunStatusFailed         = "failed"
	AgentRunStatusCancelled      = "cancelled"
	AgentRunStatusBudgetExceeded = "budget_exceeded"
)

type AgentRunDTO struct {
	ID               string                  `json:"id"`
	TaskID           string                  `json:"taskId"`
	MemberID         string                  `json:"memberId"`
	RoleID           string                  `json:"roleId,omitempty"`
	Status           string                  `json:"status"`
	Runtime          string                  `json:"runtime"`
	Provider         string                  `json:"provider"`
	Model            string                  `json:"model"`
	InputTokens      int64                   `json:"inputTokens"`
	OutputTokens     int64                   `json:"outputTokens"`
	CacheReadTokens  int64                   `json:"cacheReadTokens"`
	CostUsd          float64                 `json:"costUsd"`
	TurnCount        int                     `json:"turnCount"`
	ErrorMessage     string                  `json:"errorMessage"`
	StartedAt        string                  `json:"startedAt"`
	CompletedAt      *string                 `json:"completedAt,omitempty"`
	CreatedAt        string                  `json:"createdAt"`
	TeamID           *string                 `json:"teamId,omitempty"`
	TeamRole         string                  `json:"teamRole,omitempty"`
	CostAccounting   *CostAccountingSnapshot `json:"costAccounting,omitempty"`
	StructuredOutput json.RawMessage         `json:"structuredOutput,omitempty"`
}

type AgentRunSummaryDTO struct {
	ID              string                  `json:"id"`
	TaskID          string                  `json:"taskId"`
	TaskTitle       string                  `json:"taskTitle"`
	MemberID        string                  `json:"memberId"`
	RoleID          string                  `json:"roleId,omitempty"`
	RoleName        string                  `json:"roleName"`
	Status          string                  `json:"status"`
	Runtime         string                  `json:"runtime"`
	Provider        string                  `json:"provider"`
	Model           string                  `json:"model"`
	InputTokens     int64                   `json:"inputTokens"`
	OutputTokens    int64                   `json:"outputTokens"`
	CacheReadTokens int64                   `json:"cacheReadTokens"`
	CostUsd         float64                 `json:"costUsd"`
	BudgetUsd       float64                 `json:"budgetUsd"`
	TurnCount       int                     `json:"turnCount"`
	WorktreePath    string                  `json:"worktreePath"`
	BranchName      string                  `json:"branchName"`
	SessionID       string                  `json:"sessionId"`
	LastActivityAt  string                  `json:"lastActivityAt"`
	StartedAt       string                  `json:"startedAt"`
	CreatedAt       string                  `json:"createdAt"`
	CompletedAt     *string                 `json:"completedAt,omitempty"`
	CanResume       bool                    `json:"canResume"`
	MemoryStatus    string                  `json:"memoryStatus"`
	TeamID          *string                 `json:"teamId,omitempty"`
	TeamRole        string                  `json:"teamRole,omitempty"`
	CostAccounting  *CostAccountingSnapshot `json:"costAccounting,omitempty"`
}

type AgentPoolStatsDTO struct {
	Active          int                   `json:"active"`
	Max             int                   `json:"max"`
	Available       int                   `json:"available"`
	PausedResumable int                   `json:"pausedResumable"`
	Queued          int                   `json:"queued"`
	Warm            int                   `json:"warm"`
	Degraded        bool                  `json:"degraded"`
	Queue           []AgentPoolQueueEntry `json:"queue,omitempty"`
}

// CostSummaryDTO aggregates cost metrics across multiple agent runs.
type CostSummaryDTO struct {
	TotalCostUsd         float64 `json:"totalCostUsd"`
	TotalInputTokens     int64   `json:"totalInputTokens"`
	TotalOutputTokens    int64   `json:"totalOutputTokens"`
	TotalCacheReadTokens int64   `json:"totalCacheReadTokens"`
	TotalTurns           int     `json:"totalTurns"`
	RunCount             int     `json:"runCount"`
}

// CostTimeSeriesDTO represents a cost data point over time.
type CostTimeSeriesDTO struct {
	Date    string  `json:"date"`
	CostUsd float64 `json:"costUsd"`
	Runs    int     `json:"runs"`
}

// CostGroupDTO represents cost data grouped by a dimension.
type CostGroupDTO struct {
	GroupID string         `json:"groupId"`
	Label   string         `json:"label"`
	Summary CostSummaryDTO `json:"summary"`
}

// VelocityPointDTO represents task completion data for a single time period.
type VelocityPointDTO struct {
	Period         string  `json:"period"`
	TasksCompleted int     `json:"tasksCompleted"`
	CostUsd        float64 `json:"costUsd"`
	AvgCycleTimeH  float64 `json:"avgCycleTimeHours"`
}

// VelocityStatsDTO contains development velocity statistics.
type VelocityStatsDTO struct {
	Points         []VelocityPointDTO `json:"points"`
	TotalCompleted int                `json:"totalCompleted"`
	TotalCostUsd   float64            `json:"totalCostUsd"`
	AvgPerDay      float64            `json:"avgPerDay"`
}

// AgentPerformanceEntryDTO represents performance data for a single agent role.
type AgentPerformanceEntryDTO struct {
	BucketID           string  `json:"bucketId"`
	Label              string  `json:"label"`
	RunCount           int     `json:"runCount"`
	SuccessRate        float64 `json:"successRate"`
	AvgCostUsd         float64 `json:"avgCostUsd"`
	AvgDurationMinutes float64 `json:"avgDurationMinutes"`
	TotalCostUsd       float64 `json:"totalCostUsd"`
}

// AgentPerformanceDTO contains agent performance statistics.
type AgentPerformanceDTO struct {
	Entries []AgentPerformanceEntryDTO `json:"entries"`
}

type AgentLogEntry struct {
	Timestamp string `json:"timestamp"`
	Content   string `json:"content"`
	Type      string `json:"type"` // "output", "tool_call", "tool_result", "error", "status"
}

func (a *AgentRun) ToDTO() AgentRunDTO {
	dto := AgentRunDTO{
		ID:              a.ID.String(),
		TaskID:          a.TaskID.String(),
		MemberID:        a.MemberID.String(),
		RoleID:          a.RoleID,
		Status:          a.Status,
		Runtime:         a.Runtime,
		Provider:        a.Provider,
		Model:           a.Model,
		InputTokens:     a.InputTokens,
		OutputTokens:    a.OutputTokens,
		CacheReadTokens: a.CacheReadTokens,
		CostUsd:         a.CostUsd,
		TurnCount:       a.TurnCount,
		ErrorMessage:    a.ErrorMessage,
		StartedAt:       a.StartedAt.Format(time.RFC3339),
		CreatedAt:       a.CreatedAt.Format(time.RFC3339),
		CostAccounting:  a.CostAccounting.Clone(),
	}
	if a.CompletedAt != nil {
		s := a.CompletedAt.Format(time.RFC3339)
		dto.CompletedAt = &s
	}
	dto.TeamRole = a.TeamRole
	if a.TeamID != nil {
		s := a.TeamID.String()
		dto.TeamID = &s
	}
	if len(a.StructuredOutput) > 0 {
		dto.StructuredOutput = append(json.RawMessage(nil), a.StructuredOutput...)
	}
	return dto
}
