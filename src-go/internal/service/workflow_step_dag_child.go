package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// WorkflowStepDAGChildAdapter satisfies WorkflowDAGChildStarter by wrapping a
// *DAGWorkflowService. Introduced by bridge-legacy-to-dag-invocation so a
// legacy plugin's `workflow` step can dispatch a DAG child without the step
// router importing the full DAG service surface.
type WorkflowStepDAGChildAdapter struct {
	dag *DAGWorkflowService
}

// NewWorkflowStepDAGChildAdapter wraps dag. A nil dag produces adapter calls
// that return a configuration error at dispatch time; tests that don't exercise
// the cross-engine path can safely pass nil.
func NewWorkflowStepDAGChildAdapter(dag *DAGWorkflowService) *WorkflowStepDAGChildAdapter {
	return &WorkflowStepDAGChildAdapter{dag: dag}
}

// Start delegates to StartExecution with the parent's acting-employee id
// forwarded as the run-level attribution default. The seed is placed under
// `$event` by the DAG service so downstream nodes read it from a predictable
// path regardless of the starter.
func (a *WorkflowStepDAGChildAdapter) Start(ctx context.Context, targetWorkflowID uuid.UUID, seed map[string]any, inv WorkflowDAGChildInvocation) (uuid.UUID, error) {
	if a == nil || a.dag == nil {
		return uuid.Nil, fmt.Errorf("workflow step: dag service is not configured")
	}
	exec, err := a.dag.StartExecution(ctx, targetWorkflowID, nil, StartOptions{
		Seed:             seed,
		ActingEmployeeID: inv.ActingEmployeeID,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return exec.ID, nil
}

// Cancel delegates to DAGWorkflowService.CancelExecution. Used by the plugin
// runtime's cancellation cascade when the parent plugin run is stopped while
// a DAG child is still parked.
func (a *WorkflowStepDAGChildAdapter) Cancel(ctx context.Context, runID uuid.UUID) error {
	if a == nil || a.dag == nil {
		return fmt.Errorf("workflow step: dag service is not configured")
	}
	return a.dag.CancelExecution(ctx, runID)
}
