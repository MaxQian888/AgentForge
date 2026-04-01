## Why

`complete-cost-statistics-truthfulness` 已经把 cost workspace 收敛到权威 `/api/v1/stats/*` 合同，但这条线仍然建立在一个更底层的假设上: `agent_runs` 里记录的 token 与 `costUsd` 本身是真实的。当前这个假设并不成立。`src-bridge/src/cost/calculator.ts` 只内置了少量 Claude 旧模型键，`claude-haiku-4-5` 定价已经漂移，Codex/OpenAI 模型完全没有自己的价格表，未知模型还会回退到 Claude Sonnet 定价；与此同时，Bridge 目前只累计 `spentUsd`，没有统一的“累计 token totals + 计费来源/provenance”合同，Go 端又把每次 `cost_update` 直接覆盖到 run 持久化字段里，导致 Claude Code、Codex 这类外部 runtime 的费用与 token 统计都可能失真。

现在必须补这条线，是因为 AgentForge 已经把 `claude_code`、`codex`、`opencode` 以及更多 CLI-backed backend 当成真实可用的 coding-agent runtime。Anthropic 官方已经明确区分 `total_cost_usd`、`modelUsage`、`cache_creation_input_tokens`、`cache_read_input_tokens`，并提供 Claude Code usage report；OpenAI 官方也明确区分 Codex 的 ChatGPT 计划内使用与 API key usage-based 计费，并提供组织级 usage API。若继续把所有外部 Agent 花费都压扁成一个没有来源说明的 `costUsd`，当前 cost workspace、预算判断和后续运营分析都会把“真实账单金额”、“按官方价格估算金额”和“根本无法定价的运行”混在一起，表面完整，实际不可信。

## What Changes

- 引入一条统一的外部 runtime 成本归因链路：优先采信 runtime 原生返回的权威总价，其次基于官方价格表与 usage totals 做可解释估算，最后对无法 truthfully 定价的运行显式标记为 unpriced，而不是回退成错误美元值。
- 为 Anthropic / OpenAI 维护受控的运行时价格目录与模型别名归一化，覆盖仓库当前真实支持的 Claude Code / Codex 模型选项，并纳入 cache read / cache write 或 cached input 等官方计费维度。
- 让 Bridge `cost_update` 合同和 Go 持久化都基于累计 run usage totals，而不是混用 step deltas 与累计花费；同时持久化成本来源、计费模式和覆盖状态，避免多次事件覆盖后丢失真相。
- 扩展项目级成本查询合同，新增 runtime/provider/model 维度的成本 breakdown 与 cost coverage summary，让 standalone cost workspace、IM `/cost` 等 consumer 能分辨 billed / estimated / unpriced spend。
- 更新 cost workspace，使其在展示总花费之外，明确说明哪些外部 Agent 成本是官方权威总价、哪些是按官方价格推导的估算、哪些当前没有可用美元归因，避免继续把所有外部 runtime 花费伪装成同一种精度。

## Capabilities

### New Capabilities
- `external-runtime-cost-accounting`: 定义 Claude Code、Codex 与同类外部 runtime 的统一成本归因合同，覆盖原生 cost/usage 采集、官方价格回退、累计 usage totals、计费来源/provenance 与 unpriced 状态。

### Modified Capabilities
- `cost-query-api`: 项目级成本查询需要返回外部 runtime 的 attribution / coverage / breakdown 元数据，而不只是一个无法区分来源的总花费聚合。
- `cost-operator-workspace`: cost workspace 需要显式渲染外部 runtime 成本的 truthfulness 与覆盖状态，避免将 billed、estimated、unpriced spend 混成一个无说明总数。
- `resource-governor`: 任务级预算跟踪需要消费统一的累计 runtime cost accounting 合同，并对无法 truthfully 定价的运行显式暴露覆盖缺口，而不是继续依赖错误回退或零成本假象。

## Impact

- TS Bridge runtime accounting: `src-bridge/src/cost/*`, `src-bridge/src/handlers/*runtime*.ts`, `src-bridge/src/runtime/*`, related bridge tests.
- Go orchestration and persistence: bridge event decoding, `agent_service.go`, cost-query service/DTOs/handlers, repository seams, and any persistence shape needed for provenance or coverage metadata.
- Operator cost consumers: `app/(dashboard)/cost`, `lib/stores/cost-store.ts`, `components/cost/*`, and any IM `/cost` adapter that reuses the project cost summary.
- External truth sources and contracts: Anthropic Claude Code SDK / Admin usage reports, Anthropic pricing, OpenAI Codex pricing, OpenAI model pricing / usage APIs, and runtime auth-mode distinctions such as API-key usage-based billing vs subscription or credit-based usage.
