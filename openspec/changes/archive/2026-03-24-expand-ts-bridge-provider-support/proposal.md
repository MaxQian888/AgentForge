## Why

AgentForge 的架构文档已经把 TypeScript Bridge 定义为所有后端 AI 调用的统一出口，并明确提到可以通过 Vercel AI SDK 扩展多 provider；但当前仓库里 `src-bridge` 仍只有 Claude Agent SDK 的单一路径，轻量 AI 接口如任务分解仍是模拟结果，Go 侧透传的 `provider`/`model` 字段也没有形成真实、可校验的 Bridge 合同。现在补齐这条能力，可以把“多 provider”从文档目标态落到真实运行时，避免后续在 Go 或前端再次长出各自独立的 LLM 接入分叉。

## What Changes

- 在 `src-bridge` 增加统一的 provider 注册与解析层，明确默认 provider/model、环境变量配置、能力矩阵，以及不同 AI 路径可用的真实 provider。
- 在轻量 AI 调用路径中引入 Vercel AI SDK 及其 provider 包，先为任务分解等非 Agent 运行时场景接入真实 provider，而不是继续返回模拟结果。
- 将 `provider`/`model` 纳入 Bridge 的请求 schema 与错误语义，使 Go 到 Bridge 的请求在执行前就能得到一致的 provider 校验、默认值补全和不支持能力的显式拒绝。
- 保持 Claude Agent SDK 作为 Agent 执行路径的现有真实运行时，同时定义它与 Vercel AI SDK provider 路径之间的职责边界，避免把所有调用强行塞进同一执行器。
- 为后续继续扩更多 provider 留出稳定扩展点，包括 provider 配置约定、模型命名规范、成本计量入口和 focused verification 范围。

## Capabilities

### New Capabilities
- `bridge-provider-support`: 为 TypeScript Bridge 定义统一的 provider 注册、默认选择、能力校验和轻量 AI provider 执行路径，并以 Vercel AI SDK 作为多 provider 扩展基础。

### Modified Capabilities
- `agent-sdk-bridge-runtime`: 执行请求需要与新的 provider/model 合同对齐，明确 Agent 运行时支持范围、默认 provider 解析规则，以及不支持 provider 时的拒绝语义。
- `task-decomposition`: 任务分解需要从模拟输出升级为通过 Bridge provider 层发起的真实 AI 调用，并定义 provider 选择、失败语义和最小可验证输出约束。

## Impact

- Affected code: `src-bridge/src/server.ts`, `src-bridge/src/schemas.ts`, `src-bridge/src/types.ts`, `src-bridge/src/handlers/decompose.ts`, 新增 provider registry/config/runtime 模块，以及 `src-bridge/package.json` 依赖。
- Affected integrations: `@anthropic-ai/claude-agent-sdk`, `ai`, `@ai-sdk/anthropic`, 以及首批选定的额外 provider 包（如 `@ai-sdk/openai`、`@ai-sdk/google`）。
- Affected backend contracts: `src-go/internal/bridge/client.go` 与上游服务传入的 `provider`/`model` 字段语义，需要和 Bridge 校验规则保持一致。
- Affected behavior: 任务分解等轻量 AI 路径将从模拟响应切换到真实 provider 调用；Agent 执行路径则会保留 Claude Agent SDK 主路径，但对 provider 支持范围给出显式约束。
