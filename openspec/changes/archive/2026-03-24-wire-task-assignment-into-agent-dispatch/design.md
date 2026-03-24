## Context

`docs/PRD.md` 已经把 “任务分配给 Agent -> Orchestrator 选择/创建实例 -> worktree -> Bridge -> 实时反馈” 定义为 AgentForge 的主链路之一，但当前仓库实现仍然把这条链路拆成了互不对齐的几段。`src-go/internal/handler/task_handler.go` 的 `Assign` 仍然直接调用 `TaskRepository.UpdateAssignee(...)`，只更新 assignee 和进度信号，不会复用 `TaskService` 的状态广播，也不会触发真正的 Agent 派发；`src-go/internal/service/agent_service.go` 的 `Spawn` 已经能完成 worktree/bridge/runtime 启动，但它需要调用方显式提供 `memberID`，并不知道“当前任务已经指派给哪个 agent 成员”。

与此同时，IM 和后端 API 之间还存在真实契约漂移：`src-im-bridge/client/agentforge.go` 的 `/task assign` 仍发送 `{assignee}` 到 `PATCH /api/v1/tasks/:id/assign`，而 Go 后端已经暴露 `POST /api/v1/tasks/:id/assign` 且要求 `assigneeId + assigneeType`；`/agent spawn <task-id>` 只发送 `task_id`，但 Go 侧 `AgentHandler` 仍校验 `memberId` 必填。说明“Agent 派发流程”不仅缺少编排，还缺少一个可被 Web/IM 共同依赖的任务中心化契约。

这次变更因此需要横跨 task assignment、agent spawn、member/project 校验、WebSocket/notification 反馈以及 IM 命令契约，但应保持范围收敛：我们只把“任务指派给 Agent 后的派发流程”补成统一业务能力，不扩大到智能推荐、异步队列调度、Dashboard 视觉重构或完整的 Agent 池策略。

## Goals / Non-Goals

**Goals:**
- 让“将任务指派给 agent 成员”成为真实的派发入口，而不是只更新 assignee 字段。
- 为派发流程建立统一的任务上下文与前置校验，包括成员归属、成员类型、成员激活状态、任务/项目存在性和重复运行保护。
- 让 `POST /api/v1/tasks/:id/assign` 与 `POST /api/v1/agents/spawn` 共享同一套 dispatch orchestration，并返回可被 Web/IM 消费的结构化结果。
- 复用现有 `AgentService` 的 worktree/bridge/runtime 启动与补偿逻辑，避免再造第二套运行时编排。
- 补齐派发成功、派发阻塞和派发失败时的事件、通知与 IM 反馈语义，使后续 Dashboard/任务页能消费同一份真相。

**Non-Goals:**
- 不在本次 change 中实现 PRD 里的智能派单推荐、暖池命中策略、任务队列或多 Agent 调度器。
- 不重做完整的任务状态机、进度预警系统或 Dashboard 页面结构；这些只作为本 change 的消费方。
- 不引入新的持久化表专门记录 dispatch queue/history；优先复用现有 task、member、agent_runs 和通知基础设施。
- 不要求一次性修完 IM 客户端中的所有历史 API 漂移，只收敛与 assignment/spawn 直接相关的契约。

## Decisions

### Decision: 新增任务中心化的 dispatch orchestration 服务，负责串联 assignment 与 spawn

本次不把派发逻辑继续塞进 `TaskHandler`、`TaskService` 或 `AgentHandler` 中，而是引入一个专门的 dispatch orchestration 边界，由它统一读取 task/member/project，上收 assignment 意图，执行派发前校验，并在需要时委派给现有 `AgentService.Spawn(...)` 完成真正的运行时启动。

这样可以把“任务指派”和“Agent 启动”之间的业务决策留在一个地方：哪些 assignment 只更新负责人，哪些 assignment 需要立刻尝试 dispatch，哪些失败应该保持 assignee 但返回阻塞结果。`TaskService` 继续负责通用任务生命周期，`AgentService` 继续负责 runtime startup/compensation，dispatch service 只负责两者之间的任务级业务串联。

备选方案是把 assignment 直接扩展进 `TaskService`，或者把 assignee 推导塞进 `AgentService.Spawn(...)`。前者会把 task CRUD/service 变成同时承担 runtime 编排的上帝对象，后者则会让 runtime 服务知道过多的任务分配语义，因此不采用。

### Decision: agent assignment 采用“同步 assignment + 立即尝试 dispatch”的第一阶段模型

对于 `assigneeType=agent` 的 assignment，请求会先完成任务负责人写入与必要的状态推进，然后立即执行一次 dispatch preflight；如果前置条件满足，就直接调用统一 dispatch 路径启动 agent runtime；如果前置条件不满足，则 assignment 保持成功，但返回明确的 dispatch outcome（例如 blocked / invalid target / already running），而不是把 assignment 与 startup 混成一次全失败。

这样做的原因是当前仓库已经有同步式 `spawn` 能力与清晰的补偿逻辑，但还没有成熟的任务队列、Consumer Group 或后台调度器来承接“稍后异步派发”。同步尝试可以尽快补齐主链路，同时保留未来升级为异步排队的空间。

备选方案一是 assignment 只落库，再要求用户额外点击一次 spawn；这会继续保留当前断链。备选方案二是直接引入异步 dispatch queue；这会把范围扩大到任务队列、重试策略、可见状态和后台 worker，不适合本次 change。

### Decision: 派发结果使用结构化 outcome 返回，并区分 assignment 成功与 dispatch 成功

`/tasks/:id/assign` 不再只返回裸 `TaskDTO`；当请求进入 agent dispatch 路径时，返回体需要同时包含任务结果和一个结构化 dispatch outcome，至少区分：
- `started`: 已成功启动 agent run，并附带运行信息；
- `blocked`: assignment 成功，但由于已有活跃 run、目标成员无效/未激活、项目归属不匹配或其他前置条件不满足，未启动新的 run；
- `skipped`: 本次 assignment 不是 agent assignment，因此不尝试 dispatch。

这个设计允许调用方正确表达“任务已经分给 Agent，但还没真正启动”与“任务已启动 Agent 运行”之间的差异。对于 IM 命令和前端来说，这比把所有结果都压成 200/409 更可消费，也更贴合 PRD 里“分配”和“执行”是连续但不同阶段的建模。

备选方案是保持老的 `TaskDTO` / `AgentRunDTO` 响应形态不变，把 dispatch 结果只放在 WebSocket 或通知里。这样会导致同步调用方无法立即知道派发是否真正启动，因此不采用。

### Decision: 手动 spawn 变成任务上下文驱动的 retry/explicit entrypoint

`POST /api/v1/agents/spawn` 继续保留，但从“必须显式传入 memberId 的独立入口”调整为“可复用任务当前 agent assignee 的显式派发入口”。也就是说，请求至少要提供 `taskId`；若未提供 `memberId`，系统从任务当前 assignee 中解析 agent 目标；若任务未指派给有效的 agent 成员，则明确拒绝。

这样既能修复现有 IM `/agent spawn <task-id>` 与 Go handler 契约不一致的问题，也让手动 spawn 自然成为 dispatch retry 或 operator override 的入口，而不是另一条和 assignment 平行的编排路径。

备选方案是继续要求所有调用方总是传 `memberId`。这与当前 IM 命令面不一致，也违背“任务派发以任务为中心”的设计，因此不采用。

### Decision: 复用现有 `agent_runs`、task runtime 字段和通知设施，不新建 dispatch 专用持久层

本次 change 不新增 dispatch queue/history 表。真正进入 runtime 启动的尝试仍然复用 `agent_runs` 作为持久化事实来源，运行时元数据仍写回 task 的 `agent_branch`、`agent_worktree`、`agent_session_id`；在 preflight 阶段就被阻塞的派发则通过 assignment 响应、WebSocket 事件和站内通知暴露，而不是额外落一张新的 dispatch 表。

这个选择能把范围收敛在已有存储模型之内，同时保持 archived `agent-spawn-orchestration` 的补偿语义有效。真正需要长期分析 dispatch success rate 或排队历史时，再考虑增加专门的事件/历史表。

备选方案是立即建立一张 task dispatch attempts 表。它能提供更完整的运营分析，但会显著扩大迁移和实现范围，因此不采用。

### Decision: 事件与通知按“assignment -> dispatch outcome -> runtime lifecycle”分层，而不是只依赖已有 `agent.started`

assignment 仍然要保留任务级事件，但当 assignment 触发 agent dispatch 时，系统还需要产生派发结果事件，让前端和 IM 能明确知道这次派发是已启动、被阻塞还是失败。成功启动仍复用现有 `agent.started` / 后续 lifecycle 事件；未启动的新路径则需要新的 dispatch-blocked 结果语义和相应通知类型，以免消费者只能从“没有 agent.started”推断失败。

这样可以让后续 Dashboard、任务详情和通知中心在不访问多张表的情况下，直接消费派发阶段信号。对于 IM 入口，也能把“已分配但未启动”的情况说清楚，而不是简单回复“分配成功”。

备选方案是只保留 task assignment 事件，不引入 dispatch outcome 语义。那会继续模糊“assignment”和“execution”之间的边界，因此不采用。

## Risks / Trade-offs

- [同步 assignment 会变慢，因为它可能立即进入 worktree/bridge 启动] -> 仅把“尝试启动首个 runtime”纳入同步路径，不扩展为完整队列调度；后续如需排队可在同一 dispatch service 上升级。
- [assignment 成功但 dispatch 被阻塞，调用方可能误解为失败] -> 通过结构化 dispatch outcome 和明确的 IM/通知文案，把“assignment 成功、dispatch 未启动”单独建模。
- [多入口共享 dispatch 逻辑时，API 兼容层容易变复杂] -> 统一以 dispatch service 为唯一业务边界，由 handler 只做请求兼容解析与响应映射。
- [没有新增 dispatch 持久层会限制历史分析] -> 第一阶段优先把主链路补通；后续若运营分析成为明确需求，再单独提出 change 增加历史记录表。
- [现有前端/IM 客户端契约已漂移，实施时可能暴露更多历史不一致] -> 本 change 只收敛 assignment/spawn 相关 DTO 与字段命名，不顺手重构全部 API 客户端。

## Migration Plan

1. 先定义新的 capability specs，锁定 assignment-triggered dispatch、manual spawn fallback 和 outcome 语义。
2. 在 Go 后端引入 dispatch orchestration 服务，并让 task assignment 与 agent spawn handler 统一走这条业务路径。
3. 对齐 IM 客户端与命令面，确保 `/task assign` 和 `/agent spawn` 能消费新的任务中心化契约。
4. 补齐 WebSocket/notification payload 和 focused tests，验证 started / blocked / failed 三类结果。
5. 如果上线后同步 dispatch 对调用时延或稳定性影响过大，可以先关闭 assignment 自动尝试 dispatch，仅保留手动 spawn 使用同一 orchestration 的兼容路径；运行时启动与 worktree 补偿逻辑不需要回滚。

## Open Questions

- 首版 `assign` 是否需要显式参数允许“只分配、不自动 dispatch”，还是默认对 `assigneeType=agent` 总是尝试一次派发？
- dispatch-blocked 通知的默认接收方应只覆盖当前调用人和任务 reporter，还是要同步扩展到项目负责人？
