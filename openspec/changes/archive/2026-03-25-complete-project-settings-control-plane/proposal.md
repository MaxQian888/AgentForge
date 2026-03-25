## Why

当前 `app/(dashboard)/settings/page.tsx` 只覆盖项目基础信息、仓库地址和 coding-agent 默认值，和 PRD 中已经明确要求的项目级治理配置存在明显断层：预算阈值、审查升级/人工审批、以及面向操作者的完整设置摘要都没有进入统一前端控制面。现在补齐这条设置主线，可以把 AgentForge 的“成本可控、审查可控、配置可见”从分散文档约定收敛成真实可操作的产品表面，而不是继续停留在只改少量 JSON 字段的半成品状态。

## What Changes

- 将当前项目设置页升级为完整的项目级 settings control plane，统一展示并编辑项目基础信息、仓库元数据、coding-agent 默认值、预算与告警阈值、审查升级/人工审批策略，以及相关的操作者摘要。
- 为项目设置引入结构化持久化模型，而不是继续只支持 `settings.codingAgent`；前后端都需要能返回和保存完整的项目设置文档。
- 为设置页增加“信息显示完整”的摘要与诊断视图，让操作者在保存前后都能看到当前 runtime readiness、预算治理状态、审查策略和关键 fallback/风险提示。
- 保持现有 coding-agent runtime catalog 约定不丢失，并让它在新的统一设置保存流程里继续作为正式配置区块存在。
- 让项目级审查策略能驱动现有深度审查/人工审批链路，而不是只在 PRD 中存在“可配置”描述。

## Capabilities

### New Capabilities
- `project-settings-control-plane`: 定义项目级设置页面如何完整展示、编辑和持久化治理配置，包括基础信息、仓库、coding-agent、预算/告警、审查策略和操作者摘要。

### Modified Capabilities
- `coding-agent-provider-management`: 将 coding-agent 默认值与 runtime diagnostics 纳入统一项目设置控制面，并要求 unified save/load 流程保持 catalog 解析与默认选择语义不变。
- `deep-review-pipeline`: 让项目设置中声明的人工审批与审查升级策略成为深度审查路由的一部分，而不是固定规则之外的文档备注。

## Impact

- Frontend: `app/(dashboard)/settings/page.tsx`, `app/(dashboard)/settings/page.test.tsx`, `lib/stores/project-store.ts`, settings-related UI sections/components.
- Backend/API: project settings DTO/model/repository/service and `PUT /api/v1/projects/:id` payload shape.
- Review/runtime governance: project-level review routing inputs and project-scoped budget/governance summaries consumed by operators.
- Documentation/spec alignment: PRD-defined project governance knobs become repo-truthful, apply-ready product contracts instead of scattered references.
