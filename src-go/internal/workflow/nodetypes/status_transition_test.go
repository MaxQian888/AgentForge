package nodetypes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestStatusTransitionHandler_MissingTargetStatus(t *testing.T) {
	h := StatusTransitionHandler{}
	taskID := uuid.New()
	exec := &model.WorkflowExecution{ID: uuid.New(), TaskID: &taskID}
	node := &model.WorkflowNode{ID: "trans-1"}

	_, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config:    map[string]any{},
	})
	if err == nil {
		t.Fatal("Execute() should error when targetStatus is missing")
	}
	if !strings.Contains(err.Error(), "targetStatus") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "targetStatus")
	}
}

func TestStatusTransitionHandler_EmptyTargetStatus(t *testing.T) {
	h := StatusTransitionHandler{}
	taskID := uuid.New()
	exec := &model.WorkflowExecution{ID: uuid.New(), TaskID: &taskID}
	node := &model.WorkflowNode{ID: "trans-empty"}

	_, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config:    map[string]any{"targetStatus": ""},
	})
	if err == nil {
		t.Fatal("Execute() should error when targetStatus is empty")
	}
	if !strings.Contains(err.Error(), "targetStatus") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "targetStatus")
	}
}

func TestStatusTransitionHandler_NilTaskID(t *testing.T) {
	h := StatusTransitionHandler{}
	exec := &model.WorkflowExecution{ID: uuid.New()} // TaskID nil
	node := &model.WorkflowNode{ID: "trans-no-task"}

	_, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config:    map[string]any{"targetStatus": "done"},
	})
	if err == nil {
		t.Fatal("Execute() should error when TaskID is nil")
	}
	if !strings.Contains(err.Error(), "task") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "task")
	}
}

func TestStatusTransitionHandler_HappyPath(t *testing.T) {
	h := StatusTransitionHandler{}
	taskID := uuid.New()
	exec := &model.WorkflowExecution{ID: uuid.New(), TaskID: &taskID}
	node := &model.WorkflowNode{ID: "trans-ok"}

	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config:    map[string]any{"targetStatus": "in_progress"},
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
	if eff.Kind != EffectUpdateTaskStatus {
		t.Errorf("effect kind = %s, want %s", eff.Kind, EffectUpdateTaskStatus)
	}

	var payload UpdateTaskStatusPayload
	if err := json.Unmarshal(eff.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.TargetStatus != "in_progress" {
		t.Errorf("targetStatus = %q, want in_progress", payload.TargetStatus)
	}

	assertCapsCoverEffects(t, h, result.Effects)
}

func TestStatusTransitionHandler_Capabilities(t *testing.T) {
	h := StatusTransitionHandler{}
	caps := h.Capabilities()
	if len(caps) != 1 || caps[0] != EffectUpdateTaskStatus {
		t.Errorf("Capabilities() = %v, want [%s]", caps, EffectUpdateTaskStatus)
	}
}

func TestStatusTransitionHandler_ConfigSchema(t *testing.T) {
	h := StatusTransitionHandler{}
	schema := h.ConfigSchema()
	if len(schema) == 0 {
		t.Fatal("ConfigSchema() returned empty")
	}
	if !json.Valid(schema) {
		t.Errorf("ConfigSchema() returned invalid JSON: %s", schema)
	}
}
