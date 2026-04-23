# OpenAgents Python SDK 集成设计 (v2)

**日期**: 2026-04-23
**状态**: 设计已批准，待实施规划
**作者**: brainstorming session (Max Qian + Claude)
**目标仓库**: AgentForge (`D:\Project\AgentForge`)
**被集成方**: OpenAgents Python SDK v0.4.0 (`D:\Project\openagent-python-sdk`)
**前置 spec**: `2026-04-20-openagents-integration-design.md`（v1，本 spec 替代）

## 变更摘要（相对 v1）

| 项目 | v1 决策 | v2 决策 | 变更原因 |
|------|---------|---------|----------|
| 流式通信协议 | NDJSON over HTTP | **WebSocket 双向通信** | 支持中途 interrupt/pause/resume 控制指令 |
| 目录名 | `src-openagent/` | `src-openagents/` | 和 SDK 包名对齐 |
| 端口 | 7780 | **7782** | 避免和其他服务冲突 |
| 连接管理 | 无池化 | **WebSocket 连接池** | 复用连接，串行处理避免并发写帧 |
| 心跳/断连恢复 | 仅依赖 supervisor 重启 | **ping/pong + 自动重连 + durable resume** | SDK 有 durable resume 能力，应利用 |
| 工具调用链 | Python → Go → Bridge (HTTP) | 保持不变 | — |
| Session/Memory 归属 | 混合（Postgres + SQLite） | 保持不变 | — |
| Cost 归属 | Go 持有价格表，Python 只上报 token | 保持不变 | — |

## 核心决策

| # | 决策点 | 结论 |
|---|--------|------|
| 1 | 集成策略 | 并列路由，和 Bridge 共存。Employee 配置选择运行时。 |
| 2 | 架构切分 | Go 直连 Python 微服务，不经过 Bridge |
| 3 | 通信协议 | 同步用 HTTP POST，流式用 WebSocket 双向通信 |
| 4 | 进程生命周期 | 第 4 个 sidecar，端口 7782，PyInstaller 打包 |
| 5 | Session/Memory 归属 | 混合——transcript/artifacts 回写 Postgres，checkpoint 本地 SQLite |
| 6 | MCP 工具调用链 | Python → Go → Bridge（保持"Python 只和 Go 说话"） |
| 7 | RoleConfig 映射 | 静态字段预生成 agent.json；动态字段运行时注入 |
| 8 | 不可映射字段 | 一期延迟（file/network permissions、output_filters） |
| 9 | Pattern 配置位置 | role.yaml 里声明 |
| 10 | 前端引擎选择 | Employee 驱动显式路由（前端选 runtime: bridge | openagents） |
| 11 | 事件映射 | 扩充 AgentEventType，新增 5 种 agent.* 类型 |
| 12 | Worktree | openagent 默认不分配，role 显式 opt-in |
| 13 | Cost 归属 | Go 持有价格表；Python 只上报 token 数 |
| 14 | WebSocket 生命周期 | ping/pong 心跳 + 断连自动重连 + durable resume |

## 架构

```
前端 ──► Go(7777) ──┬──► TS Bridge(7778) ──► claude_code/codex/opencode/...
                    └──► Python sidecar(7782, openagents-service)
                           ├─ WebSocket (流式执行 + 中断/暂停控制)
                           └─ HTTP POST  (同步执行)

Python tool proxy ──HTTP──► Go /internal/tools/invoke ──► Bridge /bridge/tools/invoke
Python session writes ──HTTP──► Go /internal/sessions/* ──► Postgres
Python events ──WS──► Go ──► 前端（扩充的 AgentEventType）
```

关键属性：

- TS Bridge 代码**不改动**，继续服务现有 7 个 runtime
- Go 新增 `OpenAgentsClient`，与 `BridgeClient` 并列
- Python sidecar 内部是 FastAPI + uvicorn + OpenAgents Runtime
- 所有跨进程通信都经过 Go（单一编排中枢）

## 组件设计

### 1. Python 微服务 (`src-openagents/`)

目录：`src-openagents/`（和 `src-go/` / `src-bridge/` / `src-im-bridge/` 并列）。

```
src-openagents/
├── pyproject.toml
├── openagents_service/
│   ├── __init__.py
│   ├── main.py                 # FastAPI app，端口 7782
│   ├── config.py               # 服务配置
│   ├── schemas.py              # Pydantic 请求/响应模型
│   ├── runtime_manager.py      # Runtime 实例池管理
│   ├── routes/
│   │   ├── run.py              # POST /api/v1/run (同步执行)
│   │   ├── stream.py           # WS /api/v1/ws/run (WebSocket 流式)
│   │   ├── session.py          # GET/DELETE /api/v1/session/{id}
│   │   ├── validate.py         # POST /api/v1/validate
│   │   └── health.py           # GET /health
│   ├── ws/
│   │   ├── handler.py          # WebSocket 连接处理
│   │   ├── protocol.py         # 消息类型定义与编解码
│   │   └── lifecycle.py        # 连接生命周期管理 (心跳、超时)
│   ├── plugins/
│   │   ├── session_manager.py  # 自定义 SessionManager (回写 Go)
│   │   ├── tool_proxy.py       # 工具调用代理 (→ Go)
│   │   └── event_forwarder.py  # 事件转发 (→ Go via WS)
│   └── tests/
│       ├── test_run.py
│       ├── test_ws_lifecycle.py
│       └── test_integration.py
├── agents/                     # 默认 agent 配置
│   └── default.json
└── pyproject.toml
```

**依赖**：

```toml
[project]
name = "openagents-service"
requires-python = ">=3.10"
dependencies = [
    "io-openagent-sdk[openai,mcp,mem0,yaml,sqlite,tokenizers]",
    "fastapi>=0.115",
    "uvicorn[standard]>=0.30",
    "websockets>=13.0",
]
```

### 2. WebSocket 协议

端点：`WS /api/v1/ws/run`

**Go → Python 消息**：

```json
{"type": "run",          "task_id": "...", "agent_id": "...", "input_text": "...", "session_id": "...", "context_hints": {}, "metadata": {}, "budget": {}, "agent_config": {}}
{"type": "interrupt",    "task_id": "..."}
{"type": "pause",        "task_id": "..."}
{"type": "resume",       "task_id": "..."}
{"type": "close"}
```

**Python → Go 消息**：

```json
{"type": "run_started",      "task_id": "..."}
{"type": "llm_delta",        "task_id": "...", "data": {"text": "..."}}
{"type": "tool_started",     "task_id": "...", "data": {"tool_id": "...", "tool_name": "...", "params": {}}}
{"type": "tool_delta",       "task_id": "...", "data": {"tool_id": "...", "chunk": "..."}}
{"type": "tool_finished",    "task_id": "...", "data": {"tool_id": "...", "output": "...", "success": true}}
{"type": "artifact",         "task_id": "...", "data": {"name": "...", "content": "...", "metadata": {}}}
{"type": "run_finished",     "task_id": "...", "data": {"output": "...", "usage": {"prompt_tokens": 0, "completion_tokens": 0}, "stop_reason": "end_turn", "artifacts": []}}
{"type": "error",            "task_id": "...", "data": {"message": "...", "error_code": "...", "error_details": {}, "retryable": false}}
{"type": "pong"}
```

### 3. WebSocket 完整生命周期

| 阶段 | Go 行为 | Python 行为 |
|------|---------|-------------|
| 连接 | `gorilla/websocket` 拨号 `ws://localhost:7782/api/v1/ws/run` | 接受连接，等待指令 |
| 心跳 | 每 30s 发 `ping` | 回 `pong`，超时 60s 视为断连 |
| 执行 | 发送 `{type: "run", ...}` | 返回 `run_started`，开始执行 |
| 流式推送 | — | 持续发送 `llm_delta` / `tool_started` / `tool_finished` / `artifact` |
| 中断 | 发送 `{type: "interrupt"}` | 停止执行，返回 `run_finished` + `stop_reason: "interrupted"` |
| 暂停 | 发送 `{type: "pause"}` | 暂停 tool 调用和 LLM 请求，回 `run_finished` + `stop_reason: "paused"` |
| 恢复 | 发送 `{type: "resume"}` | 恢复执行（利用 SDK durable resume） |
| 正常完成 | — | 发送 `run_finished` + 完整结果 |
| 错误 | — | 发送 `error` + `error_details`（含 `error_code` + `retryable` 标记） |
| 断连 | WebSocket 读取失败，自动重连（指数退避 1s→2s→4s→max 30s） | 清理当前 run 资源 |
| 断连恢复 | 重连成功后发 `{type: "run", task_id: 原 task_id, ...}` | SDK durable resume 基于 session 恢复状态 |
| 连接关闭 | 发 `close` | 清理 session 资源，关闭连接 |

### 4. Go 后端改动 (`src-go/`)

新增模块 `internal/openagents/`：

```
src-go/internal/openagents/
├── client.go           # HTTP + WebSocket client
├── client_test.go
├── models.go           # 请求/响应/事件模型
├── pool.go             # WebSocket 连接池
├── pool_test.go
├── agent_config_writer.go  # RoleConfig → agent.json 生成
└── role_mapper.go          # RoleConfig → AgentDefinition 映射
```

**连接池**：

- 预建立 WebSocket 连接池，默认 4 个 worker
- 每个连接串行处理一个 task（避免并发写帧）
- 连接断开自动重连，指数退避（1s → 2s → 4s → max 30s）
- 空闲超时 5 分钟自动关闭
- 池满时新 task 排队等待

**路由集成**：

```go
func (s *Service) Spawn(req SpawnRequest) (*AgentRun, error) {
    switch req.Runtime {
    case "openagents":
        return s.openagentsClient.Run(req)
    default:
        return s.bridgeClient.Execute(req)
    }
}
```

- `Employee` 模型新增 `PreferredRuntime` 字段（`bridge` | `openagents`）
- 前端 employee 配置页新增运行时选择下拉
- `SpawnWithEmployee()` 自动读取 employee 的 `PreferredRuntime`
- **优先级**：Employee.PreferredRuntime > Role.execution_engine > 默认 `bridge`

**新增 internal HTTP 端点**（仅 localhost，共享 token 鉴权）：

- `POST /api/v1/internal/tools/invoke` — Python 工具代理；Go 做 RBAC/预算/审计，转发 Bridge
- `POST /api/v1/internal/sessions/:id/transcript` — 追加对话消息
- `POST /api/v1/internal/sessions/:id/artifacts` — 保存 artifact
- `POST /api/v1/internal/sessions/:id/events` — 兜底事件上报（主通道仍是 WS）

### 5. Role Manifest 扩展

`role.yaml` 新增字段：

```yaml
execution_engine: openagent          # bridge | openagent，默认 bridge
openagent_pattern: react             # react | plan_execute | reflexion
requires_worktree: false             # openagent 默认 false
```

### 6. Agent.json 生成规则

Go 在 role 注册或更新时，把 `RoleConfig` 映射并写入 agent.json：

| AgentForge RoleConfig | OpenAgents AgentDefinition |
|---|---|
| `role_id` | `id` |
| `name` | `name` |
| `openagent_pattern` | `pattern.type` |
| `goal` + `backstory` + `system_prompt` | 拼接后作为 LLM system prompt |
| `allowed_tools` + `plugin_bindings` | `tools[]`，每项 `{id, impl: "agentforge_tool_proxy"}` |
| `max_turns` | `budget.max_turns` |
| 无 | `memory.type: "window_buffer"`（默认） |
| 无 | `session.type: "agentforge"`（自定义 impl） |
| 无 | `events.type: "agentforge"`（自定义 impl） |

**不映射的字段（一期延迟）**：`file_permissions` / `network_permissions` / `output_filters`。

**动态字段（随 /run 请求传入）**：`session_id`、`input_text`、`knowledge_context`（context_hints）、budget overrides。

### 7. 工具调用链

```
Python agent (invoke tool)
  → AgentForgeToolProxy.invoke(params, context)
    → httpx.post("http://localhost:7777/api/v1/internal/tools/invoke")
      → Go internal handler → RBAC/预算/审计 → Bridge.InvokeTool
      ← response
    ← tool result
  ← agent loop 继续
```

MCP 进程单例性保持在 Bridge 侧，Python 不直接连 MCP。

### 8. Session 回写

```
Python 每次 transcript 更新
  → AgentForgeSessionManager.append_message(session_id, message)
    → httpx.post("http://localhost:7777/api/v1/internal/sessions/:id/transcript")
      → Go 写 Postgres (复用现有 repo)
```

Checkpoint 写本地 SQLite（SDK 内置）。

### 9. 事件翻译表

| OpenAgents event | AgentForge AgentEventType |
|---|---|
| `tool.called` | `tool_call` |
| `tool.succeeded` / `tool.failed` | `tool_result` |
| `llm.succeeded` | `output` |
| `llm.delta` | `partial_message` |
| `usage.updated` | `cost_update` |
| `session.run.started` / `run.completed` | `status_change` |
| `run.checkpoint_saved` | `agent.checkpoint_saved` **(新增)** |
| `pattern.phase` | `agent.pattern_phase` **(新增)** |
| `pattern.step_started` | `agent.pattern_step_started` **(新增)** |
| `pattern.plan_created` | `agent.plan_updated` **(新增)** |
| `memory.inject.completed` | `agent.memory_injected` **(新增)** |

新增事件一期前端只入库不特殊渲染。

### 10. Cost 流转

Python 不计算 USD。每次 LLM 调用后上报 token 数（`cost_update` 事件）。Go 的 cost 模块用自有价格表算 USD，走现有 budget 聚合逻辑。

## 错误处理

| 级别 | 场景 | 处理方式 |
|------|------|----------|
| 连接失败 | Python 服务未启动 / 端口不可达 | Go fail-closed，返回 503 |
| 执行超时 | 超过 budget 或 wall-clock 超时 | Python 返回 `run_finished` + `budget_exhausted` |
| LLM 错误 | API key 无效 / 限流 / 模型不可用 | Python 返回 `error` + `error_details`（含 `retryable`） |
| Tool 错误 | 工具执行失败 | Python 返回 `tool_finished` + 错误，Pattern 决定重试 |
| 进程崩溃 | Python OOM / panic | Go WS 读取失败，重连一次，失败标记 task error |
| 配置错误 | agent.json 无效 | `/validate` 提前校验 |

Go 侧错误模型：

```go
type OpenAgentsError struct {
    TaskID       string
    ErrorCode    string    // LLM_RATE_LIMIT / TOOL_FAILED / CONFIG_INVALID / ...
    Message      string
    Retryable    bool
    RetryAfterMs int       // 仅限流时有值
}
```

`Retryable=true`：Go 自动重试（最多 3 次，指数退避）。`Retryable=false`：直接写入 AgentRun 记录。

## 测试

### Python 微服务

| 层级 | 内容 | 框架 |
|------|------|------|
| 单元测试 | RuntimeManager、schemas 校验、WS 消息编解码 | pytest + httpx TestClient |
| 集成测试 | 完整 run → result 流程，用 mock LLM | pytest-asyncio |
| WebSocket 测试 | 连接/中断/暂停/恢复/断连恢复全生命周期 | pytest + websockets |

### Go 侧

| 层级 | 内容 |
|------|------|
| 单元测试 | OpenAgentsClient 用 httptest mock Python 服务 |
| 集成测试 | 启动真实 Python 服务，完整 round-trip |
| 路由测试 | Spawn 根据 runtime 字段正确分发 |

## 部署

### 开发环境

- `pnpm dev:all` / `pnpm dev:backend` 自动启动 OpenAgents 服务（端口 7782）
- 健康检查纳入 `dev:all:status` / `dev:backend:status`
- 启动顺序：PostgreSQL → Redis → Go → **OpenAgents (7782)** → Bridge (7778) → IM Bridge (7779) → Frontend

### Tauri 桌面端

- 新增 sidecar：`src-openagents/openagents-service`（PyInstaller 打包为单二进制）
- `tauri.conf.json` 新增 sidecar 配置
- `desktop:dev:prepare` 增加 `cd src-openagents && uv sync && uv run python -m openagents_service`
- `desktop:build:prepare` 增加 PyInstaller 打包步骤

### CI

- 新增 job：`cd src-openagents && uv sync && uv run pytest`
- 现有集成测试增加 OpenAgents 服务启动步骤

## 一期范围

**In scope**:

- Python 微服务 + FastAPI + WebSocket
- `ReAct` pattern（plan_execute/reflexion 二期）
- Employee 模型新增 `PreferredRuntime` + 前端下拉
- Role manifest 新字段 + Go dispatcher 路由
- Agent.json 生成 + 热加载
- Session/artifact 回写
- 工具代理调用链
- 事件翻译（含新增 5 种 AgentEventType）
- Cost token 上报 + Go 侧算 USD
- WebSocket 连接池 + 心跳 + 断连恢复
- Dev 模式脚本
- Tauri sidecar 打包

**Out of scope**:

- `file_permissions` / `network_permissions` / `output_filters` 在 Python 路径下的实现
- `plan_execute` / `reflexion` pattern 的前端特化 UI
- `loaded_skills` 的完整集成（一期只透传）
- Python 自定义 memory 插件的用户级扩展

## 验收标准

1. 创建 `execution_engine=openagents` + `openagent_pattern=react` 的 role，spawn 后 agent 能走完 LLM → 工具 → LLM 的完整 ReAct 循环
2. 前端 agent 详情页展示对话历史（transcript 回写 Postgres 生效）
3. 触发工具调用时 Bridge 的 MCPClientHub 被调用（审计日志可查）
4. `cost_update` 事件到达 Go，USD 在现有预算面板正确聚合
5. WebSocket 中途 interrupt 能正常终止执行
6. 断连后重连能恢复执行状态（durable resume）
7. `POST /agents/reload` 后新 spawn 使用新配置
8. Python sidecar kill 后 5 秒内被 Tauri 重启，Go 健康标记恢复

---

本 spec 批准后，由 `writing-plans` skill 生成 implementation plan。
