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
	EffectExecuteHTTPCall   EffectKind = "execute_http_call"
	EffectExecuteIMSend     EffectKind = "execute_im_send"
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
	Runtime    string  `json:"runtime"`
	Provider   string  `json:"provider"`
	Model      string  `json:"model"`
	RoleID     string  `json:"roleId"`
	MemberID   string  `json:"memberId,omitempty"`
	EmployeeID string  `json:"employeeId,omitempty"`
	BudgetUsd  float64 `json:"budgetUsd"`
}

type RequestReviewPayload struct {
	Prompt  string          `json:"prompt"`
	Context json.RawMessage `json:"context,omitempty"`
}

type WaitEventPayload struct {
	EventType string `json:"eventType"`
	MatchKey  string `json:"matchKey,omitempty"`
}

// SubWorkflowTargetKind is the closed enumeration of target engines a
// sub_workflow node may invoke. Must stay in sync with model.TriggerTargetKind
// values, because the applier looks engines up through the same target-engine
// registry the trigger router uses.
type SubWorkflowTargetKind string

const (
	// SubWorkflowTargetDAG invokes another DAG workflow definition as a child.
	SubWorkflowTargetDAG SubWorkflowTargetKind = "dag"
	// SubWorkflowTargetPlugin invokes a legacy workflow plugin as a child.
	SubWorkflowTargetPlugin SubWorkflowTargetKind = "plugin"
)

// InvokeSubWorkflowPayload carries everything the applier needs to dispatch a
// child workflow run. TargetKind picks the engine (defaults to "dag" when
// empty). TargetWorkflowID resolves through the engine's own registry —
// a UUID string for DAG, a plugin id for the plugin engine. InputMapping is
// a templated JSON object evaluated against the parent run context; its
// rendered values become the child's initial seed. WaitForCompletion is
// reserved for a future fire-and-forget mode; initial delivery always
// treats it as true.
type InvokeSubWorkflowPayload struct {
	// WorkflowID is retained for back-compat with handlers that already emit
	// the old payload shape. New handlers should prefer TargetWorkflowID.
	// When both are set, TargetWorkflowID wins.
	WorkflowID        string                `json:"workflowId,omitempty"`
	TargetKind        SubWorkflowTargetKind `json:"targetKind,omitempty"`
	TargetWorkflowID  string                `json:"targetWorkflowId,omitempty"`
	InputMapping      json.RawMessage       `json:"inputMapping,omitempty"`
	WaitForCompletion bool                  `json:"waitForCompletion,omitempty"`
	Variables         json.RawMessage       `json:"variables,omitempty"`
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

// ExecuteHTTPCallPayload carries everything the applier needs to dial an
// external HTTP endpoint. Templates like {{secrets.X}} are resolved by the
// applier at execution time via SecretResolver.
type ExecuteHTTPCallPayload struct {
	Method         string            `json:"method"`
	URL            string            `json:"url"`
	Headers        map[string]string `json:"headers,omitempty"`
	URLQuery       map[string]string `json:"urlQuery,omitempty"`
	Body           string            `json:"body,omitempty"`
	TimeoutSeconds int               `json:"timeoutSeconds"`
	TreatAsSuccess []int             `json:"treatAsSuccess,omitempty"`
	// ProjectID is duplicated into the payload so the applier can resolve
	// {{secrets.X}} templates without reaching back into the execution.
	ProjectID string `json:"projectId"`
}

// ExecuteIMSendPayload carries the templated card and dispatch parameters.
type ExecuteIMSendPayload struct {
	// RawCard is the templated ProviderNeutralCard JSON (camelCase tags
	// matching src-im-bridge/core/card_schema.ts). The applier renders
	// correlation tokens for each callback action before dispatching.
	RawCard       json.RawMessage `json:"rawCard"`
	Target        string          `json:"target"` // "reply_to_trigger" | "explicit"
	ExplicitChat  *IMSendExplicit `json:"explicit,omitempty"`
	TokenLifetime string          `json:"tokenLifetime,omitempty"` // Go duration; default 168h (7d)
}

type IMSendExplicit struct {
	Provider string `json:"provider"`
	ChatID   string `json:"chatId"`
	ThreadID string `json:"threadId,omitempty"`
}
