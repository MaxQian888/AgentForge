## 1. Prerequisites

- [ ] 1.1 Confirm `unify-wiki-and-ingested-documents` is archived and the unified `KnowledgeAsset` model is live in the codebase; abort this change until it is
- [ ] 1.2 Audit existing WebSocket event names for `agent_run.*`, `cost.*`, `review.*`, `task.*` to make sure the hub emits what the subscription filters will need; file follow-up tasks if any are missing

## 2. Projector interface and registry

- [ ] 2.1 Create `src-go/internal/knowledge/liveartifact/types.go` defining `LiveArtifactKind`, `ProjectionStatus`, `ProjectionResult`, `EventTopic`, and `LiveArtifactProjector`
- [ ] 2.2 Create `src-go/internal/knowledge/liveartifact/registry.go` with `Register(p LiveArtifactProjector)` and `Lookup(kind) (LiveArtifactProjector, bool)`
- [ ] 2.3 Wire the registry into the server bootstrap so all projectors register at startup

## 3. Agent-run projector

- [ ] 3.1 Create `agent_run_projector.go` implementing the interface against `model.AgentRun` and the agent-run repository
- [ ] 3.2 Produce a BlockNote fragment containing title/status/runtime/duration/cost/steps/last-N-logs
- [ ] 3.3 Implement `Subscribe` returning topics for `agent_run.status_changed`, `agent_run.log_appended`, `agent_run.step_changed`, `agent_run.cost_updated` scoped to the target id
- [ ] 3.4 Unit tests: running vs completed, log-line count respected, forbidden path, not-found path, Subscribe returns scoped topics

## 4. Cost-summary projector

- [ ] 4.1 Create `cost_summary_projector.go` with an aggregate query over the project's `cost_accounting` rows for the requested window
- [ ] 4.2 Implement grouping by runtime/provider/member as declared in `view_opts`
- [ ] 4.3 Compute delta vs the prior equal-length window and render a compact summary fragment
- [ ] 4.4 Enforce the cost-read sub-permission; return `forbidden` if the principal lacks it even with project-viewer
- [ ] 4.5 Implement `Subscribe` returning `cost.accounting_updated` topics relevant to the window
- [ ] 4.6 Unit tests: grouping, delta computation, forbidden path, time-range edge cases (zero-spend window, partial window)

## 5. Review projector

- [ ] 5.1 Create `review_projector.go` against `model.Review`
- [ ] 5.2 Render fragment for in-progress and finalized review states, including linked task title
- [ ] 5.3 Implement `Subscribe` for `review.state_changed`, `review.findings_updated`
- [ ] 5.4 Unit tests: each state pill, linked-task resolution, forbidden, not-found

## 6. Task-group projector

- [ ] 6.1 Create `task_group_projector.go` supporting both saved-view and inline-filter forms of `target_ref.filter`
- [ ] 6.2 Enforce the 50-row page cap with a "N more" footer when truncated
- [ ] 6.3 Implement `Subscribe` returning topics that match the filter — at minimum `task.created`, `task.updated`, `task.deleted` scoped by project
- [ ] 6.4 Unit tests: saved-view form, inline filter, truncation, status-change reflected after subscription push

## 7. Projection REST endpoint

- [ ] 7.1 Add `POST /api/v1/projects/:pid/knowledge/assets/:id/live-artifacts/project` to `internal/handler/knowledge_asset_handler.go`
- [ ] 7.2 Handler batches block refs, runs each projector, assembles `{block_id: ProjectionResult}` response
- [ ] 7.3 Handler resolves `PrincipalContext` and passes it to each projector
- [ ] 7.4 Integration test: batch with mixed ok/not_found/forbidden; latency under 500ms p95 for batches of 10

## 8. Freeze endpoint

- [ ] 8.1 Add `POST /api/v1/projects/:pid/knowledge/assets/:id/live-artifacts/:block_id/freeze`
- [ ] 8.2 Implementation: run projector; if ok, replace the live block in `content_json` with a callout + projected fragment; create `AssetVersion` capturing pre-freeze state
- [ ] 8.3 Reject freeze when projection is not ok
- [ ] 8.4 Integration test: freeze on ok, reject on not-found, version snapshot verification

## 9. WebSocket subscription manager

- [ ] 9.1 Extend `internal/ws/hub.go` with per-asset subscription filters keyed by `(client_id, asset_id)`
- [ ] 9.2 On asset-open control message from client, register filter from projector `Subscribe` results
- [ ] 9.3 On asset-close or disconnect, remove filter
- [ ] 9.4 Add the server-side event router that, on any matching entity event, builds `knowledge.asset.live_artifacts_changed` with affected block ids
- [ ] 9.5 Implement 250 ms coalescing window per `(client_id, asset_id)` and per-block 1 Hz rate cap
- [ ] 9.6 Unit tests: coalescing, rate cap, fan-out to only matching clients, cleanup on disconnect

## 10. Frontend: block components

- [ ] 10.1 Create `components/docs/live-blocks/live-artifact-block.tsx` — the shared BlockNote custom-block wrapper that loads the kind-specific component by `live_kind`
- [ ] 10.2 Create `components/docs/live-blocks/agent-run-block.tsx` rendering the projection fragment read-only
- [ ] 10.3 Create `components/docs/live-blocks/cost-summary-block.tsx`
- [ ] 10.4 Create `components/docs/live-blocks/review-block.tsx`
- [ ] 10.5 Create `components/docs/live-blocks/task-group-block.tsx`
- [ ] 10.6 Create the shared block-chrome component: header with kind label, actions menu (Freeze, Open source, Remove), status/diagnostic banner
- [ ] 10.7 Register the four components with the BlockNote editor in `components/docs/block-editor.tsx` under `type: live_artifact` with a `live_kind` discriminator
- [ ] 10.8 Lazy-load the live-block components on first use, matching the existing editor lazy-load pattern

## 11. Frontend: insertion UX

- [ ] 11.1 Add four slash-menu entries (one per live kind); each opens the appropriate picker/filter dialog
- [ ] 11.2 Agent-run picker: type-ahead search over recent runs in the current project
- [ ] 11.3 Cost-summary filter dialog: time range + optional runtime/provider/member dropdowns
- [ ] 11.4 Review picker: list of reviews the principal can read in the current project
- [ ] 11.5 Task-group filter builder: choose saved view or build inline filter with status/assignee/tag/sprint/milestone
- [ ] 11.6 Reject insertion attempts outside `kind=wiki_page` assets (slash-menu entries hidden on template/ingested-file kinds)

## 12. Frontend: projection + subscription hook

- [ ] 12.1 Create `hooks/use-live-artifact-projections.ts` that, for a given asset id, collects live-block refs from the editor doc, calls the projection endpoint on open, and replays result payloads into each block component
- [ ] 12.2 Wire a WebSocket subscription hook that listens for `knowledge.asset.live_artifacts_changed` and re-projects only affected blocks
- [ ] 12.3 On reconnect, full refresh the whole asset before reestablishing the subscription filter
- [ ] 12.4 Handle per-block TTL hints: expire cached projection after TTL and re-project lazily if the block remains visible

## 13. Frontend: actions

- [ ] 13.1 Implement "Open source" navigation for each kind (route + query params per design)
- [ ] 13.2 Implement "Freeze" calling the freeze endpoint and hot-replacing the block with the returned fragment
- [ ] 13.3 Implement "Remove" via standard block delete affordance

## 14. Tests

- [ ] 14.1 Frontend unit tests for each kind's block component covering ok/not_found/forbidden/degraded states
- [ ] 14.2 Frontend integration test for projection hook covering initial open, subscription push, reconnect refresh
- [ ] 14.3 E2E smoke: insert each of the four live block kinds in a wiki page; assert they render and refresh on a simulated entity event

## 15. Smoke verification

- [ ] 15.1 Insert an `agent_run` live block, start the referenced run, observe the block updating step/log/cost without user action
- [ ] 15.2 Insert a `cost_summary` live block, incur cost via a test agent run, observe the block's totals updating
- [ ] 15.3 Insert a `review` live block, transition the review state, observe the block reflecting the new state
- [ ] 15.4 Insert a `task_group` live block, create and update tasks matching the filter, observe the table reflecting the changes
- [ ] 15.5 Freeze any live block and verify the block is replaced by static content plus the "Frozen from {kind}" callout, and a new `AssetVersion` named accordingly was created
- [ ] 15.6 Delete the target entity of a live block, reopen the asset, verify the block renders the not-found state with "Remove block" action
- [ ] 15.7 Switch the viewing user to a role that cannot read cost data, reopen an asset with a `cost_summary` block, verify the forbidden state is rendered without leaking data
