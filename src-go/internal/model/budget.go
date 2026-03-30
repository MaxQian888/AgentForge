package model

type BudgetThresholdStatus string

const (
	BudgetThresholdInactive BudgetThresholdStatus = "inactive"
	BudgetThresholdHealthy  BudgetThresholdStatus = "healthy"
	BudgetThresholdWarning  BudgetThresholdStatus = "warning"
	BudgetThresholdExceeded BudgetThresholdStatus = "exceeded"
)

type BudgetScopeSummary struct {
	Scope                   string                `json:"scope"`
	Allocated               float64               `json:"allocated"`
	Spent                   float64               `json:"spent"`
	Remaining               float64               `json:"remaining"`
	ThresholdStatus         BudgetThresholdStatus `json:"thresholdStatus"`
	WarningThresholdPercent float64               `json:"warningThresholdPercent"`
	ItemCount               int                   `json:"itemCount,omitempty"`
}

type ProjectBudgetSummary struct {
	ProjectID               string                `json:"projectId"`
	Allocated               float64               `json:"allocated"`
	Spent                   float64               `json:"spent"`
	Remaining               float64               `json:"remaining"`
	ThresholdStatus         BudgetThresholdStatus `json:"thresholdStatus"`
	WarningThresholdPercent float64               `json:"warningThresholdPercent"`
	ActiveSprintCount       int                   `json:"activeSprintCount"`
	TasksWithBudgetCount    int                   `json:"tasksWithBudgetCount"`
	TasksAtRiskCount        int                   `json:"tasksAtRiskCount"`
	TasksExceededCount      int                   `json:"tasksExceededCount"`
	Scopes                  []BudgetScopeSummary  `json:"scopes"`
}

type SprintBudgetTaskEntry struct {
	TaskID          string                `json:"taskId"`
	Title           string                `json:"title"`
	Allocated       float64               `json:"allocated"`
	Spent           float64               `json:"spent"`
	Remaining       float64               `json:"remaining"`
	ThresholdStatus BudgetThresholdStatus `json:"thresholdStatus"`
}

type SprintBudgetDetail struct {
	SprintID                string                  `json:"sprintId"`
	ProjectID               string                  `json:"projectId"`
	SprintName              string                  `json:"sprintName"`
	Allocated               float64                 `json:"allocated"`
	Spent                   float64                 `json:"spent"`
	Remaining               float64                 `json:"remaining"`
	ThresholdStatus         BudgetThresholdStatus   `json:"thresholdStatus"`
	WarningThresholdPercent float64                 `json:"warningThresholdPercent"`
	TasksWithBudgetCount    int                     `json:"tasksWithBudgetCount"`
	Tasks                   []SprintBudgetTaskEntry `json:"tasks"`
}
