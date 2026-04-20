package nodetypes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestHumanReviewHandler_HappyPath(t *testing.T) {
	h := HumanReviewHandler{}
	exec := &model.WorkflowExecution{ID: uuid.New(), ProjectID: uuid.New()}
	node := &model.WorkflowNode{ID: "review-1"}

	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config: map[string]any{
			"prompt":   "Please approve",
			"deadline": "2026-01-01",
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
	if eff.Kind != EffectRequestReview {
		t.Errorf("effect kind = %s, want %s", eff.Kind, EffectRequestReview)
	}

	var payload RequestReviewPayload
	if err := json.Unmarshal(eff.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Prompt != "Please approve" {
		t.Errorf("Prompt = %q, want %q", payload.Prompt, "Please approve")
	}
	if len(payload.Context) == 0 {
		t.Fatal("Context is empty, want serialized config")
	}

	var ctxMap map[string]any
	if err := json.Unmarshal(payload.Context, &ctxMap); err != nil {
		t.Fatalf("unmarshal context: %v", err)
	}
	if got, _ := ctxMap["prompt"].(string); got != "Please approve" {
		t.Errorf("context.prompt = %q, want %q", got, "Please approve")
	}
	if got, _ := ctxMap["deadline"].(string); got != "2026-01-01" {
		t.Errorf("context.deadline = %q, want %q", got, "2026-01-01")
	}

	caps := h.Capabilities()
	if len(caps) != 1 || caps[0] != EffectRequestReview {
		t.Errorf("Capabilities() = %v, want [%s]", caps, EffectRequestReview)
	}
	assertCapsCoverEffects(t, h, result.Effects)
}

func TestHumanReviewHandler_DefaultPrompt(t *testing.T) {
	h := HumanReviewHandler{}
	exec := &model.WorkflowExecution{ID: uuid.New()}
	node := &model.WorkflowNode{ID: "review-1"}

	cases := []struct {
		name   string
		config map[string]any
	}{
		{"missing", map[string]any{}},
		{"empty", map[string]any{"prompt": ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := h.Execute(context.Background(), &NodeExecRequest{
				Execution: exec,
				Node:      node,
				Config:    tc.config,
			})
			if err != nil {
				t.Fatalf("Execute() returned error: %v", err)
			}
			var payload RequestReviewPayload
			_ = json.Unmarshal(result.Effects[0].Payload, &payload)
			if payload.Prompt != "Review required" {
				t.Errorf("Prompt = %q, want %q", payload.Prompt, "Review required")
			}
		})
	}
}

func TestHumanReviewHandler_ContextIsSerializedConfig(t *testing.T) {
	h := HumanReviewHandler{}
	exec := &model.WorkflowExecution{ID: uuid.New()}
	node := &model.WorkflowNode{ID: "review-1"}
	config := map[string]any{
		"prompt":   "Review required",
		"approver": "lead",
		"priority": 3.0,
	}

	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config:    config,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	var payload RequestReviewPayload
	_ = json.Unmarshal(result.Effects[0].Payload, &payload)

	if !json.Valid(payload.Context) {
		t.Fatalf("Context is not valid JSON: %s", payload.Context)
	}
	var ctxMap map[string]any
	if err := json.Unmarshal(payload.Context, &ctxMap); err != nil {
		t.Fatalf("unmarshal context: %v", err)
	}
	for k := range config {
		if _, ok := ctxMap[k]; !ok {
			t.Errorf("context missing key %q", k)
		}
	}
}

func TestHumanReviewHandler_ConfigSchema(t *testing.T) {
	h := HumanReviewHandler{}
	schema := h.ConfigSchema()
	if len(schema) == 0 {
		t.Fatal("ConfigSchema() returned empty")
	}
	if !json.Valid(schema) {
		t.Errorf("ConfigSchema() returned invalid JSON: %s", schema)
	}
}
