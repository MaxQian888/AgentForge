## 1. Backend composition service

- [x] 1.1 Add `src-go/internal/service/workflow_run_view_service.go` exposing `ListRuns(ctx, projectID, filter, cursor, limit)` and `GetRun(ctx, projectID, engine, runID)`
- [x] 1.2 Compose results from `repository.WorkflowExecutionRepo` (DAG) and `repository.WorkflowPluginRunRepo` (legacy). Query each with pre-merge filters at SQL level to avoid over-fetching.
- [x] 1.3 Define the canonical row struct `UnifiedRunRow` with fields: engine, runId, workflowRef{id,name}, status (normalized), startedAt, completedAt, actingEmployeeId, triggeredBy{kind,ref}, parentLink{execId,nodeId}
- [x] 1.4 Add status-normalization table mapping plugin-native values to canonical values; unmapped → `"unknown"` (tested)
- [x] 1.5 Implement compound cursor `(started_at, runId)` with strict ordering and correctness tests under concurrent-insert scenarios
- [x] 1.6 Unit-test the service for: DAG-only, plugin-only, mixed, paginated, each filter (engine, status, actingEmployeeId, triggeredByKind, triggerId, startedAfter/Before), empty result

## 2. Backend handler + routes

- [x] 2.1 Add `src-go/internal/handler/workflow_run_view_handler.go` mounting `GET /api/v1/projects/:pid/workflow-runs` (list) and `GET /api/v1/projects/:pid/workflow-runs/:engine/:id` (detail)
- [x] 2.2 List handler validates query params, returns `{rows: [...], nextCursor: "..."}` envelope
- [x] 2.3 Detail handler validates `engine` enum (`dag | plugin`), calls the corresponding engine-native read, wraps in the shared envelope alongside the engine-native body
- [x] 2.4 Register routes with the existing project scope middleware; apply RBAC consistent with the per-engine endpoints
- [x] 2.5 Handler tests covering list shape, cursor roundtrip, detail for both engines, unknown engine rejection, not-found

## 3. WebSocket fan-out

- [x] 3.1 Extend the WS hub (`src-go/internal/ws/hub.go` or equivalent) with a subscriber that observes DAG and plugin lifecycle events and re-emits canonical `workflow.run.status_changed` / `workflow.run.terminal` events carrying the normalized row shape
- [x] 3.2 Ensure engine-native channels continue to be emitted unchanged
- [x] 3.3 Add test covering: DAG transition → unified event; plugin transition → unified event; both original + unified events are emitted

## 4. Frontend store

- [x] 4.1 Add `lib/stores/workflow-run-store.ts` or extend `workflow-store.ts` with `fetchUnifiedRuns(filter)`, `fetchRunDetail(engine, runId)`, WS subscription to `workflow.run.*`
- [x] 4.2 Types mirror the canonical row shape plus per-engine body shapes (imported from existing types)
- [x] 4.3 Unit-test the store (filter round-trip, pagination, detail branching)

## 5. Workspace UI

- [x] 5.1 Update `app/(dashboard)/workflow/page.tsx` runs tab to use the unified list as the default view
- [x] 5.2 Add an engine-filter chip (All | DAG | Plugin) reflecting current filter state
- [x] 5.3 Render each row with engine badge, workflow name, status, started-at relative time, acting-employee chip (if present), triggered-by label
- [x] 5.4 Update detail routing to `(engine, runId)`; the detail page composes a shared header component with the existing engine-native body component (`WorkflowExecutionView` for DAG, plugin-detail component for plugin)
- [x] 5.5 Extend `components/workflow/workflow-execution-view.tsx` to accept an `engine` prop and render only DAG content when `engine='dag'`, delegating to a plugin-body component otherwise
- [x] 5.6 Component tests covering list rendering, engine filter chip, row click → detail navigation, detail rendering for both engines

## 6. Badges and summary

- [x] 6.1 Extend the list endpoint response with a `summary` field: `{running, paused, failed}` counts cross-engine
- [x] 6.2 Render these counts in the workspace tab header as badges

## 7. Verification

- [x] 7.1 Run `go test ./internal/service/... ./internal/handler/... ./internal/ws/...` scoped to touched packages; record any pre-existing unrelated failures
- [x] 7.2 Run `pnpm test` scoped to the workflow workspace and new run store
- [ ] 7.3 Smoke-verify in `pnpm dev:all`: start one DAG execution (trigger or manual) and one plugin run (trigger); open the workspace runs tab; assert both appear with correct badges; click each row; assert both detail pages render
- [ ] 7.4 Smoke-verify live updates: drive each run to completion while the list is open; assert list rows update without page refresh
