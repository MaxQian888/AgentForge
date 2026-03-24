## Why

当前仓库已经有真实的 Agent `spawn -> worktree -> bridge` 启动链路，但“把任务派发给 Agent”仍然停留在碎片化状态：`/tasks/:id/assign` 只写入 assignee，`/agents/spawn` 仍需单独手动触发，而且两条路径没有统一复用任务当前 assignee、派发校验、失败回退和结果回写。PRD 已经把“任务分配给 Agent -> Orchestrator 选择/创建实例 -> worktree -> Bridge -> 实时反馈”定义为主链路，现在需要把这段链路补成可实现、可验证的正式能力。

## What Changes

- 新增一个以任务为中心的 Agent 派发能力，使“将任务指派给 agent 成员”能够进入统一的 dispatch 流程，而不是停留在单纯更新 assignee。
- 为派发流程定义准备检查与结果语义，包括 assignee 类型校验、任务/项目上下文读取、是否允许重复派发、派发失败后的状态回退，以及派发成功后的运行时元数据回写。
- 对齐 API、WebSocket、通知与 IM 命令语义，让任务指派、Agent 启动和派发失败能够通过同一套业务结果被前端与 IM 消费。
- 收敛现有手动 `spawn` 路径，使其复用同一套 dispatch orchestration，而不是要求调用方重复提供已经存在于任务上下文中的成员信息。

## Capabilities

### New Capabilities
- `agent-task-dispatch`: 定义任务指派给 Agent 后的统一派发流程、准备检查、执行结果、失败语义和可观测反馈。

### Modified Capabilities
- `agent-spawn-orchestration`: Agent 启动需要与任务派发流程对齐，复用任务上下文中的 assignee、运行时准备和补偿语义，而不是只支持孤立的手动启动入口。

## Impact

- Affected Go backend code in `src-go/internal/handler/task_handler.go`, `src-go/internal/handler/agent_handler.go`, `src-go/internal/service/task_service.go`, `src-go/internal/service/agent_service.go`, task/member repositories, and dispatch-related WebSocket/notification plumbing.
- Affected APIs around task assignment and agent startup, including how `POST /api/v1/tasks/:id/assign` and `POST /api/v1/agents/spawn` report dispatch outcomes and validation failures.
- Affected IM command flow in `src-im-bridge/commands/task.go`, `src-im-bridge/commands/agent.go`, and the AgentForge IM client contract so IM-triggered assignment and spawn can reuse the same orchestration semantics.
- Affected frontend/store consumers that currently treat assignment and agent runtime as separate actions, because dispatch results and lifecycle feedback become part of the task-level flow.
