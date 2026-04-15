package nodetypes

import (
	"context"
	"encoding/json"
)

// TriggerHandler is the structural no-op handler for the "trigger" node type.
// It marks the entry point of a workflow and emits no effects.
type TriggerHandler struct{}

// Execute returns an empty result; triggers perform no work at execution time.
func (TriggerHandler) Execute(ctx context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	return &NodeExecResult{}, nil
}

// ConfigSchema returns nil; trigger nodes take no configuration at the handler layer.
func (TriggerHandler) ConfigSchema() json.RawMessage { return nil }

// Capabilities returns nil; trigger handlers emit no effects.
func (TriggerHandler) Capabilities() []EffectKind { return nil }
