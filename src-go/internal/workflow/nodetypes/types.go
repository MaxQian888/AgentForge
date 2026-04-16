// Package nodetypes provides the handler contract and registry for DAG workflow node types.
//
// experimental: pre-1.0, may change without notice
package nodetypes

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// NodeTypeHandler is the contract every built-in and plugin-contributed node type must satisfy.
type NodeTypeHandler interface {
	Execute(ctx context.Context, req *NodeExecRequest) (*NodeExecResult, error)
	ConfigSchema() json.RawMessage // return nil if not provided
	Capabilities() []EffectKind    // exhaustive set of effect kinds this handler may emit
}

// NodeExecRequest is the read-only input passed to a handler.
type NodeExecRequest struct {
	Execution  *model.WorkflowExecution
	Node       *model.WorkflowNode
	Config     map[string]any // already template-resolved by caller
	DataStore  map[string]any // handlers MUST NOT mutate
	NodeExecID uuid.UUID
	ProjectID  uuid.UUID
}

// NodeExecResult is what a handler returns on success.
type NodeExecResult struct {
	Result  json.RawMessage // nil → void; non-nil → written to DataStore under nodeID
	Effects []Effect
}

// ParkCount returns the number of park-and-await effects.
// A well-formed result has ParkCount() ∈ {0, 1}; >1 causes registry rejection.
func (r *NodeExecResult) ParkCount() int {
	n := 0
	for _, e := range r.Effects {
		if e.Kind.IsPark() {
			n++
		}
	}
	return n
}
