## Why

AgentForge 目前的 IM Bridge 仍以飞书起步，代码和文档都已经把 Slack 与钉钉列为下一阶段优先平台，但仓库内还没有对应的正式规格与实施约束。为了让海外团队和国内使用钉钉的团队都能直接通过现有 Bridge 访问任务、Agent 与审查流程，需要先把这两类平台支持定义成可实现、可验证的 OpenSpec 变更。

## What Changes

- 为 IM Bridge 增加 Slack 与钉钉平台接入能力，覆盖认证配置、平台启动、消息接收、命令路由与回复发送。
- 统一定义 Slack、钉钉与现有飞书 Bridge 在命令语义上的最小兼容集，确保 `/task`、`/agent`、`/review` 等核心交互在新增平台上具备一致行为。
- 定义平台能力降级规则，使富消息、按钮、卡片或编辑能力不足时能够可靠回退到纯文本响应。
- 补充与新增平台相关的运维与验证要求，包括配置示例、回调/长连接模式选择、日志与故障排查边界。

## Capabilities

### New Capabilities
- `additional-im-platform-support`: 为 AgentForge IM Bridge 引入 Slack Socket Mode 和钉钉 Stream Mode 支持，并定义统一的命令、回复与能力降级行为。

### Modified Capabilities
- None.

## Impact

- Affected code: `src-im-bridge/cmd/bridge`, `src-im-bridge/core`, `src-im-bridge/platform/*`, 以及相关配置加载与通知转发逻辑。
- Affected integrations: Slack API / Socket Mode、钉钉 Stream 模式、现有 AgentForge 后端通知与命令 API。
- Affected docs: IM Bridge 接入说明、配置模板、平台差异说明与故障排查文档。
- Operational impact: 需要新增平台凭据配置、平台特定的连接健康检查与消息格式验证。
