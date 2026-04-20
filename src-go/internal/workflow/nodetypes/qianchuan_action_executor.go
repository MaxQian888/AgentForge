package nodetypes

import (
	"context"
	"encoding/json"
)

// QianchuanActionExecutorHandler implements the "qianchuan_action_executor"
// node type. It emits a single EffectExecuteQianchuanAction carrying the
// resolved action_log ID and binding ID. The applier loads the pending
// action_log row, resolves the token, dispatches the action through the
// provider, and marks the row applied or failed.
type QianchuanActionExecutorHandler struct{}

func (QianchuanActionExecutorHandler) Execute(_ context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	if req == nil {
		return &NodeExecResult{}, nil
	}
	actionLogIDTpl, _ := req.Config["action_log_id_template"].(string)
	bindingTpl, _ := req.Config["binding_id_template"].(string)

	actionLogID := ResolveTemplateVars(actionLogIDTpl, req.DataStore)
	bindingID := ResolveTemplateVars(bindingTpl, req.DataStore)

	nodeID := ""
	if req.Node != nil {
		nodeID = req.Node.ID
	}

	payload, _ := json.Marshal(ExecuteQianchuanActionPayload{
		ActionLogID: actionLogID,
		BindingID:   bindingID,
		NodeID:      nodeID,
	})
	return &NodeExecResult{
		Effects: []Effect{{Kind: EffectExecuteQianchuanAction, Payload: payload}},
	}, nil
}

func (QianchuanActionExecutorHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "required": ["action_log_id_template", "binding_id_template"],
  "properties": {
    "action_log_id_template": {"type": "string"},
    "binding_id_template": {"type": "string"}
  }
}`)
}

func (QianchuanActionExecutorHandler) Capabilities() []EffectKind {
	return []EffectKind{EffectExecuteQianchuanAction}
}
