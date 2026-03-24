## Context

`src-bridge` 现在已经有一条真实的 Claude Agent SDK 执行链路，但 provider 支持仍停留在“调用方可以传 `provider`/`model`，Bridge 实际并不消费这些字段”的阶段。与此同时，轻量 AI 场景并没有共用同一套真实 provider 能力：`handleDecompose(...)` 仍返回模拟分解结果，`.env.example` 也只有 `ANTHROPIC_API_KEY`，Bridge 依赖里没有 `ai` 或任何 `@ai-sdk/*` provider 包。

这意味着仓库在设计上已经接受“Bridge 是统一 AI 出口，Vercel AI SDK 用于多 provider 扩展”，但运行时合同还没有把它变成真实行为。本次变更跨越 `src-bridge` 的 schema、配置、轻量 AI 处理器、执行运行时边界，以及 Go 到 Bridge 的请求契约，属于典型的需要先做技术设计再实施的跨模块能力补齐。

## Goals / Non-Goals

**Goals:**
- 为 TypeScript Bridge 引入统一的 provider 注册、默认值解析和能力校验层。
- 用 Vercel AI SDK 承载轻量 AI provider 调用，并把任务分解从模拟结果升级为真实 provider 输出。
- 让 execute/decompose 等 Bridge 请求拥有一致的 `provider`/`model` 语义、默认值和错误返回。
- 保持 Claude Agent SDK 作为 Agent 执行的真实运行时，不因为引入 AI SDK 而回退成新的模拟层。
- 为后续继续接入更多 provider 留下稳定扩展点，包括配置约定、成本统计入口和 focused verification 面。

**Non-Goals:**
- 把所有 Bridge 路径一次性重写为统一的单执行器。
- 在本次变更中承诺 OpenAI/Google 等 provider 能直接替代 Claude Agent SDK 执行完整 coding agent run。
- 扩展前端 UI 的 provider 选择器，或增加新的 Go 业务 API 入口。
- 一次性覆盖 review、intent classify 等所有轻量 AI 场景；本次以 provider 基础设施和 task decomposition 落地为主。

## Decisions

### 1. 引入 Bridge 本地 provider registry，按“能力”而不是按“SDK 名称”建模
Bridge 会新增一个 provider registry，记录每个 provider 的名称、默认模型、所支持的能力，以及对应的执行器工厂。能力至少分成两类：`agent_execution` 和 `text_generation`。这样可以明确表达 `anthropic` 同时支持 Agent 执行与轻量文本生成，而 `openai`、`google` 在本次仅支持轻量文本生成。

选择这个方案，是因为用户要的是“真实 provider 支持”，而不是简单把一堆 API key 丢进 if/else。能力矩阵能让请求校验、默认值选择、错误语义和后续扩展共用一个事实源。

备选方案：
- 按 endpoint 单独写 provider 分支。否决，因为 execute、decompose、后续 classify/review 会很快漂移。
- 直接按 SDK 分两套配置。否决，因为调用方关心的是 provider/model，不应该知道内部是 Claude Agent SDK 还是 Vercel AI SDK。

### 2. 保留 Claude Agent SDK 作为 Agent 执行主路径，只让 provider 合同显式化
`/bridge/execute` 仍由 Claude Agent SDK 驱动，不在这次变更里承诺把 OpenAI 或 Google 也变成 coding-agent 运行时。新的 provider 合同会把这件事说清楚：当请求解析为 `agent_execution` 时，只有映射到 Claude runtime 的 provider 才会被接受，其他 provider 必须得到明确的“不支持该能力”错误，而不是静默忽略。

这样做的原因是仓库现有真实运行时、事件归一化和预算逻辑都围绕 Claude Agent SDK 建立，强行把 AI SDK 拉进 execute 只会把这次 change 扩成另一条大型 runtime 重构。

备选方案：
- 让 execute 直接切到 AI SDK。否决，因为 AI SDK 更适合轻量生成，不等价于现有 Claude Agent SDK agent loop。
- 对 execute 忽略 provider 字段。否决，因为这正是当前 drift 的来源。

### 3. 轻量 AI 场景统一走 Vercel AI SDK 的 text-generation adapter
Bridge 会新增一个轻量 AI adapter，内部统一使用 `generateText()` 和 provider 包实例来跑任务分解。首批真实 provider 以 `@ai-sdk/anthropic`、`@ai-sdk/openai`、`@ai-sdk/google` 为目标集合，并通过 registry 暴露默认 provider/model。任务分解是本次第一个必须切到真实 provider 的入口，后续 classify/review 可以复用同一 adapter。

这样既满足“增加 Vercel AI SDK 作为更多 provider 支持”，也能把模拟分解替换成实际可验证的模型调用，而不破坏现有 Agent runtime。

备选方案：
- 继续在 decompose handler 里手写模拟器。否决，因为这与用户要求的真实 provider 支持相反。
- 直接在 Go 服务里引入 AI SDK。否决，因为仓库已经明确 Bridge 才是统一 AI 出口。

### 4. 在 Bridge schema 层统一 provider/model 默认值、校验和错误语义
`ExecuteRequestSchema`、`DecomposeTaskRequestSchema` 及对应 TypeScript types 会增加可选的 `provider`、`model` 字段。Bridge 在进入具体 handler 前统一做三件事：补默认值、校验 provider 是否存在、校验该 provider 是否支持目标能力。错误返回需要明确区分“provider 未配置”“provider 不存在”“provider 不支持该能力”“model 不可用/为空”等几类失败。

这样可以让 Go 与 Bridge 的合同稳定下来，也为后续前端或 IM 场景透传 provider/model 留出一致接口。

备选方案：
- 只在具体 handler 里各自解析。否决，因为会重复且容易漂移。
- 强制所有请求必须显式传 provider/model。否决，因为当前上游还没有完整 provider 选择 UI，先支持稳定默认值更符合仓库现状。

### 5. 成本统计继续按 Bridge 内统一事件/结果出口收口
Agent 执行仍沿用现有 `cost_update` 事件与 `calculateCost(...)` 体系；轻量 AI 调用则在 provider adapter 中归一化 usage/model 信息，为后续把 text-generation 成本纳入统一统计留入口。本次不强制把所有轻量 AI 成本都回传到 Go 的实时事件流，但设计会保证 provider adapter 输出中包含可计量的 usage/model 元数据。

备选方案：
- 暂时不考虑轻量 AI 成本。否决，因为多 provider 一旦接入，后续最容易失真的就是成本面。
- 立即为轻量 AI 增加新的 WS 事件协议。暂缓，因为这会放大范围，先保留 Bridge 内部统一计量点即可。

## Risks / Trade-offs

- [Bun compile 与 AI SDK/provider 包兼容性可能不一致] -> Mitigation: 把依赖与 bridge build 验证放到首批任务，并优先选官方 provider 包。
- [同一 provider 在 Agent 执行和轻量生成上的能力不对称，容易让调用方误解“支持 provider = 支持全部能力”] -> Mitigation: 通过 registry 能力矩阵和明确错误语义把支持范围写死。
- [默认 provider/model 约定如果过早写进多个地方，会再次产生 drift] -> Mitigation: 让 registry 与配置解析成为唯一事实源，schema/handler 只消费结果。
- [真实分解输出质量不稳定，可能导致下游持久化失败] -> Mitigation: 保留现有结构校验，并补充 provider 输出到 schema 的 focused 测试。
- [引入多 provider 后，环境变量和本地开发配置更复杂] -> Mitigation: 在 `.env.example` 和 spec 中明确最小配置集与缺失时的失败方式。

## Migration Plan

1. 为 `src-bridge` 增加 AI SDK core 与首批 provider 包依赖，并扩展 `.env.example` 的 provider 配置说明。
2. 新增 provider registry、配置解析和 capability 校验层，把 execute/decompose 请求都接到统一解析入口。
3. 将任务分解切换到 Vercel AI SDK text-generation adapter，并保留现有结构校验与错误归类。
4. 把 execute 请求与 Go client 的 `provider`/`model` 语义对齐，对不支持的执行 provider 返回显式错误。
5. 跑通 bridge-focused typecheck/tests，并补一组真实 provider 配置缺失或不支持能力的验证。

Rollback strategy:
- 如果 AI SDK/provider 集成带来构建或合同风险，优先整体回退 provider registry 与 decompose adapter 变更，恢复到单 Anthropic/模拟分解路径，避免留下半套 provider 合同。

## Open Questions

- 首批“更多 provider”是否固定为 `openai` 和 `google`，还是允许实现阶段根据 Bun/AI SDK 兼容性缩成一个额外 provider 再继续扩展？
- 轻量 AI 请求的 provider/model 是否只在 Bridge 内部默认，还是要同步开放给 Go API 上层显式透传？
- 轻量 AI 成本是否需要在本次就进入 Go 的实时可观测链路，还是先只保证 Bridge 内部可计量与可测试？
