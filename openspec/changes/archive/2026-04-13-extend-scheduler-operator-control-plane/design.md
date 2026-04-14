## Context

AgentForge 当前已经有一条真实的 scheduler backend 主线：Go 侧通过 `src-go/internal/scheduler/registry.go` 维护 built-in job catalog，通过 `service.go` / `repository/scheduler_repo.go` 记录 job 与 run history，通过 `handler/scheduler_handler.go` 和 `src-go/internal/server/routes.go` 暴露 `/api/v1/scheduler/stats`、`/jobs`、`/jobs/:jobKey`、`/jobs/:jobKey/runs`、`PUT /jobs/:jobKey`、`POST /jobs/:jobKey/trigger` 等控制面，同时前端 `lib/stores/scheduler-store.ts`、`app/(dashboard)/scheduler/page.tsx`、`components/scheduler/*` 已经消费这组 API 构成现有运维页。

问题不在于“还没有 scheduler”，而在于现有合同仍停留在最小实现：后端只支持 `enabled/schedule` 更新和手动触发；run history 只有按 jobKey 的简单列表；`SchedulerStats` 只统计 total/failed/active；job DTO 也没有显式 `controlState`、`supportedActions`、`schedule preview` 或配置元数据。与此同时，活跃 `enhance-frontend-panel` 的 `scheduler-control-panel` 已经要求 pause/resume/cancel、历史治理、更丰富指标、日历视图和更完整的 job 配置编辑。如果不先补齐 operator-facing backend，前端只能继续把“disabled 猜成 paused”“没有 cancel 也先画按钮”“cron preview 靠客户端自己算”这种隐式逻辑堆上去，最终会再次造成 spec、UI 与后端真相漂移。

## Goals / Non-Goals

**Goals:**
- 在不推翻现有 scheduler foundations 的前提下，把现有 built-in scheduler control plane 扩展为 operator-ready backend。
- 为已注册的 built-in jobs 增加显式 pause/resume、truthful run cancellation、丰富指标、过滤/清理历史与 upcoming schedule preview。
- 让 job 详情 API 能返回受控配置元数据、可执行动作与不支持原因，避免前端自行猜测能力边界。
- 保持 Go scheduler registry 仍是唯一事实源，并与 Bun scheduler adapter 的现有部署语义兼容。
- 为当前 `/scheduler` 页面与后续 panel 提供同一套后端合同，避免重复引入前端特判。

**Non-Goals:**
- 不把当前 built-in scheduler 扩成任意用户自定义 cron/job builder，也不为此新增自由创建 job 的通用 DSL。
- 不把 `enhance-frontend-panel` 中的 `workflow-visual-builder` 一类全新图编辑器需求混入这次 change。
- 不引入新的分布式 worker 总线、外部队列或跨进程强制 kill 机制；取消语义保持 cooperative / backend-owned。
- 不重做 Bun sidecar scheduler adapter；它继续只镜像 Go 权威 schedule，而不是持有独立调度真相。

## Decisions

### 1. 继续以 built-in catalog 为唯一 job 定义源，而不是补一个“创建任意 job”接口

当前 `background-scheduler-control-plane` 已经把系统任务定义为 Go 持有的 built-in catalog，`src-go/internal/scheduler/registry.go` 也是围绕稳定 `jobKey`、固定 `executionMode` 和受控 `config` 工作。为了让 operator surface 变完整，本次扩展的是“如何控制已存在的 built-in job”，不是把系统悄悄改造成通用 job builder。

实现上，后端会继续只返回已注册 jobs，但在 job 详情中增加：
- 显式 `controlState` / `supportedActions`
- 配置元数据与允许编辑字段
- upcoming schedule preview
- unsupported action / unsupported field 的真实原因

这样前端可以展示“这个 job 能暂停、不能取消、配置只允许改 cadence”之类的真实边界，而不是因为缺字段就默认一切都可编辑。

**备选方案：**直接新增 `POST /api/v1/scheduler/jobs` 支持任意创建。否决，因为这会突破现有 built-in scheduler 的 source-of-truth 边界，把 scope 从 operator control 扩成 generic workflow/job authoring。

### 2. 用显式 operator control endpoints 和 DTO 投影替代前端对 `enabled/lastRunStatus` 的猜测

当前前端只能用 `job.enabled` 和 `job.lastRunStatus` 猜“是否 paused”“是否可 run now”“是否正在运行”。本次会把这些推断收敛为后端投影：
- job DTO 增加 `controlState`（例如 `active` / `paused`）
- job DTO 增加 `activeRun` 摘要与 `supportedActions`
- 新增显式 control endpoints，例如 pause / resume / cancel / cleanup / preview / config-metadata 等 operator surfaces

底层存储可以继续复用现有 `enabled` 与 run history 字段，但 API 不再要求消费者自己组合这些语义。这样既能保持持久化模型稳定，也能让前端拿到一致的 operator contract。

**备选方案：**继续只保留 `PUT /jobs/:jobKey` 更新 `enabled/schedule`，由前端自行映射 pause/resume。否决，因为这会把后端能力边界藏进 UI 特判里，继续放大不同页面之间的漂移风险。

### 3. 运行中 job 的取消采用 cooperative cancellation，并通过 run lifecycle 明确暴露 `cancel_requested` / `cancelled`

现有 scheduler run 只有 `pending/running/succeeded/failed/skipped`。如果要支持 operator cancel，不能只在前端加一个按钮；后端必须给出真实状态机。设计上会扩展 `ScheduledJobRunStatus` 与 service/repository 逻辑，让支持取消的 handler 能通过 context 或 cancel hook 协作退出，并把结果持久化为 `cancel_requested` / `cancelled`。对于不支持取消的 job，API 必须明确返回拒绝原因，而不是假装“请求已接收”。

这保持了当前 scheduler 的 in-process / Go-owned 执行模型，也与 Bun adapter 兼容：Bun 只负责触发，不负责终止业务执行。

**备选方案：**实现一个前端 optimistic cancel，或者对所有 job 一律返回 accepted。否决，因为这会制造“按钮存在但系统其实不能停”的假象。

### 4. 历史治理、聚合指标与 schedule preview 一律由后端基于同一套 scheduler truth 计算

当前 run history 只有 `ListRuns(jobKey, limit)`，stats 也只有 total/failed/active。为了支撑 panel 与现有页面，本次把以下逻辑统一放到后端：
- run history 的 status/source/time-window 过滤
- retain-last-N / cleanup 历史治理
- success rate、average duration、paused count、queue depth 等聚合指标
- 基于与 registry 相同 cron parser 的 upcoming occurrences preview

这样 UI 不必自己解析 cron、拼 duration 或重新统计失败率，也能确保 dashboard/scheduler page/future panel 使用同一个数据口径。

**备选方案：**继续让前端本地做过滤、统计和 cron preview。否决，因为这会让不同 consumer 对同一个 job 产生不同答案。

### 5. job 配置编辑采用 schema-driven metadata，而不是开放 freeform JSON

`ScheduledJob.Config` 目前是字符串 JSON，但这不意味着所有 built-in job 都适合前端直接编辑原始 JSON。设计上会为每个 built-in job 提供受控的 config metadata（可编辑字段、默认值、展示标签、校验规则、只读说明），并由 Go backend 在更新时执行 per-job validation。这样 frontends 可以展示结构化配置 UI，同时 backend 继续持有最终校验与持久化真相。

**备选方案：**直接把原始 `config` 文本暴露给前端任意编辑。否决，因为这会让 built-in job 的内部配置耦合泄漏到 UI，并提升误配风险。

## Risks / Trade-offs

- [取消语义只对 cooperative handlers 生效] → 在 job metadata 中显式暴露 `supportsCancel`，对不支持取消的 built-ins 返回 truthful rejection。
- [扩展 DTO 后旧 consumer 漏跟进] → 保持现有字段兼容，新增字段只做 additive 扩展，并补 store/page contract 验证。
- [cron preview 与真实下一次执行漂移] → preview 统一复用 scheduler registry 当前使用的 cron parser 与时区策略，而不是再写一套前端计算。
- [历史清理误删活跃或关键 run] → cleanup 只允许删除 terminal runs，并提供 retain-last-N / before-time 边界保护。
- [scope 被前端 spec 带偏成 generic job creation] → 在 API 与 spec 里明确 built-in-only 边界，对 create-custom-job 诉求返回 unsupported 而不是暗中扩 scope。
- [第一版 cancellation 边界被误读] → 当前 change 没有额外标记为 intentionally non-cancellable 的 built-in jobs；如果未来某个 handler 不能可靠响应 context cancellation，必须通过 supported action metadata 显式禁用 cancel，而不是继续暴露乐观按钮。

## Migration Plan

1. 扩展 scheduler model / DTO / status 枚举与 repository 查询能力，先补 run filtering、cleanup、preview 与 richer stats 的后端骨架。
2. 在 `src-go/internal/scheduler/service.go` 增加 pause/resume/cancel 与 config-metadata / validation 流程，并把现有 built-in handlers 接到新的 capability contract 上。
3. 扩展 `handler/scheduler_handler.go` 和 `routes.go` 暴露 operator endpoints，同时保持现有 `/stats`、`/jobs`、`/jobs/:jobKey/runs` 路径兼容。
4. 更新 `lib/stores/scheduler-store.ts`、当前 `/scheduler` 页面与相关组件，改为消费显式 control state / supported actions / richer stats，而不是继续做本地猜测。
5. 做 focused verification：Go scheduler/service/repository/handler tests，必要的 WS event proof，以及 scheduler store/page contract tests。

回滚策略：
- 新增 endpoints 与字段保持 additive，回滚时可先让前端继续只使用旧字段和旧操作。
- 如果 cooperative cancellation 某个 built-in handler 不稳定，可临时将该 job 标记为 `supportsCancel=false`，不必整体回退 scheduler change。

## Open Questions

- 第一版哪些 built-in jobs 需要真正支持 cancellation：仅长时间运行的 `cost-reconcile` / `bridge-health-reconcile`，还是所有 handler 都要实现统一 cancel hook？
- config metadata 是直接走统一 JSON schema 风格，还是用更轻量的 field descriptor（type/label/default/helpText）更适合当前前端？
- history cleanup 是否只保留 per-job retain-last-N，还是需要项目/系统级批量清理入口？

