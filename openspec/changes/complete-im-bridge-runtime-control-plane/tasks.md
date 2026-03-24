## 1. Backend control-plane foundation

- [ ] 1.1 Add backend Bridge instance registration, heartbeat expiry, and unregister handling with persisted `bridge_id`, platform, transport, capability, and project-binding metadata.
- [ ] 1.2 Add authenticated control-plane delivery primitives in `src-go`, including delivery signing, `delivery_id` generation, target `bridge_id` routing, and stale-instance rejection.
- [ ] 1.3 Add pending-delivery storage and replay cursors so notifications and progress events can resume safely after Bridge reconnect.

## 2. Bridge runtime registration and secure delivery

- [ ] 2.1 Extend `src-im-bridge/cmd/bridge` configuration and startup flow to create or load a stable `bridge_id`, register on startup, heartbeat while running, and unregister on graceful shutdown.
- [ ] 2.2 Introduce a persistent Bridge control-plane channel that can receive targeted notifications or progress events and acknowledge replay cursors back to the backend.
- [ ] 2.3 Protect compatibility endpoints such as `/im/send` and `/im/notify` with shared-secret verification and `delivery_id` idempotency checks.

## 3. Reply-target persistence and progress streaming

- [ ] 3.1 Extend normalized inbound message handling to capture serializable reply-target metadata for Slack, Discord, Feishu, Telegram, and the other currently supported platforms.
- [ ] 3.2 Persist reply-target metadata alongside the backend action that was triggered so long-running task, agent, review, and decomposition flows can emit asynchronous progress updates.
- [ ] 3.3 Implement the three-stage IM update contract for long-running actions: immediate acceptance, bounded periodic progress heartbeat, and terminal completion or failure summary.
- [ ] 3.4 Prefer provider-appropriate update behavior per platform, including threaded follow-ups, deferred interaction replies, and message edits where supported instead of always posting new messages.

## 4. Verification and operational readiness

- [ ] 4.1 Add contract and integration coverage for registration, heartbeat expiry, signature rejection, bridge-targeted delivery, replay after reconnect, and duplicate-delivery suppression.
- [ ] 4.2 Add focused tests for preserved reply targets and long-running progress delivery across Slack threads, Discord deferred replies, and chat-based platforms such as Feishu or Telegram.
- [ ] 4.3 Update IM Bridge docs and runbooks with Bridge registration, control-plane secrets, reconnect behavior, rollback steps, and manual verification steps for progress streaming and targeted delivery.
