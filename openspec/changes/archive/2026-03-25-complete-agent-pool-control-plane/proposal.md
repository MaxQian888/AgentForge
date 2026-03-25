## Why

AgentForge 的 PRD 和 `docs/part/AGENT_ORCHESTRATION.md` 已经把 AgentPool 定义成 Go Orchestrator 的核心控制面，要求具备预热池、等待队列、冷启动/命中指标、生命周期恢复、状态查询与可观测能力；但当前仓库真实实现仍只是 `src-go/internal/pool/pool.go` 和 `src-bridge/src/runtime/pool-manager.ts` 两个“最大并发计数器”，池满时只能直接报错，既没有排队与复用，也没有真实的控制面状态。因此需要补齐一个 repo-truthful 的 AgentPool control plane，把现有 spawn/dispatch 链路从“能占坑”提升到“能调度、能观察、能恢复”的完整能力。

## What Changes

- 新增统一的 AgentPool 控制面，定义活跃实例、预热实例、等待队列、容量策略、冷启动/命中统计和恢复态的权威模型，而不是继续依赖前后端各自推导的简化数字。
- 把 Go 侧 agent spawn / dispatch 行为从“池满直接 blocked”升级为“根据池策略入队、复用预热实例或冷启动”，并对外返回明确的 queued / started / blocked 结果。
- 补齐 Bridge 运行时池与 Go 池之间的状态对齐接口，让 Go 侧能看到真实的 runtime occupancy、最近活动、恢复态和容量诊断，而不是只拿到 execute 时报错。
- 为 Dashboard Agent Monitor、Agent 详情和相关 store 增加池控制面视图，至少展示 active / warm / queued / available、冷启动与命中语义、等待中的任务以及可恢复运行态。
- 为 AgentPool 增加统一的 API、事件与运维反馈契约，覆盖队列进入、队列释放、实例复用、容量耗尽、恢复失败和池状态刷新，避免消费者只能靠“没启动成功”去反推问题。
- 对齐与插件系统设计中“懒加载、生命周期、健康检查、资源限制”一致的宿主原则，让 AgentPool 在调度 Agent runtime 时也具备可观察的生命周期和资源边界，而不是继续作为黑盒计数器。

## Capabilities

### New Capabilities
- `agent-pool-control-plane`: 统一管理 AgentPool 的容量模型、预热池、等待队列、池状态查询、恢复态、诊断指标，以及面向 Web/运维消费者的控制面契约。

### Modified Capabilities
- `agent-spawn-orchestration`: spawn 流程需要从“池满即失败”升级为遵循 AgentPool 控制面策略，可返回 queued / reused warm runtime / cold start 等真实启动语义。
- `agent-task-dispatch`: 任务指派后的 dispatch 结果需要支持队列化与池控制面反馈，而不是把所有容量问题都折叠成 blocked。

## Impact

- Affected code: `src-go/internal/pool/**`, `src-go/internal/service/agent_service.go`, `src-go/internal/service/task_dispatch_service.go`, `src-go/internal/handler/agent_handler.go`, `src-go/internal/model/agent_run.go`, 可能新增 AgentPool repository / status service / queue integration；`src-bridge/src/runtime/pool-manager.ts`, `src-bridge/src/server.ts`, 以及 Agent 相关 Dashboard/store 文件。
- APIs / events: `/api/v1/agents/pool`, `/api/v1/agents`, dispatch/spawn 响应体、相关 WebSocket 事件，以及 Bridge 健康/active 状态接口都需要扩展为控制面语义。
- Data / runtime systems: 需要引入池状态、等待队列、预热实例或其摘要、命中/冷启动统计以及恢复态相关的权威状态来源；队列实现可复用现有 `src-go/internal/queue/task_queue.go` 或其演进版本，但必须真实接线。
- Cross-cutting constraints: 变更需要同时对齐 `docs/PRD.md` 的 AgentPool 目标态和 `docs/part/PLUGIN_SYSTEM_DESIGN.md` 的生命周期/懒加载/健康检查原则，避免产生另一套独立于宿主运行时语义之外的池管理模型。
