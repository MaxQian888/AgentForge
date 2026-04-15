package nodetypes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// LoopDefResolver loads workflow definitions to enable topology-based node
// reset. Callers must supply an implementation that satisfies this contract;
// the loop handler needs the workflow's node+edge lists to compute which
// nodes to reset on iteration.
type LoopDefResolver interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowDefinition, error)
}

// LoopHandler implements the "loop" node type using the effect-based
// architecture. On iteration it emits a single EffectResetNodes effect whose
// payload carries the node IDs to reset and the counter key/value to persist.
// The handler is pure — it does not mutate the DataStore; the applier is
// responsible for writing the counter back.
//
// DefRepo is REQUIRED. Execute returns an error if DefRepo is nil, because
// the handler cannot compute the reset set without the workflow definition.
type LoopHandler struct {
	DefRepo LoopDefResolver
}

// Execute evaluates loop state and emits a reset-nodes effect, or exits with
// no effects when max iterations or the exit condition are reached.
func (h LoopHandler) Execute(ctx context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	if h.DefRepo == nil {
		return nil, fmt.Errorf("loop handler requires a DefRepo to resolve the workflow definition")
	}
	if req == nil || req.Execution == nil || req.Node == nil {
		return nil, fmt.Errorf("loop handler requires execution and node in the request")
	}

	targetNode, _ := req.Config["target_node"].(string)
	if targetNode == "" {
		return nil, fmt.Errorf("loop node %s missing target_node config", req.Node.ID)
	}

	maxIterations := 3
	if m, ok := req.Config["max_iterations"].(float64); ok && m > 0 {
		maxIterations = int(m)
	}
	exitCondition, _ := req.Config["exit_condition"].(string)

	counterKey := "_loop_" + req.Node.ID + "_count"
	currentIter := 0
	if req.DataStore != nil {
		if v, ok := req.DataStore[counterKey]; ok {
			if f, ok := v.(float64); ok {
				currentIter = int(f)
			}
		}
	}

	// Exit on max iterations.
	if currentIter >= maxIterations {
		return &NodeExecResult{}, nil
	}

	// Exit on exit_condition. We pass nil TaskRepo because loop exit
	// conditions are not expected to reference task.status; if a future
	// need arises, this handler can gain a TaskRepo field.
	if exitCondition != "" && EvaluateCondition(ctx, req.Execution, exitCondition, req.DataStore, nil) {
		return &NodeExecResult{}, nil
	}

	// Load workflow definition to compute topology.
	def, err := h.DefRepo.GetByID(ctx, req.Execution.WorkflowID)
	if err != nil {
		return nil, fmt.Errorf("load definition for loop: %w", err)
	}

	var allNodes []model.WorkflowNode
	var allEdges []model.WorkflowEdge
	_ = json.Unmarshal(def.Nodes, &allNodes)
	_ = json.Unmarshal(def.Edges, &allEdges)

	nodesToReset := FindNodesBetween(targetNode, req.Node.ID, allNodes, allEdges)
	nodesToReset = append(nodesToReset, targetNode)

	nextIter := currentIter + 1
	payload, _ := json.Marshal(ResetNodesPayload{
		NodeIDs:      nodesToReset,
		CounterKey:   counterKey,
		CounterValue: float64(nextIter),
	})
	return &NodeExecResult{
		Effects: []Effect{{Kind: EffectResetNodes, Payload: payload}},
	}, nil
}

// ConfigSchema describes the loop node configuration.
func (LoopHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "target_node": {"type": "string"},
    "max_iterations": {"type": "number", "minimum": 1},
    "exit_condition": {"type": "string"}
  },
  "required": ["target_node"]
}`)
}

// Capabilities declares the reset_nodes effect this handler emits.
func (LoopHandler) Capabilities() []EffectKind {
	return []EffectKind{EffectResetNodes}
}
