package nodetypes

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

// fakeSubWorkflowEngine is a minimal SubWorkflowEngine for applier tests.
type fakeSubWorkflowEngine struct {
	kind            SubWorkflowTargetKind
	projectByTarget map[string]uuid.UUID
	startErr        error
	started         []startCall
}

type startCall struct {
	target string
	seed   map[string]any
	inv    SubWorkflowInvocation
	runID  uuid.UUID
}

func (f *fakeSubWorkflowEngine) Kind() SubWorkflowTargetKind { return f.kind }

func (f *fakeSubWorkflowEngine) Validate(ctx context.Context, target string, inv SubWorkflowInvocation) error {
	pid, ok := f.projectByTarget[target]
	if !ok {
		return &SubWorkflowInvocationError{Reason: SubWorkflowRejectUnknownTarget, Message: "unknown target"}
	}
	if pid != inv.ProjectID {
		return &SubWorkflowInvocationError{
			Reason:  SubWorkflowRejectCrossProject,
			Message: "cross-project",
		}
	}
	return nil
}

func (f *fakeSubWorkflowEngine) Start(ctx context.Context, target string, seed map[string]any, inv SubWorkflowInvocation) (uuid.UUID, error) {
	if f.startErr != nil {
		return uuid.Nil, f.startErr
	}
	runID := uuid.New()
	f.started = append(f.started, startCall{target: target, seed: seed, inv: inv, runID: runID})
	return runID, nil
}

// capturingLinkRepo records Create/UpdateStatus calls for assertions.
type capturingLinkRepo struct {
	created []*SubWorkflowLinkRecord
}

func (c *capturingLinkRepo) Create(ctx context.Context, link *SubWorkflowLinkRecord) error {
	c.created = append(c.created, link)
	return nil
}
func (c *capturingLinkRepo) GetByParent(ctx context.Context, parentExecutionID uuid.UUID, parentNodeID string) (*SubWorkflowLinkRecord, error) {
	return nil, errors.New("not found")
}
func (c *capturingLinkRepo) GetByChild(ctx context.Context, engineKind string, childRunID uuid.UUID) (*SubWorkflowLinkRecord, error) {
	return nil, errors.New("not found")
}
func (c *capturingLinkRepo) ListByParentExecution(ctx context.Context, parentExecutionID uuid.UUID) ([]*SubWorkflowLinkRecord, error) {
	return nil, nil
}
func (c *capturingLinkRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return nil
}

func newSubWorkflowApplier(t *testing.T, eng *fakeSubWorkflowEngine, links SubWorkflowLinkRepo, guard *RecursionGuard) (*EffectApplier, *fakeBroadcastHub) {
	t.Helper()
	hub := &fakeBroadcastHub{}
	return &EffectApplier{
		Hub:                hub,
		SubWorkflowEngines: NewSubWorkflowEngineRegistry(eng),
		SubWorkflowLinks:   links,
		SubWorkflowGuard:   guard,
	}, hub
}

func TestApplier_InvokeSubWorkflow_ParksAndCreatesLink(t *testing.T) {
	projectID := uuid.New()
	targetWF := uuid.New().String()
	eng := &fakeSubWorkflowEngine{
		kind:            SubWorkflowTargetDAG,
		projectByTarget: map[string]uuid.UUID{targetWF: projectID},
	}
	links := &capturingLinkRepo{}
	applier, hub := newSubWorkflowApplier(t, eng, links, nil)

	exec := &model.WorkflowExecution{
		ID:        uuid.New(),
		ProjectID: projectID,
		DataStore: json.RawMessage(`{"upstream":{"taskId":"task-42"}}`),
		Context:   json.RawMessage(`{}`),
	}
	node := &model.WorkflowNode{ID: "sub-1", Type: "sub_workflow"}
	payload := InvokeSubWorkflowPayload{
		TargetKind:        SubWorkflowTargetDAG,
		TargetWorkflowID:  targetWF,
		InputMapping:      json.RawMessage(`{"task_id":"{{$parent.dataStore.upstream.taskId}}"}`),
		WaitForCompletion: true,
	}
	raw, _ := json.Marshal(payload)

	parked, err := applier.Apply(context.Background(), exec, uuid.New(), node, []Effect{{Kind: EffectInvokeSubWorkflow, Payload: raw}})
	if err != nil {
		t.Fatalf("Apply() error = %v, want nil", err)
	}
	if !parked {
		t.Errorf("parked = false, want true")
	}
	if len(eng.started) != 1 {
		t.Fatalf("engine.Start call count = %d, want 1", len(eng.started))
	}
	if got := eng.started[0].seed["task_id"]; got != "task-42" {
		t.Errorf("seed.task_id = %v, want task-42", got)
	}
	if len(links.created) != 1 {
		t.Fatalf("link repo Create call count = %d, want 1", len(links.created))
	}
	if links.created[0].ChildEngineKind != string(SubWorkflowTargetDAG) {
		t.Errorf("link child engine kind = %s, want dag", links.created[0].ChildEngineKind)
	}
	if len(hub.calls) != 1 || hub.calls[0].eventType != "workflow.sub_workflow.started" {
		t.Errorf("broadcast calls = %+v, want one sub_workflow.started", hub.calls)
	}
}

func TestApplier_InvokeSubWorkflow_CrossProjectRejected(t *testing.T) {
	parentProject := uuid.New()
	childProject := uuid.New()
	targetWF := uuid.New().String()
	eng := &fakeSubWorkflowEngine{
		kind:            SubWorkflowTargetDAG,
		projectByTarget: map[string]uuid.UUID{targetWF: childProject},
	}
	links := &capturingLinkRepo{}
	applier, _ := newSubWorkflowApplier(t, eng, links, nil)

	exec := &model.WorkflowExecution{ID: uuid.New(), ProjectID: parentProject}
	node := &model.WorkflowNode{ID: "sub-x", Type: "sub_workflow"}
	payload := InvokeSubWorkflowPayload{
		TargetKind:       SubWorkflowTargetDAG,
		TargetWorkflowID: targetWF,
	}
	raw, _ := json.Marshal(payload)

	_, err := applier.Apply(context.Background(), exec, uuid.New(), node, []Effect{{Kind: EffectInvokeSubWorkflow, Payload: raw}})
	if err == nil {
		t.Fatal("Apply() returned nil error, want cross-project rejection")
	}
	var invErr *SubWorkflowInvocationError
	if !errors.As(err, &invErr) || invErr.Reason != SubWorkflowRejectCrossProject {
		t.Errorf("Apply() err reason = %v, want cross_project", err)
	}
	if len(eng.started) != 0 {
		t.Errorf("engine was started despite cross-project rejection")
	}
	if len(links.created) != 0 {
		t.Errorf("link row created despite rejection")
	}
}

func TestApplier_InvokeSubWorkflow_CycleRejected(t *testing.T) {
	projectID := uuid.New()
	parentExec := uuid.New()
	parentWF := uuid.New()
	eng := &fakeSubWorkflowEngine{
		kind:            SubWorkflowTargetDAG,
		projectByTarget: map[string]uuid.UUID{parentWF.String(): projectID},
	}
	links := &fakeLinkRepo{byChild: map[uuid.UUID]*SubWorkflowLinkRecord{}}
	lookup := &fakeExecLookup{byExec: map[uuid.UUID]uuid.UUID{parentExec: parentWF}}
	guard := NewRecursionGuard(links, lookup, MaxSubWorkflowDepth)
	applier, _ := newSubWorkflowApplier(t, eng, &capturingLinkRepo{}, guard)

	exec := &model.WorkflowExecution{ID: parentExec, ProjectID: projectID}
	node := &model.WorkflowNode{ID: "sub-self", Type: "sub_workflow"}
	payload := InvokeSubWorkflowPayload{
		TargetKind:       SubWorkflowTargetDAG,
		TargetWorkflowID: parentWF.String(),
	}
	raw, _ := json.Marshal(payload)

	_, err := applier.Apply(context.Background(), exec, uuid.New(), node, []Effect{{Kind: EffectInvokeSubWorkflow, Payload: raw}})
	if err == nil {
		t.Fatal("Apply() returned nil, want cycle rejection")
	}
	var invErr *SubWorkflowInvocationError
	if !errors.As(err, &invErr) || invErr.Reason != SubWorkflowRejectCycle {
		t.Errorf("Apply() err reason = %v, want cycle", err)
	}
	if len(eng.started) != 0 {
		t.Errorf("engine started despite cycle detection")
	}
}

func TestApplier_InvokeSubWorkflow_UnresolvedMappingRejected(t *testing.T) {
	projectID := uuid.New()
	targetWF := uuid.New().String()
	eng := &fakeSubWorkflowEngine{
		kind:            SubWorkflowTargetDAG,
		projectByTarget: map[string]uuid.UUID{targetWF: projectID},
	}
	links := &capturingLinkRepo{}
	applier, _ := newSubWorkflowApplier(t, eng, links, nil)

	exec := &model.WorkflowExecution{
		ID:        uuid.New(),
		ProjectID: projectID,
		DataStore: json.RawMessage(`{}`),
		Context:   json.RawMessage(`{}`),
	}
	node := &model.WorkflowNode{ID: "sub-bad-map", Type: "sub_workflow"}
	payload := InvokeSubWorkflowPayload{
		TargetKind:       SubWorkflowTargetDAG,
		TargetWorkflowID: targetWF,
		InputMapping:     json.RawMessage(`{"x":"{{$parent.dataStore.missing.path}}"}`),
	}
	raw, _ := json.Marshal(payload)

	_, err := applier.Apply(context.Background(), exec, uuid.New(), node, []Effect{{Kind: EffectInvokeSubWorkflow, Payload: raw}})
	if err == nil {
		t.Fatal("Apply() returned nil, want unresolved-mapping rejection")
	}
	var invErr *SubWorkflowInvocationError
	if !errors.As(err, &invErr) || invErr.Reason != SubWorkflowRejectUnresolvedMap {
		t.Errorf("Apply() err reason = %v, want unresolved_mapping", err)
	}
	if len(eng.started) != 0 {
		t.Errorf("engine started despite unresolved mapping")
	}
}

func TestApplier_InvokeSubWorkflow_UnknownTargetKindRejected(t *testing.T) {
	projectID := uuid.New()
	// Register only DAG engine; payload declares "plugin".
	eng := &fakeSubWorkflowEngine{
		kind:            SubWorkflowTargetDAG,
		projectByTarget: map[string]uuid.UUID{},
	}
	applier, _ := newSubWorkflowApplier(t, eng, &capturingLinkRepo{}, nil)

	exec := &model.WorkflowExecution{ID: uuid.New(), ProjectID: projectID}
	node := &model.WorkflowNode{ID: "sub-missing", Type: "sub_workflow"}
	payload := InvokeSubWorkflowPayload{
		TargetKind:       SubWorkflowTargetPlugin,
		TargetWorkflowID: "some-plugin-id",
	}
	raw, _ := json.Marshal(payload)

	_, err := applier.Apply(context.Background(), exec, uuid.New(), node, []Effect{{Kind: EffectInvokeSubWorkflow, Payload: raw}})
	if err == nil {
		t.Fatal("Apply() returned nil, want unknown-target rejection")
	}
	var invErr *SubWorkflowInvocationError
	if !errors.As(err, &invErr) || invErr.Reason != SubWorkflowRejectUnknownTarget {
		t.Errorf("Apply() err reason = %v, want unknown_target", err)
	}
}
