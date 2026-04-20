# OpenAgents Python SDK 集成设计

**日期**: 2026-04-20
**状态**: 设计已批准，待实施规划
**作者**: brainstorming session (Max Qian + Claude)
**目标仓库**: AgentForge (`D:\Project\AgentForge`)
**被集成方**: OpenAgents Python SDK (`D:\Project\openagent-python-sdk`)

## 背景

AgentForge 当前的 agent 执行路径是 `Frontend → Go(7777) → TS Bridge(7778) → runtime adapters (claude_code / codex / opencode / cursor / gemini / qoder / iflow)`。Bridge 的 runtime adapter 本质是"让外部 coding agent CLI 跑起来"的适配器，把 LLM 调用委托给各家客户端。

OpenAgents Python SDK 是一个"单 agent 执行内核"，提供 ReAct / PlanExecute / Reflexion 等 agent pattern、8 个插件扩展点（memory、pattern、tool、tool_executor、context_assembler、runtime、session、events）、config-as-code 的 agent 定义，以及对 Python 生态（MCP、mem0、OpenTelemetry 等）的原生支持。

用户意图是把 OpenAgents SDK 作为 AgentForge 的**第二条编排路径**，和现有 Bridge **共存**。动机全覆盖：

1. 丰富执行模式（ReAct / PlanExecute / Reflexion）
2. 统一 agent 定义（config-as-code）
3. Python 生态接入
4. 其他（未来扩展空间）

## 核心决策

| # | 决策点 | 结论 |
|---|---|---|
| 1 | 集成策略 | 共存，不替代（Bridge 和 OpenAgents 并行） |
| 2 | 架构切分 | 双轨分治——Go 直连 Python，不经过 Bridge |
| 3 | 进程生命周期 | 第 4 个 sidecar，端口 7780，PyInstaller 打包 |
| 4 | Session/Memory 归属 | 混合——transcript/artifacts 回写 Postgres，checkpoint 本地 SQLite |
| 5 | MCP 工具调用链 | Python → Go → Bridge（保持"Python 只和 Go 说话"） |
| 6 | RoleConfig 映射 | 混合——静态字段预生成 agent.json；动态字段运行时注入 |
| 7 | 不可映射字段 | 一期延迟（file/network permissions、output_filters） |
| 8 | Pattern 配置位置 | role.yaml 里声明 |
| 9 | 前端引擎选择 | Role 驱动隐式路由（前端只选 role） |
| 10 | 事件映射 | 扩充 AgentEventType，新增 5 种 agent.* 类型 |
| 11 | Worktree | openagent 默认不分配，role 显式 opt-in |
| 12 | Cost 归属 | Go 持有价格表；Python 只上报 token 数 |

## 架构

```
前端 ──► Go(7777) ──┬──► TS Bridge(7778) ──► claude_code/codex/opencode/...
                    └──► Python sidecar(7780, openagent-runtime)

Python tool proxy ──HTTP──► Go /internal/tools/invoke ──► Bridge /bridge/tools/invoke
Python session writes ──HTTP──► Go /internal/sessions/* ──► Postgres
Python events ──WS──► Go ──► 前端（扩充的 AgentEventType）
```

关键属性：

- TS Bridge 代码**不改动**，继续服务现有 7 个 runtime
- Go 新增 `PythonAgentClient`，与 `BridgeClient` 并列
- Python sidecar 内部是 FastAPI + uvicorn + OpenAgents Runtime
- 所有跨进程通信都经过 Go（单一编排中枢）

## 组件设计

### 1. Python sidecar (`src-openagent/`)

目录：`src-openagent/`（和 `src-go/` / `src-bridge/` / `src-im-bridge/` 并列）。

FastAPI 服务暴露以下端点：

- `POST /run` — 接受 AgentForge `ExecuteRequest`，内部映射 `RunRequest`；NDJSON 流式响应（每行一个事件 + 最终 RunResult）
- `GET /health` — 返回 `{status, uptime_ms, loaded_agents_count, active_runs}`
- `POST /sessions/:id/cancel` — 取消指定 run
- `POST /agents/reload` — 从 `$APPDATA/agentforge/openagent-agents/` 重新加载 agent.json
- `GET /agents` — 列出已加载的 agent 配置（debug 用）

内部组件：

- 自定义 `SessionManagerPlugin` (`AgentForgeSessionManager`)：transcript/artifacts 通过 `httpx` 异步回写 Go；checkpoint 走本地 SQLite（SDK 内置实现）
- 自定义 `ToolPlugin` (`AgentForgeToolProxy`)：所有工具调用代理到 Go
- 自定义 `EventBusPlugin` (`AgentForgeEventForwarder`)：订阅全部 OpenAgents 事件，翻译后通过 WS 推给 Go
- Agent 注册表：启动时扫 `$APPDATA/agentforge/openagent-agents/*.json`，`POST /agents/reload` 触发重扫

### 2. Go 后端改动 (`src-go/`)

新增/修改的文件：

- `internal/openagent/client.go` — 新建。`PythonAgentClient`，签名和 `BridgeClient` 对称（`Execute` / `Cancel` / `Health` / `ReloadAgents`）
- `internal/openagent/agent_config_writer.go` — 新建。监听 RoleStore 变化，生成/更新 `agent.json` 到 `$APPDATA/agentforge/openagent-agents/<role_id>.json`
- `internal/openagent/role_mapper.go` — 新建。`RoleConfig → AgentDefinition dict` 的映射逻辑
- `internal/service/agent_service.go` — 修改 spawn 路径：根据 `role.ExecutionEngine` 字段路由到 `BridgeClient` 或 `PythonAgentClient`
- `internal/model/role_manifest.go` — 新增字段 `ExecutionEngine`、`OpenagentPattern`、`RequiresWorktree`
- `internal/handler/internal_handler.go` — 新建。`/api/v1/internal/*` endpoint 组，仅接受 localhost + bearer token
- `internal/bridge/client.go` — 扩展（如需）：暴露工具调用代理方法供 internal handler 使用（Bridge 侧已有 `/bridge/tools/*` 端点和 `ToolPluginManager.invokeTool`，Go client 若缺对应方法则补齐）
- `internal/ws/event_types.go`（或对应位置）— 新增 5 种 `AgentEventType`

新增 internal HTTP 端点（仅 localhost，共享 token 鉴权）：

- `POST /api/v1/internal/tools/invoke` — 来自 Python 工具代理；Go 做 RBAC/预算/审计，转发 Bridge
- `POST /api/v1/internal/sessions/:id/transcript` — 追加对话消息
- `POST /api/v1/internal/sessions/:id/artifacts` — 保存生成的 artifact
- `POST /api/v1/internal/sessions/:id/events` — 兜底事件上报（主通道仍是 WS）

### 3. Role Manifest 扩展

`role.yaml` 新增 3 个顶层字段：

```yaml
metadata:
  id: planner-01
  name: Planning Specialist

execution_engine: openagent  # bridge | openagent，默认 bridge
openagent_pattern: plan_execute  # react | plan_execute | reflexion，engine=openagent 时必填
requires_worktree: false  # openagent 默认 false，bridge 默认 true

# 原有字段保持不变
goal: "..."
backstory: "..."
# ...
```

### 4. Agent.json 生成规则

Go 在 role 注册或更新时，把 `RoleConfig` 映射并写入 `$APPDATA/agentforge/openagent-agents/<role_id>.json`：

| AgentForge RoleConfig | OpenAgents AgentDefinition |
|---|---|
| `role_id` | `id` |
| `name` | `name` |
| `openagent_pattern` | `pattern.type` |
| `goal` + `backstory` + `system_prompt` | 拼接后作为 LLM system prompt |
| `allowed_tools` + `plugin_bindings` | `tools[]`，每项 `{id, impl: "agentforge_tool_proxy", config: {tool_id, plugin_id}}` |
| `max_turns` | `runtime.max_steps` |
| `max_budget_usd` | **不映射**（Go 单一预算表；Python 在 RunRequest.budget 里只接收 max_steps） |
| 无 | `memory.type: "window_buffer"`（默认，后续可配）|
| 无 | `session.type: "agentforge"`（自定义 impl）|
| 无 | `events.type: "agentforge"`（自定义 impl）|

**不映射的字段（一期延迟）**：`file_permissions` / `network_permissions` / `output_filters`。Go 侧路由时若检测到这些字段且 `execution_engine=openagent`，记录 warn 日志。

**动态字段（不进 agent.json，随 `/run` 请求传入）**：`session_id`、`input_text`、`knowledge_context`（作为 context_hints）、运行时 budget overrides。

### 5. 工具调用链

```
Python agent (invoke tool)
  → AgentForgeToolProxy.invoke(params, context)
    → httpx.post("http://localhost:7777/api/v1/internal/tools/invoke", {
        tool_id, plugin_id, params, run_context: {task_id, session_id, role_id}
      })
      → Go internal handler
        → 预算检查 / RBAC 验证 / 审计日志
        → Bridge.InvokeTool(...) (复用现有 MCPClientHub)
        → 结果回传
      ← response
    ← tool result
  ← Python agent 继续 loop
```

**MCP 进程单例性保持在 Bridge 侧**，Python 不直接连 MCP。

### 6. Session 回写

```
Python 每次 transcript 更新
  → AgentForgeSessionManager.append_message(session_id, message)
    → httpx.post("http://localhost:7777/api/v1/internal/sessions/:id/transcript", message)
      → Go 写 Postgres (复用现有 session/message repo)
```

Artifact 同理。Checkpoint **不**回写 Go——直接写 `$APPDATA/agentforge/openagent-checkpoints.db`（SDK 内置 SQLite session 后端）。

### 7. 事件翻译表

| OpenAgents event | AgentForge AgentEventType |
|---|---|
| `tool.called` | `tool_call` |
| `tool.succeeded` / `tool.failed` | `tool_result` |
| `llm.succeeded` | `output` |
| `llm.delta`（run_stream） | `partial_message` |
| `usage.updated` | `cost_update` |
| `session.run.started` / `run.completed` | `status_change` |
| `run.checkpoint_saved` | `agent.checkpoint_saved` **(新增)** |
| `pattern.phase` | `agent.pattern_phase` **(新增)** |
| `pattern.step_started` | `agent.pattern_step_started` **(新增)** |
| `pattern.plan_created` | `agent.plan_updated` **(新增)** |
| `memory.inject.completed` | `agent.memory_injected` **(新增)** |
| 其余 OpenAgents 事件 | 丢弃 |

**新增事件**一期前端只入库不特殊渲染，二期为 plan_execute 做可视化面板。

### 8. Cost 流转

Python 完全**不**计算 USD。每次 LLM 调用后，OpenAgents 发出 `usage.updated` 事件（含 `input_tokens` / `output_tokens` / `cached_read_tokens` / `cached_write_tokens`）。Python sidecar 翻译成 `cost_update` 事件，payload 里**只带 token 数**。Go 的 cost 模块用自己的价格表算 USD，走现有 budget 聚合逻辑。

agent.json 里**不**配 `pricing` 字段。

### 9. Worktree

- `execution_engine=bridge`：沿用现有 `WorktreeManager`，每次 spawn 都分配
- `execution_engine=openagent` + `requires_worktree=false`（默认）：不分配
- `execution_engine=openagent` + `requires_worktree=true`：分配，worktree 路径通过 `context_hints.worktree_path` 传给 Python（OpenAgents 侧的工具可以读这个 hint）

## 打包与运行

### 开发模式

- `pnpm dev:openagent` — 新建脚本，`cd src-openagent && uv run uvicorn main:app --reload --port 7780`
- `pnpm dev:backend` — 扩展现有脚本，添加 Python sidecar 的启动/健康检查/停止
- `pnpm dev:all` — 扩展现有脚本，同步起 Python sidecar

### 生产构建

- `pnpm build:backend:openagent` — 新建，调用 PyInstaller 打包跨平台单文件二进制
- `pnpm build:backend` — 扩展，包含 openagent 二进制
- Tauri `tauri.conf.json` `externalBin` 数组新增 `openagent-runtime` 条目

### 首次启动

- Go 启动时检查 `$APPDATA/agentforge/openagent-agents/` 目录是否存在，不存在则创建
- Go 读取 RoleStore，为所有 `execution_engine=openagent` 的 role 生成初始 agent.json
- Python sidecar 启动时加载全部 agent.json
- Go 通过 `POST /agents/reload` 通知 Python 热加载（role 变更时）

## 错误 / 降级

| 场景 | 行为 |
|---|---|
| Python sidecar 未就绪 | Go 收到 spawn 请求且 `role.execution_engine=openagent` 时返回 503；日志记录 |
| Python sidecar 崩溃 | Tauri 或 systemd-like supervisor 负责重启；Go 侧通过定期健康检查恢复可用性标记 |
| 工具调用回调 Go 失败 | Python 侧返回 tool error 给 agent loop；OpenAgents 的 `ModelRetryError` 机制处理重试 |
| Session 回写 Go 失败 | Python 侧 buffer + retry with backoff；超过阈值 fail run（保证数据一致性优于宽容） |
| 价格表查不到模型 | Go 记录 warn，token 继续计数，USD 记为 0，不阻塞 run（事实和策略分离） |

## 一期范围（YAGNI）

**In scope**:

- sidecar 进程 + FastAPI `/run` endpoint
- `ReAct` pattern（plan_execute / reflexion 二期）
- Role manifest 新字段 + Go dispatcher 路由
- Agent.json 生成 + 热加载
- Session/artifact 回写（transcript 必须，artifact 可选）
- 工具代理调用链
- 事件翻译（含新增 5 种 AgentEventType，前端只入库不渲染）
- Cost token 上报 + Go 侧算 USD
- Dev 模式脚本
- Tauri sidecar 打包

**Out of scope（延迟到后续迭代）**:

- `file_permissions` / `network_permissions` / `output_filters` 在 Python 路径下的实现
- `plan_execute` / `reflexion` pattern（基础设施支持但不默认启用）
- 前端为新事件类型做特化 UI（plan 步骤树、reflexion 反思面板等）
- `loaded_skills` 的完整集成（一期只透传）
- Python 自定义 memory 插件的用户级扩展

## 开放问题（交给 implementation plan 决）

1. `PluginRecord` 里的 MCP server 如何在 Python 启动时同步——是 Go 主动推送列表，还是 Python 启动时拉？（影响启动顺序依赖）
2. Python sidecar 的 Bearer token 在哪里生成/分发——Tauri 启动时注入环境变量，还是 Go 写到文件让 Python 读？
3. Windows 下 PyInstaller onefile 启动延迟（~1-2s）是否需要加启动画面 / 延迟健康检查超时？
4. Agent.json 的 schema 版本化机制（将来 pattern 增多、插件 schema 变化时的向前兼容）

## 验收标准

- 创建一个 `openagent_engine=true` + `openagent_pattern=react` 的 role，spawn 后 agent 能走完 LLM 调用 → 工具调用 → LLM 回复的完整 ReAct 循环
- 前端 agent 详情页展示对话历史（transcript 回写 Postgres 生效）
- 触发工具调用时 Bridge 的 MCPClientHub 被调用（审计日志可查）
- `cost_update` 事件到达 Go，USD 在现有预算面板正确聚合
- 中途修改 role 配置，`POST /agents/reload` 后新 spawn 使用新配置（不影响进行中的 run）
- Python sidecar kill 后 5 秒内被 Tauri 重启；Go 健康标记从 unhealthy 恢复到 healthy

---

本 spec 批准后，由 `writing-plans` skill 生成 implementation plan。
