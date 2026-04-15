package nodetypes

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// ── Fakes ───────────────────────────────────────────────────────────────

type fakeBroadcastHub struct {
	calls []broadcastCall
}

type broadcastCall struct {
	eventType string
	projectID string
	payload   map[string]any
}

func (f *fakeBroadcastHub) BroadcastEvent(eventType, projectID string, payload map[string]any) {
	f.calls = append(f.calls, broadcastCall{eventType, projectID, payload})
}

type fakeTaskTransitioner struct {
	calls []taskTransitionCall
}

type taskTransitionCall struct {
	id     uuid.UUID
	status string
}

func (f *fakeTaskTransitioner) TransitionStatus(_ context.Context, id uuid.UUID, newStatus string) error {
	f.calls = append(f.calls, taskTransitionCall{id, newStatus})
	return nil
}

type fakeNodeExecDeleter struct {
	calls []deleteNodeExecCall
}

type deleteNodeExecCall struct {
	execID  uuid.UUID
	nodeIDs []string
}

func (f *fakeNodeExecDeleter) DeleteNodeExecutionsByNodeIDs(_ context.Context, execID uuid.UUID, ids []string) error {
	f.calls = append(f.calls, deleteNodeExecCall{execID, ids})
	return nil
}

type fakeExecDataStore struct {
	exec *model.WorkflowExecution
	// captured update calls
	updatedID    uuid.UUID
	updatedStore json.RawMessage
}

func (f *fakeExecDataStore) GetExecution(_ context.Context, id uuid.UUID) (*model.WorkflowExecution, error) {
	if f.exec != nil && f.exec.ID == id {
		return f.exec, nil
	}
	return nil, fmt.Errorf("execution %s not found", id)
}

func (f *fakeExecDataStore) UpdateExecutionDataStore(_ context.Context, id uuid.UUID, dataStore json.RawMessage) error {
	f.updatedID = id
	f.updatedStore = dataStore
	return nil
}

// ── Helpers ─────────────────────────────────────────────────────────────

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func newExec() *model.WorkflowExecution {
	return &model.WorkflowExecution{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
	}
}

func newExecWithTask() *model.WorkflowExecution {
	taskID := uuid.New()
	return &model.WorkflowExecution{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		TaskID:    &taskID,
	}
}

// ── Tests ───────────────────────────────────────────────────────────────

func TestApplier_BroadcastEvent(t *testing.T) {
	hub := &fakeBroadcastHub{}
	applier := &EffectApplier{Hub: hub}

	exec := newExec()
	effects := []Effect{
		{
			Kind: EffectBroadcastEvent,
			Payload: mustJSON(BroadcastEventPayload{
				EventType: "node_completed",
				Payload:   map[string]any{"nodeId": "n1"},
			}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), nil, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parked {
		t.Fatal("expected parked=false")
	}
	if len(hub.calls) != 1 {
		t.Fatalf("expected 1 hub call, got %d", len(hub.calls))
	}
	call := hub.calls[0]
	if call.eventType != "node_completed" {
		t.Errorf("eventType = %q, want %q", call.eventType, "node_completed")
	}
	if call.projectID != exec.ProjectID.String() {
		t.Errorf("projectID = %q, want %q", call.projectID, exec.ProjectID.String())
	}
	if call.payload["nodeId"] != "n1" {
		t.Errorf("payload[nodeId] = %v, want %q", call.payload["nodeId"], "n1")
	}
}

func TestApplier_BroadcastEvent_NilHub_NoError(t *testing.T) {
	applier := &EffectApplier{Hub: nil}

	exec := newExec()
	effects := []Effect{
		{
			Kind: EffectBroadcastEvent,
			Payload: mustJSON(BroadcastEventPayload{
				EventType: "test",
			}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), nil, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parked {
		t.Fatal("expected parked=false")
	}
}

func TestApplier_UpdateTaskStatus(t *testing.T) {
	repo := &fakeTaskTransitioner{}
	applier := &EffectApplier{TaskRepo: repo}

	exec := newExecWithTask()
	effects := []Effect{
		{
			Kind:    EffectUpdateTaskStatus,
			Payload: mustJSON(UpdateTaskStatusPayload{TargetStatus: "in_progress"}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), nil, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parked {
		t.Fatal("expected parked=false")
	}
	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(repo.calls))
	}
	if repo.calls[0].id != *exec.TaskID {
		t.Errorf("task ID = %s, want %s", repo.calls[0].id, *exec.TaskID)
	}
	if repo.calls[0].status != "in_progress" {
		t.Errorf("status = %q, want %q", repo.calls[0].status, "in_progress")
	}
}

func TestApplier_UpdateTaskStatus_NilTaskID_ReturnsError(t *testing.T) {
	repo := &fakeTaskTransitioner{}
	applier := &EffectApplier{TaskRepo: repo}

	exec := newExec() // no TaskID
	effects := []Effect{
		{
			Kind:    EffectUpdateTaskStatus,
			Payload: mustJSON(UpdateTaskStatusPayload{TargetStatus: "done"}),
		},
	}

	_, err := applier.Apply(context.Background(), exec, uuid.New(), nil, effects)
	if err == nil {
		t.Fatal("expected error for nil TaskID")
	}
}

func TestApplier_ResetNodes(t *testing.T) {
	nodeRepo := &fakeNodeExecDeleter{}
	applier := &EffectApplier{NodeRepo: nodeRepo}

	exec := newExec()
	effects := []Effect{
		{
			Kind: EffectResetNodes,
			Payload: mustJSON(ResetNodesPayload{
				NodeIDs: []string{"n1", "n2"},
			}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), nil, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parked {
		t.Fatal("expected parked=false")
	}
	if len(nodeRepo.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(nodeRepo.calls))
	}
	if nodeRepo.calls[0].execID != exec.ID {
		t.Errorf("execID = %s, want %s", nodeRepo.calls[0].execID, exec.ID)
	}
	if len(nodeRepo.calls[0].nodeIDs) != 2 {
		t.Fatalf("expected 2 nodeIDs, got %d", len(nodeRepo.calls[0].nodeIDs))
	}
}

func TestApplier_ResetNodes_WithCounter(t *testing.T) {
	nodeRepo := &fakeNodeExecDeleter{}
	exec := newExec()
	exec.DataStore = json.RawMessage(`{"existing": "value"}`)

	execRepo := &fakeExecDataStore{exec: exec}
	applier := &EffectApplier{NodeRepo: nodeRepo, ExecRepo: execRepo}

	effects := []Effect{
		{
			Kind: EffectResetNodes,
			Payload: mustJSON(ResetNodesPayload{
				NodeIDs:      []string{"n1"},
				CounterKey:   "loop_iter",
				CounterValue: 3,
			}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), nil, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parked {
		t.Fatal("expected parked=false")
	}

	// Verify node deletion happened
	if len(nodeRepo.calls) != 1 {
		t.Fatalf("expected 1 node delete call, got %d", len(nodeRepo.calls))
	}

	// Verify DataStore was updated with counter
	if execRepo.updatedID != exec.ID {
		t.Errorf("updated exec ID = %s, want %s", execRepo.updatedID, exec.ID)
	}

	var ds map[string]any
	if err := json.Unmarshal(execRepo.updatedStore, &ds); err != nil {
		t.Fatalf("unmarshal updated store: %v", err)
	}
	if ds["existing"] != "value" {
		t.Errorf("existing key lost: got %v", ds["existing"])
	}
	if ds["loop_iter"] != float64(3) {
		t.Errorf("loop_iter = %v, want 3", ds["loop_iter"])
	}
}

func TestApplier_ParkEffect_ReturnsError(t *testing.T) {
	applier := &EffectApplier{}

	exec := newExec()
	effects := []Effect{
		{
			Kind:    EffectSpawnAgent,
			Payload: mustJSON(SpawnAgentPayload{Runtime: "claude_code"}),
		},
	}

	_, err := applier.Apply(context.Background(), exec, uuid.New(), nil, effects)
	if err == nil {
		t.Fatal("expected error for park effect stub")
	}
	if got := err.Error(); !contains(got, "not yet implemented") {
		t.Errorf("error = %q, want it to contain 'not yet implemented'", got)
	}
}

func TestApplier_UnknownEffect_ReturnsError(t *testing.T) {
	applier := &EffectApplier{}

	exec := newExec()
	effects := []Effect{
		{
			Kind:    EffectKind("nonexistent_effect"),
			Payload: json.RawMessage(`{}`),
		},
	}

	_, err := applier.Apply(context.Background(), exec, uuid.New(), nil, effects)
	if err == nil {
		t.Fatal("expected error for unknown effect kind")
	}
	if got := err.Error(); !contains(got, "nonexistent_effect") {
		t.Errorf("error = %q, want it to mention the unknown kind", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
