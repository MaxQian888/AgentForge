## Context

IM Bridge 已完成 8 平台的 stub/live 适配器、命令引擎、控制面板注册/心跳/投递回放。但前端 UI 仅覆盖 5 平台（缺 WeCom/QQ/QQ Bot），DingTalk ActionCard 渲染路径未实现，review 子命令未集成，delivery 降级元数据未统一上报。

当前架构是 hub-and-spoke：每个 Bridge 进程服务一个平台，通过 WebSocket 控制面接收投递指令，HTTP 兼容通道兜底。前端通过 Go 后端 REST API 管理 channel/delivery/bridge status。

## Goals / Non-Goals

**Goals:**

1. 前端 `IMPlatform` 类型与 UI 组件覆盖全部 8 平台，包括平台特有配置字段
2. DingTalk 适配器实现 ActionCard 渲染路径，rendering profile 声明 card/update 能力
3. Bridge review 命令补齐 `/review deep`、`/review approve`、`/review request-changes`
4. 所有平台适配器统一降级上报 `X-IM-Downgrade-Reason`，delivery 记录持久化该字段
5. 前端 delivery history 增加降级诊断、payload 预览、重试
6. 提取 `PlatformBadge` / `EventBadgeList` 共享组件，IM 与 Settings/Notification 复用
7. 事件类型扩展（sprint/review/workflow 事件）

**Non-Goals:**

- 新增第 9 个平台适配器
- Bridge 多平台多实例部署拓扑变更
- 消息模板编辑器 UI（属于独立 feature）
- IM Bot 自然语言意图分类优化（intent handler 保持现状）

## Decisions

### D1: 前端平台配置差异化 — 条件字段方案

每个平台有不同的配置字段（如 WeCom 需要 corpId + agentId，QQ 需要 OneBot endpoint）。

**方案 A**：统一表单 + 条件渲染特定字段
**方案 B**：每平台独立配置组件

选择 **方案 A**。理由：当前 `IMChannelConfig` 已是统一表单，只需在 platform select 变化时渲染额外字段。8 个平台的差异字段不多（各 1-3 个额外字段），不值得拆分为独立组件。在 `im-store.ts` 中将 `IMChannel` 的配置字段改为 `Record<string, string>` 的 `platformConfig` map，前端按平台 key 渲染。

### D2: 降级上报 — Header 元数据 vs Delivery 字段

**方案 A**：仅通过 HTTP header `X-IM-Downgrade-Reason` 传递
**方案 B**：Header 传递 + delivery 记录持久化 `downgrade_reason` 字段

选择 **方案 B**。理由：降级信息需要在前端 delivery history 中长期可查，仅靠 header 无法持久化。后端在 `AckDelivery` 时将 bridge 上报的降级原因写入 delivery 记录。

### D3: Review 子命令 — Bridge 端本地处理 vs 透传后端

**方案 A**：Bridge 命令引擎本地解析子命令参数，直接调用后端 review API
**方案 B**：Bridge 仅做命令路由，参数解析交给后端

选择 **方案 A**。理由：与现有 `/task`、`/agent`、`/cost` 命令模式一致——Bridge 解析参数并调用 `AgentForgeClient` 对应方法。review.go 中已有基础结构，补齐子命令 handler 即可。

### D4: 共享组件提取位置

`PlatformBadge`（平台图标+名称）和 `EventBadgeList`（事件标签列表）放入 `components/shared/`。理由：这些组件被 IM 页面、Settings 通知配置、可能的 Webhook 日志多处使用，属于跨 feature 复用。

### D5: DingTalk ActionCard 实现路径

DingTalk 的 ActionCard 通过 Stream 模式的 card callback 接收用户操作。实现路径：
1. `platform/dingtalk/live.go` 增加 `SendActionCard()` 方法，使用 DingTalk OpenAPI 发送 ActionCard
2. rendering profile 声明 `card: true`、`card_update: false`（DingTalk 不支持 card 就地更新）
3. ActionCard callback 通过现有 `NormalizeAction()` 归一化为 `IMActionRequest`

## Risks / Trade-offs

- **[DingTalk ActionCard SDK 变更]** → DingTalk Stream SDK 的 card 接口可能在后续版本变化。Mitigation: 通过 rendering profile 的 card 能力声明做运行时检查，降级路径已有
- **[前端 platformConfig 自由字段]** → `Record<string, string>` 类型安全性弱于强类型。Mitigation: 前端按平台常量表约束可输入的 key，后端 SaveChannel 做字段校验
- **[事件类型扩展不兼容]** → 新增事件类型需要前后端同步。Mitigation: 事件列表由后端 `/im/event-types` 端点提供，前端动态加载而非硬编码
