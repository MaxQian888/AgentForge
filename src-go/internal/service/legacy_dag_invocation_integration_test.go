package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/workflow/nodetypes"
)

// stubPluginRunResumer is a PluginRunResumer implementation for verifying the
// DAG service's terminal hook routes correctly based on parent_kind. Captures
// invocation so the test can assert routing without wiring a full plugin
// runtime.
type stubPluginRunResumer struct {
	resumeCalls []struct {
		parentRunID, childRunID uuid.UUID
		outcome                 string
		outputs                 map[string]any
	}
	cancelCalls []uuid.UUID
}

func (s *stubPluginRunResumer) ResumeParkedDAGChild(_ context.Context, parentRunID, childRunID uuid.UUID, outcome string, childOutputs map[string]any) error {
	s.resumeCalls = append(s.resumeCalls, struct {
		parentRunID, childRunID uuid.UUID
		outcome                 string
		outputs                 map[string]any
	}{parentRunID, childRunID, outcome, childOutputs})
	return nil
}

func (s *stubPluginRunResumer) CancelRun(_ context.Context, runID uuid.UUID) error {
	s.cancelCalls = append(s.cancelCalls, runID)
	return nil
}

// TestDAGService_TryResumeParentFromDAGChild_PluginRunParent: when a DAG
// child completes and its parent link has parent_kind='plugin_run', the DAG
// service routes the terminal notification to the configured PluginRunResumer
// instead of mutating DAG state (bridge-legacy-to-dag-invocation).
func TestDAGService_TryResumeParentFromDAGChild_PluginRunParent(t *testing.T) {
	svc, execRepo, _, linkRepo := newResumeTestService(t)

	pluginRunID := uuid.New()
	childExecID := uuid.New()
	execRepo.byID[childExecID] = &model.WorkflowExecution{
		ID:        childExecID,
		DataStore: []byte(`{"finished":"yes"}`),
	}
	linkRepo.links = append(linkRepo.links, &model.WorkflowRunParentLink{
		ID:                uuid.New(),
		ParentExecutionID: pluginRunID,
		ParentKind:        model.SubWorkflowParentKindPluginRun,
		ParentNodeID:      "step-1",
		ChildEngineKind:   model.SubWorkflowEngineDAG,
		ChildRunID:        childExecID,
		Status:            model.SubWorkflowLinkStatusRunning,
		StartedAt:         time.Now().UTC(),
	})

	resumer := &stubPluginRunResumer{}
	svc.SetPluginRunResumer(resumer)

	svc.tryResumeParentFromDAGChild(context.Background(), execRepo.byID[childExecID], model.SubWorkflowLinkStatusCompleted)

	if len(resumer.resumeCalls) != 1 {
		t.Fatalf("expected 1 plugin resume call, got %d", len(resumer.resumeCalls))
	}
	call := resumer.resumeCalls[0]
	if call.parentRunID != pluginRunID {
		t.Errorf("resume parent run = %s, want %s", call.parentRunID, pluginRunID)
	}
	if call.childRunID != childExecID {
		t.Errorf("resume child run = %s, want %s", call.childRunID, childExecID)
	}
	if call.outcome != model.SubWorkflowLinkStatusCompleted {
		t.Errorf("resume outcome = %s, want completed", call.outcome)
	}
	if got, _ := call.outputs["finished"].(string); got != "yes" {
		t.Errorf("child outputs = %+v, want finished=yes", call.outputs)
	}
	// Link status should transition to completed even though the DAG resume
	// path was not taken.
	if len(linkRepo.statusUpdates) == 0 {
		t.Errorf("expected link status to be updated")
	}
}

// TestDAGService_TryResumeParentFromDAGChild_DAGParentStillWorks: parent_kind
// 'dag_execution' (the default) continues to use the DAG-native resume path.
func TestDAGService_TryResumeParentFromDAGChild_DAGParentStillWorks(t *testing.T) {
	svc, execRepo, nodeRepo, linkRepo := newResumeTestService(t)

	parentExecID := uuid.New()
	parentWfID := uuid.New()
	childExecID := uuid.New()
	execRepo.byID[parentExecID] = &model.WorkflowExecution{
		ID:         parentExecID,
		WorkflowID: parentWfID,
		Status:     model.WorkflowExecStatusPaused,
		DataStore:  []byte(`{}`),
		Context:    []byte(`{}`),
	}
	execRepo.byID[childExecID] = &model.WorkflowExecution{
		ID:        childExecID,
		DataStore: []byte(`{}`),
	}
	parkedNode := &model.WorkflowNodeExecution{
		ID:          uuid.New(),
		ExecutionID: parentExecID,
		NodeID:      "sub-1",
		Status:      model.NodeExecAwaitingSubWorkflow,
	}
	nodeRepo.byExec[parentExecID] = []*model.WorkflowNodeExecution{parkedNode}
	linkRepo.links = append(linkRepo.links, &model.WorkflowRunParentLink{
		ID:                uuid.New(),
		ParentExecutionID: parentExecID,
		ParentKind:        model.SubWorkflowParentKindDAGExecution,
		ParentNodeID:      "sub-1",
		ChildEngineKind:   model.SubWorkflowEngineDAG,
		ChildRunID:        childExecID,
		Status:            model.SubWorkflowLinkStatusRunning,
	})
	svc.defRepo.(*fakeDefRepo).byID[parentWfID] = &model.WorkflowDefinition{
		ID:     parentWfID,
		Status: model.WorkflowDefStatusActive,
		Nodes:  []byte(`[{"id":"sub-1","type":"sub_workflow"}]`),
		Edges:  []byte(`[]`),
	}

	// Attach a resumer that MUST NOT be called.
	resumer := &stubPluginRunResumer{}
	svc.SetPluginRunResumer(resumer)

	svc.tryResumeParentFromDAGChild(context.Background(), execRepo.byID[childExecID], model.SubWorkflowLinkStatusCompleted)

	if len(resumer.resumeCalls) != 0 {
		t.Errorf("plugin resume should not be called for dag_execution parent; got %d calls", len(resumer.resumeCalls))
	}
	if parkedNode.Status != model.NodeExecCompleted {
		t.Errorf("parent DAG node status = %s, want completed", parkedNode.Status)
	}
	_ = nodetypes.MaxSubWorkflowDepth
}
