## Why

AgentForge 现在已经有 `/team`、`/teams`、成员编辑表单和 team run 详情页，但关键工作流还没有真正闭环：成员 drill-down 链接不能在目标页保留上下文，`/teams` 与后端项目作用域契约不一致，team 详情页缺少稳定的加载/错误体验，前端 team strategy 值也和后端实际支持的策略集合发生漂移。这个缺口让“统一管理碳基成员、硅基成员和 agent team 执行流”停留在半成品状态，必须先补齐才能让团队管理表面真正可用。

## What Changes

- 完成团队管理中的成员调查链路，让成员工作负载、项目任务、Agent 活动和相关 drill-down 页面共享可消费的筛选上下文，而不是只生成无效查询参数。
- 收敛 `/team` 页的数据来源与状态管理，确保项目级成员视图使用明确的项目作用域、稳定的加载/错误反馈，以及与成员写操作一致的刷新路径。
- 完成 `/teams` 与 team detail 的项目作用域、加载、错误和重试体验，让 agent team 列表与详情页能够在真实 API 契约下稳定工作。
- 对齐前端 team startup / team run 展示与后端策略、runtime、provider、model 合同，消除 `strategy` 默认值和展示模型的漂移，并补足 queue / retry / phase 状态表达。
- 补齐团队管理相关的路由参数消费和关键测试覆盖，确保现有导航入口反映真实可继续操作的管理流，而不是占位跳转。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `team-management`: tighten the team roster, member drill-down, team run collection/detail, and startup contract requirements so the current surfaces behave as a complete project-scoped management flow.

## Impact

- Affected frontend routes: `app/(dashboard)/team/page.tsx`, `app/(dashboard)/teams/page.tsx`, `app/(dashboard)/teams/detail/page.tsx`, `app/(dashboard)/agents/page.tsx`, and `app/(dashboard)/project/page.tsx`.
- Affected frontend components/state: `components/team/**`, `components/dashboard/**` links that target team surfaces, `lib/stores/team-store.ts`, `lib/stores/member-store.ts`, `lib/stores/dashboard-store.ts`, and any filter/query-param plumbing across task and agent views.
- Affected backend/API surface: `GET /api/v1/teams`, `GET /api/v1/teams/:id`, `POST /api/v1/teams/start`, team summary DTOs, and any route/query handling needed to keep project-scoped team views consistent.
- Affected product flow: project-level member investigation, agent-team monitoring, team retry/cancel follow-up, and runtime-strategy selection/display across team startup and execution.
