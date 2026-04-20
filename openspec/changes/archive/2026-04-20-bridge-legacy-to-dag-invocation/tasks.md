## 1. Schema

- [x] 1.1 Add migration `src-go/migrations/0NN_workflow_run_parent_link_parent_kind.up.sql` + `.down.sql` adding `parent_kind TEXT NOT NULL DEFAULT 'dag_execution'` to `workflow_run_parent_link` (only if the column was not already introduced in the `bridge-sub-workflow-invocation` migration; if it was, delete this task and update the helper to interpret both kinds)
- [x] 1.2 Extend the parent-link repo to filter and query by `parent_kind`

## 2. Router input extension

- [x] 2.1 Extend `workflow_step_router.go` `executeWorkflow` to read `targetKind` from trigger payload or step config
- [x] 2.2 When `targetKind` is absent or equals `plugin`, preserve current `pluginId`/`plugin_id` resolution and child-spawn path (`WorkflowChildSpawner.Start`) exactly as today
- [x] 2.3 When `targetKind='dag'`, parse `targetWorkflowId` and dispatch through the target-engine registry's DAG adapter (introduced in `bridge-trigger-dispatch-unification`)
- [x] 2.4 Reject unknown `targetKind` values with a structured error
- [x] 2.5 Extend the output envelope with `child_engine` (`"dag" | "plugin"`) and `child_workflow_id` (when DAG); preserve `child_run_id`, `child_plugin`, `status`
- [x] 2.6 Extend `workflow_step_router_test.go` to cover: plugin-default, DAG-explicit, unknown kind, cross-project reject, recursion reject

## 3. Parking and resume on the plugin side

- [x] 3.1 Add `awaiting_sub_workflow` to the plugin run's step status enum in `model.WorkflowPluginStep` (or equivalent)
- [x] 3.2 Extend the plugin run persistence repo to read/write the new status without losing existing transitions
- [x] 3.3 On DAG child start, insert a `workflow_run_parent_link` row with `parent_kind='plugin_run'`, `child_engine_kind='dag'`
- [x] 3.4 In the plugin engine's terminal-state hook, when a DAG run reaches a terminal state, look up parent-link rows with `parent_kind='plugin_run'` referencing that child; resume the plugin step by transitioning out of `awaiting_sub_workflow` to the outcome (`completed` / `failed` / `cancelled`)
- [x] 3.5 On plugin run cancellation while a `workflow` step is parked with a DAG child, cancel the DAG child through its engine adapter
- [x] 3.6 Add tests covering: DAG child in progress parks, completion resumes, failure propagates, cancellation cascades

## 4. Cycle guard cross-engine coverage

- [x] 4.1 Update the recursion guard helper introduced by `bridge-sub-workflow-invocation` to walk across engine kinds — following `workflow_run_parent_link.parent_kind` to either DAG execution records or plugin run records at each hop
- [x] 4.2 Walk comparisons use `(engine_kind, workflow_id)` so a plugin in a chain does not collide with a DAG workflow of the same UUID
- [x] 4.3 Unit-test: DAG → plugin → DAG cycle rejection; plugin → DAG → plugin cycle rejection; benign cross-engine chains at depth ≤ 7 pass

## 5. Manifest validation

- [x] 5.1 Extend plugin manifest validator in `workflow-plugin-runtime` to accept the `targetKind` discriminator on `workflow` steps
- [x] 5.2 Validate mutually exclusive shapes (`pluginId` vs `targetKind='dag' + targetWorkflowId`); reject mixed shape
- [x] 5.3 Extend manifest schema documentation and fixtures

## 6. Integration verification

- [x] 6.1 Add Go integration test: seed a plugin manifest whose `workflow` step targets a DAG workflow in the same project; start the plugin run manually; drive the DAG child to completion; assert parent step transitions through `awaiting_sub_workflow` → `completed` and subsequent plugin steps run
- [x] 6.2 Add Go integration test: same setup, but cancel the plugin run while the DAG child is in-flight; assert DAG child is cancelled and parent step ends with `cancelled`
- [x] 6.3 Run `go test ./internal/service/... ./internal/repository/... ./internal/workflow/nodetypes/...` scoped to touched packages; record any pre-existing unrelated failures
- [x] 6.4 Document in `openspec/specs/workflow-plugin-runtime/spec.md` (delta applied at archive time) the new `targetKind` shape for manifest authors

## 7. Unified run view exposure

- [x] 7.1 Confirm the unified run view introduced by `bridge-unified-run-view` renders legacy plugin parents invoking DAG children correctly — parent link appears under the plugin run's detail envelope and child run appears under the DAG listing with `triggeredBy.kind='sub_workflow'`
- [x] 7.2 If not already covered, add targeted test cases
