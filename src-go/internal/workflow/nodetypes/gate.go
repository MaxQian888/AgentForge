package nodetypes

import (
	"context"
	"encoding/json"
)

// GateHandler is the structural no-op handler for the "gate" node type.
// Branching is evaluated by the engine's edge-condition layer; the handler itself
// emits no effects.
type GateHandler struct{}

// Execute returns an empty result; gate evaluation happens outside the handler.
func (GateHandler) Execute(ctx context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	return &NodeExecResult{}, nil
}

// ConfigSchema returns nil; gate configuration lives on outgoing edges.
func (GateHandler) ConfigSchema() json.RawMessage { return nil }

// Capabilities returns nil; gate handlers emit no effects.
func (GateHandler) Capabilities() []EffectKind { return nil }
