## Context

The dispatch system has its core service layer implemented (`TaskDispatchService`, `AgentService`, `AdmissionController`, pool/queue repository) with correct outcome semantics (`started`, `queued`, `blocked`, `skipped`). However, several integration points are disconnected:

1. **Budget governance exists but is never called during dispatch.** `BudgetGovernanceService` has `CheckSprintBudget()` and `CheckProjectBudget()` methods, but `TaskDispatchService.Assign()`, `Spawn()`, and queue promotion paths bypass them entirely.
2. **No dispatch preflight endpoint.** Callers commit to dispatch without knowing budget remaining or pool availability upfront.
3. **No dispatch observability.** The frontend agent monitor shows pool stats and queue entries but lacks per-run dispatch status and historical dispatch metrics.
4. **Queue is FIFO-only.** `ReserveNextQueuedByProject()` orders by `created_at ASC` with no priority field.
5. **Assignment recommender is untested.** `AssignmentRecommender.Recommend()` has zero test coverage.

The existing code is well-structured with clear interfaces (`DispatchRuntimeService`, `DispatchQueueWriter`, `DispatchNotificationService`), making integration points straightforward.

## Goals / Non-Goals

**Goals:**
- Wire budget governance into the dispatch admission path so budget-blocked becomes a first-class guardrail alongside pool-full and worktree-unavailable.
- Expose a preflight API that returns budget readiness, pool availability, and cost estimation before dispatch commitment.
- Add dispatch metrics and history endpoints for operator observability.
- Add a `priority` field to queue entries and update reservation to respect priority ordering.
- Achieve comprehensive test coverage for the assignment recommender and budget-integrated dispatch paths.
- Add frontend dispatch status surfaces reusing existing shared components (`confirm-dialog`, `event-badge-list`, `platform-badge`).

**Non-Goals:**
- Preemption of running agents (low-priority runs displaced by high-priority arrivals). Only queue ordering is priority-aware.
- Dynamic scoring weight configuration for `AssignmentRecommender`. Hardcoded weights remain; configurability is a separate change.
- Redis-based `TaskQueue` changes. The Redis stream queue is a separate distribution mechanism from the in-memory admission pool.
- Cost estimation model (actual per-model pricing). Preflight returns remaining budget and pool state, not dollar forecasts.

## Decisions

### D1: Budget check injection point — `TaskDispatchService` layer, not `AdmissionController`

**Decision:** Add a `DispatchBudgetChecker` interface to `TaskDispatchService` and call it in `spawnForTask()` before attempting runtime spawn or queue admission.

**Rationale:** The `AdmissionController` (in `pool/`) is a pure concurrency limiter — it knows about slots, not money. Budget governance requires project/sprint context that `TaskDispatchService` already has access to. Mixing budget into the pool layer would violate its single responsibility.

**Alternatives considered:**
- Inject budget into `AdmissionController.Decide()`: rejected because admission would need sprint/project readers, coupling pool to domain concepts.
- Budget as a middleware/handler concern: rejected because the dispatch service needs to return structured `blocked` outcomes with budget reason metadata, not HTTP-level rejections.

### D2: Preflight as a read-only endpoint, not a reservation

**Decision:** `GET /api/v1/projects/:pid/dispatch/preflight?taskId=X&memberId=Y` returns a snapshot of budget readiness and pool availability. It does not reserve a slot.

**Rationale:** Reservation introduces complexity (TTL, cleanup, race conditions) for marginal benefit. The dispatch path itself handles races via atomic pool acquisition. Preflight is advisory — it tells the UI "this will probably work" so users can make informed decisions.

### D3: Priority field on queue entries — integer with predefined levels

**Decision:** Add `priority INT NOT NULL DEFAULT 0` to `agent_pool_queue_entries`. Higher values = higher priority. Define constants: `PriorityLow = 0`, `PriorityNormal = 10`, `PriorityHigh = 20`, `PriorityCritical = 30`. Reservation orders by `priority DESC, created_at ASC`.

**Rationale:** Integer priority is simple, extensible, and avoids enum migrations. The predefined constants give semantic meaning without restricting future values. FIFO is preserved as a tiebreaker within the same priority level.

### D4: Dispatch stats as aggregation queries, not a materialized table

**Decision:** `GET /api/v1/projects/:pid/dispatch/stats` aggregates from `agent_pool_queue_entries` and `agent_runs` using COUNT/AVG queries. No new stats table.

**Rationale:** Dispatch volume is low enough (hundreds/day at most) that aggregation queries are fast. A materialized stats table adds write overhead and staleness risk for minimal benefit.

### D5: Frontend dispatch status surfaces reuse existing component patterns

**Decision:** Dispatch status in the agent run table reuses `event-badge-list` for status badges, `confirm-dialog` for the preflight confirmation, and the task-context-rail pattern for the dispatch history panel. No new component library entries.

**Rationale:** These components already handle the exact UI patterns needed (badge lists with color-coded status, confirmation dialogs with structured data, sidebar panels with lists). Building new components would duplicate existing patterns.

## Risks / Trade-offs

- **Budget check adds latency to dispatch path** → Mitigation: budget check is a single DB read (sprint or project budget). Sub-millisecond on indexed tables. If sprint reader is unavailable, fail open with a warning (consistent with existing auth pattern of fail-closed only for security-critical paths).
- **Priority queue ordering changes existing FIFO behavior** → Mitigation: Default priority is 0 (same as current implicit priority). Existing entries get priority 0 via migration default. No behavior change until callers explicitly set higher priority.
- **Preflight endpoint could become stale between check and dispatch** → Mitigation: Document as advisory. The actual dispatch path performs the authoritative check. UI can show "conditions may have changed" if the dispatch returns a different outcome than preflight predicted.

## Open Questions

- Should budget-blocked queue entries be auto-retried when budget is replenished (e.g., sprint budget increased)? Currently proposed as: entries stay queued with budget-blocked reason, operator must manually re-trigger. Auto-retry adds complexity.
