## 1. Operator console wiring and backend contracts

- [x] 1.1 Inject the live IM sender into `src-go/internal/server/routes.go` / `src-go/internal/handler/im_control_handler.go` so `/api/v1/im/test-send` uses the canonical sender path instead of returning unavailable in normal runtime wiring.
- [x] 1.2 Tighten `src-go/internal/handler/im_control_handler.go` and related IM model/handler tests so test-send returns truthful delivered/failed/pending outcomes and explicit unavailable failures.
- [x] 1.3 Replace ad hoc event-type inventory handling with one authoritative backend event inventory source that can be consumed consistently by routing code and `GET /api/v1/im/event-types`.

## 2. Channel-scoped routing consumption

- [x] 2.1 Extend `src-go/internal/service/im_control_plane.go` with helpers for resolving active configured channels by event subscription, platform, and compatibility-fallback precedence.
- [x] 2.2 Update `src-go/internal/service/wiki_service.go` so document-related IM forwarding uses configured channel/event routing first and only falls back to legacy env targets when no configured route matches.
- [x] 2.3 Update `src-go/internal/service/automation_engine_service.go` so `send_im_message` uses authoritative routing truth, fails explicitly when no usable route exists, and records delivery through the canonical IM pipeline.
- [x] 2.4 Add focused Go tests covering subscribed-channel routing, inactive/unsubscribed suppression, and compatibility fallback behavior for wiki and automation IM delivery.

## 3. Bridge event forwarding parity

- [x] 3.1 Align TS Bridge and Go backend event inventories so runtime-forwarded events such as `budget_alert`, `status_change`, and `permission_request` have canonical Go-side handling and truthful IM routing semantics.
- [x] 3.2 Update `src-go/internal/service/agent_service.go` / IM progress helpers so forwarded budget and status events prefer bound reply-target routing, preserve ordering metadata, and avoid fabricated watcher delivery when no authoritative route exists.
- [x] 3.3 Tighten `src-go/internal/service/im_control_plane.go` and related tests to enforce reply-target preference filters like `bridge_event_enabled.<type>` while keeping suppressed deliveries truthful in diagnostics/history.
- [x] 3.4 Add focused `src-bridge` and `src-go` tests covering forwarded event mapping, ordering, suppression, and replay behavior across the affected event types.

## 4. IM action entrypoints for message conversion

- [x] 4.1 Reuse `src-go/internal/service/im_action_execution.go` message conversion workflows (`save-as-doc`, `create-task`) as the canonical backend path and tighten result metadata where needed for IM follow-up delivery.
- [x] 4.2 Expose user-facing IM entrypoints in `src-im-bridge` cards/actions/interaction normalization so message-backed surfaces can trigger `save-as-doc` and `create-task` without introducing a parallel workflow.
- [x] 4.3 Preserve source message metadata and reply-target lineage end-to-end across `/im/action`, backend execution, and IM result rendering for message conversion outcomes.
- [x] 4.4 Add focused `src-im-bridge` and Go tests covering message-conversion action exposure, successful workflow execution, and truthful failure replies.

## 5. `/im` frontend completion and verification

- [x] 5.1 Update `lib/stores/im-store.ts`, `app/(dashboard)/im/page.tsx`, and `components/im/*` so `/im` consumes the authoritative event inventory, reflects configured test targets, and surfaces explicit test-send failures.
- [x] 5.2 Ensure IM history/health/operator UI reflects channel-routed delivery truth, including refreshed metrics/history after test-send and any new diagnostics fields needed by the backend changes.
- [x] 5.3 Add or update frontend tests for `/im` test-send behavior, event inventory loading, and configured-target selection.
- [x] 5.4 Run focused verification for affected Go, `src-im-bridge`, `src-bridge`, and frontend slices, then record any remaining repo-wide failures outside this change without overstating completion.
