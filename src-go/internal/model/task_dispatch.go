package model

const (
	DispatchStatusStarted = "started"
	DispatchStatusQueued  = "queued"
	DispatchStatusBlocked = "blocked"
	DispatchStatusSkipped = "skipped"
)

type DispatchOutcome struct {
	Status string               `json:"status"`
	Reason string               `json:"reason,omitempty"`
	Run    *AgentRunDTO         `json:"run,omitempty"`
	Queue  *AgentPoolQueueEntry `json:"queue,omitempty"`
}

type TaskDispatchResponse struct {
	Task     TaskDTO          `json:"task"`
	Dispatch DispatchOutcome  `json:"dispatch"`
}
