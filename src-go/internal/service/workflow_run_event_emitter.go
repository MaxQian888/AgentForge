package service

import (
	"context"
	"time"

	"github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
)

// WorkflowRunEventEmitter publishes canonical workflow.run.* events alongside
// the engine-native channels. It is an additive fan-out layer — callers keep
// emitting workflow.execution.* (DAG) and plugin-native lifecycle events, and
// invoke Emit* methods to also produce the cross-engine workflow.run.* events
// consumed by the unified workspace UI.
type WorkflowRunEventEmitter struct {
	bus eventbus.Publisher
}

// NewWorkflowRunEventEmitter returns an emitter bound to the supplied
// publisher. A nil publisher turns every emission into a no-op, which keeps
// legacy test wiring working without this dep.
func NewWorkflowRunEventEmitter(bus eventbus.Publisher) *WorkflowRunEventEmitter {
	return &WorkflowRunEventEmitter{bus: bus}
}

// EmitDAGStatusChanged publishes a workflow.run.status_changed event with the
// canonical row shape for a DAG execution transition.
func (e *WorkflowRunEventEmitter) EmitDAGStatusChanged(ctx context.Context, exec *model.WorkflowExecution) {
	if e == nil || e.bus == nil || exec == nil {
		return
	}
	row := rowFromDAGExecutionMinimal(exec)
	_ = eventbus.PublishLegacy(ctx, e.bus, eventbus.EventWorkflowRunStatusChanged, exec.ProjectID.String(), row)
}

// EmitDAGTerminal publishes a workflow.run.terminal event for a DAG execution
// that has reached completed/failed/cancelled.
func (e *WorkflowRunEventEmitter) EmitDAGTerminal(ctx context.Context, exec *model.WorkflowExecution) {
	if e == nil || e.bus == nil || exec == nil {
		return
	}
	row := rowFromDAGExecutionMinimal(exec)
	_ = eventbus.PublishLegacy(ctx, e.bus, eventbus.EventWorkflowRunTerminal, exec.ProjectID.String(), row)
}

// EmitPluginStatusChanged publishes a workflow.run.status_changed event for a
// plugin run state transition. Project scope is taken from the run directly.
func (e *WorkflowRunEventEmitter) EmitPluginStatusChanged(ctx context.Context, run *model.WorkflowPluginRun) {
	if e == nil || e.bus == nil || run == nil {
		return
	}
	row := rowFromPluginRunMinimal(run)
	_ = eventbus.PublishLegacy(ctx, e.bus, eventbus.EventWorkflowRunStatusChanged, run.ProjectID.String(), row)
}

// EmitPluginTerminal publishes a workflow.run.terminal event for a plugin run
// that has reached completed/failed/cancelled.
func (e *WorkflowRunEventEmitter) EmitPluginTerminal(ctx context.Context, run *model.WorkflowPluginRun) {
	if e == nil || e.bus == nil || run == nil {
		return
	}
	row := rowFromPluginRunMinimal(run)
	_ = eventbus.PublishLegacy(ctx, e.bus, eventbus.EventWorkflowRunTerminal, run.ProjectID.String(), row)
}

// rowFromDAGExecutionMinimal builds a canonical UnifiedRunRow without the
// optional workflow-name / parent-link enrichments. The WS payload only needs
// the transition state; the list endpoint fills in names via its own repo.
func rowFromDAGExecutionMinimal(exec *model.WorkflowExecution) UnifiedRunRow {
	row := UnifiedRunRow{
		Engine: UnifiedRunEngineDAG,
		RunID:  exec.ID.String(),
		WorkflowRef: UnifiedRunWorkflowRef{
			ID: exec.WorkflowID.String(),
		},
		Status: normalizeDAGExecutionStatus(exec.Status),
	}
	if exec.StartedAt != nil {
		row.StartedAt = exec.StartedAt.UTC().Format(time.RFC3339Nano)
	} else {
		row.StartedAt = exec.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	if exec.CompletedAt != nil {
		row.CompletedAt = exec.CompletedAt.UTC().Format(time.RFC3339Nano)
	}
	if exec.ActingEmployeeID != nil {
		row.ActingEmployeeID = exec.ActingEmployeeID.String()
	}
	if exec.TriggeredBy != nil {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "trigger", Ref: exec.TriggeredBy.String()}
	} else if exec.TaskID != nil {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "task", Ref: exec.TaskID.String()}
	} else {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "manual"}
	}
	return row
}

// rowFromPluginRunMinimal is the plugin-run counterpart to
// rowFromDAGExecutionMinimal, used by the WS emitter.
func rowFromPluginRunMinimal(run *model.WorkflowPluginRun) UnifiedRunRow {
	row := UnifiedRunRow{
		Engine: UnifiedRunEnginePlugin,
		RunID:  run.ID.String(),
		WorkflowRef: UnifiedRunWorkflowRef{
			ID:   run.PluginID,
			Name: run.PluginID,
		},
		Status:    normalizePluginRunStatus(run.Status),
		StartedAt: run.StartedAt.UTC().Format(time.RFC3339Nano),
	}
	if run.CompletedAt != nil {
		row.CompletedAt = run.CompletedAt.UTC().Format(time.RFC3339Nano)
	}
	if run.ActingEmployeeID != nil {
		row.ActingEmployeeID = run.ActingEmployeeID.String()
	}
	if run.TriggerID != nil {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "trigger", Ref: run.TriggerID.String()}
	} else if src, _ := run.Trigger["source"].(string); src == "workflow.trigger" {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "trigger"}
	} else if src == "sub_workflow" {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "sub_workflow"}
	} else if src == "task.trigger" {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "task"}
	} else {
		row.TriggeredBy = UnifiedRunTriggeredBy{Kind: "manual"}
	}
	return row
}
