package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/workflow/nodetypes"
)

// stubPluginEngine drives the PluginSubWorkflowEngine-shaped seam without
// requiring a real WorkflowExecutionService. It tracks Start calls so the test
// can assert the applier dispatched to the plugin kind and returns a
// deterministic run id for terminal-bridge correlation.
type stubPluginEngine struct {
	projectByTarget map[string]uuid.UUID
	runID           uuid.UUID
	startCalls      []struct {
		target string
		seed   map[string]any
		inv    nodetypes.SubWorkflowInvocation
	}
}

func (s *stubPluginEngine) Kind() nodetypes.SubWorkflowTargetKind {
	return nodetypes.SubWorkflowTargetPlugin
}

func (s *stubPluginEngine) Validate(ctx context.Context, target string, inv nodetypes.SubWorkflowInvocation) error {
	pid, ok := s.projectByTarget[target]
	if !ok {
		return &nodetypes.SubWorkflowInvocationError{
			Reason:  nodetypes.SubWorkflowRejectUnknownTarget,
			Message: "unknown plugin target",
		}
	}
	if pid != inv.ProjectID {
		return &nodetypes.SubWorkflowInvocationError{
			Reason:  nodetypes.SubWorkflowRejectCrossProject,
			Message: "cross-project plugin target",
		}
	}
	return nil
}

func (s *stubPluginEngine) Start(ctx context.Context, target string, seed map[string]any, inv nodetypes.SubWorkflowInvocation) (uuid.UUID, error) {
	s.startCalls = append(s.startCalls, struct {
		target string
		seed   map[string]any
		inv    nodetypes.SubWorkflowInvocation
	}{target: target, seed: seed, inv: inv})
	if s.runID == uuid.Nil {
		s.runID = uuid.New()
	}
	return s.runID, nil
}

// TestDAGSubWorkflow_PluginChild_FullLoop exercises the full parent→plugin
// invocation loop: a DAG workflow whose `sub_workflow` node targets a legacy
// plugin parks until the plugin run reaches a terminal state, then resumes with
// the child's outputs materialized into its dataStore.
//
// Wiring mirrors routes.go at the granularity that matters for this flow:
//   - real NodeTypeRegistry with the real sub_workflow handler
//   - real EffectApplier with a real SubWorkflowEngineRegistry + LinkRepoAdapter
//   - real DAGWorkflowService driving node dispatch + parent resume
//   - real PluginSubWorkflowTerminalBridge flowing plugin-run terminal events
//     back into the DAG service
//   - stub plugin engine (no actual plugin runtime) to keep the test hermetic
func TestDAGSubWorkflow_PluginChild_FullLoop(t *testing.T) {
	ctx := context.Background()

	projectID := uuid.New()
	parentWfID := uuid.New()
	pluginID := "workflow.demo.adder"
	pluginRunID := uuid.New()

	// --- Parent DAG definition: trigger → sub_workflow(plugin).
	parentDef := &model.WorkflowDefinition{
		ID:        parentWfID,
		ProjectID: projectID,
		Status:    model.WorkflowDefStatusActive,
		Nodes: json.RawMessage(`[
			{"id":"trg","type":"trigger"},
			{"id":"sub-plugin","type":"sub_workflow","config":{"targetKind":"plugin","targetWorkflowId":"` + pluginID + `","inputMapping":{"hello":"{{$event.name}}"}}}
		]`),
		Edges: json.RawMessage(`[{"source":"trg","target":"sub-plugin"}]`),
	}
	defRepo := &fakeDefRepo{byID: map[uuid.UUID]*model.WorkflowDefinition{parentWfID: parentDef}}
	execRepo := &fakeExecRepo{byID: map[uuid.UUID]*model.WorkflowExecution{}}
	nodeRepo := &fakeNodeRepo{byExec: map[uuid.UUID][]*model.WorkflowNodeExecution{}}
	linkRepo := &fakeLinkRepo{}

	// --- Registry + built-in sub_workflow handler.
	registry := nodetypes.NewRegistry(nil)
	if err := registry.RegisterBuiltin("sub_workflow", nodetypes.SubWorkflowHandler{}); err != nil {
		t.Fatalf("RegisterBuiltin: %v", err)
	}
	if err := registry.RegisterBuiltin("trigger", nodetypes.TriggerHandler{}); err != nil {
		t.Fatalf("RegisterBuiltin trigger: %v", err)
	}

	// --- Applier + sub-workflow wiring.
	linkAdapter := NewSubWorkflowLinkRepoAdapter(linkRepo)
	pluginEng := &stubPluginEngine{
		projectByTarget: map[string]uuid.UUID{pluginID: projectID},
		runID:           pluginRunID,
	}
	applier := &nodetypes.EffectApplier{
		SubWorkflowEngines: nodetypes.NewSubWorkflowEngineRegistry(pluginEng),
		SubWorkflowLinks:   linkAdapter,
	}

	// --- Real DAG service.
	svc := NewDAGWorkflowService(defRepo, execRepo, nodeRepo, nil, nil, registry, applier)
	svc.SetParentLinkRepo(linkRepo)
	// Guard attaches to the same link adapter + svc (as WorkflowExecutionLookup).
	applier.SubWorkflowGuard = nodetypes.NewRecursionGuard(linkAdapter, svc, nodetypes.MaxSubWorkflowDepth)

	// --- Start parent execution. Trigger node completes, sub_workflow node
	// dispatches to the plugin engine, and the parent parks.
	parentExec, err := svc.StartExecution(ctx, parentWfID, nil, StartOptions{Seed: map[string]any{
		"name": "world",
	}})
	if err != nil {
		t.Fatalf("StartExecution: %v", err)
	}

	// --- Assertions on the invocation side.
	if len(pluginEng.startCalls) != 1 {
		t.Fatalf("plugin engine Start call count = %d, want 1", len(pluginEng.startCalls))
	}
	if pluginEng.startCalls[0].target != pluginID {
		t.Errorf("plugin engine Start target = %q, want %q", pluginEng.startCalls[0].target, pluginID)
	}
	if got := pluginEng.startCalls[0].seed["hello"]; got != "world" {
		t.Errorf("plugin engine seed hello = %v, want world", got)
	}
	if len(linkRepo.links) != 1 {
		t.Fatalf("parent link row count = %d, want 1", len(linkRepo.links))
	}
	link := linkRepo.links[0]
	if link.ChildRunID != pluginRunID {
		t.Errorf("link child run id = %s, want %s", link.ChildRunID, pluginRunID)
	}
	if link.ChildEngineKind != model.SubWorkflowEnginePlugin {
		t.Errorf("link engine kind = %s, want %s", link.ChildEngineKind, model.SubWorkflowEnginePlugin)
	}
	if link.Status != model.SubWorkflowLinkStatusRunning {
		t.Errorf("link status = %s, want running", link.Status)
	}

	// Parent exec should be paused, sub_workflow node parked.
	refreshed, _ := execRepo.GetExecution(ctx, parentExec.ID)
	if refreshed.Status != model.WorkflowExecStatusPaused {
		t.Errorf("parent exec status = %s, want paused", refreshed.Status)
	}
	var parkedNodeExec *model.WorkflowNodeExecution
	for _, ne := range nodeRepo.byExec[parentExec.ID] {
		if ne.NodeID == "sub-plugin" {
			parkedNodeExec = ne
			break
		}
	}
	if parkedNodeExec == nil || parkedNodeExec.Status != model.NodeExecAwaitingSubWorkflow {
		t.Fatalf("parent sub-plugin node status = %+v, want awaiting_sub_workflow", parkedNodeExec)
	}

	// --- Now simulate the plugin runtime's terminal-state emission. The bridge
	// is the production wiring that connects plugin-run completion to the DAG
	// service's ResumeParentFromPluginChild.
	bridge := &PluginSubWorkflowTerminalBridge{DAG: svc}
	bridge.OnPluginRunTerminal(ctx, &model.WorkflowPluginRun{
		ID:     pluginRunID,
		Status: model.WorkflowRunStatusCompleted,
		Steps: []model.WorkflowStepRun{
			{StepID: "step-1", Output: map[string]any{"greeting": "hello world"}},
		},
	})

	// --- Assertions on the resume side.
	if parkedNodeExec.Status != model.NodeExecCompleted {
		t.Errorf("post-resume parent node status = %s, want completed", parkedNodeExec.Status)
	}
	if link.Status != model.SubWorkflowLinkStatusCompleted {
		t.Errorf("post-resume link status = %s, want completed", link.Status)
	}

	// Parent's dataStore must now carry the child outputs under the parent node
	// id. The envelope shape mirrors the design: dataStore[<parent-node-id>] =
	// {subWorkflow: {runId, engine, outputs, outcome}}.
	refreshed, _ = execRepo.GetExecution(ctx, parentExec.ID)
	var parentDS map[string]any
	_ = json.Unmarshal(refreshed.DataStore, &parentDS)
	subNodeResult, ok := parentDS["sub-plugin"].(map[string]any)
	if !ok {
		t.Fatalf("parent dataStore missing sub-plugin entry: %v", parentDS)
	}
	outputWrapper, ok := subNodeResult["output"].(map[string]any)
	if !ok {
		t.Fatalf("parent sub-plugin entry has no output envelope: %+v", subNodeResult)
	}
	subEnvelope, ok := outputWrapper["subWorkflow"].(map[string]any)
	if !ok {
		t.Fatalf("parent sub-plugin output has no subWorkflow envelope: %+v", outputWrapper)
	}
	if engine, _ := subEnvelope["engine"].(string); engine != model.SubWorkflowEnginePlugin {
		t.Errorf("subWorkflow envelope engine = %v, want plugin", subEnvelope["engine"])
	}
	outputs, ok := subEnvelope["outputs"].(map[string]any)
	if !ok {
		t.Fatalf("subWorkflow envelope outputs missing: %+v", subEnvelope)
	}
	step1, ok := outputs["step-1"].(map[string]any)
	if !ok {
		t.Fatalf("subWorkflow envelope outputs missing step-1: %+v", outputs)
	}
	if got, _ := step1["output"].(map[string]any)["greeting"].(string); got != "hello world" {
		t.Errorf("child output greeting = %v, want hello world", got)
	}
}
