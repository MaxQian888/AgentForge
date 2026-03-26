package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// BudgetSprintReader reads sprint data for budget governance checks.
type BudgetSprintReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Sprint, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Sprint, error)
}

// BudgetCheckResult represents the outcome of a budget governance check.
type BudgetCheckResult struct {
	Allowed        bool    `json:"allowed"`
	Warning        bool    `json:"warning"`
	WarningMessage string  `json:"warningMessage,omitempty"`
	Reason         string  `json:"reason,omitempty"`
}

// BudgetGovernanceService enforces budget limits at the sprint and project level.
type BudgetGovernanceService struct {
	sprintReader       BudgetSprintReader
	projectBudgetLimit float64 // optional project-level cap; 0 means no cap
}

// NewBudgetGovernanceService creates a new BudgetGovernanceService.
func NewBudgetGovernanceService(sprintReader BudgetSprintReader) *BudgetGovernanceService {
	return &BudgetGovernanceService{
		sprintReader: sprintReader,
	}
}

// SetProjectBudgetLimit sets an optional project-level budget cap in USD.
func (s *BudgetGovernanceService) SetProjectBudgetLimit(limit float64) {
	s.projectBudgetLimit = limit
}

// CheckSprintBudget verifies that requestedUsd fits within the sprint's budget.
// It warns at 80% utilization and blocks at 100%.
func (s *BudgetGovernanceService) CheckSprintBudget(ctx context.Context, sprintID uuid.UUID, requestedUsd float64) (*BudgetCheckResult, error) {
	sprint, err := s.sprintReader.GetByID(ctx, sprintID)
	if err != nil {
		return nil, fmt.Errorf("budget check: fetch sprint: %w", err)
	}

	if sprint.TotalBudgetUsd <= 0 {
		// No budget configured; allow unconditionally.
		return &BudgetCheckResult{Allowed: true}, nil
	}

	projectedTotal := sprint.SpentUsd + requestedUsd

	if projectedTotal > sprint.TotalBudgetUsd {
		return &BudgetCheckResult{
			Allowed: false,
			Reason: fmt.Sprintf(
				"sprint budget exceeded: spent $%.2f + requested $%.2f = $%.2f > limit $%.2f",
				sprint.SpentUsd, requestedUsd, projectedTotal, sprint.TotalBudgetUsd,
			),
		}, nil
	}

	warningThreshold := sprint.TotalBudgetUsd * 0.80
	if projectedTotal >= warningThreshold {
		return &BudgetCheckResult{
			Allowed: true,
			Warning: true,
			WarningMessage: fmt.Sprintf(
				"sprint budget warning: projected $%.2f / $%.2f (%.0f%% utilized)",
				projectedTotal, sprint.TotalBudgetUsd, (projectedTotal/sprint.TotalBudgetUsd)*100,
			),
		}, nil
	}

	return &BudgetCheckResult{Allowed: true}, nil
}

// CheckProjectBudget checks total spend across all sprints in a project
// against the configured project budget limit.
func (s *BudgetGovernanceService) CheckProjectBudget(ctx context.Context, projectID uuid.UUID, requestedUsd float64) (*BudgetCheckResult, error) {
	if s.projectBudgetLimit <= 0 {
		// No project-level budget limit configured; allow unconditionally.
		return &BudgetCheckResult{Allowed: true}, nil
	}

	sprints, err := s.sprintReader.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("budget check: list project sprints: %w", err)
	}

	var totalSpent float64
	for _, sp := range sprints {
		totalSpent += sp.SpentUsd
	}

	projectedTotal := totalSpent + requestedUsd

	if projectedTotal > s.projectBudgetLimit {
		return &BudgetCheckResult{
			Allowed: false,
			Reason: fmt.Sprintf(
				"project budget exceeded: spent $%.2f + requested $%.2f = $%.2f > limit $%.2f",
				totalSpent, requestedUsd, projectedTotal, s.projectBudgetLimit,
			),
		}, nil
	}

	warningThreshold := s.projectBudgetLimit * 0.80
	if projectedTotal >= warningThreshold {
		return &BudgetCheckResult{
			Allowed: true,
			Warning: true,
			WarningMessage: fmt.Sprintf(
				"project budget warning: projected $%.2f / $%.2f (%.0f%% utilized)",
				projectedTotal, s.projectBudgetLimit, (projectedTotal/s.projectBudgetLimit)*100,
			),
		}, nil
	}

	return &BudgetCheckResult{Allowed: true}, nil
}
