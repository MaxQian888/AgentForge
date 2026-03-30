package model

type CostPeriodRollupDTO struct {
	CostUsd         float64 `json:"costUsd"`
	InputTokens     int64   `json:"inputTokens"`
	OutputTokens    int64   `json:"outputTokens"`
	CacheReadTokens int64   `json:"cacheReadTokens"`
	Turns           int     `json:"turns"`
	RunCount        int     `json:"runCount"`
}

type SprintCostSummaryDTO struct {
	SprintID     string  `json:"sprintId"`
	SprintName   string  `json:"sprintName"`
	CostUsd      float64 `json:"costUsd"`
	BudgetUsd    float64 `json:"budgetUsd"`
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
}

type TaskCostDetailDTO struct {
	TaskID          string  `json:"taskId"`
	TaskTitle       string  `json:"taskTitle"`
	AgentRuns       int     `json:"agentRuns"`
	CostUsd         float64 `json:"costUsd"`
	InputTokens     int64   `json:"inputTokens"`
	OutputTokens    int64   `json:"outputTokens"`
	CacheReadTokens int64   `json:"cacheReadTokens"`
}

type ProjectCostSummaryDTO struct {
	TotalCostUsd         float64                        `json:"totalCostUsd"`
	TotalInputTokens     int64                          `json:"totalInputTokens"`
	TotalOutputTokens    int64                          `json:"totalOutputTokens"`
	TotalCacheReadTokens int64                          `json:"totalCacheReadTokens"`
	TotalTurns           int                            `json:"totalTurns"`
	RunCount             int                            `json:"runCount"`
	ActiveAgents         int                            `json:"activeAgents"`
	SprintCosts          []SprintCostSummaryDTO         `json:"sprintCosts"`
	TaskCosts            []TaskCostDetailDTO            `json:"taskCosts"`
	DailyCosts           []CostTimeSeriesDTO            `json:"dailyCosts"`
	BudgetSummary        *ProjectBudgetSummary          `json:"budgetSummary,omitempty"`
	PeriodRollups        map[string]CostPeriodRollupDTO `json:"periodRollups"`
}
