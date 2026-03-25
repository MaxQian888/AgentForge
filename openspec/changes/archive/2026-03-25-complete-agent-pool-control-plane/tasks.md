## 1. AgentPool control-plane domain and contracts

- [x] 1.1 Add AgentPool control-plane domain models, persistence schema, and repositories for queue entries, admission state, and authoritative pool summary DTOs.
- [x] 1.2 Refactor `src-go/internal/pool/**` behind an admission-oriented service contract that can return `started`, `queued`, and `blocked` results instead of only `ErrPoolFull`.
- [x] 1.3 Extend Go ↔ Bridge contracts so the backend can read runtime-pool diagnostics, warm-slot counts, and degraded-health metadata from the Bridge.

## 2. Admission flow integration in spawn and dispatch

- [x] 2.1 Update `src-go/internal/service/agent_service.go` so manual spawn uses AgentPool admission, creates real `agent_runs` only on immediate admission, and records queue entries when capacity is exhausted.
- [x] 2.2 Update `src-go/internal/service/task_dispatch_service.go`, related handler DTOs, and `DispatchOutcome` so assignment-triggered dispatch can return `queued` with an admission reference instead of collapsing capacity limits into `blocked`.
- [x] 2.3 Implement release-driven admission promotion and startup reconciliation so queued entries are promoted through the canonical spawn path when slots become available.

## 3. Bridge runtime pool and warm-slot support

- [x] 3.1 Expand `src-bridge/src/runtime/pool-manager.ts` from a max-concurrency counter into a runtime-pool manager that tracks active slots, warm slots, cold starts, warm reuse, and pool health summaries.
- [x] 3.2 Add Bridge API support for pool diagnostics and any required warm-slot lifecycle operations, then wire the Go bridge client to consume those summaries.
- [x] 3.3 Ensure pause, resume, cancel, and shutdown flows keep warm-slot and pool-summary bookkeeping consistent with the new control-plane semantics.

## 4. Operator-facing API, realtime events, and dashboard surfaces

- [x] 4.1 Expand authenticated AgentPool APIs to return authoritative summary and queue roster data needed by Web operators.
- [x] 4.2 Emit explicit AgentPool realtime events for queued, promoted, failed-promotion, and summary-refresh states, and update the relevant stores to consume them truthfully.
- [x] 4.3 Update the dashboard Agent Monitor and agent detail surfaces to show active, warm, queued, available, and queue-roster information from the control plane instead of deriving authoritative counts from the run list alone.

## 5. Verification and rollout hardening

- [x] 5.1 Add focused backend tests for admission decisions, queue persistence, promotion-on-release, spawn compensation, and expanded dispatch outcomes.
- [x] 5.2 Add Bridge and frontend coverage for warm-slot bookkeeping, pool diagnostics APIs, realtime AgentPool events, and queued admission rendering.
- [x] 5.3 Run targeted verification for Go spawn/dispatch flows, Bridge runtime-pool behavior, and dashboard pool surfaces, then document any deployment or operator expectations introduced by the new AgentPool control plane.
