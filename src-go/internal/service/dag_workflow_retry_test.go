package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/workflow/nodetypes"
	"github.com/google/uuid"
)

type flakyNodeHandler struct {
	failuresBeforeSuccess int
	calls                 int
}

func (h *flakyNodeHandler) Execute(context.Context, *nodetypes.NodeExecRequest) (*nodetypes.NodeExecResult, error) {
	h.calls++
	if h.calls <= h.failuresBeforeSuccess {
		return nil, errors.New("transient failure")
	}
	return &nodetypes.NodeExecResult{Result: json.RawMessage(`{"ok":true}`)}, nil
}

func (h *flakyNodeHandler) ConfigSchema() json.RawMessage { return nil }
func (h *flakyNodeHandler) Capabilities() []nodetypes.EffectKind {
	return nil
}

func TestDAGWorkflowService_ExecuteNode_RetriesTransientFailure(t *testing.T) {
	registry := nodetypes.NewRegistry(nil)
	handler := &flakyNodeHandler{failuresBeforeSuccess: 2}
	if err := registry.RegisterBuiltin("flaky_retry", handler); err != nil {
		t.Fatalf("RegisterBuiltin() error = %v", err)
	}

	execRepo := &fakeExecRepo{byID: map[uuid.UUID]*model.WorkflowExecution{}}
	nodeRepo := &fakeNodeRepo{byExec: map[uuid.UUID][]*model.WorkflowNodeExecution{}}
	svc := NewDAGWorkflowService(&fakeDefRepo{byID: map[uuid.UUID]*model.WorkflowDefinition{}}, execRepo, nodeRepo, nil, nil, registry, &nodetypes.EffectApplier{})

	exec := &model.WorkflowExecution{ID: uuid.New(), ProjectID: uuid.New()}
	node := &model.WorkflowNode{
		ID:     "retry-node",
		Type:   "flaky_retry",
		Config: json.RawMessage(`{"retry_count":2}`),
	}

	if err := svc.executeNode(context.Background(), exec, node, map[string]any{}); err != nil {
		t.Fatalf("executeNode() error = %v", err)
	}
	if handler.calls != 3 {
		t.Fatalf("calls = %d, want 3", handler.calls)
	}
	if len(nodeRepo.byExec[exec.ID]) != 1 || nodeRepo.byExec[exec.ID][0].Status != model.NodeExecCompleted {
		t.Fatalf("node executions = %+v", nodeRepo.byExec[exec.ID])
	}
}

func TestDAGWorkflowService_ExecuteNode_FailsAfterRetryExhausted(t *testing.T) {
	registry := nodetypes.NewRegistry(nil)
	handler := &flakyNodeHandler{failuresBeforeSuccess: 3}
	if err := registry.RegisterBuiltin("flaky_retry_fail", handler); err != nil {
		t.Fatalf("RegisterBuiltin() error = %v", err)
	}

	execRepo := &fakeExecRepo{byID: map[uuid.UUID]*model.WorkflowExecution{}}
	nodeRepo := &fakeNodeRepo{byExec: map[uuid.UUID][]*model.WorkflowNodeExecution{}}
	svc := NewDAGWorkflowService(&fakeDefRepo{byID: map[uuid.UUID]*model.WorkflowDefinition{}}, execRepo, nodeRepo, nil, nil, registry, &nodetypes.EffectApplier{})

	exec := &model.WorkflowExecution{ID: uuid.New(), ProjectID: uuid.New()}
	node := &model.WorkflowNode{
		ID:     "retry-node",
		Type:   "flaky_retry_fail",
		Config: json.RawMessage(`{"retry_count":2}`),
	}

	err := svc.executeNode(context.Background(), exec, node, map[string]any{})
	if err == nil {
		t.Fatal("expected retry exhaustion error")
	}
	if handler.calls != 3 {
		t.Fatalf("calls = %d, want 3", handler.calls)
	}
	if len(nodeRepo.byExec[exec.ID]) != 1 || nodeRepo.byExec[exec.ID][0].Status != model.NodeExecFailed {
		t.Fatalf("node executions = %+v", nodeRepo.byExec[exec.ID])
	}
}
