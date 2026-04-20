package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/workflow/nodetypes"
)

// DAGSubWorkflowEngine adapts *DAGWorkflowService so the nodetypes applier can
// dispatch a `sub_workflow` node whose target_kind is "dag". Validate performs
// project-match + existence check via the definition repo; Start reuses the
// same StartExecution seam the trigger router calls — so trigger-fired and
// sub-workflow-fired DAG runs take the same code path.
type DAGSubWorkflowEngine struct {
	svc     *DAGWorkflowService
	defRepo DAGWorkflowDefinitionRepo
}

// NewDAGSubWorkflowEngine wires the DAG adapter. svc is the DAG workflow
// service used to actually start child runs; defRepo is the definition store
// used for project-match validation. Nil inputs produce errors at dispatch
// rather than at wiring time so tests that stub the service can still
// construct the registry.
func NewDAGSubWorkflowEngine(svc *DAGWorkflowService, defRepo DAGWorkflowDefinitionRepo) *DAGSubWorkflowEngine {
	return &DAGSubWorkflowEngine{svc: svc, defRepo: defRepo}
}

// Kind declares the target kind this adapter handles.
func (e *DAGSubWorkflowEngine) Kind() nodetypes.SubWorkflowTargetKind {
	return nodetypes.SubWorkflowTargetDAG
}

// Validate resolves the target workflow definition and rejects unknown
// targets or cross-project references with the structured error shape the
// applier expects.
func (e *DAGSubWorkflowEngine) Validate(ctx context.Context, target string, inv nodetypes.SubWorkflowInvocation) error {
	if e.defRepo == nil {
		return &nodetypes.SubWorkflowInvocationError{
			Reason:  nodetypes.SubWorkflowRejectUnknownTarget,
			Message: "dag sub-workflow engine: definition repo is not configured",
		}
	}
	targetID, err := uuid.Parse(target)
	if err != nil {
		return &nodetypes.SubWorkflowInvocationError{
			Reason:  nodetypes.SubWorkflowRejectUnknownTarget,
			Message: fmt.Sprintf("target %q is not a valid UUID", target),
		}
	}
	def, err := e.defRepo.GetByID(ctx, targetID)
	if err != nil {
		return &nodetypes.SubWorkflowInvocationError{
			Reason:  nodetypes.SubWorkflowRejectUnknownTarget,
			Message: fmt.Sprintf("workflow %s: %v", target, err),
		}
	}
	if def.ProjectID != inv.ProjectID {
		return &nodetypes.SubWorkflowInvocationError{
			Reason:  nodetypes.SubWorkflowRejectCrossProject,
			Message: fmt.Sprintf("target workflow %s belongs to project %s but parent is in %s", target, def.ProjectID, inv.ProjectID),
		}
	}
	return nil
}

// Start launches a child DAG execution via the DAG workflow service. The
// acting-employee attribution is inherited from the parent so the child's
// runtime agents attribute correctly without the caller having to rebind.
func (e *DAGSubWorkflowEngine) Start(ctx context.Context, target string, seed map[string]any, inv nodetypes.SubWorkflowInvocation) (uuid.UUID, error) {
	if e.svc == nil {
		return uuid.Nil, fmt.Errorf("dag sub-workflow engine: service is not configured")
	}
	targetID, err := uuid.Parse(target)
	if err != nil {
		return uuid.Nil, fmt.Errorf("dag sub-workflow engine: target %q is not a UUID: %w", target, err)
	}
	exec, err := e.svc.StartExecution(ctx, targetID, nil, StartOptions{
		Seed:             seed,
		ActingEmployeeID: inv.ActingEmployeeID,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return exec.ID, nil
}

// PluginSubWorkflowEngine adapts *WorkflowExecutionService so the nodetypes
// applier can dispatch a `sub_workflow` node whose target_kind is "plugin".
// The plugin runtime stores run state in-memory today; the adapter reuses the
// existing StartTriggered seam so trigger-fired and sub-workflow-fired plugin
// runs are indistinguishable to downstream bookkeeping.
type PluginSubWorkflowEngine struct {
	svc     *WorkflowExecutionService
	plugins WorkflowPluginCatalog
}

// NewPluginSubWorkflowEngine wires the plugin adapter.
func NewPluginSubWorkflowEngine(svc *WorkflowExecutionService, plugins WorkflowPluginCatalog) *PluginSubWorkflowEngine {
	return &PluginSubWorkflowEngine{svc: svc, plugins: plugins}
}

// Kind declares the target kind this adapter handles.
func (e *PluginSubWorkflowEngine) Kind() nodetypes.SubWorkflowTargetKind {
	return nodetypes.SubWorkflowTargetPlugin
}

// Validate resolves the target plugin id and enforces same-project if the
// plugin has a project scope. Plugins without a scope (global catalog) match
// any project — cross-project rejection only fires when the plugin's
// CurrentInstance.ProjectID is explicitly set to a different project.
func (e *PluginSubWorkflowEngine) Validate(ctx context.Context, target string, inv nodetypes.SubWorkflowInvocation) error {
	if e.plugins == nil {
		return &nodetypes.SubWorkflowInvocationError{
			Reason:  nodetypes.SubWorkflowRejectUnknownTarget,
			Message: "plugin sub-workflow engine: plugin catalog is not configured",
		}
	}
	record, err := e.plugins.GetByID(ctx, target)
	if err != nil {
		return &nodetypes.SubWorkflowInvocationError{
			Reason:  nodetypes.SubWorkflowRejectUnknownTarget,
			Message: fmt.Sprintf("plugin %q: %v", target, err),
		}
	}
	if record.Kind != model.PluginKindWorkflow {
		return &nodetypes.SubWorkflowInvocationError{
			Reason:  nodetypes.SubWorkflowRejectUnknownTarget,
			Message: fmt.Sprintf("plugin %q is not a workflow plugin", target),
		}
	}
	if record.CurrentInstance != nil && record.CurrentInstance.ProjectID != "" {
		pluginProject, parseErr := uuid.Parse(record.CurrentInstance.ProjectID)
		if parseErr == nil && pluginProject != inv.ProjectID {
			return &nodetypes.SubWorkflowInvocationError{
				Reason:  nodetypes.SubWorkflowRejectCrossProject,
				Message: fmt.Sprintf("plugin %s belongs to project %s but parent is in %s", target, pluginProject, inv.ProjectID),
			}
		}
	}
	return nil
}

// Start launches a child workflow plugin run via the workflow execution
// service. Sub-workflow invocation of plugins currently reuses the trigger
// start seam — the `triggerID` slot carries the parent execution id as a
// synthetic identifier so downstream audit can distinguish sub-workflow
// plugin runs without adding a new code path.
func (e *PluginSubWorkflowEngine) Start(ctx context.Context, target string, seed map[string]any, inv nodetypes.SubWorkflowInvocation) (uuid.UUID, error) {
	if e.svc == nil {
		return uuid.Nil, fmt.Errorf("plugin sub-workflow engine: service is not configured")
	}
	run, err := e.svc.StartTriggeredWithEmployee(ctx, target, seed, inv.ParentExecutionID, inv.ActingEmployeeID)
	if err != nil {
		return uuid.Nil, err
	}
	return run.ID, nil
}
