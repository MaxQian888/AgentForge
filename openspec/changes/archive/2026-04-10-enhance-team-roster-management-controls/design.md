## Context

当前 `components/team/team-management.tsx` 已经具备项目切换、成员搜索、type/status 筛选、create/edit/delete、agent readiness 提示和 workload drill-down，但管理动作仍停留在“逐行编辑”层级。真实操作痛点有三类：

1. roster 没有多选和批量治理能力，遇到一批成员需要 suspend/reactivate 或统一整理状态时，只能逐个打开编辑表单。
2. setup-required、inactive、suspended 这些真正需要训练员处理的成员只是散落在表格里，没有一个集中 attention 入口来快速收敛问题。
3. 常见单成员治理动作仍然要进入完整编辑态，缺少快速 suspend/reactivate / open-fix-flow 之类的轻量控制，也没有明确的 in-flight 反馈来避免重复提交。

这条 change 需要同时改前端 team workspace、member store 和 Go member API contract，属于跨模块的治理能力增强，适合先写 design 锁定边界。

## Goals / Non-Goals

**Goals:**
- 让 `/team` 成为可治理的 roster workspace，而不只是可编辑的成员表。
- 支持项目作用域内的批量 availability 治理，至少覆盖 `active` / `inactive` / `suspended` 三态切换。
- 给训练员一个清晰的 attention 入口，快速定位 setup-required agent、inactive 成员和 suspended 成员。
- 为单成员常用治理动作提供更快的入口，同时保留现有完整编辑表单作为深度修复路径。
- 用 focused Jest / Go tests 锁定批量治理、attention 过滤、快速动作反馈和项目切换后的状态清理。

**Non-Goals:**
- 不重做 `/teams` / `/teams/detail` 的 team run 页面，也不扩展 team strategy / runtime contract。
- 不引入组织权限矩阵、审批流、邀请系统或跨项目成员目录。
- 不实现任意字段的批量编辑；首轮只覆盖 availability 治理和 attention/remediation 流。
- 不新增新的 dashboard 聚合接口；仍复用当前 member roster + dashboard summary enrich 路径。

## Decisions

### Decision: 为项目成员治理新增显式 bulk-update contract，而不是在前端循环调用单成员 `PUT /members/:id`

新增一个 project-scoped member bulk action contract（例如 `POST /api/v1/projects/:pid/members/bulk-update`），请求体包含 `memberIds` 与目标治理动作（首轮为 status 变更），响应返回每个成员的结果项与失败原因。前端执行批量治理后统一刷新当前项目成员列表与 summary。

这样做的原因：
- 批量治理需要一次操作拿到完整结果，便于在 UI 中反馈“哪些成员成功、哪些失败”。
- 比起前端 `Promise.all` 多次调用单成员 update，更容易保证 project scope、校验和错误汇总一致。
- 后端测试可以直接锁定 bulk contract，而不是把“批量行为”拆散到多个单条请求里间接验证。

备选方案是继续复用 `PUT /api/v1/members/:id` 并在前端串行/并行发多次请求。这个方案实现快，但错误汇总、部分失败反馈、重复点击保护和 project-scope 约束都会分散到前端，不采用。

### Decision: bulk management 首轮只做 canonical availability 治理，不开放任意字段 mass edit

批量能力首轮只支持 `active` / `inactive` / `suspended` 三态治理，不支持批量改角色、技能、agent profile 或 IM 身份。角色绑定、runtime/provider/model 修复仍通过单成员编辑流完成。

这样做的原因：
- 用户当前最明确的管理诉求是“快速治理成员可用性”和“集中处理异常成员”，availability 是最直接、最安全的切口。
- status 是已有 canonical contract，前后端都已支持，扩成批量动作的改造面最小。
- 批量修改 role/agent profile 会引入复杂的冲突和校验语义，容易把 focused change 膨胀成“批量表单系统”。

备选方案是做通用 bulk edit 面板。这个方案灵活，但范围和验证成本都明显更高，因此不采用。

### Decision: attention workspace 基于现有 roster 派生，不新增独立数据源

attention 区域由当前 team roster 派生，聚焦三类成员：
- setup-required 的 agent 成员；
- `inactive` 成员；
- `suspended` 成员。

前端在 `TeamPageClient` 拿到 enriched roster 后派生 attention counts、attention filter、默认 quick action 文案；不新增单独的 attention API 或新的持久化表。

这样做的原因：
- 这些信号已经存在于当前 `TeamMember` + readiness 计算结果中，不需要再创建一条平行读路径。
- 维持项目切换时“一次 roster 刷新，所有治理视图同步更新”的行为，更容易保证状态一致。
- 对 focused change 来说，这比引入额外 backend summary endpoint 风险更低。

备选方案是新增 `/team/attention` 专用 API。该方案会带来额外接口维护成本，而且当前没有必要，不采用。

### Decision: 深度修复继续复用现有编辑器，快速治理通过 row action / attention CTA 进入

`components/team/team-management.tsx` 保留现有 create/edit form 作为完整编辑器；新增的管理体验通过三类入口提供：
- roster 多选后的 bulk action toolbar；
- row-level quick actions（如 suspend/reactivate）；
- attention card / setup-required CTA 直接打开带高亮字段的编辑态。

这样做的原因：
- 现有 edit flow 已经支持 role binding、runtime/provider/model、IM 身份、skills 等深度修复，没有必要再维护一套并行表单。
- 训练员在治理时通常先需要“定位问题 + 快速处理”，只有复杂个案才需要进入完整编辑器。
- 复用已有 edit form 能显著降低 UI 与测试重复度。

备选方案是新增独立 side panel 或专门的 remediation wizard。这个方案视觉上更完整，但对当前 focused seam 来说过重，不采用。

## Risks / Trade-offs

- [Bulk update 存在部分成功、部分失败的结果] → 响应必须返回 per-member outcomes，前端展示明确结果并在刷新后保留失败上下文。
- [attention 规则来自前端派生，若 readiness 规则未来变化可能出现漂移] → 继续复用 `lib/team/agent-profile.ts` / roster normalization 的单一 readiness helper，避免在 UI 再写一份规则。
- [项目切换时 selection 或 attention filter 残留会误导治理对象] → 在 project change 后重置 selection、bulk toolbar 状态和非 URL 持久化的 attention 过滤。
- [新增 bulk API 会扩大 member handler/repository 的写路径] → 用 focused Go tests 锁定 request validation、status normalization 和 mixed-result behavior，避免回归现有单成员 CRUD。

## Migration Plan

1. 在 `team-management` delta spec 中新增 bulk governance 与 attention workflow 的需求场景。
2. 为 Go member 层增加 bulk-update request/response 模型、handler、repository/service support 与 targeted tests。
3. 扩展 `lib/stores/member-store.ts` 支持 bulk action 调用与 per-member outcome normalization。
4. 在 `TeamPageClient` / `TeamManagement` 中加入 attention summary、多选状态、bulk toolbar、quick actions 与 project-switch cleanup。
5. 补齐 focused Jest tests；若 bulk API 或 UI 回归，先回退 bulk toolbar/attention 入口，再保留现有单成员编辑流，确保 roster 基本可用。

## Open Questions

- bulk action 的结果反馈是采用 toast + inline summary，还是只在 roster 顶部显示一次结果条？本次 design 倾向“顶部 inline summary + disabled pending state”，实现时可按现有通知体系再细化。
- 是否需要把 `setup-required` attention 默认为首次进入 `/team` 的高优先筛选？本次先不强制默认切换，只提供显式 attention 快捷入口。
