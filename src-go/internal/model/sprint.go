package model

import (
	"time"

	"github.com/google/uuid"
)

type Sprint struct {
	ID            uuid.UUID `db:"id"`
	ProjectID     uuid.UUID `db:"project_id"`
	Name          string    `db:"name"`
	StartDate     time.Time `db:"start_date"`
	EndDate       time.Time `db:"end_date"`
	Status        string    `db:"status"`
	TotalBudgetUsd float64  `db:"total_budget_usd"`
	SpentUsd      float64   `db:"spent_usd"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

const (
	SprintStatusPlanning = "planning"
	SprintStatusActive   = "active"
	SprintStatusClosed   = "closed"
)

type SprintDTO struct {
	ID             string  `json:"id"`
	ProjectID      string  `json:"projectId"`
	Name           string  `json:"name"`
	StartDate      string  `json:"startDate"`
	EndDate        string  `json:"endDate"`
	Status         string  `json:"status"`
	TotalBudgetUsd float64 `json:"totalBudgetUsd"`
	SpentUsd       float64 `json:"spentUsd"`
	CreatedAt      string  `json:"createdAt"`
}

type CreateSprintRequest struct {
	Name           string  `json:"name" validate:"required,min=1,max=100"`
	StartDate      string  `json:"startDate" validate:"required"`
	EndDate        string  `json:"endDate" validate:"required"`
	TotalBudgetUsd float64 `json:"totalBudgetUsd"`
}

type UpdateSprintRequest struct {
	Name           *string  `json:"name"`
	StartDate      *string  `json:"startDate"`
	EndDate        *string  `json:"endDate"`
	Status         *string  `json:"status"`
	TotalBudgetUsd *float64 `json:"totalBudgetUsd"`
}

func (s *Sprint) ToDTO() SprintDTO {
	return SprintDTO{
		ID:             s.ID.String(),
		ProjectID:      s.ProjectID.String(),
		Name:           s.Name,
		StartDate:      s.StartDate.Format(time.RFC3339),
		EndDate:        s.EndDate.Format(time.RFC3339),
		Status:         s.Status,
		TotalBudgetUsd: s.TotalBudgetUsd,
		SpentUsd:       s.SpentUsd,
		CreatedAt:      s.CreatedAt.Format(time.RFC3339),
	}
}
