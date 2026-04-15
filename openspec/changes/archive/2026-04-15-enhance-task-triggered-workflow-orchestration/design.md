## Context

AgentForge 当前已经有三条彼此独立但还没完全打通的编排能力：

- `TaskWorkflowService` 会在任务状态迁移后读取 project workflow config，并触发 `notify`、`auto_assign_agent`、`auto_transition` 这类轻量动作。
- official built-in workflow starter library 已经落地，`standard-dev-flow`、`task-delivery-flow`、`review-escalation-flow` 都通过 `WorkflowPlugin` manifest 和 canonical workflow runtime 运行。
- workflow runtime 已经能持久化 `WorkflowPluginRun`、step outputs、pause state、review handoff 和 child workflow metadata。

但当前“任务状态变化驱动正式 workflow orchestration”仍然是断开的：

- 前端 workflow config panel 暴露的 trigger action（`auto_assign` / `dispatch_agent`）和 Go backend 实际识别的 action（`auto_assign_agent` / `auto_transition`）不一致。
- built-in workflow starters 在 manifest 和 spec 上已经开始讨论 task-driven activation，但后端 task trigger engine 还不能以 canonical 方式启动这些 starter。
- task transition 触发 workflow 仍然是 fire-and-forget，外部只能看到一条很薄的 `workflow.trigger_fired` event，看不到 structured outcome、run lineage 或 duplicate/dependency failure verdict。

这次 change 需要补的是一条 focused 的 control-plane：让 project task trigger 真正能启动官方 workflow starter，并把结果对齐成现有 dispatch / runtime contract 那种“truthful outcome”，而不是把任务编排扩成全新的 automation 平台。

## Goals / Non-Goals

**Goals:**

- 让 project-level task transition trigger 支持 canonical workflow starter activation，而不只停留在通知或轻量状态动作。
- 统一 workflow trigger action vocabulary，收掉前后端 drift，并为 legacy action 名提供兼容归一。
- 为 task-triggered workflow execution 输出结构化 outcome、reason、starter/run lineage 和 duplicate/dependency verdict。
- 让 `task-delivery-flow` 这样的官方 starter 可以声明并兑现真实可执行的 task-driven trigger profile。
- 在不阻塞 task transition 主链路的前提下，把 richer outcome 通过现有 realtime / task activity seam 对外广播。

**Non-Goals:**

- 不重写 visual workflow editor、workflow page 大界面，或把这条 change 扩成前端 mega-panel。
- 不把整个 workflow engine 重构成新的 DAG/runtime 统一层；plugin workflow runtime 与 DAG workflow runtime 继续保留现有分工。
- 不引入新的 scheduler orchestration、cron authoring 或广义 automation rule 平台。
- 不为 task-triggered workflow 历史单独新建完整审计表；本次以现有 websocket / progress activity / workflow run store 为主。

## Decisions

### Decision 1: Task workflow trigger contract 采用 canonical action vocabulary，并保留 legacy alias 归一

当前前端 workflow config draft 使用 `auto_assign` / `dispatch_agent`，而 Go backend 只识别 `auto_assign_agent`。这种漂移会让“看起来可配”的任务编排在 runtime 里并不稳定。

本次设计会定义一组 canonical action：

- `dispatch_agent`: 触发已有 task dispatch / assignment control-plane
- `start_workflow`: 启动 canonical workflow starter / workflow plugin run
- `notify`: 发送通知
- `auto_transition`: 执行状态流转

为了兼容当前已存配置，backend 会继续接受 `auto_assign` 与 `auto_assign_agent` 这类历史值，但在内部一律归一到 `dispatch_agent` 再执行和广播。

**Alternative considered:** 继续保留现在的字符串 switch，只额外再加几个 action 名。  
**Rejected because:** 这会把 drift 固化进更多 consumer，后续 event/docs/tests 仍然无法确定哪一个名字才是 canonical truth。

### Decision 2: Task-driven workflow activation 复用 WorkflowPlugin runtime，而不是再造一条 task-only workflow runner

官方成功案例已经明确落在 `WorkflowPlugin` starter library：`task-delivery-flow`、`review-escalation-flow` 都通过 `WorkflowExecutionService.Start(...)` 进入 canonical workflow run persistence、step output 和 pause semantics。任务状态驱动 starter 时，也必须复用这条 runtime，而不是走 `TaskWorkflowService` 自己的 ad hoc runner。

这意味着 `TaskWorkflowService` 只负责：

- 匹配 trigger rule
- 归一 action/config
- 验证目标 starter 的 trigger profile 和依赖状态
- 构造 task-scoped trigger payload
- 调用 canonical workflow runtime start seam

真正的 workflow step execution、retry、approval pause、child workflow lineage 仍由现有 workflow runtime 负责。

**Alternative considered:** 直接在 `TaskWorkflowService` 里 clone DAG template 或硬编码 planner -> coder -> reviewer 流程。  
**Rejected because:** 这会绕开已经归档的 workflow starter/runtime 成功案例，重新制造第二套“任务编排”真相。

### Decision 3: Starter 的 task-driven availability 以 manifest trigger profile 为真相源

当前 `PluginWorkflowTrigger` 只有简单的 `event` 字段，足够表达 `manual`，但不足以表达“这个 starter 能否被 task transition 启动、需要什么 task context、适用于哪些状态变化”。本次 change 需要把 trigger profile 扩成结构化声明，至少能够表达：

- 触发类型，例如 `manual` 或 `task.transition`
- task-driven profile 所需的 transition context（如 `fromStatus` / `toStatus` 或等价 profile id）
- 是否必须带 task identity

这样 `task-delivery-flow` 可以声明至少一个可执行的 task-driven profile，而 `review-escalation-flow` 可以继续 truthfully 保持 manual-only，除非仓库真的补上对应 task/review activation seam。

**Alternative considered:** 把 task-driven availability 只写在 built-in bundle metadata 或 project workflow config 里。  
**Rejected because:** runtime 真相应该来自 workflow manifest，本次 change 需要的是“声明能跑什么”和“实际能跑什么”保持一致。

### Decision 4: Trigger evaluation 继续异步执行，但 outcome 要结构化并复用现有 broadcast / progress seam

`task_handler.go` 当前在任务状态更新成功后异步调用 `EvaluateTransition(...)`。这个异步边界是合理的，因为 task transition API 不应该被后续 workflow orchestration 阻塞。但异步不等于无真相。

本次 change 会让 `TriggerResult` 升级为 outcome-bearing result，至少包含：

- normalized action
- outcome: `started | completed | blocked | skipped | failed`
- reason code / reason message
- plugin/starter identity when action is `start_workflow`
- workflow run id when成功启动

对外传播上优先复用：

- `workflow.trigger_fired` websocket event，增加结构化 outcome payload
- task progress activity source，记录 workflow-triggered orchestration 对任务活跃度的影响

这样 workflow panel 的 recent activity、task progress、以及后续 IM/operator consumer 都能拿到 richer truth，而不用先引入新的持久化审计表。

**Alternative considered:** 把 trigger 执行改成同步返回到 task transition API，或者额外新建专用 trigger-history persistence。  
**Rejected because:** 前者会扩大现有 task API 的 latency/失败面，后者会把 focused change 扩成更宽的 automation history 项目。

### Decision 5: Duplicate task-triggered workflow starts 采用 task plus starter profile guard，而不是盲目重复起 run

如果同一任务在短时间内重复命中同一个 starter profile，而已有对应 workflow run 仍处于 `pending | running | paused`，继续启动新 run 只会把 task orchestration 变成并发噪音。

因此 control-plane 需要在启动前检查“相同 task identity + starter/plugin id + trigger profile”的 active run 是否已存在。实现上优先复用现有 `WorkflowRunStore`，通过 plugin-scoped recent run lookup 和 trigger payload 过滤完成，而不是先引入新的专用索引。

重复命中时的 verdict 不应伪装成成功；它需要以 `blocked` 或 `skipped` 的 machine-readable outcome 对外暴露。

**Alternative considered:** 不做 guard，默认允许同一 task 因多次 transition/retry 重复启动同一个 starter。  
**Rejected because:** 这会直接破坏“任务编排”的幂等性，也会让 workflow run history 很快失真。

## Risks / Trade-offs

- **[Risk] Trigger profile schema 扩展会波及 plugin manifest decode / validation** → **Mitigation:** 采用 additive 字段扩展并保留 `manual` 现有写法兼容，不要求所有 starter 一次性迁移到 task-driven profile。
- **[Risk] 只用 websocket + progress activity 不足以支撑长期 trigger 审计** → **Mitigation:** 本次只解决 canonical outcome truth 和 starter activation；若后续需要历史审计，再单开 change 复用 automation/log seam。
- **[Risk] Legacy action alias 兼容期过长，consumer 继续偷懒用旧名** → **Mitigation:** docs、DTO、panel consumer、tests 全部切到 canonical names，backend alias 只做读取兼容。
- **[Risk] plugin-scoped recent run filtering 可能不足以覆盖所有 duplicate edge cases** → **Mitigation:** 先对 official task-driven starters 做 deterministic guard，必要时在实现阶段补 store helper，但不提前设计成通用复杂索引。
- **[Risk] `review-escalation-flow` 是否支持 task-driven activation 仍存在真实依赖差异** → **Mitigation:** 把 starter availability truth 放进 manifest/profile 校验，未完成的 starter 明确返回 unavailable，而不是假装可触发。

## Migration Plan

1. 先更新 OpenSpec delta specs，锁定 task trigger canonical action、task-driven starter activation、以及 structured outcome contract。
2. 扩展 workflow/plugin manifest trigger profile 与 validation path，让 built-in starter 能声明 manual 或 task-driven availability。
3. 在 Go backend 中收敛 `TaskWorkflowService` 的 action normalization、starter validation、workflow runtime start seam 和 structured outcome shaping。
4. 更新 websocket payload、task progress source、task/workflow docs，以及 workflow config consumer 的 canonical trigger action vocabulary。
5. 回滚策略：若 task-driven starter activation 实现不稳定，可保留 canonical action normalization 与 richer outcome event，但暂时移除对应 starter 的 task-driven profile，让它回到 manual-only truth，而不必回滚整个 workflow runtime。

## Open Questions

- `start_workflow` 的 project workflow config 最终是否只允许引用 official built-in starters，还是允许 project-local workflow plugin id？当前倾向先支持任何通过 runtime 验证的 workflow plugin，但 first-party docs 只推荐 official starters。
- duplicate verdict 对外更适合归类为 `blocked` 还是 `skipped`？当前倾向：已有 active run 时用 `blocked`，manifest 声明不支持该 trigger profile 时用 `failed` 或 `blocked` with dependency reason，真正无动作匹配才用 `skipped`。
- task progress activity 是否需要新增明确的 `workflow_trigger` source 常量，还是复用现有 generic task transition source？当前倾向新增，避免把 orchestration activity 混进纯状态变化。
