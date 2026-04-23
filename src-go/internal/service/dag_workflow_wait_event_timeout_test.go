package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type waitTimeoutDefRepo struct {
	def *model.WorkflowDefinition
}

func (r *waitTimeoutDefRepo) Create(context.Context, *model.WorkflowDefinition) error { return nil }
func (r *waitTimeoutDefRepo) GetByID(_ context.Context, id uuid.UUID) (*model.WorkflowDefinition, error) {
	if r.def != nil && r.def.ID == id {
		return r.def, nil
	}
	return nil, nil
}
func (r *waitTimeoutDefRepo) ListByProject(context.Context, uuid.UUID) ([]*model.WorkflowDefinition, error) {
	return nil, nil
}
func (r *waitTimeoutDefRepo) Update(context.Context, uuid.UUID, *model.WorkflowDefinition) error {
	return nil
}
func (r *waitTimeoutDefRepo) Delete(context.Context, uuid.UUID) error { return nil }

type waitTimeoutExecRepo struct {
	execs         map[uuid.UUID]*model.WorkflowExecution
	updateCalls   []uuid.UUID
	updatedStatus string
	updatedErr    string
}

func (r *waitTimeoutExecRepo) CreateExecution(context.Context, *model.WorkflowExecution) error {
	return nil
}
func (r *waitTimeoutExecRepo) GetExecution(_ context.Context, id uuid.UUID) (*model.WorkflowExecution, error) {
	return r.execs[id], nil
}
func (r *waitTimeoutExecRepo) ListExecutions(context.Context, uuid.UUID) ([]*model.WorkflowExecution, error) {
	return nil, nil
}
func (r *waitTimeoutExecRepo) ListActiveByProject(context.Context, uuid.UUID) ([]*model.WorkflowExecution, error) {
	return nil, nil
}
func (r *waitTimeoutExecRepo) ListActiveExecutions(context.Context) ([]*model.WorkflowExecution, error) {
	out := make([]*model.WorkflowExecution, 0, len(r.execs))
	for _, exec := range r.execs {
		out = append(out, exec)
	}
	return out, nil
}
func (r *waitTimeoutExecRepo) UpdateExecution(_ context.Context, id uuid.UUID, status string, _ json.RawMessage, errMsg string) error {
	r.updateCalls = append(r.updateCalls, id)
	r.updatedStatus = status
	r.updatedErr = errMsg
	if exec := r.execs[id]; exec != nil {
		exec.Status = status
		exec.ErrorMessage = errMsg
	}
	return nil
}
func (r *waitTimeoutExecRepo) UpdateExecutionDataStore(context.Context, uuid.UUID, json.RawMessage) error {
	return nil
}
func (r *waitTimeoutExecRepo) CompleteExecution(context.Context, uuid.UUID, string) error { return nil }

type waitTimeoutNodeRepo struct {
	nodeExecs     map[uuid.UUID][]*model.WorkflowNodeExecution
	updatedNodeID uuid.UUID
	updatedStatus string
	updatedErrMsg string
}

func (r *waitTimeoutNodeRepo) CreateNodeExecution(context.Context, *model.WorkflowNodeExecution) error {
	return nil
}
func (r *waitTimeoutNodeRepo) UpdateNodeExecution(_ context.Context, id uuid.UUID, status string, _ json.RawMessage, errorMessage string) error {
	r.updatedNodeID = id
	r.updatedStatus = status
	r.updatedErrMsg = errorMessage
	for _, list := range r.nodeExecs {
		for _, nodeExec := range list {
			if nodeExec.ID == id {
				nodeExec.Status = status
				nodeExec.ErrorMessage = errorMessage
			}
		}
	}
	return nil
}
func (r *waitTimeoutNodeRepo) ListNodeExecutions(_ context.Context, executionID uuid.UUID) ([]*model.WorkflowNodeExecution, error) {
	return r.nodeExecs[executionID], nil
}
func (r *waitTimeoutNodeRepo) DeleteNodeExecutionsByNodeIDs(context.Context, uuid.UUID, []string) error {
	return nil
}

func TestSweepExpiredWaitEvents_FailsExpiredWaitingNode(t *testing.T) {
	execID := uuid.New()
	defID := uuid.New()
	nodeExecID := uuid.New()
	startedAt := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)

	def := &model.WorkflowDefinition{
		ID: defID,
		Nodes: json.RawMessage(`[
			{"id":"wait-1","type":"wait_event","label":"Wait","config":{"timeout_seconds":30}}
		]`),
	}
	execRepo := &waitTimeoutExecRepo{
		execs: map[uuid.UUID]*model.WorkflowExecution{
			execID: {ID: execID, WorkflowID: defID, ProjectID: uuid.New(), Status: model.WorkflowExecStatusRunning},
		},
	}
	nodeRepo := &waitTimeoutNodeRepo{
		nodeExecs: map[uuid.UUID][]*model.WorkflowNodeExecution{
			execID: {{
				ID:          nodeExecID,
				ExecutionID: execID,
				NodeID:      "wait-1",
				Status:      model.NodeExecWaiting,
				StartedAt:   &startedAt,
				CreatedAt:   startedAt,
			}},
		},
	}

	svc := NewDAGWorkflowService(&waitTimeoutDefRepo{def: def}, execRepo, nodeRepo, nil, nil, nil, nil)
	count, err := svc.SweepExpiredWaitEvents(context.Background(), startedAt.Add(31*time.Second))
	if err != nil {
		t.Fatalf("SweepExpiredWaitEvents() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("expired count = %d, want 1", count)
	}
	if nodeRepo.updatedNodeID != nodeExecID || nodeRepo.updatedStatus != model.NodeExecFailed {
		t.Fatalf("node update = %s/%s", nodeRepo.updatedNodeID, nodeRepo.updatedStatus)
	}
	if execRepo.updatedStatus != model.WorkflowExecStatusFailed {
		t.Fatalf("execution status = %s, want failed", execRepo.updatedStatus)
	}
}

func TestSweepExpiredWaitEvents_SkipsUnexpiredNode(t *testing.T) {
	execID := uuid.New()
	defID := uuid.New()
	startedAt := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)

	def := &model.WorkflowDefinition{
		ID: defID,
		Nodes: json.RawMessage(`[
			{"id":"wait-1","type":"wait_event","label":"Wait","config":{"timeout_seconds":90}}
		]`),
	}
	execRepo := &waitTimeoutExecRepo{
		execs: map[uuid.UUID]*model.WorkflowExecution{
			execID: {ID: execID, WorkflowID: defID, ProjectID: uuid.New(), Status: model.WorkflowExecStatusRunning},
		},
	}
	nodeRepo := &waitTimeoutNodeRepo{
		nodeExecs: map[uuid.UUID][]*model.WorkflowNodeExecution{
			execID: {{
				ID:          uuid.New(),
				ExecutionID: execID,
				NodeID:      "wait-1",
				Status:      model.NodeExecWaiting,
				StartedAt:   &startedAt,
				CreatedAt:   startedAt,
			}},
		},
	}

	svc := NewDAGWorkflowService(&waitTimeoutDefRepo{def: def}, execRepo, nodeRepo, nil, nil, nil, nil)
	count, err := svc.SweepExpiredWaitEvents(context.Background(), startedAt.Add(30*time.Second))
	if err != nil {
		t.Fatalf("SweepExpiredWaitEvents() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("expired count = %d, want 0", count)
	}
	if nodeRepo.updatedNodeID != uuid.Nil || execRepo.updatedStatus != "" {
		t.Fatalf("unexpected updates: node=%s execStatus=%s", nodeRepo.updatedNodeID, execRepo.updatedStatus)
	}
}
