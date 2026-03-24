## Why

AgentForge 的 IM Bridge 当前虽然已经有 `feishu`、`slack`、`dingtalk` 三个平台选择与统一命令入口，但运行时仍主要停留在本地 stub 适配器，尚未形成可上线的真实传输层，也还缺少 Telegram、Discord 这类高频海外协作平台。与此同时，平台官方文档已经明确了各自推荐的接入模式，例如 Slack Socket Mode、钉钉 Stream、飞书长连接、Telegram Bot API 轮询或 Webhook、Discord Interactions，这使得现在非常适合把“平台覆盖”和“provider 完整度”一起收敛为新的实现约束。

## What Changes

- 把现有 `feishu`、`slack`、`dingtalk` 平台从 stub-first 骨架提升为具备真实消息接入、命令路由、回复发送、通知投递与错误恢复语义的可上线适配器。
- 为 IM Bridge 新增 Telegram 与 Discord 平台支持，并定义它们与现有 `/task`、`/agent`、`/cost`、`/help`、`@AgentForge` 交互的兼容边界。
- 建立统一的平台能力矩阵，明确纯文本回复、富消息/卡片、按钮交互、异步确认、重试与降级行为，避免每新增一个平台都重新散落实现约束。
- 补齐各平台的配置样例、凭据要求、连接健康检查、命令验证、故障排查与本地测试策略，让 IM Bridge 后续继续扩平台时有稳定模板可复用。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `additional-im-platform-support`: 将现有“飞书之外增加 Slack/钉钉基线支持”的能力扩展为“现有 provider 达到 live transport 完整度，并把活动平台范围扩展到 Telegram 与 Discord，同时定义跨平台能力矩阵、交互确认语义和可靠降级规则”。

## Impact

- Affected code: `src-im-bridge/cmd/bridge`, `src-im-bridge/core`, `src-im-bridge/platform/*`, `src-im-bridge/notify`, `src-im-bridge/client`, 以及相关配置与测试代码。
- Affected integrations: Feishu Open Platform、Slack API / Socket Mode、钉钉开放平台 Stream 推送、Telegram Bot API、Discord Interactions / Application Commands。
- Affected docs: `src-im-bridge/README.md`、IM Bridge 接入与运维文档、平台差异说明、配置模板与故障排查文档。
- Operational impact: 需要新增或细化平台凭据管理、连接续期/重连策略、签名校验或 ack 语义、命令注册/同步流程，以及跨平台验证矩阵。
