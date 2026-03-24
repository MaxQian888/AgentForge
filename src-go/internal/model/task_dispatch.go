package model

const (
	DispatchStatusStarted = "started"
	DispatchStatusBlocked = "blocked"
	DispatchStatusSkipped = "skipped"
)

type DispatchOutcome struct {
	Status string       `json:"status"`
	Reason string       `json:"reason,omitempty"`
	Run    *AgentRunDTO `json:"run,omitempty"`
}

type TaskDispatchResponse struct {
	Task     TaskDTO          `json:"task"`
	Dispatch DispatchOutcome  `json:"dispatch"`
}
