## Why

AgentForge 已经具备 `ToolPlugin -> MCP -> TS Bridge -> Go Registry` 的基础闭环，但目前这条链路只覆盖了注册、启用、激活、健康检查和粗粒度工具列表，距离 `docs/PRD.md` 与 `docs/part/PLUGIN_SYSTEM_DESIGN.md` 中承诺的 MCP First 交互层还差一整段可操作能力。现在最主要的缺口是：控制面无法查看或刷新 MCP server 的完整能力面，也无法通过统一 API 安全地做资源读取、提示词发现、工具试调用和运行态诊断，这会让插件管理、角色绑定和后续 Workflow/Review 集成都停留在“能挂上但不可运维”的状态。

## What Changes

- 补齐 MCP 交互控制面能力，为已注册的 `ToolPlugin` 提供完整的工具、资源、提示词发现与刷新流程，而不只返回一次性的 `discovered_tools` 快照。
- 增加 Go Orchestrator 到 TS Bridge 的 MCP 操作代理能力，覆盖工具试调用、资源读取、提示词/能力枚举、按插件诊断和运行态同步。
- 补充 TS Bridge 内部 MCP 运行态模型，使其能记录最近一次发现结果、最近一次调用摘要、失败原因、刷新时间、连接传输信息和能力计数。
- 扩展插件注册表与运行态同步契约，让操作员能从统一插件记录中看到 MCP 交互相关的运行元数据，而不需要直接依赖 Bridge 进程内状态。
- 新增面向调试与运维的 API 契约，明确哪些 MCP 操作允许人工触发、如何做权限校验、如何回传调用结果与错误，以及哪些信息必须进入审计事件。
- 为 MCP 交互控制面补齐 focused 验证与文档，确保当前仓库记录的“真实可用 MCP 交互面”与 PRD/插件设计文档对齐。

## Capabilities

### New Capabilities
- `mcp-interaction-surface`: 定义 ToolPlugin 的 MCP 能力发现、资源/提示词枚举、工具试调用、资源读取、刷新诊断与审计回传契约。

### Modified Capabilities
- `plugin-runtime`: 扩展 TS Bridge 托管的 ToolPlugin 运行时要求，使其不仅跟踪生命周期，还要暴露可刷新的 MCP 能力面、交互诊断和最近操作摘要。
- `plugin-registry`: 扩展注册表要求，使其持久化并展示 MCP 相关运行元数据、最近同步快照和操作员可见的交互状态，而不是只保留基础生命周期字段。

## Impact

- Affected Go code: `src-go/internal/bridge/client.go`, `src-go/internal/service/plugin_service.go`, `src-go/internal/handler/plugin_handler.go`, `src-go/internal/server/routes.go`, 以及相关插件模型/仓储代码
- Affected TS bridge code: `src-bridge/src/mcp/*`, `src-bridge/src/plugins/*`, `src-bridge/src/server.ts`, `src-bridge/src/ws/event-stream.ts`
- Affected APIs: `/api/v1/plugins/*` 相关管理接口、`/internal/plugins/runtime-state` 同步载荷、以及新增的 MCP 发现/读取/试调用控制面接口
- Affected operators flows: ToolPlugin 安装后诊断、能力检查、资源读取、提示词发现、工具试调用、故障排查与审计事件观察
- Verification impact: 需要新增 Go/TS focused tests，覆盖 MCP 能力发现、刷新、交互代理、错误回传和注册表同步场景
