package nodetypes

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type fakeWaitExecLookup struct {
	exec *model.WorkflowExecution
}

func (f *fakeWaitExecLookup) GetExecution(_ context.Context, id uuid.UUID) (*model.WorkflowExecution, error) {
	if f.exec == nil || f.exec.ID != id {
		return nil, errors.New("not found")
	}
	return f.exec, nil
}

type fakeWaitNodeExecLookup struct {
	items []*model.WorkflowNodeExecution
}

func (f *fakeWaitNodeExecLookup) ListNodeExecutions(_ context.Context, _ uuid.UUID) ([]*model.WorkflowNodeExecution, error) {
	return f.items, nil
}

type fakeWaitNodeExecWriter struct {
	statusByID map[uuid.UUID]string
	resultByID map[uuid.UUID]json.RawMessage
}

func (f *fakeWaitNodeExecWriter) UpdateNodeExecution(_ context.Context, id uuid.UUID, status string, result json.RawMessage, _ string) error {
	if f.statusByID == nil {
		f.statusByID = map[uuid.UUID]string{}
		f.resultByID = map[uuid.UUID]json.RawMessage{}
	}
	f.statusByID[id] = status
	f.resultByID[id] = result
	return nil
}

type fakeWaitAdvancer struct{ advanced []uuid.UUID }

func (f *fakeWaitAdvancer) AdvanceExecution(_ context.Context, id uuid.UUID) error {
	f.advanced = append(f.advanced, id)
	return nil
}

type fakeWaitDataStoreWriter struct{ merged map[string]any }

func (f *fakeWaitDataStoreWriter) MergeNodeResult(_ context.Context, _ uuid.UUID, nodeID string, payload map[string]any) error {
	if f.merged == nil {
		f.merged = map[string]any{}
	}
	f.merged[nodeID] = payload
	return nil
}

func TestWaitEventResumer_HappyPath(t *testing.T) {
	execID := uuid.New()
	nodeExecID := uuid.New()
	execLookup := &fakeWaitExecLookup{exec: &model.WorkflowExecution{
		ID:     execID,
		Status: model.WorkflowExecStatusRunning,
	}}
	nodeLookup := &fakeWaitNodeExecLookup{items: []*model.WorkflowNodeExecution{
		{ID: nodeExecID, ExecutionID: execID, NodeID: "wait-1", Status: model.NodeExecWaiting},
	}}
	writer := &fakeWaitNodeExecWriter{}
	advancer := &fakeWaitAdvancer{}
	ds := &fakeWaitDataStoreWriter{}

	r := &WaitEventResumer{
		ExecLookup: execLookup, NodeLookup: nodeLookup,
		NodeWriter: writer, Advancer: advancer, DataStore: ds,
	}

	err := r.Resume(context.Background(), execID, "wait-1", map[string]any{"action_id": "approve", "value": "y"})
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}
	if writer.statusByID[nodeExecID] != model.NodeExecCompleted {
		t.Errorf("node status = %q, want completed", writer.statusByID[nodeExecID])
	}
	if len(advancer.advanced) != 1 || advancer.advanced[0] != execID {
		t.Errorf("advancer.advanced = %v, want [%s]", advancer.advanced, execID)
	}
	if got, _ := ds.merged["wait-1"].(map[string]any)["action_id"]; got != "approve" {
		t.Errorf("dataStore[wait-1].action_id = %v, want approve", got)
	}
}

func TestWaitEventResumer_NotWaiting(t *testing.T) {
	execID := uuid.New()
	nodeExecID := uuid.New()
	r := &WaitEventResumer{
		ExecLookup: &fakeWaitExecLookup{exec: &model.WorkflowExecution{ID: execID, Status: model.WorkflowExecStatusCompleted}},
		NodeLookup: &fakeWaitNodeExecLookup{items: []*model.WorkflowNodeExecution{
			{ID: nodeExecID, ExecutionID: execID, NodeID: "wait-1", Status: model.NodeExecCompleted},
		}},
		NodeWriter: &fakeWaitNodeExecWriter{}, Advancer: &fakeWaitAdvancer{}, DataStore: &fakeWaitDataStoreWriter{},
	}
	err := r.Resume(context.Background(), execID, "wait-1", nil)
	if !errors.Is(err, ErrWaitEventNotWaiting) {
		t.Fatalf("err = %v, want ErrWaitEventNotWaiting", err)
	}
}

func TestWaitEventResumer_StatusConstantStability(t *testing.T) {
	if model.NodeExecWaiting == "" {
		t.Fatal("model.NodeExecWaiting must be a non-empty constant")
	}
}
