## Context

Two engines, two persistence stores, two WebSocket event families, and two read endpoints exist today:

| Engine | Table | Read endpoint | WS channel |
|---|---|---|---|
| DAG | `workflow_executions` + `workflow_node_executions` | `GET /api/v1/projects/:pid/executions` and `GET /executions/:id` | `workflow.execution.*` |
| Plugin | `workflow_plugin_run` | existing plugin runtime read seams | `workflow.plugin.*` (if any) |

The workflow workspace UI at `app/(dashboard)/workflow/page.tsx` reads only the DAG endpoint. Any legacy plugin run — whether started from a task trigger (`task-triggered-workflow-automation`), a registered IM/schedule trigger (after `bridge-trigger-dispatch-unification`), or a DAG sub_workflow invocation (after `bridge-sub-workflow-invocation`) — is invisible in that workspace. Operators currently have to open Go logs or hit the plugin runtime's internal endpoints directly to see them.

The existing per-engine detail views have substantial, engine-specific UI surface that works well: DAG shows a node-graph replay, plugin shows a step-by-step list with retries. We should not rewrite either. What we need is a cross-engine **index and routing layer**, not a replacement for the engine-native detail UIs.

## Goals / Non-Goals

**Goals:**
- One paginated listing surface in the project workflow workspace that shows every in-flight and recent workflow run across both engines, with an engine discriminator on each row.
- A filter surface that operators actually need: engine, status, acting employee, triggered-by (trigger row id, manual, sub-workflow-parent), time window.
- A routing layer in the detail view so `(engine, runId)` lands on the correct per-engine detail component without the router forking into divergent URLs.
- A live-update channel that does not require the frontend to multiplex two engine-specific event feeds.

**Non-Goals:**
- Merging the two persistence stores. That would entangle engine semantics (DAG graph state vs plugin step state) in one table and is not the value we want here.
- Rewriting the DAG replay UI or the plugin step UI. Both remain authoritative detail views; we only route to them.
- Deep drill-down enrichment in the unified list. The list carries only what is necessary for identification and sorting; detail lives on the detail page.
- Admin-grade cross-project aggregation. Project scope is respected; this change adds no new cross-project read surface.

## Decisions

### Decision 1: Composition service, not a materialized cache

**Chosen**: A new `WorkflowRunViewService` composes results by querying both the DAG executions repo and the plugin runs repo, then merges, sorts, filters, and paginates in-memory. No new table, no new cache layer, no scheduled sync job.

**Alternatives considered**:
- *Materialized `workflow_runs_view` table updated on every run transition* — doubles the write path and introduces a new source of truth that can drift from the engines'.
- *Database view (SQL `VIEW`) unioning the two tables* — schema coupling is strong; future column changes in either table break the view at migration time.

**Rationale**: The two tables are small enough (single-project scope, typical tens to hundreds of rows per list query) that in-memory composition is fast and keeps the two engines independent. Indexed columns (status, started_at, acting_employee_id, project_id) already exist on both tables after the upstream bridges land.

### Decision 2: Canonical row schema with engine discriminator

**Chosen**: Every row in the unified list returns:

```
{
  engine: "dag" | "plugin",
  runId: uuid,
  workflowRef: { id: uuid, name: string },        // engine-specific: DAG workflow or plugin id
  status: "pending" | "running" | "paused" | "completed" | "failed" | "cancelled",
  startedAt: datetime,
  completedAt: datetime | null,
  actingEmployeeId: uuid | null,
  triggeredBy: { kind: "trigger" | "manual" | "sub_workflow", ref: string | null },
  parentLink: { parentExecutionId: uuid, parentNodeId: string } | null
}
```

Status values are normalized to the set above. DAG-native statuses (`pending/running/completed/failed/cancelled/paused`) already match; plugin statuses that differ are translated by the service layer.

**Alternatives considered**:
- *Return full engine-native DTOs in the list* — bloats the payload and forces the frontend to handle two shapes for what is essentially the same header.

**Rationale**: A single canonical row shape keeps the frontend simple and lets the backend absorb engine differences.

### Decision 3: Detail view stays engine-native, routed by `(engine, runId)`

**Chosen**: `/api/v1/projects/:pid/workflow-runs/:engine/:id` returns an engine-specific response body — the same body each engine's existing read endpoint would return — plus a shared envelope of cross-engine fields (status, actingEmployeeId, triggeredBy, parentLink). The frontend detail page consumes the envelope for the shared header and delegates the body to the engine-native component (`WorkflowExecutionView` for DAG, a plugin-specific equivalent for plugin).

**Rationale**: Avoids rewriting detail UI; keeps a single URL pattern so the app router doesn't need two branches.

### Decision 4: WebSocket fan-out, not channel change

**Chosen**: Keep the engine-specific channels (`workflow.execution.*`, `workflow.plugin.*`) as authoritative emitters. Add a fan-out subscriber in the WS hub that re-broadcasts a canonical `workflow.run.*` event for each underlying event. The unified frontend view subscribes only to `workflow.run.*`. Engine-specific detail views keep subscribing to their native channel.

**Rationale**: No disruption to existing subscribers; additive layer; lets us normalize the payload at fan-out time the same way the list endpoint normalizes rows.

### Decision 5: Filter surface is a normalized query, not engine-specific

**Chosen**: Accepted query parameters:
- `engine=dag|plugin` (omit = both)
- `status=running|paused|failed|...` (repeatable)
- `actingEmployeeId=<uuid>`
- `triggeredByKind=trigger|manual|sub_workflow`
- `triggerId=<uuid>` (requires triggeredByKind=trigger)
- `startedAfter=<iso>`, `startedBefore=<iso>`
- `limit=<int>`, `cursor=<opaque>`

Cursor pagination (not offset) so cross-engine merge remains stable under churn.

**Rationale**: Normalized filters match what operators actually ask the list for; cursor avoids the "duplicated rows on new arrivals" problem that offset pagination has when underlying data is merged from two sources.

## Risks / Trade-offs

- **Risk**: In-memory merge could be slow if either table grows large per project. → **Mitigation**: both tables are indexed on `(project_id, started_at DESC)`; the service applies filters at the SQL level before merging; `limit` is enforced per-engine at query time (e.g. fetch `2 * limit` from each, merge, truncate). Re-evaluate if profiles show this dominating latency.
- **Risk**: Status translation for plugin engine could drift if new plugin statuses are added. → **Mitigation**: central translation table in `WorkflowRunViewService`; covered by unit tests; any unmapped status returns `"unknown"` explicitly rather than silently falling into a wrong bucket.
- **Risk**: Cursor pagination + cross-engine merge is subtle around ties at `started_at`. → **Mitigation**: cursor is `(started_at, runId)` compound; run id as tiebreaker ensures strict ordering.
- **Trade-off**: Two WS channels stay; fan-out layer duplicates some bytes. Acceptable — the unified channel is consumed by one subscriber (the workspace view), not tens of subscribers.
- **Trade-off**: The new endpoints add complexity. We accept that because the alternative (every consumer learns two engines) is worse.

## Migration Plan

1. Add `service/workflow_run_view_service.go` with interfaces satisfied by existing repos.
2. Add `handler/workflow_run_view_handler.go` with list and detail routes.
3. Add WS fan-out for `workflow.run.*`.
4. Add `lib/stores/workflow-run-store.ts` (or extend `workflow-store.ts`).
5. Update `app/(dashboard)/workflow/page.tsx` list tab to default to the unified view with an engine filter chip.
6. Update the execution detail route to accept `(engine, runId)` and render the shared header + engine-native body.
7. Tests per "Verification impact."

Rollback: the unified endpoints and WS channel are additive. Frontend can flip back to per-engine reads by reverting the page component.

## Open Questions

- Should the list endpoint also return a project-level count summary (`{ running: X, paused: Y, failed: Z }`) so the workspace header can render badges without a second request? (Expected: yes, small addition; included in tasks.)
- Do we want an export button for the unified list (CSV)? (Not now; revisit if operators ask.)
