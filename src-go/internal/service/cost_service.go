package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	eventbus "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
)

// CostStats aggregates cost information for a task or project.
type CostStats struct {
	TotalCostUsd      float64 `json:"totalCostUsd"`
	TotalInputTokens  int64   `json:"totalInputTokens"`
	TotalOutputTokens int64   `json:"totalOutputTokens"`
	TotalTurns        int     `json:"totalTurns"`
	RunCount          int     `json:"runCount"`
}

type CostService struct {
	agentRunRepo  AgentRunRepository
	taskRepo      TaskRepository
	hub           *ws.Hub
	bus           eventbus.Publisher
	budgetWarnPct float64 // 0.8 = warn at 80%
	budgetKillPct float64 // 1.0 = kill at 100%
}

func NewCostService(agentRunRepo AgentRunRepository, taskRepo TaskRepository, hub *ws.Hub, bus eventbus.Publisher) *CostService {
	return &CostService{
		agentRunRepo:  agentRunRepo,
		taskRepo:      taskRepo,
		hub:           hub,
		bus:           bus,
		budgetWarnPct: 0.8,
		budgetKillPct: 1.0,
	}
}

// RecordCost updates cost on an agent run and checks budget thresholds.
func (s *CostService) RecordCost(ctx context.Context, runID uuid.UUID, inputTokens, outputTokens, cacheReadTokens int64, costUsd float64, turnCount int) error {
	if err := s.agentRunRepo.UpdateCost(ctx, runID, inputTokens, outputTokens, cacheReadTokens, costUsd, turnCount, nil); err != nil {
		return fmt.Errorf("record cost: %w", err)
	}

	// Check task budget.
	run, err := s.agentRunRepo.GetByID(ctx, runID)
	if err != nil {
		return err
	}

	task, err := s.taskRepo.GetByID(ctx, run.TaskID)
	if err != nil {
		return err
	}

	taskCost, err := s.GetTaskCost(ctx, run.TaskID)
	if err != nil {
		return err
	}

	if task.BudgetUsd > 0 {
		ratio := taskCost.TotalCostUsd / task.BudgetUsd

		if ratio >= s.budgetKillPct {
			_ = eventbus.PublishLegacy(ctx, s.bus, ws.EventBudgetExceeded, task.ProjectID.String(), map[string]any{
				"taskId": task.ID.String(),
				"budget": task.BudgetUsd,
				"spent":  taskCost.TotalCostUsd,
			})
		} else if ratio >= s.budgetWarnPct {
			_ = eventbus.PublishLegacy(ctx, s.bus, ws.EventBudgetWarning, task.ProjectID.String(), map[string]any{
				"taskId":  task.ID.String(),
				"budget":  task.BudgetUsd,
				"spent":   taskCost.TotalCostUsd,
				"percent": ratio * 100,
			})
		}
	}

	return nil
}

// GetTaskCost returns aggregated cost stats for a task.
func (s *CostService) GetTaskCost(ctx context.Context, taskID uuid.UUID) (*CostStats, error) {
	runs, err := s.agentRunRepo.GetByTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task runs: %w", err)
	}
	return aggregateCosts(runs), nil
}

// GetCostStats returns cost stats for all active runs.
func (s *CostService) GetCostStats(ctx context.Context) (*CostStats, error) {
	runs, err := s.agentRunRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active runs: %w", err)
	}
	return aggregateCosts(runs), nil
}

func aggregateCosts(runs []*model.AgentRun) *CostStats {
	stats := &CostStats{RunCount: len(runs)}
	for _, r := range runs {
		stats.TotalCostUsd += r.CostUsd
		stats.TotalInputTokens += r.InputTokens
		stats.TotalOutputTokens += r.OutputTokens
		stats.TotalTurns += r.TurnCount
	}
	return stats
}
