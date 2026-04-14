## 1. Canonical preflight evaluation

- [x] 1.1 Introduce a shared dispatch preflight verdict helper in `src-go/internal/service` that resolves task/member context, task/sprint/project budget pressure, active-run conflicts, and pool readiness without committing side effects.
- [x] 1.2 Wire `src-go/internal/handler/dispatch_preflight_handler.go` to the shared evaluator, including additive guardrail fields and optional candidate runtime/budget inputs in the advisory response.
- [x] 1.3 Reuse the shared evaluator from `src-go/internal/service/task_dispatch_service.go` so manual spawn and preflight use the same non-start classification, then add focused consistency tests.

## 2. Queue promotion truth and history

- [x] 2.1 Update `src-go/internal/service/agent_service.go` promotion revalidation to reuse the canonical preflight verdict and distinguish recoverable re-queue versus terminal invalidation.
- [x] 2.2 Extend dispatch attempt recording in `src-go/internal/model/dispatch_attempt.go` and `src-go/internal/repository/dispatch_attempt_repo.go` so promotion rechecks persist queue-linked verdict history entries.
- [x] 2.3 Finalize queue completion and realtime emission ordering for promoted and failed queue events so emitted payloads include the finalized queue state and linked run metadata, with focused service tests.

## 3. Observability contracts and verification

- [x] 3.1 Expand `src-go/internal/handler/dispatch_observability_handler.go` and supporting query paths to expose promotion lifecycle metrics and optional time-window filters for dispatch stats.
- [x] 3.2 Update OpenAPI and AsyncAPI contract docs for the richer preflight, stats, history, and queue lifecycle payloads.
- [x] 3.3 Run focused Go verification for preflight, promotion revalidation, dispatch history, stats, and realtime queue payload ordering, then confirm the change stays apply-ready.
