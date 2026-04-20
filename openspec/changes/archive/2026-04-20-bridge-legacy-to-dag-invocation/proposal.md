## Why

The legacy workflow step router supports a `workflow` action that starts a child workflow run (`src-go/internal/service/workflow_step_router.go:185-211`), but it resolves only through `WorkflowChildSpawner.Start(ctx, pluginID, req)` — a seam whose `pluginID string` argument hard-codes the child as another **legacy workflow plugin**. There is no path for a legacy plugin to invoke a **DAG** workflow as a child. That asymmetry breaks the complementary two-engine story: `bridge-sub-workflow-invocation` gives DAG the ability to call plugins, but plugins still cannot call DAG workflows — so curated, reusable DAG workflows authored in the visual editor cannot be embedded inside stable plugin pipelines. To make the two engines truly interoperable, the legacy `workflow` action must be able to target either engine.

## What Changes

- Extend the legacy `workflow` step action payload to accept a target discriminator: either `pluginId` (legacy, default, existing behavior) or `{targetKind: "dag", targetWorkflowId: <uuid>}` (new).
- Replace the single-engine `WorkflowChildSpawner` interface with an engine-aware dispatch that uses the same `TargetEngine` registry introduced by `bridge-trigger-dispatch-unification`, so legacy step → DAG child goes through the DAG adapter already in service.
- Extend the plugin runtime's "start child workflow" flow to park the parent plugin run when the child is a DAG workflow, register a parent↔child linkage (reusing the `workflow_run_parent_link` table from `bridge-sub-workflow-invocation`), and resume the parent step only when the DAG child reaches a terminal state.
- Translate DAG terminal outcomes into the shape the legacy step router's output envelope already emits (`child_run_id`, `child_plugin`, `status`) so existing consumers of the `workflow` action output stay compatible; add a new `child_engine` field to disambiguate.
- Guard recursion and same-project constraints identically to `bridge-sub-workflow-invocation` — a legacy plugin MUST NOT invoke a DAG ancestor already in its invocation chain, and MUST NOT cross project scope.
- Extend the workflow plugin manifest schema so plugin authors can declare a `workflow` step whose target is an engine-kind-plus-workflow-id reference, not just a plugin id. Keep the legacy `plugin_id`-only shape as a supported alias.

## Capabilities

### New Capabilities
- None. This change reuses `workflow-sub-invocation` (for invocation / parking / linkage / cycle-guard semantics) and `workflow-trigger-dispatch` (for the target-engine adapter registry) and does not introduce a new capability.

### Modified Capabilities
- `workflow-engine`: Extends the `workflow` step action contract so it can target either a legacy workflow plugin (default, existing behavior) or a DAG workflow. Action semantics for the other four actions (`agent`, `review`, `task`, `approval`) are unchanged.
- `workflow-plugin-runtime`: Extends plugin manifest step validation so a `workflow` step may reference either a plugin id (existing behavior) or a `{targetKind, targetWorkflowId}` pair. Existing manifests continue to parse unchanged.
- `workflow-sub-invocation`: Extends the "parent ↔ child linkage" requirement to cover the new parent kind (legacy workflow plugin run invoking a DAG child). Linkage semantics are the same; only the parent side is new.

## Impact

- **Affected backend seams**: `src-go/internal/service/workflow_step_router.go` (executeWorkflow), `src-go/internal/service/workflow_execution_service.go` or wherever `WorkflowChildSpawner` is currently satisfied, the plugin runtime's manifest validator, the `TargetEngine` registry (consumer only; no change to the registry itself), `workflow_run_parent_link_repo` (consumer only).
- **Affected consumer seams**: plugin manifest schema documentation; any plugin author tooling that validates `workflow` step config; the `workflow-engine` tests that exercise `workflow` actions; the unified run view automatically picks up the new parent kind via `triggeredBy.kind='sub_workflow'`.
- **Data model impact**: None. Reuses `workflow_run_parent_link` from `bridge-sub-workflow-invocation`.
- **API impact**: Plugin manifest and legacy step input payloads gain a new discriminated shape. Existing inputs continue to work.
- **Verification impact**: Step router unit tests cover plugin child, DAG child, recursion rejection across engines, and same-project guard; plugin manifest validator tests cover both shapes; an integration test boots both engines and exercises a legacy plugin whose `workflow` step invokes a DAG child, asserting correct parking, resumption, and output envelope.

## Dependency

This change depends on `bridge-trigger-dispatch-unification` (for the `TargetEngine` registry) and `bridge-sub-workflow-invocation` (for the parent↔child linkage table and resume mechanics) landing first.
