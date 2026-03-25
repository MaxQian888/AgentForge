## Context

`complete-go-dispatch-control-plane` 是在两条已经归档的主线之上继续补齐的 focused change：
- `wire-task-assignment-into-agent-dispatch` 已经引入 `src-go/internal/service/task_dispatch_service.go`，让 assignment、manual spawn 和结构化 `TaskDispatchResponse` 初步接上真实派发链路。
- `complete-agent-pool-control-plane` 已经把 AgentPool admission、queue entry 和 promotion 基础能力落成主 spec，并在 Go 侧建立了队列化的 `queued` 结果。

当前仓库的真实问题，不再是“有没有 dispatch”，而是“Go dispatch control plane 还没有一份能被所有入口共同信任的完整真相”。现状主要分成四个裂缝：

1. `TaskDispatchService.Assign/Spawn(...)`、`AgentService.RequestSpawn(...)` 和 queued promotion 仍然各自持有一部分 guardrail 判断，manual spawn、assignment dispatch 和 promotion failure 还没有共享同一套 decision model。
2. 预算治理仍是分裂状态：`src-go/internal/service/agent_service.go` 的 `UpdateCost(...)` 只做 task 级阈值判断；`src-go/internal/service/cost_service.go` 也保留了一套 task-only 广播；`src-go/internal/cost/tracker.go` 只有 per-task 内存累计；scheduler 的 `cost-reconcile` 只回填 task/team spend，PRD 里要求的 sprint/project dispatch guardrail 没有形成控制面。
3. queue truth 在 Go 内部存在，但跨面契约不完整：Go `DispatchOutcome` 已有 `Queue`，而 `src-im-bridge/client/agentforge.go` 的 `DispatchOutcome` 没有 queue 字段；`formatTaskDispatchReply(...)` / `formatAgentSpawnReply(...)` 对 `queued` / `skipped` 仍然回落成泛化文案。
4. PRD 和 `docs/part/AGENT_ORCHESTRATION.md` 把 Go 派发定义成“assignment/spawn -> admission -> worktree -> bridge -> budget -> realtime/IM”的完整控制面，但当前实现仍主要把这些阶段拆成 handler/service 级拼接，而不是一个可复用的 control-plane contract。

本次设计必须在不扩成新产品线的前提下，把这些裂缝补齐：保留现有 task-centered / queue-backed / worktree-backed 路线，同时让 dispatch、queue、budget、IM contract 形成一个共享真相。

## Goals / Non-Goals

**Goals:**
- 为 assignment-triggered dispatch、manual spawn 和 queued promotion 建立一套共享的 Go dispatch control-plane decision model。
- 让 `TaskDispatchResponse`、WebSocket dispatch payload 和 IM/client contract 对 `started`、`queued`、`blocked`、`skipped` 拥有稳定、可机读、可展示的共同语义。
- 把 budget governance 纳入 dispatch 生命周期，覆盖 task / sprint / project 三层准入、promotion recheck、runtime threshold handling 和 operator-visible feedback。
- 复用现有 `TaskDispatchService`、`AgentService.Spawn(...)`、AgentPool queue repo、scheduler reconcile、WS 事件和 IM binding 机制，而不是再造第二套派发系统。
- 让队列中的 admission context 在 enqueue、still-queued、promoted、failed 各阶段都不丢失，便于 dashboard、IM 和运维面消费。

**Non-Goals:**
- 不重写 TS bridge runtime、本次不扩展新的 runtime/provider 功能线。
- 不做新的 dashboard 视觉重构；前端只消费更完整的 control-plane contract。
- 不把本次范围扩展成多 Agent planner / coder / reviewer 调度编排。
- 不顺手统一所有历史 API 命名漂移，只处理与 dispatch / queue / budget control plane 直接相关的 DTO 与事件契约。
- 不依赖当前其他未完成 change（例如 project settings 相关新线）作为前置条件。

## Decisions

### Decision: 引入共享的 `DispatchControlPlane` 语义层，统一 assignment、manual spawn 与 promotion guardrails

本次不会再把“判定能否派发”继续分散在 `TaskDispatchService`、`AgentService.RequestSpawn(...)` 和 queued promotion 路径里，而是引入一层共享的 Go dispatch control-plane 语义层。它的职责是：
- 读取 task / member / project / sprint / active run / queue entry 上下文；
- 评估 dispatch preflight（成员有效性、重复运行、pool admission、worktree readiness、budget readiness）；
- 生成统一的 `DispatchDecision` / `DispatchOutcome`；
- 对 started / queued / blocked / skipped 分支应用一致的 payload、通知和 realtime publishing。

`TaskDispatchService` 继续扮演 task-centered API orchestration 边界，`AgentService.Spawn(...)` 继续扮演“真正启动 runtime”的执行器，但 guardrail decision 和 outcome shaping 不再各处重复拼装。queued promotion 也走同一层 decision service，再决定继续排队、标记失败，还是调用 `Spawn(...)`。

备选方案一是继续扩展 `TaskDispatchService` 直到它同时承担 assignment、spawn、promotion、budget。这样会把 task-facing service 变成新的上帝对象。备选方案二是继续让 `AgentService.RequestSpawn(...)` 负责 manual spawn、让 `TaskDispatchService` 负责 assignment。这样无法收敛 shared truth，因此都不采用。

### Decision: `DispatchOutcome` 升级为跨 API / WS / IM 的 canonical contract，而不是继续依赖自由文本

当前 Go 侧 `DispatchOutcome` 只包含 `status`、`reason`、`run`、`queue`，而部分消费者甚至没有完整实现这四个字段。本次设计会把它当成真正的 canonical contract：
- 所有 dispatch 入口统一返回 `started` / `queued` / `blocked` / `skipped` 四态；
- `queued` 必须带 queue reference 和 admission context；
- `blocked` 必须带 machine-readable reason code 与可展示 reason；
- `skipped` 不能再退化成“默认成功但没更多信息”。

为避免继续依赖自由文本，本次会在 Go contract 中增加 guardrail-oriented machine fields，例如 `reasonCode` 与可选 `budgetScope` / `guardrailScope`。HTTP 响应、WebSocket payload、IM client DTO 与格式化函数都消费同一对象，IM 和前端不再自行“猜” queue 或 blocked 的真实含义。

备选方案是继续保留纯字符串 `reason`，只靠每个消费者分支文案。仓库已经证明这样会导致 IM client 丢 queue 字段、`queued` 被格式化成“未启动”，因此不采用。

### Decision: 预算治理拆成独立的 `DispatchBudgetGovernance` 组件，并同时覆盖 preflight 与 runtime 两个阶段

预算不再只是 `UpdateCost(...)` 里的 task-only if/else。本次会引入一个独立的 budget governance 组件，分成两条路径：

1. **Preflight governance**：在 assignment-triggered dispatch、manual spawn 和 queue promotion 前，判断 task / sprint / project 当前是否还允许启动新的 runtime。
2. **Runtime governance**：在 TS bridge cost update 到达 Go 后，统一更新多层 spend，给出 `none` / `warn` / `exceeded` / `freeze-admission` 等动作，并驱动 cancel、status update、queue freeze 与 realtime feedback。

task 与 sprint 已经有明确的 budget/spend 字段可复用；project 级预算在文档里是核心能力，但当前 schema/model 不存在，因此本次会新增 first-class project budget persistence，而不是把这部分塞进 `projects.settings`。原因是：
- dispatch preflight 与 operator reporting 都需要频繁查询、聚合和比较预算数值；
- 本仓库另有 project settings 新线，复用 settings JSON 会制造额外耦合；
- task/sprint/team 预算本来就是 first-class numeric model，project 保持一致更利于 reconcile 与 operator API。

备选方案一是继续保留 task-only runtime kill，再把 sprint/project 预算留给未来；这会继续让 dispatch control plane 与 PRD 目标态脱节。备选方案二是把 project budget 临时塞进 settings JSON；它能减少 migration，但会让查询、reconcile 和当前其他 settings 变更线高度耦合，因此不采用。

### Decision: queued entry 的“仍在排队”和“终态失败”要分开建模，promotion recheck 复用同一 guardrail 结果

队列提升不是简单地在释放 slot 后直接调用 `Spawn(...)`。本次会把 promotion 分成：
- 拉取下一条 queued entry；
- 重新解析 task/member/runtime/role/budget 上下文；
- 重新执行 dispatch control-plane preflight；
- 根据 guardrail verdict 决定 `promoted`、`still queued` 或 `failed`。

其中：
- **可恢复问题**（例如 sprint/project budget 临时耗尽、Bridge/Worktree 短暂降级）保持队列项为 `queued`，更新 reason / reasonCode / updatedAt，让 operator 能看到“为什么还没被提升”；
- **不可恢复问题**（任务不存在、成员失效、任务已有活跃 run、严重 worktree ownership 问题）标记为 `failed`，保留 admission history，不再无限重试；
- 真正开始 runtime 时再进入 `promoted` -> `started` 链路。

备选方案是 promotion 只做 best-effort `Spawn(...)`，失败就统一记为 `failed` 或直接吞掉。这样会让队列丢失“仍可恢复但暂未提升”的真相，因此不采用。

### Decision: 保持现有 WebSocket 事件名尽量稳定，但 payload 升级为 canonical dispatch / budget envelope

前端 store 已经在消费 `agent.queued`、`task.dispatch_blocked`、`budget.warning`、`budget.exceeded` 等事件，本次不新增一套完全平行的 event namespace。设计上保持主要事件名稳定，但 payload 统一提升：
- dispatch 相关事件统一包含 canonical `dispatch` 对象；
- queue 相关事件包含 queue reference、reasonCode、budgetScope 和 admission timestamps；
- budget 相关事件保持 `budget.warning` / `budget.exceeded`，但增加 `scope`、`scopeId`、`dispatchImpact` 等 machine fields。

这样前端与 IM 都能在兼容现有事件订阅的同时拿到更完整的 control-plane truth。备选方案是把 task dispatch、queue promotion、budget governance 全部拆成新事件名；这会放大 store 改造范围，因此不采用。

### Decision: IM client 与命令面不再自定义降级逻辑，而是镜像 Go DTO 并消费 canonical status branches

`src-im-bridge/client/agentforge.go` 目前没有 `DispatchOutcome.Queue`，`formatTaskDispatchReply(...)` / `formatAgentSpawnReply(...)` 也只对 `started` / `blocked` 做了真实分支处理。设计上，IM client 将直接镜像 Go 的 canonical dispatch DTO（包括 queue 和新增 guardrail fields），命令格式化函数必须明确处理：
- `started`: 展示 run identity；
- `queued`: 展示 queue/reference 与“已进入等待队列”的真实语义；
- `blocked`: 展示原因和 guardrail 分类；
- `skipped`: 明确表达“已分配但本次不启动 agent”。

备选方案是保留当前 client struct，不断在 reply formatter 中添加字符串 heuristics。仓库已经证明这会继续丢字段和漂移，因此不采用。

## Risks / Trade-offs

- [project 级预算需要新增持久化模型，迁移面会扩大] -> 采用默认 `0 = unbounded` 的向后兼容 schema；先加字段与 repo，再逐步接入 preflight 和 reporting。
- [dispatch decision 变得更结构化，会暴露更多“未启动但不是失败”的状态给前端和 IM] -> 通过 canonical reasonCode、queue metadata 和统一 reply formatter，把这些状态解释成 operator-usable truth，而不是模糊成功/失败二元。
- [promotion recheck 更严格后，队列可能更长时间停留在 queued] -> 明确区分 `still queued` 与 `failed`，并在 queue roster / realtime payload 中暴露当前 guardrail 原因，避免“黑箱等待”。
- [现有 `AgentService.UpdateCost(...)` 与 `CostService.RecordCost(...)` 存在职责重叠] -> 本次以 dispatch control-plane 路径为权威，`CostService` 收缩为聚合查询或被薄封装，避免双写双广播继续扩散。
- [前端/IM/store 需要同步更新 DTO，短期内容易出现兼容差异] -> 新字段全部保持可选并提供兼容映射层，先保证 started/queued/blocked/skipped 四态可正确显示，再逐步启用更细的 reasonCode / budgetScope。

## Migration Plan

1. 新增/扩展 dispatch 与 budget 相关模型、持久化字段和 repository contract，包含 project budget state、queue guardrail metadata 与 canonical dispatch DTO。
2. 引入共享的 Go dispatch control-plane / budget governance 组件，并让 `TaskDispatchService`、`AgentService.RequestSpawn(...)` 和 queued promotion 统一走这条 decision path。
3. 把 runtime cost update、scheduler cost reconcile 与 budget events 收敛到新的 governance 语义，补齐 task / sprint / project 级联 spend 更新与 warning/exceeded 行为。
4. 升级 WebSocket、IM client、IM command formatter 与前端相关 stores，使其消费 canonical dispatch / queue / budget payload。
5. 跑 focused verification，覆盖 assignment dispatch、manual spawn、queue promotion、budget warning/exceeded、IM reply branches 与 queue roster truthfulness。

回滚策略以“逻辑回滚、数据前向兼容”为主：新增字段保持可空或默认 unbounded，旧事件名不移除；如果实现阶段需要临时回退，只需撤回新的 control-plane decision path，保留 additive schema 和 DTO 字段不影响旧消费者。

## Open Questions

- project 级预算窗口应按自然月自动重置，还是先按“当前累计窗口 + 运维手动重置”落地，再把月度结转单独提 change？
- 当 sprint/project 预算超限时，首版是否只取消触发超限的当前 run，还是要像 PRD 目标态那样主动中止该 scope 内全部活跃 dispatch？
