## Context

TS Bridge (`src-bridge/`) 已实现完整的路由体系和核心能力（Agent 执行、Runtime/Provider 管理、Plugin/MCP 集成、轻量 AI 操作），但 Go 后端（`src-go/`）的 service 层仅调用了 18 个 bridge client 方法中的 7 个，前端单 Agent 执行缺少运行时配置入口。本设计补齐三层联动：Go service → Bridge API → 前端展示/操作。

现有架构层次：
```
Frontend (React/Next.js, port 3000)
  ↓ REST / WebSocket
Go Backend (port 7777)
  ↓ HTTP (bridge client)
TS Bridge (port 7778)
  ↓ Claude Agent SDK / MCP / Vercel AI SDK
```

## Goals / Non-Goals

**Goals:**
- Go service 层补齐 Bridge 健康探针、status polling、runtimes catalog、generate、classify-intent 的调用链路
- 前端单 Agent spawn 复用 team dialog 中的运行时选择组件，提供完整配置
- 前端增加 Bridge 运行时仪表盘（runtimes、pool、health），集成到 Agents 页
- 前端补齐 Agent 暂停/恢复操作工作流
- Bridge 侧和 Go client 侧测试补齐

**Non-Goals:**
- 不修改 Bridge 路由定义或核心逻辑（Bridge 功能本身已完整）
- 不新增 Codex/OpenCode runtime adapter 实现（仅注册在 catalog 中）
- 不改变 WebSocket 事件流协议
- 不涉及 IM Bridge（`src-im-bridge/`）的变更
- 不做 Plugin 安装/管理的新 UI（已有 Plugin 管理面板）

## Decisions

### D1: Bridge 健康探针采用 Go service 层周期探测 + 状态下沉

Go 后端新增 `BridgeHealthService`，启动时等待 Bridge 就绪（readiness probe with retry），运行中每 30s 调用 `bridge.Health()`，状态写入内存。Handler 层通过 `GET /api/v1/bridge/health` 暴露给前端。

**替代方案**: Bridge 主动向 Go 注册心跳 → 增加 Bridge 侧改动，不符合"Bridge 无需修改"的约束。

### D2: 提取 RuntimeSelector 为共享组件

当前 `StartTeamDialog` 内联了运行时/Provider/Model 选择逻辑。提取为 `<RuntimeSelector>` 共享组件，同时用于：
- `StartTeamDialog`（现有）
- 新增 `SpawnAgentDialog`（单 Agent spawn）
- Agents 页面的 Runtime Catalog 展示

**数据源**: 通过 `GET /api/v1/bridge/runtimes` 获取 catalog，存入 `agent-store` 。

### D3: Status polling 由前端 WebSocket 事件驱动，不做轮询

Go agent handler 在 spawn 后已通过 WS 收到 Bridge 事件（agent.started, progress, completed 等），状态已同步到数据库。前端通过 `ws-store` 实时收到更新。无需额外轮询 `/bridge/status/:id`。

但 Go service 层在 spawn 后需要 `GetStatus()` 做一次确认（防止 WS 断连丢消息），作为 fallback 校验。

### D4: 前端 Agent Resume 工作流

Agents 页面显示 paused agents 列表（从 agent-store 过滤 `state === 'paused'`）。每行提供 Resume 按钮，调用 `POST /api/v1/agents/:id/resume`，Go handler 调用 `bridge.Resume()`。

### D5: Generate/ClassifyIntent 作为 AI 工具端点暴露

Go handler 新增：
- `POST /api/v1/ai/generate` → `bridge.Generate()`
- `POST /api/v1/ai/classify-intent` → `bridge.ClassifyIntent()`

这些端点供 IM 处理和未来 AI 工具面板使用，前端暂不新增专用 UI。

### D6: 测试策略

- Bridge `server.test.ts`: 补齐 `/bridge/active`、`/bridge/pool` 详细场景、`/bridge/tools/*` CRUD、`/bridge/plugins/*` 生命周期测试
- Go `client_test.go`: 补齐 `GetStatus`、`Health`、`Generate`、`ClassifyIntent`、`GetRuntimeCatalog` 的 httptest mock 测试
- 前端: `SpawnAgentDialog` 和 `RuntimeSelector` 组件测试

## Risks / Trade-offs

- **[Bridge 不可用时 Go 降级]** → BridgeHealthService 标记降级状态，agent spawn 返回 503，前端展示降级 banner。不做自动重启（Bridge 由 Tauri sidecar 管理）。
- **[RuntimeSelector 提取可能影响 StartTeamDialog]** → 提取时保持 props 兼容，StartTeamDialog 内部改为引用共享组件，功能不变。
- **[Generate/ClassifyIntent 滥用]** → 端点需 auth middleware 保护，预算由 Bridge 侧 provider 层控制。
