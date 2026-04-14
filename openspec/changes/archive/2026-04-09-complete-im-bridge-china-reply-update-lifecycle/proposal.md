## Why

AgentForge 的 IM Bridge 已经把中国平台的 provider、readiness tier、reply-target persistence、以及 truthful fallback 合同铺出来了，但真正可兑现的 reply/update lifecycle 仍然不完整：目前只有 Feishu 具备完整 native update 路径，DingTalk、WeCom、QQ Bot 仍主要停留在 send-or-reply 加 fallback，QQ 更是 text-first。现在需要把“合同 truthful”继续推进到“异步进度和终态回写也尽量走平台原生路径”，避免 control-plane、/im action、以及长任务回写长期停留在 capability 声明已细化、实现却仍偏 fallback-only 的状态。

## What Changes

- 补齐中国平台在长任务 progress / terminal update、interactive action completion、以及 replay recovery 中的 provider-owned reply/update lifecycle，重点覆盖 DingTalk、WeCom、QQ Bot，并保持 QQ 的 text-first truth 不被伪装成 richer parity。
- 收紧共享 delivery / reply strategy / progress streaming 合同，让 control-plane 重放、`/api/v1/im/action` 结果回写、以及 bound progress 更新都先尝试平台原生 reply/update 路径，再按统一 downgrade reason 回退。
- 让 health / registration / capability matrix 与新的 reply/update truth 对齐，避免仍把某些平台表述成“native-send with fallback”但实际异步 completion 只能走 generic text send。
- 为中国平台补 focused contract tests 和 replay/update smoke matrix，证明 reply-target 恢复、provider callback context、fallback metadata、以及 reconnect 后的终态回写都符合运行时真相。
- 本 change 只覆盖 IM Bridge runtime / backend control-plane / provider seams；`/im` operator console 的前端补完继续留在现有更宽的 `enhance-frontend-panel` seam，不在这里重新开平行 UI 线。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `additional-im-platform-support`: 收紧中国平台 health、registration、capability matrix 与异步 reply/update truth 的一致性。
- `im-platform-native-interactions`: 扩展中国平台的 native callback、reply-target、async completion、以及 mutable-update-or-fallback 语义。
- `im-bridge-progress-streaming`: 要求中国平台的 progress / terminal updates 在 queueing、replay、以及 reconnect 后仍优先沿原生 reply/update 路径回写。
- `wecom-im-platform-support`: 收紧 WeCom 的 response-url、direct-send、reply-target restoration、以及 richer update fallback 合同。
- `qq-im-platform-support`: 明确 QQ 的 text-first progress / completion / replay 语义，避免继续把 reply reuse 误写成 richer update 支持。
- `qqbot-im-platform-support`: 收紧 QQ Bot 的 markdown / keyboard send、reply-target reuse、以及 mutable-update fallback 语义。

## Impact

- Affected code: `src-im-bridge/core`, `src-im-bridge/notify`, `src-im-bridge/cmd/bridge`, `src-im-bridge/platform/dingtalk`, `src-im-bridge/platform/wecom`, `src-im-bridge/platform/qq`, `src-im-bridge/platform/qqbot`, and related reply-target / control-plane seams in `src-go`.
- Affected runtime contracts: `/ws/im-bridge` replay, `POST /api/v1/im/action`, bound progress delivery, registration metadata, health metadata, and provider capability matrices.
- Affected verification: China-platform package tests, delivery/reply-strategy contract tests, control-plane replay and progress-update focused verification, and platform smoke/runbook matrix.
- Out of scope for this change: `/im` dashboard/operator UI expansion, generic frontend polling widgets, and unrelated email-provider cleanup.
