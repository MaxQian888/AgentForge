## Why

TS Bridge 现在对 `cursor`、`gemini`、`qoder`、`iflow` 这批额外 coding-agent runtime 已经有 catalog 和 launch wiring，但当前实现仍主要依赖 Bridge 自己手写的 CLI flags、stdin prompt 传递方式和 `stream-json` 假设，而这些假设并没有全部对齐各自当前官方 headless/runtime contract。结果是 Bridge 可能把 runtime 标成“可用”，上游也能选中它们，但真实执行时却会因为 prompt transport、output flag、approval mode、auth/profile 前提或已公开的产品生命周期约束而连不真、跑不通、或者静默降级。

现在需要补一条 focused change，把额外 CLI-backed runtime 的连接合同重新校准到各自官方文档和当前上游真相，避免 Go、前端和 operator surface 继续建立在错误的 runtime 假设之上；同时把 iFlow 官方已公布的停服时间（2026-04-17，北京时间）纳入 catalog 与 diagnostics，防止系统继续把即将下线的 backend 当成普通可持续支持对象。

## What Changes

- 对齐 `cursor`、`gemini`、`qoder`、`iflow` 的 headless launch contract，逐个收紧 prompt 传递方式、non-interactive/print mode 参数、output format、approval/sandbox/additional directories、resume/session 相关能力，以及模型与 provider/profile 选择约束。
- 替换或拆分当前通用 `buildCliRuntimeLaunch(...)` / `streamCommandRuntime(...)` 的“统一猜测式”启动假设；对于官方有明确 headless contract 的 runtime，Bridge 必须使用对应的正式入口；对于官方没有 truthfully supported 的能力，Bridge 必须在 execute preflight、catalog metadata 和 route behavior 上显式拒绝，而不是继续 best-effort 假跑。
- 收紧额外 CLI runtime 的 readiness/auth diagnostics：不仅检查可执行文件和环境变量，还要校验登录方式、官方 profile/config 前提、required output mode、以及 runtime 是否仍处于可持续支持窗口。
- 扩展 runtime catalog 与 Go/前端消费合同，让上游能看到每个 CLI runtime 的 launch contract 状态、degraded/unsupported 原因、安装与认证提示、以及像 iFlow 这类即将停服 runtime 的 deprecation/sunset 信息，而不是只看到一个静态 runtime key。
- 用官方文档校准的 focused tests 固化 Cursor/Gemini/Qoder/iFlow 的 headless invocation、capability publishing、diagnostics、以及 truthfully unsupported 边界，避免后续继续扩 bridge 功能时把 CLI runtime 接线再次漂移。

## Capabilities

### New Capabilities
<!-- None. -->

### Modified Capabilities
- `cli-agent-runtime-adapters`: 将额外 CLI-backed runtime 的 adapter/profile 合同从“统一命令包装”升级为逐 runtime 的官方 headless contract 对齐，明确哪些 launch/input/lifecycle 语义是真支持，哪些必须显式 degraded 或 unsupported。
- `bridge-agent-runtime-registry`: runtime registry 与 `/bridge/runtimes` 必须发布更真实的 CLI runtime readiness、launch-contract、deprecation/sunset 和 input parity diagnostics，避免把 profile 存在误报为可执行连接。
- `coding-agent-provider-management`: 项目级 runtime catalog、settings/selector、以及 operator-facing diagnostics 需要消费 Bridge 发布的 CLI runtime degraded/deprecated 真相，而不是继续把 `cursor` / `gemini` / `qoder` / `iflow` 当成与 dedicated runtime 同质的稳定选项。

## Impact

- **TS Bridge runtime layer**: `src-bridge/src/runtime/registry.ts`, `src-bridge/src/runtime/backend-profiles.ts`, `src-bridge/src/handlers/command-runtime.ts`, 以及额外 CLI runtime 的 launch/normalization helper 与测试。
- **Go/backend catalog consumers**: `src-go/internal/bridge/client.go`, `src-go/internal/service/coding_agent.go`, `src-go/internal/model/project.go`，以及任何消费 bridge runtime catalog 的 settings / launch / diagnostics seam。
- **Frontend/runtime selection surfaces**: `lib/stores/agent-store.ts`, `lib/stores/project-store.ts`, `components/shared/runtime-selector.tsx`, settings 和 team/agent 启动界面中的 runtime hints、disabled/degraded 展示与警示文案。
- **Docs and operator guidance**: `README.md`、runtime/setup docs、以及与额外 CLI backend 安装、认证、停服迁移相关的说明文档。
- **External systems**: Cursor Agent CLI、Gemini CLI、Qoder CLI、iFlow CLI 当前官方文档与 lifecycle 声明将作为这条 change 的校准基线，其中 iFlow 的 2026-04-17 停服约束需要被系统 truthfully surfaced。
