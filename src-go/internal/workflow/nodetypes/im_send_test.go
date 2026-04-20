package nodetypes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestIMSendHandler_EmitsEffect(t *testing.T) {
	h := IMSendHandler{}
	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: &model.WorkflowExecution{ID: uuid.New(), ProjectID: uuid.New()},
		Node:      &model.WorkflowNode{ID: "im-1"},
		Config: map[string]any{
			"target": "reply_to_trigger",
			"card": map[string]any{
				"title":   "Build complete",
				"status":  "success",
				"summary": "PR #42 merged",
				"actions": []any{
					map[string]any{
						"id": "approve", "label": "Approve", "type": "callback",
						"payload": map[string]any{"thread": "x"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Effects) != 1 {
		t.Fatalf("effects=%d", len(result.Effects))
	}
	if result.Effects[0].Kind != EffectExecuteIMSend {
		t.Errorf("kind = %s", result.Effects[0].Kind)
	}
	var p ExecuteIMSendPayload
	if err := json.Unmarshal(result.Effects[0].Payload, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Target != "reply_to_trigger" {
		t.Errorf("target = %s", p.Target)
	}
	if len(p.RawCard) == 0 {
		t.Error("rawCard empty")
	}
}
