## Why

当前 `app/(dashboard)/cost` 已经是一个独立的成本统计入口，但它消费的前端 contract 与 Go 后端真实返回并不一致：`/api/v1/stats/cost` 目前只返回基础 totals，而页面与 `cost-store` 却期待 `activeAgents`、`sprintCosts`、`taskCosts`、`dailyCosts`、velocity cost 数据和扁平的 agent performance 记录。结果是这个页面只能依赖空数组、错误形状响应，甚至回退到 `agent-store` 的派生值来凑总花费与活跃 Agent，导致“成本统计”表面不再是系统内权威、可验证的真实数据入口。

现在需要补齐这条线，是因为仓库里已经存在真实的成本链路和部分可复用聚合：TS Bridge 会发送 `cost_update` 事件，Go 会持久化 `agent_runs` 的 token/cost 字段，IM `/cost` 命令可调用 `/api/v1/stats/cost`，project dashboard widget 也已经有 cost trend 与 agent cost breakdown 的聚合逻辑。但这些 consumer 当前对同一路由的理解并不一致，Standalone page 与 IM `/cost` 都没有消费一个真正权威的合同。如果继续维持这套漂移 contract，用户看到的统计将长期与现有 runtime、budget、dashboard 和 IM 面不一致。

## What Changes

- 为 standalone 成本统计补一个权威查询合同，覆盖项目级总花费、token/turn 汇总、活跃运行数、按日成本趋势、按 Sprint 成本对比、按任务成本明细，以及供 operator 直接消费的 velocity / agent performance 数据。
- 为现有 IM `/cost` consumer 保留一个可兼容的摘要视图或等价字段投影，使消息端看到的总花费、预算和周期统计来自与 cost workspace 相同的权威聚合，而不是另一套漂移 schema。
- 让 `app/(dashboard)/cost` 和 `lib/stores/cost-store.ts` 改为严格消费权威后端返回，不再用 `agent-store` 的派生 cost/activeAgent 数据为主，也不再依赖与后端不一致的 payload 形状。
- 复用或收敛现有 `dashboard_widget_service`、`agent_run_repo`、`stats_service` 等已存在聚合 seam，避免为 cost page 再造一套平行统计实现。
- 为 `/api/v1/stats/cost`、`/api/v1/stats/velocity`、`/api/v1/stats/agent-performance` 以及 cost page/store 补齐 focused tests，确保项目级查询、空状态、错误状态和 shape 对齐都可验证。

## Capabilities

### New Capabilities
- `cost-query-api`: 定义 standalone 成本统计所需的权威查询接口与返回合同，覆盖项目级成本汇总、时间趋势、任务/Sprint 明细，以及与 cost workspace 对齐的 velocity / agent performance 统计。
- `cost-operator-workspace`: 定义 cost 页面如何只展示权威成本数据、如何处理空/错/加载状态，以及如何避免回退到无关 store 或占位派生值。

### Modified Capabilities
- None.

## Impact

- Frontend: `app/(dashboard)/cost/page.tsx`, `lib/stores/cost-store.ts`, `components/cost/*`, cost page/store tests.
- Backend API: `src-go/internal/handler/cost_handler.go`, `src-go/internal/handler/stats_handler.go`, associated route contracts under `/api/v1/stats/*`.
- Backend services/repositories: `src-go/internal/service/stats_service.go`, `src-go/internal/service/dashboard_widget_service.go`, `src-go/internal/repository/agent_run_repo.go`, and related DTOs in `src-go/internal/model`.
- Operator surfaces and integrations that rely on truthful cost data: standalone cost page first, with reuse alignment against existing dashboard widgets and IM `/cost` expectations.
- IM bridge compatibility: `src-im-bridge/client/agentforge.go`, `src-im-bridge/commands/cost.go`, and related tests if the canonical cost response shape changes.
