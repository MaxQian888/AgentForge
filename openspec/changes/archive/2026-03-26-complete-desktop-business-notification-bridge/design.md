## Context

这次调研确认，AgentForge 的 Tauri 基础壳能力已经不再是主要缺口: `src-tauri/src/lib.rs` 已经能监督 backend/bridge sidecar、暴露运行态、更新 tray、注册快捷键并触发 updater 检查，`lib/platform-runtime.ts` 和 `hooks/use-platform-capability.ts` 也已经形成统一前端入口。当前真正断开的，是“现有业务通知真相”到“桌面原生通知增强层”的最后一段桥。

当前 repo-truth 缺口主要集中在以下位置:

- `components/layout/dashboard-shell.tsx` 只负责认证后拉取通知并连接 WS，没有桌面通知协调器来观察标准化通知流并决定是否触发原生通知。
- `lib/stores/notification-store.ts` 只做 hydration、追加和已读计数，没有为桌面桥接暴露去重、变更观察或投递结果上下文。
- `lib/stores/ws-store.ts` 会把实时 `notification` 事件写入同一个 store，但当前没有任何桌面层逻辑来处理 fetch + WS 双路进入导致的重复原生投递风险。
- `lib/platform-runtime.ts` 的 `sendNotification(...)` 仍只接受 `{ title, body }`，丢失通知 ID、类型、`href`、创建时间和投递策略，也没有 tray unread 同步入口。
- `src-tauri/src/lib.rs::send_notification(...)` 只发出通用 `notification.sent` 事件; 它既不知道业务通知标识，也不会区分 delivered / suppressed / failed 这类结果。
- `components/layout/header.tsx` 只在页头展示未读数，没有把 unread 变化同步到桌面 tray。

基于这些现状，这次 change 的目标不是再补一轮泛化 Tauri 骨架，而是把已有 notification API、WebSocket 与前端 store 正式接到桌面增强层，同时保持“后端与共享 store 才是业务真相，Tauri 只是增强层”这条架构边界。

另外，这轮审计也确认了几项仍缺但不适合并入本 change 的桌面能力: 插件生命周期事件桥、窗口/菜单原生控制、桌面通知交互回调。这些会作为后续 backlog 保持可见，但不在本次内扩成宽范围重构。

## Goals / Non-Goals

**Goals:**

- 在认证后的 dashboard 壳层引入统一的桌面通知协调入口，把现有通知 hydration 与实时 `notification` 事件接入原生通知桥。
- 升级共享平台能力中的通知调用合同，支持结构化业务通知 payload，而不是继续依赖手动 title/body 调用。
- 为桌面通知补齐去重、前台抑制、托盘未读摘要同步和 delivery outcome 事件，确保桌面增强层具备稳定诊断面。
- 保持后端通知 API、`useNotificationStore` 和现有 mark-read 行为继续作为唯一业务真相来源，不把 Tauri 变成第二条通知链路。

**Non-Goals:**

- 不在本次内补齐插件生命周期事件桥或 sidecar stdout 业务事件解析。
- 不在本次内扩展窗口控制、多窗口编排、原生菜单系统或其他非通知类桌面能力。
- 不在本次内引入桌面通知点击动作回调、原生 deep-link 跳转或桌面端直接 mark-read 的新语义。
- 不改变后端通知模型的核心资源边界; 若需增加字段，仅允许向后兼容的上下文补充。

## Decisions

### 1. 在 `DashboardShell` 级别引入桌面通知协调器，而不是让页面或 Rust 直接消费后端通知

桌面通知桥应该挂在认证后的壳层，而不是放在单个页面里，也不是让 Rust 直接去拉后端通知。协调器只消费 `useNotificationStore` 中已经标准化过的通知记录，并在桌面模式下决定是否调用共享平台能力。

这样做的原因:

- `DashboardShell` 已经是认证、WS 连接和通知 hydration 的统一入口，最适合承载“一次挂载、全局生效”的桌面通知桥。
- 复用标准化 store 可以保留 `href`、`type`、未读状态等现有业务上下文，避免在 Rust 侧重复实现 API 调用和 payload 解析。
- 页面级处理会造成重复逻辑和重复弹窗，而 Rust 直连后端会把认证和去重逻辑分叉成第二套。

备选方案:

- 让 `Header` 或插件页自己调用 `sendNotification(...)`。缺点是通知语义会散落到页面层，难以避免重复原生投递。
- 让 Rust 直接轮询或订阅通知。缺点是要重复认证、数据归一化和前台可见性判断。

### 2. 将桌面通知 payload 升级为结构化业务通知合同

共享平台能力里的通知入口应该从 `{ title, body }` 升级为至少包含 `notificationId`、`type`、`title`、`body`、`href`、`createdAt` 和 `deliveryPolicy` 的结构化 payload。Rust 侧只负责按这个 payload 触发原生通知和桌面结果事件，不负责修改业务已读状态。

这样做的原因:

- 现有 `title/body` 形式无法做稳定去重，也无法把桌面结果事件与 in-app 通知记录关联起来。
- `href` 和 `type` 已经是现有通知体系的一部分，保留下来才能让 tray 摘要、未来 drill-down 和审计结果保持一致。
- 结构化 payload 能让 delivery outcome 事件在失败、抑制或成功时都带回足够上下文。

备选方案:

- 继续沿用自由文本通知接口。缺点是只能满足“手动测试发一条 toast”，无法支撑真实业务通知桥。
- 让 Rust 自己推断 payload。缺点是会把现有前端标准化逻辑复制一份。

### 3. 去重和前台抑制放在前端协调器，而不是后端 sent 标记或 Tauri 插件内部

桌面通知协调器维护一份 session 级 delivery ledger，以业务通知 ID 为主键记录最近的投递结果和时间。初始 hydration、后续 websocket replay 和重复 store 更新都要先过这层判定，再决定 delivered、suppressed 或 failed。

这样做的原因:

- 当前通知可能通过 fetch 和 WS 双路进入，只有在最靠近共享 store 的位置做 ID 级判定，才能稳定避免双弹窗。
- 前台可见性、当前窗口聚焦与是否需要抑制，都是前端壳层最容易判断的上下文。
- 把 sent 标记下沉到后端会把桌面会话生命周期和业务通知状态耦合过深。

备选方案:

- 后端记录桌面端 sent 状态。缺点是不同桌面会话、前台抑制与失败重试很难建模，且会污染业务真相。
- 只依赖 Tauri 自身是否成功 show notification。缺点是无法覆盖 fetch + WS 重放这种业务级重复来源。

### 4. Tray 摘要和 delivery outcome 事件都沿用现有平台能力与桌面事件桥

本次不新建第二套桌面状态总线。协调器仍通过 `usePlatformCapability()` 调 `sendNotification` 与 `updateTray`，Rust 侧继续通过 `agentforge://desktop-event` 回传结果，但事件类型需要收敛为 `notification.delivered`、`notification.suppressed`、`notification.failed` 这类规范化结果，并附带通知 ID 与摘要上下文。

这样做的原因:

- 当前 repo 已经有统一平台 facade 和桌面事件桥，继续复用能避免把桌面通知做成孤立旁路。
- tray 未读摘要属于桌面增强反馈，最适合沿用现有 `update_tray` 接缝，而不是额外引入 tray 专属状态管理。
- outcome 事件能让前端明确区分“没尝试发送”“被抑制”“发送失败”，便于后续观察与测试。

备选方案:

- 在通知桥里静默吞掉结果。缺点是页面和测试无法分辨投递行为是否发生。
- 直接让协调器写本地状态而不发桌面事件。缺点是与现有 desktop event bridge 模型不一致。

## Risks / Trade-offs

- [Hydration 与 WS replay 仍可能造成重复原生投递] -> 用通知 ID 作为 ledger 主键，并在协调器里统一处理 fetch/append 两类入口。
- [前台抑制策略过强可能让用户误以为通知丢失] -> 抑制时同步 tray 摘要并发出 `notification.suppressed` 结果事件，保持 in-app 通知中心不变。
- [结构化 payload 升级可能影响现有手动通知调用点] -> 在 facade 层保留兼容包装，先让旧调用能映射到新 payload，再逐步收敛到业务通知入口。
- [Tray 更新过于频繁会造成噪音] -> 只在未读数或最近高优先级摘要发生变化时更新 tray，而不是对每次 store 变动都强制刷新。
- [这次仍未覆盖插件事件桥等其他桌面缺口] -> 在 proposal/design 中明确把这些缺口列为后续 backlog，保持本 change focused。

## Migration Plan

1. 定义结构化桌面通知 payload 和 delivery outcome 事件合同，并同步修改相关 desktop specs。
2. 在前端认证壳层引入单一桌面通知协调器，消费 `useNotificationStore` 和 WS 归一化后的通知流。
3. 扩展 `lib/platform-runtime.ts`、`hooks/use-platform-capability.ts` 和 `src-tauri/src/lib.rs`，让通知与 tray 同步走统一平台能力入口。
4. 让 header unread 变化与通知桥协同驱动 tray 摘要，同时保留现有 in-app 通知中心和 mark-read 路径。
5. 通过 store、shell、desktop-event 和 tray 相关测试验证 hydration、WS replay、前台抑制、失败降级和非桌面 fallback。

回滚策略:

- 如果结构化 payload 扩展在前后端接缝上出现阻塞，可先保留旧手动通知接口兼容层，不回退 dashboard 壳层协调器。
- 如果桌面结果事件在第一轮过于噪声，可先保留 failed/suppressed 两类关键结果，再补充更细粒度 delivered 元数据。
- 如果 tray 摘要策略不稳定，可暂时退回仅同步 unreadCount，但不回退 notification bridge 和去重逻辑。

## Open Questions

- 第一阶段哪些通知类型默认进入原生桌面通知: 全量未读通知，还是仅高信号类型如任务告警、评审完成、调度异常?
- tray tooltip 第一阶段是否只显示未读数，还是要附带最近一条高优先级通知标题?
- 结构化 payload 是否需要立即为未来通知交互回调预留 `actions` 或 `actionable` 字段，还是先保持最小合同?
