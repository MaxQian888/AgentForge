## Why

AgentForge 的 TS Bridge 现在已经具备 Claude Code、Codex、OpenCode 三条 runtime 的基础接线，也已经落下了一批 advanced feature spec；但当前 source of truth 仍然有两个真实断点：一是 Bridge 对外发布的 runtime 能力仍主要停留在扁平 `supported_features` 和少量路由，无法把“支持什么、怎么交互、哪里 truthfully unsupported”讲清楚；二是 Claude Code / Codex / OpenCode 官方近期开出来的成功交互模式（Claude 的 hooks/subagents/live controls，Codex 的 config/MCP/approval surface，OpenCode 的 server/OpenAPI session control）还没有被 Bridge 收口成一套稳定、可验证的上游合同。

现在需要补一条 focused 的 spec change，把 TS Bridge 自己负责的 runtime interaction contract 对齐到最新官方能力面，并把“已有底层能力但上游接口、目录、诊断和 conformance 还不完整”的部分补齐，避免后续 Go、前端、IM/operator surface 再各自猜一套 runtime 语义。

## What Changes

- 收紧 TS Bridge 的 runtime catalog / metadata contract：从扁平 feature list 升级为可供上游真实消费的 interaction capability matrix，明确每个 runtime 的输入能力、生命周期操作、权限/审批回路、session controls、MCP 集成、以及 truthful unsupported / degraded diagnostics。
- 对齐 Claude Code interaction surface 到最新官方 Agent SDK / Claude Code docs，补齐当前 Bridge 仍未覆盖或未完全发布的 hook event、subagent/runtime live control、thinking/status introspection 与 callback semantics。
- 对齐 Codex interaction surface 到最新官方 Codex CLI / config / MCP / sandbox 文档，要求 Bridge 通过 bridge-owned config overlay 和 capability publishing 暴露 approvals、sandbox、MCP、image/search、session fork 等真实交互能力，而不是只靠零散 CLI flag。
- 对齐 OpenCode interaction surface 到最新官方 server / SDK / permissions 文档，补齐 Bridge 对 server-backed session controls、provider auth/config updates、shell/command/message surfaces、permission loops 与 catalog metadata 的完整承接。
- 增加 doc-grounded conformance proof：用基于官方文档/官方接口的 focused tests 和 fixtures 证明 `src-bridge` 发布出来的 request fields、routes、event shapes、capability metadata 与各 runtime 当前官方 contract 保持一致。
- 明确边界：本 change 只处理 `src-bridge` 自己拥有的 runtime interaction seam，不重开 Go backend ↔ Bridge 拓扑收口，也不扩成新的前端产品面板或 IM UI。

## Capabilities

### New Capabilities
<!-- None. -->

### Modified Capabilities
- `bridge-agent-runtime-registry`: runtime catalog 从平面 feature list 提升为可消费的 interaction capability matrix，并要求 readiness / unsupported diagnostics 与实际 upstream runtime 合同一致。
- `bridge-cross-runtime-extensions`: 收紧跨 runtime 的 request/event/lifecycle 扩展，明确哪些 controls 是 canonical、哪些需要显式 unsupported / degraded 反馈，以及它们如何在 catalog 与 HTTP surface 上发布。
- `bridge-claude-advanced-features`: 补齐 Claude Code 最新 hooks/subagents/thinking/live control 相关 requirement，使 Bridge 对 Claude Agent SDK 的交互面与官方文档保持一致。
- `bridge-codex-advanced-features`: 补齐 Codex config / MCP / approvals / sandbox / session interaction requirement，使 Bridge 对 Codex CLI 的交互面不再只依赖零散 flag 假设。
- `bridge-opencode-advanced-features`: 补齐 OpenCode server-backed session/provider/auth/command/message/permission requirement，使 Bridge 对 OpenCode server 与 SDK 的控制面覆盖完整。
- `bridge-http-contract`: 收紧 Bridge 发布 runtime interaction controls、messages/command/model/callback routes、以及 capability metadata 的 canonical HTTP contract。

## Impact

- **TS Bridge runtime layer**: `src-bridge/src/runtime/registry.ts`, `src-bridge/src/runtime/backend-profiles.ts`, `src-bridge/src/types.ts`, `src-bridge/src/schemas.ts`。
- **Runtime handlers / transports**: `src-bridge/src/handlers/claude-runtime.ts`, `src-bridge/src/handlers/codex-runtime.ts`, `src-bridge/src/handlers/opencode-runtime.ts`, `src-bridge/src/opencode/transport.ts`, `src-bridge/src/server.ts`。
- **Verification**: `src-bridge` focused tests, route tests, runtime contract fixtures, upstream-doc-grounded conformance checks。
- **Upstream consumers**: Go backend、前端 runtime selector/operator surfaces、以及任何消费 `/bridge/runtimes` / advanced runtime routes 的调用方将得到更稳定且更 truthfully documented 的能力发布面。
- **External systems**: Claude Code / Claude Agent SDK、Codex CLI、OpenCode server / SDK 的官方 contract 将成为这条 change 的校准基线。
