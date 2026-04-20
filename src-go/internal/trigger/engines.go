package trigger

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
)

// TriggerRun identifies a workflow run fired by a TargetEngine in response
// to a matched trigger event. It is engine-agnostic so downstream consumers
// (outcome logging, audit, WS broadcast) need not switch on engine type.
type TriggerRun struct {
	Engine model.TriggerTargetKind `json:"engine"`
	RunID  uuid.UUID               `json:"runId"`
}

// TargetEngine adapts a workflow execution engine to the unified trigger
// dispatch pipeline. Each engine registers exactly one adapter keyed by
// its Kind(); the Router looks the adapter up per trigger row.
//
// The adapter receives the full *model.WorkflowTrigger so it can interpret
// engine-specific identifiers (WorkflowID for DAG, PluginID for plugin)
// without the Router having to understand either.
type TargetEngine interface {
	Kind() model.TriggerTargetKind
	Start(ctx context.Context, trig *model.WorkflowTrigger, seed map[string]any) (TriggerRun, error)
}

// -------------------------------------------------------------------------
// DAG adapter
// -------------------------------------------------------------------------

// DAGEngineStarter is the minimum dependency DAGEngineAdapter needs.
// *service.DAGWorkflowService satisfies it structurally.
type DAGEngineStarter interface {
	StartExecution(ctx context.Context, workflowID uuid.UUID, taskID *uuid.UUID, opts service.StartOptions) (*model.WorkflowExecution, error)
}

// DAGEngineAdapter wraps *service.DAGWorkflowService so the trigger router
// can start DAG workflow executions. Preserves the existing StartOptions.Seed
// and TriggeredBy plumbing.
type DAGEngineAdapter struct {
	starter DAGEngineStarter
}

// NewDAGEngineAdapter returns a new adapter wrapping starter.
func NewDAGEngineAdapter(starter DAGEngineStarter) *DAGEngineAdapter {
	return &DAGEngineAdapter{starter: starter}
}

// Kind returns the target kind this adapter handles.
func (a *DAGEngineAdapter) Kind() model.TriggerTargetKind { return model.TriggerTargetDAG }

// Start resolves the DAG workflow_id on the trigger row and calls StartExecution.
// Passes through the trigger's ActingEmployeeID (may be nil) so the started
// WorkflowExecution persists it as its run-level attribution default.
func (a *DAGEngineAdapter) Start(ctx context.Context, trig *model.WorkflowTrigger, seed map[string]any) (TriggerRun, error) {
	if a.starter == nil {
		return TriggerRun{}, fmt.Errorf("dag engine adapter: starter is not configured")
	}
	if trig == nil || trig.WorkflowID == nil {
		return TriggerRun{}, fmt.Errorf("dag engine adapter: trigger missing workflow_id")
	}
	triggerID := trig.ID
	exec, err := a.starter.StartExecution(ctx, *trig.WorkflowID, nil, service.StartOptions{
		Seed:             seed,
		TriggeredBy:      &triggerID,
		ActingEmployeeID: trig.ActingEmployeeID,
	})
	if err != nil {
		return TriggerRun{}, err
	}
	return TriggerRun{Engine: model.TriggerTargetDAG, RunID: exec.ID}, nil
}

// -------------------------------------------------------------------------
// Plugin adapter
// -------------------------------------------------------------------------

// PluginEngineStarter is the minimum dependency PluginEngineAdapter needs.
// *service.WorkflowExecutionService satisfies it once StartTriggered /
// StartTriggeredWithEmployee are exported.
type PluginEngineStarter interface {
	StartTriggered(ctx context.Context, pluginID string, seed map[string]any, triggerID uuid.UUID) (*model.WorkflowPluginRun, error)
	StartTriggeredWithEmployee(ctx context.Context, pluginID string, seed map[string]any, triggerID uuid.UUID, actingEmployeeID *uuid.UUID) (*model.WorkflowPluginRun, error)
	StartTriggeredForProject(ctx context.Context, pluginID string, seed map[string]any, triggerID uuid.UUID, actingEmployeeID *uuid.UUID, projectID uuid.UUID) (*model.WorkflowPluginRun, error)
}

// PluginEngineAdapter wraps the workflow plugin runtime's trigger-start seam
// so the trigger router can fire legacy workflow plugin runs uniformly.
type PluginEngineAdapter struct {
	starter PluginEngineStarter
}

// NewPluginEngineAdapter returns a new adapter wrapping starter.
func NewPluginEngineAdapter(starter PluginEngineStarter) *PluginEngineAdapter {
	return &PluginEngineAdapter{starter: starter}
}

// Kind returns the target kind this adapter handles.
func (a *PluginEngineAdapter) Kind() model.TriggerTargetKind { return model.TriggerTargetPlugin }

// Start resolves the plugin_id on the trigger row and calls the plugin
// runtime's trigger-start seam. The returned RunID is the workflow_plugin_run.ID.
// Passes through the trigger's ActingEmployeeID so the legacy plugin run is
// stamped with the run-level attribution default.
func (a *PluginEngineAdapter) Start(ctx context.Context, trig *model.WorkflowTrigger, seed map[string]any) (TriggerRun, error) {
	if a.starter == nil {
		return TriggerRun{}, fmt.Errorf("plugin engine adapter: starter is not configured")
	}
	if trig == nil || trig.PluginID == "" {
		return TriggerRun{}, fmt.Errorf("plugin engine adapter: trigger missing plugin_id")
	}
	run, err := a.starter.StartTriggeredForProject(ctx, trig.PluginID, seed, trig.ID, trig.ActingEmployeeID, trig.ProjectID)
	if err != nil {
		return TriggerRun{}, err
	}
	return TriggerRun{Engine: model.TriggerTargetPlugin, RunID: run.ID}, nil
}
