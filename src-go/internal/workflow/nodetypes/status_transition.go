package nodetypes

import (
	"context"
	"encoding/json"
	"fmt"
)

// StatusTransitionHandler implements the "status_transition" node type. It
// emits a single EffectUpdateTaskStatus effect that instructs the applier to
// transition the execution's task to the configured target status.
//
// The handler does not carry a task repo; all task-mutation side effects are
// deferred to the applier per the effect-based architecture.
type StatusTransitionHandler struct{}

// Execute validates that the execution carries a task and that a non-empty
// targetStatus is configured, then emits the effect.
func (StatusTransitionHandler) Execute(_ context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	if req == nil || req.Execution == nil || req.Execution.TaskID == nil {
		return nil, fmt.Errorf("status_transition requires a task ID in the execution context")
	}
	target, ok := req.Config["targetStatus"].(string)
	if !ok || target == "" {
		nodeID := ""
		if req.Node != nil {
			nodeID = req.Node.ID
		}
		return nil, fmt.Errorf("status_transition node %s missing targetStatus config", nodeID)
	}

	payload, _ := json.Marshal(UpdateTaskStatusPayload{TargetStatus: target})
	return &NodeExecResult{
		Effects: []Effect{{Kind: EffectUpdateTaskStatus, Payload: payload}},
	}, nil
}

// ConfigSchema describes the status_transition node configuration.
func (StatusTransitionHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "targetStatus": {"type": "string"}
  },
  "required": ["targetStatus"]
}`)
}

// Capabilities declares the update_task_status effect this handler emits.
func (StatusTransitionHandler) Capabilities() []EffectKind {
	return []EffectKind{EffectUpdateTaskStatus}
}
