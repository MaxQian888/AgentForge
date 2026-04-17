## Context

The unified `KnowledgeAsset` model (from `unify-wiki-and-ingested-documents`) gives every wiki page one storage shape: a BlockNote JSON `content_json`. BlockNote already supports custom blocks (AgentForge uses it today for formula/mermaid/entity-card). The gap is not the editor — it is the projection pipeline that turns "a reference to an agent run" into "the current state of that agent run, rendered as a block, kept fresh while the reader has the doc open."

Four entity kinds drive the initial scope because they are the artifacts operators most want to embed in PRDs, runbooks, postmortems, and sprint reviews:

| Kind | Entity | Typical embed | Authoritative surface |
|---|---|---|---|
| `agent_run` | `model.AgentRun` | Single run's status, duration, cost, outcome, last N log lines | `/agents/:id` |
| `cost_summary` | derived from `model.CostAccounting` | Cost over a window, grouped by runtime/provider/member | `/cost?range=...` |
| `review` | `model.Review` | Review state, findings count, reviewer, task link | `/reviews/:id` |
| `task_group` | derived from `model.Task` filter | Compact table of tasks matching a filter or a saved view | `/tasks?filter=...` or saved view URL |

Three constraints shape the design:

1. **Authoritative source, not cache**: the block must never look stale when the operator has a doc open. Projection runs on-demand at open time and on WebSocket pushes.
2. **Block references survive save/version/restore**: the BlockNote JSON stored in `content_json` carries the reference, not the projection. This means versioning a doc doesn't freeze the live view — intentional, because restoring a document to a week-old state should still show today's agent-run status.
3. **RBAC per block, not per doc**: a user who can read a doc may not be able to read a referenced cost summary (cost is typically gated). The block must degrade to a "no access" state, not leak data.

## Goals / Non-Goals

**Goals**

- Four working live-block kinds covering the most embedded artifacts: agent_run, cost_summary, review, task_group.
- Projection contract stable enough that future kinds (plan, sprint burndown, dispatch queue, memory) are additive.
- Fresh-on-open and live-on-push refresh with no client polling.
- Freeze-as-static, open-source, orphan handling, per-block RBAC.
- Zero new schema — lives entirely inside the existing `content_json` field plus in-memory subscription state.

**Non-Goals**

- Inline editing inside live blocks (the authoritative surface owns editing).
- Write-back from live blocks to entities (covered by `review-doc-writeback` separately for review→doc).
- Custom user-authored projector kinds (plugin-provided projectors can come later).
- Embedding live blocks inside ingested-file or template assets (scope: `kind=wiki_page` only).
- Rendering historical views ("cost as of last week" — out of scope; operators who need this can freeze a snapshot).

## Decisions

### D1: Block reference schema lives in BlockNote `props`

**Chosen**: Each live block is a BlockNote custom block with `type: "live_artifact"` and `props: { live_kind, target_ref, view_opts, last_rendered_at, cache_key? }`. The block has no `content` array (BlockNote custom blocks can opt out of rich-text content).

`target_ref` is a tagged union:
- `{ kind: "agent_run", id }`
- `{ kind: "cost_summary", filter: { range_start, range_end, runtime?, provider?, member_id? } }`
- `{ kind: "review", id }`
- `{ kind: "task_group", filter: { saved_view_id? | { status?, assignee?, tag?, sprint_id?, milestone_id? } } }`

**Alternatives considered**:
- **Sidecar table** keyed by `(asset_id, block_id)`: rejected — duplicates the BlockNote source-of-truth, complicates version snapshots, breaks when blocks are copy-pasted across docs.
- **Proxy entity** (`LiveEmbed` row per block): rejected for the same reasons plus extra migration.

**Rationale**: BlockNote props already serialize cleanly as JSON inside `content_json`. Version snapshots and restore pick up the reference for free. Copy-paste across docs replicates the reference; two docs referencing the same entity render identically — that's the desired behavior.

### D2: Projection is server-side; block payload is opaque BlockNote JSON

**Chosen**: On open and on subscription push, the frontend calls `POST /api/v1/projects/:pid/knowledge/assets/:id/live-artifacts/project` with `{block_refs: [{block_id, live_kind, target_ref, view_opts}, ...]}`. The server runs each `LiveArtifactProjector` and returns `{block_id: {status, projection, projected_at, ttl_hint?}}` where `projection` is a BlockNote JSON fragment the client renders read-only.

**Alternatives considered**:
- **Client-side projection**: the client fetches raw entity data and renders. Rejected — every projector would need a matching frontend component; RBAC filtering would run client-side where it's not authoritative; adding a new kind would require a frontend release.
- **Server-side rendering to HTML**: rejected — breaks BlockNote's in-place interaction and styling model; harder to animate refreshes.

**Rationale**: server-authored BlockNote fragments keep the rendering path uniform with the rest of the editor, keep RBAC server-side, and let a new `live_kind` ship without a frontend change beyond a kind-label and icon.

### D3: Projector interface

```go
type LiveArtifactProjector interface {
    Kind() LiveArtifactKind
    RequiredRole() projectrbac.Role  // e.g. viewer, or viewer-with-cost-access
    Project(ctx context.Context, principal PrincipalContext, projectID uuid.UUID, targetRef json.RawMessage, viewOpts json.RawMessage) (ProjectionResult, error)
    // Subscribe returns the set of WS event topics that, when emitted, should trigger a re-projection of this block.
    Subscribe(targetRef json.RawMessage) []EventTopic
}

type ProjectionResult struct {
    Status       ProjectionStatus  // ok | forbidden | not_found | degraded
    Projection   json.RawMessage   // BlockNote JSON fragment; nil when Status != ok
    ProjectedAt  time.Time
    TTLHint      *time.Duration    // optional; signals the client how long the projection is reasonably fresh
    Diagnostics  string            // set when Status == degraded
}
```

Registered in `internal/knowledge/liveartifact/registry.go`. Each projector lives in its own file alongside the registry.

### D4: Subscription fan-out via the existing WS hub

**Chosen**: The WebSocket hub already broadcasts entity-level events (`agent_run.*`, `cost.*`, `review.*`, `task.*`). The live-artifact subscription service registers, per open asset, a set of filters based on each block's `target_ref`. When an event matches a filter, the hub emits `knowledge.asset.live_artifacts_changed` with `{asset_id, block_ids_affected[]}` to the specific clients viewing that asset. The client then calls the projection endpoint with only the affected blocks.

**Alternatives considered**:
- **Separate per-live-block WS channels**: rejected — explodes channel count for docs with many blocks; fan-out and filter is cheaper server-side.
- **Client polls projection endpoint on a timer**: rejected — either too frequent (wasteful) or too slow (stale); we already have WS wiring.

### D5: Orphan handling policy

- `target_ref` cannot be resolved (entity deleted, filter matches nothing for a singleton kind) → projector returns `Status=not_found`; block renders a "no longer available" state with "freeze last known snapshot" (if one exists in the block's `cache_key`) or "remove block" actions.
- RBAC denies → `Status=forbidden`; block renders "you don't have access to this live artifact"; does NOT leak title or id.
- Projector errors (timeout, dependency failure) → `Status=degraded` with `Diagnostics`; block renders last successful projection if still within TTL, else "temporarily unavailable."

### D6: Freeze-as-static

The "freeze" action POSTs `POST /api/v1/projects/:pid/knowledge/assets/:id/live-artifacts/:block_id/freeze`. Server-side:
1. Runs the projector once to get the current BlockNote fragment.
2. Replaces the `live_artifact` block in `content_json` with the fragment, prefaced by a `callout` block: "Frozen from {live_kind} {target summary} on {ISO date}."
3. Creates a new `AssetVersion` with name "Frozen live artifact {block_id}" so the freeze is reversible via version restore.

The reverse operation ("unfreeze"/"re-liven") is not part of this change — operators who want a live view again insert a new live block.

### D7: Task-group projection vs entity-card

The existing `entity-card` block (from `block-document-editor`) renders a single task inline. `task_group` is different: it renders a filtered list as a compact table and updates the set as tasks change state.

- `entity-card` stays the right tool for "I want a specific task referenced here" — it is optimized for inline prose flow.
- `task_group` is the right tool for "I want the tasks matching this filter to live here" — it is a block-level table.

Both coexist. The slash-menu labels them distinctly: "Embed task card" vs "Embed task group (live)."

### D8: Cost summary scope is project-scoped

`cost_summary` projection runs only over the current project's cost accounting. Cross-project aggregates are out of scope. The filter supports a time range plus optional grouping dimensions (`runtime`, `provider`, `member_id`). The projection renders a small summary: total, top N rows, sparkline-like delta.

### D9: View-opts are projector-specific but versioned

Each `live_kind` defines its `view_opts` shape (e.g. `agent_run` has `{show_log_lines: 10|25|50, show_steps: bool}`). View-opts are versioned per projector via a `view_opts_schema_version` integer in the block props. If the projector bumps the schema, unknown old versions are normalized to current defaults (additive, non-breaking) with a diagnostic if unresolvable.

### D10: Initial render latency budget

Projection endpoint MUST return within 500ms p95 for batches of up to 10 blocks. Individual projectors target 50ms p95:
- `agent_run`, `review`: single-row lookup + RBAC → trivial.
- `cost_summary`: aggregate query over cost-accounting rows in a window → backed by existing indexes from `cost-query-api`.
- `task_group`: filter query against tasks → backed by existing indexes; page-limited to 50 rows.

## Risks / Trade-offs

- **Open-doc fan-out under load**: a doc with 30 live blocks opened by 50 viewers generates 30 × 50 = 1,500 projection slots. Mitigated by batching the projection endpoint (one request per asset-open) and server-side coalescing of subscription pushes (N block events within a 250ms window become one `live_artifacts_changed` payload per client).
- **RBAC latency cost**: each projector runs an RBAC check; for 10 blocks this is 10 checks. Mitigated by caching the principal's resolved project role on the `PrincipalContext` for the request lifetime.
- **Versioning mismatch on restore**: a restored version references a target that has since been deleted. Acceptable — block renders orphan state on restore.
- **Copy-paste leaks block id**: BlockNote assigns a unique block id; pasting a live block into another doc replicates the reference cleanly. This is desired.
- **BlockNote custom block ergonomics**: adding four new custom block types grows editor bundle size. Mitigated by lazy-loading live-block components on first use, matching the existing editor lazy-load pattern.
- **Freeze is irreversible without version restore**: acceptable; versioning covers it.
- **Subscription storms on high-churn entities**: a runaway agent_run that emits 100 events/sec would cause heavy coalescing. Mitigated by the 250ms coalescing window and a max-refresh-rate (1 Hz) per block.

## Migration Plan

No schema migration. Pure-additive behavior on top of the unified asset model. Deployment order:

1. `unify-wiki-and-ingested-documents` must be archived first (see that change's migration).
2. Add `internal/knowledge/liveartifact/` package with registry + four projectors + tests.
3. Wire the projection endpoint into the knowledge-asset handler.
4. Extend the WS hub subscription manager with the per-asset filter set.
5. Add frontend block components, slash-menu entries, action affordances.
6. Turn it on — existing docs are unaffected; new blocks require explicit insertion.

Rollback: revert the server and frontend changes; existing docs contain no live blocks unless operators inserted them, and the BlockNote renderer ignores unknown block types by default with a visible "unsupported block" placeholder.

## Open Questions

- Should the projection endpoint be `POST` (body carries block refs) or `GET` (refs in query)? Leaning `POST` because refs can be large and contain structured filters. Decision not load-bearing; adjust during implementation.
- Should `task_group` support inline checkbox interactions (mark tasks done from within the embedded table)? Out of scope per D1 "read-only"; if demand is strong, a follow-up change can open a narrow edit path gated by a separate capability.
- Should cost_summary respect the project's currency preferences if multiple are configured? Assume a single currency per project matches the existing cost spec; revisit if that changes.
- Should live-block orphan states be searchable (to help operators find "broken" embeds)? Probably yes via a maintenance endpoint, but defer until real orphan-count data exists.
