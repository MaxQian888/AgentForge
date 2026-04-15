package model

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

type Sprint struct {
	ID             uuid.UUID  `db:"id"`
	ProjectID      uuid.UUID  `db:"project_id"`
	Name           string     `db:"name"`
	StartDate      time.Time  `db:"start_date"`
	EndDate        time.Time  `db:"end_date"`
	MilestoneID    *uuid.UUID `db:"milestone_id"`
	Status         string     `db:"status"`
	TotalBudgetUsd float64    `db:"total_budget_usd"`
	SpentUsd       float64    `db:"spent_usd"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
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
	MilestoneID    *string `json:"milestoneId,omitempty"`
	Status         string  `json:"status"`
	TotalBudgetUsd float64 `json:"totalBudgetUsd"`
	SpentUsd       float64 `json:"spentUsd"`
	CreatedAt      string  `json:"createdAt"`
}

type CreateSprintRequest struct {
	Name           string  `json:"name" validate:"required,min=1,max=100"`
	StartDate      string  `json:"startDate" validate:"required"`
	EndDate        string  `json:"endDate" validate:"required"`
	MilestoneID    *string `json:"milestoneId"`
	TotalBudgetUsd float64 `json:"totalBudgetUsd"`
}

type UpdateSprintRequest struct {
	Name           *string  `json:"name"`
	StartDate      *string  `json:"startDate"`
	EndDate        *string  `json:"endDate"`
	MilestoneID    *string  `json:"milestoneId"`
	Status         *string  `json:"status"`
	TotalBudgetUsd *float64 `json:"totalBudgetUsd"`
}

// validSprintTransitions defines allowed sprint status transitions.
var validSprintTransitions = map[string][]string{
	SprintStatusPlanning: {SprintStatusActive, SprintStatusClosed},
	SprintStatusActive:   {SprintStatusClosed},
}

// ValidateSprintTransition checks whether a sprint status transition is allowed.
func ValidateSprintTransition(from, to string) error {
	allowed, ok := validSprintTransitions[from]
	if !ok {
		return fmt.Errorf("cannot transition from status: %s", from)
	}
	for _, s := range allowed {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("invalid sprint transition from %s to %s", from, to)
}

type SprintBurndownPointDTO struct {
	Date           string `json:"date"`
	RemainingTasks int    `json:"remainingTasks"`
	CompletedTasks int    `json:"completedTasks"`
}

type SprintMetricsDTO struct {
	Sprint          SprintDTO                `json:"sprint"`
	PlannedTasks    int                      `json:"plannedTasks"`
	CompletedTasks  int                      `json:"completedTasks"`
	RemainingTasks  int                      `json:"remainingTasks"`
	CompletionRate  float64                  `json:"completionRate"`
	VelocityPerWeek float64                  `json:"velocityPerWeek"`
	TaskBudgetUsd   float64                  `json:"taskBudgetUsd"`
	TaskSpentUsd    float64                  `json:"taskSpentUsd"`
	Burndown        []SprintBurndownPointDTO `json:"burndown"`
}

func (s *Sprint) ToDTO() SprintDTO {
	dto := SprintDTO{
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
	if s.MilestoneID != nil {
		value := s.MilestoneID.String()
		dto.MilestoneID = &value
	}
	return dto
}

func BuildSprintMetricsDTO(sprint *Sprint, tasks []*Task, now time.Time) SprintMetricsDTO {
	if sprint == nil {
		return SprintMetricsDTO{}
	}

	plannedTasks := len(tasks)
	completedTasks := 0
	taskBudgetUsd := 0.0
	taskSpentUsd := 0.0

	for _, task := range tasks {
		if task == nil {
			continue
		}
		taskBudgetUsd += task.BudgetUsd
		taskSpentUsd += task.SpentUsd
		if taskCompletionTime(task) != nil {
			completedTasks++
		}
	}

	remainingTasks := plannedTasks - completedTasks
	completionRate := 0.0
	if plannedTasks > 0 {
		completionRate = roundTo2(float64(completedTasks) / float64(plannedTasks) * 100)
	}

	startDay := normalizeSprintDayStart(sprint.StartDate)
	endDay := normalizeSprintDayStart(sprint.EndDate)
	if endDay.Before(startDay) {
		endDay = startDay
	}

	elapsedEnd := normalizeSprintDayStart(now.UTC())
	if elapsedEnd.Before(startDay) {
		elapsedEnd = startDay
	}
	if elapsedEnd.After(endDay) {
		elapsedEnd = endDay
	}
	elapsedDays := int(elapsedEnd.Sub(startDay).Hours()/24) + 1
	if elapsedDays < 1 {
		elapsedDays = 1
	}

	velocityPerWeek := roundTo2(float64(completedTasks) * 7 / float64(elapsedDays))

	burndown := make([]SprintBurndownPointDTO, 0)
	for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		completedByDay := 0
		dayEnd := day.AddDate(0, 0, 1).Add(-time.Nanosecond)
		for _, task := range tasks {
			completedAt := taskCompletionTime(task)
			if completedAt != nil && !completedAt.After(dayEnd) {
				completedByDay++
			}
		}
		burndown = append(burndown, SprintBurndownPointDTO{
			Date:           day.Format("2006-01-02"),
			RemainingTasks: plannedTasks - completedByDay,
			CompletedTasks: completedByDay,
		})
	}

	return SprintMetricsDTO{
		Sprint:          sprint.ToDTO(),
		PlannedTasks:    plannedTasks,
		CompletedTasks:  completedTasks,
		RemainingTasks:  remainingTasks,
		CompletionRate:  completionRate,
		VelocityPerWeek: velocityPerWeek,
		TaskBudgetUsd:   roundTo2(taskBudgetUsd),
		TaskSpentUsd:    roundTo2(taskSpentUsd),
		Burndown:        burndown,
	}
}

func normalizeSprintDayStart(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func taskCompletionTime(task *Task) *time.Time {
	if task == nil {
		return nil
	}
	if task.CompletedAt != nil {
		completedAt := task.CompletedAt.UTC()
		return &completedAt
	}
	if task.Status == TaskStatusDone {
		updatedAt := task.UpdatedAt.UTC()
		return &updatedAt
	}
	return nil
}

func roundTo2(value float64) float64 {
	return math.Round(value*100) / 100
}
