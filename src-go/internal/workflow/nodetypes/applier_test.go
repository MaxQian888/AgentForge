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

// ── Park-effect fakes ───────────────────────────────────────────────────

type fakeAgentSpawner struct {
	calls     []spawnCall
	returnRun *model.AgentRun
	returnErr error
}

type spawnCall struct {
	taskID    uuid.UUID
	memberID  uuid.UUID
	runtime   string
	provider  string
	modelName string
	budgetUsd float64
	roleID    string
}

func (f *fakeAgentSpawner) Spawn(_ context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error) {
	f.calls = append(f.calls, spawnCall{taskID, memberID, runtime, provider, modelName, budgetUsd, roleID})
	if f.returnErr != nil {
		return nil, f.returnErr
	}
	if f.returnRun != nil {
		return f.returnRun, nil
	}
	return &model.AgentRun{ID: uuid.New(), TaskID: taskID, MemberID: memberID}, nil
}

type fakeRunMappingRepo struct {
	created []*model.WorkflowRunMapping
	err     error
}

func (f *fakeRunMappingRepo) Create(_ context.Context, mapping *model.WorkflowRunMapping) error {
	f.created = append(f.created, mapping)
	return f.err
}

type fakeReviewRepo struct {
	created []*model.WorkflowPendingReview
	err     error
}

func (f *fakeReviewRepo) Create(_ context.Context, review *model.WorkflowPendingReview) error {
	f.created = append(f.created, review)
	return f.err
}

// ── Park-effect tests ───────────────────────────────────────────────────

func TestApplier_SpawnAgent_CreatesRunAndMapping(t *testing.T) {
	runID := uuid.New()
	spawner := &fakeAgentSpawner{returnRun: &model.AgentRun{ID: runID}}
	mappingRepo := &fakeRunMappingRepo{}
	applier := &EffectApplier{AgentSpawner: spawner, MappingRepo: mappingRepo}

	exec := newExecWithTask()
	memberID := uuid.New()
	node := &model.WorkflowNode{ID: "n-llm"}

	effects := []Effect{
		{
			Kind: EffectSpawnAgent,
			Payload: mustJSON(SpawnAgentPayload{
				Runtime:   "claude_code",
				Provider:  "anthropic",
				Model:     "claude-opus-4-6",
				RoleID:    "role-qa",
				MemberID:  memberID.String(),
				BudgetUsd: 7.5,
			}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), node, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parked {
		t.Fatal("expected parked=true")
	}

	if len(spawner.calls) != 1 {
		t.Fatalf("expected 1 spawn call, got %d", len(spawner.calls))
	}
	call := spawner.calls[0]
	if call.taskID != *exec.TaskID {
		t.Errorf("taskID = %s, want %s", call.taskID, *exec.TaskID)
	}
	if call.memberID != memberID {
		t.Errorf("memberID = %s, want %s", call.memberID, memberID)
	}
	if call.runtime != "claude_code" {
		t.Errorf("runtime = %q, want %q", call.runtime, "claude_code")
	}
	if call.provider != "anthropic" {
		t.Errorf("provider = %q, want %q", call.provider, "anthropic")
	}
	if call.modelName != "claude-opus-4-6" {
		t.Errorf("model = %q, want %q", call.modelName, "claude-opus-4-6")
	}
	if call.budgetUsd != 7.5 {
		t.Errorf("budgetUsd = %v, want 7.5", call.budgetUsd)
	}
	if call.roleID != "role-qa" {
		t.Errorf("roleID = %q, want %q", call.roleID, "role-qa")
	}

	if len(mappingRepo.created) != 1 {
		t.Fatalf("expected 1 mapping create, got %d", len(mappingRepo.created))
	}
	mapping := mappingRepo.created[0]
	if mapping.ExecutionID != exec.ID {
		t.Errorf("mapping.ExecutionID = %s, want %s", mapping.ExecutionID, exec.ID)
	}
	if mapping.NodeID != node.ID {
		t.Errorf("mapping.NodeID = %q, want %q", mapping.NodeID, node.ID)
	}
	if mapping.AgentRunID != runID {
		t.Errorf("mapping.AgentRunID = %s, want %s", mapping.AgentRunID, runID)
	}
}

func TestApplier_SpawnAgent_NilTaskID_ReturnsError(t *testing.T) {
	spawner := &fakeAgentSpawner{}
	applier := &EffectApplier{AgentSpawner: spawner}

	exec := newExec() // no TaskID
	effects := []Effect{
		{
			Kind:    EffectSpawnAgent,
			Payload: mustJSON(SpawnAgentPayload{Runtime: "claude_code"}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), &model.WorkflowNode{ID: "n"}, effects)
	if err == nil {
		t.Fatal("expected error for nil TaskID")
	}
	if parked {
		t.Fatal("expected parked=false on error")
	}
	if len(spawner.calls) != 0 {
		t.Errorf("expected Spawn not to be called, got %d calls", len(spawner.calls))
	}
}

func TestApplier_SpawnAgent_NilSpawner_ReturnsError(t *testing.T) {
	applier := &EffectApplier{AgentSpawner: nil}

	exec := newExecWithTask()
	effects := []Effect{
		{
			Kind:    EffectSpawnAgent,
			Payload: mustJSON(SpawnAgentPayload{Runtime: "claude_code"}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), &model.WorkflowNode{ID: "n"}, effects)
	if err == nil {
		t.Fatal("expected error for nil AgentSpawner")
	}
	if parked {
		t.Fatal("expected parked=false on error")
	}
}

func TestApplier_SpawnAgent_NilMappingRepo_StillParks(t *testing.T) {
	spawner := &fakeAgentSpawner{}
	applier := &EffectApplier{AgentSpawner: spawner, MappingRepo: nil}

	exec := newExecWithTask()
	effects := []Effect{
		{
			Kind: EffectSpawnAgent,
			Payload: mustJSON(SpawnAgentPayload{
				Runtime: "claude_code",
				RoleID:  "role-qa",
			}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), &model.WorkflowNode{ID: "n"}, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parked {
		t.Fatal("expected parked=true even with nil MappingRepo")
	}
	if len(spawner.calls) != 1 {
		t.Fatalf("expected spawn to happen, got %d calls", len(spawner.calls))
	}
}

func TestApplier_RequestReview_PersistsReview(t *testing.T) {
	reviewRepo := &fakeReviewRepo{}
	applier := &EffectApplier{ReviewRepo: reviewRepo}

	exec := newExec()
	node := &model.WorkflowNode{ID: "n-review"}

	effects := []Effect{
		{
			Kind: EffectRequestReview,
			Payload: mustJSON(RequestReviewPayload{
				Prompt:  "Approve this deployment?",
				Context: json.RawMessage(`{"env":"prod"}`),
			}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), node, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parked {
		t.Fatal("expected parked=true")
	}

	if len(reviewRepo.created) != 1 {
		t.Fatalf("expected 1 review create, got %d", len(reviewRepo.created))
	}
	r := reviewRepo.created[0]
	if r.ExecutionID != exec.ID {
		t.Errorf("ExecutionID = %s, want %s", r.ExecutionID, exec.ID)
	}
	if r.NodeID != "n-review" {
		t.Errorf("NodeID = %q, want %q", r.NodeID, "n-review")
	}
	if r.ProjectID != exec.ProjectID {
		t.Errorf("ProjectID = %s, want %s", r.ProjectID, exec.ProjectID)
	}
	if r.Prompt != "Approve this deployment?" {
		t.Errorf("Prompt = %q, want %q", r.Prompt, "Approve this deployment?")
	}
	if r.Decision != model.ReviewDecisionPending {
		t.Errorf("Decision = %q, want %q", r.Decision, model.ReviewDecisionPending)
	}
	if string(r.Context) != `{"env":"prod"}` {
		t.Errorf("Context = %q, want %q", string(r.Context), `{"env":"prod"}`)
	}
}

func TestApplier_RequestReview_NilReviewRepo_StillParks(t *testing.T) {
	applier := &EffectApplier{ReviewRepo: nil}

	exec := newExec()
	effects := []Effect{
		{
			Kind:    EffectRequestReview,
			Payload: mustJSON(RequestReviewPayload{Prompt: "review?"}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), &model.WorkflowNode{ID: "n"}, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parked {
		t.Fatal("expected parked=true even with nil ReviewRepo")
	}
}

func TestApplier_WaitEvent_BroadcastsAndParks(t *testing.T) {
	hub := &fakeBroadcastHub{}
	applier := &EffectApplier{Hub: hub}

	exec := newExec()
	node := &model.WorkflowNode{ID: "n-wait"}

	effects := []Effect{
		{
			Kind: EffectWaitEvent,
			Payload: mustJSON(WaitEventPayload{
				EventType: "webhook.received",
				MatchKey:  "order-123",
			}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), node, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parked {
		t.Fatal("expected parked=true")
	}

	if len(hub.calls) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(hub.calls))
	}
	call := hub.calls[0]
	if call.eventType != "workflow.node.waiting" {
		t.Errorf("eventType = %q, want %q", call.eventType, "workflow.node.waiting")
	}
	if call.projectID != exec.ProjectID.String() {
		t.Errorf("projectID = %q, want %q", call.projectID, exec.ProjectID.String())
	}
	if call.payload["eventType"] != "webhook.received" {
		t.Errorf("payload[eventType] = %v, want %q", call.payload["eventType"], "webhook.received")
	}
	if call.payload["matchKey"] != "order-123" {
		t.Errorf("payload[matchKey] = %v, want %q", call.payload["matchKey"], "order-123")
	}
	if call.payload["nodeId"] != "n-wait" {
		t.Errorf("payload[nodeId] = %v, want %q", call.payload["nodeId"], "n-wait")
	}
	if call.payload["executionId"] != exec.ID.String() {
		t.Errorf("payload[executionId] = %v, want %q", call.payload["executionId"], exec.ID.String())
	}
}

func TestApplier_WaitEvent_NilHub_StillParks(t *testing.T) {
	applier := &EffectApplier{Hub: nil}

	exec := newExec()
	effects := []Effect{
		{
			Kind:    EffectWaitEvent,
			Payload: mustJSON(WaitEventPayload{EventType: "webhook.received"}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), &model.WorkflowNode{ID: "n"}, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parked {
		t.Fatal("expected parked=true even with nil Hub")
	}
}

func TestApplier_InvokeSubWorkflow_StubParks(t *testing.T) {
	applier := &EffectApplier{}

	exec := newExec()
	effects := []Effect{
		{
			Kind: EffectInvokeSubWorkflow,
			Payload: mustJSON(InvokeSubWorkflowPayload{
				WorkflowID: "wf-child",
			}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), &model.WorkflowNode{ID: "n"}, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parked {
		t.Fatal("expected parked=true for sub_workflow stub")
	}
}

// ── EmployeeSpawner fake ────────────────────────────────────────────────

type fakeEmployeeSpawner struct {
	called    bool
	lastIn    EmployeeInvokeInput
	runID     uuid.UUID
	invokeErr error
}

func (f *fakeEmployeeSpawner) Invoke(_ context.Context, in EmployeeInvokeInput) (*EmployeeInvokeResult, error) {
	f.called = true
	f.lastIn = in
	if f.invokeErr != nil {
		return nil, f.invokeErr
	}
	return &EmployeeInvokeResult{AgentRunID: f.runID}, nil
}

// ── Employee-spawner routing tests ─────────────────────────────────────

func TestApplier_SpawnAgent_RoutesThroughEmployeeSpawner(t *testing.T) {
	empRunID := uuid.New()
	empSpawner := &fakeEmployeeSpawner{runID: empRunID}
	agentSpawner := &fakeAgentSpawner{}
	mappingRepo := &fakeRunMappingRepo{}
	applier := &EffectApplier{
		AgentSpawner:    agentSpawner,
		EmployeeSpawner: empSpawner,
		MappingRepo:     mappingRepo,
	}

	exec := newExecWithTask()
	empID := uuid.New()
	node := &model.WorkflowNode{ID: "n-emp"}

	effects := []Effect{
		{
			Kind: EffectSpawnAgent,
			Payload: mustJSON(SpawnAgentPayload{
				EmployeeID: empID.String(),
				BudgetUsd:  3.0,
			}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), node, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parked {
		t.Fatal("expected parked=true")
	}
	// EmployeeSpawner must have been called.
	if !empSpawner.called {
		t.Fatal("expected EmployeeSpawner.Invoke to be called")
	}
	if empSpawner.lastIn.EmployeeID != empID {
		t.Errorf("EmployeeID = %s, want %s", empSpawner.lastIn.EmployeeID, empID)
	}
	if empSpawner.lastIn.TaskID != *exec.TaskID {
		t.Errorf("TaskID = %s, want %s", empSpawner.lastIn.TaskID, *exec.TaskID)
	}
	if empSpawner.lastIn.ExecutionID != exec.ID {
		t.Errorf("ExecutionID = %s, want %s", empSpawner.lastIn.ExecutionID, exec.ID)
	}
	if empSpawner.lastIn.NodeID != node.ID {
		t.Errorf("NodeID = %q, want %q", empSpawner.lastIn.NodeID, node.ID)
	}
	if empSpawner.lastIn.BudgetUsd != 3.0 {
		t.Errorf("BudgetUsd = %v, want 3.0", empSpawner.lastIn.BudgetUsd)
	}
	// Raw AgentSpawner must NOT have been called.
	if len(agentSpawner.calls) != 0 {
		t.Errorf("AgentSpawner.Spawn should not have been called, got %d call(s)", len(agentSpawner.calls))
	}
	// Mapping must have been persisted with the returned run ID.
	if len(mappingRepo.created) != 1 {
		t.Fatalf("expected 1 mapping create, got %d", len(mappingRepo.created))
	}
	m := mappingRepo.created[0]
	if m.ExecutionID != exec.ID {
		t.Errorf("mapping.ExecutionID = %s, want %s", m.ExecutionID, exec.ID)
	}
	if m.NodeID != node.ID {
		t.Errorf("mapping.NodeID = %q, want %q", m.NodeID, node.ID)
	}
	if m.AgentRunID != empRunID {
		t.Errorf("mapping.AgentRunID = %s, want %s", m.AgentRunID, empRunID)
	}
}

func TestApplier_SpawnAgent_FallsBackToAgentSpawnerWhenNoEmployeeID(t *testing.T) {
	empSpawner := &fakeEmployeeSpawner{}
	agentSpawner := &fakeAgentSpawner{}
	mappingRepo := &fakeRunMappingRepo{}
	applier := &EffectApplier{
		AgentSpawner:    agentSpawner,
		EmployeeSpawner: empSpawner,
		MappingRepo:     mappingRepo,
	}

	exec := newExecWithTask()
	node := &model.WorkflowNode{ID: "n-raw"}

	effects := []Effect{
		{
			Kind: EffectSpawnAgent,
			Payload: mustJSON(SpawnAgentPayload{
				Runtime:  "claude_code",
				Provider: "anthropic",
				Model:    "claude-opus-4-6",
			}),
		},
	}

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), node, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parked {
		t.Fatal("expected parked=true")
	}
	if len(agentSpawner.calls) != 1 {
		t.Fatalf("expected AgentSpawner.Spawn to be called once, got %d", len(agentSpawner.calls))
	}
	if empSpawner.called {
		t.Fatal("EmployeeSpawner.Invoke should NOT have been called when EmployeeID is empty")
	}
}

func TestApplier_SpawnAgent_NilEmployeeSpawnerWhenNeededErrors(t *testing.T) {
	agentSpawner := &fakeAgentSpawner{}
	applier := &EffectApplier{
		AgentSpawner:    agentSpawner,
		EmployeeSpawner: nil, // explicitly nil
	}

	exec := newExecWithTask()
	empID := uuid.New()

	effects := []Effect{
		{
			Kind: EffectSpawnAgent,
			Payload: mustJSON(SpawnAgentPayload{
				EmployeeID: empID.String(),
			}),
		},
	}

	_, err := applier.Apply(context.Background(), exec, uuid.New(), &model.WorkflowNode{ID: "n"}, effects)
	if err == nil {
		t.Fatal("expected error when EmployeeSpawner is nil but payload has employeeId")
	}
	if !contains(err.Error(), "EmployeeSpawner is nil") {
		t.Errorf("error = %q, want it to mention 'EmployeeSpawner is nil'", err.Error())
	}
}

func TestApplier_SpawnAgent_InvalidEmployeeIDReturnsError(t *testing.T) {
	empSpawner := &fakeEmployeeSpawner{}
	agentSpawner := &fakeAgentSpawner{}
	applier := &EffectApplier{
		AgentSpawner:    agentSpawner,
		EmployeeSpawner: empSpawner,
	}

	exec := newExecWithTask()

	effects := []Effect{
		{
			Kind: EffectSpawnAgent,
			Payload: mustJSON(SpawnAgentPayload{
				EmployeeID: "not-a-uuid",
			}),
		},
	}

	_, err := applier.Apply(context.Background(), exec, uuid.New(), &model.WorkflowNode{ID: "n"}, effects)
	if err == nil {
		t.Fatal("expected error for malformed employeeId")
	}
	if !contains(err.Error(), "invalid employeeId") {
		t.Errorf("error = %q, want it to mention 'invalid employeeId'", err.Error())
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
