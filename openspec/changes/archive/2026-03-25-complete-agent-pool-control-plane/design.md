## Context

当前仓库已经具备 Agent 运行链路的几个关键片段，但 AgentPool 仍停留在“薄计数器”阶段：

- `src-go/internal/pool/pool.go` 只记录活跃 run 是否占用槽位，能力仅有 `Acquire` / `Release` / `ActiveCount` / `Available` / `List`。
- `src-go/internal/service/agent_service.go` 和 `task_dispatch_service.go` 在池满时直接返回 `ErrAgentPoolFull`，把容量问题折叠成同步 blocked 结果，没有等待队列、复用或恢复语义。
- `src-go/internal/queue/task_queue.go` 已经有 Redis Streams 任务队列原型，但目前没有接入真实的 dispatch / spawn 链路，也没有和 AgentPool 状态、UI、事件打通。
- `src-bridge/src/runtime/pool-manager.ts` 只有 `runtimes.size >= maxConcurrent` 的容量判断，缺少 warm pool、池健康摘要、冷启动/命中统计与权威诊断输出。
- Dashboard 的 `app/(dashboard)/agents/page.tsx` 与 `lib/stores/agent-store.ts` 只消费 `active / max / available / pausedResumable`，并且一部分数字还是前端根据 agent roster 二次推导，不是权威控制面。

与此同时，`docs/PRD.md` 与 `docs/part/AGENT_ORCHESTRATION.md` 已经明确给出了 AgentPool 目标态：预热池、等待队列、冷启动/预热命中、容量上限、恢复态、状态查询、指标与运维反馈。`docs/part/PLUGIN_SYSTEM_DESIGN.md` 虽然聚焦插件体系，但它也对宿主运行时提出了统一要求：懒加载、生命周期、健康检查、资源限制和跨进程状态同步。AgentPool 如果继续只是局部计数器，就会与 repo 当前的宿主语义脱节。

## Goals / Non-Goals

**Goals:**

- 为 AgentForge 引入一个 Go 权威的 AgentPool 控制面，统一描述 admission、活跃实例、预热实例、等待队列、容量诊断和恢复态。
- 让 task dispatch / manual spawn 在池容量受限时支持 queue-first 结果，而不是只会返回 blocked。
- 为 Bridge runtime pool 增加可被 Go 消费的池摘要与 warm/cold 语义，使前后端能看到同一套池状态。
- 为 Dashboard Agent Monitor、agent detail、WebSocket 事件和相关 API 暴露真实的 pool/queue/operator 信息。
- 保持与插件系统设计一致的宿主原则：懒加载、生命周期、健康检查、资源边界和跨进程状态同步。

**Non-Goals:**

- 不在本次 change 中实现 PRD 里所有长期多 Agent 编排策略，例如 planner/coder/reviewer 多角色自动拆分和全自动批量调度。
- 不把 AgentPool 直接扩成独立的分布式任务队列平台；本次只处理单实例 Agent admission 与可恢复控制面。
- 不重写已有的 `agent-spawn-orchestration`、`agent-task-dispatch`、scheduler control plane 或 team runtime，只对它们做必要的接线与合同升级。
- 不在这次 change 中引入完整的资源计费/内存限制执行器；只定义 AgentPool 需要消费和暴露的资源边界摘要。

## Decisions

### Decision: Go owns the canonical AgentPool admission state; Bridge mirrors runtime occupancy and warm-slot health

AgentPool 的权威状态放在 Go 侧，由新的 control-plane service 统一管理：

- 任务当前是否被 admitted、queued、running、paused-resumable 或 admission-blocked
- 项目级 active / queued / available / warm / max 等容量摘要
- queue entry 的持久化事实、顺序和升级为实际启动的结果

Bridge 不再被动地只在 execute 时抛出“pool at capacity”，而是额外提供 runtime pool summary 与 warm-slot 诊断接口，供 Go 定时拉取或在 admission 前预检。这样可以保持 PRD 中“Go Orchestrator 是业务状态权威”的原则，也符合插件系统设计里跨进程状态同步的宿主语义。

备选方案一是让 Bridge 成为池权威，Go 只根据 execute 结果推断状态；这会让 Dashboard、dispatch 和 queue 只能依赖二手推导，无法做真实 operator 视图。备选方案二是完全不信任 Bridge 状态，只保留 Go 本地计数；这会继续掩盖 runtime occupancy、warm reuse 和恢复态。因此不采用。

### Decision: Queue admission is persisted as first-class pool state, while real agent runs are still created only when execution starts

池满时不立即创建 `agent_runs` 记录，而是先创建 queue entry（例如新的 `agent_pool_queue_entries` 或等价权威存储），记录：

- `task_id` / `project_id` / `member_id`
- runtime/provider/model/role/budget admission 请求快照
- enqueue 原因、优先级、顺序号、来源（manual spawn / assignment dispatch）
- 当前 queue status（queued / admitted / cancelled / expired / failed）

只有当 queue entry 被真正 admitted 时，才进入既有的 spawn path 去创建 `agent_runs`、worktree 和 bridge execute。这样可以避免引入“queued run”这种半启动状态污染当前 runtime / cost / log 语义，同时仍然满足 PRD 的队列化目标。

备选方案一是在池满时直接创建 `agent_runs(status=queued)`；这会把运行态和 admission 态混在一起，增加现有查询和状态机复杂度。备选方案二是只用 Redis Streams 内存队列；这对消费很方便，但 operator UI、重启恢复和人工排障缺少稳定事实源。因此采用“持久化 queue entry + 运行时再创建 real run”的模式。

### Decision: Reuse the existing queue prototype as transport assistance, but introduce a control-plane repository for operator-truthful state

`src-go/internal/queue/task_queue.go` 已经提供 Redis Streams enqueue/dequeue 基础，但它目前没有接线，也不足以承载完整 operator 视图。本次设计保留它作为可选的 transport/trigger 加速层，但不把它当成唯一事实源。事实源放在 control-plane repository，Redis queue 只负责：

- 可选的异步唤醒 / next-admission 提示
- 减少纯轮询带来的延迟
- 为未来更高并发场景保留演进空间

这样既复用了当前 repo 已有雏形，也避免把关键可观测状态埋进 Redis 消息体而失去可维护性。

备选方案是彻底忽略现有 `task_queue.go`，从零写另一套 queue transport；这会浪费现有 seam。另一种极端是完全依赖 Redis Streams 作为唯一 queue 真相；这会让 Web/ops 查询、重放和手工修复成本过高，因此不采用。

### Decision: Warm pool is a Bridge-side reusable slot model with Go-visible summaries, not a hidden implementation detail

Warm pool 放在 Bridge runtime pool 侧实现，因为真正的 runtime、provider 可执行环境、session continuity 和工具插件激活都在 TS Bridge 宿主中。Bridge 需要维护一组可复用 warm slots，并对外暴露：

- `warm_total`
- `warm_available`
- `warm_reuse_hits`
- `cold_starts`
- `last_reconcile_at`
- 关键 runtime key（runtime/provider/model 维度）的摘要

Go admission 决策不直接操纵 warm slot internals，而是通过 summary + acquire/release 结果感知 warm reuse 是否发生，并把这一事实写入 pool events、queue admission 结果和 operator metrics。这样既符合插件系统设计中的“懒加载 + 生命周期 + 健康检查”原则，也避免把 TS runtime 细节硬编码进 Go。

备选方案一是在 Go 侧伪造 warm 池计数，但实际 Bridge 完全不知道 warm slot；这会让指标失真。备选方案二是在 Go 侧持有完整 warm runtime 对象；当前 runtime 实现在 Bridge 中，不适合跨语言搬迁。因此采用 Bridge warm-slot + Go summary mirror。

### Decision: Dispatch and manual spawn both consume one admission result model

无论入口是 `/api/v1/agents/spawn` 还是 task assignment dispatch，AgentPool admission 都统一返回一个结构化结果，例如：

- `started`: 已立即获得执行槽位，并附带实际 run 引用
- `queued`: 已记录等待项，并附带 queue entry 引用、当前位置/前方任务数或 admission reason
- `blocked`: 因无效成员、任务已有活动 run、worktree 不可用、bridge 不健康等原因被阻止

这样可以收敛当前 `agent_service.go` 与 `task_dispatch_service.go` 各自解释容量错误的分叉，也让前端、WebSocket、IM 反馈共享同一组语义。与此同时，已有 `DispatchOutcome` 和 spawn summary 只需要做增量扩展，不需要再造一套平行合同。

备选方案是让 manual spawn 继续返回错误，而 assignment dispatch 使用 queued 结果；这会制造用户认知差异，并让实现不断分叉，因此不采用。

### Decision: Operator surfaces use pool summary plus queue roster, and stop deriving authoritative numbers from the agent roster alone

前端 Agent Monitor 与 store 不再只靠 agent roster 推导 `active` 或 `pausedResumable`。新的 API 会直接返回 pool summary 和 queue roster，前端只做展示层聚合，例如：

- summary cards: active / warm / queued / available / max
- queue table: task, member, runtime tuple, enqueuedAt, queue reason, next admission hint
- detail panels: last warm reuse / cold start reason / recovery state / bridge health

WebSocket 事件也需要新增 queue 相关事件，如 `agent.pool.updated`、`agent.queued`、`agent.queue.promoted`、`agent.queue.failed`，避免 UI 只能靠轮询和本地推导猜测状态。

备选方案是一切继续复用 `/api/v1/agents` agent roster，只在前端上再塞几个推导数字；这无法表达“还没开始跑但已成功入队”的状态，因此不采用。

## Risks / Trade-offs

- [Queue state and runtime state drift after crashes] → 通过持久化 queue entry、admission reconciliation 与 terminal run completion hook，在服务重启后恢复待执行项并修正孤儿状态。
- [Warm pool adds cross-process complexity] → Warm slot internals仍留在 Bridge，只把摘要和 acquire/release 结果镜像给 Go，减少跨语言耦合面。
- [Dispatch semantics become broader than current UI expects] → 统一扩展 `DispatchOutcome` / spawn 响应模型，并让 WebSocket 与前端 store 同步升级，避免某个入口仍只认识 started/blocked。
- [Existing queue prototype could diverge from new persistence model] → 明确 Redis queue 不是唯一事实源，只把它当 transport hint；持久化 repository 负责 operator-truthful 状态。
- [Scope expands into general workflow scheduling] → 本次只覆盖 AgentPool admission、warm/queue/recovery/operator 视图，不引入任意工作流编排 DSL 或多节点分布式队列。

## Migration Plan

1. 引入 AgentPool control-plane domain model、持久化 queue entry/reconciliation 逻辑，以及扩展后的 summary DTO。
2. 将 `agent_service.go` 的直接 `pool.Acquire` 升级为 admission service 调用；池满时返回 queued 或 blocked，而不是直接 `ErrAgentPoolFull`。
3. 将 `task_dispatch_service.go` 切换到统一 admission 结果模型，并扩展 `DispatchOutcome` 以支持 queued。
4. 在 Bridge runtime pool 中加入 warm-slot summary 与 pool diagnostics 接口，同时让 Go bridge client 能读取这些摘要。
5. 扩展 `/api/v1/agents/pool`、相关 WebSocket 事件、Dashboard Agent Monitor 和 agent detail 页面，展示 queue/warm/control-plane 视图。
6. 增加 release-driven next-admission 和启动时 reconciliation，保证 queued 项能在槽位释放后被推进。

回滚策略：

- 可以临时关闭 queued admission，只保留 immediate start + blocked 模式，但仍保留新的 summary API 和 queue persistence 表结构。
- 如果 Bridge warm-slot 实现出现不稳定，可先降级为 cold-start-only，同时保持控制面和 queue 语义不变。

## Operator Notes

- `POST /api/v1/agents/spawn` 在 dispatcher 路径下不再保证只返回已启动的 run；当池容量不足时会返回 `202 Accepted` 和 `dispatch.status=queued`。
- `/api/v1/agents/pool` 现在是权威的 operator summary，包含 queued roster、warm count 和 degraded 标记；前端不应再只从 agent roster 推导这些数字。
- Bridge 的 warm slot 目前是宿主侧摘要和 bookkeeping，不代表 Go 侧持有可直接操作的 runtime 对象；Go 只消费其 summary 和启动结果。

## Open Questions

- queue entry 的事实源是独立表还是现有 task/agent runtime 表的扩展字段更合适？当前更偏向独立表，但实现前需要结合现有 repository 风格再确认。
- 第一版 queue ordering 是否直接采用 FIFO + priority，还是需要把项目预算/成员亲和度一并纳入 admission 排序？当前建议先做 priority + createdAt。
- 是否在第一版就补上 IM `/agent status` 的池控制面摘要，还是先交付 API/Web Dashboard，再由 IM 复用同一 summary contract？
