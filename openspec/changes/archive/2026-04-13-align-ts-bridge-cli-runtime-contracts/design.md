## Context

AgentForge 现在的 TS Bridge 对 `cursor`、`gemini`、`qoder`、`iflow` 采用的是“共享 CLI adapter family + runtime-specific launch recipe”的结构：catalog、readiness diagnostics、settings/selector、Go default-resolution 和 execute preflight 都已经能识别这些 runtime，但真正的 launch 仍集中在 `buildCliRuntimeLaunch(...)` 与 `streamCommandRuntime(...)` 里。当前问题不是“系统完全不知道这些 runtime”，而是“Bridge 还没有把它们的官方 headless contract 当作 source of truth”。

仓库真相已经暴露出几类风险：
- `cursor` 当前仍依赖 stdin prompt 与本地猜测的 mode flags，而官方文档主线是 headless prompt / output 参数；
- `qoder` 与 `iflow` 也仍存在 output flag、approval mode、interactive vs non-interactive prompt transport 的漂移风险；
- `gemini` 虽然更接近官方 headless 使用方式，但认证模式、machine-readable output、以及 capability publishing 仍需要按当前官方文档再校准；
- `iflow` 官方已经声明将于 **2026-04-17（北京时间）停服并迁移到 Qoder**，所以 Bridge 不能继续把它当成普通长期稳定 runtime。

这条 change 是一条 cross-cutting contract cleanup：会同时影响 TS Bridge runtime layer、runtime catalog、Go/backend catalog consumer、前端 runtime selector，以及 operator-facing diagnostics。

## Goals / Non-Goals

**Goals:**
- 让 `cursor`、`gemini`、`qoder`、`iflow` 的 launch contract 与当前官方 headless/runtime docs 对齐，而不是继续沿用 Bridge 自己的猜测式参数组合。
- 让 runtime catalog 与 execute preflight 基于同一份 CLI runtime truth，明确区分 supported / degraded / unsupported / sunset。
- 让 Go 和前端消费到更真实的 catalog 元数据，能够在 launch 之前就看到 install/auth/profile/deprecation/contract mismatch 风险。
- 把 iFlow 的官方停服窗口纳入 Bridge 诊断与 catalog 生命周期信息，给出 migration guidance，而不是等运行失败后才暴露。
- 用 focused tests 固化各 runtime 的 documented invocation 和 truthfully unsupported 边界。

**Non-Goals:**
- 不把 `cursor`、`gemini`、`qoder`、`iflow` 升级成和 Claude/Codex/OpenCode 同等级的 full dedicated connector。
- 不在这条 change 中重做 Go ↔ Bridge 拓扑、前端大范围 UI 重构、或新增独立 operator panel。
- 不在这条 change 中承诺额外 CLI runtime 的高级 lifecycle parity（例如完整 fork/rollback/session replay），除非该 runtime 当前官方 contract 已明确提供且 Bridge 能 truthfully 承接。
- 不把 iFlow 立即从系统中物理移除；这里先做 truthful deprecation/sunset surfacing 与 launch gating。

## Decisions

### 1. 用“runtime-specific documented launch descriptor”替换当前部分共享猜测逻辑
- 决策：保留 `cli-agent-runtime-adapters` 这条 shared family，但把每个 runtime 的 headless invocation 从零散 `switch` + 通用 stdin/JSONL 假设，收敛成显式的 documented launch descriptor。
- 原因：当前问题不是一定要把四个 runtime 都做成完全独立 adapter，而是需要一个能表达“prompt 通过什么入口传、output 通过什么 machine-readable mode 取、approval mode 是 CLI flag 还是 config-only、哪些能力根本不该发布”的模型。
- 备选方案：
  - **方案 A：继续增强通用 `streamCommandRuntime(...)`**。优点是改动小；缺点是会继续把“官方 contract 差异”压平到一个 shared parser 里，后续仍容易漂移。
  - **方案 B：四个 runtime 全部做 full dedicated adapter**。优点是最彻底；缺点是当前 scope 过大，也会把本次 focused repair 扩成另一轮 runtime foundation 重写。
- 结论：选中间路线——保留 shared family，但让 launch contract 和 parser capability 变成 per-runtime 明确声明。

### 2. 以官方文档可证实的 headless/runtime contract 作为唯一校准基线
- 决策：Bridge 对额外 CLI runtime 的 prompt transport、output format、approval mode、auth/profile prerequisites、additional directories、resume/continue/deprecation 等，都只以官方当前文档可证实的 contract 为准。
- 原因：现有问题正是源于 repo 内部 launch recipe 跑在官方 contract 前面。只有把官方文档变成校准基线，catalog / preflight / route behavior 才能保持一致。
- 备选方案：继续接受“用户实际机器上碰巧可用”的 undocumented flags。问题是这会让 runtime catalog 失真，也无法形成稳定测试。
- 结论：文档未证明的能力一律降到 degraded / unsupported，直到后续有新的官方 contract 可以重新收口。

### 3. 把 deprecation/sunset 作为 runtime readiness 的一部分，而不是文档附注
- 决策：runtime catalog 与 execute preflight 需要能表达 vendor-published lifecycle status，例如 `deprecated`、`sunset_at`、`migration_target`、`replacement guidance`。iFlow 是首个必须纳入这套合同的 runtime。
- 原因：对 operator 来说，“还能不能启动”并不只取决于 binary 和 API key；如果上游产品已经官方宣布停服窗口，系统就必须提前暴露这个事实。
- 备选方案：只在 README 或帮助文档里写停服提示。缺点是 launch surface、settings selector、automation 都看不到，会继续把问题推到运行时。
- 结论：deprecation/sunset 进入 catalog 与 preflight 的正式 contract。

### 4. 让 Go/前端只消费 Bridge 发布的 runtime truth，不再各自推断 CLI backend 稳定性
- 决策：Go fallback catalog、project settings response、frontend runtime selector 继续以 Bridge catalog 为权威来源，但需要扩展到消费 CLI launch-contract diagnostics 和 lifecycle metadata。
- 原因：如果上游页面或 Go fallback 仍只认 `runtime key + available`，就会把 CLI runtime 当成和 dedicated runtime 一样稳定，破坏这次 change 的目标。
- 备选方案：只修 Bridge 执行路径，不改 catalog consumer。缺点是用户仍会在前端选到“看似可用但其实合同不成立”的 runtime。
- 结论：catalog consumer 需要最小扩展，至少把 degraded/deprecated/sunset 讲明白并阻止明显错误提交。

### 5. 测试以“文档对齐 + truthful failure”双轨验证
- 决策：focused tests 同时覆盖两类行为：
  1. 当 runtime 当前官方 contract 支持某种 headless path 时，Bridge 使用正确 invocation；
  2. 当 runtime 当前官方 contract 没有 truthfully supported 的输入/控制时，Bridge 在 catalog 与 execute preflight 上显式拒绝。
- 原因：只测 happy path 会重新掉回“能跑就算接上”；而这条 change 的核心是 truthful contract，不只是把进程启动起来。

## Risks / Trade-offs

- **[风险] 官方 CLI 文档更新快，参数面可能继续变化** → **缓解**：把 documented launch descriptor 和 focused tests 放在一起维护，后续只需更新 runtime-specific contract 与测试，不再全局搜索修补。
- **[风险] 某些 runtime 的官方文档对 machine-readable output 或 approval controls 描述不完整** → **缓解**：优先发布 degraded/unsupported，不用 undocumented 行为硬凑“支持”。
- **[风险] Go fallback catalog 与 Bridge live catalog 出现短时漂移** → **缓解**：把 fallback 限制为保守模式，只发布已知稳定字段；CLI launch-contract truth 以 Bridge live catalog 为主。
- **[风险] iFlow 停服窗口很近，设计刚落地时 runtime 可能已进入 sunset 后状态** → **缓解**：在实现中使用当前时间判断；到达或超过 2026-04-17（北京时间）后默认拒绝新启动，并明确迁移到 Qoder。
- **[风险] 用户仍期待四个 CLI runtime 立即具备和 Codex/OpenCode 一样的高级控制面** → **缓解**：在 specs 中把 scope 明确限定为 contract alignment 与 truthful connectivity，不承诺未文档化的高级 lifecycle parity。

## Migration Plan

1. 先引入 per-runtime documented launch descriptor 与 capability metadata，不立即改 Go/前端结构。
2. 用 descriptor 替换 `buildCliRuntimeLaunch(...)` 中的猜测式参数与 stdin prompt 假设，并收紧 execute preflight。
3. 扩 runtime catalog / Go DTO / frontend selector 的 degraded/deprecated/sunset 展示。
4. 加 focused tests 覆盖 Cursor/Gemini/Qoder/iFlow 的 documented invocation 和 truthful rejection。
5. 若实现后发现某个 runtime 当前已无法形成稳定官方 headless path，则保持 catalog 可见但 marked unavailable/degraded，而不是伪造 green support。

## Open Questions

- Cursor 当前官方文档是否公开了足够稳定的 machine-readable event stream schema，还是只能保证 headless text/json output？如果没有稳定 event schema，本次要把它降为 text-first normalization。
- Gemini 的 approval/sandbox 控制中，哪些是稳定 CLI flags，哪些仅适合配置级别表达？实现前需要以最新官方文档再确认一次。
- Qoder 与 iFlow 的 continue/resume/headless session 语义是否足够稳定，可否进入 catalog metadata，还是继续标为 unsupported。
