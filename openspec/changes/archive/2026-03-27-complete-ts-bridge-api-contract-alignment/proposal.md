## Why

AgentForge 当前关于 TS Bridge 的文档和实际使用已经出现明显漂移：PRD 与部分设计文档里同时保留了 `/api/*`、`/agent/*`、gRPC 契约示意，而当前 `src-go` 与 `src-bridge` 的真实稳定交互主要依赖 `/bridge/*` 以及少量兼容别名。这会让后续功能接入、调用方实现、测试和运维说明继续沿着不同接口面扩散，既不符合“Bridge 是统一 AI 出口”的项目文档目标，也会让功能被错误使用。

## What Changes

- 为 TS Bridge 定义一套统一的 HTTP + WebSocket 契约，覆盖 agent 执行、轻量 AI 调用、运行时目录、健康与控制面入口，并明确哪些路径是 canonical contract、哪些只是兼容别名。
- 收敛 Go Orchestrator、Bridge 自身路由注册、测试样例和项目文档对 TS Bridge 的使用方式，避免继续同时传播 `/api/*`、`/agent/*`、`/bridge/*` 三套说法。
- 明确兼容策略：现有兼容路径可以保留用于平滑迁移，但新实现、主文档和上游调用方必须围绕统一契约工作，不能再依赖历史漂移接口。
- 补齐 TS Bridge 轻量 AI 路径与 agent runtime 路径的接口边界说明，确保任务分解、意图识别、文本生成与 agent 执行都通过正确的入口被使用。
- 为后续 `/opsx:apply` 阶段提供 focused verification 目标，验证 Go client、Bridge route surface、文档与测试约束已经一致。

## Capabilities

### New Capabilities
- `bridge-http-contract`: 定义 TS Bridge 的统一 HTTP/WS API surface、canonical route family、兼容别名策略，以及调用方和文档必须遵循的使用约束。

### Modified Capabilities
- `agent-sdk-bridge-runtime`: 将 agent 执行、状态、取消、恢复、健康相关要求收敛到统一的 bridge contract，并明确 legacy alias 只作为兼容路径而不是新的主要接口。
- `bridge-provider-support`: 将轻量 AI 路径的 provider-aware 调用入口与统一 bridge contract 对齐，明确任务分解、意图识别、文本生成等能力应通过哪些 canonical endpoints 被调用。

## Impact

- Affected bridge code: `src-bridge/src/server.ts`, 相关 handler/schema/test 文件，以及任何依赖 route alias 的内部调用点。
- Affected Go code: `src-go/internal/bridge/client.go` 与其测试，以及任何直接描述或封装 Bridge HTTP 接口的服务代码。
- Affected docs: `docs/PRD.md`、`docs/part/AGENT_ORCHESTRATION.md`、`docs/part/PLUGIN_SYSTEM_DESIGN.md`、`docs/part/TECHNICAL_CHALLENGES.md`、README/运行文档中关于 TS Bridge API 的描述。
- Operational impact: 后续新增调用方、调试脚本、测试用例和文档示例需要遵循统一接口面，减少错误接入和跨文档漂移。
