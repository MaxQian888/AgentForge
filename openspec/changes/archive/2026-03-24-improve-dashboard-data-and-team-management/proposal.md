## Why

当前 Dashboard 只有少量静态汇总卡片和占位中的活动区，无法体现 PRD 中强调的任务进度、成本、审查压力与团队协同信号；同时仓库虽然已经在 PRD 和数据设计里定义了 `members`、人机混合成员和项目级成员接口，但产品表面还没有可用的团队管理入口。现在补齐这两块，能让 AgentForge 的“统一看板管理碳基+硅基员工”从文档设想进入可实施、可验证的产品能力。

## What Changes

- 增强 Dashboard 首页的数据表达，提供面向任务、Agent、审查和成本的结构化概览，而不是只展示 4 个基础数字。
- 增加团队管理能力，支持在项目上下文中查看成员列表、区分 human/agent 成员、查看职责与状态，并为任务分配与协作提供统一成员入口。
- 为 Dashboard 和团队管理定义清晰的空状态、加载状态和基础导航入口，使首页、项目看板、Agent 监控与成员管理形成连贯流转。
- 为前端状态层和后端 API 合同补齐团队相关读取能力，确保 Dashboard 中的团队数据来自统一成员模型，而不是临时拼接。

## Capabilities

### New Capabilities
- `dashboard-insights`: Dashboard 提供可操作的项目、任务、Agent、审查和成本概览，并能展示近期活动与风险信号。
- `team-management`: 系统提供项目级团队成员管理视图，统一展示 human 与 agent 成员的身份、职责、状态和协作入口。

### Modified Capabilities
- None.

## Impact

- Affected frontend routes: `app/(dashboard)/page.tsx`, `app/(dashboard)/project/page.tsx`, `app/(dashboard)/agents/page.tsx`, sidebar/dashboard shell navigation.
- Affected frontend state: `lib/stores/task-store.ts`, `lib/stores/agent-store.ts`, and a new project member/team store derived from the documented member model.
- Affected API surface: project member listing/management endpoints described in `docs/PRD.md`, plus any dashboard aggregation endpoint or composition layer needed to drive richer首页数据.
- Affected product flow: dashboard landing experience, task assignment entrypoints, and project/team collaboration visibility.
