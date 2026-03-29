## Why

AgentForge 的项目文档已经把团队成员定义成带有可协作身份的统一成员模型，而当前 live 实现仍停留在更窄的 `name/type/role/email/skills/agent_config/isActive` 合同。`docs/PRD.md` 与 `docs/part/DATA_AND_REALTIME_DESIGN.md` 明确承诺了成员状态三态、IM 身份字段和面向智能分配的成员元数据，但当前团队管理页面、成员 API、Go model/migration 与下游分配流还没有围绕这份文档合同闭环，导致“人机混合团队管理”与仓库文档真相继续漂移。

## What Changes

- Extend the canonical team member contract to cover the documented member availability model instead of a boolean-only `isActive` flag.
- Add documented collaboration identity fields for team members so team management can persist and display IM-facing member identity where the project docs already require it.
- Upgrade team management surfaces to create, edit, filter, and summarize members using the documented status/contact contract for both human and agent members.
- Align member-consuming flows that depend on member availability and skills, including assignment/recommendation paths, so suspended or otherwise unavailable members are not treated like ready collaborators.
- Tighten focused validation coverage around the member API, persistence mapping, team roster UI, and assignment/recommendation behavior affected by the documented contract alignment.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `team-management`: expand the team member contract and team roster/editing behavior to include documented member status and collaboration identity fields, plus downstream use of that canonical availability state.

## Impact

- Affected frontend surfaces: `app/(dashboard)/team/page.tsx`, `components/team/**`, `lib/stores/member-store.ts`, `lib/dashboard/summary.ts`, and task-assignment UI that consumes member availability.
- Affected backend/API surface: `src-go/internal/model/member.go`, `src-go/internal/handler/member_handler.go`, `src-go/internal/repository/member_repo.go`, member-related migrations, and any task assignment/recommendation code that assumes boolean activity only.
- Affected docs/specs: `openspec/specs/team-management/spec.md` and the documented member contract already described in `docs/PRD.md` and `docs/part/DATA_AND_REALTIME_DESIGN.md`.
