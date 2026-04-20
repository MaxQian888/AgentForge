package nodetypes

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

// fakeLoopDefRepo is a stub LoopDefResolver returning a pre-baked definition.
type fakeLoopDefRepo struct {
	def *model.WorkflowDefinition
	err error
}

func (f *fakeLoopDefRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.WorkflowDefinition, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.def, nil
}

// buildLinearLoopDef builds a definition with a->b->loop and returns it. The
// loop node's target is "a", so after a loop iteration nodes "a" and "b" are
// expected to reset.
func buildLinearLoopDef(t *testing.T, workflowID uuid.UUID) *model.WorkflowDefinition {
	t.Helper()
	nodes := []model.WorkflowNode{
		{ID: "a", Type: "function"},
		{ID: "b", Type: "function"},
		{ID: "loop", Type: "loop"},
	}
	edges := []model.WorkflowEdge{
		{Source: "a", Target: "b"},
		{Source: "b", Target: "loop"},
	}
	nodesJSON, _ := json.Marshal(nodes)
	edgesJSON, _ := json.Marshal(edges)
	return &model.WorkflowDefinition{
		ID:    workflowID,
		Nodes: nodesJSON,
		Edges: edgesJSON,
	}
}

func TestLoopHandler_FirstIterationEmitsResetEffect(t *testing.T) {
	workflowID := uuid.New()
	execID := uuid.New()
	def := buildLinearLoopDef(t, workflowID)

	h := LoopHandler{DefRepo: &fakeLoopDefRepo{def: def}}
	exec := &model.WorkflowExecution{ID: execID, WorkflowID: workflowID}
	node := &model.WorkflowNode{ID: "loop"}

	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config: map[string]any{
			"target_node":    "a",
			"max_iterations": float64(3),
		},
		DataStore: map[string]any{},
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
	if eff.Kind != EffectResetNodes {
		t.Errorf("effect kind = %s, want %s", eff.Kind, EffectResetNodes)
	}

	var payload ResetNodesPayload
	if err := json.Unmarshal(eff.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	wantKey := "_loop_loop_count"
	if payload.CounterKey != wantKey {
		t.Errorf("counterKey = %q, want %q", payload.CounterKey, wantKey)
	}
	if payload.CounterValue != float64(1) {
		t.Errorf("counterValue = %v, want 1", payload.CounterValue)
	}

	got := append([]string(nil), payload.NodeIDs...)
	sort.Strings(got)
	want := []string{"a", "b"}
	if !equalStringSlice(got, want) {
		t.Errorf("nodeIds = %v, want %v", got, want)
	}

	assertCapsCoverEffects(t, h, result.Effects)
}

func TestLoopHandler_ExitOnMaxIterations(t *testing.T) {
	workflowID := uuid.New()
	def := buildLinearLoopDef(t, workflowID)
	h := LoopHandler{DefRepo: &fakeLoopDefRepo{def: def}}
	exec := &model.WorkflowExecution{ID: uuid.New(), WorkflowID: workflowID}
	node := &model.WorkflowNode{ID: "loop"}

	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config: map[string]any{
			"target_node":    "a",
			"max_iterations": float64(3),
		},
		DataStore: map[string]any{"_loop_loop_count": float64(3)},
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if len(result.Effects) != 0 {
		t.Errorf("expected 0 effects on exit, got %d", len(result.Effects))
	}
}

func TestLoopHandler_ExitOnExitCondition(t *testing.T) {
	workflowID := uuid.New()
	def := buildLinearLoopDef(t, workflowID)
	h := LoopHandler{DefRepo: &fakeLoopDefRepo{def: def}}
	exec := &model.WorkflowExecution{ID: uuid.New(), WorkflowID: workflowID}
	node := &model.WorkflowNode{ID: "loop"}

	result, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config: map[string]any{
			"target_node":    "a",
			"max_iterations": float64(5),
			"exit_condition": "true",
		},
		DataStore: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if len(result.Effects) != 0 {
		t.Errorf("expected 0 effects on exit, got %d", len(result.Effects))
	}
}

func TestLoopHandler_MissingTargetNode(t *testing.T) {
	h := LoopHandler{DefRepo: &fakeLoopDefRepo{}}
	exec := &model.WorkflowExecution{ID: uuid.New(), WorkflowID: uuid.New()}
	node := &model.WorkflowNode{ID: "loop"}

	_, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config: map[string]any{
			"max_iterations": float64(3),
		},
		DataStore: map[string]any{},
	})
	if err == nil {
		t.Fatal("Execute() should error when target_node is missing")
	}
	if !strings.Contains(err.Error(), "target_node") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "target_node")
	}
}

func TestLoopHandler_NilDefRepo(t *testing.T) {
	h := LoopHandler{DefRepo: nil}
	exec := &model.WorkflowExecution{ID: uuid.New(), WorkflowID: uuid.New()}
	node := &model.WorkflowNode{ID: "loop"}

	_, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config: map[string]any{
			"target_node": "a",
		},
		DataStore: map[string]any{},
	})
	if err == nil {
		t.Fatal("Execute() should error when DefRepo is nil")
	}
}

func TestLoopHandler_Capabilities(t *testing.T) {
	h := LoopHandler{}
	caps := h.Capabilities()
	if len(caps) != 1 || caps[0] != EffectResetNodes {
		t.Errorf("Capabilities() = %v, want [%s]", caps, EffectResetNodes)
	}
}

func TestLoopHandler_ConfigSchema(t *testing.T) {
	h := LoopHandler{}
	schema := h.ConfigSchema()
	if len(schema) == 0 {
		t.Fatal("ConfigSchema() returned empty")
	}
	if !json.Valid(schema) {
		t.Errorf("ConfigSchema() returned invalid JSON: %s", schema)
	}
}

func TestLoopHandler_DoesNotMutateDataStore(t *testing.T) {
	workflowID := uuid.New()
	def := buildLinearLoopDef(t, workflowID)
	h := LoopHandler{DefRepo: &fakeLoopDefRepo{def: def}}
	exec := &model.WorkflowExecution{ID: uuid.New(), WorkflowID: workflowID}
	node := &model.WorkflowNode{ID: "loop"}

	ds := map[string]any{"existing": "value"}
	_, err := h.Execute(context.Background(), &NodeExecRequest{
		Execution: exec,
		Node:      node,
		Config:    map[string]any{"target_node": "a"},
		DataStore: ds,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if _, ok := ds["_loop_loop_count"]; ok {
		t.Error("handler must not mutate DataStore to write counter key")
	}
	if ds["existing"] != "value" {
		t.Error("existing DataStore entry was mutated")
	}
}
