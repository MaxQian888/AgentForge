package model

import (
	"time"

	"github.com/google/uuid"
)

type AgentRun struct {
	ID              uuid.UUID  `db:"id"`
	TaskID          uuid.UUID  `db:"task_id"`
	MemberID        uuid.UUID  `db:"member_id"`
	RoleID          string     `db:"role_id"`
	Status          string     `db:"status"`
	Provider        string     `db:"provider"`
	Model           string     `db:"model"`
	InputTokens     int64      `db:"input_tokens"`
	OutputTokens    int64      `db:"output_tokens"`
	CacheReadTokens int64      `db:"cache_read_tokens"`
	CostUsd         float64    `db:"cost_usd"`
	TurnCount       int        `db:"turn_count"`
	ErrorMessage    string     `db:"error_message"`
	StartedAt       time.Time  `db:"started_at"`
	CompletedAt     *time.Time `db:"completed_at"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
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
	ID              string  `json:"id"`
	TaskID          string  `json:"taskId"`
	MemberID        string  `json:"memberId"`
	RoleID          string  `json:"roleId,omitempty"`
	Status          string  `json:"status"`
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	InputTokens     int64   `json:"inputTokens"`
	OutputTokens    int64   `json:"outputTokens"`
	CacheReadTokens int64   `json:"cacheReadTokens"`
	CostUsd         float64 `json:"costUsd"`
	TurnCount       int     `json:"turnCount"`
	ErrorMessage    string  `json:"errorMessage"`
	StartedAt       string  `json:"startedAt"`
	CompletedAt     *string `json:"completedAt,omitempty"`
	CreatedAt       string  `json:"createdAt"`
}

func (a *AgentRun) ToDTO() AgentRunDTO {
	dto := AgentRunDTO{
		ID:              a.ID.String(),
		TaskID:          a.TaskID.String(),
		MemberID:        a.MemberID.String(),
		RoleID:          a.RoleID,
		Status:          a.Status,
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
	}
	if a.CompletedAt != nil {
		s := a.CompletedAt.Format(time.RFC3339)
		dto.CompletedAt = &s
	}
	return dto
}
