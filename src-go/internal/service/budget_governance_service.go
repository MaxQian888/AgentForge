package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
)

// BudgetSprintReader reads sprint data for budget governance checks.
type BudgetSprintReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Sprint, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Sprint, error)
}

type BudgetTaskReader interface {
	List(ctx context.Context, projectID uuid.UUID, q model.TaskListQuery) ([]*model.Task, int, error)
	ListBySprint(ctx context.Context, projectID uuid.UUID, sprintID uuid.UUID) ([]*model.Task, error)
}

// BudgetCheckResult represents the outcome of a budget governance check.
type BudgetCheckResult struct {
	Allowed        bool   `json:"allowed"`
	Warning        bool   `json:"warning"`
	Scope          string `json:"scope,omitempty"`
	WarningMessage string `json:"warningMessage,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

// BudgetGovernanceService enforces budget limits at the sprint and project level.
type BudgetGovernanceService struct {
	sprintReader       BudgetSprintReader
	taskReader         BudgetTaskReader
	projectBudgetLimit float64 // optional project-level cap; 0 means no cap
	automation         AutomationEventEvaluator
}

// NewBudgetGovernanceService creates a new BudgetGovernanceService.
func NewBudgetGovernanceService(sprintReader BudgetSprintReader, taskReaders ...BudgetTaskReader) *BudgetGovernanceService {
	var taskReader BudgetTaskReader
	if len(taskReaders) > 0 {
		taskReader = taskReaders[0]
	}
	return &BudgetGovernanceService{sprintReader: sprintReader, taskReader: taskReader}
}

// SetProjectBudgetLimit sets an optional project-level budget cap in USD.
func (s *BudgetGovernanceService) SetProjectBudgetLimit(limit float64) {
	s.projectBudgetLimit = limit
}

func (s *BudgetGovernanceService) SetAutomationEvaluator(evaluator AutomationEventEvaluator) {
	s.automation = evaluator
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
			Scope:   "sprint",
			Reason: fmt.Sprintf(
				"sprint budget exceeded: spent $%.2f + requested $%.2f = $%.2f > limit $%.2f",
				sprint.SpentUsd, requestedUsd, projectedTotal, sprint.TotalBudgetUsd,
			),
		}, nil
	}

	warningThreshold := sprint.TotalBudgetUsd * 0.80
	if projectedTotal >= warningThreshold {
		if s.automation != nil {
			_ = s.automation.EvaluateRules(ctx, AutomationEvent{
				EventType: model.AutomationEventBudgetThresholdReached,
				ProjectID: sprint.ProjectID,
				Data: map[string]any{
					"threshold_percentage": 80,
					"budget_percent":       (projectedTotal / sprint.TotalBudgetUsd) * 100,
					"spent_usd":            projectedTotal,
					"budget_usd":           sprint.TotalBudgetUsd,
				},
			})
		}
		return &BudgetCheckResult{
			Allowed: true,
			Warning: true,
			Scope:   "sprint",
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
			Scope:   "project",
			Reason: fmt.Sprintf(
				"project budget exceeded: spent $%.2f + requested $%.2f = $%.2f > limit $%.2f",
				totalSpent, requestedUsd, projectedTotal, s.projectBudgetLimit,
			),
		}, nil
	}

	warningThreshold := s.projectBudgetLimit * 0.80
	if projectedTotal >= warningThreshold {
		if s.automation != nil {
			_ = s.automation.EvaluateRules(ctx, AutomationEvent{
				EventType: model.AutomationEventBudgetThresholdReached,
				ProjectID: projectID,
				Data: map[string]any{
					"threshold_percentage": 80,
					"budget_percent":       (projectedTotal / s.projectBudgetLimit) * 100,
					"spent_usd":            projectedTotal,
					"budget_usd":           s.projectBudgetLimit,
				},
			})
		}
		return &BudgetCheckResult{
			Allowed: true,
			Warning: true,
			Scope:   "project",
			WarningMessage: fmt.Sprintf(
				"project budget warning: projected $%.2f / $%.2f (%.0f%% utilized)",
				projectedTotal, s.projectBudgetLimit, (projectedTotal/s.projectBudgetLimit)*100,
			),
		}, nil
	}

	return &BudgetCheckResult{Allowed: true}, nil
}

func (s *BudgetGovernanceService) CheckBudget(ctx context.Context, projectID uuid.UUID, sprintID *uuid.UUID, requestedUsd float64) (*BudgetCheckResult, error) {
	var warning *BudgetCheckResult
	if sprintID != nil {
		result, err := s.CheckSprintBudget(ctx, *sprintID, requestedUsd)
		if err != nil {
			return nil, err
		}
		if result != nil && !result.Allowed {
			return result, nil
		}
		if result != nil && result.Warning {
			warning = result
		}
	}
	result, err := s.CheckProjectBudget(ctx, projectID, requestedUsd)
	if err != nil {
		return nil, err
	}
	if result != nil && (!result.Allowed || result.Warning) {
		return result, nil
	}
	if warning != nil {
		return warning, nil
	}
	return result, nil
}

var ErrBudgetSprintNotFound = errors.New("budget sprint not found")

func (s *BudgetGovernanceService) GetProjectBudgetSummary(ctx context.Context, projectID uuid.UUID) (*model.ProjectBudgetSummary, error) {
	sprints, err := s.sprintReader.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("project budget summary: list sprints: %w", err)
	}

	tasks, err := s.listProjectTasks(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("project budget summary: list tasks: %w", err)
	}

	projectSpent := 0.0
	activeSprintAllocated := 0.0
	activeSprintSpent := 0.0
	activeSprintCount := 0
	for _, sprint := range sprints {
		if sprint == nil {
			continue
		}
		projectSpent += sprint.SpentUsd
		if sprint.Status == model.SprintStatusActive {
			activeSprintAllocated += sprint.TotalBudgetUsd
			activeSprintSpent += sprint.SpentUsd
			activeSprintCount++
		}
	}

	taskAllocated := 0.0
	taskSpent := 0.0
	tasksWithBudgetCount := 0
	tasksAtRiskCount := 0
	tasksExceededCount := 0
	for _, task := range tasks {
		if task == nil || task.BudgetUsd <= 0 {
			continue
		}
		tasksWithBudgetCount++
		taskAllocated += task.BudgetUsd
		taskSpent += task.SpentUsd
		switch budgetThresholdStatus(task.BudgetUsd, task.SpentUsd) {
		case model.BudgetThresholdExceeded:
			tasksExceededCount++
			tasksAtRiskCount++
		case model.BudgetThresholdWarning:
			tasksAtRiskCount++
		}
	}

	scopes := make([]model.BudgetScopeSummary, 0, 3)
	if projectScope := buildBudgetScope("project", s.projectBudgetLimit, projectSpent, 0); projectScope != nil {
		scopes = append(scopes, *projectScope)
	}
	if activeSprintScope := buildBudgetScope("active_sprints", activeSprintAllocated, activeSprintSpent, activeSprintCount); activeSprintScope != nil {
		scopes = append(scopes, *activeSprintScope)
	}
	if taskScope := buildBudgetScope("tasks", taskAllocated, taskSpent, tasksWithBudgetCount); taskScope != nil {
		scopes = append(scopes, *taskScope)
	}

	allocated, spent, remaining, threshold := projectBudgetHeadline(scopes)

	return &model.ProjectBudgetSummary{
		ProjectID:               projectID.String(),
		Allocated:               allocated,
		Spent:                   spent,
		Remaining:               remaining,
		ThresholdStatus:         threshold,
		WarningThresholdPercent: 80,
		ActiveSprintCount:       activeSprintCount,
		TasksWithBudgetCount:    tasksWithBudgetCount,
		TasksAtRiskCount:        tasksAtRiskCount,
		TasksExceededCount:      tasksExceededCount,
		Scopes:                  scopes,
	}, nil
}

func (s *BudgetGovernanceService) GetSprintBudgetDetail(ctx context.Context, sprintID uuid.UUID) (*model.SprintBudgetDetail, error) {
	sprint, err := s.sprintReader.GetByID(ctx, sprintID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrBudgetSprintNotFound
		}
		return nil, fmt.Errorf("sprint budget detail: fetch sprint: %w", err)
	}

	tasks, err := s.listSprintTasks(ctx, sprint.ProjectID, sprintID)
	if err != nil {
		return nil, fmt.Errorf("sprint budget detail: list tasks: %w", err)
	}

	entries := make([]model.SprintBudgetTaskEntry, 0, len(tasks))
	tasksWithBudgetCount := 0
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if task.BudgetUsd > 0 {
			tasksWithBudgetCount++
		}
		entries = append(entries, model.SprintBudgetTaskEntry{
			TaskID:          task.ID.String(),
			Title:           task.Title,
			Allocated:       roundBudgetValue(task.BudgetUsd),
			Spent:           roundBudgetValue(task.SpentUsd),
			Remaining:       roundBudgetValue(task.BudgetUsd - task.SpentUsd),
			ThresholdStatus: budgetThresholdStatus(task.BudgetUsd, task.SpentUsd),
		})
	}

	allocated := roundBudgetValue(sprint.TotalBudgetUsd)
	spent := roundBudgetValue(sprint.SpentUsd)
	remaining := 0.0
	threshold := model.BudgetThresholdInactive
	if allocated > 0 {
		remaining = roundBudgetValue(allocated - spent)
		threshold = budgetThresholdStatus(allocated, spent)
	}

	return &model.SprintBudgetDetail{
		SprintID:                sprint.ID.String(),
		ProjectID:               sprint.ProjectID.String(),
		SprintName:              sprint.Name,
		Allocated:               allocated,
		Spent:                   spent,
		Remaining:               remaining,
		ThresholdStatus:         threshold,
		WarningThresholdPercent: 80,
		TasksWithBudgetCount:    tasksWithBudgetCount,
		Tasks:                   entries,
	}, nil
}

func (s *BudgetGovernanceService) listProjectTasks(ctx context.Context, projectID uuid.UUID) ([]*model.Task, error) {
	if s.taskReader == nil {
		return nil, nil
	}

	const pageSize = 200
	page := 1
	tasks := make([]*model.Task, 0)
	for {
		items, total, err := s.taskReader.List(ctx, projectID, model.TaskListQuery{Page: page, Limit: pageSize})
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, items...)
		if len(tasks) >= total || len(items) == 0 {
			return tasks, nil
		}
		page++
	}
}

func (s *BudgetGovernanceService) listSprintTasks(ctx context.Context, projectID uuid.UUID, sprintID uuid.UUID) ([]*model.Task, error) {
	if s.taskReader == nil {
		return nil, nil
	}
	return s.taskReader.ListBySprint(ctx, projectID, sprintID)
}

func buildBudgetScope(scope string, allocated float64, spent float64, itemCount int) *model.BudgetScopeSummary {
	if allocated <= 0 {
		return nil
	}
	return &model.BudgetScopeSummary{
		Scope:                   scope,
		Allocated:               roundBudgetValue(allocated),
		Spent:                   roundBudgetValue(spent),
		Remaining:               roundBudgetValue(allocated - spent),
		ThresholdStatus:         budgetThresholdStatus(allocated, spent),
		WarningThresholdPercent: 80,
		ItemCount:               itemCount,
	}
}

func projectBudgetHeadline(scopes []model.BudgetScopeSummary) (float64, float64, float64, model.BudgetThresholdStatus) {
	if len(scopes) == 0 {
		return 0, 0, 0, model.BudgetThresholdInactive
	}

	selected := scopes[0]
	for _, candidate := range scopes {
		if candidate.Scope == "project" {
			selected = candidate
			break
		}
		if selected.Scope != "project" && candidate.Scope == "active_sprints" {
			selected = candidate
		}
	}

	threshold := model.BudgetThresholdHealthy
	for _, scope := range scopes {
		if scope.ThresholdStatus == model.BudgetThresholdExceeded {
			threshold = model.BudgetThresholdExceeded
			break
		}
		if scope.ThresholdStatus == model.BudgetThresholdWarning {
			threshold = model.BudgetThresholdWarning
		}
	}

	return selected.Allocated, selected.Spent, selected.Remaining, threshold
}

func budgetThresholdStatus(allocated float64, spent float64) model.BudgetThresholdStatus {
	if allocated <= 0 {
		return model.BudgetThresholdInactive
	}
	if spent > allocated {
		return model.BudgetThresholdExceeded
	}
	if (spent / allocated) >= 0.80-1e-9 {
		return model.BudgetThresholdWarning
	}
	return model.BudgetThresholdHealthy
}

func roundBudgetValue(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
