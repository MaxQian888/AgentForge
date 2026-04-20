package nodetypes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestConditionHandler_EmptyExpression(t *testing.T) {
	h := ConditionHandler{}
	result, err := h.Execute(context.Background(), &NodeExecRequest{Config: map[string]any{}})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if result.Result != nil {
		t.Errorf("Execute() result.Result = %v, want nil", result.Result)
	}
	assertCapsCoverEffects(t, h, result.Effects)
}

func TestConditionHandler_TrueExpression(t *testing.T) {
	h := ConditionHandler{}
	cfg := map[string]any{"expression": "5 > 1"}
	result, err := h.Execute(context.Background(), &NodeExecRequest{Config: cfg})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	assertCapsCoverEffects(t, h, result.Effects)
}

func TestConditionHandler_FalseExpressionErrors(t *testing.T) {
	h := ConditionHandler{}
	cfg := map[string]any{"expression": "5 < 1"}
	result, err := h.Execute(context.Background(), &NodeExecRequest{Config: cfg})
	if err == nil {
		t.Fatal("Execute() returned nil error, want condition-not-met error")
	}
	if !strings.Contains(err.Error(), "condition not met") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "condition not met")
	}
	if result != nil {
		t.Errorf("Execute() result = %v, want nil", result)
	}
}

func TestConditionHandler_TemplateVarResolution(t *testing.T) {
	h := ConditionHandler{}
	cfg := map[string]any{"expression": "{{input.count}} > 5"}
	ds := map[string]any{
		"input": map[string]any{"count": float64(10)},
	}
	result, err := h.Execute(context.Background(), &NodeExecRequest{Config: cfg, DataStore: ds})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	// Same expression but count is below threshold → should error.
	cfg2 := map[string]any{"expression": "{{input.count}} > 5"}
	ds2 := map[string]any{"input": map[string]any{"count": float64(2)}}
	_, err = h.Execute(context.Background(), &NodeExecRequest{Config: cfg2, DataStore: ds2})
	if err == nil {
		t.Fatal("Execute() with count=2 should error")
	}
}

func TestConditionHandler_TaskStatusViaRepo(t *testing.T) {
	taskID := uuid.New()
	exec := &model.WorkflowExecution{TaskID: &taskID}
	repo := &fakeTaskRepo{status: "done"}

	h := ConditionHandler{TaskRepo: repo}
	cfg := map[string]any{"expression": `task.status == "done"`}
	req := &NodeExecRequest{Config: cfg, Execution: exec}

	result, err := h.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if !repo.called {
		t.Errorf("expected TaskRepo.GetByID to be called")
	}
}

func TestConditionHandler_ConfigSchema(t *testing.T) {
	h := ConditionHandler{}
	schema := h.ConfigSchema()
	if len(schema) == 0 {
		t.Fatal("ConfigSchema() returned empty")
	}
	if !json.Valid(schema) {
		t.Errorf("ConfigSchema() returned invalid JSON: %s", schema)
	}
}

func TestConditionHandler_CapabilitiesNil(t *testing.T) {
	h := ConditionHandler{}
	if caps := h.Capabilities(); len(caps) != 0 {
		t.Errorf("Capabilities() = %v, want nil/empty", caps)
	}
}
