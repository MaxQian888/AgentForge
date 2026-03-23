// Package cost provides in-memory per-task cost tracking with budget thresholds.
package cost

import (
	"sync"
)

const (
	// BudgetWarnPercent is the threshold to emit a warning (80%).
	BudgetWarnPercent = 0.8
	// BudgetKillPercent is the threshold to kill the agent (100%).
	BudgetKillPercent = 1.0
)

// ThresholdAction indicates what action the caller should take.
type ThresholdAction int

const (
	ActionNone    ThresholdAction = iota
	ActionWarn                    // Budget >= 80%
	ActionKill                    // Budget >= 100%
)

// TaskCost tracks accumulated cost for a single task.
type TaskCost struct {
	InputTokens     int64
	OutputTokens    int64
	CacheReadTokens int64
	CostUsd         float64
	TurnCount       int
}

// Tracker provides thread-safe in-memory cost tracking per task.
type Tracker struct {
	mu    sync.RWMutex
	costs map[string]*TaskCost // keyed by task ID
}

// NewTracker creates a new cost tracker.
func NewTracker() *Tracker {
	return &Tracker{
		costs: make(map[string]*TaskCost),
	}
}

// Record adds cost data for a task and returns the threshold action based on budget.
func (t *Tracker) Record(taskID string, inputTokens, outputTokens, cacheReadTokens int64, costUsd float64, turnCount int, budgetUsd float64) ThresholdAction {
	t.mu.Lock()
	defer t.mu.Unlock()

	tc, ok := t.costs[taskID]
	if !ok {
		tc = &TaskCost{}
		t.costs[taskID] = tc
	}

	tc.InputTokens += inputTokens
	tc.OutputTokens += outputTokens
	tc.CacheReadTokens += cacheReadTokens
	tc.CostUsd += costUsd
	tc.TurnCount += turnCount

	if budgetUsd <= 0 {
		return ActionNone
	}

	ratio := tc.CostUsd / budgetUsd
	if ratio >= BudgetKillPercent {
		return ActionKill
	}
	if ratio >= BudgetWarnPercent {
		return ActionWarn
	}
	return ActionNone
}

// Get returns the current cost for a task. Returns zero values if not tracked.
func (t *Tracker) Get(taskID string) TaskCost {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if tc, ok := t.costs[taskID]; ok {
		return *tc
	}
	return TaskCost{}
}

// Reset clears cost tracking for a task.
func (t *Tracker) Reset(taskID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.costs, taskID)
}

// All returns a snapshot of all tracked costs.
func (t *Tracker) All() map[string]TaskCost {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]TaskCost, len(t.costs))
	for k, v := range t.costs {
		result[k] = *v
	}
	return result
}
