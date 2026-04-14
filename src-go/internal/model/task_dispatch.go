package model

const (
	DispatchStatusStarted = "started"
	DispatchStatusQueued  = "queued"
	DispatchStatusBlocked = "blocked"
	DispatchStatusSkipped = "skipped"
)

const (
	DispatchGuardrailTypeBudget = "budget"
	DispatchGuardrailTypePool   = "pool"
	DispatchGuardrailTypeTarget = "target"
	DispatchGuardrailTypeTask   = "task"
	DispatchGuardrailTypeSystem = "system"
)

type DispatchBudgetWarning struct {
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

type DispatchOutcome struct {
	Status         string                 `json:"status"`
	Reason         string                 `json:"reason,omitempty"`
	Runtime        string                 `json:"runtime,omitempty"`
	Provider       string                 `json:"provider,omitempty"`
	Model          string                 `json:"model,omitempty"`
	RoleID         string                 `json:"roleId,omitempty"`
	GuardrailType  string                 `json:"guardrailType,omitempty"`
	GuardrailScope string                 `json:"guardrailScope,omitempty"`
	BudgetWarning  *DispatchBudgetWarning `json:"budgetWarning,omitempty"`
	Run            *AgentRunDTO           `json:"run,omitempty"`
	Queue          *AgentPoolQueueEntry   `json:"queue,omitempty"`
}

type TaskDispatchResponse struct {
	Task     TaskDTO         `json:"task"`
	Dispatch DispatchOutcome `json:"dispatch"`
}
