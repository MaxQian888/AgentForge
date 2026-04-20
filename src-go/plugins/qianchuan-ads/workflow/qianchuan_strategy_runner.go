package qcworkflow

import (
	"context"
	"encoding/json"

	nodetypes "github.com/react-go-quick-starter/server/internal/workflow/nodetypes"
)

// QianchuanStrategyRunnerHandler implements the "qianchuan_strategy_runner"
// node type. It emits a single nodetypes.EffectRunQianchuanStrategy effect carrying
// the resolved strategy ID, snapshot reference, and binding ID for the
// applier to evaluate the strategy rules and persist action_log rows.
type QianchuanStrategyRunnerHandler struct{}

func (QianchuanStrategyRunnerHandler) Execute(_ context.Context, req *nodetypes.NodeExecRequest) (*nodetypes.NodeExecResult, error) {
	if req == nil {
		return &nodetypes.NodeExecResult{}, nil
	}
	strategyTpl, _ := req.Config["strategy_id_template"].(string)
	snapshotRefTpl, _ := req.Config["snapshot_ref"].(string)
	bindingTpl, _ := req.Config["binding_id_template"].(string)

	strategyID := nodetypes.ResolveTemplateVars(strategyTpl, req.DataStore)
	snapshotRef := nodetypes.ResolveTemplateVars(snapshotRefTpl, req.DataStore)
	bindingID := nodetypes.ResolveTemplateVars(bindingTpl, req.DataStore)

	nodeID := ""
	if req.Node != nil {
		nodeID = req.Node.ID
	}

	payload, _ := json.Marshal(nodetypes.RunQianchuanStrategyPayload{
		StrategyID:  strategyID,
		SnapshotRef: json.RawMessage(snapshotRef),
		BindingID:   bindingID,
		NodeID:      nodeID,
	})
	return &nodetypes.NodeExecResult{
		Effects: []nodetypes.Effect{{Kind: nodetypes.EffectRunQianchuanStrategy, Payload: payload}},
	}, nil
}

func (QianchuanStrategyRunnerHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "required": ["strategy_id_template", "snapshot_ref", "binding_id_template"],
  "properties": {
    "strategy_id_template": {"type": "string"},
    "snapshot_ref": {"type": "string"},
    "binding_id_template": {"type": "string"}
  }
}`)
}

func (QianchuanStrategyRunnerHandler) Capabilities() []nodetypes.EffectKind {
	return []nodetypes.EffectKind{nodetypes.EffectRunQianchuanStrategy}
}
