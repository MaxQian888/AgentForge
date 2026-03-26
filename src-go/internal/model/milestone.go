package model

import (
	"math"
	"time"

	"github.com/google/uuid"
)

const (
	MilestoneStatusPlanned    = "planned"
	MilestoneStatusInProgress = "in_progress"
	MilestoneStatusCompleted  = "completed"
	MilestoneStatusMissed     = "missed"
)

type Milestone struct {
	ID          uuid.UUID  `db:"id"`
	ProjectID   uuid.UUID  `db:"project_id"`
	Name        string     `db:"name"`
	TargetDate  *time.Time `db:"target_date"`
	Status      string     `db:"status"`
	Description string     `db:"description"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}

type MilestoneMetrics struct {
	TotalTasks     int     `json:"totalTasks"`
	CompletedTasks int     `json:"completedTasks"`
	TotalSprints   int     `json:"totalSprints"`
	CompletionRate float64 `json:"completionRate"`
}

type MilestoneDTO struct {
	ID          string            `json:"id"`
	ProjectID   string            `json:"projectId"`
	Name        string            `json:"name"`
	TargetDate  *string           `json:"targetDate,omitempty"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
	DeletedAt   *string           `json:"deletedAt,omitempty"`
	Metrics     *MilestoneMetrics `json:"metrics,omitempty"`
}

type CreateMilestoneRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=120"`
	TargetDate  *string `json:"targetDate"`
	Status      string  `json:"status"`
	Description string  `json:"description"`
}

type UpdateMilestoneRequest struct {
	Name        *string `json:"name"`
	TargetDate  *string `json:"targetDate"`
	Status      *string `json:"status"`
	Description *string `json:"description"`
}

func BuildMilestoneMetrics(totalTasks int, completedTasks int, totalSprints int) MilestoneMetrics {
	completionRate := 0.0
	if totalTasks > 0 {
		completionRate = math.Round((float64(completedTasks)/float64(totalTasks))*10000) / 100
	}
	return MilestoneMetrics{
		TotalTasks:     totalTasks,
		CompletedTasks: completedTasks,
		TotalSprints:   totalSprints,
		CompletionRate: completionRate,
	}
}

func (m *Milestone) ToDTO(metrics *MilestoneMetrics) MilestoneDTO {
	dto := MilestoneDTO{
		ID:          m.ID.String(),
		ProjectID:   m.ProjectID.String(),
		Name:        m.Name,
		Status:      m.Status,
		Description: m.Description,
		CreatedAt:   m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   m.UpdatedAt.Format(time.RFC3339),
		Metrics:     metrics,
	}
	if m.TargetDate != nil {
		value := m.TargetDate.Format(time.RFC3339)
		dto.TargetDate = &value
	}
	if m.DeletedAt != nil {
		value := m.DeletedAt.Format(time.RFC3339)
		dto.DeletedAt = &value
	}
	return dto
}
