package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/plugin"
)

// HierarchicalExecutor implements WorkflowExecutor for process: hierarchical.
// Phases:
//  1. Manager decomposes the input into worker tasks.
//  2. Workers run in parallel (capped by MaxParallelWorkers).
//  3. Manager aggregates worker outputs into the final result.
//
// WorkerFailurePolicy: "fail_fast" stops the run on the first worker error;
// "best_effort" (default) collects errors and proceeds to aggregation with
// the surviving outputs.
type HierarchicalExecutor struct {
	stepRouter WorkflowStepExecutor
}

func NewHierarchicalExecutor(router WorkflowStepExecutor) *HierarchicalExecutor {
	return &HierarchicalExecutor{stepRouter: router}
}

func (e *HierarchicalExecutor) Mode() model.WorkflowProcessMode {
	return model.WorkflowProcessHierarchical
}

func (e *HierarchicalExecutor) Cancel(_ context.Context, _ string) error { return nil }

func (e *HierarchicalExecutor) Execute(ctx context.Context, plan plugin.WorkflowPlan) (<-chan plugin.WorkflowEvent, error) {
	if plan.Spec == nil {
		return nil, fmt.Errorf("hierarchical workflow %q has no spec", plan.PluginID)
	}
	spec := plan.Spec
	if spec.ManagerRole == "" {
		return nil, fmt.Errorf("hierarchical workflow %q requires managerRole", plan.PluginID)
	}
	if len(spec.WorkerRoles) == 0 {
		return nil, fmt.Errorf("hierarchical workflow %q requires at least one workerRole", plan.PluginID)
	}

	ch := make(chan plugin.WorkflowEvent, 32)
	go func() {
		defer close(ch)

		// Phase 1: manager decompose.
		const decomposeID = "manager:decompose"
		ch <- plugin.WorkflowEvent{Type: "step_started", StepID: decomposeID}
		decomposeReq := WorkflowStepExecutionRequest{
			PluginID: plan.PluginID,
			Process:  model.WorkflowProcessHierarchical,
			Step: model.WorkflowStepDefinition{
				ID:     decomposeID,
				Role:   spec.ManagerRole,
				Action: model.WorkflowActionAgent,
				Config: map[string]any{
					"phase":        "decompose",
					"worker_roles": spec.WorkerRoles,
				},
			},
			Input: plan.Input,
		}
		decomposeResult, err := e.stepRouter.Execute(ctx, decomposeReq)
		if err != nil {
			ch <- plugin.WorkflowEvent{Type: "step_failed", StepID: decomposeID, Err: err}
			ch <- plugin.WorkflowEvent{Type: "failed", Err: fmt.Errorf("manager decompose: %w", err)}
			return
		}
		decomposeOutput := map[string]any{}
		if decomposeResult != nil {
			decomposeOutput = decomposeResult.Output
		}
		ch <- plugin.WorkflowEvent{Type: "step_completed", StepID: decomposeID, Payload: decomposeOutput}

		// Phase 2: workers in parallel.
		maxParallel := spec.MaxParallelWorkers
		if maxParallel <= 0 {
			maxParallel = len(spec.WorkerRoles)
		}
		sem := make(chan struct{}, maxParallel)
		type workerResult struct {
			role   string
			output map[string]any
			err    error
		}
		results := make(chan workerResult, len(spec.WorkerRoles))
		var wg sync.WaitGroup

		for _, role := range spec.WorkerRoles {
			wg.Add(1)
			go func(workerRole string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				stepID := "worker:" + workerRole
				ch <- plugin.WorkflowEvent{Type: "step_started", StepID: stepID}

				req := WorkflowStepExecutionRequest{
					PluginID: plan.PluginID,
					Process:  model.WorkflowProcessHierarchical,
					Step: model.WorkflowStepDefinition{
						ID:     stepID,
						Role:   workerRole,
						Action: model.WorkflowActionAgent,
					},
					Input: sequentialMergeInputs(plan.Input, decomposeOutput),
				}
				result, err := e.stepRouter.Execute(ctx, req)
				if err != nil {
					ch <- plugin.WorkflowEvent{Type: "step_failed", StepID: stepID, Err: err}
					results <- workerResult{role: workerRole, err: err}
					return
				}
				output := map[string]any{}
				if result != nil {
					output = result.Output
				}
				ch <- plugin.WorkflowEvent{Type: "step_completed", StepID: stepID, Payload: output}
				results <- workerResult{role: workerRole, output: output}
			}(role)
		}

		wg.Wait()
		close(results)

		workerOutputs := map[string]any{}
		var firstErr error
		policy := spec.WorkerFailurePolicy
		for r := range results {
			if r.err != nil {
				if firstErr == nil {
					firstErr = r.err
				}
				if policy == "fail_fast" {
					ch <- plugin.WorkflowEvent{Type: "failed", Err: fmt.Errorf("worker %s: %w", r.role, r.err)}
					return
				}
				continue
			}
			workerOutputs[r.role] = r.output
		}

		// Phase 3: manager aggregate.
		const aggregateID = "manager:aggregate"
		ch <- plugin.WorkflowEvent{Type: "step_started", StepID: aggregateID}
		aggregateReq := WorkflowStepExecutionRequest{
			PluginID: plan.PluginID,
			Process:  model.WorkflowProcessHierarchical,
			Step: model.WorkflowStepDefinition{
				ID:     aggregateID,
				Role:   spec.ManagerRole,
				Action: model.WorkflowActionAgent,
				Config: map[string]any{"phase": "aggregate"},
			},
			Input: sequentialMergeInputs(plan.Input, map[string]any{
				"worker_results": workerOutputs,
			}),
		}
		aggregateResult, err := e.stepRouter.Execute(ctx, aggregateReq)
		if err != nil {
			ch <- plugin.WorkflowEvent{Type: "step_failed", StepID: aggregateID, Err: err}
			ch <- plugin.WorkflowEvent{Type: "failed", Err: fmt.Errorf("manager aggregate: %w", err)}
			return
		}
		aggregateOutput := map[string]any{}
		if aggregateResult != nil {
			aggregateOutput = aggregateResult.Output
		}
		ch <- plugin.WorkflowEvent{Type: "step_completed", StepID: aggregateID, Payload: aggregateOutput}
		ch <- plugin.WorkflowEvent{Type: "completed", Payload: aggregateOutput}
	}()
	return ch, nil
}
