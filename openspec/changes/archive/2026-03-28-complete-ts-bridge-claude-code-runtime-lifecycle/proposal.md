## Why

AgentForge 已经把 `claude_code` 作为 TS Bridge 的默认 coding-agent runtime 接入，但当前 Claude 运行时闭环仍停留在“能启动一次 execute”的层级。实际代码里 pause/resume 主要依赖重新提交原始 request，快照没有保留足够的 Claude 会话连续性元数据，且 Bridge 对 Claude launch tuple 与连接诊断的表达也还不完整，导致“连接 Claude Code 并稳定继续执行”这件事没有成为可信产品合同。

现在需要把这条链路补成一个聚焦 change，避免它继续和更广义的 Bridge API surface、前端 dashboard、或多 runtime 产品化工作混在一起。这样后续实现时可以直接围绕 Claude Code 的启动、连续性、恢复和诊断闭环推进，而不是再做一轮大而散的 TS Bridge 补齐。

## What Changes

- 把 TS Bridge 连接 Claude Code 的 runtime contract 收敛成完整生命周期：启动、运行中快照、暂停、恢复、取消、预算终止和失败诊断都围绕同一条 Claude session 真相工作。
- 明确 Bridge 在执行 `claude_code` 时必须把已解析的 runtime/provider/model、权限模式、工具/MCP 配置、以及支持的 Claude 会话连续性参数完整传给底层 runtime adapter，而不是只保留最小 execute 字段。
- 补齐 Claude runtime continuity snapshot contract，使 pause、budget stop、runtime error 之后保存的是可用于后续恢复或诊断的真实连续性元数据，而不是仅能重新跑一次初始 prompt 的浅层快照。
- 定义 Bridge resume 语义：优先恢复已有 Claude continuity state；只有在明确不支持恢复时才返回显式错误或回退策略，不能把“重新 execute 同一 request”继续伪装成完整 resume。
- 收紧 Claude runtime diagnostics 和状态元数据，确保 Go 或操作面能区分缺凭据、缺少连续性状态、恢复前置条件不满足、以及 Claude launch tuple 不完整等失败原因。
- 补齐聚焦验证与文档，覆盖 Claude Code runtime lifecycle 的 canonical route、状态元数据稳定性、以及 pause/resume/terminal snapshot 行为。

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `agent-sdk-bridge-runtime`: Claude-backed runtime contract now needs truthful lifecycle completeness, including full launch tuple mapping, continuity-state persistence, and resume semantics that preserve Claude session identity instead of replaying a fresh execute request.

## Impact

- Affected TS Bridge runtime code: `src-bridge/src/handlers/claude-runtime.ts`, `src-bridge/src/handlers/execute.ts`, `src-bridge/src/server.ts`, `src-bridge/src/runtime/agent-runtime.ts`, `src-bridge/src/session/*`, and related schema/type files.
- Affected verification surface: `src-bridge/src/server.test.ts`, `src-bridge/src/handlers/claude-runtime.test.ts`, `src-bridge/src/handlers/execute.test.ts`, and any focused lifecycle/resume coverage added for Claude runtime behavior.
- Affected bridge contract/docs: `openspec/specs/agent-sdk-bridge-runtime/spec.md`, any runtime lifecycle documentation that currently treats replay-based resume as sufficient, and operator-facing notes that describe Claude Code readiness or recovery behavior.
