package nodetypes

import (
	"context"
	"encoding/json"
)

// ParallelJoinHandler is the structural no-op handler for the "parallel_join"
// node type. Join semantics (wait for N predecessors) are enforced by the engine,
// not the handler.
type ParallelJoinHandler struct{}

// Execute returns an empty result; join gating is the engine's responsibility.
func (ParallelJoinHandler) Execute(ctx context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	return &NodeExecResult{}, nil
}

// ConfigSchema returns nil; parallel_join takes no per-node configuration.
func (ParallelJoinHandler) ConfigSchema() json.RawMessage { return nil }

// Capabilities returns nil; parallel_join handlers emit no effects.
func (ParallelJoinHandler) Capabilities() []EffectKind { return nil }
