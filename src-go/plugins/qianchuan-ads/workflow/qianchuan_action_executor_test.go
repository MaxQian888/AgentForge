package qcworkflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/react-go-quick-starter/server/internal/model"
	nodetypes "github.com/react-go-quick-starter/server/internal/workflow/nodetypes"
)

func TestQianchuanActionExecutor_EmitsEffect_WithResolvedActionLogID(t *testing.T) {
	h := QianchuanActionExecutorHandler{}
	req := &nodetypes.NodeExecRequest{
		Node: &model.WorkflowNode{ID: "execute_action"},
		Config: map[string]any{
			"action_log_id_template": "{{$dataStore.run_strategy.actions.0.action_log_id}}",
			"binding_id_template":    "{{$context.binding_id}}",
		},
		DataStore: map[string]any{
			"$context": map[string]any{"binding_id": "11111111-2222-3333-4444-555555555555"},
			"$dataStore": map[string]any{
				"run_strategy": map[string]any{
					"actions": map[string]any{
						"0": map[string]any{"action_log_id": "deadbeef-1234-5678-9abc-def012345678"},
					},
				},
			},
		},
	}
	out, err := h.Execute(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Effects) != 1 || out.Effects[0].Kind != nodetypes.EffectExecuteQianchuanAction {
		t.Fatalf("expected one execute_qianchuan_action effect, got %+v", out.Effects)
	}
	var p nodetypes.ExecuteQianchuanActionPayload
	_ = json.Unmarshal(out.Effects[0].Payload, &p)
	if p.BindingID != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("binding_id not resolved: %q", p.BindingID)
	}
	if p.NodeID != "execute_action" {
		t.Fatalf("nodeID not set: %q", p.NodeID)
	}
}
