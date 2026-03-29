## Why

PRD 与 `docs/part/CC_CONNECT_REUSE_GUIDE.md` 仍把 `QQ (NapCat/OneBot)` 和 `QQ Bot 官方` 作为 AgentForge IM Bridge 的目标平台与复用目录结构的一部分，但当前 `src-im-bridge/platform/` 只有 `feishu`、`slack`、`dingtalk`、`telegram`、`discord`、`wecom` 六个平台，QQ 系列 provider 完全缺失。这会让 IM Bridge 的平台矩阵与项目文档设计脱节，也会让后续继续扩展平台时绕开已经建立好的 provider contract、control-plane、typed delivery 与组件复用 seam。

## What Changes

- 为 `src-im-bridge` 增加 `QQ (NapCat/OneBot)` live + stub provider，通过现有 provider registry、配置校验、控制面注册、命令归一化与通知回传路径接入共享 IM Bridge 运行时。
- 为 `src-im-bridge` 增加 `QQ Bot 官方` live + stub provider，通过相同的共享 seam 接入，不再把 QQ 系列平台停留在文档承诺或占位目录层面。
- 补齐 QQ 与 QQ Bot 的 capability metadata、rendering profile、reply-target 与显式降级语义，使 shared delivery、`/im/health`、`/ws/im-bridge` replay、`POST /im/notify` 等路径都能按声明能力运行。
- 更新主规格、`src-im-bridge` 文档、runbook、smoke fixtures 与 focused verification matrix，确保项目文档、控制面设计与现有组件复用策略保持一致。

## Capabilities

### New Capabilities
- `qq-im-platform-support`: 定义 QQ (NapCat/OneBot) provider 的运行时接入、命令归一化、reply-target、通知投递与降级合同。
- `qqbot-im-platform-support`: 定义 QQ Bot 官方 provider 的运行时接入、命令归一化、reply-target、通知投递与降级合同。

### Modified Capabilities
- `additional-im-platform-support`: 将当前支持平台范围从 `Feishu/Slack/DingTalk/Telegram/Discord/WeCom` 扩展到包含 `QQ` 与 `QQ Bot`，并要求它们通过共享 provider/runtime seam 提供 live transport、source metadata 与 control-plane-compatible delivery。
- `im-platform-plugin-contract`: 扩展 provider descriptor、config validation、capability metadata 与 rendering profile contract，使 QQ 与 QQ Bot 通过与现有内置平台相同的注册和激活路径启动。
- `im-rich-delivery`: 扩展 typed delivery 合同，使 QQ 与 QQ Bot 的结构化/富消息能力通过 provider-owned rendering profile 解析，并在不支持 richer surface 时显式回退到平台支持的文本或链接表示。

## Impact

- Affected code: `src-im-bridge/cmd/bridge`, `src-im-bridge/core`, `src-im-bridge/platform/*`, `src-im-bridge/notify`, `src-im-bridge/client`, 以及对应测试、smoke script 与文档。
- Affected docs/specs: `docs/PRD.md` 对齐的 IM 平台矩阵、`docs/part/CC_CONNECT_REUSE_GUIDE.md` 复用策略落地、`src-im-bridge/README.md`、`src-im-bridge/docs/platform-runbook.md`、`openspec/specs/*`。
- Affected integrations: NapCat/OneBot WebSocket or HTTP callback intake、QQ Bot 官方开放平台消息与回调能力、IM Bridge 控制面注册/重放、typed outbound delivery。
