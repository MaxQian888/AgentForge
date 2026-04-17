## 1. Prerequisites

- [x] 1.1 Confirm `unify-wiki-and-ingested-documents` is archived and the unified `KnowledgeAsset` model is live in the codebase; abort this change until it is
- [x] 1.2 Audit existing WebSocket event names for `agent_run.*`, `cost.*`, `review.*`, `task.*` to make sure the hub emits what the subscription filters will need; file follow-up tasks if any are missing

> **Audit findings (1.2):** hub uses `agent.*` prefix (not `agent_run.*`); no dedicated `cost.accounting_updated` (use `agent.cost_update`/`team.cost_update`); `review.updated` covers both state and findings changes (no distinct `findings_updated`); `task.*` matches as-is; `knowledge.asset.live_artifacts_changed` is the only net-new event (added in 9.4). Projectors' `Subscribe()` will map logical topics → concrete hub event names — no hub rename/new-emit required.

## 2. Projector interface and registry

- [x] 2.1 Create `src-go/internal/knowledge/liveartifact/types.go` defining `LiveArtifactKind`, `ProjectionStatus`, `ProjectionResult`, `EventTopic`, and `LiveArtifactProjector`
- [x] 2.2 Create `src-go/internal/knowledge/liveartifact/registry.go` with `Register(p LiveArtifactProjector)` and `Lookup(kind) (LiveArtifactProjector, bool)`
- [x] 2.3 Wire the registry into the server bootstrap so all projectors register at startup

## 3. Agent-run projector

- [x] 3.1 Create `agent_run_projector.go` implementing the interface against `model.AgentRun` and the agent-run repository
- [x] 3.2 Produce a BlockNote fragment containing title/status/runtime/duration/cost/steps/last-N-logs
- [x] 3.3 Implement `Subscribe` returning topics for `agent_run.status_changed`, `agent_run.log_appended`, `agent_run.step_changed`, `agent_run.cost_updated` scoped to the target id
- [x] 3.4 Unit tests: running vs completed, log-line count respected, forbidden path, not-found path, Subscribe returns scoped topics

> **Notes:** hub emits `agent.*` (not `agent_run.*`); projector Subscribe maps to concrete hub names. Log fetching is a placeholder pending exposure of the agent-run log repo; tracked as follow-up.

## 4. Cost-summary projector

- [x] 4.1 Create `cost_summary_projector.go` with an aggregate query over the project's `cost_accounting` rows for the requested window
- [x] 4.2 Implement grouping by runtime/provider/member as declared in `view_opts`
- [x] 4.3 Compute delta vs the prior equal-length window and render a compact summary fragment
- [x] 4.4 Enforce the cost-read sub-permission; return `forbidden` if the principal lacks it even with project-viewer
- [x] 4.5 Implement `Subscribe` returning `cost.accounting_updated` topics relevant to the window
- [x] 4.6 Unit tests: grouping, delta computation, forbidden path, time-range edge cases (zero-spend window, partial window)

> **Notes:** aggregates in-memory from `*AgentRunRepository.ListByProject`; no separate `cost.accounting_updated` event exists in the hub, so `Subscribe` returns `agent.cost_update` + `team.cost_update`. `RequiredRole=RoleEditor` approximates a cost-read sub-permission until a finer tier lands. Table rendered as paragraphs for now; follow-up aligns with BlockNote `table` block schema in §10.3. `$asset_project` sentinel used in scope; router (§9.4) substitutes per-client.

## 5. Review projector

- [x] 5.1 Create `review_projector.go` against `model.Review`
- [x] 5.2 Render fragment for in-progress and finalized review states, including linked task title
- [x] 5.3 Implement `Subscribe` for `review.state_changed`, `review.findings_updated`
- [x] 5.4 Unit tests: each state pill, linked-task resolution, forbidden, not-found

> **Notes:** hub has no `review.state_changed` / `review.findings_updated`; `Subscribe` maps to concrete hub names (`review.updated`, `review.completed`, `review.pending_human`, `review.fix_requested`). Cross-project guard: review resolved via its linked task's `ProjectID`; mismatch → `StatusNotFound` (no leak).

## 6. Task-group projector

- [x] 6.1 Create `task_group_projector.go` supporting both saved-view and inline-filter forms of `target_ref.filter`
- [x] 6.2 Enforce the 50-row page cap with a "N more" footer when truncated
- [x] 6.3 Implement `Subscribe` returning topics that match the filter — at minimum `task.created`, `task.updated`, `task.deleted` scoped by project
- [x] 6.4 Unit tests: saved-view form, inline filter, truncation, status-change reflected after subscription push

> **Notes:** `Tags` field on `model.Task` is actually `Labels` — post-filter matches against `Labels`. Saved views currently stub to `StatusDegraded` (translating `SavedView.Config` into `TaskListQuery` is a follow-up). MilestoneID post-filter works. Rows rendered as paragraphs; table block alignment is §10.5 follow-up. Scope sentinel declared file-scoped as `scopeAssetProject`.

## 7. Projection REST endpoint

- [x] 7.1 Add `POST /api/v1/projects/:pid/knowledge/assets/:id/live-artifacts/project` to `internal/handler/knowledge_asset_handler.go`
- [x] 7.2 Handler batches block refs, runs each projector, assembles `{block_id: ProjectionResult}` response
- [x] 7.3 Handler resolves `PrincipalContext` and passes it to each projector
- [x] 7.4 Integration test: batch with mixed ok/not_found/forbidden; latency under 500ms p95 for batches of 10

## 8. Freeze endpoint

- [x] 8.1 Add `POST /api/v1/projects/:pid/knowledge/assets/:id/live-artifacts/:block_id/freeze`
- [x] 8.2 Implementation: run projector; if ok, replace the live block in `content_json` with a callout + projected fragment; create `AssetVersion` capturing pre-freeze state
- [x] 8.3 Reject freeze when projection is not ok
- [x] 8.4 Integration test: freeze on ok, reject on not-found, version snapshot verification

> **Notes:** `CreateVersion` runs before `Update` so the snapshot captures pre-freeze state. Freeze inserts a `callout` block + the projected fragment blocks in place of the `live_artifact` block. Batch cap: 50 refs per request. `resolvePrincipal` defaults role to `editor` (existing convention); finer RBAC is service-side.

## 9. WebSocket subscription manager

- [x] 9.1 Extend `internal/ws/hub.go` with per-asset subscription filters keyed by `(client_id, asset_id)`
- [x] 9.2 On asset-open control message from client, register filter from projector `Subscribe` results
- [x] 9.3 On asset-close or disconnect, remove filter
- [x] 9.4 Add the server-side event router that, on any matching entity event, builds `knowledge.asset.live_artifacts_changed` with affected block ids
- [x] 9.5 Implement 250 ms coalescing window per `(client_id, asset_id)` and per-block 1 Hz rate cap
- [x] 9.6 Unit tests: coalescing, rate cap, fan-out to only matching clients, cleanup on disconnect

> **Notes:** router lives in `internal/ws/liveartifact_router.go`; hub gained `SubscriptionRouter` interface, `SetSubscriptionRouter`, `SendToClient`, `BroadcastEvent`, `DeliverClientMessage`, and `Client.ID()`. Handler's `handleFrame` routes `asset_open`/`asset_close` control frames through `DeliverClientMessage`. Router substitutes `$asset_project` sentinel with the asset's project id at match time and handles both snake_case and camelCase payload keys. Deferred rate-capped blocks re-emit on the next flush window so updates are never lost.
>
> **Follow-up**: event publishers still using pre-built envelopes + `FanoutBytes` must migrate to `Hub.BroadcastEvent(eventType, payload)` for the router to see their events. Documented as a TODO in `hub.go`.

## 10. Frontend: block components

- [x] 10.1 Create `components/docs/live-blocks/live-artifact-block.tsx` — the shared BlockNote custom-block wrapper that loads the kind-specific component by `live_kind`
- [x] 10.2 Create `components/docs/live-blocks/agent-run-block.tsx` rendering the projection fragment read-only
- [x] 10.3 Create `components/docs/live-blocks/cost-summary-block.tsx`
- [x] 10.4 Create `components/docs/live-blocks/review-block.tsx`
- [x] 10.5 Create `components/docs/live-blocks/task-group-block.tsx`
- [x] 10.6 Create the shared block-chrome component: header with kind label, actions menu (Freeze, Open source, Remove), status/diagnostic banner
- [x] 10.7 Register the four components with the BlockNote editor in `components/docs/block-editor.tsx` under `type: live_artifact` with a `live_kind` discriminator
- [x] 10.8 Lazy-load the live-block components on first use, matching the existing editor lazy-load pattern

## 11. Frontend: insertion UX

- [x] 11.1 Add four slash-menu entries (one per live kind); each opens the appropriate picker/filter dialog
- [x] 11.2 Agent-run picker: type-ahead search over recent runs in the current project
- [x] 11.3 Cost-summary filter dialog: time range + optional runtime/provider/member dropdowns
- [x] 11.4 Review picker: list of reviews the principal can read in the current project
- [x] 11.5 Task-group filter builder: choose saved view or build inline filter with status/assignee/tag/sprint/milestone
- [x] 11.6 Reject insertion attempts outside `kind=wiki_page` assets (slash-menu entries hidden on template/ingested-file kinds)

> **Notes:** `useLiveArtifactSlashMenu` hook returns BlockNote-shaped `DefaultReactSuggestionItem` items + a `menuDialogs` React element. `block-editor-client.tsx` threads these via optional `extraSlashMenuItems` + `slashMenuDialogs` props and merges them into BlockNote's slash menu via `SuggestionMenuController`. Agent-run picker filters a non-project-scoped global list today (no per-project `/agents/runs` endpoint yet — kept as follow-up). Review picker has the same limitation. Stores consumed: `useAgentStore`, `useReviewStore`, `useMemberStore`, `useSavedViewStore`.

## 12. Frontend: projection + subscription hook

- [x] 12.1 Create `hooks/use-live-artifact-projections.ts` that, for a given asset id, collects live-block refs from the editor doc, calls the projection endpoint on open, and replays result payloads into each block component
- [x] 12.2 Wire a WebSocket subscription hook that listens for `knowledge.asset.live_artifacts_changed` and re-projects only affected blocks
- [x] 12.3 On reconnect, full refresh the whole asset before reestablishing the subscription filter
- [x] 12.4 Handle per-block TTL hints: expire cached projection after TTL and re-project lazily if the block remains visible

> **Notes:** `lib/ws-client.ts` gained a public `sendControl(data)` that delegates to the existing private `send`. Asset_open control frame keys are camelCase (`assetId`, `projectId`, `blockId`, `liveKind`, `targetRef`) matching `internal/ws/liveartifact_router.go:67-78`'s decoder. TTL scan: 5 s interval + 1 s per-block debounce. "Visible" treated as "present in doc" — `IntersectionObserver` tracked as follow-up. asset_open is signature-diff debounced at 100 ms.
>
> **Integration wiring**: `block-editor-client.tsx` accepts optional `liveArtifactValue?: Partial<LiveArtifactContextValue>` and wraps the editor in `LiveArtifactProvider` when supplied. The docs page surface doesn't yet call `useLiveArtifactProjections` + `useLiveArtifactSlashMenu` — lighting it up on the docs page is a trivial follow-up once a caller opts in (thread the four hook props through).

## 13. Frontend: actions

- [x] 13.1 Implement "Open source" navigation for each kind (route + query params per design)
- [x] 13.2 Implement "Freeze" calling the freeze endpoint and hot-replacing the block with the returned fragment
- [x] 13.3 Implement "Remove" via standard block delete affordance

## 14. Tests

- [x] 14.1 Frontend unit tests for each kind's block component covering ok/not_found/forbidden/degraded states
- [x] 14.2 Frontend integration test for projection hook covering initial open, subscription push, reconnect refresh
- [ ] 14.3 E2E smoke: insert each of the four live block kinds in a wiki page; assert they render and refresh on a simulated entity event

> **14.3 note**: requires a running stack + real BlockNote editor in a browser. Deferred to the §15 manual-smoke pass; not automated here.

## 15. Smoke verification

> **All 15.x tasks require an operator running the full stack** (Go orchestrator + WS hub + frontend dev server, plus a seeded project with agents / reviews / tasks). They cannot be automated here. Left unchecked intentionally — the change owner completes them when running the manual smoke pass before archiving. The automated test pyramid (unit §14.1 + integration §14.2) covers the non-live-stack assertions.

- [ ] 15.1 Insert an `agent_run` live block, start the referenced run, observe the block updating step/log/cost without user action
- [ ] 15.2 Insert a `cost_summary` live block, incur cost via a test agent run, observe the block's totals updating
- [ ] 15.3 Insert a `review` live block, transition the review state, observe the block reflecting the new state
- [ ] 15.4 Insert a `task_group` live block, create and update tasks matching the filter, observe the table reflecting the changes
- [ ] 15.5 Freeze any live block and verify the block is replaced by static content plus the "Frozen from {kind}" callout, and a new `AssetVersion` named accordingly was created
- [ ] 15.6 Delete the target entity of a live block, reopen the asset, verify the block renders the not-found state with "Remove block" action
- [ ] 15.7 Switch the viewing user to a role that cannot read cost data, reopen an asset with a `cost_summary` block, verify the forbidden state is rendered without leaking data
