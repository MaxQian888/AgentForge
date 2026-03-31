## Why

AgentForge 当前已经把 `claude_code`、`codex`、`opencode` 做成了真实可用的 coding-agent runtime catalog，但产品合同、Bridge schema、Go 选择逻辑和前端设置页仍默认“支持的 backend 只有这三种”。与此同时，`cc-connect` 已经证明 Cursor Agent、Gemini CLI、Qoder CLI、iFlow CLI 这类 coding-agent backend 可以通过统一 registry/adapter 模式稳定接入，因此现在需要把 AgentForge 的 runtime 扩展能力从“三个特例”升级为“可持续扩展的多 backend 合同”。

## What Changes

- 将项目级 coding-agent catalog 从当前固定的 `claude_code` / `codex` / `opencode` 扩展为多 backend runtime catalog，首批覆盖 `cursor`、`gemini`、`qoder`、`iflow` 这类 CLI-backed coding-agent backend。
- 在 Bridge 侧新增一层面向额外 CLI backend 的 adapter/profile 合同，统一描述命令发现、鉴权/登录前提、模型与 provider 选择能力、事件归一化、以及 pause/resume/fork/rollback 等 feature 支持矩阵。
- 扩展 `ExecuteRequest`、runtime registry、runtime catalog、status/snapshot metadata 和 diagnostics 语义，使新增 backend 可以保留真实的 runtime identity，并在能力不对等时返回明确限制，而不是被迫伪装成 Claude/Codex/OpenCode 语义。
- 更新 Go 侧 runtime 解析与项目设置合同，使 settings、单 Agent 启动、Team 启动、运行摘要和运维诊断都消费同一份多 backend catalog，而不是继续把 runtime key、provider 兼容性和默认值硬编码在不同层。
- 更新 README 与 operator-facing 文档，明确新增 backend 的安装方式、认证前提、模型/provider/profile 约束，以及哪些高级生命周期能力在首版中是“truthful unsupported”而非假支持。

## Capabilities

### New Capabilities
- `cli-agent-runtime-adapters`: 定义 Bridge 中额外 CLI-backed coding-agent backend 的统一 adapter/profile 合同，包括命令发现、原生命令行事件归一化、能力矩阵、以及 truthful lifecycle semantics。

### Modified Capabilities
- `coding-agent-provider-management`: 将项目级 runtime catalog、settings/launch 选择和 operator diagnostics 从三种固定 runtime 扩展为多 backend profile 驱动的选择合同。
- `bridge-agent-runtime-registry`: 将 Bridge runtime registry 从固定 union 扩展为支持更多 runtime key、capability-aware diagnostics 和 backend-specific catalog metadata。
- `agent-sdk-bridge-runtime`: 扩展 canonical execute/status/snapshot/cancel/resume 合同，使新增 backend 可以在能力不完全一致的前提下保持真实执行语义和 runtime identity。

## Impact

- **TS Bridge** (`src-bridge/`): runtime key schema、registry、additional CLI runtime adapter/profile layer、status/snapshot typing、server routes、focused tests。
- **Go backend** (`src-go/`): runtime catalog fallback/default logic、project settings contract、launch tuple validation、team propagation、bridge client and DTO normalization。
- **Frontend** (`app/`, `components/`, `lib/stores/`): settings/runtime selector、agent/team launch surfaces、runtime diagnostics display、backend capability hints。
- **Docs** (`README.md`, product/runtime docs): supported backend matrix、install/auth prerequisites、scoped verification guidance。
- **External dependencies**: Cursor Agent CLI, Gemini CLI, Qoder CLI, iFlow CLI 等额外 backend 的本地可执行文件与认证前提需要纳入 readiness 诊断。
