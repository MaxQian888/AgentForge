## Context

AgentForge 的 `src-bridge` 已经从“只会转发 Claude”演进成多 runtime 执行网关：`runtime/registry.ts` 统一选择 Claude Code、Codex、OpenCode 及其他 CLI-backed profiles，`server.ts` 已公开 `/bridge/execute`、`/bridge/fork`、`/bridge/rollback`、`/bridge/messages`、`/bridge/command`、`/bridge/model` 等 canonical route，`handlers/*-runtime.ts` 也已经分别吃下了一批高级能力。与此同时，`openspec/specs/*` 已经记录了大量 runtime-specific advanced features。

但当前 repo 真相显示，Bridge 仍处在“底层 capability 有了不少，上游合同还不够稳”的阶段：

- runtime catalog 仍以 `supported_features: string[]` 为主，不能完整表达输入能力、session lifecycle controls、approval/permission loop、MCP surface、truthful unsupported diagnostics 与 upstream provenance。
- Claude Code 侧已经有 `hooks_config`、`agents`、`thinking_config`、`onElicitation`、`setModel` 等底层接点，但 hook event 范围与 live query control surface 仍未完整对齐到最新官方 Claude Code / Agent SDK 文档。
- Codex 侧已经支持 `--output-schema`、`--image`、`--search`、MCP config overrides 与 `fork`，但交互语义仍偏“拼 CLI flag”，缺少与官方 config / MCP / approvals / sandbox 语义对齐的 bridge-owned capability publishing。
- OpenCode 侧已经接上 `opencode serve` 的 session / diff / revert / todo / messages / command / skills / agents 等 transport 能力，但 provider auth、runtime config update、permission loop、以及 server-backed control plane 还没有被 Bridge 作为一套稳定上游合同发布完整。

这次 change 的关键约束有三条：

1. 不能重开 `complete-backend-bridge-connectivity` 正在处理的 Go↔Bridge 拓扑收口；范围必须留在 `src-bridge` 自己拥有的 runtime interaction seam。
2. 不能为了“看起来全能”而伪造 parity；每个 runtime 只暴露官方文档和当前实现都能站得住的能力，不支持的操作必须显式发布 unsupported / degraded diagnostics。
3. 需要把“充分学习网上的成功案例”落实成官方 contract 对齐与 conformance proof，而不是只在 proposal 里引用几句文档名词。

## Goals / Non-Goals

**Goals:**
- 让 `/bridge/runtimes` 从平面 feature list 升级为可被 Go、前端、operator surface 稳定消费的 interaction capability matrix。
- 对齐 Claude Code、Codex、OpenCode 三条 runtime 的官方交互模式，把 Bridge 真正承诺支持的 request fields、routes、callback semantics、session controls、diagnostics、catalog metadata 写成清晰合同。
- 用 bridge-owned translators 隔离 runtime 差异：Claude 走 SDK callbacks / hooks，Codex 走 config overlay + CLI launch normalization，OpenCode 走 server/OpenAPI transport。
- 为这些合同补上 focused conformance tests / fixtures，防止官方 SDK/CLI/server 升级后仓库继续“spec 说支持、代码实际上漂移”。
- 保持现有上游调用面兼容：已有 canonical routes 继续存在，旧的 `supported_features` 可以保留为兼容字段，但不再是唯一能力发布面。

**Non-Goals:**
- 不修改 Go backend、前端页面或 IM/operator UI 的业务流程；本 change 只定义并实现它们未来应消费的 Bridge runtime contract。
- 不新增新的 runtime 或 provider；只对齐现有 Claude Code、Codex、OpenCode。
- 不尝试把三个 runtime 硬拉成完全同构；差异要被发布出来，而不是被隐藏。
- 不引入对非官方私有协议的依赖；Claude 以官方 Agent SDK 为准，Codex 以官方 CLI/config/MCP 文档为准，OpenCode 以官方 server/OpenAPI/SDK 为准。

## Decisions

### Decision 1: 用 structured interaction capability matrix 取代“只发 supported_features”

`/bridge/runtimes` 将继续返回兼容性的 `supported_features`，但新增结构化 capability metadata，至少拆成：

- `inputs`: structured output、attachments、additional directories、env、web search、agents、hooks、thinking 等；
- `lifecycle`: execute / pause / resume / fork / rollback / revert / diff / messages / command / interrupt / set_model；
- `approval`: hook callbacks、tool permission、MCP elicitation、runtime-native approval policy；
- `mcp`: upstream runtime 如何消费 MCP server config、tool approvals、status visibility；
- `diagnostics`: ready / degraded / unsupported 的原因、缺失前置条件、以及是否有官方 upstream support。

这样上游可以按能力渲染和降级，而不是继续靠 runtime name 或零散 feature string 猜测。

**Alternative considered:** 继续扩展 `supported_features` 字符串数组。  
**Rejected because:** 字符串列表无法表达参数化能力、route-level contract、unsupported reason 与 runtime-native approval semantics，越堆越不可维护。

### Decision 2: 每条 runtime 采用“官方控制面优先”的 bridge-owned translator

- **Claude Code**：Bridge 继续通过 Agent SDK `query()` 与 Query methods 工作，但扩展 hook schema、subagent / thinking / live-control publishing，使其与官方 hooks / subagents / CLI reference 对齐。
- **Codex**：Bridge 不把 Codex 看成“拼 flags 的黑盒 CLI”；而是生成临时 config overlay / per-run overrides，把 MCP、approval policy、sandbox-related intent、image/search/model choices 收口成一个 bridge-owned translator，再由 launcher 负责 CLI 映射。
- **OpenCode**：Bridge 把 `opencode serve` 的 HTTP server / OpenAPI 视为 canonical control plane，补齐 provider auth、config update、session shell / command / messages / permissions / catalog surfaces，并尽量用 transport/SDK 类型而不是 ad hoc JSON 假设。

**Alternative considered:** 继续让每个 handler 独立演进，想到什么就加一个字段或 route。  
**Rejected because:** 这会继续制造“代码似乎支持、catalog 说不清、上游也不知道能不能调用”的漂移。

### Decision 3: capability publishing 必须同时声明 truthful unsupported / degraded semantics

所有 interaction controls 都需要三态表达：

- **supported**：Bridge 已实现且 upstream 官方 contract 明确支持；
- **degraded**：Bridge 有条件支持，但缺少当前前置条件（如认证、callback URL、配置）；
- **unsupported**：官方当前不支持或 Bridge 故意未承诺，不允许上游把它误当成可调用。

HTTP route 的错误也要对齐这个模型：不是简单 500，而是 capability-aware validation/configuration/unsupported response。

**Alternative considered:** 只在运行时调用失败时返回错误。  
**Rejected because:** 这会让前端/Go/operator surface 无法提前判断能力，最终继续把失败当成偶发 bug。

### Decision 4: conformance proof 采用“官方文档例子 + repo 夹具”双轨验证

每个 runtime 各补一组 focused proof：

- 官方文档例子映射：验证 Bridge 公开的字段/route 与官方文档中的成功调用模式一致；
- repo fixture 验证：在 `src-bridge` 内用 mock runner / mock transport / stub callbacks 验证 event shapes、capability metadata、unsupported diagnostics 和 canonical route behavior。

这组 proof 只覆盖 Bridge 自己的 seam，不扩大成全仓 E2E。

**Alternative considered:** 只保留单元测试或只靠人工文档审查。  
**Rejected because:** 单元测试不足以证明合同；纯文档审查又挡不住 SDK/CLI/server 漂移。

### Decision 5: 保持 additive migration，旧调用方逐步转向新 metadata

现有 `/bridge/*` route 和已有字段尽量保持兼容，新增的 capability matrix、diagnostics shape、以及 runtime-specific interaction metadata 采用 additive 方式发布。旧调用方仍可读取 `supported_features`，但新 spec 会要求上游逐步改用结构化 metadata 做判定。

**Alternative considered:** 直接替换 catalog 响应或重命名现有 route。  
**Rejected because:** 当前仓库还有 Go、前端、测试、以及 operator surfaces 在消费旧字段，强切会把 focused TS Bridge change 扩成跨栈迁移。

## Risks / Trade-offs

- **[Risk] 官方 Claude/Codex/OpenCode 文档继续迭代，spec 容易再次漂移** → **Mitigation:** 在 design 中明确官方 contract baseline，并为关键交互面补 doc-grounded conformance fixtures。
- **[Risk] capability matrix 过细，catalog 变得难消费** → **Mitigation:** 结构上按 inputs/lifecycle/approval/mcp/diagnostics 分组，保持字段稳定且 machine-readable，避免把 runtime-specific 原始对象直接透传给上游。
- **[Risk] Codex/OpenCode 的配置与权限语义差异很大，容易被错误“归一化”** → **Mitigation:** 只归一化 AgentForge 真正需要的交互合同，其余差异通过 capability metadata 与 unsupported semantics 显式暴露。
- **[Risk] 新增 route / metadata 后，上游短期内仍继续消费旧字段** → **Mitigation:** 保持 additive 兼容，先让 Bridge truthfully 发布，再由后续 change 消费新 metadata。
- **[Risk] conformance tests 过度依赖真实外部工具，导致 CI 不稳定** → **Mitigation:** 主体使用 mock runner / stub transport / snapshot fixtures，真实工具只保留非常窄的 smoke seam。

## Migration Plan

1. 先修改 OpenSpec：补齐 runtime registry、cross-runtime extensions、Claude/Codex/OpenCode advanced features、以及 HTTP contract 的 delta specs。
2. 在 `src-bridge` 中引入新的 capability metadata builders 与 route/diagnostics shapes，同时保留旧 `supported_features` 输出。
3. 分 runtime 补 translator：Claude 扩 hook/live control publishing；Codex 扩 config overlay / approval metadata；OpenCode 扩 server control plane / auth/config publishing。
4. 为新增 contract 补 focused tests 与 doc-grounded fixtures；必要时记录哪些能力仍是 explicit unsupported。
5. 后续消费方可逐步切到新 metadata；若实现中途出现回归，可按 runtime translator 或 route group 局部回滚，而不需要推翻整个 registry 结构。

## Open Questions

- Codex 的 granular approval / sandbox 语义在 Bridge 里发布到什么粒度最合适：只发布 capability + effective mode，还是发布近似 config overlay 摘要？当前倾向是发布 capability + effective mode，避免暴露太多 Codex 私有实现细节。
- Claude Code 的 hook coverage 要不要一步到位扩到全部最新事件类型（如 SessionStart/SessionEnd/Notification/UserPromptSubmit 等）？当前倾向是先覆盖和 AgentForge 编排最相关的事件，再把其余事件纳入 capability metadata。
- OpenCode provider OAuth flow 是否在本 change 就做完整 callback handling，还是先发布 auth-required capability + start URL？当前倾向是先打通 canonical auth handshake 和 permission event round-trip，复杂 UI 回显留给后续上游消费 change。
