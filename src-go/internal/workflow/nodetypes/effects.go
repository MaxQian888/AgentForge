package nodetypes

import "encoding/json"

// EffectKind is the closed enumeration of effect types handlers may emit.
type EffectKind string

const (
	EffectSpawnAgent        EffectKind = "spawn_agent"
	EffectRequestReview     EffectKind = "request_review"
	EffectWaitEvent         EffectKind = "wait_event"
	EffectInvokeSubWorkflow EffectKind = "invoke_sub_workflow"
	EffectBroadcastEvent    EffectKind = "broadcast_event"
	EffectUpdateTaskStatus  EffectKind = "update_task_status"
	EffectResetNodes        EffectKind = "reset_nodes"
)

// IsPark reports whether the effect parks the node (node enters `waiting` state).
func (k EffectKind) IsPark() bool {
	switch k {
	case EffectSpawnAgent, EffectRequestReview, EffectWaitEvent, EffectInvokeSubWorkflow:
		return true
	default:
		return false
	}
}

// Effect is a single structured side-effect emitted by a handler.
type Effect struct {
	Kind    EffectKind
	Payload json.RawMessage
}

// Per-effect payload structs.

type SpawnAgentPayload struct {
	Runtime   string  `json:"runtime"`
	Provider  string  `json:"provider"`
	Model     string  `json:"model"`
	RoleID    string  `json:"roleId"`
	MemberID  string  `json:"memberId,omitempty"`
	BudgetUsd float64 `json:"budgetUsd"`
}

type RequestReviewPayload struct {
	Prompt  string          `json:"prompt"`
	Context json.RawMessage `json:"context,omitempty"`
}

type WaitEventPayload struct {
	EventType string `json:"eventType"`
	MatchKey  string `json:"matchKey,omitempty"`
}

type InvokeSubWorkflowPayload struct {
	WorkflowID string          `json:"workflowId"`
	Variables  json.RawMessage `json:"variables,omitempty"`
}

type BroadcastEventPayload struct {
	EventType string         `json:"eventType"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type UpdateTaskStatusPayload struct {
	TargetStatus string `json:"targetStatus"`
}

type ResetNodesPayload struct {
	NodeIDs      []string `json:"nodeIds"`
	CounterKey   string   `json:"counterKey,omitempty"`
	CounterValue float64  `json:"counterValue,omitempty"`
}
