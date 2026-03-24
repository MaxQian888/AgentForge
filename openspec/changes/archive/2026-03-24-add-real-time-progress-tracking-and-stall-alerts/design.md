## Context

当前仓库已经具备任务状态机、`TaskService` 里的状态流转广播、`NotificationService` 的站内通知能力、Go WebSocket Hub，以及 `src-im-bridge/notify/receiver.go` 的 IM 通知入口，但这些能力仍是割裂的。`src-go/internal/model/task.go` 只保存 workflow `status` 与通用时间戳，没有“最近有效活动”“是否停滞”“为何预警”这类进度信号；前端 `lib/stores/ws-store.ts` 目前也只消费很少几类事件，无法把任务进度风险稳定地同步到任务视图或 Dashboard。

PRD 已经把“实时状态流转，自动检测停滞并预警”列为 P0，同时现有的 `improve-dashboard-data-and-team-management` change 也依赖任务风险信号来丰富首页洞察。因此这次设计需要先定义一条仓库真相一致的任务进度链路：任务域如何产生活动信号、何时判断任务停滞、如何去重告警，以及如何把这些信号交给 Web、站内通知和 IM。

## Goals / Non-Goals

**Goals:**
- 在不改变现有任务 workflow 基础语义的前提下，为任务增加独立的进度信号模型，表达最近活动、风险等级、停滞状态和恢复状态。
- 定义事件驱动 + 定时检查结合的停滞检测方案，使人工任务、Agent 任务和 Review 等待态都能被统一评估。
- 让任务列表/详情、Dashboard 消费方、站内通知和 IM 通知共享同一套进度风险判断，而不是各自猜测。
- 为后续实现锁定告警去重、恢复通知和降级策略，避免“定时扫描 = 无限刷屏”。

**Non-Goals:**
- 不在本次 change 中实现新的任务视图布局、Sprint 燃尽图或完整绩效评分体系。
- 不扩展项目级权限矩阵、关注者系统或复杂的多级升级审批链。
- 不把所有领域事件都并入统一活动时间线，只处理任务进度判断所需的最小活动来源。
- 不要求本次 change 同时交付新的 Dashboard 页面结构；Dashboard 只作为本 change 的消费方，而不是设计中心。

## Decisions

### Decision: 将任务 workflow 状态与进度健康状态分离，新增任务进度信号持久层

现有 `tasks.status` 只适合表达业务流转节点，例如 `assigned`、`in_progress`、`in_review`，但无法表达“任务仍在 in_progress，却已经 3 小时没有任何活动”的运营问题。本次设计采用独立的任务进度信号记录来保存 `last_activity_at`、`last_transition_at`、当前风险等级、停滞开始时间、最近触发原因和最近告警状态，并在 task list/detail DTO 中回填这些字段。

这样可以保持现有 workflow 规则和 `ValidateTransition` 不变，同时允许后端和前端把“状态”与“健康度”分层处理。终态任务在关闭时同步关闭进度信号，开放态任务继续被检测。

备选方案是直接复用 `tasks.updated_at` 或把停滞字段塞进 `tasks` 主表。前者会把标题编辑、预算更新等非进度事件误判为“任务有进展”，后者会让任务主模型混入大量派生运营字段，后续更难维护，因此不采用。

### Decision: 使用“事件驱动刷新 + 周期性超时评估”的混合检测模型

任务创建、指派、状态流转、Agent 输出/状态变化、Review 结果等事件会立即刷新任务的最近活动时间和活动来源；同时，后台增加一个周期性检测器，按配置阈值扫描非终态任务，判断哪些任务已进入风险窗口或停滞窗口。只有当信号状态真正变化时，系统才会持久化新状态并触发后续告警。

这种混合模式能兼顾实时性和时间阈值判断。事件驱动负责快速更新，周期检测负责回答“已经沉默多久了”。对 `blocked`、`cancelled`、`done` 等状态则采用豁免或终止策略，避免把本来就不该推进的任务误报为停滞。

备选方案一是纯定时扫描，只看数据库时间戳；这会忽略流式 Agent 活动和 review 流转等高价值信号。备选方案二是纯事件驱动、不做定时检查；这样无法自然识别“长时间没有新事件”的停滞，因此不采用。

### Decision: 复用现有 NotificationService 和 IM Receiver，但以“信号变化”驱动去重与升级

仓库已经有 `NotificationService`、`notifications` 表和 `src-im-bridge/notify/receiver.go`，因此本次不新建平行告警系统，而是在这些既有通道之上定义进度预警类型，例如停滞预警、临近停滞提醒和恢复通知。告警只在信号发生变化、严重级别提升，或跨过冷却窗口时重新发送；重复扫描看到相同停滞状态时，只更新时间戳而不重复刷通知。

预警 fan-out 的最小集合是项目级 WebSocket 事件和站内通知；如果任务能映射到可路由的 IM 目标，再追加 IM 投递。IM 发送失败不会回滚站内通知或进度状态，只会记录为降级结果。

备选方案是单独再建一个告警投递服务或把 IM 作为主通道。前者会重复已有通知基础设施，后者则会让 IM 路由缺失时整条链路不可用，因此不采用。

### Decision: 先扩展既有 task DTO 与实时事件，再让 Dashboard 等消费方复用这些字段

当前前端已有 `task-store`、`ws-store` 和通知 store，因此本次优先扩展 task API 返回结构和 WebSocket 事件，使任务列表、详情和 Dashboard 消费方都能读取统一的进度字段与告警原因。除了现有 `task.transitioned` / `task.assigned` 等事件外，新增面向进度信号的实时事件，例如任务进入风险、进入停滞、从停滞恢复。

这样可以让不同页面共享同一份任务健康数据，而不需要再引入只服务于某个页面的专用接口。后续 `improve-dashboard-data-and-team-management` 只需要消费这些信号即可，而不必重新定义停滞判断。

备选方案是仅通过新的 dashboard summary 聚合接口暴露风险数据。那会让任务页和通知链路继续依赖另一套判断逻辑，也会增加后续 change 之间的耦合，因此不采用。

## Risks / Trade-offs

- [进度信号与真实任务活动产生漂移] → 所有刷新入口统一收敛到一个 progress projector/manager，避免任务服务、Agent 服务、Review 服务各自计算。
- [周期检测导致通知刷屏] → 用“信号变化才告警 + 冷却窗口 + 恢复后才能重新告警”的策略限制 fan-out。
- [历史任务缺少足够活动数据，首次上线时误报] → 初始回填只使用当前状态和已有时间戳生成保守默认值，对缺少证据的任务标记为未知来源，而不是直接判定异常。
- [IM 路由信息不完整] → 站内通知和 WebSocket 是必达主通道，IM 为可选增强；发送失败只记录日志/状态，不中断主流程。
- [前端事件接入不完整导致页面显示不一致] → 规范 task DTO 与新增事件 payload 使用同一 progress shape，并为 `ws-store` 与任务页面补 focused verification。

## Migration Plan

1. 新增任务进度信号持久层与后端模型，基于现有 task 记录为开放态任务做一次保守回填。
2. 在 Go 后端接入 progress projector 和周期检测器，先打通 task/agent/review 三类活动源与状态变更事件。
3. 扩展 task API DTO、WebSocket 事件和通知类型，让前端任务 store、通知 store 和 Dashboard 消费方都能读取统一字段。
4. 开启 IM 投递增强，仅在存在可路由目标时发送 progress alerts，并验证失败降级路径。
5. 如果发布后信号质量或噪音不可接受，可先关闭周期检测器和 IM fan-out，保留持久层与基础字段，不影响既有任务 CRUD 和状态流转。

## Open Questions

- 默认停滞阈值是否按 `status + assignee_type` 双维度配置，还是第一版只做全局默认值？
- 首版预警接收人是否只覆盖 assignee / reporter，还是需要补一个项目负责人映射规则？
- `blocked` 状态是完全豁免停滞检测，还是在长时间未解除阻塞时也需要二级提醒？
