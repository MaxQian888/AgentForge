package nodetypes

import (
	"context"
	"encoding/json"
)

// QianchuanMetricsFetcherHandler implements the "qianchuan_metrics_fetcher"
// node type. It emits a single EffectFetchQianchuanMetrics effect; the
// applier resolves the access_token via Spec 1B's secrets store and calls
// adsplatform.Provider.FetchMetrics. The applier writes the snapshot back
// to dataStore[nodeID] = {snapshot} so downstream nodes can reference
// {{$dataStore.<nodeID>.snapshot}} via ResolveTemplateVars.
type QianchuanMetricsFetcherHandler struct{}

func (QianchuanMetricsFetcherHandler) Execute(_ context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	var (
		bindingTpl string
		dims       []string
	)
	if req != nil {
		bindingTpl, _ = req.Config["binding_id_template"].(string)
		if raw, ok := req.Config["dimensions"].([]any); ok {
			for _, d := range raw {
				if s, ok := d.(string); ok {
					dims = append(dims, s)
				}
			}
		}
	}
	binding := ResolveTemplateVars(bindingTpl, req.DataStore)
	nodeID := ""
	if req != nil && req.Node != nil {
		nodeID = req.Node.ID
	}
	payload, _ := json.Marshal(FetchQianchuanMetricsPayload{
		BindingID: binding, Dimensions: dims, NodeID: nodeID,
	})
	return &NodeExecResult{
		Effects: []Effect{{Kind: EffectFetchQianchuanMetrics, Payload: payload}},
	}, nil
}

func (QianchuanMetricsFetcherHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "required": ["binding_id_template"],
  "properties": {
    "binding_id_template": {"type": "string"},
    "dimensions": {"type": "array", "items": {"type": "string"}}
  }
}`)
}

func (QianchuanMetricsFetcherHandler) Capabilities() []EffectKind {
	return []EffectKind{EffectFetchQianchuanMetrics}
}
