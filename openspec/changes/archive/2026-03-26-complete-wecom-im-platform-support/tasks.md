## 1. WeCom Provider Foundation

- [x] 1.1 Add `src-im-bridge/platform/wecom` with stub and live provider entrypoints that satisfy the shared `core.Platform` contract.
- [x] 1.2 Extend `src-im-bridge/cmd/bridge` config loading, provider registry, and descriptor tests so `IM_PLATFORM=wecom` validates required live credentials/callback settings and no longer resolves as planned-only.
- [x] 1.3 Update health, registration, and capability-matrix surfaces so WeCom reports truthful runtime metadata through the same control-plane contracts as the other built-in providers.

## 2. Shared Command And Delivery Integration

- [x] 2.1 Implement WeCom inbound normalization so supported callback messages/events map into `core.Message` and preserve WeCom source plus reply-target context for backend-bound commands.
- [x] 2.2 Extend typed delivery and notify/send resolution so WeCom deliveries use a provider-owned rendering profile with supported richer output and explicit text fallback metadata.
- [x] 2.3 Ensure WeCom reply targets survive action binding, control-plane queueing, and replay, and that missing richer update context degrades truthfully instead of failing silently.

## 3. Models, Compatibility Surfaces, And Docs

- [x] 3.1 Align `src-go/internal/model/im.go` and any bridge/client compatibility payload parsing so WeCom is treated as a fully defined platform across send, notify, action, and control-plane payloads.
- [x] 3.2 Update `src-im-bridge/README.md`, `src-im-bridge/docs/platform-runbook.md`, and related examples to remove planned-only wording and document WeCom live/stub behavior, fallback semantics, and rollout constraints.
- [x] 3.3 Add WeCom smoke fixtures or manual verification guidance so the built-in connector matrix is complete and repo-truthful.

## 4. Verification

- [x] 4.1 Add focused tests for WeCom provider selection, config validation, inbound normalization, typed delivery fallback, and reply-target replay behavior.
- [x] 4.2 Re-run scoped regression coverage for the existing built-in IM providers to confirm the shared provider and delivery seams did not regress while landing WeCom support.
- [x] 4.3 Update the OpenSpec task checklist with the observed verification results and confirm `openspec status --change complete-wecom-im-platform-support --json` reports the change as apply-ready.
