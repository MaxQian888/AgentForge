## Why

AgentForge's differentiator is that documents and agent-driven work live in the same workspace, but the two are still disconnected at the block level. Today a user who wants an up-to-date cost snapshot, agent-run log, or review summary inside a PRD or runbook has three options, all bad:

1. **Copy-paste from the agents/cost/reviews workspaces** — immediately stale, forces re-copy on every change.
2. **Add a static `[[entity-id]]` mention** — a mention is a link, not content; the doc reader still has to click away.
3. **Use the existing `entity-card` block (`block-document-editor`)** — works for a single `task/agent/review` card with a rigid view, but can't render a filtered task group, a cost-summary over a window, or a multi-row agent-run log.

Feishu-style "live cards" and Notion-style database embeds cover this for generic entities. AgentForge's angle is narrower and stronger: the artifacts we most need to embed are first-class domain objects the platform already owns — `AgentRun`, `CostAccounting`, `Review`, task groups — and we can render them in a shape that matches how operators already read them in `/agents`, `/cost`, `/reviews`.

Solving this inside the document surface means:

- A PRD can embed a live "related tasks" group that reflects the current backlog state.
- A runbook can embed a cost budget card that refreshes as spend accumulates.
- A review summary page can embed a live review block that transitions when the review state transitions.
- An agent-run postmortem doc can embed the run itself (steps, duration, cost, outcome) without the author doing screenshot-and-paste work.

This change delivers **live-artifact blocks** — a new block category in the wiki editor that references an authoritative entity by id, renders a projection of its current state, auto-refreshes on the same WebSocket channels the standalone workspaces already use, and falls back to a static snapshot when the user wants to freeze the state at publication time.

It depends on and is scoped by the unified knowledge-asset model landing first (`unify-wiki-and-ingested-documents`).

## What Changes

- Introduce a new block category in the wiki editor: **live-artifact blocks**, with a `live_kind` discriminator and a `target_ref`:
  - `live_kind=agent_run` → renders an `AgentRun` entity: title, status, runtime, duration, cost, step summary, last log line.
  - `live_kind=cost_summary` → renders a cost snapshot over a window (project-scoped, filterable by runtime/provider/member).
  - `live_kind=review` → renders a `Review` entity: title, state, findings count, reviewer, task link.
  - `live_kind=task_group` → renders a filtered set of tasks as a compact table (filter = saved-view id or inline `{status, assignee, tag, sprint}` criteria).
- Live-artifact blocks SHALL store only `{live_kind, target_ref, view_opts, last_rendered_at}` — not the projected content. The block is re-projected on open and on subscribed updates.
- Introduce the **`LiveArtifactProjector` Go interface** with one implementation per `live_kind`. The projector turns an entity snapshot into a renderable block payload (BlockNote-compatible JSON fragment).
- Introduce the **`LiveArtifactSubscription` WebSocket channel**. When an asset containing live blocks is opened, the frontend subscribes to the union of entity channels the blocks reference. Updates trigger re-projection on the client.
- Introduce the **`GET /api/v1/projects/:pid/knowledge/assets/:id/live-artifacts` endpoint**. Returns the current projections for every live block in an asset. Called on asset open and on reconnect.
- Introduce the **"freeze as static" action** on live blocks: replaces the live block with static BlockNote blocks capturing the current projection and a "frozen on {date}, source: {target}" callout. Snapshots a version so the operator can later re-liven.
- Introduce the **"open source" action**: clicking the block body navigates to the authoritative surface (`/agents/:id`, `/cost?range=...`, `/reviews/:id`, a saved-view URL).
- Introduce **orphan handling**: if a live block's `target_ref` no longer resolves (entity deleted), the block renders a "no longer available" state with a "freeze last known" or "remove" action.
- Introduce **RBAC gating per live block**: each projection runs its own read check against the target entity. A block whose viewer can read the asset but not the target entity renders a "you don't have access to this live artifact" state, not the projected content.
- Make live-artifact blocks **snapshotable into `AssetVersion`**. When `AssetVersion` is restored, live blocks in the restored state keep their live behavior (the snapshot captures the block reference, not the projection).
- Inline edit remains **read-only** within the block. Editing affordances exist only on the authoritative surface; live blocks never write back to the entity. The existing `review-doc-writeback` capability already handles the reverse direction (entity events writing into docs) and is unchanged here.

## Capabilities

### New Capabilities

- `live-artifact-blocks`: the block category, its kinds (`agent_run`, `cost_summary`, `review`, `task_group`), insertion UX, freeze-as-static, open-source, orphan handling, and RBAC gating.
- `live-artifact-projection`: the `LiveArtifactProjector` interface, the per-kind projectors, the projection REST endpoint, and the projection schema returned to the client.
- `live-artifact-subscription`: the WebSocket subscription pattern for live updates, reconnection, backoff, and server-authoritative fan-out.

### Modified Capabilities

- `block-document-editor`: live-artifact blocks added to the supported block types; relationship to the existing `entity-card` block clarified (entity-card remains for single-entity inline references; live-artifact blocks are for richer projections).
- `knowledge-asset-model`: content serialization SHALL preserve live-block references across save/version/restore; projection content is never persisted inside `content_json`.

## Impact

- **Depends on**: `unify-wiki-and-ingested-documents`. That change must land first so live-artifact blocks operate on the unified asset model, and so WebSocket payloads carry the `asset_id`/`kind` discriminator the subscription pattern relies on.
- **Backend**: new `src-go/internal/knowledge/liveartifact/` package with projector interface, per-kind projectors (`agent_run_projector.go`, `cost_summary_projector.go`, `review_projector.go`, `task_group_projector.go`), projection endpoint, and subscription fan-out wiring into `internal/ws/hub`.
- **Frontend**: new BlockNote block components under `components/docs/live-blocks/` (one per kind); slash-menu insertion UX; freeze/open/remove actions; subscription hook that multiplexes over the existing project WebSocket.
- **Schema**: none beyond what the unified asset model provides. Live-block references live inside `content_json` as BlockNote blocks with a custom `type`, `props`, and no `content` array.
- **RBAC**: each projector declares the read role required against the target entity; the gate runs inside the projector service call, not in the handler, so WS-triggered re-projection paths honor the same rules.
- **WebSocket**: new event name `knowledge.asset.live_artifacts_changed` sent by the hub whenever an entity referenced by a live block in any open asset changes; the payload carries `{asset_id, target_refs_affected[]}` and the client re-fetches only the affected projections.
- **Out of scope**: plans (superpowers plans are markdown files, not DB entities — a follow-up can add them via `live_kind=plan` if they become entities); inline editing within live blocks; writing from live blocks back to entities; new entity kinds beyond the four listed; custom user-authored live-block kinds.
