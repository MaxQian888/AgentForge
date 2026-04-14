## Why

AgentForge 已经分别落地了 Go backend、TS Bridge、IM Bridge、以及多种 external coding-agent runtime 的基础能力，但这些后端连接合同分散在多条已归档 change 与多个 spec 中，缺少一条端到端的“连接完整性”定义。现在需要把真实拓扑、调用责任边界、上下文传递、回退策略与诊断面收口到同一条 change 里，避免 Go↔Bridge、Go↔IM Bridge、IM Bridge↔AI proxy、以及 external runtime 调用链继续出现局部完成但整体断点的情况。

## What Changes

- 新增一条后端连接完整性能力，明确 AgentForge 的唯一真实拓扑：Go backend 作为中枢协调 TS Bridge、IM Bridge 与 external runtimes，而不是让 TS Bridge 直接依赖 IM Bridge。
- 补齐并锁定 Go backend 到 TS Bridge 的 canonical 连接合同，覆盖 execute/status/pause/resume/health/runtime catalog/lightweight AI 调用，以及 runtime/provider/model/MCP 等执行上下文的完整传播。
- 补齐并锁定 Go backend 到 IM Bridge 的 control-plane 合同，覆盖 registration、heartbeat、targeted delivery、reply-target 绑定、progress/terminal delivery、ack/replay 与签名校验。
- 统一 IM Bridge 的能力路由规则：需要 AI/runtime diagnostics 的命令通过 Go proxy 调用 TS Bridge；需要持久化或业务工作流的命令继续走 Go backend；禁止在实现或文档层把 TS Bridge 描述成直接调用 IM Bridge。
- 补齐外部 coding-agent runtime 调用链的后端完整性要求，确保 runtime identity、launch tuple、diagnostics、status/resume、以及 IM/ops 可观测面在 Go 与 TS Bridge 之间保持一致。

## Capabilities

### New Capabilities
- `backend-bridge-connectivity`: 定义 Go backend、TS Bridge、IM Bridge、external runtimes 之间的端到端连接拓扑、责任边界、上下文传播、回退策略与诊断完整性合同。

### Modified Capabilities
- `bridge-http-contract`: 收紧 Go backend 到 TS Bridge 的 canonical `/bridge/*` 与 `/api/v1/ai/*` 代理调用面，避免路由、字段与职责漂移。
- `agent-sdk-bridge-runtime`: 扩展 Go→Bridge 执行合同，要求 runtime/provider/model/team/MCP 等执行上下文与运行身份在 external runtime 调用链中完整保留。
- `im-bridge-control-plane`: 收紧 Go backend 到 IM Bridge 的 control-plane registration、targeted delivery、reply-target 绑定、ack/replay 与实例选择语义。
- `im-bridge-ai-integration`: 明确 IM Bridge 对 AI/intent/runtime-capability 请求必须经由 Go proxy 消费 TS Bridge 能力，并定义 truthful fallback。
- `im-action-execution`: 明确 IM action 在 backend 工作流执行后必须保留 delivery/reply-target/terminal outcome 上下文，确保结果能正确回送到原 IM 会话。

## Impact

- **Go backend** (`src-go/`): `internal/bridge/client.go`、`internal/service/agent_service.go`、`internal/service/im_service.go`、`internal/service/im_control_plane.go`、`internal/service/task_progress_service.go`、`internal/service/im_action_execution.go`、`internal/server/routes.go`、相关 handlers/tests。
- **TS Bridge** (`src-bridge/`): runtime registry、execute/status/diagnostics/AI route handlers、request schema、runtime identity/status metadata、focused tests。
- **IM Bridge** (`src-im-bridge/`): API client、startup/control-plane wiring、capability routing、command/action/result delivery、reply-target propagation、focused tests。
- **OpenSpec / docs**: 需要把“Go backend 是中枢，TS Bridge 不直接调用 IM Bridge”的真实拓扑写成 source of truth，并补齐端到端验证要求。
- **External systems**: Claude Code、Codex、OpenCode 及其他 CLI-backed runtimes 的 readiness/diagnostics 将被纳入统一的 backend connectivity contract。
