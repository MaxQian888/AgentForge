package nodetypes

import (
	"context"
	"encoding/json"
)

// FunctionHandler implements the "function" node type. It evaluates an
// expression (or passes through an input) against the request DataStore and
// returns the result as a JSON-serialized value. It emits no effects.
type FunctionHandler struct{}

// Execute evaluates config.expression against req.DataStore. Behavior:
//   - empty expression + config.input present  → marshal the input as JSON
//   - empty expression + no input              → return JSON null
//   - non-empty expression                     → resolve template vars, then
//     return the resolved string as-is when it is valid JSON, otherwise
//     EvaluateExpression and marshal the result.
func (FunctionHandler) Execute(_ context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	var (
		config    map[string]any
		dataStore map[string]any
	)
	if req != nil {
		config = req.Config
		dataStore = req.DataStore
	}

	expression, _ := config["expression"].(string)
	if expression == "" {
		if input, ok := config["input"]; ok {
			b, _ := json.Marshal(input)
			return &NodeExecResult{Result: b}, nil
		}
		return &NodeExecResult{Result: json.RawMessage("null")}, nil
	}

	resolved := ResolveTemplateVars(expression, dataStore)

	if json.Valid([]byte(resolved)) {
		return &NodeExecResult{Result: json.RawMessage(resolved)}, nil
	}

	val := EvaluateExpression(resolved, dataStore)
	b, _ := json.Marshal(val)
	return &NodeExecResult{Result: b}, nil
}

// ConfigSchema describes the function node configuration.
func (FunctionHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "expression": {"type": "string"},
    "input": {}
  }
}`)
}

// Capabilities returns nil; function handlers emit no effects.
func (FunctionHandler) Capabilities() []EffectKind { return nil }
