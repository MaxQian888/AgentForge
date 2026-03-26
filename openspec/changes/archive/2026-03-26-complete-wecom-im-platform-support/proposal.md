## Why

`src-im-bridge` 的内置 IM provider 目前只有 `wecom` 仍停留在“模型和 provider contract 已预留，但 runtime 不可启动”的半成品状态。既然仓库已经把 Feishu、Slack、DingTalk、Telegram、Discord 的启动、交互、rich delivery 和控制面链路补齐，继续保留 `wecom` 这种挂名 connector 会让“现有社交媒体连接器功能完整”这件事始终不成立，也会让健康检查、文档、枚举、控制面和 smoke 验证矩阵继续失真。

## What Changes

- 为 IM Bridge 补齐 `wecom` 的 runnable provider，实现与现有 provider contract 一致的 descriptor、stub/live 启动路径、配置校验、health/registration metadata、以及最小可用的命令与通知闭环。
- 为 WeCom 定义明确的平台能力矩阵、reply-target 语义、structured/native 降级策略和控制面兼容边界，避免继续把它标成“planned only”。
- 对齐 `src-go` 与 `src-im-bridge` 的 IM 平台枚举、payload、runbook、README、smoke/验证矩阵，让“现有内置连接器”在文档和运行时都可被 truthfully 声明为 supported 或 explicitly degraded。
- 补充面向现有 connector 完整性的 focused verification，确保 `wecom` 落地后不会破坏已经存在的 Feishu、Slack、DingTalk、Telegram、Discord 行为基线。

## Capabilities

### New Capabilities
- `wecom-im-platform-support`: 定义 AgentForge IM Bridge 中 WeCom provider 的启动、消息归一化、通知投递、reply-target 与显式降级合同。

### Modified Capabilities
- `additional-im-platform-support`: 将可启动的内置 live/stub platform 集合从 Feishu、Slack、DingTalk、Telegram、Discord 扩展到包含 WeCom，并补充其 transport/config/runtime metadata 要求。
- `im-platform-plugin-contract`: 将 provider contract 的 runnable built-in provider 集合扩展到 WeCom，并要求 health、registration、capability matrix 与 unavailable/planned 语义同步更新。
- `im-rich-delivery`: 为 WeCom 增加 typed delivery 在 text/structured/action/update 场景下的支持与显式 fallback 语义，确保控制面与兼容 HTTP 路径不会把它当成未定义平台。

## Impact

- Affected code: `src-im-bridge/cmd/bridge`, `src-im-bridge/core`, `src-im-bridge/notify`, `src-im-bridge/client`, `src-im-bridge/platform/wecom`（新）, `src-im-bridge/scripts/smoke`, `src-im-bridge/docs/platform-runbook.md`, `src-im-bridge/README.md`, `src-go/internal/model/im.go` 及相关测试。
- Affected APIs: `/api/v1/im/send`, `/api/v1/im/notify`, `/api/v1/im/action`, `/api/v1/im/bridge/register`, `/api/v1/im/bridge/heartbeat`, `/ws/im-bridge` 的 WeCom 平台契约与 metadata。
- Affected systems: IM provider startup/config validation, platform health/registration surfaces, compatibility HTTP fallback, control-plane replay, smoke/manual verification matrix.
