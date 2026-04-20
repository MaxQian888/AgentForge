package nodetypes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// SubWorkflowHandler implements the `sub_workflow` node type. It emits a single
// EffectInvokeSubWorkflow effect whose payload names the target engine (DAG or
// legacy plugin) and the target workflow identifier; the applier performs the
// actual invocation and parks the node.
//
// The handler is deliberately thin: it validates the config shape, normalizes
// defaults (targetKind defaults to "dag"; waitForCompletion defaults to true),
// and serializes the payload. Recursion guards, same-project checks, and
// engine dispatch live in the applier so a bad config surfaces at execute
// time with a structured error rather than at dispatch time after state has
// already mutated.
type SubWorkflowHandler struct{}

// subWorkflowConfig is the typed shape accepted from a node's Config JSON.
// Both camelCase (DAG editor) and snake_case (plugin author) spellings are
// accepted so hand-authored workflows in either style work without touching
// upstream producers.
type subWorkflowConfig struct {
	// New-style fields.
	TargetKind        string          `json:"targetKind,omitempty"`
	TargetWorkflowID  string          `json:"targetWorkflowId,omitempty"`
	InputMapping      json.RawMessage `json:"inputMapping,omitempty"`
	WaitForCompletion *bool           `json:"waitForCompletion,omitempty"`

	// Legacy snake_case aliases (kept for hand-authored JSON).
	TargetKindSnake       string          `json:"target_kind,omitempty"`
	TargetWorkflowIDSnake string          `json:"target_workflow_id,omitempty"`
	InputMappingSnake     json.RawMessage `json:"input_mapping,omitempty"`
	WaitSnake             *bool           `json:"wait_for_completion,omitempty"`

	// Original payload field retained for back-compat with handlers that
	// already emit the old shape.
	WorkflowID string          `json:"workflowId,omitempty"`
	Variables  json.RawMessage `json:"variables,omitempty"`
}

func (c *subWorkflowConfig) normalized() (kind SubWorkflowTargetKind, target string, mapping json.RawMessage, wait bool, variables json.RawMessage) {
	rawKind := c.TargetKind
	if rawKind == "" {
		rawKind = c.TargetKindSnake
	}
	rawKind = strings.ToLower(strings.TrimSpace(rawKind))
	if rawKind == "" {
		rawKind = string(SubWorkflowTargetDAG)
	}
	kind = SubWorkflowTargetKind(rawKind)

	target = strings.TrimSpace(c.TargetWorkflowID)
	if target == "" {
		target = strings.TrimSpace(c.TargetWorkflowIDSnake)
	}
	if target == "" {
		target = strings.TrimSpace(c.WorkflowID)
	}

	mapping = c.InputMapping
	if len(mapping) == 0 {
		mapping = c.InputMappingSnake
	}

	wait = true // default true; reserve false for future fire-and-forget.
	if c.WaitForCompletion != nil {
		wait = *c.WaitForCompletion
	} else if c.WaitSnake != nil {
		wait = *c.WaitSnake
	}

	variables = c.Variables
	return
}

// Execute validates config and returns a single EffectInvokeSubWorkflow effect.
// Missing or malformed config surfaces here so the applier never sees an
// under-specified payload.
func (SubWorkflowHandler) Execute(ctx context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	if req == nil || req.Node == nil {
		return nil, errors.New("sub_workflow: nil request or node")
	}

	var raw subWorkflowConfig
	if len(req.Node.Config) > 0 {
		if err := json.Unmarshal(req.Node.Config, &raw); err != nil {
			return nil, fmt.Errorf("sub_workflow: invalid config json: %w", err)
		}
	}

	kind, target, mapping, wait, variables := raw.normalized()

	if target == "" {
		return nil, errors.New("sub_workflow: targetWorkflowId is required")
	}
	if kind != SubWorkflowTargetDAG && kind != SubWorkflowTargetPlugin {
		return nil, fmt.Errorf("sub_workflow: unknown targetKind %q (expected \"dag\" or \"plugin\")", kind)
	}
	if len(mapping) > 0 && !json.Valid(mapping) {
		return nil, errors.New("sub_workflow: inputMapping is not valid json")
	}

	payload := InvokeSubWorkflowPayload{
		WorkflowID:        target,
		TargetKind:        kind,
		TargetWorkflowID:  target,
		InputMapping:      mapping,
		WaitForCompletion: wait,
		Variables:         variables,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("sub_workflow: marshal payload: %w", err)
	}

	return &NodeExecResult{
		Effects: []Effect{{Kind: EffectInvokeSubWorkflow, Payload: encoded}},
	}, nil
}

// ConfigSchema describes the shape of sub_workflow node configuration. Accepted
// by the editor's schema-driven config panel.
func (SubWorkflowHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "targetKind": {"type": "string", "enum": ["dag", "plugin"], "default": "dag"},
    "targetWorkflowId": {"type": "string", "description": "DAG workflow UUID or plugin id"},
    "inputMapping": {"type": "object", "description": "Templated keys resolved against parent run context"},
    "waitForCompletion": {"type": "boolean", "default": true},
    "workflowId": {"type": "string", "description": "Deprecated alias for targetWorkflowId"},
    "variables": {"type": "object", "description": "Legacy back-compat field"}
  },
  "required": ["targetWorkflowId"]
}`)
}

// Capabilities declares the single effect kind this handler emits.
func (SubWorkflowHandler) Capabilities() []EffectKind {
	return []EffectKind{EffectInvokeSubWorkflow}
}
