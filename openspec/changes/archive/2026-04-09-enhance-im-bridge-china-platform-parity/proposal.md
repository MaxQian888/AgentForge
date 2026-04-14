## Why

AgentForge 的 IM Bridge 经过多轮归档 change 后，已经具备 `feishu`、`dingtalk`、`wecom`、`qq`、`qqbot` 等中国平台的 provider、文档与 targeted tests，但这些平台的真实能力并不对称：Feishu 已有完整的卡片回调与 delayed update 生命周期，DingTalk/WeCom 仍存在显式 richer fallback，QQ/QQ Bot 也主要停留在 text-first 或 markdown-first 语义。现在需要把“平台已接入”进一步收紧为“平台能力是否 truthfully complete”，避免 README、health metadata、control-plane delivery 与真实 provider 行为继续出现平铺式支持错觉。

## What Changes

- 为 IM Bridge 增加一套以运行时真相为准的平台 readiness / parity 审计边界，优先覆盖 `feishu`、`dingtalk`、`wecom`、`qq`、`qqbot`，明确区分“完整 native lifecycle”“native send only”“text-first fallback”“暂不支持的 richer path”。
- 收紧中国平台的 native interaction 与 rich delivery 合同：Feishu 作为完整卡片生命周期基线；DingTalk 明确 ActionCard 发送、回调、异步完成与 truthful fallback；WeCom 明确模板卡片、回调、可更新路径与显式降级；QQ / QQ Bot 明确 text/markdown 边界、reply-target 持久化与不可伪装的限制。
- 统一各中国平台在 `/im/send`、`/im/notify`、`/ws/im-bridge` replay、`/api/v1/im/action` 里的 provider-owned rendering / fallback 行为，让 control-plane、compat HTTP 与 action completion 使用同一套 provider-aware 结果与 downgrade reason。
- 更新 README、runbook、health/registration metadata 与 focused verification matrix，使平台说明、操作面可见状态、以及测试边界和最新官方接入约束保持一致，而不是继续沿用“接上了就算完整支持”的口径。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `additional-im-platform-support`: 将“已支持平台”从静态枚举收紧为带 readiness tier 的运行时真相，要求中国平台的 live/stub、reply-target、health metadata 与文档边界一致。
- `im-platform-native-interactions`: 收紧 Feishu / DingTalk / WeCom / QQ / QQ Bot 的 native callback、mutable update、async completion 与 truthful downgrade 合同，避免继续以 Feishu happy path 代表所有中国平台。
- `im-provider-rendering-profiles`: 要求中国平台声明可实际兑现的 native surfaces、structured surfaces、update semantics 与 formatting constraints，并把受限能力显式暴露为 profile truth。
- `im-rich-delivery`: 要求中国平台的 typed envelope、rendering plan、reply-target restoration 与 fallback metadata 在 direct notify、compat HTTP、queue replay 和 action completion 中保持一致。
- `feishu-rich-card-lifecycle`: 保留 Feishu 作为完整 rich-card baseline，但补充它与其他中国平台做 parity 比较时必须暴露 delayed update token、callback window 与 fallback reason 的契约。
- `wecom-im-platform-support`: 收紧 WeCom 的 callback、template-card、reply-target 恢复与 richer-path fallback 语义，防止把可接入等同于具备 Feishu 级别更新能力。
- `qq-im-platform-support`: 明确 QQ 的 text-first / no-native-update 边界、reply-target 恢复与 structured downgrade 语义，确保 README 与 health surface 不再暗示不存在的 richer parity。
- `qqbot-im-platform-support`: 明确 QQ Bot 的 markdown / keyboard / webhook / OpenAPI 能力边界、回调限制与异步 completion 语义，避免把 markdown 能力误表述成完整 card lifecycle。

## Impact

- Affected code: `src-im-bridge/platform/feishu`, `src-im-bridge/platform/dingtalk`, `src-im-bridge/platform/wecom`, `src-im-bridge/platform/qq`, `src-im-bridge/platform/qqbot`, `src-im-bridge/core`, `src-im-bridge/notify`, `src-im-bridge/cmd/bridge`。
- Affected docs: `src-im-bridge/README.md`, `src-im-bridge/docs/platform-runbook.md`, 以及相关 smoke / verification 指引。
- Affected specs: `additional-im-platform-support`, `im-platform-native-interactions`, `im-provider-rendering-profiles`, `im-rich-delivery`, `feishu-rich-card-lifecycle`, `wecom-im-platform-support`, `qq-im-platform-support`, `qqbot-im-platform-support`。
- Affected integrations: Feishu Open Platform 卡片回调与延时更新、钉钉 Stream + ActionCard、企业微信回调与模板卡片、QQ OneBot / NapCat、QQ Bot webhook + OpenAPI。
