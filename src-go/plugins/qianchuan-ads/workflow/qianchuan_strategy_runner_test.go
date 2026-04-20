package qcworkflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/react-go-quick-starter/server/internal/model"
	nodetypes "github.com/react-go-quick-starter/server/internal/workflow/nodetypes"
)

func TestQianchuanStrategyRunner_EmitsEffect_WithResolvedFields(t *testing.T) {
	h := QianchuanStrategyRunnerHandler{}
	req := &nodetypes.NodeExecRequest{
		Node: &model.WorkflowNode{ID: "run_strategy"},
		Config: map[string]any{
			"strategy_id_template": "{{$context.strategy_id}}",
			"snapshot_ref":         "{{$dataStore.fetch_metrics.snapshot}}",
			"binding_id_template":  "{{$context.binding_id}}",
		},
		DataStore: map[string]any{
			"$context": map[string]any{
				"strategy_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				"binding_id":  "11111111-2222-3333-4444-555555555555",
			},
			"$dataStore": map[string]any{
				"fetch_metrics": map[string]any{
					"snapshot": map[string]any{"ads": []any{map[string]any{"ad_id": "AD7", "roi": 1.2}}},
				},
			},
		},
	}
	out, err := h.Execute(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Effects) != 1 || out.Effects[0].Kind != nodetypes.EffectRunQianchuanStrategy {
		t.Fatalf("expected one run_qianchuan_strategy effect, got %+v", out.Effects)
	}
	var p nodetypes.RunQianchuanStrategyPayload
	_ = json.Unmarshal(out.Effects[0].Payload, &p)
	if p.StrategyID != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("strategy_id not resolved: %q", p.StrategyID)
	}
	if p.BindingID != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("binding_id not resolved: %q", p.BindingID)
	}
	if p.NodeID != "run_strategy" {
		t.Fatalf("nodeID not set: %q", p.NodeID)
	}
}
