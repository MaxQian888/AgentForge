package nodetypes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestWaitEventHandler_HappyPath(t *testing.T) {
	h := WaitEventHandler{}
	exec := &model.WorkflowExecution{ID: uuid.New(), ProjectID: uuid.New()}
	node := &model.WorkflowNode{ID: "wait-1"}

	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config: map[string]any{
			"event_type": "webhook",
			"match_key":  "invoice-123",
		},
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if result.Result != nil {
		t.Errorf("result.Result = %v, want nil", result.Result)
	}
	if len(result.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(result.Effects))
	}
	eff := result.Effects[0]
	if eff.Kind != EffectWaitEvent {
		t.Errorf("effect kind = %s, want %s", eff.Kind, EffectWaitEvent)
	}

	var payload WaitEventPayload
	if err := json.Unmarshal(eff.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.EventType != "webhook" {
		t.Errorf("EventType = %q, want webhook", payload.EventType)
	}
	if payload.MatchKey != "invoice-123" {
		t.Errorf("MatchKey = %q, want invoice-123", payload.MatchKey)
	}

	caps := h.Capabilities()
	if len(caps) != 1 || caps[0] != EffectWaitEvent {
		t.Errorf("Capabilities() = %v, want [%s]", caps, EffectWaitEvent)
	}
	assertCapsCoverEffects(t, h, result.Effects)
}

func TestWaitEventHandler_NoMatchKey(t *testing.T) {
	h := WaitEventHandler{}
	exec := &model.WorkflowExecution{ID: uuid.New()}
	node := &model.WorkflowNode{ID: "wait-1"}

	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config:    map[string]any{"event_type": "webhook"},
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	var payload WaitEventPayload
	_ = json.Unmarshal(result.Effects[0].Payload, &payload)
	if payload.EventType != "webhook" {
		t.Errorf("EventType = %q, want webhook", payload.EventType)
	}
	if payload.MatchKey != "" {
		t.Errorf("MatchKey = %q, want empty string", payload.MatchKey)
	}
}

func TestWaitEventHandler_NoEventType(t *testing.T) {
	h := WaitEventHandler{}
	exec := &model.WorkflowExecution{ID: uuid.New()}
	node := &model.WorkflowNode{ID: "wait-1"}

	// No event_type set — preserves current service behavior (not an error).
	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config:    map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if len(result.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(result.Effects))
	}
	var payload WaitEventPayload
	_ = json.Unmarshal(result.Effects[0].Payload, &payload)
	if payload.EventType != "" {
		t.Errorf("EventType = %q, want empty string", payload.EventType)
	}
	if payload.MatchKey != "" {
		t.Errorf("MatchKey = %q, want empty string", payload.MatchKey)
	}
	assertCapsCoverEffects(t, h, result.Effects)
}

func TestWaitEventHandler_ConfigSchema(t *testing.T) {
	h := WaitEventHandler{}
	schema := h.ConfigSchema()
	if len(schema) == 0 {
		t.Fatal("ConfigSchema() returned empty")
	}
	if !json.Valid(schema) {
		t.Errorf("ConfigSchema() returned invalid JSON: %s", schema)
	}
}
