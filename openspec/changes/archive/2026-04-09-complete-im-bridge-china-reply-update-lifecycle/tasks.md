## 1. Capability Matrix And Completion Metadata

- [x] 1.1 Extend the China-platform capability matrix and provider metadata to expose async completion truth for DingTalk, WeCom, QQ Bot, and QQ without breaking existing health or registration consumers.
- [x] 1.2 Preserve provider-native completion hints and downgrade categories in reply-target or delivery metadata so direct delivery, bound progress, action completion, and replay can use the same truth.
- [x] 1.3 Add focused tests for health, registration, and capability-matrix output covering China-platform completion-mode metadata.

## 2. Provider Reply And Update Lifecycle

- [x] 2.1 Tighten DingTalk completion behavior so session-webhook and conversation-scoped reply paths are preferred before plain-text fallback.
- [x] 2.2 Tighten WeCom completion behavior so preserved `response_url` reply is preferred before direct app-message send, with explicit fallback metadata when callback context is unavailable.
- [x] 2.3 Tighten QQ Bot completion behavior so preserved `msg_id` and conversation context are reused for supported reply or follow-up delivery before generic fallback.
- [x] 2.4 Tighten QQ completion behavior so progress and terminal updates remain conversation-scoped and text-first instead of drifting toward fake richer parity.

## 3. Shared Delivery, Action Completion, And Replay

- [x] 3.1 Route China-platform progress and terminal updates through one provider-aware delivery decision path shared by direct notify, bound progress, and replayed deliveries.
- [x] 3.2 Route `/api/v1/im/action` completion responses for China platforms through the same provider-aware reply or update path, including explicit downgrade reporting.
- [x] 3.3 Ensure replay recovery preserves provider completion-mode preference and fallback truth for DingTalk, WeCom, QQ Bot, and QQ without duplicating initial acceptance messages.

## 4. Documentation And Focused Verification

- [x] 4.1 Add or extend focused tests across `src-im-bridge/platform/*`, `core`, `notify`, and the relevant `src-go` control-plane seams to cover China-platform completion and replay truth.
- [x] 4.2 Update the IM Bridge README, platform runbook, and smoke or verification matrix so China-platform completion paths and fallback boundaries match runtime truth.
- [x] 4.3 Run the scoped verification commands for the changed seams and record the exact passing commands plus any unrelated blockers that remain outside this change.
