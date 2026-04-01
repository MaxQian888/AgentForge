## Context

AgentForge 当前已经有一条可工作的成本统计链路: TS Bridge 运行 Claude Code / Codex / OpenCode 等 runtime，向 Go 发 `cost_update`；Go 把 `agent_runs` 的 token/cost 字段持久化；`CostQueryService` 再把这些 persisted runs 聚合成 `/api/v1/stats/cost`、`/api/v1/stats/velocity`、`/api/v1/stats/agent-performance`。`complete-cost-statistics-truthfulness` 已经把 cost workspace 从“前端猜数据”修到了“消费权威查询合同”。

但现在新的问题出在更底层的 accounting truth 上，而不是 cost page 合同本身:

- `src-bridge/src/cost/calculator.ts` 只有少量 Claude 旧模型键，Codex/OpenAI 没有自己的价格目录，未知模型会错误回退到 Claude Sonnet 定价。
- `AgentRuntime` 只维护 `spentUsd`，没有统一维护累计 token totals、计费来源、cache creation tokens 或多模型 breakdown。
- Go 目前把每次 bridge `cost_update` 看作该 run 的最新 totals 并直接覆盖 run 字段，这要求 bridge 发来的 token/cost 必须是累计快照；当前不同 runtime handler 并没有一个显式、统一、可验证的 cumulative accounting contract。
- cost workspace 当前只能显示一个总花费，不知道其中哪些来自官方权威总价、哪些是按官方价格推导的估算、哪些运行实际上根本没有 truthful USD 归因。

官方资料说明这个问题现在已经不能继续靠 hardcode/默认回退糊过去:

- Anthropic 官方 Claude Code SDK 文档明确把 `result.total_cost_usd` 作为一次 `query()` 的权威总价，并在 TypeScript 里暴露 `modelUsage`、`cache_read_input_tokens`、`cache_creation_input_tokens` 等更细粒度字段；Anthropic 还提供 Usage/Cost Admin API 与 Claude Code Usage Report。
- OpenAI 官方 Codex 文档明确区分两种计费面: ChatGPT 计划内的 usage limits/credits，和 API key 下按标准 API pricing 计费；OpenAI 组织 usage API 暴露 `input_tokens`、`output_tokens`、`input_cached_tokens`，模型页面与 Codex/API pricing 页面提供按模型的 token 价格。

因此这次设计的核心不是“把所有 runtime 都算成美元”，而是“建立一个 truthful accounting contract”: 能拿到权威总价时就采信；只能按官方价格估算时就明确标估算；当前没有 truthful USD 归因时就显式标 unpriced，而不是继续伪装成精确账单。

## Goals / Non-Goals

**Goals:**

- 为 Claude Code、Codex 以及同类外部 runtime 建立统一的 per-run cost accounting contract，覆盖 native total、usage + official pricing fallback、以及 unpriced 状态。
- 让 bridge `cost_update` 与 Go 持久化基于累计 run snapshot，而不是依赖各 runtime 隐式约定。
- 保留当前 `agent_runs` 标量 totals 兼容性，同时持久化足够的 provenance / coverage / model breakdown 信息，支撑 cost workspace 与未来轻量 consumer。
- 扩展 cost query 与 cost workspace，让 operator 能看到 billed / estimated / unpriced spend 的边界，以及 runtime/provider/model 维度的 breakdown。
- 让 resource-governor 明确感知“有些运行当前无 truthful USD 归因”，不再把 coverage gap 悄悄伪装成 `$0.00`。

**Non-Goals:**

- 不把这次 change 扩成新的 BI/reporting 平台，也不重做已归档的 cost workspace foundation。
- 不要求所有 CLI-backed backend 在本次都获得官方美元定价；对于没有 truthful billing surface 的 runtime，只要求显式暴露 unpriced/coverage gap。
- 不在本次引入新的 warehouse 级明细表或事件溯源 ledger；现有项目级查询仍以 persisted runs 为主。
- 不在本次强制阻止所有 unpriced runtime 启动；预算 hard-block 策略继续基于当前可记录的 run totals 工作，但必须把 coverage gap 暴露出来。

## Decisions

### Decision: 引入统一的 `RuntimeCostSnapshot`，以“权威总价优先、官方价格回退其次、无法定价显式暴露”为单一优先级

Bridge 不再把“收到一点 usage 就立刻套一个默认价格表”当成通用逻辑，而是统一走同一条 accounting precedence:

1. runtime 原生事件若提供权威总价，优先采信该总价。
2. 若没有权威总价，但提供累计 usage totals，且当前 billing mode 对应官方定价可用，则按官方价格表估算。
3. 若当前 auth/billing surface 只有 usage limits / credits / subscription，或者 runtime 根本没有足够 usage 信号，则标记为 `unpriced` 或 `plan_included`，不伪造美元。

之所以这样定，是因为 Anthropic 和 OpenAI 当前公开合同已经明确不是一类东西。Claude Code SDK 有 `total_cost_usd`；Codex ChatGPT plan 则是 usage limits/credits，而 API key 才是 usage-based billing。继续用单一的“按模型套美元单价”逻辑会把这两类账单语义混在一起。

备选方案是“所有 runtime 都一律按 repo 内价格表折算美元”。拒绝原因是它会把 ChatGPT 计划内的 Codex 使用伪装成 billable USD，也会继续放大模型别名和价格漂移问题。

### Decision: 采用一份 checked-in 的共享价格目录与模型别名表，而不是让各 runtime handler 各自 hardcode

新增一份 repo-owned pricing catalog，定义:

- provider/runtime/model alias 到 canonical pricing model 的映射
- Anthropic / OpenAI 当前支持模型的 input/output/cache pricing
- 每个 runtime 在不同 billing mode 下允许的 fallback 策略

这份目录会像现有 `coding_agent_backend_profiles.json` 一样作为 checked-in source of truth，由 TS Bridge 直接消费，必要时 Go 测试或工具也可读取同一份定义。

备选方案 A 是继续把价格硬编码在 `src-bridge/src/cost/calculator.ts`。拒绝原因: 当前问题正是由这种散落 hardcode 导致的。备选方案 B 是只依赖远程 pricing API。拒绝原因: 运行时成本路径必须能离线稳定工作，且 Anthropic/OpenAI 当前公开的价格信息本身就是文档合同，不是统一的机器可读 pricing endpoint。

### Decision: `agent_runs` 保留现有标量 totals，同时新增 JSONB `cost_accounting` 承载 provenance 与多模型 breakdown

现有 `agent_runs` 标量字段 `input_tokens` / `output_tokens` / `cache_read_tokens` / `cost_usd` / `turn_count` 继续保留，因为它们已经被 budget tracking、DTO、query service 和现有页面使用。为了避免引入一轮过大的 schema 重构，本次新增一个 JSONB 字段，例如 `cost_accounting`，承载扩展 accounting 元数据，例如:

- `mode`: `authoritative_total` / `estimated_api_pricing` / `plan_included` / `unpriced`
- `source`: `anthropic_result_total` / `openai_api_pricing` / `native_runtime_total` 等
- `coverage`: `full` / `partial` / `none`
- `components`: 可选的 per-model / per-provider breakdown
- `notes`: 对 subscription/credits/unpriced 的解释

之所以选 JSONB 而不是新建 `agent_run_cost_components` 子表，是因为当前 `CostQueryService` 本身就是 `ListByProject(...)` 后在 Go 内存里聚合，而不是依赖复杂 SQL group-by。先把 truthful metadata 跟 run 绑定起来，就已经足够支撑当前 cost workspace 与 summary consumer；若以后确实需要大规模 SQL 聚合，再单开 change 把 components 正规化成子表。

备选方案是只扩展现有标量列，不持久化 provenance / breakdown。拒绝原因是这样无法 truthfully 表达 Claude Code 的多模型 `modelUsage`，也无法让 cost workspace 区分 billed / estimated / unpriced。

### Decision: 明确 `cost_update` 是“latest cumulative snapshot”合同，不是 per-step delta 合同

本次不会让 Go 去猜每个 runtime 发来的是 delta 还是 total。Bridge 统一对外发送 latest cumulative snapshot:

- 标量 totals: input/output/cache-read/cost/turn count
- 可选扩展: cache creation totals、cost mode/source、coverage

Go 继续使用“更新当前 run 最新 totals，再按 persisted runs 重新求 task spent”的方式。这与当前 `UpdateCost -> sumTaskRunCost` 的实现一致，也天然避免 periodic update 或重复上报时的 double count。

备选方案是让 Go 改成 delta accumulator。拒绝原因: 现有 runtime 既有 periodic 当前总价、也有 native total、也有 usage fallback，把 delta 语义再引入 Go 只会让 Bridge/Go 双方各自猜测，复杂度更高。

### Decision: cost query 与 cost workspace 公开 coverage，而不是继续假装“一个总花费就够了”

`cost-query-api` 将在当前 project summary 基础上增加两类信息:

- `runtimeBreakdown`: 按 runtime/provider/model 聚合的 run count、priced/unpriced count、total cost、cost mode 概览
- `costCoverage`: 汇总 authoritative / estimated / unpriced spend 与 run 数，外加 `hasCoverageGap`

`cost-operator-workspace` 将消费这两类字段，在 headline 下方或专门 section 中显示:

- 当前选定项目的 external runtime cost coverage
- 运行时来源 badge（authoritative / estimated / unpriced）
- 明确的 gap 文案，而不是继续把 unpriced runs 隐形或算成 `$0`

备选方案是只修 backend accounting，不改 workspace。拒绝原因: 用户请求就是“完善现在成本统计功能”；如果页面仍只展示一个总花费，新的 truthfulness 信息对 operator 不可见，这次 change 的目标就只完成了一半。

### Decision: 对 unpriced runtime 先暴露 coverage gap，不在本次直接阻断运行

对于当前没有 truthful USD 归因的 runtime / billing mode，本次 change 的行为是:

- 不伪造美元
- 在 run/task/project summary 上暴露 coverage gap
- 让 operator 知道当前 budget/cost 视图不覆盖这部分运行

本次不把策略升级成“检测到 unpriced runtime 就拒绝 dispatch”。这样做的原因是当前仓库已经把多 runtime 作为一等公民，直接阻断会扩大产品行为范围，也会影响现有 operator 使用方式。更合理的路径是先把 truthfulness 拉直，再基于真实 coverage gap 决定是否需要下一条 change 去收紧 admission policy。

## Risks / Trade-offs

- [官方价格与 billing 文档会继续演进，checked-in catalog 仍可能过时] -> 价格目录单独建源文件并补 focused tests，保证更新路径明确；设计上优先 native authoritative totals，减少对本地价格表的依赖面。
- [Codex 当前 billing mode 可能只有“已登录”，未必总能稳定区分 API key 与 ChatGPT plan] -> billing mode 无法 truthfully 判定时默认收紧到 `unpriced` / `plan_included`，绝不伪造 billable USD。
- [JSONB `cost_accounting` 会把部分聚合从 SQL 推到 Go 内存] -> 当前 `CostQueryService` 本来就是 list-and-aggregate 模式，这个成本在现有规模下可接受；如果后续 summary 需要更强 SQL 可查询性，再升成子表。
- [workspace 同时展示 authoritative / estimated / unpriced 可能让 UI 更复杂] -> 通过 coverage summary + runtime breakdown 一次解释清楚，避免 operator 在多个 section 间自行猜测。
- [预算治理当前仍主要依赖 `cost_usd` 标量 totals，unpriced runs 不会触发等价的美元 hard stop] -> 本次显式暴露 coverage gap，把“不能 truthfully 治理”从隐性错误变成显性事实；后续若需要严格治理，再单开 admission/control-plane change。

## Migration Plan

1. 为 `agent_runs` 增加 `cost_accounting` JSONB 字段，并把 persistence model / DTO 映射扩到该字段。
2. 在 TS Bridge 引入共享 pricing catalog 与 accounting normalizer，逐步替换 `calculator.ts` 的单文件 hardcode。
3. 扩展 `AgentRuntime` 维护 latest cumulative accounting snapshot，并更新 Claude/Codex/OpenCode/runtime family handlers 统一发 cumulative `cost_update`。
4. 更新 Go bridge event decoding 与 `AgentService.UpdateCost` 路径，让 run 标量 totals + `cost_accounting` 一起持久化，再由 `CostQueryService` 聚合 coverage / runtime breakdown。
5. 更新 `/api/v1/stats/cost` contract、cost store 与 cost workspace，增加 attribution/coverage 渲染。
6. 运行 focused TS bridge tests、Go service/handler tests、frontend cost workspace tests，确认 billed/estimated/unpriced 三类路径都可验证。

回滚策略:

- 应用层可整体回退到旧的 runtime accounting 与 query/workspace 渲染逻辑。
- `agent_runs.cost_accounting` 作为新增 JSONB 字段，即使回滚应用代码也可以保持空置/忽略，不要求紧急回删数据列。

## Open Questions

- IM `/cost` 在本次是否只复用 `costCoverage` 的摘要文案，还是要同时暴露 runtime breakdown？当前倾向先给 summary + warning，不把 IM 命令扩成多表格输出。
- 若 OpenAI 后续公开 Codex credits 或 ChatGPT 计划内使用的更稳定程序化账单接口，是否应把 `plan_included` 从“无美元”升级为另一类可量化成本？当前不在本次承诺。
- Claude Code 的 `modelUsage` 若出现大量多模型 component，workspace 首版应直接展示完整 model breakdown，还是先聚合到 runtime/provider/model 三级并保留 drill-down 作为后续增强？当前倾向先做三级 breakdown，保留组件详情在 `cost_accounting` 中。
