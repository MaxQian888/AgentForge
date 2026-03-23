package model

import (
	"time"

	"github.com/google/uuid"
)

type Review struct {
	ID             uuid.UUID `db:"id"`
	TaskID         uuid.UUID `db:"task_id"`
	ReviewerID     uuid.UUID `db:"reviewer_id"`
	ReviewerType   string    `db:"reviewer_type"` // "human" or "agent"
	Status         string    `db:"status"`
	Recommendation string    `db:"recommendation"` // "approve", "request_changes", "reject"
	Summary        string    `db:"summary"`
	Comments       string    `db:"comments"` // JSON string
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

const (
	ReviewStatusPending   = "pending"
	ReviewStatusCompleted = "completed"

	ReviewRecommendationApprove        = "approve"
	ReviewRecommendationRequestChanges = "request_changes"
	ReviewRecommendationReject         = "reject"
)

type ReviewDTO struct {
	ID             string `json:"id"`
	TaskID         string `json:"taskId"`
	ReviewerID     string `json:"reviewerId"`
	ReviewerType   string `json:"reviewerType"`
	Status         string `json:"status"`
	Recommendation string `json:"recommendation"`
	Summary        string `json:"summary"`
	Comments       string `json:"comments"`
	CreatedAt      string `json:"createdAt"`
}

type CreateReviewRequest struct {
	ReviewerID   string `json:"reviewerId" validate:"required"`
	ReviewerType string `json:"reviewerType" validate:"required,oneof=human agent"`
}

func (r *Review) ToDTO() ReviewDTO {
	return ReviewDTO{
		ID:             r.ID.String(),
		TaskID:         r.TaskID.String(),
		ReviewerID:     r.ReviewerID.String(),
		ReviewerType:   r.ReviewerType,
		Status:         r.Status,
		Recommendation: r.Recommendation,
		Summary:        r.Summary,
		Comments:       r.Comments,
		CreatedAt:      r.CreatedAt.Format(time.RFC3339),
	}
}
