package nodetypes

import (
	"context"
	"encoding/json"
)

// ParallelSplitHandler is the structural no-op handler for the "parallel_split"
// node type. Fan-out is performed by the engine scheduler based on outgoing edges.
type ParallelSplitHandler struct{}

// Execute returns an empty result; fan-out is a scheduling concern, not a handler concern.
func (ParallelSplitHandler) Execute(ctx context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	return &NodeExecResult{}, nil
}

// ConfigSchema returns nil; parallel_split takes no per-node configuration.
func (ParallelSplitHandler) ConfigSchema() json.RawMessage { return nil }

// Capabilities returns nil; parallel_split handlers emit no effects.
func (ParallelSplitHandler) Capabilities() []EffectKind { return nil }
