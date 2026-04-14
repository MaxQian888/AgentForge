## 1. Scheduler model and repository extensions

- [x] 1.1 Extend `src-go/internal/model/scheduler.go` with operator-facing job projection fields, richer stats payloads, and additional run lifecycle statuses needed for cancel-requested / cancelled flows.
- [x] 1.2 Update `src-go/internal/repository/scheduler_repo.go` to support filtered run-history queries, terminal-history cleanup, richer aggregate metrics, and any persistence needed by the extended run lifecycle.
- [x] 1.3 Add scheduler config-metadata structures for built-in jobs so the backend can describe editable fields and unsupported actions without exposing freeform config editing.

## 2. Scheduler service and built-in job control logic

- [x] 2.1 Extend `src-go/internal/scheduler/service.go` with explicit pause / resume orchestration for built-in jobs and truthful job control-state projection.
- [x] 2.2 Implement cooperative run-cancellation support in the scheduler service and built-in handler registration path, including explicit rejection when a job cannot be cancelled.
- [x] 2.3 Add service-layer helpers for schedule preview generation, schema-driven config validation, filtered history access, history cleanup, and richer stats calculation.

## 3. Operator-facing API and realtime contracts

- [x] 3.1 Update `src-go/internal/handler/scheduler_handler.go` and `src-go/internal/server/routes.go` to expose pause / resume / cancel / cleanup / preview / config-metadata operator endpoints while keeping existing scheduler routes compatible.
- [x] 3.2 Extend existing scheduler list/detail/stats responses so consumers receive explicit control state, supported actions, active-run summaries, and upcoming occurrence preview from the backend.
- [x] 3.3 Update scheduler realtime broadcasting so pause / resume / cancel-requested / cancelled transitions and richer run summaries are visible to operator consumers.

## 4. Scheduler frontend consumer alignment

- [x] 4.1 Update `lib/stores/scheduler-store.ts` to consume the expanded scheduler DTOs and operator endpoints instead of inferring paused/running/cancellable state from legacy fields.
- [x] 4.2 Update `app/(dashboard)/scheduler/page.tsx` and `components/scheduler/*` to render supported actions, richer metrics, filtered history, and truthful unsupported-action messaging from backend data.
- [x] 4.3 Ensure the current scheduler workspace can reuse the same backend contract expected by the future `scheduler-control-panel` without reintroducing frontend-only cron preview or control-state guessing.

## 5. Verification and rollout hardening

- [x] 5.1 Add or update focused Go tests for scheduler repository filtering/cleanup, service pause-resume-cancel flows, config validation, preview generation, and handler error semantics.
- [x] 5.2 Add frontend/store contract coverage for the expanded scheduler responses and operator actions, including unsupported-action and history-filter scenarios.
- [x] 5.3 Run targeted verification across `src-go` scheduler paths and the current scheduler UI consumer paths, then document any built-in jobs that intentionally remain non-cancellable in the first rollout.



