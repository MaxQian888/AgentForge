## Why

`docs/PRD.md` 和 `docs/part/AGENT_ORCHESTRATION.md` 已经把 Go Orchestrator 的派发链路定义成一个完整控制面：任务分配或手动 spawn 进入统一 admission，随后完成 worktree / bridge 启动、成本治理，以及同步与实时反馈。但当前仓库真实实现仍停留在“部分链路已接通、整体控制面仍分裂”的状态：assignment dispatch、manual spawn、queue promotion、budget handling、IM/client contract 分散在不同层，已经做过的 change 解决了接线问题，却还没有把 Go 派发闭环补成一个对 Web、IM 和运维都真实一致的能力。

现在需要继续补这一层，是因为后续 dashboard、IM、team/role/runtime 等表面已经开始消费这些 dispatch 结果；如果不先收敛 Go 侧的权威 dispatch contract，队列、预算、blocked/queued 语义和非启动结果仍会在不同入口继续漂移。

## What Changes

- 补齐 Go 派发控制面，使 assignment-triggered dispatch、manual spawn、queue promotion 和 runtime cost enforcement 共享同一套任务中心化 outcome contract，而不是分别返回各自简化结果。
- 增加 dispatch budget governance，覆盖 task / sprint / project 三层预算的 admission preflight、promotion recheck、warning / exceeded 状态与 operator-visible feedback，不再只依赖运行中 task budget 的单点阈值判断。
- 对齐 queue 与 promotion 生命周期语义，让 queued 请求持续携带 runtime / provider / model / role / budget 上下文，并把 promotion failure 或 budget-blocked 结果作为控制面事实暴露出来，而不是退化成“未启动”。
- 对齐 Go API、WebSocket、IM client/commands 以及前端消费契约，使 `TaskDispatchResponse` 和相关实时事件能够稳定表达 `started`、`queued`、`blocked`、`skipped`、queue reference 与 reason，而不是由各个消费方自行降级或丢字段。
- 保持范围聚焦在 Go orchestration 及其紧耦合契约补全；本次不扩展到新的 UI 重做、模型供应商扩展、多 Agent planner 策略或额外产品功能线。

## Capabilities

### New Capabilities
- `dispatch-budget-governance`: 定义 Go 派发链路中的预算准入、运行中成本阈值、task / sprint / project 级联治理，以及与 dispatch lifecycle 对齐的预算反馈事实。

### Modified Capabilities
- `agent-task-dispatch`: 任务分配触发派发的 REQUIREMENTS 需要扩展为完整的 `started` / `queued` / `blocked` / `skipped` 合同，并保证 HTTP、WebSocket、IM 与前端消费者拿到同一份 dispatch truth。
- `agent-spawn-orchestration`: 手动 spawn 与 queued promotion 需要复用同一套 dispatch control-plane preflight、budget admission 和 structured non-started outcome，而不是绕过任务中心化派发语义。
- `agent-pool-control-plane`: 队列提升与运营态可见性需要纳入 dispatch guardrail revalidation，尤其是 budget-blocked / promotion-failed / still-queued 等结果不能丢失 admission history。

## Impact

- Affected Go backend code in `src-go/internal/service/task_dispatch_service.go`, `src-go/internal/service/agent_service.go`, `src-go/internal/service/cost_service.go`, `src-go/internal/cost/tracker.go`, `src-go/internal/pool/**`, related handlers, repositories, schedulers, and WebSocket event plumbing.
- Affected APIs and contracts around `POST /api/v1/tasks/:id/assign`, `POST /api/v1/agents/spawn`, pool summary / queue roster responses, `TaskDispatchResponse`, budget lifecycle events, and IM-facing reply formatting.
- Affected closely coupled consumers in `src-im-bridge/client/**`, `src-im-bridge/commands/**`, and frontend stores that currently consume dispatch outcomes, queue state, or budget realtime signals.
- Affected operational systems including queued admission visibility, promotion reconciliation, budget enforcement sources for tasks / sprints / projects, and the bridge-to-Go runtime cost propagation path.
