## 1. Platform configuration and bootstrap

- [x] 1.1 Extend `src-im-bridge/cmd/bridge/main.go` configuration parsing to support an explicit `IM_PLATFORM` selector plus Slack and DingTalk credential sets.
- [x] 1.2 Replace the Feishu-only startup branch with a platform factory/selection path that instantiates `feishu`, `slack`, or `dingtalk` as the single active platform and fails fast on invalid configuration.
- [x] 1.3 Add or update startup/health logging so the running bridge reports the selected platform and configuration errors clearly.

## 2. Slack and DingTalk platform adapters

- [x] 2.1 Implement `src-im-bridge/platform/slack` behind `core.Platform`, including inbound message mapping into `core.Message` and outbound `Reply`/`Send` support.
- [x] 2.2 Implement `src-im-bridge/platform/dingtalk` behind `core.Platform`, including inbound message mapping into `core.Message` and outbound `Reply`/`Send` support.
- [x] 2.3 Add platform-local test doubles or fakes so Slack and DingTalk command/notification paths can be verified without relying only on live third-party environments.

## 3. Platform-aware command and notification flow

- [x] 3.1 Update `src-im-bridge/client` so backend-bound requests propagate the actual IM source platform instead of hardcoding `feishu`.
- [x] 3.2 Update `src-im-bridge/notify/receiver.go` to validate notification platform matching against the active bridge platform before sending.
- [x] 3.3 Preserve rich-message behavior through `core.CardSender` where available and fall back to plain-text notification content when the active platform does not support cards.

## 4. Verification and documentation

- [x] 4.1 Add or update tests covering Slack and DingTalk startup validation, command routing, source propagation, and notification fallback/rejection scenarios.
- [x] 4.2 Update IM Bridge documentation and configuration examples to show how to run Feishu, Slack, and DingTalk bridge instances and what capability differences to expect.
- [x] 4.3 Run the relevant bridge validation commands/tests and capture the scoped verification results needed for implementation sign-off.
