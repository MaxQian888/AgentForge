## Why

AgentForge 的角色编辑与 preview/sandbox 工作区已经基本成形，但“角色定义完成后如何被团队成员、Agent 启动流、插件/工作流和历史运行安全消费”这条生命周期仍然不完整。现在补这条线，是因为仓库已经把 role 变成跨多个控制面的真实系统资产；如果继续只保证 authoring 成功，而不治理 stale role 引用、删除预检和运行前校验，角色管理仍会在真正执行时晚失败。

## What Changes

- 为角色管理引入统一的引用治理能力，定义角色当前被哪些成员、插件/工作流、排队中的执行请求和历史运行记录消费，以及这些引用里哪些会阻止删除、哪些只作为提示。
- 扩展角色页删除前反馈，让操作者在确认删除前就能看到完整下游消费者和阻塞原因，而不是只在删除请求失败后得到笼统报错。
- 补齐 team/member 侧的角色绑定治理：Agent 成员编辑和 roster 摘要要区分“未绑定角色”和“绑定了已失效角色”，并给出可操作的修复入口。
- 补齐 spawn/dispatch 前的角色有效性保护：无论 roleId 来自显式选择还是成员画像里的已绑定角色，只要引用已失效，都必须在运行启动前返回明确诊断，而不是等到运行时晚失败。
- 保留历史运行和审计可见性：历史 `agent_runs` 等记录继续保留已使用过的 role_id 作为审计上下文，但不再被当作可继续执行的有效绑定。

## Capabilities

### New Capabilities
- `role-reference-governance`: 定义角色引用清单、阻塞与非阻塞消费者分类、删除预检、以及 stale role 绑定的权威治理规则。

### Modified Capabilities
- `role-management-panel`: 角色工作区和角色库的删除/审查流需要显示完整下游引用与删除阻塞原因，而不是只展示局部插件消费者。
- `team-management`: Agent 成员的角色绑定需要校验权威 role registry，并在 roster 与编辑流中区分未绑定、可用绑定和已失效绑定。
- `agent-spawn-orchestration`: spawn 与 dispatch 入口需要在运行启动前拒绝失效角色绑定，并返回可操作的 preflight 反馈。

## Impact

- Affected frontend role surfaces: `app/(dashboard)/roles/page.tsx`, `components/roles/*`, and related role context or delete flows.
- Affected team/member surfaces: `components/team/*`, `lib/team/agent-profile.ts`, `lib/dashboard/summary.ts`, and `lib/stores/member-store.ts`.
- Affected spawn/dispatch surfaces: `components/tasks/spawn-agent-dialog.tsx`, related agent stores, `src-go/internal/handler/agent_handler.go`, and `src-go/internal/service/agent_service.go`.
- Affected backend governance seams: `src-go/internal/handler/role_handler.go`, member and queue repositories/services, plus role/plugin or workflow dependency coordination.
- No new external dependency is required; this change tightens existing contracts and diagnostics around current role consumers.
