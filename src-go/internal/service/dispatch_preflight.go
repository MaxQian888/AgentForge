package service

import (
	"context"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type DispatchRunReader interface {
	GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.AgentRun, error)
}

type DispatchPoolStatsProvider interface {
	PoolStats(ctx context.Context) model.AgentPoolStatsDTO
}

type DispatchPreflightResult struct {
	Outcome         model.DispatchOutcome
	AdmissionLikely bool
	PoolStats       *model.AgentPoolStatsDTO
}

func EvaluateDispatchPreflight(
	ctx context.Context,
	task *model.Task,
	member *model.Member,
	input DispatchSpawnInput,
	budgetChecker DispatchBudgetChecker,
	runs DispatchRunReader,
	pool DispatchPoolStatsProvider,
) DispatchPreflightResult {
	contextFields := dispatchOutcomeContextFromInput(input)
	result := DispatchPreflightResult{
		Outcome: contextFields,
	}

	switch {
	case task == nil:
		result.Outcome = applyDispatchOutcomeContext(model.DispatchOutcome{
			Status:         model.DispatchStatusBlocked,
			Reason:         "dispatch task is unavailable",
			GuardrailType:  model.DispatchGuardrailTypeTask,
			GuardrailScope: "task",
		}, contextFields)
		return result
	case member == nil:
		result.Outcome = applyDispatchOutcomeContext(model.DispatchOutcome{
			Status:         model.DispatchStatusBlocked,
			Reason:         "dispatch target is unavailable",
			GuardrailType:  model.DispatchGuardrailTypeTarget,
			GuardrailScope: "member",
		}, contextFields)
		return result
	case member.Type != model.MemberTypeAgent:
		result.Outcome = applyDispatchOutcomeContext(model.DispatchOutcome{
			Status: model.DispatchStatusSkipped,
			Reason: "dispatch target is a human member",
		}, contextFields)
		return result
	case member.ProjectID != task.ProjectID || !member.IsActive:
		result.Outcome = applyDispatchOutcomeContext(model.DispatchOutcome{
			Status:         model.DispatchStatusBlocked,
			Reason:         "dispatch target is not an active agent member",
			GuardrailType:  model.DispatchGuardrailTypeTarget,
			GuardrailScope: "member",
		}, contextFields)
		return result
	}

	if hasActiveDispatchRun(ctx, task.ID, runs) {
		result.Outcome = applyDispatchOutcomeContext(model.DispatchOutcome{
			Status:         model.DispatchStatusBlocked,
			Reason:         "task already has an active agent run",
			GuardrailType:  model.DispatchGuardrailTypeTask,
			GuardrailScope: "task",
		}, contextFields)
		return result
	}

	if warning, blocked := evaluateDispatchBudget(ctx, task, input.BudgetUSD, budgetChecker); blocked != nil {
		result.Outcome = applyDispatchOutcomeContext(*blocked, contextFields)
		return result
	} else if warning != nil {
		result.Outcome.BudgetWarning = warning
	}

	if pool != nil {
		stats := pool.PoolStats(ctx)
		result.PoolStats = &stats
		result.AdmissionLikely = stats.Available > 0
		if result.AdmissionLikely {
			result.Outcome.Status = model.DispatchStatusStarted
		} else {
			result.Outcome.Status = model.DispatchStatusQueued
			result.Outcome.Reason = "agent pool is at capacity"
			result.Outcome.GuardrailType = model.DispatchGuardrailTypePool
			result.Outcome.GuardrailScope = "project"
		}
		return result
	}

	result.AdmissionLikely = true
	result.Outcome.Status = model.DispatchStatusStarted
	return result
}

func hasActiveDispatchRun(ctx context.Context, taskID uuid.UUID, runs DispatchRunReader) bool {
	if runs == nil {
		return false
	}
	activeRuns, err := runs.GetByTask(ctx, taskID)
	if err != nil {
		return false
	}
	for _, run := range activeRuns {
		if run == nil {
			continue
		}
		switch run.Status {
		case model.AgentRunStatusStarting, model.AgentRunStatusRunning, model.AgentRunStatusPaused:
			return true
		}
	}
	return false
}

func evaluateDispatchBudget(ctx context.Context, task *model.Task, requestedUSD float64, budgetChecker DispatchBudgetChecker) (*model.DispatchBudgetWarning, *model.DispatchOutcome) {
	if task == nil {
		return nil, nil
	}

	var warning *model.DispatchBudgetWarning
	if taskWarning, taskBlocked := checkTaskBudget(task, requestedUSD); taskBlocked != nil {
		return nil, taskBlocked
	} else if taskWarning != nil {
		warning = taskWarning
	}

	if budgetChecker == nil {
		return warning, nil
	}
	result, err := budgetChecker.CheckBudget(ctx, task.ProjectID, task.SprintID, requestedUSD)
	if err != nil || result == nil {
		return warning, nil
	}
	scope := result.Scope
	if scope == "" {
		scope = inferBudgetScope(result.Reason + " " + result.WarningMessage)
	}
	if !result.Allowed {
		return nil, &model.DispatchOutcome{
			Status:         model.DispatchStatusBlocked,
			Reason:         result.Reason,
			GuardrailType:  model.DispatchGuardrailTypeBudget,
			GuardrailScope: scope,
		}
	}
	if result.Warning {
		return &model.DispatchBudgetWarning{
			Scope:   scope,
			Message: result.WarningMessage,
		}, nil
	}
	return warning, nil
}
