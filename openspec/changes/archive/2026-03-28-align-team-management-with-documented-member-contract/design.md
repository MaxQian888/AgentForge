## Context

`docs/PRD.md` 与 `docs/part/DATA_AND_REALTIME_DESIGN.md` 已经把成员合同定义成项目级统一成员模型：成员既有 `human|agent` 类型，也有 `status=active|inactive|suspended`、`im_platform`、`im_user_id`、`skills` 与 `agent_config` 等协作字段。当前 live repo 里的团队管理能力只实现了这份合同的一个子集：

- `src-go/migrations/002_create_projects_members_sprints.up.sql`、`src-go/internal/model/member.go` 和 `src-go/internal/repository/member_repo.go` 仍以 `is_active BOOLEAN` 为主，没有文档里的三态状态和 IM 身份字段。
- `components/team/team-management.tsx`、`lib/stores/member-store.ts` 和 `lib/dashboard/summary.ts` 只支持 active/inactive 的布尔语义，团队页也没有 IM 协作身份的 round-trip。
- `src-go/internal/service/assignment_recommender.go` 以及相关任务分配入口仍把“可分配成员”近似成 `isActive=true`，无法表达 `suspended` 这类“仍存在于团队，但不应被推荐或启动”的状态。

这次 change 不是新增团队产品面，而是把当前成员合同补到与项目文档一致，并让依赖该合同的团队页与分配入口消费同一份 availability 真相。

## Goals / Non-Goals

**Goals:**

- 为成员模型引入文档一致的 canonical `status` 与 IM 身份字段，并形成前后端 round-trip。
- 让团队管理页面可以创建、编辑、显示、筛选这份文档一致的成员合同，而不是继续停留在布尔启用开关。
- 让任务推荐/分配等依赖“成员可用性”的入口消费 canonical status，避免 suspended 成员被当成 ready collaborator。
- 通过兼容性设计减少当前 checkout 中其他活跃 change 受到的破坏。

**Non-Goals:**

- 不实现 IM 平台通讯录同步、组织架构导入或成员邀请工作流。
- 不重做 team run orchestration、role authoring 或 agent runtime provider 选择逻辑。
- 不把任务分配算法扩展成新的评分体系；本次只收敛成员 availability 输入合同。
- 不在这次 change 中清理所有旧字段历史包袱到零；兼容层可以保留到后续集中清理。

## Decisions

### 1. `status` 成为 canonical availability，`isActive` 仅作为兼容派生语义保留

采用文档一致的三态 `status`（`active|inactive|suspended`）作为成员可用性的单一真相源，而不是继续把 availability 建模成布尔值。为降低现有前端 store、dashboard 聚合和未完成 active changes 的破坏范围，本次实现应保留 `isActive` 兼容输出，但由 canonical `status` 派生，而不是继续让两套状态各自独立演化。

不直接继续扩展 `isActive` 的原因：

- 文档已经明确需要三态状态，布尔值无法表达 suspended 这种“成员仍在团队中，但不可参与协作”的语义。
- 推荐/分配/治理相关逻辑继续依赖布尔值，只会把 suspended 退化成 inactive 或 active 之一，丢失产品意图。

### 2. IM 身份字段保持扁平 API 形状，直接映射文档列名语义

成员 API 与前端 draft/store 应新增 `imPlatform`、`imUserId` 这类扁平字段，并由后端直接映射到持久化层。这样更贴近 `docs/PRD.md` 与数据设计里的 `im_platform` / `im_user_id` 合同，也更容易在团队页表单、列表摘要、唯一索引和校验逻辑里保持一致。

不采用嵌套 `imIdentity` JSON 对象的原因：

- 当前成员 API 已经以扁平字段为主，嵌套对象会额外引入前后端序列化和部分更新复杂度。
- 底层 schema 与唯一索引都是按列建模，扁平映射更容易避免“文档是列，API 是 blob”的再次漂移。

### 3. 团队管理 UI 在现有 member-type-aware 表单上增量补齐状态与协作身份

现有 `components/team/team-management.tsx` 已经区分 human/agent 创建与编辑流，因此本次不重做整个团队页，而是在现有类型感知表单上增量加入：

- canonical status 选择与筛选；
- human/agent 共用的 IM 平台与用户 ID 字段；
- roster 中对状态与 IM 身份的展示；
- 对 suspended / inactive 成员的明确文案与操作入口。

这样可以复用现有 agent profile 编辑能力，避免把一次“成员合同对齐”膨胀为新的工作台重构。

### 4. 下游 recommendation / assignment 通过共享 availability helper 消费 canonical status

任务推荐、手动分配和 agent dispatch preflight 等入口不再各自推断 availability，而是通过共享 helper 判断成员是否：

- 可见但不可协作（inactive / suspended）；
- 可分配为 human collaborator；
- 可作为 active agent dispatch target。

这能避免同一成员在团队页显示“Suspended”，但在推荐列表里仍被当作可用候选的合同分裂。

## Risks / Trade-offs

- [Dual-field drift between `status` and `isActive`] -> 以 `status` 为 canonical 输入，集中派生 `isActive`，并通过 focused tests 锁定 DTO/store/repository 映射。
- [New IM unique index may reject duplicate bindings in dirty data] -> 迁移阶段先清晰定义 backfill/validation 规则，创建或更新时返回可操作错误，不在 UI 静默吞掉冲突。
- [Active changes may still read `isActive`] -> 在本次 change 中保留兼容字段和 helper，先让消费端平滑迁移，再考虑后续集中移除。
- [Status semantics may sprawl into unrelated workflows] -> 本次只收敛团队成员、推荐、分配入口，不把组织权限、审批或通知策略一并纳入。

## Migration Plan

1. Add member persistence columns for canonical `status`, `im_platform`, and `im_user_id`, and backfill `status` from existing `is_active` data.
2. Update Go model/repository/handler DTOs so reads and writes round-trip the new fields while still deriving compatibility `isActive`.
3. Update frontend member store, summary normalization, and team management UI to consume/display canonical status plus IM identity.
4. Update recommendation/assignment availability checks to use the shared canonical status helper.
5. Run focused backend/frontend verification for member CRUD, roster rendering/filtering, and recommendation behavior.

Rollback can keep the old `is_active` behavior by ignoring the new fields at the handler/UI layer if needed; the additive schema change does not require immediate destructive column removal.

## Open Questions

- None that block proposal generation. UI copy for status badges and IM identity labels can be finalized during apply without changing the contract defined here.
