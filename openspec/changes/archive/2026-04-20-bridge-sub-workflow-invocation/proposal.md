## Why

The DAG workflow engine advertises a `sub_workflow` node type in `model.NodeTypeSubWorkflow` and registers `SubWorkflowHandler` in `src-go/internal/workflow/nodetypes/bootstrap.go:30`, but the handler is a fail-fast stub: `src-go/internal/workflow/nodetypes/sub_workflow.go:9` documents "fail-fast stub," and the applier logs `[WARN] EffectApplier: TODO: sub_workflow not wired` at `applier.go:358`. The effect type `EffectInvokeSubWorkflow` is defined (`effects.go:12`) and the payload shape is stubbed (`effects.go:56`), but no actual invocation happens. As a result, users cannot nest workflows at all from the DAG editor — neither DAG→DAG composition nor DAG→legacy-plugin composition works. Since the two-engine complementary story depends on DAG workflows being able to invoke curated, versioned legacy workflow plugins as black-box sub-steps, this stub blocks the entire "plugin as reusable primitive" idea.

## What Changes

- Replace the `sub_workflow` handler stub with a real implementation that resolves a target workflow (DAG workflow or legacy workflow plugin) and emits a live `EffectInvokeSubWorkflow` effect with the correct target kind and seed.
- Extend `InvokeSubWorkflowPayload` with `TargetKind ("dag" | "plugin")`, `InputMapping` (templated against parent run context), and a `WaitForCompletion` flag; omit of `TargetKind` defaults to `dag`.
- Implement the applier side of `EffectInvokeSubWorkflow` so it starts the child run through the same `TargetEngine` registry introduced by `bridge-trigger-dispatch-unification`, parks the parent node in `awaiting_sub_workflow` status, records a parent↔child linkage, and resumes the parent node when the child reaches a terminal state.
- Persist the parent↔child run linkage in a new `workflow_run_parent_link` table (or reuse `workflow_run_mapping_repo` if its semantics align) so operators can walk up/down the invocation tree during debugging.
- Guard recursion: a sub_workflow invocation MUST reject a child target that would produce a cycle (target workflow equals any ancestor in the current invocation chain), returning a structured non-success effect.
- Handle child terminal outcomes: completed → parent node completes with child's outputs in `dataStore`; failed → parent node fails with structured reason; cancelled → parent node cancels; paused-awaiting-approval → parent node remains parked.
- Update the DAG editor's `sub_workflow` config panel to let users pick target kind (DAG or plugin), pick target workflow from a filtered dropdown scoped to the project, and edit the input mapping with the same templating syntax used elsewhere.

## Capabilities

### New Capabilities
- `workflow-sub-invocation`: Defines how a DAG workflow node invokes another workflow (DAG or legacy plugin) as a child run, how input mapping flows parent → child, how the parent parks and resumes, how parent↔child linkage is persisted, and how recursion is guarded.

### Modified Capabilities
- None. The existing `workflow-engine` and `workflow-plugin-runtime` contracts are unchanged; this change only adds a new invocation pathway that consumes their existing start seams.

## Impact

- **Affected backend seams**: `src-go/internal/workflow/nodetypes/sub_workflow.go`, `src-go/internal/workflow/nodetypes/effects.go` (payload extension), `src-go/internal/workflow/nodetypes/applier.go` (wire `applyInvokeSubWorkflow`), `src-go/internal/service/dag_workflow_service.go` (parking/resuming support), `src-go/internal/repository/workflow_run_mapping_repo.go` (parent link), new migration if the schema change is needed, and the `TargetEngine` adapter registry (from `bridge-trigger-dispatch-unification`).
- **Affected consumer seams**: `components/workflow-editor/config-panel/node-configs/sub-workflow-config.tsx`, `lib/stores/workflow-store.ts` (loads target lists), `lib/stores/marketplace-store.ts` is NOT affected — plugin resolution goes through the plugin runtime, not the marketplace.
- **Data model impact**: optional new table / column for parent↔child linkage; additive column on parked node execution records to store child run id and child engine kind.
- **API impact**: DAG node config DTO for `sub_workflow` gains `targetKind`, `targetWorkflowId`, `inputMapping`, `waitForCompletion`. Execution read DTOs expose parent↔child linkage.
- **Verification impact**: applier unit tests must replace the stub "parks-fail-fast" assertion with real parking + resumption; an integration test starts a DAG workflow whose `sub_workflow` node targets a legacy plugin, exercises the plugin run to completion, and asserts the parent DAG node completes with the child's outputs visible in `dataStore`; recursion-cycle rejection is explicitly tested.

## Dependency

This change depends on `bridge-trigger-dispatch-unification` landing first; the `TargetEngine` registry introduced there is the same seam used here to start child runs through the correct engine adapter.
