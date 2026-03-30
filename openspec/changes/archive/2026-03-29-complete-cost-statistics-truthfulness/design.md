## Context

当前仓库已经有多条“成本相关”真链路，但 standalone cost surface 并没有消费同一个权威合同：

- 前端 `app/(dashboard)/cost/page.tsx` 和 `lib/stores/cost-store.ts` 期待的是一个富聚合模型：`activeAgents`、`dailyCosts`、`sprintCosts`、`taskCosts`、带 `costUsd` 的 velocity points，以及扁平的 agent performance rows。
- 后端 `src-go/internal/handler/cost_handler.go` 现在对 `GET /api/v1/stats/cost?projectId=...` 只返回 `CostSummaryDTO` totals；`stats_handler.go` 返回的 velocity/performance 也分别是 `points`/`entries` 包装，且缺少当前前端真正使用的 `costUsd`、分钟级时长和稳定 display label 语义。
- `src-im-bridge/client/agentforge.go` 的 `/cost` 命令仍然把 `/api/v1/stats/cost` 当作另一套 snake_case summary schema 来消费，这和 Go 端现状同样不一致。
- 与此同时，仓库里又已经存在可复用的真实 seam：`dashboard_widget_service.go` 能计算 cost trend / agent cost breakdown，`budget_governance_service.go` 已提供项目预算摘要，`agent_runs` / `tasks` / `sprints` 已持久化 token、cost、spent、budget 等核心字段。

所以这次 change 的问题不是“缺一个成本系统”，而是“已有成本统计 surface 没有统一到真实、可复用、可验证的合同上”。

## Goals / Non-Goals

**Goals:**

- 为 standalone cost workspace 定义并落地一个权威的后端查询合同，覆盖项目总览、预算摘要、趋势、Sprint/任务明细、velocity 和 performance。
- 让 cost page 严格消费权威查询结果，不再用 `agent-store` 或错型 payload 为 headline 和 section 兜底。
- 让 lightweight consumer（尤其是 IM `/cost`）也能从同一套权威聚合得到可用摘要，而不是维持另一套漂移 schema。
- 最大化复用现有 `dashboard_widget_service`、`budget_governance_service`、`agent_run_repo`、`task_repo` 等 seam，而不是为 cost surface 新造平行统计实现。
- 为这条 surface 补齐 focused verification，证明前后端 shape、空态、错误态与兼容 consumer 都可验证。

**Non-Goals:**

- 不重做 `dispatch-budget-governance` 的 admission / runtime enforcement 语义，也不扩写新的预算治理主线。
- 不把本次 change 扩成新的项目 dashboard foundation、全量 BI/reporting 或更大范围的统计平台。
- 不引入新的 event-sourced cost ledger；首版仍基于当前已持久化的 run/task/sprint/project 数据工作。
- 不承诺“逐秒/逐 turn”的精确历史成本重放；当前仓库没有支撑这一点的持久化事实源。

## Decisions

### Decision: 用共享的 typed cost query seam 收敛 `/api/v1/stats/*`，而不是继续让页面直接对接分裂 handler

本次会把 standalone cost surface 所需查询收敛到一个共享的 typed cost query seam，由它统一组合项目级成本摘要、velocity、performance 与兼容摘要。`CostHandler` 不再只直连 `AgentRunRepository.AggregateByProject(...)`，`StatsHandler` 也不再暴露与前端错位的“半成品包装”而无人负责语义对齐。

这样做的原因是：

- 当前漂移的根因不是单个 endpoint 少字段，而是三个 route、两个前端 section、一个 IM consumer 各自理解不同。
- 现有 `dashboard_widget_service` 已经证明仓库里有可复用的聚合逻辑，但 widget payload 是 `map[string]any` 且配置驱动，不适合作为 standalone API 的权威合同。
- 用 typed service 收敛后，可以把 dashboard widget、cost page、IM `/cost` 的数据来源保持一致，但各自保留自己的 route / UI 形态。

备选方案 A 是只改前端 store，让它适配今天的后端 shape。拒绝原因：这只会把 `/stats/cost`、`/stats/velocity`、`/stats/agent-performance` 的漂移藏进前端 adapter，IM `/cost` 仍然是错的。备选方案 B 是让 cost page 直接复用 dashboard widget API。拒绝原因：widget contract 是配置驱动且非 typed，无法成为 standalone operator surface 的长期真相。

### Decision: `GET /api/v1/stats/cost?projectId=...` 升级为 rich summary，但仍保持现有 route surface

这次不新增一个“仅供页面使用”的新 route，而是在现有 `/api/v1/stats/cost` 上定义一个 richer project-scoped summary contract。原因是：

- `app/(dashboard)/cost`、IM `/cost`、以及 PRD 中对 `/api/v1/stats/cost` 的认知都已经指向这条 route。
- 保留 route surface 可以把变更集中在服务/DTO/consumer 上，而不是扩散到更多路由与权限接线。

新的 summary 会包含：

- 项目级 totals：cost / input / output / cache / turns / activeAgents。
- `dailyCosts`：基于当前持久化 run 记录导出的趋势数据。
- `sprintCosts` 与 `taskCosts`：供 standalone tables 直接消费的 breakdown。
- `budgetSummary`：复用现有 `BudgetGovernanceService.GetProjectBudgetSummary(...)` 的权威预算摘要。
- `periodRollups`：至少覆盖今日、最近 7 天、最近 30 天的成本汇总，供 IM `/cost` 等 lightweight consumer 直接使用。

备选方案是继续把 compact summary 和 rich summary 拆成两套 schema。拒绝原因：这会把当前最主要的漂移源永久化。

### Decision: Velocity 与 Performance 继续保留独立 endpoint，但 contract 明确对齐 cost workspace

`/api/v1/stats/velocity` 和 `/api/v1/stats/agent-performance` 继续作为独立 endpoint 存在，因为这能保留当前页面 section、store 刷新边界和后端组织方式。但它们的 contract 需要明确对齐 cost workspace：

- `velocity` 返回的 `points` 必须同时包含 `tasksCompleted` 和同一时间粒度下的 `costUsd`，这样 `VelocityChart` 才不再依赖虚构字段。
- `agent-performance` 返回的 `entries` 必须包含稳定标识、显示标签、成功率、平均成本、平均时长（分钟）与总成本；前端 store 负责做 wrapper 到组件 props 的一次性 normalization。

备选方案 A 是把这两个 endpoint 改成页面专用 plain array。没有采用，是因为当前 handler/service 已经有 wrapper 结构，保留 wrapper 更便于未来扩展 totals / metadata。备选方案 B 是继续返回今天的 shape 再由前端猜测。拒绝原因：这正是当前问题的一部分。

### Decision: Performance 首版按执行角色/执行画像聚合，而不是伪造“历史单个智能体”身份

仓库当前 `AggregatePerformance(...)` 是按 `role_id` 聚合，这比“每条 run 当作一个 agent”更接近可解释的稳定维度。`agent_runs` 目前并没有一个能跨历史 run、跨队列/重试稳定代表“同一个智能体”的 operator-facing identity，因此本次不会为了迎合现有文案去伪造 per-agent 历史。

因此首版选择：

- 后端 performance entry 返回稳定 bucket id + display label。
- 当前若没有更好元数据，允许 label 回退到 `roleId`。
- cost workspace 的文案与表头需要改成与该聚合语义一致，不再暗示“单个历史智能体”的精确身份。

备选方案是按 `member_id`、`run_id` 或当前运行时 session 拼一个“agentName”。拒绝原因：这些维度都不稳定，展示出来只会让统计更不真实。

### Decision: 趋势数据只承诺“基于当前持久化事实的 recorded trend”，不伪装成 event ledger

当前仓库没有单独持久化每次 `cost_update` 的历史 ledger，只有 run/task/sprint/project 上的累计结果。因此：

- `dailyCosts` 会基于现有持久化事实导出 recorded trend。
- `velocity.costUsd` 会与同时间窗口内完成任务的成本事实保持一致，而不是前端再拼一套估算。

这比当前空数组/错型字段/派生 fallback 更真实，但仍不代表“逐日精确回放每一次 runtime 花费”。文案与 spec 都应该避免超出当前数据能力的承诺。

## Risks / Trade-offs

- [当前仓库没有 event-sourced cost ledger，daily trend 只能基于现有持久化事实导出] -> 在 spec 与 UI 上明确这是 recorded trend，不伪装成逐笔成本流水；未来若需要更细颗粒度，再单开 change 补 ledger。
- [共享 typed query seam 组合 run/task/sprint/budget 数据，容易在多个 DTO 间再次漂移] -> 用一个服务层统一装配 summary/velocity/performance，并补 handler + service + consumer tests。
- [Performance 语义从“Agent”收紧到“执行角色/执行画像”会触发前端 copy/test 变更] -> 同步更新 i18n、组件断言和空态文案，不保留误导性命名。
- [IM `/cost` 兼容如果只靠隐式字段约定，未来仍会再次漂移] -> 通过 `periodRollups` / `budgetSummary` 这类明确字段或显式 adapter 收口，并用 IM client/command tests 固化。

## Migration Plan

1. 在 Go 侧引入共享的 cost query DTO/service，组合 `agent_run_repo`、task/sprint reader、`budget_governance_service` 和现有 widget 可复用逻辑。
2. 更新 `/api/v1/stats/cost`、`/api/v1/stats/velocity`、`/api/v1/stats/agent-performance` handler，使 route surface 保持不变但响应 shape 与语义对齐。
3. 更新 `lib/stores/cost-store.ts` 与 `app/(dashboard)/cost/page.tsx`，去掉 `agent-store` headline fallback，并按真实 wrapper/entry 语义渲染。
4. 视最终 contract 调整 `src-im-bridge/client/agentforge.go` 与 `/cost` 命令，使 lightweight summary 从同一套聚合读取。
5. 运行 focused backend + frontend + IM tests，确认 shape、空态、错误态与兼容 consumer 都通过。

回滚方式是整体回退这一组 handler/store/client 变更；本次不引入新持久化表，因此不存在额外数据迁移回滚成本。

## Open Questions

- `dailyCosts` 首版应该按哪个既有时间维度导出 recorded trend：`created_at` 还是“最后一次持久化 cost 的时间维度”？当前建议先用仓库里最稳定、最容易测试的既有持久化维度，并在 spec 中避免暗示逐笔回放。
- 若 role metadata 可用，performance label 是否应优先显示角色名称而不是 `roleId`？当前建议支持 label enrichment，但保证 `roleId` 回退路径始终成立。
- `periodRollups` 是否直接作为 rich summary 的一部分暴露，还是由 IM adapter 本地从 `dailyCosts` 计算？当前建议服务端直接暴露，避免多 consumer 重复实现窗口计算逻辑。
