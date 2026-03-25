## Why

当前 `src-im-bridge` 已经支持多平台单活运行，也已经为飞书实现了基础 interactive card、按钮回调和 delayed update 路径，但平台启动与能力装配仍主要依赖 `cmd/bridge/platform_registry.go` 中的硬编码注册表。随着仓库主系统已经形成统一的 plugin manifest、runtime、capability 词汇，这种“桥内私有注册表 + 平台代码直连”的模式开始限制 IM Bridge 的后续扩展，尤其不利于继续增强飞书卡片、模板变量、回调 schema 2.0、链接预览等 richer capability。

现在推进这项变更的原因是两条线已经汇合：一方面，仓库已有明确的 IntegrationPlugin / Go WASM runtime 方向；另一方面，飞书官方能力已经把卡片发送、卡片模板变量、`card.action.trigger` 回调、延时更新卡片和长连接接收事件这些能力做成了清晰的正式契约。我们需要先把 IM Bridge 的平台扩展面整理成 plugin-compatible contract，再在这个 contract 之上把飞书 richer capability 做成可持续扩展的一等能力，而不是继续在单个平台目录里堆条件分支。

## What Changes

- 为 `src-im-bridge` 引入面向平台适配器的 plugin-compatible provider contract，收敛平台描述、配置校验、能力声明、出入站消息处理和可选交互能力装配方式。
- 将当前硬编码 `platformDescriptors()` 迁移为内建 provider 描述与装配机制，保留“单进程单活平台”的现有部署模型，但让新增平台或后续外置插件不再依赖继续改主启动分支。
- 为飞书补齐 richer capability contract，包括卡片 JSON 与模板卡片的统一发送面、卡片变量/模板元数据入口、`card.action.trigger` 新版回调语义、3 秒内响应与 30 分钟内延时更新的生命周期约束。
- 明确 Feishu provider 的能力声明与降级策略，让 bridge/notify/backend 控制面都能知道什么时候可以原位更新卡片、什么时候只能回复文本、什么时候需要显式降级。
- 为后续更多飞书能力预留可扩展接口，例如链接预览、卡片局部更新、消息卡片模板版本与多语言变量，而不要求本次一次性实现所有 surface。
- 更新 IM Bridge 文档、运行说明与验证入口，使“平台 provider seam + 飞书 richer capability”成为后续功能扩展的真实基线。

## Capabilities

### New Capabilities
- `im-platform-plugin-contract`: 定义 IM Bridge 平台 provider 的可扩展契约、内建 provider 发现与装配方式，以及与仓库现有 plugin/runtime 词汇兼容的能力声明面。
- `feishu-rich-card-lifecycle`: 定义飞书 richer card 能力的统一生命周期，包括 JSON/模板卡片发送、模板变量入口、交互回调、即时响应与延时更新约束。

### Modified Capabilities
- `additional-im-platform-support`: 将平台启动与配置校验要求从硬编码平台分支扩展为基于 provider contract 的装配模型，同时保留单活平台部署语义。
- `im-platform-native-interactions`: 扩展飞书原生交互规范，使其明确覆盖新版卡片回调、模板卡片、延时更新令牌与 provider-aware 降级策略。

## Impact

- Affected code: `src-im-bridge/cmd/bridge`, `src-im-bridge/core`, `src-im-bridge/platform/feishu`, `src-im-bridge/notify`, `src-im-bridge/client`, 以及相关 README / runbook / smoke 文档。
- Affected architecture: IM Bridge 平台扩展边界将从“硬编码注册表”提升为“内建 provider + plugin-compatible contract”，并与仓库现有 `IntegrationPlugin` / Go runtime 设计对齐。
- External dependencies and references: 飞书官方开放平台的发送消息、回复消息、编辑消息、延时更新卡片、卡片回调、长连接/事件订阅、卡片搭建工具与模板变量文档将作为设计依据。
- Non-goal for this change: 不要求本次把所有现有平台立即迁出为外部可安装插件，也不要求一次性落完所有飞书开放能力；重点是先把扩展 contract 和飞书高价值 surface 做正确。
