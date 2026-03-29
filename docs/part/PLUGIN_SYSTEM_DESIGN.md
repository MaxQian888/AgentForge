# AgentForge 插件系统架构设计

> 版本：v1.2 | 日期：2026-03-23
> 基于 3 个并行 Agent 调研（覆盖 9 大平台、7 种运行时、5 种分发方案）
> v1.1 更新：引入 Tauri 桌面层，补充插件间通信架构
> v1.2 更新：Go↔TS 通信改为 HTTP+WS，砍掉 gRPC/Redis，补充生命周期跨进程同步、Token 双重计费、角色配置传递原则
> v1.3 更新：确定 TS Bridge 打包方案为 Bun compile
>
> live contract note: 当前 Go↔TS Bridge 的实现入口以 canonical `/bridge/*` HTTP routes 加 WebSocket 事件流为准。本文中如果仍出现 `gRPC → TS Bridge`、旧裸路径或历史 proto 表述，应视为历史参考而不是当前实现接口。

---

## 目录

1. [设计目标与原则](#一设计目标与原则)
2. [四层运行时架构](#二四层运行时架构)
3. [插件间通信架构](#三插件间通信架构)
4. [插件类型分层](#四插件类型分层)
5. [核心架构](#五核心架构)
6. [角色插件系统（Role Plugin）](#六角色插件系统role-plugin)
7. [工具插件系统（Tool Plugin）](#七工具插件系统tool-plugin)
8. [工作流插件系统（Workflow Plugin）](#八工作流插件系统workflow-plugin)
9. [集成插件系统（Integration Plugin）](#九集成插件系统integration-plugin)
10. [审查插件系统（Review Plugin）](#十审查插件系统review-plugin)
11. [插件协议与接口规范](#十一插件协议与接口规范)
12. [安全与沙箱架构](#十二安全与沙箱架构)
13. [插件生命周期管理](#十三插件生命周期管理)
14. [插件分发与市场](#十四插件分发与市场)
15. [Plugin SDK 设计](#十五plugin-sdk-设计)
16. [与现有架构整合](#十六与现有架构整合)
17. [分阶段实施路线](#十七分阶段实施路线)
18. [数据模型扩展](#十八数据模型扩展)

---

## 一、设计目标与原则

### 1.1 设计目标

| # | 目标 | 说明 |
|---|------|------|
| 1 | **数字员工可定制** | 用户可通过角色插件快速创建具备特定能力的数字员工 |
| 2 | **能力可扩展** | 通过工具插件给 Agent 增加新能力（MCP 兼容） |
| 3 | **流程可编排** | 通过工作流插件定义多 Agent 协作模式 |
| 4 | **系统可集成** | 通过集成插件对接外部系统（IM、CI/CD、云服务） |
| 5 | **审查可自定义** | 通过审查插件添加团队专属代码审查规则 |
| 6 | **安全可控** | 所有插件在沙箱中运行，权限显式声明 |
| 7 | **生态可持续** | 提供 SDK、市场、文档，降低开发者门槛 |

### 1.2 设计原则

1. **MCP First** — Agent 工具扩展以 MCP 协议为标准，与 AI 生态兼容
2. **声明式优先** — 插件通过 YAML Manifest 声明能力和权限，减少代码量
3. **进程隔离** — 插件在独立进程/沙箱中运行，崩溃不影响平台
4. **渐进复杂度** — 简单角色只需一个 YAML 文件，复杂插件可用完整 SDK
5. **双侧一致** — Go Orchestrator 和 TS Bridge 使用统一的插件协议
6. **懒加载** — 插件按需激活，不浪费资源

### 1.3 调研结论摘要

基于对 Claude/MCP、LangChain、Dify、Coze、CrewAI、AutoGen、Composio、n8n 等平台的深入调研：

| 调研发现 | AgentForge 应对 |
|----------|----------------|
| MCP 成为 AI 工具扩展的事实标准 | 原生支持 MCP 协议 |
| Dify 的 Manifest + 签名 + 沙箱最成熟 | 借鉴其安全模型 |
| CrewAI 的 role/goal/backstory 角色隐喻最直觉 | 采用类似的角色定义模式 |
| Coze 的 Go + TS 架构与我们一致 | 参考其节点扩展机制 |
| HashiCorp go-plugin 是 Go 生态最佳实践 | Go 侧插件运行时 |
| Extism WASM 是跨语言插件的最佳方案 | 长期引入 |
| 插件市场 + 审核是成熟生态的标配 | 分阶段建设 |

### 1.4 当前 OpenSpec MVP 实现边界

这份架构文档覆盖的是插件系统的长期蓝图，但当前仓库真相已经不再停留在最早的 `establish-plugin-runtime-and-registry` MVP。到 `2026-03-29` 为止，仓库已经把统一契约、双宿主运行时映射、Go 权威注册中心、ReviewPlugin 深审扩展、WorkflowPlugin 顺序执行、SDK 与脚手架、catalog/trust 控制面、内置插件 bundle/readiness 校验，以及 repo-local 作者命令推进到同一条实现线上，但它仍然不等同于“公开 marketplace 已完成”。

本次 MVP 的可执行插件映射固定为：

| 插件类型 | 第一阶段允许运行时 | 宿主 | 当前状态 |
|---|---|---|---|
| `ToolPlugin` | `mcp` | TS Bridge | 已实现注册、MCP 能力发现、交互审计，以及 TS SDK / scaffold |
| `IntegrationPlugin` | `wasm` | Go Orchestrator | 已实现 Go-hosted WASM runtime、SDK、样例、构建/调试/verify 辅助 |
| `RolePlugin` | `declarative` | Go Registry / 配置层 | 已实现声明式兼容与注册中心收敛 |
| `WorkflowPlugin` | `wasm` | Go Orchestrator | 已实现 sequential workflow manifest 校验、运行状态查询、Go-hosted starter；其他 process mode 明确 unsupported |
| `ReviewPlugin` | `mcp` | TS Bridge | 已实现 manifest 选择、Layer 2 深审执行接入、结果 provenance 持久化、TS SDK / scaffold |

当前实现同时明确拒绝不受支持的 `kind/runtime` 组合，例如 `ToolPlugin + go-plugin`、`IntegrationPlugin + mcp`。禁用态插件不会被激活；TS Bridge 和 Go Orchestrator 都只使用统一的生命周期语义：`installed`、`enabled`、`activating`、`active`、`degraded`、`disabled`。

当前已经有明确 operator / author surfaces 的实现包括：

- `GET /api/v1/plugins/catalog` 与 `POST /api/v1/plugins/catalog/install`，用于区分 catalog 条目与安装记录
- `pnpm create-plugin`，用于生成受维护的 `tool` / `review` / `workflow` / `integration` starter
- `pnpm plugin:verify`，用于受维护 Go WASM 样例的 `build -> debug health` smoke 验证
- `pnpm plugin:verify:builtins`，用于校验内置插件 bundle 与注册元数据
- 插件前端控制面中的 built-in readiness / runtime telemetry 增强显示，但业务真相仍以后端控制面为准

本次 OpenSpec 明确延后的事项包括：

- 公开可运营的远程插件市场 UI 与远程 artifact 拉取器
- 比当前 digest + signature / approval gate 更完整的供应链信任根与自动化签名链路
- `Extism WASM Runtime` 与其他跨语言运行时分发方案
- 公开发布级的 Plugin SDK 分发、脚手架包和开发者门户
- 多实例分布式插件编排、远程托管和 Serverless 运行时

因此，下面各章节里涉及的 Registry、市场、WASM、Redis Streams、全量 Workflow/Review 执行器等内容，应视为后续阶段设计储备，而不是当前仓库已经交付的 MVP 范围。

---

## 二、四层运行时架构

### 2.1 架构全景

AgentForge 采用 **Tauri + Next.js + Go + TS** 四层架构（参考 react-go-quick-starter 模板）：

```
┌─────────────────────────────────────────────────────────────┐
│  Layer 0: Tauri Shell (Rust)                                │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ • 桌面窗口管理 (Webview2 / WKWebKit / WebKitGTK)      │ │
│  │ • Sidecar 进程管理 (tauri-plugin-shell)               │ │
│  │ • 原生 OS 能力 (文件系统、系统通知、托盘图标)          │ │
│  │ • Tauri Commands IPC (Rust ↔ Frontend)                │ │
│  │ • Tauri Event System (跨层事件广播)                    │ │
│  └────────────────────────────────────────────────────────┘ │
│       │ invoke()              │ sidecar spawn                │
│       ▼                       ▼                              │
│  ┌──────────────────┐  ┌──────────────────────────────┐    │
│  │ Layer 1: Frontend │  │ Layer 2: Go Orchestrator     │    │
│  │ (Next.js/React)   │  │ (Echo, sidecar on :7777)     │    │
│  │                    │  │                              │    │
│  │ • 仪表盘 UI       │  │ • 任务管理                   │    │
│  │ • 角色配置面板     │  │ • Agent 生命周期             │    │
│  │ • 工作流可视化     │  │ • go-plugin 插件管理         │    │
│  │ • 插件市场 UI     │  │ • Event Bus (Redis PubSub)   │    │
│  │                    │  │ • 集成插件运行时             │    │
│  └────────┬─────────┘  └──────────┬───────────────────┘    │
│           │ HTTP/WS               │ gRPC                     │
│           │                       ▼                          │
│           │              ┌──────────────────────────────┐   │
│           └─────────────►│ Layer 3: TS Agent Bridge     │   │
│                          │ (Claude Agent SDK)           │   │
│                          │                              │   │
│                          │ • MCP Client Hub (工具插件)  │   │
│                          │ • Agent 执行引擎             │   │
│                          │ • Token/Cost 计量            │   │
│                          └──────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 部署模式

| 模式 | Layer 0 | Layer 1 | Layer 2 | Layer 3 | 适用场景 |
|------|---------|---------|---------|---------|---------|
| **桌面模式** | Tauri 窗口 | Webview 内 | Tauri sidecar | Go 子进程 | 个人/小团队 |
| **Web 模式** | 无 | Vercel/静态托管 | Docker 容器 | Docker 容器 | 团队/企业 |
| **混合模式** | Tauri 窗口 | Webview 内 | 远程服务器 | 远程服务器 | 桌面客户端 + 云后端 |

关键设计约束：
- **插件系统必须在有无 Tauri 时都能工作**
- Layer 0 (Tauri) 是可选的 — Web 模式下完全不存在
- 插件通信不应依赖 Tauri IPC 作为必需通道

### 2.3 打包与分发

#### TS Bridge 打包：Bun compile

TS Bridge 使用 `bun build --compile` 打包为单二进制文件。

**选型理由**：
- 依赖全是纯 JS/TS（`@anthropic-ai/sdk`、`@modelcontextprotocol/sdk`、`ws`），无 native addon，Bun 完全兼容
- 一条命令跨平台编译：`bun build --compile --target=bun-linux-x64 ./src/index.ts`
- Claude Code CLI 本身就是 Bun 打包的，Anthropic 已收购 Bun，SDK 兼容性有保障
- `child_process.spawn` 在编译后完整支持（MCP stdio Server 启动需要）
- 二进制体积 50-60MB (macOS/Linux)，~105MB (Windows)

**构建流程**：
```bash
# 为所有平台构建 TS Bridge sidecar
bun build --compile --target=bun-linux-x64   ./src/index.ts --outfile=bridge-x86_64-unknown-linux-gnu
bun build --compile --target=bun-linux-arm64  ./src/index.ts --outfile=bridge-aarch64-unknown-linux-gnu
bun build --compile --target=bun-darwin-x64   ./src/index.ts --outfile=bridge-x86_64-apple-darwin
bun build --compile --target=bun-darwin-arm64 ./src/index.ts --outfile=bridge-aarch64-apple-darwin
bun build --compile --target=bun-windows-x64  ./src/index.ts --outfile=bridge-x86_64-pc-windows-msvc.exe
```

#### 各层打包产物

| 层 | 打包方式 | 产物 | 大小 |
|---|---|---|---|
| Layer 0 (Tauri) | `tauri build` | .msi / .dmg / .AppImage | ~10MB (不含 sidecar) |
| Layer 1 (Next.js) | `next build` + `output: "export"` | 静态 HTML/CSS/JS in `out/` | ~5-20MB |
| Layer 2 (Go) | `go build` + `CGO_ENABLED=0` | 单二进制 | ~15-25MB |
| Layer 3 (TS Bridge) | `bun build --compile` | 单二进制 | ~50-105MB |

#### 桌面模式完整分发包

```
Tauri 安装包 (~150-200MB)
  ├── tauri-app (Rust 壳, ~10MB)
  ├── out/ (Next.js 静态资源, ~10MB)
  ├── binaries/
  │     ├── server-{target-triple}      # Go Orchestrator (~20MB)
  │     └── bridge-{target-triple}      # TS Bridge Bun binary (~60-105MB)
  └── icons/ + metadata
```

Tauri 启动流程：
1. Tauri 启动 → spawn Go sidecar (`binaries/server --port 7777`)
2. Go 启动 → spawn TS Bridge (`binaries/bridge --port 7778 --go-ws ws://localhost:7777/internal/ws`)
3. TS Bridge 启动 → 连接 Go 的 WS → 上报 ready
4. Go 收到 ready → 标记系统就绪 → WS 通知前端

#### Web/Docker 模式

```dockerfile
# Dockerfile.server (Go + TS Bridge)
FROM golang:1.23 AS go-builder
WORKDIR /app
COPY src-go/ .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM oven/bun:1.3 AS ts-builder
WORKDIR /app
COPY src-bridge/ .
RUN bun build --compile ./src/index.ts --outfile=/bridge

FROM debian:bookworm-slim
COPY --from=go-builder /server /usr/local/bin/
COPY --from=ts-builder /bridge /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/server"]
# Go 启动后自动 spawn bridge
```

#### MCP Server 工具插件的运行环境

**前提：用户本地提供 Node.js 和 uv (Python 包管理) 环境。**

这意味着 MCP Server 可以直接通过 `npx` / `uvx` 启动，无需预编译或内嵌：

| MCP Server 类型 | 启动方式 | 示例 |
|---|---|---|
| **npm 包 (TS/JS)** | `npx @anthropic/mcp-xxx` | GitHub Tool, Web Search |
| **Python 包** | `uvx mcp-server-xxx` | DB Query, 数据分析 |
| **HTTP 远程** | 直接 HTTP 连接 | Composio, 自建服务 |
| **独立二进制** | 直接执行 | Go/Rust 写的 MCP Server |

TS Bridge 通过 `child_process.spawn` 启动 MCP stdio Server 时，直接调用用户环境中的 `npx` / `uvx`：

```typescript
// TS Bridge 启动 MCP Server
const server = spawn("npx", ["-y", "@anthropic/mcp-github"], { stdio: "pipe" });
const server = spawn("uvx", ["mcp-server-sqlite", "--db", "data.db"], { stdio: "pipe" });
```

**桌面模式和 Web 模式行为一致** — 都依赖宿主环境的 Node/uv。Docker 镜像中预装 Node + uv 即可。

### 2.4 进程拓扑

```
桌面模式进程树:

tauri-app (Rust)
  ├── webview (渲染 Next.js 静态资源)
  ├── go-orchestrator (Bun-compiled sidecar, --port 7777)
  │     ├── integration-plugin-a (go-plugin 子进程)
  │     └── integration-plugin-b (go-plugin 子进程)
  └── ts-agent-bridge (Bun-compiled binary, --port 7778)
        ├── mcp-server-a (内嵌, 进程内)
        ├── mcp-server-b (内嵌, 进程内)
        └── mcp-server-c (HTTP 远程连接)

Web/Docker 模式进程树:

Container:
  go-orchestrator (:7777)
    ├── integration-plugin-a (go-plugin 子进程)
    └── integration-plugin-b (go-plugin 子进程)
  ts-agent-bridge (:7778)
    ├── mcp-server-a (内嵌 / bunx 启动)
    ├── mcp-server-b (内嵌 / bunx 启动)
    └── mcp-server-c (HTTP 远程连接)

Frontend (独立部署):
  next.js (Vercel / CDN / Nginx)
```

---

## 三、插件间通信架构

### 3.1 通信通道总览

插件间通信涉及 4 层之间的 **5 条通信通道、2 种协议（HTTP/WS + MCP）**：

```
                    Tauri (Rust)
                   ╱            ╲
          ① invoke()          ② sidecar mgmt
            IPC                  + stdout
           ╱                       ╲
    Frontend (TS)              Go Orchestrator (:7777)
          ╲                       ╱  ╲
        ③ HTTP/WS          ④ HTTP    ④ WS
            REST API         命令通道   事件/流通道
              ╲               ╱  ╲    ╱
              TS Agent Bridge (:7778)
                    │
              ⑤ MCP (stdio/HTTP)
                    │
               MCP Servers
```

| # | 通道 | 协议 | 方向 | 用途 |
|---|------|------|------|------|
| ① | Tauri ↔ Frontend | Tauri Commands (IPC) | 双向 | 原生能力 (文件选择、通知、窗口控制)，桌面模式专属 |
| ② | Tauri → Go | Sidecar 管理 + stdout | 单向 | Go 进程生命周期、日志收集，桌面模式专属 |
| ③ | Frontend ↔ Go | HTTP REST + WebSocket | 双向 | API 调用、实时状态推送，**唯一对外网络通道** |
| ④ | Go ↔ TS Bridge | HTTP + WebSocket | 双向 | Go→TS: HTTP 命令（执行任务、管理插件）；TS→Go: WS 事件流（进度、Token 消耗、状态上报） |
| ⑤ | TS Bridge ↔ MCP Servers | MCP (JSON-RPC) | 双向 | Agent 工具调用 |

**事件广播**：不使用 Redis PubSub。Go 内部 EventEmitter 接收所有事件（包括 TS Bridge 通过 WS 上报的事件），通过 WebSocket Hub 广播到前端。

**Go ↔ TS 双向通信详解**：

```
命令通道 (Go 主动调 TS):
  Go (:7777) ──HTTP POST──► TS (:7778)
  • POST /bridge/execute     — 执行 Agent 任务（附带完整角色配置）
  • POST /tools/install  — 安装/启动 MCP Server
  • POST /tools/uninstall — 卸载/停止 MCP Server
  • POST /bridge/cancel       — 终止正在执行的 Agent
  • GET  /bridge/health      — 健康检查
  • GET  /tools       — 列出所有已加载工具

兼容说明：裸路径 alias 仍可作为迁移兼容入口存在，但当前 live contract 以 `/bridge/*` 为准。

事件通道 (TS 主动推给 Go):
  TS ═══WebSocket═══► Go (:7777/internal/ws)
  • TS 启动后主动连接 Go 的 WS 端点
  • 推送: Agent 执行进度（token 流）
  • 推送: Token/Cost 消耗实时上报
  • 推送: MCP Server 状态变更（启动/崩溃/恢复）
  • 推送: 工具调用日志
  • 心跳: 每 10s 上报 TS Bridge 健康状态 + 所有 MCP Server 状态

转发链路:
  TS ══WS══► Go (内部 EventEmitter) ══WS══► Frontend
              └── 同时写入 Plugin 事件日志
```

**启动顺序**：Go 先启动 → TS 启动后主动连 Go 的 WS → 连接建立后 Go 才标记 TS Bridge 为 ready。

### 3.2 插件通信矩阵

**插件 A 要和插件 B 通信时，走哪条路？**

| 发起方 → 接收方 | 通信路径 | 协议 | 示例 |
|-----------------|---------|------|------|
| **Tool ↔ Tool** (MCP) | 不直接通信，通过 Agent 推理链路 | N/A | Agent 先用搜索工具，再用分析工具 |
| **Tool → Go** (结果回报) | TS Bridge ──WS──► Go | WS 事件 | 工具执行完通知 Orchestrator |
| **Integration → Integration** | Go 内部 EventEmitter 路由 | 内存 | 飞书消息 → 触发 GitHub 操作 |
| **Integration → Tool** | Go ──HTTP──► TS Bridge ──MCP──► Tool | HTTP + MCP | IM 收到指令 → 触发 Agent 工具 |
| **Workflow → Tool** | Go ──HTTP──► TS Bridge ──MCP──► Tool | HTTP + MCP | 工作流步骤调用 Agent |
| **Workflow → Integration** | Go 内部调用 (同进程 go-plugin) | go-plugin | 工作流完成 → 发送 IM 通知 |
| **Review → Tool** | MCP → TS Bridge (同层) | MCP | 审查插件调用分析工具 |
| **任意插件 → Frontend** | TS ──WS──► Go EventEmitter ──WS──► Frontend | WS 转发 | 插件状态变化推送到 UI |
| **Frontend → 任意插件** | HTTP API → Go → 路由到目标层 | HTTP | 用户在 UI 配置/操作插件 |
| **Tauri → 插件** | Tauri Event → Frontend → HTTP → Go | Tauri IPC + HTTP | 原生菜单触发插件操作 |

### 3.3 核心通信模式

#### 模式 1: Event Hub（Go 内存 EventEmitter + WS 广播）

**适用**: 插件间通知、状态变更、审计日志

```
┌────────────┐  go-plugin内部   ┌───────────────────────────────────────┐
│ 飞书 Plugin │ ─────────────► │  Go Orchestrator                       │
│ (Integration)│                │                                        │
└────────────┘                │  EventEmitter (内存)                    │
                               │    │                                    │
┌────────────┐  WS 上报        │    ├── 路由到 go-plugin (GitHub Plugin)│
│ TS Bridge  │ ═══════════════►│    ├── WS Hub ──► Frontend             │
│ (MCP事件)  │                 │    └── 写入 plugin_events 日志         │
└────────────┘                └───────────────────────────────────────┘
```

**所有事件汇聚到 Go**，Go 负责路由和广播。不引入 Redis PubSub，MVP 阶段内存 EventEmitter 足够。未来如需多 Go 实例再引入 Redis Streams。

**事件格式**:
```json
{
  "event_id": "evt_abc123",
  "type": "integration.im.message_received",
  "source": "feishu-adapter",
  "timestamp": "2026-03-23T10:00:00Z",
  "payload": {
    "channel_id": "oc_xxx",
    "sender": "user_123",
    "text": "@AgentForge 修复 issue #42"
  },
  "metadata": {
    "project_id": "proj_xxx",
    "trace_id": "trace_abc"
  }
}
```

**事件命名规范**:
```
{plugin_type}.{category}.{action}

示例:
  integration.im.message_received     # IM 收到消息
  integration.im.message_sent         # IM 发出消息
  integration.ci.build_completed      # CI 构建完成
  tool.execution.started              # 工具开始执行
  tool.execution.completed            # 工具执行完成
  workflow.step.completed             # 工作流步骤完成
  review.check.failed                 # 审查检查失败
  agent.task.status_changed           # Agent 任务状态变更
  system.plugin.installed             # 插件安装事件
  system.plugin.error                 # 插件错误事件
```

#### 模式 2: Request-Response（同步调用）

**适用**: 工具调用、API 请求、需要返回值的操作

```
Frontend                Go Orchestrator (:7777)    TS Bridge (:7778)      MCP Server
   │                         │                        │                      │
   │  POST /api/v1/agent/    │                        │                      │
   │  execute-tool           │                        │                      │
   │ ───────────────────►    │                        │                      │
   │                         │  POST /bridge/execute  │                      │
   │                         │ ───────────────────►   │                      │
   │                         │                        │  MCP tools/call      │
   │                         │                        │ ──────────────────►  │
   │                         │                        │                      │
   │                         │                        │  ◄──────────────────  │
   │                         │                        │  MCP result          │
   │                         │  ◄───────────────────  │                      │
   │                         │  HTTP 200 + result     │                      │
   │  ◄───────────────────   │                        │                      │
   │  HTTP 200 + result      │                        │                      │
```

#### 模式 3: Streaming（实时流推送）

**适用**: Agent 执行过程实时反馈、长时间任务进度

```
Frontend              Go Orchestrator (:7777)    TS Bridge (:7778)      Agent/LLM
   │                         │                        │                      │
   │  WebSocket /ws          │                        │                      │
   │ ═══════════════════►    │                        │                      │
   │                         │  POST /bridge/execute  │                      │
   │                         │ ───────────────────►   │  (HTTP 返回 taskId)  │
   │                         │                        │  Claude API Stream   │
   │                         │                        │ ═══════════════════► │
   │                         │                        │                      │
   │                         │                        │  ◄═══ token chunk    │
   │                         │  ◄═══ WS event         │  (TS 通过 WS 推给 Go)│
   │  ◄═══ WS message       │  (Go 转发到 Frontend)  │                      │
   │  { type: "token",      │                        │                      │
   │    content: "..." }    │                        │                      │
   │                         │                        │                      │
   │                         │                        │  ◄═══ tool_use       │
   │                         │  ◄═══ WS event         │                      │
   │  ◄═══ WS message       │                        │                      │
   │  { type: "tool_call",  │                        │                      │
   │    tool: "web_search"} │                        │                      │
```

**流式调用模式**：Go 通过 HTTP POST 发起任务（立即返回 taskId），TS Bridge 执行过程中通过已建立的 WS 连接实时推送进度事件。Go 转发到前端的 WS。

#### 模式 4: Tauri Native Bridge（桌面专属）

**适用**: 需要原生 OS 能力的插件操作（仅桌面模式可用）

```
Frontend                   Tauri (Rust)              OS
   │                           │                      │
   │  invoke("select_files",   │                      │
   │    { filters: [...] })    │                      │
   │ ─────────────────────►    │                      │
   │                           │  native file dialog  │
   │                           │ ──────────────────►  │
   │                           │  ◄──────────────────  │
   │  ◄─────────────────────   │  selected paths      │
   │  ["/path/to/file.ts"]    │                      │
   │                           │                      │
   │  // 然后通过 HTTP 传给 Go │                      │
   │  POST /api/v1/agent/      │                      │
   │    add-context            │                      │
   │    { files: [...] }       │                      │
```

**关键原则**: Tauri 提供原生能力，但**不参与插件业务逻辑**。插件通信的核心路径始终是 Go ↔ TS，Tauri 只是前端的"增强层"。

### 3.4 通信架构设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 插件间通信中枢 | **Go Orchestrator** | 所有插件最终通过 Go 路由，避免多中心 |
| Go ↔ TS 通信协议 | **HTTP (命令) + WS (事件流)** | 调试方便，不需要 protobuf 代码生成，WS 天然支持流式进度推送 |
| Tool 插件间通信 | **不直接通信** | 通过 Agent 推理链路串联，保持工具独立性 |
| 跨类型插件事件 | **Go 内存 EventEmitter** | MVP 不引入 Redis PubSub，内存够用，未来多实例再加 Redis Streams |
| 实时推送 | **WS 双级转发** (TS→Go→Frontend) | Go 是 WS Hub，聚合 TS Bridge 事件 + 内部事件，统一推给前端 |
| Tauri 层角色 | **仅提供原生能力增强** | 不作为通信中枢，保持 Web/桌面模式一致 |
| TS Bridge 状态 | **无状态执行器** | 每次 HTTP 调用传完整角色配置，TS 不缓存，Go 是 single source of truth |
| Token 计费 | **双重控制** | TS 本地硬限制（Go 下发 maxBudget），同时 WS 实时上报消耗给 Go 全局预算 |

### 3.5 Tauri 层的插件扩展点

虽然 Tauri 不参与插件业务通信，但可以为桌面模式提供**增强能力**：

```yaml
# Tauri 增强能力（桌面模式专属，Web 模式有 fallback）
tauri_capabilities:
  # 1. 原生文件选择器 (Web fallback: <input type="file">)
  - name: "native_file_picker"
    tauri_command: "select_files"
    web_fallback: "html_file_input"

  # 2. 系统通知 (Web fallback: Web Notification API)
  - name: "system_notification"
    tauri_command: "send_notification"
    web_fallback: "web_notification"

  # 3. 系统托盘 (Web fallback: 浏览器 tab 标题闪烁)
  - name: "tray_icon"
    tauri_command: "update_tray"
    web_fallback: "title_flash"

  # 4. 全局快捷键 (Web fallback: 无)
  - name: "global_shortcut"
    tauri_command: "register_shortcut"
    web_fallback: null

  # 5. 自动更新 (Web fallback: 无需, 浏览器自动最新)
  - name: "auto_update"
    tauri_command: "check_update"
    web_fallback: null
```

**实现模式**: 前端通过 `usePlatformCapability()` Hook 自动选择 Tauri 或 Web 实现：

```typescript
// hooks/use-platform-capability.ts
export function usePlatformCapability() {
  const isTauriEnv = isTauri();

  return {
    async selectFiles(filters: FileFilter[]) {
      if (isTauriEnv) {
        return invoke<string[]>("select_files", { filters });
      }
      // Web fallback
      return showHtmlFilePicker(filters);
    },

    async sendNotification(title: string, body: string) {
      if (isTauriEnv) {
        return invoke("send_notification", { title, body });
      }
      return new Notification(title, { body });
    },
  };
}
```

### 3.6 完整通信流程示例

**场景**: 用户在飞书发消息 "@AgentForge 修复 issue #42" → Agent 自动修复并通知

```
飞书 App                                                     AgentForge
  │                                                              │
  │  webhook POST /api/v1/webhook/feishu                         │
  │ ─────────────────────────────────────────────────────────►   │
  │                                                              │
  │         ┌─────────────────────────────────────────────────┐  │
  │         │  Go Orchestrator                                │  │
  │         │                                                 │  │
  │         │  1. 飞书 Integration Plugin 解析消息             │  │
  │         │     → InboundEvent { text: "修复 issue #42" }   │  │
  │         │                                                 │  │
  │         │  2. Event Bus 发布:                              │  │
  │         │     integration.im.message_received             │  │
  │         │                                                 │  │
  │         │  3. Workflow Engine 匹配触发规则                 │  │
  │         │     → 启动 "bug-fix-flow" 工作流                │  │
  │         │                                                 │  │
  │         │  4. 创建 Task + 分配给 coding-agent 角色        │  │
  │         │                                                 │  │
  │         │  5. gRPC → TS Bridge: Execute(task, role)       │  │
  │         └──────────────────────────┬──────────────────────┘  │
  │                                    │                         │
  │         ┌──────────────────────────▼──────────────────────┐  │
  │         │  TS Agent Bridge                                │  │
  │         │                                                 │  │
  │         │  6. 加载 coding-agent 角色配置                   │  │
  │         │     → system prompt + tools + knowledge         │  │
  │         │                                                 │  │
  │         │  7. MCP 调用 GitHub Tool → 读取 issue #42       │  │
  │         │  8. MCP 调用 Code Editor → 分析代码             │  │
  │         │  9. MCP 调用 Code Editor → 修复 bug             │  │
  │         │  10. MCP 调用 GitHub Tool → 创建 PR             │  │
  │         │                                                 │  │
  │         │  11. gRPC stream → Go: 实时进度事件             │  │
  │         └──────────────────────────┬──────────────────────┘  │
  │                                    │                         │
  │         ┌──────────────────────────▼──────────────────────┐  │
  │         │  Go Orchestrator                                │  │
  │         │                                                 │  │
  │         │  12. Event Bus 发布:                             │  │
  │         │      agent.task.completed                       │  │
  │         │      → WebSocket Hub → Frontend UI 更新         │  │
  │         │                                                 │  │
  │         │  13. 飞书 Integration Plugin:                    │  │
  │         │      发送消息 "PR #87 已创建，请审查"            │  │
  │         └─────────────────────────────────────────────────┘  │
  │                                                              │
  │  ◄─── 飞书消息: "PR #87 已创建，请审查"                      │
  │                                                              │

同时 (桌面模式):
  │
  Frontend (Tauri Webview)
  │  ◄═══ WebSocket: { type: "task_completed", pr: "#87" }
  │  → 更新看板 UI
  │  → Tauri 系统通知: "Agent 已修复 issue #42"
```

### 3.7 通信可靠性保障

| 场景 | 检测方 | 策略 |
|------|--------|------|
| MCP Server 崩溃 | TS (进程退出) | TS WS 上报 → Go 指令重启 (max 3 次) → 超限 disable |
| go-plugin 崩溃 | Go (health check) | go-plugin 内置进程监控，自动重启 |
| TS Bridge 崩溃 | Go (WS 连接断开) | Go 重启 TS 进程，等待 WS 重连 |
| Go ↔ TS WS 断开 | 双方检测 | TS 指数退避重连，Go 标记 MCP 插件为 unknown |
| Frontend WS 断开 | Frontend | 前端自动重连，Go WS Hub 补发最近 50 条事件 |
| Tauri sidecar 崩溃 | Tauri (退出码) | Tauri 检测退出码，自动重启 Go 进程 |
| Token 预算超限 | TS (本地) + Go (全局) | TS 本地硬限制立即终止；Go 全局限制通过 HTTP abort |

---

## 四、插件类型分层

```
┌─────────────────────────────────────────────────────┐
│                AgentForge Plugin System               │
├─────────────────────────────────────────────────────┤
│                                                       │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌──────────┐ │
│  │  Role   │ │  Tool   │ │ Workflow │ │Integration│ │
│  │ Plugin  │ │ Plugin  │ │  Plugin  │ │  Plugin   │ │
│  │         │ │         │ │          │ │           │ │
│  │ 角色模板 │ │ Agent   │ │ 多Agent  │ │ 外部系统  │ │
│  │ 能力组合 │ │ 工具扩展 │ │ 协作编排 │ │ 对接桥梁  │ │
│  │ 知识绑定 │ │ MCP兼容 │ │ 状态图   │ │ IM/CI/CD │ │
│  └─────────┘ └─────────┘ └──────────┘ └──────────┘ │
│                                                       │
│  ┌──────────┐                                        │
│  │ Review   │                                        │
│  │ Plugin   │                                        │
│  │          │                                        │
│  │ 自定义    │                                        │
│  │ 审查规则  │                                        │
│  └──────────┘                                        │
│                                                       │
├─────────────────────────────────────────────────────┤
│  Plugin Runtime Layer                                 │
│  MCP Server | go-plugin (gRPC) | Event Hook | WASM   │
├─────────────────────────────────────────────────────┤
│  Security Layer                                       │
│  Manifest 权限声明 | 进程隔离 | 资源限制 | 签名验证    │
└─────────────────────────────────────────────────────┘
```

### 插件类型对比

| 类型 | 运行侧 | 运行时 | 典型用途 | 开发难度 |
|------|--------|--------|---------|---------|
| **Role** | 声明式 (无代码) | N/A (YAML 配置) | 创建数字员工角色 | ★☆☆☆☆ |
| **Tool** | TS Bridge | MCP Server | 给 Agent 增加新工具 | ★★☆☆☆ |
| **Workflow** | Go Orchestrator | go-plugin / YAML | 定义多 Agent 协作流程 | ★★★☆☆ |
| **Integration** | Go Orchestrator | go-plugin (gRPC) | 对接外部系统 | ★★★☆☆ |
| **Review** | Go + TS | MCP + go-plugin | 自定义代码审查规则 | ★★★★☆ |

---

## 五、核心架构

### 5.1 MVP 架构（0-6 个月）

```
┌──────────────────────────────────────────────────────────────────┐
│  Tauri Shell (可选 — Web 模式下不存在)                             │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ sidecar 管理 | 系统通知 | 文件对话框 | 托盘 | 自动更新      │  │
│  └──────────────┬───────────────────────┬─────────────────────┘  │
│                 │ invoke()              │ sidecar spawn           │
│  ┌──────────────▼──────────┐  ┌────────▼───────────────────┐    │
│  │   Frontend (Next.js)    │  │   Go Orchestrator (:7777)  │    │
│  │                         │  │                            │    │
│  │ • 仪表盘/看板           │  │ ┌────────────────────────┐ │    │
│  │ • 角色配置              │  │ │ Plugin Manager         │ │    │
│  │ • 插件管理              │  │ │ (go-plugin + Role Reg) │ │    │
│  │ • WebSocket 实时        │  │ └───────────┬────────────┘ │    │
│  │                         │  │ ┌───────────▼────────────┐ │    │
│  │                         │  │ │ EventEmitter + WS Hub  │ │    │
│  │                         │  │ └────────────────────────┘ │    │
│  └────────────┬────────────┘  └──────┬─────────┬──────────┘    │
│               │ HTTP/WS              │ HTTP    │ WS             │
│               │                      │ (命令)  │ (事件流)        │
│               │               ┌──────▼─────────▼────────────┐  │
│               └──────────────►│  TS Agent Bridge (:7778)    │  │
│                               │  ┌────────────────────────┐ │  │
│                               │  │ MCP Client Hub         │ │  │
│                               │  │ (Claude SDK 内置)      │ │  │
│                               │  └───────────┬────────────┘ │  │
│                               └──────────────┼──────────────┘  │
│                                              │                  │
└──────────────────────────────────────────────┼──────────────────┘
                                               │
                    ┌──────────────────────────┼──────────────┐
                    │                          │              │
              ┌─────▼──────┐  ┌───────▼──────┐  ┌───────▼──────┐
              │ Go Plugins │  │ MCP Servers  │  │ MCP Servers  │
              │ (go-plugin)│  │ (stdio)      │  │ (HTTP)       │
              │            │  │              │  │              │
              │ IM 适配器   │  │ GitHub Tool  │  │ Remote Tool  │
              │ CI/CD Hook │  │ Web Search   │  │              │
              │ 审批流程    │  │ DB Query     │  │              │
              └────────────┘  └──────────────┘  └──────────────┘
```

### 5.2 长期架构（6-18 个月）

在 MVP 基础上新增：

- **Extism WASM Runtime** — Go 和 TS 双侧加载同一 .wasm 插件
- **Plugin Registry** — OCI 兼容的自建插件市场
- **Visual Workflow Editor** — 可视化工作流编排器
- **Plugin SDK** — 脚手架、类型定义、测试框架
- **Tauri Plugin API** — 桌面增强能力暴露给第三方插件

---

## 六、角色插件系统（Role Plugin）

### 4.1 设计理念

角色插件是**最低门槛的插件类型** — 只需一个 YAML 文件即可定义一个数字员工。

采用 **Template-Profile + Capability-Composition + Skill-Tree** 三层混合架构：

```
┌─────────────────────────────────────────────┐
│ Layer 1: Template-Profile (身份层)            │
│   角色名称、个性、语言、风格                    │
├─────────────────────────────────────────────┤
│ Layer 2: Capability-Composition (能力层)      │
│   预设能力包 + 自选工具 + MCP Server           │
├─────────────────────────────────────────────┤
│ Layer 3: Skill-Tree (知识层)                  │
│   领域知识、技能文件、RAG 知识库               │
├─────────────────────────────────────────────┤
│ Layer 4: Security & Governance (治理层)       │
│   权限边界、资源限制、输出过滤、审计           │
└─────────────────────────────────────────────┘
```

### 4.2 角色定义 Schema

```yaml
# roles/frontend-developer/role.yaml
apiVersion: agentforge/v1
kind: Role
metadata:
  id: "frontend-developer"
  name: "前端开发工程师"
  version: "1.0.0"
  author: "agentforge-official"
  tags: ["development", "frontend", "web"]
  icon: "code-bracket"
  description: "专业的前端开发工程师，擅长 React/Vue 生态系统"

# ---- Layer 1: 身份 ----
identity:
  role: "Senior Frontend Developer"
  goal: "高质量完成前端开发任务，遵循团队规范"
  backstory: |
    你是一位资深前端工程师，拥有 8 年 Web 开发经验。
    擅长 React 生态、性能优化和用户体验设计。
  personality: "detail-oriented, collaborative"
  language: "zh-CN"
  response_style:
    tone: "professional"
    verbosity: "concise"

# ---- Layer 2: 能力 ----
capabilities:
  # 预设能力包（平台内置）
  packages:
    - "web-development"      # 代码编辑、终端、Git
    - "code-review"          # 代码审查工具链
    - "testing"              # 测试框架集成

  # 工具列表
  tools:
    built_in:
      - code_editor
      - terminal
      - browser_preview
      - git_client
    mcp_servers:             # MCP 工具扩展
      - name: "figma-viewer"
        url: "npx @anthropic/mcp-figma"
      - name: "npm-search"
        url: "npx @anthropic/mcp-npm"

  # 技能树（按需加载的专业知识）
  skills:
    - path: "skills/react"
      auto_load: true
    - path: "skills/typescript"
      auto_load: true
    - path: "skills/css-animation"
      auto_load: false       # 仅在需要时加载

# ---- Layer 3: 知识 ----
knowledge:
  # 共享知识库引用
  shared:
    - ref: "company-standards"
    - ref: "product-docs"

  # 角色私有知识
  private:
    - type: "vector"
      sources:
        - "knowledge/react-patterns.md"
        - "knowledge/performance-guide.md"

  # 记忆配置
  memory:
    episodic:
      enabled: true
      retention_days: 90     # 情景记忆保留 90 天
    semantic:
      enabled: true
      auto_extract: true     # 自动从交互中提取知识
    procedural:
      enabled: true          # 从反馈中学习技能

# ---- Layer 4: 协作 ----
collaboration:
  can_delegate_to: ["backend-developer", "designer"]
  accepts_delegation_from: ["project-manager", "tech-lead"]
  communication:
    preferred_channel: "structured"
    escalation_policy: "auto"

# ---- Layer 5: 安全 ----
security:
  profile: "development"     # 安全模板

  permissions:
    file_access:
      allowed_paths: ["src/", "public/", "tests/"]
      denied_paths: [".env", "secrets/", "*.key"]
    network:
      allowed_domains: ["github.com", "npmjs.com"]
    code_execution:
      sandbox: true
      languages: ["javascript", "typescript", "shell"]

  resource_limits:
    token_budget:
      per_task: 100000
      per_day: 1000000
    cost_limit:
      per_task: "$5"
      per_day: "$30"

  output_filters:
    - no_credentials
    - no_pii
    - code_lint_check

# ---- 触发器 ----
triggers:
  - event: "pr_created"
    action: "auto_review"
    condition: "pr.files.any(f => f.path.startsWith('src/frontend/'))"
  - event: "issue_assigned"
    action: "analyze_and_plan"
    condition: "issue.labels.includes('frontend')"

# ---- 继承（可选）----
extends: "base-developer"
overrides:
  capabilities.packages:
    add: ["design-integration"]
```

> 当前仓库实现说明：角色插件的 authoring 面已经不只是“保存 YAML”。Go 侧角色控制面提供 preview 和 sandbox 两类非持久化能力，用于查看 effective manifest、execution profile、readiness diagnostics 与 bounded prompt probe。Marketplace、团队共享和版本历史仍然是后续能力，不应与当前 authoring completeness 混淆。

### 4.3 角色继承机制

```
base-employee (基础数字员工)
  ├── base-developer (基础开发者)
  │     ├── frontend-developer (前端)
  │     │     ├── react-specialist (React 专家)
  │     │     └── vue-specialist (Vue 专家)
  │     ├── backend-developer (后端)
  │     │     ├── go-developer
  │     │     └── python-developer
  │     └── fullstack-developer (全栈)
  ├── base-reviewer (基础审查员)
  │     ├── security-reviewer (安全审查)
  │     └── performance-reviewer (性能审查)
  ├── project-manager (项目经理)
  └── devops-engineer (运维工程师)
```

继承规则：
- `metadata` — 子角色完全覆盖
- `identity` — 子角色覆盖
- `capabilities.packages` — 合并（可通过 `overrides` 增删）
- `capabilities.tools` — 合并
- `knowledge.shared` — 合并
- `security` — 取更严格的约束（子角色不能放松安全限制）

### 4.4 预设角色模板

平台内置以下开箱即用角色：

| 角色 ID | 名称 | 核心能力 |
|---------|------|---------|
| `coding-agent` | 编码助手 | 代码生成、Bug 修复、重构 |
| `code-reviewer` | 代码审查员 | 代码审查、最佳实践检查 |
| `test-engineer` | 测试工程师 | 单元测试、集成测试编写 |
| `doc-writer` | 文档工程师 | API 文档、README 生成 |
| `devops-agent` | 运维助手 | CI/CD 配置、部署脚本 |
| `security-auditor` | 安全审计员 | 安全扫描、漏洞分析 |
| `project-assistant` | 项目助手 | 需求分析、任务分解 |

---

## 七、工具插件系统（Tool Plugin）

### 5.1 设计理念

工具插件让 Agent 获得新能力。**以 MCP 为标准协议**，天然兼容 AI 生态。

### 5.2 架构

```
TS Agent SDK Bridge
  │
  ├── MCP Client Hub
  │     ├── MCP Server A (stdio)    — 本地进程
  │     ├── MCP Server B (HTTP)     — 远程服务
  │     └── MCP Server C (stdio)    — npm 包
  │
  └── Tool Registry
        ├── 工具发现 (tools/list)
        ├── 工具调用 (tools/call)
        ├── 资源访问 (resources/read)
        └── 工具搜索 (动态按需加载)
```

### 5.3 工具插件 Manifest

```yaml
# plugins/tools/web-search/manifest.yaml
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: "web-search"
  name: "网页搜索"
  version: "1.0.0"
  author: "agentforge-community"
  description: "搜索互联网获取最新信息"
  tags: ["search", "web"]

spec:
  # MCP Server 配置
  runtime: "mcp"
  transport: "stdio"           # stdio | http
  command: "npx"
  args: ["-y", "@anthropic/mcp-web-search"]
  env:
    SEARCH_API_KEY: "${SEARCH_API_KEY}"  # 引用环境变量

  # 工具声明（自动从 MCP Server 发现，此处可覆盖）
  tools:
    - name: "web_search"
      description: "搜索互联网"
      inputSchema:
        type: object
        properties:
          query:
            type: string
            description: "搜索关键词"
          limit:
            type: integer
            default: 10
        required: ["query"]

  # 权限声明
  permissions:
    network:
      required: true
      domains: ["*.google.com", "*.bing.com"]
    file_system:
      required: false

  # 资源限制
  resource_limits:
    memory_mb: 256
    timeout_seconds: 30
    max_concurrent: 5
```

> 当前仓库实现说明：官方内置工具插件清单现在由 `plugins/builtin-bundle.yaml` 管理，当前随仓库交付的 ToolPlugin 包括 `web-search`、`github-tool` 和 `db-query`。它们都会通过 built-in discovery 与 catalog 暴露真实来源和可用性说明，而不是只保留文档示例。

### 5.4 MCP Server 集成流程

```
1. 用户安装工具插件
   agentforge plugin install @agentforge/tool-web-search

2. 平台解析 manifest.yaml

3. 启动 MCP Server (懒加载 — Agent 首次调用时启动)
   spawn("npx", ["-y", "@anthropic/mcp-web-search"], { stdio: "pipe" })

4. MCP 初始化握手
   Client → Server: { method: "initialize", params: { protocolVersion: "2025-03-26" } }
   Server → Client: { result: { capabilities: { tools: {} } } }

5. 工具发现
   Client → Server: { method: "tools/list" }
   Server → Client: { result: { tools: [...] } }

6. Agent 调用工具
   LLM 决定调用 web_search →
   Client → Server: { method: "tools/call", params: { name: "web_search", arguments: {...} } }
   Server → Client: { result: { content: [...] } }

7. 结果返回给 Agent 继续推理
```

---

## 八、工作流插件系统（Workflow Plugin）

### 6.1 设计理念

工作流插件定义**多数字员工如何协作**。支持三种编排模式：

| 模式 | 说明 | 适用场景 |
|------|------|---------|
| **Sequential** | A → B → C 顺序执行 | 标准开发流程 |
| **Hierarchical** | Manager 分派任务给 Workers | 大型项目分解 |
| **Event-Driven** | 基于事件触发角色响应 | CI/CD、监控告警 |

### 6.2 工作流定义

```yaml
# plugins/workflows/standard-dev-flow/manifest.yaml
apiVersion: agentforge/v1
kind: WorkflowPlugin
metadata:
  id: "standard-dev-flow"
  name: "标准开发流程"
  version: "1.0.0"
  description: "编码 → 审查的内置顺序工作流 starter"

spec:
  runtime: "wasm"
  module: "./dist/standard-dev-flow.wasm"
  abiVersion: "v1"
  workflow:
    process: "sequential"
    roles:
      - id: "coding-agent"
      - id: "code-reviewer"
    steps:
      - id: "implement"
        role: "coding-agent"
        action: "agent"
        next: ["review"]
      - id: "review"
        role: "code-reviewer"
        action: "review"
    triggers:
      - event: "manual"
```

> 当前仓库实现说明：repo 内置 starter 位于 `plugins/workflows/standard-dev-flow/manifest.yaml`，并使用现有 `coding-agent` / `code-reviewer` role id，因此它可以直接走当前 Go workflow runtime 的顺序执行路径。

### 6.3 层级编排模式

```yaml
spec:
  process: "hierarchical"

  # Manager 角色
  manager:
    ref: "project-assistant"
    delegation_strategy: "skill_match"   # 按技能匹配分派
    max_delegation_depth: 2

  # Worker 池
  workers:
    - ref: "frontend-developer"
      max_concurrent_tasks: 3
    - ref: "backend-developer"
      max_concurrent_tasks: 3
    - ref: "test-engineer"
      max_concurrent_tasks: 2

  # Manager 自动决策
  # 接收到任务后，Manager 分析需求并分派给合适的 Worker
  # Worker 完成后汇报给 Manager，Manager 汇总并交付
```

---

## 九、集成插件系统（Integration Plugin）

### 7.1 架构

集成插件运行在 Go Orchestrator 侧，通过 go-plugin (gRPC) 通信。

```
Go Orchestrator
  │
  ├── Integration Plugin Manager (go-plugin)
  │     ├── IM Adapter Plugin (飞书)       — gRPC 子进程
  │     ├── IM Adapter Plugin (钉钉)       — gRPC 子进程
  │     ├── CI/CD Plugin (GitHub Actions)  — gRPC 子进程
  │     └── Notification Plugin (邮件)     — gRPC 子进程
  │
  └── Event Bus (Redis Pub/Sub)
        ├── integration.im.message_received
        ├── integration.ci.build_completed
        └── integration.notify.send
```

### 7.2 集成插件接口（Protobuf）

```protobuf
// proto/integration/v1/integration.proto
syntax = "proto3";
package agentforge.integration.v1;

service IntegrationPlugin {
  // 插件自描述
  rpc Describe(DescribeRequest) returns (DescribeResponse);

  // 初始化（传入配置）
  rpc Initialize(InitializeRequest) returns (InitializeResponse);

  // 处理入站事件（如 IM 消息）
  rpc HandleInbound(InboundEvent) returns (InboundResult);

  // 发送出站消息（如通知）
  rpc SendOutbound(OutboundMessage) returns (OutboundResult);

  // 健康检查
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}

message DescribeResponse {
  string id = 1;
  string name = 2;
  string version = 3;
  string type = 4;          // "im" | "ci" | "notification" | "storage"
  repeated string events = 5; // 支持的事件类型
}

message InboundEvent {
  string event_type = 1;
  string source = 2;
  bytes payload = 3;
  map<string, string> metadata = 4;
}

message OutboundMessage {
  string target = 1;        // 目标（如飞书群 ID）
  string message_type = 2;  // "text" | "card" | "markdown"
  bytes content = 3;
  map<string, string> metadata = 4;
}
```

### 7.3 IM 适配器示例

```yaml
# plugins/integrations/feishu-adapter/manifest.yaml
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: "feishu-adapter"
  name: "飞书集成"
  version: "1.0.0"

spec:
  runtime: "go-plugin"
  binary: "./feishu-adapter"     # 编译后的 Go 二进制
  type: "im"

  config:
    app_id: "${FEISHU_APP_ID}"
    app_secret: "${FEISHU_APP_SECRET}"

  events:
    inbound:
      - "im.message.receive_v1"
      - "im.message.reaction.create_v1"
    outbound:
      - "send_text"
      - "send_card"
      - "send_markdown"

  permissions:
    network:
      required: true
      domains: ["*.feishu.cn", "*.larksuite.com"]
```

---

## 十、审查插件系统（Review Plugin）

### 8.1 三层审查流水线 + 插件扩展点

```
PR 提交
  │
  ▼
Layer 1: 快速审查 (内置)
  │  ├── ESLint / 代码格式
  │  ├── 安全扫描 (基础)
  │  └── 类型检查
  │
  ▼
Layer 2: 深度审查 (插件扩展)        ◄── Review Plugin 扩展点
  │  ├── [Plugin] 架构规范检查
  │  ├── [Plugin] 性能分析
  │  ├── [Plugin] 安全深度扫描
  │  └── [Plugin] 团队自定义规则
  │
  ▼
Layer 3: 人工审查 (Agent 辅助)
     ├── Review Agent 生成审查摘要
     └── 人工 1-click 批准/拒绝
```

### 8.2 审查插件定义

```yaml
# plugins/reviews/architecture-check/manifest.yaml
apiVersion: agentforge/v1
kind: ReviewPlugin
metadata:
  id: "architecture-check"
  name: "架构规范检查"
  version: "1.0.0"

spec:
  runtime: "mcp"                   # 通过 MCP 提供审查工具
  transport: "stdio"
  command: "bun"
  args: ["run", "src/index.ts"]

  # 触发条件
  review:
    entrypoint: "review:run"
    triggers:
      events:
        - "pull_request.updated"
      filePatterns:
      - "src/**/*.ts"
      - "src/**/*.go"
    output:
      format: "findings/v1"
```

> 当前仓库实现说明：官方内置审查插件清单同样由 `plugins/builtin-bundle.yaml` 管理，当前随仓库交付的 built-in ReviewPlugin 包括 `architecture-check` 和 `performance-check`，它们通过与自定义 ReviewPlugin 相同的 provenance 字段参与 Layer 2 聚合。

---

## 十一、插件协议与接口规范

### 9.1 协议矩阵

| 插件类型 | 协议 | 传输 | 运行侧 |
|---------|------|------|--------|
| Role | N/A (YAML 解析) | N/A | Go Orchestrator |
| Tool | MCP (JSON-RPC 2.0) | stdio / Streamable HTTP | TS Bridge |
| Workflow | gRPC (go-plugin) 或 YAML | 子进程 | Go Orchestrator |
| Integration | gRPC (go-plugin) | 子进程 | Go Orchestrator |
| Review | MCP + gRPC | stdio / 子进程 | 双侧 |

### 9.2 统一 Manifest 格式

所有插件共享统一的 Manifest 顶层结构：

```yaml
apiVersion: agentforge/v1           # 必填：API 版本
kind: RolePlugin | ToolPlugin | ... # 必填：插件类型
metadata:                            # 必填：元数据
  id: string                         # 唯一标识 (kebab-case)
  name: string                       # 显示名称
  version: string                    # 语义化版本
  author: string                     # 作者
  description: string                # 描述
  tags: string[]                     # 标签
  license: string                    # 开源协议
  homepage: string                   # 主页 URL
  repository: string                 # 仓库 URL
spec: object                         # 各类型特定配置
```

### 9.3 MCP Tool Schema 兼容

工具插件的 tool 定义严格遵循 MCP 规范：

```json
{
  "name": "tool_name",
  "description": "Tool description for LLM",
  "inputSchema": {
    "type": "object",
    "properties": { ... },
    "required": [ ... ]
  }
}
```

这确保 AgentForge 的工具插件可以直接在 Claude Desktop、Cursor 等 MCP 客户端中使用。

---

## 十二、安全与沙箱架构

### 10.1 安全分层

```
┌──────────────────────────────────────────┐
│  Layer 1: Manifest 权限声明               │
│  插件必须显式声明所需权限                   │
│  未声明的权限 → 直接拒绝                   │
├──────────────────────────────────────────┤
│  Layer 2: 签名验证                        │
│  官方/认证插件 → 密码学签名验证             │
│  未签名插件 → 显示安全警告                  │
├──────────────────────────────────────────┤
│  Layer 3: 进程隔离                        │
│  go-plugin → 独立子进程                    │
│  MCP Server → 独立子进程                   │
│  WASM → 沙箱内存隔离 (长期)               │
├──────────────────────────────────────────┤
│  Layer 4: 资源限制                        │
│  内存上限 | CPU 时间 | 网络带宽 | 存储配额  │
├──────────────────────────────────────────┤
│  Layer 5: 审计日志                        │
│  所有插件操作可追溯                        │
└──────────────────────────────────────────┘
```

### 10.2 权限模型

```yaml
# 权限声明规范
permissions:
  network:
    required: boolean
    domains: string[]              # 允许访问的域名白名单
  file_system:
    required: boolean
    paths: string[]                # 允许访问的路径
    mode: "read" | "write" | "readwrite"
  code_execution:
    required: boolean
    languages: string[]
    sandbox: boolean
  database:
    required: boolean
    operations: ["read"] | ["read", "write"]
  secrets:
    required: boolean
    keys: string[]                 # 需要的密钥名称
```

### 10.3 安全策略模板

| 策略 | network | file_system | code_execution | database | 适用场景 |
|------|---------|-------------|----------------|----------|---------|
| `strict` | 无 | 只读项目目录 | 禁止 | 只读 | 审计、安全扫描 |
| `standard` | 白名单 | 项目目录 | 沙箱 | 读写 | 日常开发 |
| `permissive` | 全开 | 全开 | 沙箱 | 读写 | 开发调试 |

### 10.4 WASM 沙箱（长期）

长期引入 Extism/wazero 后的安全模型：

```
┌─────────────────────────────────┐
│  WASM Plugin (.wasm)            │
│  ├── 线性内存 (隔离)            │
│  ├── 无直接 syscall             │
│  ├── Host Function 白名单       │
│  │     ├── http_request (受控)  │
│  │     ├── log (安全)           │
│  │     └── kv_store (受控)      │
│  └── 资源限制                   │
│        ├── 内存: 256MB max      │
│        ├── CPU: 指令计数限制     │
│        └── 执行时间: 30s max    │
└─────────────────────────────────┘
```

---

## 十三、插件生命周期管理

### 11.1 生命周期状态机

```
                  install
  [Not Installed] ──────► [Installed]
                              │
                         enable│
                              ▼
                          [Enabled]
                           │    ▲
                    activate│    │deactivate (idle timeout)
                           ▼    │
                         [Active]
                              │
                         disable│
                              ▼
                         [Disabled]
                              │
                        uninstall│
                              ▼
                       [Not Installed]
```

### 11.2 各阶段行为

| 阶段 | 触发 | 行为 |
|------|------|------|
| **Install** | CLI 或 Web UI | 下载插件包、验证签名、解析 manifest、注册到数据库 |
| **Enable** | 用户手动或默认 | 加载配置、验证权限、标记可用 |
| **Activate** | Agent 首次调用 (懒加载) | 启动子进程/MCP Server、初始化握手 |
| **Active** | 正常运行 | 处理请求、资源监控、健康检查 |
| **Deactivate** | 空闲超时 (默认 5 分钟) | 优雅关闭子进程、释放资源 |
| **Disable** | 用户手动 | 停止所有实例、保留配置 |
| **Uninstall** | 用户手动 | 删除文件、清理数据库记录 |
| **Update** | 新版本可用 | 下载新版 → Disable → 替换 → Enable |

### 13.3 健康检查与跨进程状态同步

Go Orchestrator 和 TS Bridge 分别管理不同类型的插件。**状态一致性**是关键挑战。

**原则：TS Bridge 上报，Go 裁决。**

```
Go Orchestrator (状态权威方)
  │
  ├── go-plugin 插件: Go 直接管理
  │     └── 每 30s gRPC health check
  │
  └── MCP 工具插件: Go 存储状态，但 TS 管实际进程
        │
        │  TS Bridge 通过 WS 心跳上报:
        │  (每 10s 一次)
        │  {
        │    "type": "heartbeat",
        │    "bridge_status": "healthy",
        │    "mcp_servers": [
        │      { "id": "github-tool", "status": "active", "pid": 1005, "uptime": "2h" },
        │      { "id": "web-search", "status": "active", "pid": 1006, "uptime": "1h" },
        │      { "id": "db-query",   "status": "crashed", "error": "OOM killed" }
        │    ]
        │  }
        │
        └── Go 收到心跳后:
              ├── 对比数据库中的 plugin_instances 状态
              ├── 如果不一致 → 以 TS 上报为准更新数据库
              ├── 如果 MCP Server 崩溃 → 指令 TS 重启 (HTTP POST /tools/restart)
              ├── 连续 3 次重启失败 → Go 标记为 Disabled + WS 通知前端
              └── 如果 TS Bridge WS 连接断开超过 30s → 所有 MCP 插件标记为 unknown
```

**异常恢复流程**:

| 场景 | 检测方 | 恢复动作 |
|------|--------|---------|
| MCP Server 崩溃 | TS (进程退出) → WS 上报 Go | Go 指令 TS 重启 (max 3 次) |
| TS Bridge 崩溃 | Go (WS 连接断开) | Go 重新启动 TS 进程，等待 WS 重连 |
| Go 重启 | TS (WS 连接断开) | TS 自动重连 Go 的 WS，重新上报所有状态 |
| go-plugin 崩溃 | Go (gRPC health check 失败) | Go 重启子进程 (go-plugin 内置) |

### 13.4 Token 计费双重控制

Token 消耗发生在 TS Bridge（Claude API 调用），但预算管理在 Go Orchestrator。

```
Go 下发任务:
  POST /bridge/execute {
    task_id: "task_xxx",
    role_config: { ... },
    budget: {
      max_tokens: 100000,       # TS 本地硬限制
      max_cost_usd: 5.0,        # TS 本地硬限制
      warn_threshold: 0.8       # 80% 时 WS 上报警告
    }
  }

TS Bridge 执行时:
  1. 每次 Claude API 调用后，本地累计 token 消耗
  2. 达到 warn_threshold → WS 推送警告事件给 Go
  3. 达到 max_tokens / max_cost_usd → 本地立即终止 Agent
  4. 每 5s 通过 WS 上报累计消耗:
     { "type": "cost_update", "task_id": "task_xxx",
       "tokens_used": 45000, "cost_usd": 2.15 }

Go Orchestrator 收到上报后:
  1. 更新任务级消耗记录
  2. 累加到 Sprint 级 / 项目级预算
  3. 如果项目级预算超限 → HTTP POST /bridge/cancel 终止 TS 所有任务
  4. 通过 WS 推送消耗数据到前端仪表盘
```

**为什么需要双重控制？**
- TS 本地限制：**兜底**。即使 Go→TS 的 abort 指令因网络延迟到达，TS 也不会超支
- Go 全局限制：**跨任务/跨Sprint**。只有 Go 有全局视野（多个 Agent 并发时的总消耗）

### 13.5 角色配置传递原则

**TS Bridge 是无状态执行器。** 每次 Go 调 TS 都传完整角色配置。

```
Go → TS POST /bridge/execute:
{
  "task_id": "task_xxx",
  "role": {                        # 完整角色配置，不传 role_id 让 TS 自己查
    "identity": { "role": "...", "goal": "...", "backstory": "..." },
    "system_prompt": "完整的系统提示词...",
    "tools": ["github-tool", "web-search"],   # TS 据此连接对应 MCP Server
    "knowledge_context": "注入的知识库内容...",
    "output_filters": ["no_credentials", "no_pii"]
  },
  "task": {
    "description": "修复 issue #42",
    "context": "相关代码片段..."
  },
  "budget": { ... }
}
```

**为什么不让 TS 缓存角色？**
- Go 随时可能更新角色配置（用户在 UI 修改了）
- 多个 TS 实例时不需要同步缓存
- 排查问题时，看 Go 的 HTTP 日志就知道传了什么配置，不用猜 TS 缓存了哪个版本

---

## 十四、插件分发与市场

### 12.1 MVP 阶段：Git + npm

```
分发渠道:
  ├── Role Plugin     → Git 仓库 (YAML 文件)
  ├── Tool Plugin     → npm 包 (@agentforge/tool-xxx)
  ├── Workflow Plugin  → Git 仓库 (YAML 文件)
  ├── Integration     → GitHub Release (Go 二进制)
  └── Review Plugin   → npm 包

安装方式:
  agentforge plugin install @agentforge/tool-web-search
  agentforge plugin install github:user/my-role-plugin
  agentforge plugin install ./local-plugin-dir
```

### 12.2 长期阶段：Plugin Registry

```
┌──────────────────────────────────────────┐
│        AgentForge Plugin Registry         │
│                                           │
│  ┌────────┐  ┌────────┐  ┌────────┐     │
│  │ 搜索   │  │ 分类   │  │ 评分   │     │
│  │ 浏览   │  │ 标签   │  │ 评论   │     │
│  │ 筛选   │  │ 排序   │  │ 统计   │     │
│  └────────┘  └────────┘  └────────┘     │
│                                           │
│  元数据: PostgreSQL                        │
│  制品存储: S3/MinIO (OCI 兼容)             │
│  安全: 自动扫描 + 人工审核 + 签名          │
│                                           │
│  CLI: agentforge plugin search/install    │
│  Web: plugins.agentforge.dev              │
│  API: registry.agentforge.dev/v1/...      │
└──────────────────────────────────────────┘
```

### 12.3 插件包格式

```
my-plugin.afpkg (tar.gz)
  ├── manifest.yaml          # 插件元数据
  ├── README.md              # 文档
  ├── CHANGELOG.md           # 变更日志
  ├── icon.png               # 图标
  ├── dist/                  # 编译产物
  │     ├── plugin           # Go 二进制 (Integration)
  │     └── index.js         # TS 入口 (Tool/Review)
  ├── skills/                # 技能文件 (Role)
  └── examples/              # 使用示例
```

---

## 十五、Plugin SDK 设计

### 13.1 SDK 组成

| SDK | 语言 | 用途 | 发布渠道 |
|-----|------|------|---------|
| `@agentforge/plugin-sdk` | TypeScript | Tool/Review 插件开发 | npm |
| `agentforge-plugin-sdk` | Go | Integration/Workflow 插件开发 | go module |
| `@agentforge/create-plugin` | TypeScript | 脚手架工具 | npm (npx) |

### 13.2 脚手架

```bash
# 创建新插件
npx @agentforge/create-plugin

? 插件名称: my-search-tool
? 插件类型: Tool Plugin (MCP)
? 开发语言: TypeScript
? 描述: 自定义搜索工具

✓ 创建 my-search-tool/
  ├── manifest.yaml
  ├── src/
  │     └── index.ts        # MCP Server 入口
  ├── tests/
  │     └── index.test.ts
  ├── package.json
  ├── tsconfig.json
  └── README.md

✓ 依赖安装完成
✓ 运行 `npm run dev` 开始开发
```

### 13.3 TS Tool Plugin SDK 示例

```typescript
// src/index.ts
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";

const server = new McpServer({
  name: "my-search-tool",
  version: "1.0.0",
});

// 定义工具
server.tool(
  "custom_search",
  "搜索自定义数据源",
  {
    query: z.string().describe("搜索关键词"),
    limit: z.number().default(10).describe("返回数量"),
  },
  async ({ query, limit }) => {
    const results = await doSearch(query, limit);
    return {
      content: [{ type: "text", text: JSON.stringify(results) }],
    };
  }
);

// 启动
const transport = new StdioServerTransport();
await server.connect(transport);
```

### 13.4 Go Integration Plugin SDK 示例

```go
// main.go
package main

import (
    "fmt"

    sdk "github.com/agentforge/plugin-sdk-go"
)

type FeishuAdapter struct{}

func (f *FeishuAdapter) Describe(ctx *sdk.Context) (*sdk.Descriptor, error) {
    return &sdk.Descriptor{
        APIVersion: "agentforge/v1",
        Kind:       "IntegrationPlugin",
        ID:         "feishu-adapter",
        Name:       "飞书集成",
        Version:    "1.0.0",
        Runtime:    "wasm",
        ABIVersion: sdk.ABIVersion,
        Capabilities: []sdk.Capability{
            {Name: "health"},
            {Name: "send_message"},
        },
    }, nil
}

func (f *FeishuAdapter) Init(ctx *sdk.Context) error {
    return nil
}

func (f *FeishuAdapter) Health(ctx *sdk.Context) (*sdk.Result, error) {
    return sdk.Success(map[string]any{
        "status": "ok",
    }), nil
}

func (f *FeishuAdapter) Invoke(ctx *sdk.Context, invocation sdk.Invocation) (*sdk.Result, error) {
    if invocation.Operation != "send_message" {
        return nil, sdk.NewRuntimeError("unsupported_operation", fmt.Sprintf("unsupported operation %s", invocation.Operation))
    }
    return sdk.Success(map[string]any{
        "status": "sent",
    }), nil
}

var runtime = sdk.NewRuntime(&FeishuAdapter{})

//go:wasmexport agentforge_abi_version
func agentforgeABIVersion() uint64 { return sdk.ExportABIVersion(runtime) }

//go:wasmexport agentforge_run
func agentforgeRun() uint32 { return sdk.ExportRun(runtime) }

func main() { sdk.Autorun(runtime) }
```

---

## 十六、与现有架构整合

### 16.1 对 PRD 各模块的影响

| PRD 模块 | 影响 | 变更 |
|---------|------|------|
| **任务管理中心** | 低 | 任务可触发工作流插件 |
| **Agent 编排引擎** | 中 | Agent 实例需关联 Role Plugin 配置 |
| **审查流水线** | 中 | Layer 2 开放为 Review Plugin 扩展点 |
| **IM 适配层** | 高 | 从 cc-connect fork 改为 Integration Plugin 架构 |
| **数据模型** | 中 | 新增插件相关表（见下节） |
| **Bridge HTTP/WS Surface** | 低 | 新增插件管理、catalog、MCP 交互与 runtime-state sync surface |
| **Tauri 桌面层** | 中 | 新增 sidecar 进程树管理、系统通知转发、原生能力 API |

### 16.1.1 Tauri 层新增职责

基于 react-go-quick-starter 的 Tauri 架构，AgentForge 桌面模式需要扩展：

```rust
// src-tauri/src/lib.rs 新增
use tauri::Manager;

#[tauri::command]
fn get_backend_url(state: State<BackendState>) -> String { ... }

// 新增: 插件相关 Tauri Commands
#[tauri::command]
fn get_plugin_status(state: State<AppState>) -> Vec<PluginStatus> {
    // 通过 HTTP 调用 Go Orchestrator /api/v1/plugins
}

#[tauri::command]
fn send_system_notification(title: String, body: String) -> Result<(), String> {
    // 原生系统通知 (插件事件转发)
}

// 新增: 监听 Go sidecar 的插件事件 (通过 stdout 日志)
fn handle_sidecar_output(line: &str, app_handle: &AppHandle) {
    if let Some(event) = parse_plugin_event(line) {
        // 转发到前端
        app_handle.emit("plugin-event", event).ok();
    }
}
```

**Tauri capabilities 扩展** (`src-tauri/capabilities/default.json`):
```json
{
  "permissions": [
    "core:default",
    "core:event:default",
    "notification:default",
    "notification:allow-notify",
    {
      "identifier": "shell:allow-execute",
      "allow": [
        { "name": "server", "sidecar": true }
      ]
    }
  ]
}
```

### 16.2 Go Orchestrator 新增模块

```go
// 新增到 Go Orchestrator
type PluginManager struct {
    roleRegistry       *RoleRegistry        // YAML 角色解析和管理
    wasmRuntimeManager *WASMRuntimeManager  // Go-hosted WASM 插件生命周期
    eventBus           *EventBus            // 当前实现的事件聚合与分发
    workflowEngine     *WorkflowEngine      // 工作流编排执行
}

// 接口定义
type RoleRegistry interface {
    LoadRole(path string) (*Role, error)
    GetRole(id string) (*Role, error)
    ListRoles() ([]*Role, error)
    ValidateRole(role *Role) error
}

type WASMRuntimeManager interface {
    Install(manifest *Manifest) error
    Enable(pluginID string) error
    Disable(pluginID string) error
    Call(pluginID string, method string, args interface{}) (interface{}, error)
    HealthCheck(pluginID string) error
}
```

### 16.3 TS Bridge 新增模块

```typescript
// 新增到 TS Agent SDK Bridge
class MCPClientHub {
  private clients: Map<string, MCPClient> = new Map();

  async connectServer(manifest: ToolManifest): Promise<void>;
  async disconnectServer(pluginId: string): Promise<void>;
  async callTool(pluginId: string, toolName: string, args: any): Promise<any>;
  async listTools(pluginId: string): Promise<Tool[]>;
  async discoverAllTools(): Promise<Tool[]>;
}
```

### 16.4 Bridge HTTP/WS 新增接口

```text
Go-facing control plane:

POST /api/v1/plugins/:id/mcp/refresh
POST /api/v1/plugins/:id/mcp/tools/call
POST /api/v1/plugins/:id/mcp/resources/read
POST /api/v1/plugins/:id/mcp/prompts/get
GET  /api/v1/plugins/catalog
POST /api/v1/plugins/catalog/install

TS Bridge internal surface:

POST /bridge/plugins/:id/mcp/refresh
POST /bridge/plugins/:id/mcp/tools/call
POST /bridge/plugins/:id/mcp/resources/read
POST /bridge/plugins/:id/mcp/prompts/get
POST /internal/plugins/runtime-state
```

### 16.5 当前仓库已落地的 MCP 交互控制面（2026-03-25）

当前实现延续 `Go 控制面 -> TS Bridge -> MCP Server` 的宿主边界，不开放前端直连 TS Bridge 的 MCP route。真实可用的 operator-facing 入口如下：

#### Go 控制面 API（对前端/CLI/自动化开放）

| 方法 | 路径 | 作用 |
|------|------|------|
| `POST` | `/api/v1/plugins/:id/mcp/refresh` | 刷新 ToolPlugin 的 MCP 能力面，返回工具/资源/提示词快照与能力计数 |
| `POST` | `/api/v1/plugins/:id/mcp/tools/call` | 通过 Go 代理调用 MCP tool，入参 `tool_name` + `arguments` |
| `POST` | `/api/v1/plugins/:id/mcp/resources/read` | 读取 MCP resource，入参 `uri` |
| `POST` | `/api/v1/plugins/:id/mcp/prompts/get` | 获取 MCP prompt 预览，入参 `name` + `arguments` |

这些接口只接受已注册且 `active` 的 `ToolPlugin`。Go 会先校验插件类型、宿主归属和必要字段，再调用 TS Bridge 内部 route。

#### TS Bridge 内部路由（仅 Go 使用）

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/bridge/plugins/:id/mcp/refresh` | 刷新并返回 `mcp_capability_snapshot` |
| `POST` | `/bridge/plugins/:id/mcp/tools/call` | 执行一次 typed tool call |
| `POST` | `/bridge/plugins/:id/mcp/resources/read` | 执行一次 typed resource read |
| `POST` | `/bridge/plugins/:id/mcp/prompts/get` | 执行一次 typed prompt get |

#### 运行态元数据（当前权威字段）

`ToolPlugin` 的权威运行态仍然保存在 Go 注册表记录里，但只持久化摘要，不持久化完整 MCP 响应体。当前 `runtime_metadata.mcp` 字段如下：

```json
{
  "runtime_metadata": {
    "mcp": {
      "transport": "stdio",
      "last_discovery_at": "2026-03-25T10:00:00Z",
      "tool_count": 2,
      "resource_count": 1,
      "prompt_count": 1,
      "latest_interaction": {
        "operation": "call_tool",
        "status": "succeeded",
        "at": "2026-03-25T10:05:00Z",
        "target": "search",
        "summary": "found 3 files",
        "error_code": "",
        "error_message": ""
      }
    }
  }
}
```

约束如下：

- `runtime_metadata.mcp` 只保存 transport、发现时间、能力计数和最近一次交互摘要。
- 完整工具返回体、资源正文、prompt 消息不会写入注册表，只通过实时 API 返回。
- Go 的 `/internal/plugins/runtime-state` 会接收 TS Bridge 上报的 `runtime_metadata.mcp`，并把它合并回同一条插件记录。

#### 审计事件（当前事件类型）

MCP 相关 operator 操作会写入插件审计流：

- `mcp_discovery`: 能力刷新成功/失败，payload 包含 `operation=refresh`、状态、目标和截断摘要。
- `mcp_interaction`: tool/resource/prompt 交互成功/失败，payload 包含 `operation`、`status`、`target`、`summary`、`error_code`、`error_message`。
- `runtime_sync`: TS Bridge 通过 `/internal/plugins/runtime-state` 上报时的宿主级同步事件。

推荐前端或诊断面板优先显示 `runtime_metadata.mcp.latest_interaction` 作为当前状态，再用 `plugin_events` 回看历史。

#### 当前仓库的 scoped verification

完成 MCP 控制面相关改动后，最小验证集合如下：

```bash
cd src-bridge
bun test src/mcp/client-hub.test.ts
bun test src/plugins/tool-plugin-manager.test.ts
bun test src/server.tools.test.ts

cd ../src-go
go test ./internal/bridge ./internal/service ./internal/handler ./internal/server -count=1
```

如果需要只验证 MCP 控制面的新增 Go 覆盖面，可进一步收窄到：

```bash
cd src-go
go test ./internal/bridge ./internal/service ./internal/handler ./internal/server -run 'TestClient(RefreshToolPluginMCPSurface|InvokeToolPluginMCPTool|ReadToolPluginMCPResource|GetToolPluginMCPPrompt)|TestPluginServiceControlPlane_(RefreshMCPPersistsSummaryAndAuditEvent|CallToolUpdatesLatestInteractionSummary|RuntimeStateSyncReconcilesMCPSummary)|TestPluginHandlerControlPlane_(MCPRefreshAndCallRoutes|MCPCallValidation|MCPResourceAndPromptRoutes)|TestRegisterRoutes_PluginControlPlaneCompatibilityRoutesPresent' -count=1
```

---

## 十七、分阶段实施路线

### Phase 0: 基础设施（第 1-2 周）

- [ ] 设计并实现统一 Manifest 解析器 (YAML → 强类型结构)
- [ ] Go 侧: Plugin Manager 框架 (接口定义 + 生命周期状态机)
- [ ] TS 侧: MCP Client Hub 基础框架
- [ ] 数据库: 插件相关表 (plugins, plugin_configs, plugin_instances)
- [ ] CLI: `agentforge plugin` 命令骨架

### Phase 1: 角色系统（第 3-4 周）

- [ ] Role Schema 解析和验证 (JSON Schema 校验)
- [ ] 角色继承和合并逻辑
- [ ] 内置 7 个预设角色模板
- [ ] 角色 ↔ Agent 实例绑定
- [ ] Web UI: 角色选择和配置面板
- [ ] CLI: `agentforge role create/list/apply`

### Phase 2: 工具插件（第 5-6 周）

- [ ] MCP Client Hub 完整实现 (stdio + HTTP 传输)
- [ ] 工具插件 Manifest 解析
- [ ] 工具懒加载和健康检查
- [ ] 集成 3-5 个常用 MCP Server (GitHub, Web Search, DB)
- [ ] CLI: `agentforge plugin install/enable/disable`

### Phase 3: 集成插件（第 7-8 周）

- [ ] go-plugin 基础设施搭建
- [ ] Integration Plugin 接口 (Protobuf 定义)
- [ ] 飞书 IM 适配器插件 (从 cc-connect 迁移)
- [ ] Event Bus (Redis Pub/Sub) 事件分发
- [ ] 插件进程监控和自动重启

### Phase 4: 工作流 + 审查（第 9-12 周）

- [ ] 工作流 YAML 解析和执行引擎
- [ ] Sequential 编排模式实现
- [ ] Review Plugin 扩展点开放
- [ ] 1-2 个内置审查插件
- [ ] Web UI: 工作流可视化 (只读，展示执行状态)

### Phase 5: 生态建设（第 13-18 周）

- [ ] Plugin SDK 发布 (TS + Go)
- [ ] 脚手架工具 `create-plugin`
- [ ] 插件开发文档站
- [ ] Plugin Registry 基础版
- [ ] Hierarchical 编排模式
- [ ] WASM 运行时评估和原型

---

## 十八、数据模型扩展

### 新增表

```sql
-- 插件注册表
CREATE TABLE plugins (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_id       VARCHAR(128) NOT NULL UNIQUE,  -- manifest.metadata.id
    kind            VARCHAR(32) NOT NULL,           -- Role|Tool|Workflow|Integration|Review
    name            VARCHAR(256) NOT NULL,
    version         VARCHAR(32) NOT NULL,
    author          VARCHAR(128),
    description     TEXT,
    tags            TEXT[],
    manifest        JSONB NOT NULL,                 -- 完整 manifest
    status          VARCHAR(32) NOT NULL DEFAULT 'installed',  -- installed|enabled|disabled
    signature       TEXT,                           -- 签名 (可选)
    installed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 插件实例（运行时状态）
CREATE TABLE plugin_instances (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_id       UUID NOT NULL REFERENCES plugins(id),
    project_id      UUID NOT NULL REFERENCES projects(id),
    config          JSONB,                          -- 项目级配置覆盖
    status          VARCHAR(32) NOT NULL DEFAULT 'inactive',  -- inactive|active|error
    pid             INTEGER,                        -- 子进程 PID
    last_health     TIMESTAMPTZ,
    error_count     INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 角色定义（Role Plugin 具体化）
CREATE TABLE roles (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_id       UUID REFERENCES plugins(id),    -- 可选关联插件
    role_id         VARCHAR(128) NOT NULL,
    name            VARCHAR(256) NOT NULL,
    definition      JSONB NOT NULL,                 -- 完整角色定义
    parent_role_id  VARCHAR(128),                   -- 继承的父角色
    is_builtin      BOOLEAN NOT NULL DEFAULT false,
    project_id      UUID REFERENCES projects(id),   -- NULL = 全局角色
    created_by      UUID REFERENCES members(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(role_id, project_id)
);

-- 插件事件日志（审计）
CREATE TABLE plugin_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_id       UUID NOT NULL REFERENCES plugins(id),
    project_id      UUID REFERENCES projects(id),
    event_type      VARCHAR(64) NOT NULL,           -- install|enable|call|error|...
    event_data      JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
) PARTITION BY RANGE (created_at);

-- 索引
CREATE INDEX idx_plugins_kind ON plugins(kind);
CREATE INDEX idx_plugins_status ON plugins(status);
CREATE INDEX idx_plugin_instances_project ON plugin_instances(project_id);
CREATE INDEX idx_roles_project ON roles(project_id);
CREATE INDEX idx_plugin_events_plugin ON plugin_events(plugin_id, created_at);
```

### 修改现有表

```sql
-- agent_runs 表新增角色关联
ALTER TABLE agent_runs ADD COLUMN role_id UUID REFERENCES roles(id);

-- tasks 表新增工作流关联
ALTER TABLE tasks ADD COLUMN workflow_instance_id UUID;

-- members 表区分人类/数字员工
-- （已有 is_agent boolean，无需修改）
```

---

## 附录

### A. 技术选型决策记录

| 决策 | 选项 | 选择 | 理由 |
|------|------|------|------|
| Go ↔ TS 通信 | gRPC vs HTTP vs stdio | **HTTP (命令) + WS (事件流)** | 调试友好（curl/wscat），无 protobuf 代码生成，WS 天然流式 |
| Agent 工具扩展协议 | MCP vs gRPC vs REST | **MCP** | AI 行业标准，Claude SDK 原生支持 |
| Go 侧插件运行时 | go-plugin vs Yaegi vs WASM | **go-plugin** | Terraform/Vault 验证，最成熟 |
| TS Bridge 打包 | Bun compile vs Node SEA vs Deno compile | **Bun compile** | 一命令跨平台编译，依赖全纯 JS 无兼容问题，Claude Code 已验证 |
| TS 侧工具沙箱 | isolated-vm vs Worker vs QuickJS | **MCP 进程隔离** | MCP Server 天然进程隔离，无需额外沙箱 |
| 角色定义格式 | YAML vs JSON vs Protobuf | **YAML** | 人类可读，支持多行文本，版本控制友好 |
| 跨语言插件 (长期) | Extism vs WasmEdge vs Container | **Extism** | Go+TS 双侧 SDK，wazero 零依赖 |
| 插件分发 (MVP) | npm vs OCI vs Git | **npm + Git** | 开发者最熟悉，零基础设施 |
| 插件分发 (长期) | 自建 vs 第三方 | **自建 OCI Registry** | 可控、可定制、统一多类型插件 |
| 事件系统 (MVP) | Redis PubSub vs 内存 EventEmitter | **内存 EventEmitter + WS Hub** | 单实例够用，不引入额外依赖 |
| 事件系统 (长期) | Redis Streams vs Kafka vs NATS | **Redis Streams** | 多 Go 实例时引入，已有 Redis |
| TS Bridge 状态模型 | 有状态缓存 vs 无状态 | **无状态执行器** | 每次传完整配置，Go 是 single source of truth |
| Token 计费 | Go 单点 vs 双重控制 | **双重控制** | TS 本地硬限制兜底 + Go 全局预算跨任务管控 |

### B. 参考项目

| 项目 | 借鉴点 |
|------|--------|
| **Terraform Provider** | go-plugin 架构、版本协商、Plugin SDK 模式 |
| **Dify Plugin** | Manifest 规范、签名机制、多运行时模式 |
| **VS Code Extension** | 懒加载、Activation Events、Extension Host |
| **Grafana Plugin SDK** | 前后端插件分离、实例管理、内置 Observability |
| **Backstage** | Extension Points、Service 注入 |
| **CrewAI** | 角色定义 (role/goal/backstory)、YAML 配置 |
| **MCP 协议** | 工具/资源/提示三大原语、JSON-RPC 通信 |
| **Coze Studio** | Go+TS 架构、DAG 工作流、节点扩展 |

### C. 风险评估

| 风险 | 影响 | 概率 | 缓解策略 |
|------|------|------|---------|
| 插件生态冷启动 | 高 | 高 | 内置 10+ 官方插件，提供优质 SDK 和文档 |
| 插件安全漏洞 | 高 | 中 | 进程隔离 + 权限声明 + 签名验证 |
| 插件性能影响 | 中 | 中 | 懒加载 + 资源限制 + 健康检查 |
| MCP 协议变更 | 中 | 低 | 抽象层隔离，跟踪协议演进 |
| go-plugin 维护风险 | 低 | 低 | HashiCorp 核心依赖，长期维护有保障 |

---

> 本文档基于 3 份并行调研报告综合设计：
> - `PLUGIN_RESEARCH_PLATFORMS.md` — 9 大平台插件架构分析
> - `PLUGIN_RESEARCH_ROLES.md` — 角色自定义最佳实践
> - `PLUGIN_RESEARCH_TECH.md` — 技术实现方案对比
