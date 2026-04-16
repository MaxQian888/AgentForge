package nodetypes

import (
	"context"
	"encoding/json"
	"errors"
)

// SubWorkflowHandler is a fail-fast stub for the "sub_workflow" node type.
//
// Track A intentionally does not wire sub-workflow invocation. The handler is
// registered so that schemas and capability declarations stay in sync with the
// closed EffectKind enum, but Execute returns an error to prevent indefinite
// parking during development. A future track will replace Execute with a real
// implementation that emits an EffectInvokeSubWorkflow effect.
type SubWorkflowHandler struct{}

// Execute returns an error; sub_workflow is not implemented in Track A.
func (SubWorkflowHandler) Execute(ctx context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	return nil, errors.New("sub_workflow not implemented in Track A; slated for a future track")
}

// ConfigSchema describes the shape of sub_workflow node configuration so
// downstream tracks can validate definitions without schema drift.
func (SubWorkflowHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "workflowId": {"type": "string"},
    "variables": {"type": "object"}
  },
  "required": ["workflowId"]
}`)
}

// Capabilities declares the single effect kind a real implementation will emit.
// This stays declared even though Execute currently errors, so future wiring
// has zero schema drift.
func (SubWorkflowHandler) Capabilities() []EffectKind {
	return []EffectKind{EffectInvokeSubWorkflow}
}
