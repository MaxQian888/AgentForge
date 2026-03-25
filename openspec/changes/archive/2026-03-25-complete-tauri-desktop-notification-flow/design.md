## Context

AgentForge 已经具备完整的业务通知主链路，但这条链路目前只停在 Web/in-app 层：

- Go 后端有持久化通知模型、通知 API 和 WebSocket 广播，`NotificationService.Create(...)` 会写库并发出 `notification` 事件。
- 前端 `useNotificationStore` 会拉取 `/api/v1/notifications` 并把 `data.href` 归一化成可导航上下文；`useWSStore` 会把实时 `notification` 事件写回同一个 store。
- `DashboardShell` 负责在认证后连接 WS 并拉通知，`Header` 负责展示 in-app 通知中心。
- Tauri 侧虽然已经有 `send_notification` 和 `update_tray`，但它们现在只是手动能力，业务通知并不会自动进入原生通知或托盘摘要。

这说明当前缺的不是“通知数据”或“桌面通知插件接入”，而是缺一个 repo-truthful 的桌面通知桥接层，把现有通知真相安全地映射到桌面增强能力，而不重新发明第二条通知通道。

另一个关键约束来自官方 Tauri notification plugin 文档：通知 action / interaction callback 在官方能力里是 mobile-only，桌面端不能把“点击原生通知后可靠回跳应用”当作第一阶段契约。因此这次设计必须把重点放在可实现的桌面通知投递、去重、前台抑制、托盘摘要和 in-app 上下文保留，而不是承诺桌面端原生点击路由。

## Goals / Non-Goals

**Goals:**

- 把 AgentForge 已有的持久化通知和实时通知接入 Tauri 原生系统通知，而不引入第二条后端通知真相来源。
- 在桌面模式下建立统一的通知投递协调层，覆盖 hydration、WebSocket 实时流、重复投递保护、前台抑制和托盘未读摘要。
- 保持共享平台能力 facade 仍是前端唯一桌面通知入口，避免页面直接依赖 Tauri 原始 notification API。
- 为桌面通知建立规范化的 delivery outcome 事件，让前端能观察已投递、被抑制和失败结果，而不影响现有业务通知 API/WS。
- 保留通知的 `href`/上下文元数据，确保 native toast 之外的 in-app 通知中心仍然是可导航、可标记已读的正式交互面。

**Non-Goals:**

- 不在本次内定义桌面端原生通知点击回调或操作按钮合同，因为当前官方通知交互能力并不覆盖桌面通用场景。
- 不新增独立的通知后端、轮询服务或 Tauri 直接访问后端通知 API 的第二条认证路径。
- 不改变现有通知持久化 schema、后端通知 API 的基本资源模型，除非为了统一上下文字段做向后兼容的增量补充。
- 不把所有通知消费页面都改造成桌面专属 UI；第一波以 `DashboardShell` 级桥接和现有 header/in-app 通知中心协同为主。

## Decisions

### 1. 桌面通知协调器放在前端认证壳层，而不是让 Rust 直接拉后端通知

设计上新增一个 `DashboardShell` 级的桌面通知协调器或 hook，由它观察 `useNotificationStore` 的标准化结果，并在桌面模式下决定是否调用共享平台能力发送原生通知。

这样做的原因：

- 认证、通知 hydration、实时写入和 `href` 归一化已经都在前端完成；若让 Rust 直接访问通知 API，会重复认证状态、目标过滤和数据规范化逻辑。
- `DashboardShell` 已经是连接 WS 与拉通知的唯一壳层，最适合承载“一次挂载、全局生效”的桌面桥接。
- 可以让桌面通知继续遵守“后端 API/WS 是业务真相，Tauri 只是增强层”的既有设计。

备选方案：

- 让 Tauri/Rust 轮询或直接订阅后端通知。缺点是需要在桌面壳重复实现认证、去重和 payload 解析，并把通知真相拆成两处。
- 让每个页面自己在需要时调用 `sendNotification`。缺点是会快速形成重复逻辑和多次弹窗。

### 2. 桌面通知使用结构化投递 payload，但导航与已读仍走现有 in-app 路径

共享平台能力层应从“只接收 title/body”升级为能接收结构化通知 payload，例如：

- `notificationId`
- `type`
- `title`
- `body`
- `href`
- `createdAt`
- `deliveryPolicy`（如默认、允许前台抑制）

但第一阶段不要求桌面原生通知成为可点击导航入口。`href` 的职责是保存在通知上下文中，供 in-app 通知中心、任务告警 rail 或后续平台能力继续复用；已读同步也仍通过现有 `markRead` API 路径完成，而不是由 Tauri 侧隐式改变后端状态。

这样做的原因：

- 与当前 `notification-store` 的数据形状一致，避免再造一套桌面专属 notification DTO。
- 保证桌面投递失败时，通知仍能在 header / dashboard 内按现有方式被查看和处理。
- 避免承诺当前桌面平台并不稳定支持的 notification action/click 语义。

备选方案：

- 继续只传 `title/body`。缺点是无法做去重、类型过滤和托盘摘要，也丢失与 in-app 通知的关联。
- 让 Tauri 自动 mark-read。缺点是用户只是收到原生 toast，并不代表真的完成业务处理。

### 3. 去重与前台抑制在前端桥接层实现，键值以通知 ID 为主

桌面通知协调器应维护一份短生命周期的 delivery ledger，至少记录：

- 已尝试投递的通知 ID
- 最后一次投递结果（delivered / suppressed / failed）
- 最近一次投递时间

桥接策略采用：

1. 初始 hydration 的未读通知只对“此前未投递”的记录尝试原生投递。
2. WebSocket replay 或重复 payload 以通知 ID 去重，避免 fetch + WS 造成双弹窗。
3. 当前桌面窗口可见且通知策略允许抑制时，不再弹原生 toast，而是只更新 in-app 未读和 tray 摘要，并发出 `notification.suppressed` 结果事件。

这样做的原因：

- 当前 repo 的通知真相已经可能通过 API 和 WS 双路进入前端，不做 ID 级去重会天然双发。
- 前台可见时继续弹系统通知，容易让桌面模式比 Web 模式更吵。
- 去重 ledger 放在前端最容易拿到“当前窗口是否前台”“通知是否已存在于 store”这类判据。

备选方案：

- 在 Go 后端强制做桌面专属 sent 标记。缺点是会把桌面 UI 生命周期和后端通知状态耦合过紧。
- 只靠 Tauri `active()` 或系统通知插件状态去重。缺点是与业务 notification ID 脱节，且不能覆盖 hydration replay。

### 4. Tray 摘要与桌面事件桥接沿用现有能力，而不是新增第二套桌面状态总线

桌面通知桥接应复用现有 `update_tray` 和 `agentforge://desktop-event` 两条已存在的桌面接缝：

- unreadCount 或高优先级告警变化时，协调器统一更新 tray title/tooltip/visible 状态。
- 每次桌面通知投递结果都通过规范化 desktop event 回传前端，事件类型至少包括 `notification.delivered`、`notification.suppressed`、`notification.failed`。

这样做的原因：

- 当前 repo 已经有 tray 与 desktop event bridge，不需要再发明新的桌面状态订阅机制。
- outcome 事件有利于插件页或未来设置页观察桌面通知健康度，但不会替代后端 `notification` 业务事件。

备选方案：

- 在 notification store 内静默处理，不输出桌面结果事件。缺点是调试困难，也无法区分“没收到通知”和“通知被抑制”。

## Risks / Trade-offs

- [前端桥接层可能在 hydration 与 WS 重放之间误判重复] -> 用通知 ID 作为主键，并把最近投递结果保存在协调器 ledger 中，而不是只按时间窗口猜测。
- [前台抑制策略过强会让用户以为桌面通知丢失] -> 抑制时同步更新 tray 摘要，并通过 `notification.suppressed` outcome 让 UI 可诊断。
- [桌面通知 payload 结构扩展后，页面可能绕过 facade 直接调用旧接口] -> 在 spec 中明确共享平台能力是唯一入口，并把桥接放在壳层而不是页面层。
- [官方桌面通知交互能力有限，产品容易继续要求“点击就跳转”] -> 在契约里明确第一阶段只保留 `href` 上下文，不承诺桌面端原生点击路由。
- [Tray 过度依赖 unreadCount 可能对高优先级告警表达不足] -> 第一阶段先保证未读数量和最近严重级别摘要可见，复杂分组/优先级图标留到后续 change。

## Migration Plan

1. 在 proposal/spec 基础上定义结构化桌面通知 payload、delivery outcome event 和桥接策略。
2. 在前端壳层引入单一桌面通知协调器，接入 `notification-store`、`ws-store` 和 `usePlatformCapability()`。
3. 扩展 Tauri 通知命令与桌面事件发射，使其接受结构化 payload 并返回 delivery outcome 所需的最小信息。
4. 把 unreadCount 与最近通知摘要接入现有 tray 更新路径，保证抑制和失败场景也有可见反馈。
5. 通过 store / hook / desktop-event / tray 相关测试验证 fetch、WS、重复 payload、桌面不可用和前台抑制边界。

回滚策略：

- 如果结构化 payload 落地受阻，可以先保留 in-app 通知真相与 tray 摘要，只暂停 native toast 投递，而不回退通知 store / WS 主链路。
- 如果桌面 outcome event 过于噪声，可先收窄为失败事件与 tray 更新，但不回退去重策略和共享 facade 入口。

## Open Questions

- 第一阶段哪些通知类型默认进入原生桌面通知：全部未读通知，还是只覆盖任务进度告警、评审完成、派单阻塞等高信号事件？
- tray 摘要是否只展示未读数量，还是要在 tooltip 中附带最近一条高优先级通知标题？
- 是否需要为未来支持桌面 notification interaction 的平台预留 `actionable` 元数据字段，但在当前阶段保持 no-op？
