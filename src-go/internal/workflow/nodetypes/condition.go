package nodetypes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/react-go-quick-starter/server/internal/model"
)

// ConditionHandler resolves task.status via TaskRepo (may be nil for repo-less evaluation).
//
// This handler intentionally carries one field — TaskRepo — because condition
// expressions can reference `task.status` and need a way to load the current
// task state. All other state (execution, dataStore, config) flows through the
// NodeExecRequest, keeping the handler otherwise stateless.
type ConditionHandler struct {
	TaskRepo ConditionTaskResolver
}

// Execute evaluates config.expression. An empty expression passes through. A
// false-evaluating expression returns an error of the form "condition not met:
// <expression>", preserving the historical fail-on-false behavior of the
// service-layer implementation.
func (h ConditionHandler) Execute(ctx context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	var (
		config    map[string]any
		dataStore map[string]any
		exec      *model.WorkflowExecution
	)
	if req != nil {
		config = req.Config
		dataStore = req.DataStore
		exec = req.Execution
	}

	expression, _ := config["expression"].(string)
	if expression == "" {
		return &NodeExecResult{}, nil
	}

	if !EvaluateCondition(ctx, exec, expression, dataStore, h.TaskRepo) {
		return nil, fmt.Errorf("condition not met: %s", expression)
	}
	return &NodeExecResult{}, nil
}

// ConfigSchema describes the condition node configuration.
func (ConditionHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "expression": {"type": "string"}
  }
}`)
}

// Capabilities returns nil; condition handlers emit no effects.
func (ConditionHandler) Capabilities() []EffectKind { return nil }
