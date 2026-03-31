## Context

AgentForge 现有的 coding-agent 体系已经把 `claude_code`、`codex`、`opencode` 打通到了产品层：Bridge 有 runtime registry，Go 有 resolved runtime tuple，前端 settings / team / agent launch 也开始消费统一 catalog。但是当前实现仍然把“支持哪些 backend”写死在多个层面：

- `src-bridge/src/types.ts` 和 `src-bridge/src/schemas.ts` 只接受三种 runtime key。
- `src-bridge/src/runtime/registry.ts` 虽然已经具备 registry 结构和一个半成品 `command-runtime` seam，但真正注册的 adapter 仍只有三种 backend。
- `src-go/internal/service/coding_agent.go` 仍维护一个三项静态 runtime map。
- `lib/stores/project-store.ts`、`lib/stores/agent-store.ts`、`components/shared/runtime-selector.tsx` 只消费 `compatibleProviders + defaultModel + diagnostics` 这一版极简 catalog，无法表达“某 backend provider 固定”“某 backend 有多组建议模型”“某 backend 不支持 resume/fork”等差异。

外部参考 `cc-connect` 已经验证了两件事：

1. 更多 coding-agent backend 可以通过 registry/adapter 模式在一个产品面里并存。
2. 对于 Cursor Agent、Gemini CLI、Qoder CLI、iFlow CLI 这类 CLI-backed backend，真正可复用的不是 UI，而是“后端 profile + adapter + diagnostics”的结构化接入方式。

因此，这次 change 不是简单“往 union 里再塞四个字符串”，而是要把 AgentForge 从“三个特例 runtime”升级为“多 backend runtime contract”。

## Goals / Non-Goals

**Goals:**

- 将 `cursor`、`gemini`、`qoder`、`iflow` 引入为 AgentForge 的一等 coding-agent runtime/backend。
- 移除 Bridge schema、Go fallback catalog、前端 selector 对“三种固定 runtime”的隐式假设。
- 引入一份跨 Go / Bridge 可复用的 backend profile 元数据，统一表达 label、provider/model 约束、feature matrix、安装/认证前提。
- 保留当前 `runtime / provider / model` 三元组合同，但允许不同 backend 真实表达“固定 provider”“建议模型列表”“truthful unsupported lifecycle”。
- 保持现有 `claude_code`、`codex`、`opencode` 的行为兼容，不倒退已完成的 dedicated connector 与 continuity contract。

**Non-Goals:**

- 这次 change 不引入 ACP 通用协议支持，也不在同一批里接入 Copilot/OpenClaw 等 ACP backend。
- 这次 change 不要求新增 backend 在首版就具备与 Claude/Codex/OpenCode 完全对齐的 pause/resume/fork/rollback 能力。
- 这次 change 不改成 team 内按 role 分别选择不同 backend；仍沿用一条 team run 共享一个 resolved runtime tuple。
- 这次 change 不重写现有 Codex / OpenCode dedicated connector，为了“统一形式”退回 generic command runner。

## Decisions

### 1. 引入一份 checked-in backend profile manifest，作为 Go 与 Bridge 共享的产品级 metadata 真相源

当前 drift 的根源不是 registry 机制缺失，而是“支持哪些 runtime、每个 runtime 的默认 provider/model、哪些 feature 对用户可见”分散在 Go、Bridge、前端三层的静态结构里。继续扩四个 backend，如果仍沿用多份硬编码表，会很快再次失真。

因此本次设计引入一份 checked-in backend profile manifest（文件位置实现时最终确认），至少包含：

- canonical runtime key 与 display label
- default provider、compatible providers
- default model 与可选的 suggested model options
- supported feature flags
- executable / auth / env prerequisite metadata
- backend family（dedicated vs cli-backed）

Bridge 仍然是运行时 truth source：真实 availability 诊断、动态探测、advanced operation 支持都以 Bridge 为准。但 Go 的 fallback catalog、project default resolution、frontend contract DTO 不再各自维护另一套 runtime 名单，而是基于同一份 profile metadata 构造。

备选方案：

- 继续在 Go 和 Bridge 中维护两份平行静态表。否决，因为这正是当前三 runtime 语义持续 drift 的来源。
- 让 Bridge 成为唯一 metadata source，Go 无 fallback。否决，因为 settings/default resolution 仍需要在 Bridge 不可用时提供受控降级，而不是把产品语义完全绑死在运行中的 sidecar 上。

### 2. Bridge runtime adapter 分成两族：保留 dedicated connector，同时新增 CLI-backed profile adapter family

`claude_code`、`codex`、`opencode` 已经有明显不同的 truthful integration surface：

- Claude Code 依赖 Claude Agent SDK
- Codex 依赖官方 Codex CLI connector
- OpenCode 依赖官方 HTTP transport

这些 backend 不应该为了形式统一而降级成 generic subprocess shelling。

但新增的 `cursor`、`gemini`、`qoder`、`iflow` 明显更适合走同一类 CLI-backed profile family：共享 subprocess launch / parser / diagnostics / capability enforcement 的骨架，每个 runtime profile 只提供：

- binary discovery / install hint
- command builder 与 env mapping
- native event parser strategy
- auth/login preflight
- continuity policy
- supported advanced operations

这允许 AgentForge 吸收 `cc-connect` 的 registry/adapter 思路，而不把整个 `cc-connect` agent core 搬进来。

备选方案：

- 为每个新 backend 单独复制一套专用 adapter。否决，因为重复高，且无法形成可持续扩展的 contract。
- 让所有 backend 都走一个完全泛化的 command runner。否决，因为 Codex/OpenCode 已经证明需要 dedicated connector 才能保持 truthful lifecycle semantics。

### 3. 运行时 capability 必须显式出现在 catalog 中，前端与 advanced routes 只能消费“已验证支持”的能力

当前 `RuntimeCatalogEntry` 已有 `supportedFeatures`，但前端 selector 与 settings 几乎没有使用；同时 catalog 只有 `defaultModel` 一个模型字段，无法表达建议模型列表或“固定模型 vs 用户可改”。

本次设计沿用并扩展 catalog contract：

- 继续保留 `defaultProvider`、`compatibleProviders`、`defaultModel`
- 新增或明确支持 suggested model options
- 继续保留 `supportedFeatures` 作为 runtime capability matrix
- 用 diagnostics 表达 install/auth/profile blocking state

前端 settings / RuntimeSelector / start dialogs 必须从 catalog 派生行为：

- provider 只有一个时，不得发明额外 provider 选项
- runtime 给出建议模型列表时，只允许从 catalog 中选择，除非 feature 明确允许自定义 model
- pause/resume/fork/rollback 等高级操作只在 runtime feature matrix 明确支持时显示或启用

这次 change 明确采用“truthful unsupported”而不是“表面统一”：

- 某 backend 如果没有可验证的 resume 语义，就标记为不支持 resume
- 某 backend 如果没有真实成本或 tool detail，就只输出可证明的 canonical subset

备选方案：

- 对所有 backend 伪造统一生命周期支持。否决，因为会把 unsupported backend 伪装成可恢复/可 fork，违反当前仓库对 runtime 真实性的要求。
- 前端继续只渲染固定 provider/model 下拉框。否决，因为新增 backend 后 UI 将无法表达真实差异，只能继续硬编码例外。

### 4. 保留 `runtime / provider / model` 三元组，但允许 backend profile 定义“固定、建议、可选”的差异

`runtime / provider / model` 三元组已经写入 agent run、team summary、status metadata 和 project settings，替换掉它会放大本次变更范围，也会破坏现有产品与 OpenSpec 合同。

因此本次不废弃三元组，而是在 backend profile 上引入更细粒度的语义：

- fixed provider runtime：例如 `qoder`，provider 只是 canonical alias，不允许自由切换
- multi-provider runtime：例如 `gemini`、`iflow`，provider 选择与 auth/profile 绑定
- fixed default model runtime：只有一个稳定推荐模型
- suggested models runtime：catalog 提供候选列表，但仍由 resolved tuple 输出最终 `model`

这让 Go persistence、API DTO、frontend state shape 保持兼容，同时补足多 backend 扩展所需的产品语义。

备选方案：

- 将 provider 合并进 runtime key，例如 `gemini_vertex`。否决，因为会把 profile 和 runtime family 混淆，放大 catalog 数量并弱化统一 launch tuple。
- 允许 model 完全自由文本输入，不给 catalog 模型元数据。否决，因为 settings/start surfaces 失去预校验能力。

### 5. Go 继续负责最终 launch tuple 解析，Team/单 Agent 都走同一条 catalog-driven resolution path

当前 repo 的正确方向已经很明确：Go 解析 project defaults + explicit overrides，Bridge 只接受明确的 runtime tuple，而不再让前端或 direct caller 猜测。

本次变更延续这一原则：

- Go 用 shared backend profile metadata 构建 fallback/default catalog
- Go 在 launch 前根据 runtime profile 校验 provider/model 组合
- 单 Agent 启动与 Team 启动都走同一条 resolution path
- Team 仍共享一个 resolved runtime tuple，不做 per-role backend 拆分

如果某 runtime 暂不支持某些 team-related execution expectation，也必须在 catalog/diagnostics 中显式暴露，让前端在提交前阻断，而不是等 Team run 启动后才隐式失败。

备选方案：

- 让 Team flow 为新增 backend 单独维护例外规则。否决，因为会再次产生 Team 与单 Agent 的 contract drift。
- 让 Bridge 自行兜底解析 provider/model。否决，因为这会重新把产品级默认值与验证职责下放到执行层。

### 6. ACP 作为后续扩展点保留，但不与本次 CLI-backed runtime 扩展耦合

`cc-connect` 的 ACP agent 说明了一个清晰方向：未来如果 Cursor/Copilot/OpenClaw 等 backend 提供稳定 ACP surface，AgentForge 可以新增另一族 protocol-backed adapters，而不是无限制继续叠加 CLI parser。

但本次 change 仍聚焦于仓库里已有命令/connector seam 最容易承接的四类 backend：Cursor Agent、Gemini CLI、Qoder CLI、iFlow CLI。这样 scope 保持在“真实 runtime product completeness”，而不是升级成新的 protocol platform。

## Risks / Trade-offs

- [Risk] shared backend profile manifest 与 Bridge adapter hooks 可能再次产生 drift -> Mitigation: 增加 contract tests，要求 manifest 中的 runtime keys 与 Bridge registry registration 一一对应，且 Go fallback catalog 与 Bridge DTO 字段集一致。
- [Risk] 新增 CLI backend 的 stdout/stderr 事件格式可能不稳定 -> Mitigation: 每个 backend 使用 fixture-based parser tests，并在 parser 无法识别时退回最小 truthful event subset，而不是 silently pretending full feature parity。
- [Risk] 前端 selector contract 从单一 `defaultModel` 升级到 richer metadata 后，settings/start dialogs 可能出现联动回归 -> Mitigation: 复用 shared `RuntimeSelector` 组件并补全 provider/model filtering、fixed selection、unsupported feature hints 的 focused tests。
- [Risk] iFlow / Gemini / Cursor 的认证路径在不同平台上差异很大 -> Mitigation: 首版诊断先覆盖可验证 prerequisite（command、login、env、config profile），对无法稳定验证的路径默认 blocking rather than optimistic available。
- [Risk] 对 pause/resume/fork 的 truthful unsupported 可能让一部分 operator 感到“新 backend 功能更少” -> Mitigation: 在 catalog 与文档中显式展示 capability matrix，把限制前置到选择阶段，而不是运行后才暴露。

## Migration Plan

1. 先引入 backend profile metadata 与新的 runtime catalog DTO 字段，保持旧三 runtime 行为不变。
2. 在 Bridge 中扩 runtime key/schema、registry 和 CLI-backed profile adapter family，并先让新 backend 进入 diagnostics/catalog surface。
3. 再接 Go fallback/default resolution、project settings DTO、agent/team launch validation，使前后端都消费新 catalog。
4. 最后更新 `RuntimeSelector`、settings/team/agent launch surfaces、README 和 scoped verification 文档。

回滚策略：

- 如果新增 backend adapter 或 selector contract 出现问题，可以先从 profile manifest 中移除对应 runtime，旧三 runtime 继续工作。
- 保持 API DTO 向后兼容：新增 catalog metadata 字段为 additive；旧字段 `defaultProvider`、`compatibleProviders`、`defaultModel`、`diagnostics` 继续保留。
- 如某个 backend 的 parser 或 auth preflight 不稳定，可单独将其标记为 `available=false` 并保留 install/auth diagnostics，而不必回滚整条多 backend contract。

## Open Questions

- canonical runtime key 最终使用 `cursor` / `gemini` / `qoder` / `iflow`，还是带 `_cli` / `_agent` 后缀以区分未来 protocol-backed family？
- `iFlow` 是否在本次 change 中就宣称可恢复 continuity，还是首版统一按 start-only runtime 处理，等单独验证后再开放 resume？
- Go fallback catalog 是否直接读取 shared profile manifest，还是在构建阶段生成 Go-friendly snapshot 以避免运行时解析成本与格式漂移？
- `RuntimeSelector` 是否只支持 catalog-provided suggested models，还是需要同时提供“custom model” escape hatch；如果要提供，哪些 runtime profile 允许？
