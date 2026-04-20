package qcworkflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/react-go-quick-starter/server/internal/model"
	nodetypes "github.com/react-go-quick-starter/server/internal/workflow/nodetypes"
)

func TestQianchuanMetricsFetcher_EmitsEffect_WithResolvedBindingID(t *testing.T) {
	h := QianchuanMetricsFetcherHandler{}
	req := &nodetypes.NodeExecRequest{
		Node: &model.WorkflowNode{ID: "fetch_metrics"},
		Config: map[string]any{
			"binding_id_template": "{{$context.binding_id}}",
			"dimensions":          []any{"ads", "live"},
		},
		DataStore: map[string]any{
			"$context": map[string]any{"binding_id": "11111111-2222-3333-4444-555555555555"},
		},
	}
	out, err := h.Execute(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Effects) != 1 || out.Effects[0].Kind != nodetypes.EffectFetchQianchuanMetrics {
		t.Fatalf("expected one fetch_qianchuan_metrics effect, got %+v", out.Effects)
	}
	var p nodetypes.FetchQianchuanMetricsPayload
	_ = json.Unmarshal(out.Effects[0].Payload, &p)
	if p.BindingID != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("binding id template not resolved: %q", p.BindingID)
	}
	if len(p.Dimensions) != 2 || p.Dimensions[0] != "ads" {
		t.Fatalf("dimensions not propagated: %+v", p.Dimensions)
	}
	if p.NodeID != "fetch_metrics" {
		t.Fatalf("nodeID not set: %q", p.NodeID)
	}
}

func TestQianchuanMetricsFetcher_Capabilities(t *testing.T) {
	h := QianchuanMetricsFetcherHandler{}
	caps := h.Capabilities()
	if len(caps) != 1 || caps[0] != nodetypes.EffectFetchQianchuanMetrics {
		t.Fatalf("unexpected capabilities: %v", caps)
	}
}
