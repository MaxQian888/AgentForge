## Context

当前仓库已经出现了多种“事实上属于定时任务”的后台行为，但它们分散在不同地方，且缺少统一控制面：

- `src-go/cmd/server/main.go` 通过 `time.NewTicker` 周期执行 `runTaskProgressDetector(...)`。
- Go 启动时会做一次 worktree startup sweep，但没有持续性的 GC 调度、运行历史和失败恢复。
- PRD 明确把“任务调度”列为 Go Orchestrator 的核心职责，并列出了进度催促、定时报告、健康检查、worktree cron 清理、5 分钟快照/成本校准等周期任务。
- 插件系统设计文档已经把 Go 定位为业务与状态权威，把 TS Bridge 定位为 Bun compile 打包的执行侧；这意味着真正的调度事实源不能漂到前端或某个散落的 Bun loop 里。

同时，Bun 在 2026-03-18 发布的 v1.3.11 引入了 `Bun.cron`，可以跨平台注册 OS 级 cron/Task Scheduler/launchd 任务。这给 AgentForge 的桌面/本地模式提供了一个新的真实机会：在 Tauri/Bun sidecar 场景下补齐“应用不常驻时的定时触发能力”，而不必为此新引入另一套平台特化脚本。

## Goals / Non-Goals

**Goals:**
- 为 AgentForge 引入统一的后台调度控制面，管理内建定时任务的定义、启停、执行、失败、历史和人工触发。
- 将现有零散后台 loop 收敛为可观察、可配置、可审计的调度任务，首批覆盖任务进度 detector 和 worktree stale-state GC。
- 为调度能力补齐服务端 API、前端管理入口和实时事件，避免后台任务默默失效。
- 在桌面/本地部署模式下利用 `Bun.cron` 提供跨平台 OS 级触发，但保持 Go 仍是调度状态与业务效果的权威。
- 为后续扩展 Bridge 健康巡检、成本校准、报告生成和插件/工作流激活任务提供统一插槽。

**Non-Goals:**
- 不在本次 change 中实现完整的分布式多节点 scheduler、Redis Streams job bus 或任意用户自定义 cron 编辑器。
- 不在本次 change 中交付 PRD 里所有长期数据维护任务，例如尚未落地的数据分区维护、归档存储清理、全量审计报表生成。
- 不把 TS Bridge 变成调度权威；Bridge 只负责本地/桌面触发适配和回调执行。
- 不在本次 change 中引入 Temporal、外部 job worker 集群或独立的消息队列编排系统。

## Decisions

### Decision: Go scheduler registry is the source of truth; Bun.cron is only a deployment adapter

调度任务的定义、启停状态、下一次执行时间、最近一次运行结果和人工触发入口全部由 Go 持有。桌面/本地模式下，如果某个任务被标记为需要 OS-level persistence，则由 Bun sidecar 使用 `Bun.cron` 向当前平台注册任务，并在触发时回调 Go 的内部 scheduler API 执行实际业务逻辑。

这样做的原因是 PRD 和插件系统设计都已经把 Go 设定为业务状态的 single source of truth。`Bun.cron` 非常适合跨平台注册 cron/Task Scheduler/launchd，但它不应直接拥有任务结果和业务副作用；否则 Web 模式、Docker 模式和桌面模式会分叉成两套调度真相。

备选方案一是完全在 Go 内继续用 `ticker`/`gocron` 做所有模式下的调度。这样虽然简单，但无法解决桌面模式下应用不常驻时的 OS-level 调度需求。备选方案二是让 Bun 直接管理调度与运行历史。这样会把任务权威从 Go 拆走，不符合现有架构，因此不采用。

### Decision: Model scheduler state explicitly with job definitions and append-only run history

调度控制面引入两层持久化模型：

- `scheduled_jobs`: 每个内建任务一条权威定义，包含 `job_key`、作用域、enabled、schedule、execution_mode、overlap_policy、last_run_status`、`last_run_at`、`next_run_at` 和配置 JSON。
- `scheduled_job_runs`: 每次触发一条追加式运行记录，保存 trigger source（cron/manual/recovery）、开始/结束时间、摘要结果、错误信息、受影响对象统计和相关 cursor/lease 信息。

这样既能支撑 UI/运维查看“当前配置长什么样”，也能回答“最近到底跑了什么、失败在哪里、是否重复失败”。已有的 `task_progress_snapshots`、worktree inspection 结果等业务表继续保存业务事实；scheduler 表只保存调度事实。

备选方案是只保留当前内存状态，不落库。这样会让桌面重启、服务重启和失败排障都失去依据。另一个备选方案是只存最新状态，不保留运行历史；这会削弱可观测性和问题追踪，因此不采用。

### Decision: Built-in jobs share one execution contract with singleton overlap semantics

首批内建任务统一实现一个 job handler 契约，例如：

- `task-progress-detector`
- `worktree-garbage-collector`
- `bridge-health-reconcile`
- `cost-reconcile`
- `scheduler-history-retention`

每个 handler 只负责自己的业务执行与结果摘要；调度框架负责：

- 根据 schedule 决定何时触发
- 保证同一个 `job_key + scope` 同时最多一个运行实例
- 记录 run history
- 发送 realtime / notification / log 事件
- 处理手动触发与失败重试节流

第一版采用 `singleton + skip-overlap` 语义，而不是排队并行。对任务进度 detector、worktree GC、健康巡检这类后台任务来说，跳过重叠比积压多个旧 run 更安全，也更容易理解 UI 状态。

备选方案是为 scheduler 自己再引入一套队列与 worker 消费模型。当前仓库还没有成熟的后台 job 基础设施，这会把范围迅速扩大到重试、租约、死信和消费顺序问题，因此不采用。

### Decision: Surface scheduler operations through first-class API/UI and realtime events

调度器不能继续藏在 `main.go` 里。系统会新增调度相关 API 与界面，至少提供：

- 列出内建任务、启停状态、最近/下次执行时间、最后结果摘要
- 查看运行历史与失败详情
- 手动触发一次任务
- 更新允许配置的 cadence / enable 状态
- 订阅 scheduler lifecycle / run result 事件

这部分界面不依赖 Dashboard headline cards，而是走独立的运维入口；Dashboard 只消费关键风险信号，而不承担全部调度管理职责。

备选方案是仅靠日志或管理命令行。这会让前端操作者无法理解后台 job 是否在正常工作，也违背了仓库正在建设的统一管理面方向，因此不采用。

### Decision: Scheduler rollout is incremental and repo-truthful, not “implement every cron idea in the PRD”

本次 change 的目标是先补齐“统一 scheduler 平台 + 当前已存在或即将依赖的核心 built-ins”，而不是一次性实现 PRD 里所有长期 cron 场景。优先级按当前仓库真实接缝排序：

1. `task-progress-detector` 从隐式 ticker 迁移为注册任务
2. `worktree-garbage-collector` 从启动时一次性 sweep 升级为周期任务 + 人工补救触发
3. `bridge-health-reconcile` 补齐健康检查/恢复巡检的统一入口
4. `cost-reconcile` 为预算最终一致性提供周期校准 hook

像分区维护、冷数据保留等依赖未来存储演进的任务保留为扩展位，而不是在这次 change 里伪实现。

## Risks / Trade-offs

- [桌面模式的 Bun.cron 与 Go 内部 schedule 发生漂移] → 由 Go 保存权威 schedule，Bun 只做注册镜像；每次启动和配置变更都做 reconcile。
- [后台任务失败后无人注意] → 所有 run 都写历史、更新 last status，并通过 WS/通知暴露失败摘要。
- [把现有 ticker 迁移到 scheduler 后引入行为回归] → 保持原始阈值与业务语义不变，先做“接管执行器”，再扩展配置/UI。
- [未来多实例部署时单实例 scheduler 不够用] → 数据模型和触发接口预留 lease/leader 字段，但本次不提前引入完整分布式复杂度。
- [scope 膨胀成通用工作流引擎] → 任务边界固定为 built-in system jobs，不开放任意用户 DSL 或 Workflow Plugin 执行器。

## Migration Plan

1. 引入 scheduler domain model、registry 和 run history 持久层，但先以只读内建 catalog 启动。
2. 将 `task-progress-detector` 接入新调度器，验证其 cadence、dedupe 和 alert 行为与现有逻辑一致。
3. 将 worktree startup sweep 抽象为 `worktree-garbage-collector`，保留启动时快速修复，同时增加周期 run 和手动触发入口。
4. 新增 scheduler API、WebSocket 事件和前端管理界面，让任务状态可见。
5. 在桌面/本地模式加入 Bun.cron 注册/清理适配层，并通过 reconcile 确保 OS-level 任务与 Go 权威配置一致。
6. 逐步把 Bridge 健康巡检和成本校准迁入统一 scheduler catalog。

回滚策略：
- 可以先关闭 scheduler enable 开关，退回到“无周期执行，仅保留手动触发和历史查看”。
- 对已迁移的任务可以临时恢复旧 loop，但只应作为短期应急方案。

## Open Questions

- 调度管理入口应该落在独立的 `/settings/scheduler`，还是先挂到现有 dashboard/settings 运维面板下？
- `bridge-health-reconcile` 的首版是否只做健康探测与事件告警，还是直接负责触发恢复动作？
- Bun.cron 回调在桌面模式下是直接唤起 Go sidecar，还是通过一个极简 bridge helper 进行转发更稳妥？
- 是否需要在第一版就暴露 per-project schedule override，还是先做全局 built-in 配置即可？
