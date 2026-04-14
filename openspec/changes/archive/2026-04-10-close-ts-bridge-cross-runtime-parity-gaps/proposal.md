## Why

TS Bridge 的 canonical `/bridge/*` 路由、Go proxy、以及 cross-runtime registry 已经基本成型，但当前仍有几处会直接破坏合同真实性的剩余缺口：OpenCode 执行路径会静默丢失部分 ExecuteRequest 扩展字段，rollback 在不同 runtime 上没有真正闭环，runtime catalog 也会把某些高级控制能力发布得比真实可用范围更乐观。现在收口这些问题，能避免上游 Go/前端/IM 面继续围绕错误能力假设扩展，并把已经存在的 Bridge 高级控制面变成可依赖的系统合同。

## What Changes

- 收口 `ExecuteRequest` 在 Claude、Codex、OpenCode 之间的剩余 parity 缺口，重点补齐 OpenCode 对 `attachments`、`env`、`web_search` 等扩展字段的真实映射或显式拒绝语义，避免静默丢字段。
- 完成 canonical `/bridge/rollback` 的 runtime-specific 闭环，让 Claude、Codex、OpenCode 都通过各自 continuity / upstream control path 返回真实结果，而不是在部分 runtime 上长期停留在 blanket unsupported。
- 让 `/bridge/runtimes` 发布的 `interaction_capabilities` 与 runtime readiness、provider-auth、以及 live control 真相保持一致，避免 catalog 先宣称支持、实际 route 再临时报 unsupported 的漂移。
- 用 focused tests 固化 parity-sensitive 执行输入、rollback 控制、以及 capability metadata truthfulness，确保后续继续扩 runtime 时不会把这些 contract 再次回退。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `bridge-cross-runtime-extensions`: 补齐跨 runtime 的 ExecuteRequest parity 与 canonical rollback 闭环，确保扩展输入和控制路由不会被静默降级。
- `bridge-agent-runtime-registry`: 收紧 runtime catalog 的 interaction capability 发布逻辑，使其反映真实 readiness、auth、和 live control 可用性。
- `bridge-opencode-advanced-features`: 扩展 OpenCode 对 parity-sensitive 执行输入与 continuity-backed rollback 的支持，使其不再只是部分 control-plane 完整。
- `bridge-codex-advanced-features`: 补全 Codex 在 canonical rollback/control contract 下的行为与诊断语义，和现有 fork/web-search 能力保持同一合同层级。

## Impact

- Affected code: `src-bridge/src/runtime/registry.ts`, `src-bridge/src/handlers/claude-runtime.ts`, `src-bridge/src/handlers/codex-runtime.ts`, `src-bridge/src/handlers/opencode-runtime.ts`, `src-bridge/src/opencode/transport.ts`, `src-bridge/src/server.ts`，以及对应的 runtime/server tests。
- Affected contracts: `/bridge/execute`, `/bridge/rollback`, `/bridge/runtimes`, `/bridge/permission-response/*`, `/bridge/opencode/provider-auth/*` 的 route/capability 对齐语义。
- Affected upstreams: Go backend 与前端/IM 这类 catalog consumer 会拿到更真实的 runtime capability / degraded reason，但不需要改 canonical route family。
- Dependencies: 依赖现有 Claude Query、Codex connector、OpenCode transport 与 continuity snapshot 机制；不计划引入新的外部依赖。
