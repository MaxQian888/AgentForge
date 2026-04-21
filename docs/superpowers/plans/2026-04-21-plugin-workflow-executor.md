# Plugin WorkflowExecutor Framework — Hierarchical & Event-Driven Modes

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `WorkflowExecutor` interface that routes by the already-existing `spec.workflow.process` field, extract `SequentialExecutor` from inline logic, and implement `HierarchicalExecutor` (manager → workers → aggregate) and `EventDrivenExecutor` (persistent EventBus subscriber).

**Architecture:** `WorkflowExecutionService` gains a `map[WorkflowProcessMode]WorkflowExecutor` registry. Sequential logic is extracted into `SequentialExecutor` (no behavior change). `HierarchicalExecutor` lives in `internal/service/` and uses existing `TaskDispatchService.Spawn()`. `EventDrivenExecutor` is a background goroutine that subscribes to EventBus as a Mod; it is NOT routed through the existing Trigger Engine (which handles one-time dispatch only).

**Tech Stack:** Go 1.23+, existing `src-go/internal/service/workflow_execution_service.go`, `workflow_step_router.go`, `src-go/internal/eventbus/`, `src-go/internal/model/plugin.go`.

---

## Critical Context (read before coding)

- `WorkflowProcessMode` and its constants (`sequential`, `hierarchical`, `event-driven`, `wave`) **already exist** in `src-go/internal/model/plugin.go` — do not redefine them.
- `WorkflowPluginSpec.Process` already maps `process:` from YAML — do not add a `process_mode` field.
- The existing Trigger Engine (`src-go/internal/trigger/`) handles one-time runs; EventDrivenExecutor is separate and subscribes directly to EventBus.
- `WorkflowStepRouterExecutor` at `src-go/internal/service/workflow_step_router.go` handles step dispatch — the executor interface wraps the *orchestration loop*, not the per-step dispatch.

---

## File Map

| Action | Path | Responsibility |
|--------|------|---------------|
| Modify | `src-go/internal/model/plugin.go` | Add `ManagerRole`, `WorkerRoles`, `MaxParallelWorkers`, `WorkerFailurePolicy`, `Aggregation` to `WorkflowPluginSpec`; add `Filter`, `Role`, `Action`, `MaxConcurrent` to `PluginWorkflowTrigger` |
| Create | `src-go/internal/plugin/workflow_executor.go` | `WorkflowExecutor` interface definition |
| Create | `src-go/internal/service/sequential_executor.go` | `SequentialExecutor` (extracted from `WorkflowExecutionService`) |
| Modify | `src-go/internal/service/workflow_execution_service.go` | Add executor registry, route by `Process` mode |
| Create | `src-go/internal/service/sequential_executor_test.go` | Tests for sequential mode (same behaviour as before) |
| Create | `src-go/internal/service/hierarchical_executor.go` | `HierarchicalExecutor` |
| Create | `src-go/internal/service/hierarchical_executor_test.go` | Tests for hierarchical dispatch |
| Create | `src-go/internal/service/event_driven_executor.go` | `EventDrivenExecutor` (background goroutine) |
| Create | `src-go/internal/service/event_driven_executor_test.go` | Tests for event-driven lifecycle |
| Create | `plugins/workflows/hierarchical-demo/manifest.yaml` | Sample hierarchical workflow manifest |
| Create | `plugins/workflows/event-driven-demo/manifest.yaml` | Sample event-driven workflow manifest |

---

## Task 1: Add hierarchical and event-driven fields to the model

**Files:**
- Modify: `src-go/internal/model/plugin.go`

- [ ] **Step 1: Write the failing test**

Create `src-go/internal/model/plugin_workflow_modes_test.go`:

```go
package model_test

import (
	"testing"
	"gopkg.in/yaml.v3"
)

func TestWorkflowPluginSpec_HierarchicalFields(t *testing.T) {
	raw := `
process: hierarchical
managerRole: project-assistant
workerRoles: [coding-agent, test-engineer]
maxParallelWorkers: 2
workerFailurePolicy: best_effort
aggregation: manager_summarize
`
	var spec WorkflowPluginSpec
	if err := yaml.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if spec.Process != WorkflowProcessHierarchical {
		t.Errorf("Process = %q, want hierarchical", spec.Process)
	}
	if spec.ManagerRole != "project-assistant" {
		t.Errorf("ManagerRole = %q", spec.ManagerRole)
	}
	if len(spec.WorkerRoles) != 2 {
		t.Errorf("WorkerRoles len = %d, want 2", len(spec.WorkerRoles))
	}
	if spec.MaxParallelWorkers != 2 {
		t.Errorf("MaxParallelWorkers = %d, want 2", spec.MaxParallelWorkers)
	}
	if spec.WorkerFailurePolicy != "best_effort" {
		t.Errorf("WorkerFailurePolicy = %q", spec.WorkerFailurePolicy)
	}
}

func TestPluginWorkflowTrigger_EventDrivenFields(t *testing.T) {
	raw := `
event: integration.im.message_received
filter:
  channel: general
  contains_mention: true
role: project-assistant
action: reply
maxConcurrent: 2
`
	var trigger PluginWorkflowTrigger
	if err := yaml.Unmarshal([]byte(raw), &trigger); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if trigger.Role != "project-assistant" {
		t.Errorf("Role = %q", trigger.Role)
	}
	if trigger.MaxConcurrent != 2 {
		t.Errorf("MaxConcurrent = %d, want 2", trigger.MaxConcurrent)
	}
	if trigger.Filter["channel"] != "general" {
		t.Errorf("Filter[channel] = %v", trigger.Filter["channel"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./internal/model/... -run "TestWorkflowPluginSpec_HierarchicalFields|TestPluginWorkflowTrigger_EventDriven" -v
```

Expected: compile error — fields not defined.

- [ ] **Step 3: Add fields to `plugin.go`**

In `src-go/internal/model/plugin.go`, update `WorkflowPluginSpec`:

```go
type WorkflowPluginSpec struct {
	Process             WorkflowProcessMode      `yaml:"process" json:"process"`
	Roles               []WorkflowRoleBinding    `yaml:"roles,omitempty" json:"roles,omitempty"`
	Steps               []WorkflowStepDefinition `yaml:"steps,omitempty" json:"steps,omitempty"`
	Triggers            []PluginWorkflowTrigger  `yaml:"triggers,omitempty" json:"triggers,omitempty"`
	Limits              *WorkflowExecutionLimits `yaml:"limits,omitempty" json:"limits,omitempty"`
	// Hierarchical-mode fields
	ManagerRole         string   `yaml:"managerRole,omitempty" json:"managerRole,omitempty"`
	WorkerRoles         []string `yaml:"workerRoles,omitempty" json:"workerRoles,omitempty"`
	MaxParallelWorkers  int      `yaml:"maxParallelWorkers,omitempty" json:"maxParallelWorkers,omitempty"`
	WorkerFailurePolicy string   `yaml:"workerFailurePolicy,omitempty" json:"workerFailurePolicy,omitempty"`
	Aggregation         string   `yaml:"aggregation,omitempty" json:"aggregation,omitempty"`
}
```

Update `PluginWorkflowTrigger`:

```go
type PluginWorkflowTrigger struct {
	Event         string         `yaml:"event,omitempty" json:"event,omitempty"`
	Profile       string         `yaml:"profile,omitempty" json:"profile,omitempty"`
	RequiresTask  bool           `yaml:"requiresTask,omitempty" json:"requiresTask,omitempty"`
	// Event-driven-mode fields
	Filter        map[string]any `yaml:"filter,omitempty" json:"filter,omitempty"`
	Role          string         `yaml:"role,omitempty" json:"role,omitempty"`
	Action        string         `yaml:"action,omitempty" json:"action,omitempty"`
	MaxConcurrent int            `yaml:"maxConcurrent,omitempty" json:"maxConcurrent,omitempty"`
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd src-go && go test ./internal/model/... -run "TestWorkflowPluginSpec_HierarchicalFields|TestPluginWorkflowTrigger_EventDriven" -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/model/plugin.go src-go/internal/model/plugin_workflow_modes_test.go
git commit -m "feat(model): add hierarchical and event-driven fields to WorkflowPluginSpec"
```

---

## Task 2: Create `WorkflowExecutor` interface

**Files:**
- Create: `src-go/internal/plugin/workflow_executor.go`

- [ ] **Step 1: Create the interface file**

Create `src-go/internal/plugin/workflow_executor.go`:

```go
package plugin

import (
	"context"

	"agentforge/internal/model"
)

// WorkflowPlan is the resolved workflow definition passed to an executor.
type WorkflowPlan struct {
	PluginID string
	Spec     *model.WorkflowPluginSpec
	Input    map[string]any
}

// WorkflowEvent is emitted by executors during execution.
type WorkflowEvent struct {
	Type    string         // "step_started" | "step_completed" | "step_failed" | "completed" | "failed"
	StepID  string
	Payload map[string]any
	Err     error
}

// WorkflowExecutor orchestrates a workflow according to its process mode.
// Each mode (sequential, hierarchical, event-driven) has its own implementation.
type WorkflowExecutor interface {
	// Mode returns the WorkflowProcessMode this executor handles.
	Mode() model.WorkflowProcessMode

	// Execute starts the workflow and streams events. The channel is closed when done.
	Execute(ctx context.Context, plan WorkflowPlan) (<-chan WorkflowEvent, error)

	// Cancel requests cancellation of a running workflow instance.
	Cancel(ctx context.Context, instanceID string) error
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd src-go && go build ./internal/plugin/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add src-go/internal/plugin/workflow_executor.go
git commit -m "feat(plugin): add WorkflowExecutor interface"
```

---

## Task 3: Extract `SequentialExecutor` and wire routing into `WorkflowExecutionService`

**Files:**
- Create: `src-go/internal/service/sequential_executor.go`
- Create: `src-go/internal/service/sequential_executor_test.go`
- Modify: `src-go/internal/service/workflow_execution_service.go`

- [ ] **Step 1: Write the test**

Create `src-go/internal/service/sequential_executor_test.go`:

```go
package service_test

import (
	"context"
	"testing"

	"agentforge/internal/model"
	"agentforge/internal/plugin"
)

func TestSequentialExecutor_Mode(t *testing.T) {
	exec := &SequentialExecutor{}
	if exec.Mode() != model.WorkflowProcessSequential {
		t.Errorf("Mode() = %q, want sequential", exec.Mode())
	}
}

func TestSequentialExecutor_EmptySteps(t *testing.T) {
	exec := &SequentialExecutor{stepRouter: &stubStepRouter{}}
	plan := plugin.WorkflowPlan{
		PluginID: "test-workflow",
		Spec: &model.WorkflowPluginSpec{
			Process: model.WorkflowProcessSequential,
			Steps:   []model.WorkflowStepDefinition{},
		},
		Input: map[string]any{},
	}
	ch, err := exec.Execute(context.Background(), plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var events []plugin.WorkflowEvent
	for e := range ch {
		events = append(events, e)
	}
	// Empty steps → one "completed" event
	if len(events) != 1 || events[0].Type != "completed" {
		t.Errorf("expected [completed], got %v", events)
	}
}

// stubStepRouter satisfies the WorkflowStepExecutor interface used by SequentialExecutor.
type stubStepRouter struct{}

func (s *stubStepRouter) Execute(ctx context.Context, req WorkflowStepExecutionRequest) (*WorkflowStepExecutionResult, error) {
	return &WorkflowStepExecutionResult{Output: map[string]any{"ok": true}}, nil
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./internal/service/... -run TestSequentialExecutor -v
```

Expected: compile error — `SequentialExecutor` not defined.

- [ ] **Step 3: Create `sequential_executor.go`**

Create `src-go/internal/service/sequential_executor.go`:

```go
package service

import (
	"context"
	"fmt"

	"agentforge/internal/model"
	"agentforge/internal/plugin"
)

// SequentialExecutor runs workflow steps one-by-one in declaration order.
// It wraps the existing WorkflowStepExecutor (WorkflowStepRouterExecutor).
type SequentialExecutor struct {
	stepRouter WorkflowStepExecutor
}

func NewSequentialExecutor(router WorkflowStepExecutor) *SequentialExecutor {
	return &SequentialExecutor{stepRouter: router}
}

func (e *SequentialExecutor) Mode() model.WorkflowProcessMode {
	return model.WorkflowProcessSequential
}

func (e *SequentialExecutor) Cancel(_ context.Context, _ string) error {
	// Sequential execution is synchronous; cancellation is via ctx.
	return nil
}

func (e *SequentialExecutor) Execute(ctx context.Context, plan plugin.WorkflowPlan) (<-chan plugin.WorkflowEvent, error) {
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
				Step:     step,
				Input:    mergeInputs(plan.Input, stepOutput),
			}
			result, err := e.stepRouter.Execute(ctx, req)
			if err != nil {
				ch <- plugin.WorkflowEvent{Type: "step_failed", StepID: step.ID, Err: err}
				ch <- plugin.WorkflowEvent{Type: "failed", Err: fmt.Errorf("step %s: %w", step.ID, err)}
				return
			}
			stepOutput = result.Output
			ch <- plugin.WorkflowEvent{Type: "step_completed", StepID: step.ID, Payload: result.Output}
		}
		ch <- plugin.WorkflowEvent{Type: "completed", Payload: stepOutput}
	}()
	return ch, nil
}

func mergeInputs(base, override map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(override))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd src-go && go test ./internal/service/... -run TestSequentialExecutor -v
```

Expected: PASS.

- [ ] **Step 5: Wire executor registry into `WorkflowExecutionService`**

In `src-go/internal/service/workflow_execution_service.go`, add an `executors` map and routing. Locate the `WorkflowExecutionService` struct and add:

```go
type WorkflowExecutionService struct {
	// existing fields unchanged ...
	executors map[model.WorkflowProcessMode]plugin.WorkflowExecutor
}
```

In the constructor (`NewWorkflowExecutionService` or equivalent), register the sequential executor:

```go
svc.executors = map[model.WorkflowProcessMode]plugin.WorkflowExecutor{
	model.WorkflowProcessSequential: NewSequentialExecutor(svc.executor),
}
```

Add a helper that routes to the right executor (used by the existing run loop):

```go
func (s *WorkflowExecutionService) resolveExecutor(mode model.WorkflowProcessMode) (plugin.WorkflowExecutor, error) {
	if exec, ok := s.executors[mode]; ok {
		return exec, nil
	}
	return nil, fmt.Errorf("unsupported workflow process mode: %q", mode)
}
```

- [ ] **Step 6: Run the full service test suite to confirm no regressions**

```bash
cd src-go && go test ./internal/service/... -v
```

Expected: all tests PASS (no regressions in sequential workflows).

- [ ] **Step 7: Commit**

```bash
git add src-go/internal/service/sequential_executor.go src-go/internal/service/sequential_executor_test.go src-go/internal/service/workflow_execution_service.go
git commit -m "refactor(service): extract SequentialExecutor + add executor registry to WorkflowExecutionService"
```

---

## Task 4: Implement `HierarchicalExecutor`

**Files:**
- Create: `src-go/internal/service/hierarchical_executor.go`
- Create: `src-go/internal/service/hierarchical_executor_test.go`
- Create: `plugins/workflows/hierarchical-demo/manifest.yaml`

- [ ] **Step 1: Write the failing test**

Create `src-go/internal/service/hierarchical_executor_test.go`:

```go
package service_test

import (
	"context"
	"testing"
	"time"

	"agentforge/internal/model"
	"agentforge/internal/plugin"
)

func TestHierarchicalExecutor_Mode(t *testing.T) {
	exec := &HierarchicalExecutor{}
	if exec.Mode() != model.WorkflowProcessHierarchical {
		t.Errorf("Mode() = %q, want hierarchical", exec.Mode())
	}
}

func TestHierarchicalExecutor_RequiresManagerRole(t *testing.T) {
	exec := &HierarchicalExecutor{stepRouter: &stubStepRouter{}}
	plan := plugin.WorkflowPlan{
		PluginID: "test",
		Spec: &model.WorkflowPluginSpec{
			Process:     model.WorkflowProcessHierarchical,
			ManagerRole: "", // missing
			WorkerRoles: []string{"coding-agent"},
		},
		Input: map[string]any{},
	}
	_, err := exec.Execute(context.Background(), plan)
	if err == nil {
		t.Error("expected error when ManagerRole is empty")
	}
}

func TestHierarchicalExecutor_DispatchesWorkersAndCompletes(t *testing.T) {
	callLog := make([]string, 0)
	router := &recordingStepRouter{log: &callLog}
	exec := &HierarchicalExecutor{stepRouter: router}

	plan := plugin.WorkflowPlan{
		PluginID: "test",
		Spec: &model.WorkflowPluginSpec{
			Process:             model.WorkflowProcessHierarchical,
			ManagerRole:         "project-assistant",
			WorkerRoles:         []string{"coding-agent", "test-engineer"},
			MaxParallelWorkers:  2,
			WorkerFailurePolicy: "best_effort",
		},
		Input: map[string]any{"task": "build the feature"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := exec.Execute(ctx, plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var events []plugin.WorkflowEvent
	for e := range ch {
		events = append(events, e)
	}

	// Must end with "completed"
	last := events[len(events)-1]
	if last.Type != "completed" {
		t.Errorf("last event type = %q, want completed; events: %v", last.Type, events)
	}
	// Manager must be called (decompose + aggregate = 2 calls)
	managerCalls := 0
	for _, r := range callLog {
		if r == "project-assistant" {
			managerCalls++
		}
	}
	if managerCalls < 1 {
		t.Errorf("expected manager role to be called, log: %v", callLog)
	}
}

// recordingStepRouter records which roles were executed.
type recordingStepRouter struct {
	log *[]string
}

func (r *recordingStepRouter) Execute(ctx context.Context, req WorkflowStepExecutionRequest) (*WorkflowStepExecutionResult, error) {
	*r.log = append(*r.log, req.Step.Role)
	return &WorkflowStepExecutionResult{Output: map[string]any{"result": "done", "role": req.Step.Role}}, nil
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./internal/service/... -run TestHierarchicalExecutor -v
```

Expected: compile error.

- [ ] **Step 3: Implement `hierarchical_executor.go`**

Create `src-go/internal/service/hierarchical_executor.go`:

```go
package service

import (
	"context"
	"fmt"
	"sync"

	"agentforge/internal/model"
	"agentforge/internal/plugin"
)

// HierarchicalExecutor implements WorkflowExecutor for process: hierarchical.
// Flow: manager decomposes input → workers run in parallel → manager aggregates results.
type HierarchicalExecutor struct {
	stepRouter WorkflowStepExecutor
}

func NewHierarchicalExecutor(router WorkflowStepExecutor) *HierarchicalExecutor {
	return &HierarchicalExecutor{stepRouter: router}
}

func (e *HierarchicalExecutor) Mode() model.WorkflowProcessMode {
	return model.WorkflowProcessHierarchical
}

func (e *HierarchicalExecutor) Cancel(_ context.Context, _ string) error {
	return nil
}

func (e *HierarchicalExecutor) Execute(ctx context.Context, plan plugin.WorkflowPlan) (<-chan plugin.WorkflowEvent, error) {
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

		// Phase 1: Manager decomposes the task.
		ch <- plugin.WorkflowEvent{Type: "step_started", StepID: "manager:decompose"}
		managerDecomposeReq := WorkflowStepExecutionRequest{
			PluginID: plan.PluginID,
			Step: model.WorkflowStepDefinition{
				ID:     "manager:decompose",
				Role:   spec.ManagerRole,
				Action: model.WorkflowActionAgent,
				Config: map[string]any{"phase": "decompose", "worker_roles": spec.WorkerRoles},
			},
			Input: plan.Input,
		}
		decomposeResult, err := e.stepRouter.Execute(ctx, managerDecomposeReq)
		if err != nil {
			ch <- plugin.WorkflowEvent{Type: "step_failed", StepID: "manager:decompose", Err: err}
			ch <- plugin.WorkflowEvent{Type: "failed", Err: fmt.Errorf("manager decompose: %w", err)}
			return
		}
		ch <- plugin.WorkflowEvent{Type: "step_completed", StepID: "manager:decompose", Payload: decomposeResult.Output}

		// Phase 2: Dispatch workers in parallel (capped by MaxParallelWorkers).
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

				workerStepID := "worker:" + workerRole
				ch <- plugin.WorkflowEvent{Type: "step_started", StepID: workerStepID}

				req := WorkflowStepExecutionRequest{
					PluginID: plan.PluginID,
					Step: model.WorkflowStepDefinition{
						ID:     workerStepID,
						Role:   workerRole,
						Action: model.WorkflowActionAgent,
					},
					Input: mergeInputs(plan.Input, decomposeResult.Output),
				}
				result, err := e.stepRouter.Execute(ctx, req)
				if err != nil {
					ch <- plugin.WorkflowEvent{Type: "step_failed", StepID: workerStepID, Err: err}
					results <- workerResult{role: workerRole, err: err}
					return
				}
				ch <- plugin.WorkflowEvent{Type: "step_completed", StepID: workerStepID, Payload: result.Output}
				results <- workerResult{role: workerRole, output: result.Output}
			}(role)
		}

		wg.Wait()
		close(results)

		// Collect worker results.
		workerOutputs := map[string]any{}
		var firstErr error
		for r := range results {
			if r.err != nil {
				if firstErr == nil {
					firstErr = r.err
				}
				if spec.WorkerFailurePolicy == "fail_fast" {
					ch <- plugin.WorkflowEvent{Type: "failed", Err: fmt.Errorf("worker %s: %w", r.role, r.err)}
					return
				}
				continue
			}
			workerOutputs[r.role] = r.output
		}

		// Phase 3: Manager aggregates.
		ch <- plugin.WorkflowEvent{Type: "step_started", StepID: "manager:aggregate"}
		aggregateReq := WorkflowStepExecutionRequest{
			PluginID: plan.PluginID,
			Step: model.WorkflowStepDefinition{
				ID:     "manager:aggregate",
				Role:   spec.ManagerRole,
				Action: model.WorkflowActionAgent,
				Config: map[string]any{"phase": "aggregate"},
			},
			Input: mergeInputs(plan.Input, map[string]any{"worker_results": workerOutputs}),
		}
		aggregateResult, err := e.stepRouter.Execute(ctx, aggregateReq)
		if err != nil {
			ch <- plugin.WorkflowEvent{Type: "step_failed", StepID: "manager:aggregate", Err: err}
			ch <- plugin.WorkflowEvent{Type: "failed", Err: fmt.Errorf("manager aggregate: %w", err)}
			return
		}
		ch <- plugin.WorkflowEvent{Type: "step_completed", StepID: "manager:aggregate", Payload: aggregateResult.Output}
		ch <- plugin.WorkflowEvent{Type: "completed", Payload: aggregateResult.Output}
	}()
	return ch, nil
}
```

- [ ] **Step 4: Register in `WorkflowExecutionService`**

In `src-go/internal/service/workflow_execution_service.go`, add to the executor registry (in the constructor):

```go
svc.executors[model.WorkflowProcessHierarchical] = NewHierarchicalExecutor(svc.executor)
```

- [ ] **Step 5: Create sample manifest**

Create `plugins/workflows/hierarchical-demo/manifest.yaml`:

```yaml
apiVersion: agentforge/v1
kind: WorkflowPlugin
metadata:
  id: hierarchical-demo
  name: Hierarchical Demo
  version: 0.1.0
  description: Demo hierarchical workflow — manager assigns tasks to workers
  tags: [demo, workflow, hierarchical]
spec:
  runtime: wasm
  module: ./dist/hierarchical-demo.wasm
  abiVersion: v1
  workflow:
    process: hierarchical
    managerRole: project-assistant
    workerRoles:
      - coding-agent
      - test-engineer
    maxParallelWorkers: 2
    workerFailurePolicy: best_effort
    aggregation: manager_summarize
source:
  type: builtin
  path: ./plugins/workflows/hierarchical-demo/manifest.yaml
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd src-go && go test ./internal/service/... -run TestHierarchicalExecutor -v
```

Expected: PASS (3 tests).

- [ ] **Step 7: Commit**

```bash
git add src-go/internal/service/hierarchical_executor.go src-go/internal/service/hierarchical_executor_test.go src-go/internal/service/workflow_execution_service.go plugins/workflows/hierarchical-demo/
git commit -m "feat(service): implement HierarchicalExecutor workflow mode"
```

---

## Task 5: Implement `EventDrivenExecutor`

**Files:**
- Create: `src-go/internal/service/event_driven_executor.go`
- Create: `src-go/internal/service/event_driven_executor_test.go`
- Create: `plugins/workflows/event-driven-demo/manifest.yaml`

- [ ] **Step 1: Write the failing test**

Create `src-go/internal/service/event_driven_executor_test.go`:

```go
package service_test

import (
	"context"
	"testing"
	"time"

	"agentforge/internal/eventbus"
	"agentforge/internal/model"
	"agentforge/internal/plugin"
)

func TestEventDrivenExecutor_Mode(t *testing.T) {
	exec := &EventDrivenExecutor{}
	if exec.Mode() != model.WorkflowProcessEventDriven {
		t.Errorf("Mode() = %q, want event-driven", exec.Mode())
	}
}

func TestEventDrivenExecutor_MatchesTriggerAndDispatchesAgent(t *testing.T) {
	router := &recordingStepRouter{log: &[]string{}}
	bus := eventbus.NewBus()

	exec := NewEventDrivenExecutor(router, bus)

	plan := plugin.WorkflowPlan{
		PluginID: "test-event-driven",
		Spec: &model.WorkflowPluginSpec{
			Process: model.WorkflowProcessEventDriven,
			Triggers: []model.PluginWorkflowTrigger{
				{
					Event:         "integration.im.message_received",
					Filter:        map[string]any{"platform": "slack"},
					Role:          "project-assistant",
					Action:        "reply",
					MaxConcurrent: 2,
				},
			},
		},
		Input: map[string]any{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := exec.Execute(ctx, plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Publish a matching event to the bus.
	go func() {
		time.Sleep(10 * time.Millisecond)
		_ = bus.Publish(ctx, &eventbus.Event{
			Type:    "integration.im.message_received",
			Payload: mustMarshal(map[string]any{"platform": "slack", "text": "hello"}),
		})
	}()

	// Expect a step_started event from the triggered run.
	select {
	case ev := <-ch:
		if ev.Type != "step_started" {
			t.Errorf("got event type %q, want step_started", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for triggered event")
	}
}

func TestEventDrivenExecutor_DoesNotMatchWrongEvent(t *testing.T) {
	router := &recordingStepRouter{log: &[]string{}}
	bus := eventbus.NewBus()
	exec := NewEventDrivenExecutor(router, bus)

	plan := plugin.WorkflowPlan{
		PluginID: "test-event-driven",
		Spec: &model.WorkflowPluginSpec{
			Process: model.WorkflowProcessEventDriven,
			Triggers: []model.PluginWorkflowTrigger{
				{
					Event:  "integration.im.message_received",
					Filter: map[string]any{"platform": "slack"},
					Role:   "project-assistant",
					Action: "reply",
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	ch, _ := exec.Execute(ctx, plan)

	// Publish a non-matching event (discord, not slack).
	_ = bus.Publish(ctx, &eventbus.Event{
		Type:    "integration.im.message_received",
		Payload: mustMarshal(map[string]any{"platform": "discord"}),
	})

	select {
	case ev, ok := <-ch:
		if ok && ev.Type == "step_started" {
			t.Error("should not have dispatched for non-matching filter")
		}
	case <-ctx.Done():
		// Expected: timeout means nothing was dispatched.
	}
}

func mustMarshal(v map[string]any) []byte {
	b, _ := json.Marshal(v)
	return b
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./internal/service/... -run TestEventDrivenExecutor -v
```

Expected: compile error.

- [ ] **Step 3: Implement `event_driven_executor.go`**

Create `src-go/internal/service/event_driven_executor.go`:

```go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"agentforge/internal/eventbus"
	"agentforge/internal/model"
	"agentforge/internal/plugin"
)

// EventDrivenExecutor is a persistent EventBus subscriber.
// It is NOT routed through the Trigger Engine (which handles one-time dispatch).
// Lifecycle: Execute() subscribes; ctx cancellation unsubscribes.
type EventDrivenExecutor struct {
	stepRouter WorkflowStepExecutor
	bus        *eventbus.Bus
}

func NewEventDrivenExecutor(router WorkflowStepExecutor, bus *eventbus.Bus) *EventDrivenExecutor {
	return &EventDrivenExecutor{stepRouter: router, bus: bus}
}

func (e *EventDrivenExecutor) Mode() model.WorkflowProcessMode {
	return model.WorkflowProcessEventDriven
}

func (e *EventDrivenExecutor) Cancel(_ context.Context, _ string) error {
	return nil // cancellation via ctx in Execute
}

func (e *EventDrivenExecutor) Execute(ctx context.Context, plan plugin.WorkflowPlan) (<-chan plugin.WorkflowEvent, error) {
	if len(plan.Spec.Triggers) == 0 {
		return nil, fmt.Errorf("event-driven workflow %q requires at least one trigger", plan.PluginID)
	}

	outCh := make(chan plugin.WorkflowEvent, 64)

	// semaphores per trigger index to enforce MaxConcurrent
	sems := make([]chan struct{}, len(plan.Spec.Triggers))
	for i, trig := range plan.Spec.Triggers {
		cap := trig.MaxConcurrent
		if cap <= 0 {
			cap = 1
		}
		sems[i] = make(chan struct{}, cap)
	}

	go func() {
		defer close(outCh)

		eventCh := e.bus.Subscribe(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-eventCh:
				if !ok {
					return
				}
				for i, trig := range plan.Spec.Triggers {
					if ev.Type != trig.Event {
						continue
					}
					if !matchesFilter(ev, trig.Filter) {
						continue
					}
					// Dispatch under semaphore (non-blocking if at capacity).
					select {
					case sems[i] <- struct{}{}:
					default:
						continue // at capacity, skip this event
					}
					go func(trigger model.PluginWorkflowTrigger, sem chan struct{}, busEvent *eventbus.Event) {
						defer func() { <-sem }()

						stepID := fmt.Sprintf("event:%s:%s", trigger.Event, trigger.Role)
						outCh <- plugin.WorkflowEvent{Type: "step_started", StepID: stepID}

						var payload map[string]any
						_ = json.Unmarshal(busEvent.Payload, &payload)

						req := WorkflowStepExecutionRequest{
							PluginID: plan.PluginID,
							Step: model.WorkflowStepDefinition{
								ID:     stepID,
								Role:   trigger.Role,
								Action: model.WorkflowActionType(trigger.Action),
							},
							Input: mergeInputs(plan.Input, payload),
						}
						result, err := e.stepRouter.Execute(ctx, req)
						if err != nil {
							outCh <- plugin.WorkflowEvent{Type: "step_failed", StepID: stepID, Err: err}
							return
						}
						outCh <- plugin.WorkflowEvent{Type: "step_completed", StepID: stepID, Payload: result.Output}
					}(trig, sems[i], ev)
				}
			}
		}
	}()

	return outCh, nil
}

// matchesFilter returns true if all filter key-value pairs match the event payload.
func matchesFilter(ev *eventbus.Event, filter map[string]any) bool {
	if len(filter) == 0 {
		return true
	}
	var payload map[string]any
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		return false
	}
	for k, want := range filter {
		if got, ok := payload[k]; !ok || fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
			return false
		}
	}
	return true
}
```

**Note:** `e.bus.Subscribe(ctx)` assumes the EventBus exposes a subscription channel. If the current EventBus doesn't have a `Subscribe` method, add it to `src-go/internal/eventbus/bus.go`:

```go
// Subscribe returns a channel that receives all events published while ctx is live.
func (b *Bus) Subscribe(ctx context.Context) <-chan *Event {
	ch := make(chan *Event, 64)
	b.mu.Lock()
	b.subscribers = append(b.subscribers, ch)
	b.mu.Unlock()
	go func() {
		<-ctx.Done()
		b.mu.Lock()
		for i, s := range b.subscribers {
			if s == ch {
				b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
				break
			}
		}
		b.mu.Unlock()
		close(ch)
	}()
	return ch
}
```

Add `subscribers []chan *Event` to the `Bus` struct and fan out in `Publish`:

```go
// In Bus.Publish, after existing pipeline processing:
b.mu.Lock()
subs := b.subscribers
b.mu.Unlock()
for _, sub := range subs {
	select {
	case sub <- e:
	default: // drop if subscriber is slow
	}
}
```

- [ ] **Step 4: Register in `WorkflowExecutionService`**

In the executor registry constructor:

```go
svc.executors[model.WorkflowProcessEventDriven] = NewEventDrivenExecutor(svc.executor, svc.eventBus)
```

(`svc.eventBus` must be injected — add it to `WorkflowExecutionService` if not already present.)

- [ ] **Step 5: Create sample manifest**

Create `plugins/workflows/event-driven-demo/manifest.yaml`:

```yaml
apiVersion: agentforge/v1
kind: WorkflowPlugin
metadata:
  id: event-driven-demo
  name: Event-Driven Demo
  version: 0.1.0
  description: Demo event-driven workflow — responds to IM messages and PR events
  tags: [demo, workflow, event-driven]
spec:
  runtime: wasm
  module: ./dist/event-driven-demo.wasm
  abiVersion: v1
  workflow:
    process: event-driven
    triggers:
      - event: integration.im.message_received
        filter:
          platform: slack
          contains_mention: true
        role: project-assistant
        action: reply
        maxConcurrent: 2
      - event: vcs.pull_request.opened
        filter:
          base_branch: main
        role: code-reviewer
        action: review
        maxConcurrent: 1
source:
  type: builtin
  path: ./plugins/workflows/event-driven-demo/manifest.yaml
```

- [ ] **Step 6: Run all service tests**

```bash
cd src-go && go test ./internal/service/... -v
```

Expected: all PASS (no regressions).

- [ ] **Step 7: Commit**

```bash
git add src-go/internal/service/event_driven_executor.go src-go/internal/service/event_driven_executor_test.go src-go/internal/eventbus/ src-go/internal/service/workflow_execution_service.go plugins/workflows/event-driven-demo/
git commit -m "feat(service): implement EventDrivenExecutor with persistent EventBus subscription"
```

---

## Final Check

- [ ] **Run all affected tests**

```bash
cd src-go && go test ./internal/model/... ./internal/plugin/... ./internal/service/... ./internal/eventbus/... -v
```

Expected: all PASS.
