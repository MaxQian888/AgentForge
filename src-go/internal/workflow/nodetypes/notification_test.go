package nodetypes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestNotificationHandler_DefaultMessage(t *testing.T) {
	h := NotificationHandler{}
	execID := uuid.New()
	exec := &model.WorkflowExecution{ID: execID}
	node := &model.WorkflowNode{ID: "notify-1"}

	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config:    map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if len(result.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(result.Effects))
	}
	eff := result.Effects[0]
	if eff.Kind != EffectBroadcastEvent {
		t.Errorf("effect kind = %s, want %s", eff.Kind, EffectBroadcastEvent)
	}

	var payload BroadcastEventPayload
	if err := json.Unmarshal(eff.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.EventType != "workflow.notification" {
		t.Errorf("eventType = %q, want workflow.notification", payload.EventType)
	}
	if got, _ := payload.Payload["message"].(string); got != "Workflow notification" {
		t.Errorf("message = %q, want default %q", got, "Workflow notification")
	}
	if got, _ := payload.Payload["executionId"].(string); got != execID.String() {
		t.Errorf("executionId = %q, want %q", got, execID.String())
	}
	if got, _ := payload.Payload["nodeId"].(string); got != "notify-1" {
		t.Errorf("nodeId = %q, want notify-1", got)
	}

	assertCapsCoverEffects(t, h, result.Effects)
}

func TestNotificationHandler_CustomMessage(t *testing.T) {
	h := NotificationHandler{}
	exec := &model.WorkflowExecution{ID: uuid.New()}
	node := &model.WorkflowNode{ID: "notify-x"}

	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config:    map[string]any{"message": "custom text"},
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if len(result.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(result.Effects))
	}

	var payload BroadcastEventPayload
	if err := json.Unmarshal(result.Effects[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got, _ := payload.Payload["message"].(string); got != "custom text" {
		t.Errorf("message = %q, want custom text", got)
	}

	assertCapsCoverEffects(t, h, result.Effects)
}

func TestNotificationHandler_EmptyMessageFallsBackToDefault(t *testing.T) {
	h := NotificationHandler{}
	exec := &model.WorkflowExecution{ID: uuid.New()}
	node := &model.WorkflowNode{ID: "n"}

	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config:    map[string]any{"message": ""},
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	var payload BroadcastEventPayload
	_ = json.Unmarshal(result.Effects[0].Payload, &payload)
	if got, _ := payload.Payload["message"].(string); got != "Workflow notification" {
		t.Errorf("message = %q, want default", got)
	}
	assertCapsCoverEffects(t, h, result.Effects)
}

func TestNotificationHandler_Capabilities(t *testing.T) {
	h := NotificationHandler{}
	caps := h.Capabilities()
	if len(caps) != 1 || caps[0] != EffectBroadcastEvent {
		t.Errorf("Capabilities() = %v, want [%s]", caps, EffectBroadcastEvent)
	}
}

func TestNotificationHandler_ConfigSchema(t *testing.T) {
	h := NotificationHandler{}
	schema := h.ConfigSchema()
	if len(schema) == 0 {
		t.Fatal("ConfigSchema() returned empty")
	}
	if !json.Valid(schema) {
		t.Errorf("ConfigSchema() returned invalid JSON: %s", schema)
	}
}
