## Why

当前 `/team` 页面已经能做成员的基础增删改查，也补上了状态、IM 身份、技能和 agent profile 的结构化编辑；但它仍然更像“可编辑的成员表”，还不是一个真正可治理的团队管理面板。对训练员最直接的卡点是：发现一批成员需要暂停/恢复、筛出配置不完整的 Agent、集中处理 inactive/suspended 人员，仍要逐行点开编辑器手动处理，管理成本高，且缺少明确的批量反馈与注意事项收口。

现在补这条 seam，能把 PRD 里“统一看板管理碳基+硅基员工”的目标从“能看、能改”推进到“能治理、能收敛、能快速纠偏”，同时避免把需求误塞进 `enhance-frontend-panel` 这种范围过大的 umbrella change。

## What Changes

- 把 `/team` 从基础 roster 编辑页升级为面向运营治理的团队管理工作台，增加按成员状态、类型、readiness 风险聚焦问题成员的管理视图与摘要。
- 为团队 roster 增加多选与批量管理动作，至少覆盖 activate / inactive / suspend 这类 canonical availability 治理操作，并在批量执行后返回明确的成功/失败结果。
- 为缺少 runtime / provider / model / role 绑定的 agent 成员提供集中 attention 流，让训练员能从“待处理成员”入口直接跳到修复动作，而不是逐行扫描表格。
- 为单成员操作补齐更直接的治理控制（例如快速 suspend/reactivate、快速打开 setup-required 编辑、处理中禁用重复提交），减少必须进入完整表单才能完成常见管理动作的摩擦。
- 补齐团队治理相关的 focused tests，覆盖批量状态更新、attention 过滤、快速操作反馈，以及项目切换后治理视图不残留旧项目状态。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `team-management`: expand the team workspace from basic member CRUD into an operator-focused management surface with bulk governance controls, readiness triage, and faster member lifecycle actions.

## Impact

- Frontend routes/components: `app/(dashboard)/team/page.tsx`, `components/team/team-page-client.tsx`, `components/team/team-management.tsx`, and their focused tests.
- Frontend state/helpers: `lib/stores/member-store.ts`, `lib/dashboard/summary.ts`, `lib/team/agent-profile.ts`, plus any shared member-status helper needed by batch actions and attention filters.
- Backend/API: extend member-management support under project-scoped member routes so bulk availability updates can be persisted and return actionable per-member outcomes; likely touches `src-go/internal/server/routes.go`, `src-go/internal/handler/member_handler.go`, `src-go/internal/model/member.go`, and `src-go/internal/repository/member_repo.go`.
- Verification: focused Jest coverage for the team workspace and targeted Go tests for any new member bulk-update contract.
