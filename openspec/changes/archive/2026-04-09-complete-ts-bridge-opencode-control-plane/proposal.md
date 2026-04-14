## Why

`src-bridge` 已经把 OpenCode 接到真实的 server-backed runtime 上，但 OpenCode 这条线仍然缺少最后一段 control-plane 完整性：provider auth 只会报错不会引导、permission response 只停留在 catalog/route 壳子、而 paused session 的 diff/messages/shell/command 等控制仍依赖 active pool。现在如果不把这些缺口补齐，Bridge 会继续出现“catalog 说支持、实际调用却断线或 404”的不真实行为。

## What Changes

- 补齐 OpenCode provider auth / OAuth 控制面，让 Bridge 能把“需要认证”从 blocking unavailable 提升为可操作的 auth-required 状态，并完成认证往返闭环。
- 补齐 OpenCode permission request / response 闭环，把上游 permission 事件、Bridge pending state、以及 OpenCode `permissions/:id` 响应路径真实接通。
- 让 OpenCode 的 canonical interaction routes 在任务暂停后仍能通过 persisted continuity 解析到同一 upstream session，覆盖 messages、diff、command、shell、revert 等 server-backed controls。
- 收紧 OpenCode runtime catalog 的 truthfulness：agents/skills/providers/auth readiness/session controls 不能再以 best-effort 缺失静默降级，而要显式发布 degraded diagnostics 与缺失原因。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `bridge-opencode-advanced-features`: OpenCode auth、permission、paused-session controls 与 catalog metadata 的要求需要从“具备基础 route/transport”提升为完整 control-plane contract。
- `bridge-agent-runtime-registry`: OpenCode catalog 需要显式发布 discovery failure、provider-auth readiness 与 session-control availability，不能把缺失元数据伪装成“不存在”。
- `bridge-http-contract`: canonical Bridge interaction routes 需要对 OpenCode paused session 保持 continuity-aware control，而不是只在 active runtime pool 中查找任务。
- `opencode-runtime-bridge`: OpenCode continuity 需要覆盖暂停后的 session-bound control operations，而不仅是 execute/pause/resume 的基础绑定。

## Impact

- Affected code: `src-bridge/src/opencode/transport.ts`, `src-bridge/src/handlers/opencode-runtime.ts`, `src-bridge/src/runtime/registry.ts`, `src-bridge/src/server.ts`, `src-bridge/src/session/manager.ts`, 以及对应测试。
- Affected APIs: `/bridge/runtimes`, `/bridge/permission-response/:request_id`, `/bridge/messages/:task_id`, `/bridge/diff/:task_id`, `/bridge/command`, `/bridge/shell`, 以及新增或补齐的 OpenCode provider-auth handshake surface。
- Affected behavior: OpenCode readiness diagnostics、provider authentication flow、permission callbacks、paused-session control continuity、以及 runtime catalog truthfulness。
