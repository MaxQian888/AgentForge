## Why

AgentForge 的产品与技术文档已经把“AI 分解任务”定义为核心链路，并明确要求所有轻量 AI 调用统一经过 TypeScript Bridge；但当前运行时只落了 `/bridge/execute` 等 Agent 执行接口，Go 任务 API 和 IM `/task` 命令都还没有任务分解入口。现在补齐这条能力，可以把文档中的目标态收敛成可实现、可验证的交付边界，避免后续在 Go 侧再次引入直接调用 LLM 的分叉。

## What Changes

- 在 `src-bridge` 增加轻量 AI task decomposition 请求/响应 schema、`/bridge/decompose` HTTP 接口，以及与现有 Agent runtime 分离的执行路径。
- 在 `src-go` 增加任务分解编排能力，包括调用 Bridge、校验父任务可分解、生成/持久化子任务、返回结构化分解结果，并暴露 `POST /api/v1/tasks/:id/decompose`。
- 在 `src-im-bridge` 增加 `/task decompose <id>` 子命令与用户反馈文案，使 IM 入口能够触发任务分解并回显结果。
- 对齐三端的接口命名、错误语义和非目标范围，明确本次只交付“按已有任务生成子任务”的分解链路，不扩展自然语言意图识别或自动派单。

## Capabilities

### New Capabilities
- `task-decomposition`: 允许用户从现有任务触发 AI 分解，由 Go 后端统一编排并通过 TypeScript Bridge 获取分解结果，再以结构化子任务和可读摘要返回给 API 与 IM 调用方。

### Modified Capabilities

None.

## Impact

- Affected code: `src-bridge/src/server.ts`, `src-bridge/src/schemas.ts`, 新增轻量 AI handler/service；`src-go/internal/bridge`, `src-go/internal/service`, `src-go/internal/handler`, `src-go/internal/server`; `src-im-bridge/client`, `src-im-bridge/commands/task.go`
- Affected APIs: `POST /bridge/decompose`, `POST /api/v1/tasks/:id/decompose`, `/task decompose <id>`
- Dependencies/systems: TypeScript Bridge 作为统一 AI 出口的职责边界、任务仓储对子任务创建的支持、IM Bridge 到 Go API 的任务命令映射