## Why

AgentForge 已经具备 Role YAML、团队成员模型和基础管理页面，但前端里的 `Roles` 与 `Team Management` 仍停留在“能看、能做最小 CRUD”的阶段，和 `docs/PRD.md`、`docs/part/PLUGIN_SYSTEM_DESIGN.md` 中“数字员工可定制、可编辑、可复用”的产品目标存在明显断层。现在补齐角色与工程师配置面，才能把后端已经存在的角色 schema、`agent_config` 入口和人机混合团队模型真正变成可操作的产品能力，而不是继续让用户靠猜字段或手工拼接 JSON。

## What Changes

- 将角色页升级为结构化角色管理面板，覆盖 PRD/Role YAML 的核心字段分区编辑、模板起步、继承配置、版本与执行约束摘要，而不是只编辑少量基础字段。
- 为角色编辑增加“执行画像预览”与保存前校验反馈，帮助操作者在提交前看清 system prompt、工具约束、预算/回合上限、权限模式与路径限制等关键结果。
- 将团队页里的 Agent 成员创建/编辑能力升级为结构化“工程师画像”编辑流，覆盖技能标签、角色绑定、Agent profile/`agent_config`、激活状态，以及面向运行的关键默认项与摘要。
- 让 Team roster 能直接展示 Agent 成员的角色来源、关键执行约束与配置就绪度，而不再只显示名字、类型和一个笼统的 role 字符串。
- 补充前后端合同与测试，确保成员 API 能返回并更新前端真正需要的 agent profile 字段，而不是前端有类型占位、后端有存储入口、UI 却无法完整使用。

## Capabilities

### New Capabilities
- `role-management-panel`: 定义前端如何以结构化方式创建、编辑、预览和比较角色配置，包括模板/继承起步、执行约束摘要与保存前校验反馈。

### Modified Capabilities
- `team-management`: 团队管理需要从基础 roster CRUD 扩展为可配置的人机混合成员管理，明确 Agent 成员的画像编辑、角色绑定、配置摘要与就绪度展示要求。

## Impact

- Affected frontend pages and components: `app/(dashboard)/roles/page.tsx`, `components/roles/*`, `app/(dashboard)/team/page.tsx`, `components/team/*`
- Affected client stores and types: `lib/stores/role-store.ts`, `lib/stores/member-store.ts`, `lib/dashboard/summary.ts`
- Affected backend contracts: `src-go/internal/model/member.go`, `src-go/internal/handler/member_handler.go`, `src-go/internal/repository/member_repo.go`, and related member tests/DTOs
- Affected docs and validation scope: role/team management specs, focused frontend tests for role editing and agent member profile flows
