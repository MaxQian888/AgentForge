package model

import (
	"time"

	"github.com/google/uuid"
)

type ReviewAggregation struct {
	ID             uuid.UUID   `db:"id"`
	PRURL          string      `db:"pr_url"`
	TaskID         uuid.UUID   `db:"task_id"`
	ReviewIDs      []uuid.UUID `db:"review_ids"`
	OverallRisk    string      `db:"overall_risk"`
	Recommendation string      `db:"recommendation"`
	Findings       string      `db:"findings"` // JSONB stored as string
	Summary        string      `db:"summary"`
	Metrics        string      `db:"metrics"` // JSONB stored as string
	HumanDecision  *string     `db:"human_decision"`
	HumanReviewer  *uuid.UUID  `db:"human_reviewer"`
	HumanComment   *string     `db:"human_comment"`
	DecidedAt      *time.Time  `db:"decided_at"`
	TotalCostUsd   float64     `db:"total_cost_usd"`
	CreatedAt      time.Time   `db:"created_at"`
	UpdatedAt      time.Time   `db:"updated_at"`
}

type ReviewAggregationDTO struct {
	ID             string   `json:"id"`
	PRURL          string   `json:"prUrl"`
	TaskID         string   `json:"taskId"`
	ReviewIDs      []string `json:"reviewIds"`
	OverallRisk    string   `json:"overallRisk"`
	Recommendation string   `json:"recommendation"`
	Findings       string   `json:"findings"`
	Summary        string   `json:"summary"`
	Metrics        string   `json:"metrics"`
	HumanDecision  *string  `json:"humanDecision,omitempty"`
	HumanComment   *string  `json:"humanComment,omitempty"`
	DecidedAt      *string  `json:"decidedAt,omitempty"`
	TotalCostUsd   float64  `json:"totalCostUsd"`
	CreatedAt      string   `json:"createdAt"`
}

func (ra *ReviewAggregation) ToDTO() ReviewAggregationDTO {
	dto := ReviewAggregationDTO{
		ID:             ra.ID.String(),
		PRURL:          ra.PRURL,
		TaskID:         ra.TaskID.String(),
		OverallRisk:    ra.OverallRisk,
		Recommendation: ra.Recommendation,
		Findings:       ra.Findings,
		Summary:        ra.Summary,
		Metrics:        ra.Metrics,
		HumanDecision:  ra.HumanDecision,
		HumanComment:   ra.HumanComment,
		TotalCostUsd:   ra.TotalCostUsd,
		CreatedAt:      ra.CreatedAt.Format(time.RFC3339),
	}
	dto.ReviewIDs = make([]string, 0, len(ra.ReviewIDs))
	for _, id := range ra.ReviewIDs {
		dto.ReviewIDs = append(dto.ReviewIDs, id.String())
	}
	if ra.DecidedAt != nil {
		s := ra.DecidedAt.Format(time.RFC3339)
		dto.DecidedAt = &s
	}
	return dto
}
