package service

import (
	"context"
	"fmt"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/plugin"
)

// SequentialExecutor runs workflow steps one-by-one in declaration order.
// It wraps the existing WorkflowStepExecutor (typically WorkflowStepRouterExecutor).
// On step failure it emits "step_failed" + terminal "failed" and stops.
type SequentialExecutor struct {
	stepRouter WorkflowStepExecutor
}

func NewSequentialExecutor(router WorkflowStepExecutor) *SequentialExecutor {
	return &SequentialExecutor{stepRouter: router}
}

func (e *SequentialExecutor) Mode() model.WorkflowProcessMode {
	return model.WorkflowProcessSequential
}

// Cancel is a no-op: sequential execution honors context cancellation
// directly inside Execute, so callers cancel by cancelling the ctx they
// passed to Execute.
func (e *SequentialExecutor) Cancel(_ context.Context, _ string) error {
	return nil
}

func (e *SequentialExecutor) Execute(ctx context.Context, plan plugin.WorkflowPlan) (<-chan plugin.WorkflowEvent, error) {
	if plan.Spec == nil {
		return nil, fmt.Errorf("sequential workflow %q has no spec", plan.PluginID)
	}
	ch := make(chan plugin.WorkflowEvent, 16)
	go func() {
		defer close(ch)

		stepOutput := map[string]any{}
		for _, step := range plan.Spec.Steps {
			if ctx.Err() != nil {
				ch <- plugin.WorkflowEvent{Type: "failed", StepID: step.ID, Err: ctx.Err()}
				return
			}
			ch <- plugin.WorkflowEvent{Type: "step_started", StepID: step.ID}

			req := WorkflowStepExecutionRequest{
				PluginID: plan.PluginID,
				Process:  model.WorkflowProcessSequential,
				Step:     step,
				Input:    sequentialMergeInputs(plan.Input, stepOutput),
			}
			result, err := e.stepRouter.Execute(ctx, req)
			if err != nil {
				ch <- plugin.WorkflowEvent{Type: "step_failed", StepID: step.ID, Err: err}
				ch <- plugin.WorkflowEvent{Type: "failed", Err: fmt.Errorf("step %s: %w", step.ID, err)}
				return
			}
			if result != nil {
				stepOutput = result.Output
			} else {
				stepOutput = map[string]any{}
			}
			ch <- plugin.WorkflowEvent{Type: "step_completed", StepID: step.ID, Payload: stepOutput}
		}
		ch <- plugin.WorkflowEvent{Type: "completed", Payload: stepOutput}
	}()
	return ch, nil
}

// sequentialMergeInputs returns a fresh map containing base then override.
// Used by the executors in this package; lives here (not as a free function
// shared with the legacy router) because the router builds its own input map
// from a different shape.
func sequentialMergeInputs(base, override map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(override))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}
