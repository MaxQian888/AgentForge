## 1. Control-Plane Contract And State

- [x] 1.1 Extend `src-go/internal/model/im.go` with operator snapshot types, provider diagnostics fields, delivery settlement timestamps, and request/response DTOs for batch retry and test-send flows.
- [x] 1.2 Update `src-go/internal/service/im_control_plane.go` so queued control-plane deliveries are recorded as `pending`, terminal settlement updates history truthfully, and operator snapshot metrics are derived from pending/history state.
- [x] 1.3 Add filtered delivery-history query support and operator snapshot aggregation logic to the IM control-plane service without breaking the existing `/api/v1/im/deliveries` list contract.

## 2. Bridge Settlement And Diagnostics Reporting

- [x] 2.1 Extend the bridge control-plane ack path in `src-im-bridge/cmd/bridge` and related client types so settlement reports carry terminal status, processed timestamp, failure reason, and downgrade reason.
- [x] 2.2 Add optional provider diagnostics metadata reporting on bridge registration/heartbeat so last-known platform diagnostics can flow into the backend operator snapshot.
- [x] 2.3 Update Go-side ack handling in `src-go/internal/service/im_control_plane.go` and `src-go/internal/handler/im_control_handler.go` to persist settlement outcomes and clear pending backlog only after terminal settlement.

## 3. Operator APIs

- [x] 3.1 Implement enriched operator status and filtered history handlers in `src-go/internal/handler/im_control_handler.go` and register any new query/response paths in `src-go/internal/server/routes.go`.
- [x] 3.2 Implement `POST /api/v1/im/deliveries/retry-batch` with explicit delivery-id inputs, per-item outcomes, and reuse of the existing retry logic.
- [x] 3.3 Implement `POST /api/v1/im/test-send` so it reuses the canonical IM send pipeline, waits for bounded settlement, and returns `delivered` / `failed` / `pending` operator results with delivery identifiers.

## 4. Frontend Store And Data Wiring

- [x] 4.1 Expand `lib/stores/im-store.ts` with operator snapshot models, history filters, batch retry actions, and test-send actions aligned with the new backend contract.
- [x] 4.2 Update `lib/stores/im-store.test.ts` to cover pending-vs-settled status truth, enriched status normalization, filtered history loading, batch retry, and test-send result handling.
- [x] 4.3 Add any shared formatter/helpers needed for provider diagnostics, latency display, and delivery counters without duplicating API-shape parsing across components.

## 5. `/im` Operator Console UI

- [x] 5.1 Upgrade `app/(dashboard)/im/page.tsx` and `components/im/im-bridge-health.tsx` into an operator console that shows summary metrics, provider cards, queue/backlog indicators, and diagnostics from the canonical snapshot.
- [x] 5.2 Update `components/im/im-message-history.tsx` to support filters, richer settlement detail fields, explicit empty states, and batch retry for retryable rows.
- [x] 5.3 Add test-send and configure drill-through UX that reuses the existing `/im` channel configuration surface instead of introducing a parallel IM settings page.

## 6. Verification And Documentation

- [x] 6.1 Add Go handler/service tests for operator snapshot aggregation, terminal settlement transitions, filtered delivery history, batch retry, and bounded test-send behavior.
- [x] 6.2 Add frontend page/component tests for IM summary metrics, provider diagnostics, filter flows, batch retry, configure drill-through, and test-send outcomes.
- [x] 6.3 Update `src-im-bridge/README.md`, `src-im-bridge/docs/platform-runbook.md`, and any operator-facing docs to explain the new operator console semantics, pending/settled lifecycle, and new IM operator endpoints.
