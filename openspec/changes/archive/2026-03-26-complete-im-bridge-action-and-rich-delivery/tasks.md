## 1. Canonical Delivery Contract

- [x] 1.1 Extend `src-go/internal/model/im.go` and related bridge/client payload structs with a canonical typed outbound delivery envelope and canonical action result shape while preserving legacy text-only compatibility fields.
- [x] 1.2 Refactor `src-go/internal/service/im_service.go` and `src-go/internal/service/im_control_plane.go` so `Notify(...)`, `Send(...)`, control-plane queueing, and bound progress all build the same typed delivery contract and signature input.
- [x] 1.3 Update `src-im-bridge/client`, `src-im-bridge/cmd/bridge/control_plane.go`, and compatibility notify/send parsing so replayed and direct deliveries consume the typed envelope without dropping reply-target data or fallback metadata.

## 2. Backend Action Execution

- [x] 2.1 Introduce a focused backend action execution seam that maps shared IM actions such as `assign-agent`, `decompose`, `approve`, and `request-changes` onto the existing task dispatch, decomposition, and review workflows.
- [x] 2.2 Replace the placeholder `/api/v1/im/action` acknowledgements in `src-go/internal/service/im_service.go` with truthful execution outcomes that report started, completed, blocked, or failed results plus preserved reply-target metadata.
- [x] 2.3 Add stale-entity, invalid-transition, and blocked-workflow handling so unsupported or outdated interactive actions return explicit terminal failures instead of fake success responses.

## 3. Bridge Delivery Resolution

- [x] 3.1 Unify `src-im-bridge/notify/receiver.go` and `src-im-bridge/cmd/bridge/control_plane.go` around shared delivery-resolution helpers for text, structured, and provider-native payloads.
- [x] 3.2 Ensure queued progress and terminal updates reuse the canonical typed delivery contract so Slack thread replies, Discord follow-up or edit paths, Feishu delayed updates, DingTalk session-webhook replies, and Telegram edits can survive queueing and replay.
- [x] 3.3 Expand focused tests across `src-go` and `src-im-bridge` for rich replay, fallback-reason propagation, canonical action outcomes, and compatibility HTTP parity.

## 4. Docs And Verification

- [x] 4.1 Update `src-im-bridge/README.md`, `.env.example`, and related runbook content to document action closure, typed delivery semantics, compatibility fallback behavior, and the supported verification paths.
- [x] 4.2 Extend smoke or manual verification guidance so supported platforms cover action-button closure, control-plane replay, and rich-delivery fallback expectations in addition to basic command replies.
- [x] 4.3 Run focused verification for the affected `src-go` and `src-im-bridge` surfaces, then sync the task checklist and OpenSpec artifacts with the observed results.
