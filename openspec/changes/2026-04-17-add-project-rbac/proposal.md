## Why

AgentForge 目前的成员模型（`member.type = human | agent`）只记录"是谁"，没有记录"能在这个项目里做什么"。任何通过项目作用域 API 进入的已认证用户，理论上都能对该项目的任务、成员、设置、workflow、team run 做任意写入。agent 成员的动作（dispatch、team run、workflow 执行）也没有回溯到"发起的是哪个 human"的权限约束，使 agent 可以成为绕过人类权限的通道。

随着多人协作进入真实使用，必须让项目具备明确的角色分层，并把 agent 动作同样纳入到**发起者 human 的 projectRole** 约束之下。

## What Changes

- 引入项目级人类角色 `projectRole`：`owner | admin | editor | viewer`，四级，不可自定义。
- 在 `member` 记录上新增 `projectRole` 字段，覆盖 human 和 agent 两种成员（agent 成员保留该字段仅作 display/filter 用途，其运行时权限不参考自身 projectRole）。
- 项目创建者自动成为该项目的 `owner`；member 新建默认 `editor`；末位 `owner` 不可被降级或移除。
- 新增后端 RBAC 中间件 + 集中式 action→roles 映射矩阵；所有项目作用域写操作必须通过矩阵校验。
- agent 动作（task dispatch、team run start/retry、workflow execute、automation trigger-by-human）在入口处解析"发起者 human"并用其 `projectRole` 做校验，而非 agent 自身角色。
- 前端 store 暴露当前用户在当前项目的 `projectRole`；UI 按角色隐藏或禁用无权动作，并在后端仍兜底拒绝。
- 破坏性：`MemberDTO` 必含 `projectRole`；member 创建请求必须接受 role；前端 dispatch/run-start 入口新增 role 校验失败路径。无兼容层（API 稳定期自由破坏）。

## Capabilities

### New Capabilities
- `project-access-control`：定义 projectRole 分类、action→roles 映射矩阵、RBAC 中间件契约，以及 agent 动作必须用发起者 projectRole 校验的规则。

### Modified Capabilities
- `team-management`：member 契约扩展 `projectRole` 字段、末位 owner 保护、role 变更的授权约束。
- `project-management-api-contracts`：所有项目作用域读写 API 在 ownership 校验之后必须再经 RBAC action 校验。
- `agent-task-dispatch`：dispatch 入口必须解析发起者 human 并用其 projectRole 校验。

## Impact

- 后端 model/repo/handler：`src-go/internal/model/member.go`, `src-go/internal/repository/member_repo.go`, `src-go/internal/handler/member_handler.go`, `src-go/internal/handler/project_handler.go`（项目创建自动设置 owner）。
- 后端 middleware（新增）：`src-go/internal/middleware/rbac.go`（action→roles 矩阵 + 解析当前用户 projectRole）；`src-go/internal/middleware/project.go`（串联 RBAC 到 projectGroup）。
- 后端 service 入口：`src-go/internal/service/agent_service.go`, `src-go/internal/service/team_service.go`, `src-go/internal/service/dispatch_preflight.go`, `src-go/internal/service/automation_engine_service.go`, `src-go/internal/service/dag_workflow_service.go`（或 workflow_execute 所在路径），每个 agent/workflow 动作入口接收并校验 initiator 的 projectRole。
- 数据库迁移：新增 `members.project_role` 列（默认 `editor`），项目创建时回填首位成员为 `owner`。
- 前端 store/hook：`lib/stores/member-store.ts`（MemberDTO 扩展）, `lib/stores/auth-store.ts`（暴露当前用户在当前项目的 role），新增 `hooks/use-project-role.ts`。
- 前端 gate：`components/team/team-management.tsx`, `components/team/invite-member-dialog.tsx`, `components/team/edit-team-dialog.tsx`, `components/team/start-team-dialog.tsx`, `components/tasks/dispatch-preflight-dialog.tsx`, `components/tasks/spawn-agent-dialog.tsx`, `components/project/edit-project-dialog.tsx`, `components/workflow-editor/**`（按 role 隐藏/禁用写操作）。
- 相关 DTO：`/api/v1/projects` 创建响应、`/api/v1/projects/:pid/members` 列表+创建+更新、`/api/v1/projects/:pid/tasks/**` 写入与 dispatch 路径、`/api/v1/teams/**` 启动与重试入口、workflow execute 与 automation trigger 入口。
