package model

import (
	"time"

	"github.com/google/uuid"
)

type ReviewFinding struct {
	Category    string   `json:"category"`
	Subcategory string   `json:"subcategory,omitempty"`
	Severity    string   `json:"severity"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Message     string   `json:"message"`
	Suggestion  string   `json:"suggestion,omitempty"`
	CWE         string   `json:"cwe,omitempty"`
	Sources     []string `json:"sources,omitempty"`
}

type Review struct {
	ID             uuid.UUID       `db:"id"`
	TaskID         uuid.UUID       `db:"task_id"`
	PRURL          string          `db:"pr_url"`
	PRNumber       int             `db:"pr_number"`
	Layer          int             `db:"layer"`
	Status         string          `db:"status"`
	RiskLevel      string          `db:"risk_level"`
	Findings       []ReviewFinding `db:"findings"`
	Summary        string          `db:"summary"`
	Recommendation string          `db:"recommendation"`
	CostUSD        float64         `db:"cost_usd"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
}

const (
	ReviewLayerQuick = 1
	ReviewLayerDeep  = 2

	ReviewStatusPending    = "pending"
	ReviewStatusInProgress = "in_progress"
	ReviewStatusCompleted  = "completed"
	ReviewStatusFailed     = "failed"

	ReviewRiskLevelCritical = "critical"
	ReviewRiskLevelHigh     = "high"
	ReviewRiskLevelMedium   = "medium"
	ReviewRiskLevelLow      = "low"

	ReviewRecommendationApprove        = "approve"
	ReviewRecommendationRequestChanges = "request_changes"
	ReviewRecommendationReject         = "reject"

	ReviewTriggerAgent  = "agent"
	ReviewTriggerLayer1 = "layer1"
	ReviewTriggerManual = "manual"
)

type ReviewDTO struct {
	ID             string          `json:"id"`
	TaskID         string          `json:"taskId"`
	PRURL          string          `json:"prUrl"`
	PRNumber       int             `json:"prNumber"`
	Layer          int             `json:"layer"`
	Status         string          `json:"status"`
	RiskLevel      string          `json:"riskLevel"`
	Findings       []ReviewFinding `json:"findings"`
	Summary        string          `json:"summary"`
	Recommendation string          `json:"recommendation"`
	CostUSD        float64         `json:"costUsd"`
	CreatedAt      string          `json:"createdAt"`
	UpdatedAt      string          `json:"updatedAt"`
}

type TriggerReviewRequest struct {
	TaskID     string   `json:"taskId"`
	PRURL      string   `json:"prUrl" validate:"required"`
	PRNumber   int      `json:"prNumber"`
	Trigger    string   `json:"trigger" validate:"required,oneof=agent layer1 manual"`
	Dimensions []string `json:"dimensions"`
	Diff       string   `json:"diff"`
}

type CompleteReviewRequest struct {
	RiskLevel      string          `json:"riskLevel" validate:"required,oneof=critical high medium low"`
	Findings       []ReviewFinding `json:"findings"`
	Summary        string          `json:"summary"`
	Recommendation string          `json:"recommendation" validate:"required,oneof=approve request_changes reject"`
	CostUSD        float64         `json:"costUsd"`
}

type ApproveReviewRequest struct {
	Comment string `json:"comment"`
}

type RejectReviewRequest struct {
	Comment string `json:"comment"`
	Reason  string `json:"reason" validate:"required"`
}

func (r *Review) ToDTO() ReviewDTO {
	return ReviewDTO{
		ID:             r.ID.String(),
		TaskID:         r.TaskID.String(),
		PRURL:          r.PRURL,
		PRNumber:       r.PRNumber,
		Layer:          r.Layer,
		Status:         r.Status,
		RiskLevel:      r.RiskLevel,
		Findings:       r.Findings,
		Summary:        r.Summary,
		Recommendation: r.Recommendation,
		CostUSD:        r.CostUSD,
		CreatedAt:      r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      r.UpdatedAt.Format(time.RFC3339),
	}
}
