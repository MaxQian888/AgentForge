## Why

AgentForge persists workflow runs in two authoritative tables: `workflow_executions` for the DAG engine (`src-go/internal/repository/workflow_repo.go`) and `workflow_plugin_run` for legacy workflow plugins (`src-go/internal/repository/workflow_plugin_run_repo.go`). The existing workflow workspace UI (`app/(dashboard)/workflow/page.tsx:1-694`) reads from the DAG execution list only — so operators looking at "what is running?" in a project see a truthful-but-partial answer. Once triggers fan out to both engines (via `bridge-trigger-dispatch-unification`) and workflows cross-invoke via sub-workflow (`bridge-sub-workflow-invocation`), the UI's DAG-only view becomes actively misleading: a triggered plugin run or a sub-workflow plugin child would not appear in the workspace at all. The two-engine complementary architecture only works if operators can see the whole fleet of running workflows through one lens.

## What Changes

- Introduce a unified workflow-run read API that merges DAG executions and legacy workflow plugin runs into a single paginated, filter-capable listing. The API MUST return each row with a discriminator (`engine: "dag" | "plugin"`), the run identifier, the workflow (or plugin) reference, status, start time, optional completion time, and engine-specific metadata needed for display (parent linkage, acting employee, triggered-by).
- Add a unified detail fetcher that, given `(engine, runId)`, returns the engine-native DTO (already exposed by each engine's existing read endpoint) plus cross-engine enrichments (parent↔child linkage, acting employee, trigger outcome).
- Update the workflow workspace UI to consume the unified listing as the default view. Provide an engine filter chip so operators can narrow to one engine when they need to.
- Update the execution-detail view to accept `(engine, runId)` and render both engine types through a shared header (status chip, started-at, acting employee, trigger source, parent linkage) with engine-specific body sections delegated to existing per-engine components.
- Emit a unified `workflow.run.*` WebSocket channel so live status updates reach the unified view regardless of engine. DAG and plugin run lifecycle events continue to be emitted on their existing channels; the unified channel is an additional fan-out layer.

## Capabilities

### New Capabilities
- `workflow-run-unified-view`: Defines the cross-engine workflow-run list/detail API contract, the response shape unifying DAG executions and legacy plugin runs under one schema with an engine discriminator, the filter surface (engine, status, acting employee, triggered-by, time window), and the matching live-update channel semantics.

### Modified Capabilities
- None. Per-engine specs (`workflow-engine`, `workflow-plugin-runtime`, `workflow-trigger-dispatch`) keep their existing contracts — this change only layers a cross-engine read/presentation surface on top of them.

## Impact

- **Affected backend seams**: new `src-go/internal/handler/workflow_run_view_handler.go` mounting `/api/v1/projects/:pid/workflow-runs` list and `/workflow-runs/:engine/:id` detail; new `src-go/internal/service/workflow_run_view_service.go` composing DAG + plugin repos; `src-go/internal/ws/hub.go` (or equivalent WS seam) extended with a `workflow.run.*` channel; no schema migration.
- **Affected consumer seams**: `lib/stores/workflow-store.ts` or a new `lib/stores/workflow-run-store.ts`; `app/(dashboard)/workflow/page.tsx` (list tab defaulted to unified view); `components/workflow/workflow-execution-view.tsx` (accept engine discriminator).
- **Data model impact**: None. No new columns or tables.
- **API impact**: New additive endpoints. Existing per-engine endpoints stay untouched and remain authoritative for deep drill-downs.
- **Verification impact**: Service-layer tests for the unified composition (DAG-only, plugin-only, mixed, paginated, filtered); handler tests for the new endpoints; frontend tests for the unified list rendering and the engine-aware detail view; an end-to-end smoke that triggers one DAG run and one plugin run in the same project and asserts both appear in the unified view with correct discriminators.

## Dependency

This change consumes fields introduced by `bridge-trigger-dispatch-unification` (target kind, trigger outcome with engine identity) and `bridge-employee-attribution-legacy` (run-level `acting_employee_id`) and `bridge-sub-workflow-invocation` (parent↔child linkage). It MAY land before those complete, but its unified schema then exposes only the fields available at the time. To ensure the unified view surfaces every enrichment, schedule this change after the three bridges above.
