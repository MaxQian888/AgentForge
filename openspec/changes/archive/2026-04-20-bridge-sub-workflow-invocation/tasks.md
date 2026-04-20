## 1. Schema and models

- [x] 1.1 Add migration `src-go/migrations/0NN_workflow_run_parent_link.up.sql` + `.down.sql` creating `workflow_run_parent_link(id, parent_execution_id, parent_node_id, child_engine_kind, child_run_id, status, started_at, terminated_at)` with unique index on `(parent_execution_id, parent_node_id)`
- [x] 1.2 Extend `model.WorkflowNodeExecution` to include `awaiting_sub_workflow` status value
- [x] 1.3 Extend `workflow/nodetypes/effects.go` `InvokeSubWorkflowPayload` with `TargetKind`, `TargetWorkflowID`, `InputMapping json.RawMessage`, `WaitForCompletion bool`
- [x] 1.4 Add repository `workflow_run_parent_link_repo.go` with Create/GetByParent/GetByChild/UpdateStatus; unit-test it

## 2. Sub_workflow handler

- [x] 2.1 Replace `workflow/nodetypes/sub_workflow.go` stub with a real handler that validates config, emits a complete `InvokeSubWorkflowPayload`, and returns `Capabilities() == [EffectInvokeSubWorkflow]`
- [x] 2.2 Add unit tests for: successful emission, missing `TargetWorkflowID`, unknown `TargetKind`, unresolvable input mapping
- [x] 2.3 Remove or update `sub_workflow_test.go` "not implemented" assertion to match real behavior

## 3. Applier wiring

- [x] 3.1 Replace the `applyInvokeSubWorkflow` TODO stub in `workflow/nodetypes/applier.go` with a real implementation
- [x] 3.2 Within the real implementation: render input mapping against parent dataStore/context, call recursion guard helper, validate same-project, insert parent link row, call `TargetEngine.Start`, park node with `awaiting_sub_workflow`
- [x] 3.3 Extend applier tests to cover: successful park with link row, cycle rejection, cross-project rejection, unresolvable mapping

## 4. Recursion guard

- [x] 4.1 Add a `workflow.recursionGuard` helper that walks up to N=8 ancestors via `workflow_run_parent_link_repo`
- [x] 4.2 Expose `maxSubWorkflowDepth` as a package-level constant (initially 8); document in design/spec comments
- [x] 4.3 Unit-test the guard against: direct self-recursion, 3-cycle A→B→C→A, 7-deep non-cyclic chain passes, 9-deep chain rejects

## 5. Engine resume paths

- [x] 5.1 In `dag_workflow_service.go`, after a DAG execution reaches a terminal state, look up parent link rows referencing it; if present, call `resumeParent(parentExecID, parentNodeID, childOutcome)`
- [x] 5.2 Implement `resumeParent` to materialize child outputs into `parent.dataStore[parentNodeID].subWorkflow.outputs`, mark the parent node complete/failed, call `AdvanceExecution`
- [x] 5.3 Add a `plugin_runtime.OnTerminalState` seam on the plugin runtime service and wire it to call the same resume path for plugin-engine children
- [x] 5.4 Handle cancellation: a cancel on a DAG execution or plugin run with an active parent link MUST trigger parent resume with `failed` outcome
- [x] 5.5 Add tests covering resume on complete, fail, cancel, and extended park while child awaits approval

## 6. Target-engine adapter reuse

- [x] 6.1 Reuse `TargetEngine` registry from `bridge-trigger-dispatch-unification` — no new engine abstractions created here
- [x] 6.2 Add integration test: a DAG workflow whose `sub_workflow` node targets a legacy plugin; trigger the parent; assert plugin run executes and parent completes with child outputs in `dataStore`

## 7. Node-config save validation

- [x] 7.1 Extend `workflow_handler.go` workflow save path to validate each `sub_workflow` node's target: resolvable, same project, and (for DAG targets) not self-referential in a single-hop trivial way
- [x] 7.2 Return structured rejections with machine-readable reasons (cross-project, unknown target, trivial self-loop)

## 8. Editor UI

- [x] 8.1 Update `components/workflow-editor/config-panel/node-configs/sub-workflow-config.tsx` to expose: target kind picker (DAG | Plugin), target workflow selector (filtered by project + kind), input mapping editor with existing template syntax
- [x] 8.2 Show parent↔child linkage in execution detail view via a linkage strip (read from the execution read DTO's linkage field)
- [x] 8.3 Add or extend tests for the config panel covering both target kinds

## 9. Run read DTO exposure

- [x] 9.1 Extend DAG execution read DTO to include a `subInvocations` field listing parent-link rows originating at this execution
- [x] 9.2 Extend DAG execution read DTO to include a `invokedByParent` field when this execution is a child
- [x] 9.3 Extend plugin run read DTO symmetrically for `invokedByParent`
- [x] 9.4 Add handler tests covering both exposures

## 10. Verification

- [x] 10.1 Run `go test ./internal/workflow/nodetypes/... ./internal/service/... ./internal/repository/...` scoped to touched packages; record any pre-existing unrelated failures
- [x] 10.2 Run `pnpm test` scoped to `sub-workflow-config` and editor tests
- [x] 10.3 Smoke-verify manually in `pnpm dev:all`: build a DAG with a `sub_workflow` node targeting a legacy plugin; trigger it from IM; assert both run records and the parent-link row exist and the parent completes after the child _(skipped per operator decision: equivalent coverage provided by `dag_workflow_plugin_sub_invocation_test.go` which exercises the full parent→sub_workflow→plugin→terminal-bridge→resume loop)_
