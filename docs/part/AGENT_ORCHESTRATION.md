# AgentForge Agent 编排架构设计

> 版本：v1.0 | 日期：2026-03-22
> 基于 PRD v1.0、后端技术选型（Go）、Claude Agent SDK (TypeScript)
>
> v1.2 live contract update: 当前 Go↔TS Bridge 实现以 HTTP 命令 + WebSocket 事件流为准，canonical route family 为 `/bridge/*`。本文保留的 gRPC / proto 片段仅作历史参考，不能当作当前实现入口。

---

## 当前实现快照（2026-03-29）

这份文档描述的是 AgentForge 编排层的完整蓝图，但当前仓库已经有一条可工作的实现主线，阅读后续章节时应优先用下面这些 live contract 理解“当前系统怎么跑”：

- TS Bridge 的 canonical HTTP surface 以 `/bridge/*` 为准，当前已落地 `execute`、`decompose`、`classify-intent`、`generate`、`review`、`status`、`runtimes`、`cancel`、`pause`、`resume`、`health`、`active`、`pool`，兼容 alias 仅用于历史调用迁移。
- Go 侧当前公开的 AI/Bridge 入口同时包含 `GET /api/v1/bridge/health`、`GET /api/v1/bridge/runtimes`、`POST /api/v1/ai/generate`、`POST /api/v1/ai/classify-intent`，而不是文档里更早期的 gRPC 服务发现模型。
- 编码运行时已经不再只是 Claude 单一路径。当前 runtime catalog 支持 `claude_code`、`codex`、`opencode`，并由 Go 负责把 `runtime/provider/model` 元组解析后传给 Bridge。
- Codex 与 OpenCode continuity 已有真实实现：Codex 通过 `codex exec --json` / `codex exec resume <thread-id>` 保留 `thread_id`，OpenCode 通过 `OPENCODE_SERVER_URL` 指向的 `/session` / `/prompt_async` / `/abort` 维持上游 `session_id`。
- 实时事件当前通过 TS Bridge 主动连回 Go 的 WebSocket hub，再由 Go 广播到前端与其他消费者；当前实现不是 Redis Streams 驱动的多跳主链路。
- Team 生命周期上下文已显式进入运行时请求：planner / coder / reviewer 会保留 `team_id`、`team_role` 和统一的 runtime tuple，而不是只靠数据库侧推断阶段。

---

## 目录

1. [整体编排架构](#1-整体编排架构)
2. [Agent SDK Bridge 设计](#2-agent-sdk-bridge-设计)
3. [Agent 池管理](#3-agent-池管理)
4. [任务到 Agent 分配流程](#4-任务到-agent-分配流程)
5. [Git Worktree 管理](#5-git-worktree-管理)
6. [成本控制实现](#6-成本控制实现)
7. [会话管理](#7-会话管理)
8. [多 Agent 模式（P2）](#8-多-agent-模式p2)
9. [竞品编排对比](#9-竞品编排对比)
10. [可观测性](#10-可观测性)

---

## 1. 整体编排架构

### 1.1 核心矛盾与决策

AgentForge 的编排层面临一个核心架构决策：**Go 是最佳后端语言（goroutine 并发模型天然适配 Agent 生命周期管理、go-git 纯 Go 实现），但 Claude Agent SDK 仅提供 TypeScript/Python 版本**。

我们采用 **Go Orchestrator + TypeScript Agent SDK Bridge** 双进程架构。**关键决策：Bridge 是所有 AI 调用的统一出口，Go 侧不直接调用任何 LLM（去掉 LangChainGo 依赖）**。

| 组件 | 语言 | 职责范围 |
|------|------|---------|
| **Go Orchestrator** | Go | 纯业务逻辑：API 网关、任务调度、Agent 池管理、成本控制、Git 操作（go-git）、WebSocket Hub、IM Bridge — **不直接调用 LLM** |
| **TS Agent SDK Bridge** | TypeScript | **所有 AI 调用统一出口**：Agent 编码执行（query()）、AI 任务分解、IM 意图识别、Review Agent、会话管理、流式输出中继 |
| **前端 Vercel AI SDK v6** | TypeScript | 用户侧 AI 交互：聊天 UI、流式渲染、多 Provider 切换 |

**两条 AI 调用路径（取代原来的三条）：**

```
路径 1（前端 UI）: Dashboard → Vercel AI SDK v6 → Claude/GPT（用户聊天交互）
路径 2（所有后端 AI）: Go → HTTP(`/bridge/*`) + WS 事件流 → Bridge → Claude Agent SDK / Vercel AI SDK → LLM
                        · Agent 编码执行（query()，重量级）
                        · AI 任务分解（generateText()，轻量级）
                        · IM 意图识别（generateText()，轻量级）
                        · Review Agent（query()，重量级）
```

**统一出口的好处：**
- 单一成本追踪点：所有 Token 消耗经过 Bridge，统一计费
- 单一 API Key 管理：Go 侧无需配置 LLM 密钥
- 减少依赖：Go 后端不依赖 LangChainGo，纯业务逻辑更清晰
- 多 Provider 扩展：Bridge 可通过 Vercel AI SDK Provider 抽象层接入其他模型

### 1.2 整体架构图

```
                           用户层
     ┌──────────┐  ┌──────────┐  ┌──────────────────────────┐
     │ 飞书/钉钉 │  │ Slack/TG │  │ Web Dashboard             │
     │ 企微/QQ   │  │ Discord  │  │ (Next.js 16 + shadcn/ui)  │
     └─────┬─────┘  └─────┬────┘  └────────────┬──────────────┘
           │               │                    │
     ┌─────▼───────────────▼────┐               │
     │   IM Bridge Service       │               │
     │   (cc-connect fork, Go)   │               │
     └──────────┬────────────────┘               │
                │ HTTP/WS                        │ REST / WebSocket / SSE
                │                                │
  ┌─────────────▼────────────────────────────────▼────────────────────┐
  │                     Go Orchestrator (Fiber/Echo)                   │
  │                                                                    │
  │  ┌──────────────┐ ┌──────────────┐ ┌───────────────────────────┐  │
  │  │ Task Service  │ │ Agent Pool   │ │ Review Pipeline           │  │
  │  │               │ │ Manager      │ │ Coordinator               │  │
  │  │ · CRUD        │ │              │ │                           │  │
  │  │ · CRUD        │ │ · 池调度     │ │ · claude-code-action 触发 │  │
  │  │ · 智能分配    │ │ · 生命周期   │ │ · 自建 Review Agent       │  │
  │  │ · 进度追踪    │ │ · 资源限制   │ │ · 假阳性过滤              │  │
  │  └───────┬──────┘ └──────┬───────┘ └──────────┬────────────────┘  │
  │          │               │                     │                   │
  │  ┌───────▼───────────────▼─────────────────────▼────────────────┐  │
  │  │                    Orchestration Core                         │  │
  │  │                                                              │  │
  │  │  ┌──────────────┐ ┌───────────────┐ ┌────────────────────┐  │  │
  │  │  │ Cost Tracker  │ │ Worktree Mgr  │ │ WebSocket Hub      │  │  │
  │  │  │ (实时计费)    │ │ (go-git)      │ │ (Redis Streams)    │  │  │
  │  │  └──────────────┘ └───────────────┘ └────────────────────┘  │  │
  │  └──────────────────────────┬───────────────────────────────────┘  │
  └─────────────────────────────┼──────────────────────────────────────┘
                                │ HTTP 命令 + WS 事件流
                                │
  ┌─────────────────────────────▼──────────────────────────────────────┐
  │                  Agent SDK Bridge (TypeScript)                      │
  │                                                                    │
  │  ┌──────────────┐ ┌────────────────┐ ┌─────────────────────────┐  │
  │  │ HTTP Route +  │ │ Agent Runtime  │ │ Session Manager          │  │
  │  │ WS Relay      │ │ Pool           │ │                          │  │
  │  │ · /bridge/*   │ │                │ │ · 会话快照               │  │
  │  │ · WS 上报     │ │ · query() ×N   │ │ · 暂停/恢复             │  │
  │  │ · 兼容 alias  │ │ · 工具沙箱     │ │ · 崩溃恢复              │  │
  │  │ · 健康检查    │ │ · Hook 链      │ │                          │  │
  │  └──────────────┘ └────────────────┘ └─────────────────────────┘  │
  │                           │                                        │
  │                    Claude Agent SDK                                │
  │                    @anthropic-ai/claude-agent-sdk                  │
  │                    query() → async iterator → messages             │
  └────────────────────────────────────────────────────────────────────┘
                                │
                         Claude API (Anthropic)
```

### 1.3 数据流路径

```
用户在飞书发消息: "帮我修复 issue #42"
    │
    ▼
IM Bridge → Go API Gateway → Task Service (创建任务)
    │
    ▼
Task Service → Orchestration Core → Agent Pool Manager
    │
    ├── 1. Worktree Manager: 创建 /data/worktrees/<project>/<task-id>/
    ├── 2. Cost Tracker: 初始化预算追踪 (budget_usd: $5.00)
    └── 3. Agent Pool Manager: 选择/创建 Agent 实例
            │
            ▼
        HTTP POST /bridge/execute → Agent SDK Bridge
            │
            ▼
        Bridge 调用 query({
            prompt: "修复 issue #42: token 刷新逻辑 bug...",
            options: {
                allowedTools: ["Read", "Edit", "Bash", "Glob", "Grep"],
                permissionMode: "bypassPermissions",
                systemPrompt: "你是 AgentForge 编码 Agent..."
            }
        })
            │
            ▼
        Claude API ← 流式 token →  Bridge ← WebSocket 事件流 → Go Backend
                                                                │
                                                    ┌───────────┼───────────┐
                                                    ▼           ▼           ▼
                                               WebSocket    IM Bridge   PostgreSQL
                                               (前端)       (飞书通知)  (持久化)
```

---

## 2. Agent SDK Bridge 设计

### 2.1 Bridge 服务定位

Agent SDK Bridge 是一个独立的 TypeScript/Bun 服务，其核心职责是：

1. 封装 `@anthropic-ai/claude-agent-sdk` 的 `query()` 调用
2. 通过 HTTP canonical `/bridge/*` routes 与 WebSocket 事件流暴露 Agent 执行能力给 Go Orchestrator
3. 管理多个并发 Agent 运行时实例
4. 处理 Agent SDK 的会话状态（session snapshot/resume）
5. 执行工具沙箱策略（白名单、路径限制）

### 2.2 历史 Proto 参考（非 live contract）

```protobuf
syntax = "proto3";
package agentforge.bridge;

option go_package = "github.com/agentforge/agentforge/pkg/proto/bridge";

// =============================================================
// Agent SDK Bridge 服务定义
// =============================================================
service AgentBridge {
  // 双向流：Go 发送命令，TS 返回 Agent 事件流
  rpc Execute(stream AgentCommand) returns (stream AgentEvent);

  // 查询 Agent 当前状态
  rpc GetStatus(StatusRequest) returns (AgentStatus);

  // 终止 Agent 执行
  rpc Cancel(CancelRequest) returns (CancelResponse);

  // 健康检查
  rpc HealthCheck(HealthRequest) returns (HealthResponse);

  // 批量查询所有活跃 Agent
  rpc ListActive(ListActiveRequest) returns (ListActiveResponse);

  // ─── 轻量 AI 调用（不启动 Agent 实例，直接调 LLM）───
  // AI 任务分解：大任务 → 子任务列表
  rpc DecomposeTask(DecomposeRequest) returns (DecomposeResponse);

  // IM 意图识别：自然语言 → 结构化命令
  rpc ClassifyIntent(IntentRequest) returns (IntentResponse);

  // 通用 AI 补全（用于其他轻量场景：日报生成、新人引导等）
  rpc GenerateText(GenerateTextRequest) returns (GenerateTextResponse);
}

// =============================================================
// 命令消息（Go → TS Bridge）
// =============================================================
message AgentCommand {
  string task_id = 1;
  string session_id = 2;

  oneof command {
    ExecuteTask execute = 3;
    PauseTask pause = 4;
    ResumeTask resume = 5;
    ProvideInput provide_input = 6;
  }
}

message ExecuteTask {
  string prompt = 1;
  string worktree_path = 2;
  string branch_name = 3;
  string system_prompt = 4;
  int32 max_turns = 5;
  double budget_usd = 6;
  repeated string allowed_tools = 7;
  string permission_mode = 8;        // "bypassPermissions" | "acceptEdits"
  repeated McpServerConfig mcp_servers = 9;
  string session_snapshot = 10;       // 恢复用：上次会话快照 JSON
  string team_id = 11;                // Team 流程中的显式团队身份
  string team_role = 12;              // "planner" | "coder" | "reviewer"
}

message PauseTask {}

message ResumeTask {
  string session_snapshot = 1;
}

message ProvideInput {
  string input = 1;
}

message McpServerConfig {
  string name = 1;
  string url = 2;
  map<string, string> env = 3;
}

// =============================================================
// 事件消息（TS Bridge → Go）
// =============================================================
message AgentEvent {
  string task_id = 1;
  string session_id = 2;
  int64 timestamp_ms = 3;

  oneof event {
    AgentOutput output = 10;
    ToolCall tool_call = 11;
    ToolResult tool_result = 12;
    StatusChange status_change = 13;
    CostUpdate cost_update = 14;
    AgentError error = 15;
    SessionSnapshot snapshot = 16;
  }
}

message AgentOutput {
  string content = 1;
  string content_type = 2;  // "text" | "code" | "diff" | "markdown"
  int32 turn_number = 3;
}

message ToolCall {
  string tool_name = 1;
  string tool_input = 2;   // JSON 序列化的工具参数
  string call_id = 3;
}

message ToolResult {
  string call_id = 1;
  string output = 2;
  bool is_error = 3;
}

message StatusChange {
  string old_status = 1;
  string new_status = 2;   // "starting" | "running" | "paused" | "completed" | "failed"
  string reason = 3;
}

message CostUpdate {
  int64 input_tokens = 1;
  int64 output_tokens = 2;
  int64 cache_read_tokens = 3;
  double cost_usd = 4;
  double budget_remaining_usd = 5;
}

message AgentError {
  string code = 1;           // "RATE_LIMIT" | "BUDGET_EXCEEDED" | "SESSION_EXPIRED"
  string message = 2;
  map<string, string> metadata = 3;
  bool retryable = 4;
}

message SessionSnapshot {
  string snapshot_data = 1;  // JSON: conversation history + tool state
  int32 turn_number = 2;
  double spent_usd = 3;
}

// =============================================================
// 辅助消息
// =============================================================
message StatusRequest { string task_id = 1; }
message AgentStatus {
  string task_id = 1;
  string state = 2;          // "idle" | "thinking" | "tool_executing" | "stuck"
  int32 turn_number = 3;
  string last_tool = 4;
  int64 last_activity_ms = 5;
  double spent_usd = 6;
  string runtime = 7;
  string provider = 8;
  string model = 9;
  string role_id = 10;
  string team_id = 11;
  string team_role = 12;
}

message CancelRequest { string task_id = 1; string reason = 2; }
message CancelResponse { bool success = 1; }

message HealthRequest {}
message HealthResponse {
  string status = 1;          // "SERVING" | "NOT_SERVING"
  int32 active_agents = 2;
  int32 max_agents = 3;
  int64 uptime_ms = 4;
}

message ListActiveRequest {}
message ListActiveResponse { repeated AgentStatus agents = 1; }
```

### 2.3 TypeScript Bridge 实现

```typescript
// src/bridge-server.ts
import { Server, ServerCredentials, ServerDuplexStream } from "@grpc/grpc-js";
import { query, ClaudeAgentOptions, AssistantMessage, ResultMessage } from "@anthropic-ai/claude-agent-sdk";
import { AgentCommand, AgentEvent } from "./proto/bridge";

interface AgentRuntime {
  taskId: string;
  sessionId: string;
  abortController: AbortController;
  status: "starting" | "running" | "paused" | "completed" | "failed";
  turnNumber: number;
  spentUsd: number;
  lastActivity: number;
}

class AgentBridgeServer {
  private runtimes: Map<string, AgentRuntime> = new Map();
  private maxConcurrent: number;

  constructor(maxConcurrent = 10) {
    this.maxConcurrent = maxConcurrent;
  }

  /**
   * 双向流 RPC：接收 Go 侧命令，返回 Agent 事件流
   */
  async execute(
    call: ServerDuplexStream<AgentCommand, AgentEvent>
  ): Promise<void> {
    call.on("data", async (command: AgentCommand) => {
      if (command.execute) {
        await this.handleExecute(call, command);
      } else if (command.pause) {
        await this.handlePause(call, command.taskId);
      } else if (command.resume) {
        await this.handleResume(call, command);
      }
    });

    call.on("end", () => call.end());
  }

  /**
   * 执行 Agent 任务 -- 核心方法
   */
  private async handleExecute(
    call: ServerDuplexStream<AgentCommand, AgentEvent>,
    command: AgentCommand
  ): Promise<void> {
    const { taskId, sessionId } = command;
    const exec = command.execute!;

    // 检查并发限制
    if (this.runtimes.size >= this.maxConcurrent) {
      call.write(this.makeErrorEvent(taskId, sessionId, {
        code: "MAX_CONCURRENT",
        message: `已达最大并发 Agent 数 (${this.maxConcurrent})`,
        retryable: true,
      }));
      return;
    }

    const abortController = new AbortController();
    const runtime: AgentRuntime = {
      taskId,
      sessionId,
      abortController,
      status: "starting",
      turnNumber: 0,
      spentUsd: 0,
      lastActivity: Date.now(),
    };
    this.runtimes.set(taskId, runtime);

    // 发送状态变更事件
    call.write(this.makeStatusEvent(taskId, sessionId, "idle", "starting"));

    try {
      // 构建 Agent SDK 选项
      const options: ClaudeAgentOptions = {
        allowedTools: exec.allowedTools.length > 0
          ? exec.allowedTools
          : ["Read", "Edit", "Bash", "Glob", "Grep"],
        permissionMode: (exec.permissionMode as any) || "bypassPermissions",
        systemPrompt: exec.systemPrompt || this.defaultSystemPrompt(taskId),
        cwd: exec.worktreePath,
        abortSignal: abortController.signal,
        maxTurns: exec.maxTurns || 30,
      };

      // 如果有 MCP 服务器配置
      if (exec.mcpServers?.length) {
        options.mcpServers = exec.mcpServers.map((s) => ({
          name: s.name,
          url: s.url,
          env: Object.fromEntries(Object.entries(s.env)),
        }));
      }

      runtime.status = "running";
      call.write(this.makeStatusEvent(taskId, sessionId, "starting", "running"));

      // 核心：调用 Claude Agent SDK query()
      for await (const message of query({
        prompt: exec.prompt,
        options,
      })) {
        runtime.lastActivity = Date.now();

        if (message instanceof AssistantMessage) {
          for (const block of message.content) {
            if ("text" in block) {
              call.write({
                taskId, sessionId,
                timestampMs: Date.now(),
                output: {
                  content: block.text,
                  contentType: "text",
                  turnNumber: runtime.turnNumber,
                },
              });
            }
            if ("name" in block) {
              runtime.turnNumber++;
              call.write({
                taskId, sessionId,
                timestampMs: Date.now(),
                toolCall: {
                  toolName: block.name,
                  toolInput: JSON.stringify(block.input),
                  callId: block.id || "",
                },
              });
            }
          }
        }

        // 提取 usage 信息生成 CostUpdate
        if (message.usage) {
          const costUsd = this.calculateCost(message.usage);
          runtime.spentUsd += costUsd;
          call.write({
            taskId, sessionId,
            timestampMs: Date.now(),
            costUpdate: {
              inputTokens: message.usage.input_tokens || 0,
              outputTokens: message.usage.output_tokens || 0,
              cacheReadTokens: message.usage.cache_read_input_tokens || 0,
              costUsd,
              budgetRemainingUsd: exec.budgetUsd - runtime.spentUsd,
            },
          });

          // 预算检查
          if (runtime.spentUsd >= exec.budgetUsd) {
            abortController.abort();
            call.write(this.makeErrorEvent(taskId, sessionId, {
              code: "BUDGET_EXCEEDED",
              message: `预算已耗尽: $${runtime.spentUsd.toFixed(4)} / $${exec.budgetUsd}`,
              retryable: false,
            }));
            break;
          }
        }

        if (message instanceof ResultMessage) {
          runtime.status = "completed";
          call.write(this.makeStatusEvent(
            taskId, sessionId, "running", "completed", message.subtype
          ));
        }
      }
    } catch (err: any) {
      runtime.status = "failed";
      const agentError = this.classifyError(err);
      call.write(this.makeErrorEvent(taskId, sessionId, agentError));
    } finally {
      this.runtimes.delete(taskId);
    }
  }

  /**
   * 错误分类：将 SDK 异常映射为结构化错误
   */
  private classifyError(
    err: any
  ): { code: string; message: string; retryable: boolean } {
    if (err.status === 429) {
      return { code: "RATE_LIMIT", message: err.message, retryable: true };
    }
    if (err.status === 401 || err.status === 403) {
      return { code: "AUTH_FAILED", message: err.message, retryable: false };
    }
    if (err.name === "AbortError") {
      return { code: "CANCELLED", message: "Agent 被取消", retryable: false };
    }
    return { code: "INTERNAL", message: err.message, retryable: false };
  }

  /**
   * 成本计算（Claude 定价）
   */
  private calculateCost(usage: any): number {
    const INPUT_PER_MTOK = 3.00;   // Claude Sonnet 4
    const OUTPUT_PER_MTOK = 15.00;
    const CACHE_READ_PER_MTOK = 0.30;

    const input = (usage.input_tokens || 0) / 1_000_000 * INPUT_PER_MTOK;
    const output = (usage.output_tokens || 0) / 1_000_000 * OUTPUT_PER_MTOK;
    const cache = (usage.cache_read_input_tokens || 0) / 1_000_000
                  * CACHE_READ_PER_MTOK;

    return input + output + cache;
  }

  private defaultSystemPrompt(taskId: string): string {
    return `你是 AgentForge 平台的编码 Agent。你的任务 ID 是 ${taskId}。
规则：
1. 仅在指定的 worktree 目录内操作
2. 完成编码后运行测试确保通过
3. 使用 git add + git commit 提交你的修改
4. commit message 格式: feat/fix/refactor: <描述> [agent/${taskId}]
5. 不要修改 .env、密钥文件或配置文件中的凭证`;
  }

  // ... 辅助方法: makeStatusEvent, makeErrorEvent 等
}
```

### 2.4 流式支持详解

Bridge 的流式架构在当前实现里是 HTTP 命令 + WebSocket 事件回传主链路：

```
Claude API ──token──→ Agent SDK ──message──→ Bridge ──AgentEvent──→ Go WS Hub
   (HTTP SSE)        (async iter)          (JSON encode)          (project-scoped broadcast)
                                                                        │
                                                              ┌─────────┼─────────┐
                                                              ▼         ▼         ▼
                                                         Frontend    IM Bridge   PostgreSQL
                                                         Dashboard   / sidecars  / task state
```

历史 gRPC Stream 设计仍可作为架构演化参考，但当前 live path 是 TS Bridge 主动连接 Go WebSocket hub 并上报事件。Go 侧再按项目维度扇出到前端、IM 与持久化消费者。

Go 侧当前的扇出模型可以抽象为：

```go
func (s *AgentService) relayStream(stream pb.AgentBridge_ExecuteClient, taskID string) {
    // 使用带缓冲的 channel 防止背压
    eventCh := make(chan *pb.AgentEvent, 256)

    // 启动多个消费者 goroutine
    var wg sync.WaitGroup

    wg.Add(3)
    go func() { defer wg.Done(); s.wsHub.BroadcastEvents(taskID, eventCh) }()
    go func() { defer wg.Done(); s.imBridge.ForwardEvents(taskID, eventCh) }()
    go func() { defer wg.Done(); s.agentRunRepo.BatchPersist(taskID, eventCh) }()

    // 生产者：从 gRPC stream 读取事件
    for {
        event, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            s.handleStreamError(taskID, err)
            break
        }

        // 扇出到所有消费者（非阻塞）
        select {
        case eventCh <- event:
        default:
            // 缓冲区满：记录指标，丢弃最旧事件
            metrics.AgentEventDropped.WithLabelValues(taskID).Inc()
        }
    }

    close(eventCh)
    wg.Wait()
}
```

---

## 3. Agent 池管理

### 3.1 池化策略

Agent 池采用 **弹性池 + 预热** 模式，在启动延迟和资源利用之间取得平衡：

```
┌───────────────────────────────────────────────────────────┐
│                    Agent Pool Manager                      │
│                                                           │
│  ┌────────────────────────────────────────────────────┐   │
│  │  预热池 (Warm Pool)                                 │   │
│  │  ┌────────┐ ┌────────┐ ┌────────┐                 │   │
│  │  │ Agent  │ │ Agent  │ │ Agent  │  ← 已初始化     │   │
│  │  │ (idle) │ │ (idle) │ │ (idle) │    Bridge 连接   │   │
│  │  └────────┘ └────────┘ └────────┘    就绪          │   │
│  │  warmPoolSize: 2-3 (可配置)                        │   │
│  └────────────────────────────────────────────────────┘   │
│                                                           │
│  ┌────────────────────────────────────────────────────┐   │
│  │  活跃池 (Active Pool)                               │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ │   │
│  │  │ Agent 1 │ │ Agent 2 │ │ Agent 3 │ │ ...     │ │   │
│  │  │ task-a  │ │ task-b  │ │ task-c  │ │         │ │   │
│  │  │ running │ │ running │ │ paused  │ │         │ │   │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ │   │
│  │  maxActive: 20 (受服务器资源限制)                    │   │
│  └────────────────────────────────────────────────────┘   │
│                                                           │
│  ┌────────────────────────────────────────────────────┐   │
│  │  等待队列 (Wait Queue)                              │   │
│  │  [task-d (priority:1)] → [task-e (priority:2)] → ...│  │
│  │  Redis Sorted Set, 按优先级排序                      │   │
│  └────────────────────────────────────────────────────┘   │
└───────────────────────────────────────────────────────────┘
```

### 3.2 池大小策略

```go
type PoolConfig struct {
    // 核心参数
    MaxActive       int           `yaml:"max_active" default:"20"`
    WarmPoolSize    int           `yaml:"warm_pool_size" default:"2"`
    MaxQueueSize    int           `yaml:"max_queue_size" default:"100"`

    // 资源限制（每 Agent）
    PerAgentMemoryMB   int       `yaml:"per_agent_memory_mb" default:"512"`
    PerAgentCPUShares  int       `yaml:"per_agent_cpu_shares" default:"1024"`

    // 超时
    IdleTimeout        time.Duration `yaml:"idle_timeout" default:"5m"`
    MaxExecutionTime   time.Duration `yaml:"max_execution_time" default:"30m"`
    WarmPoolRefreshInterval time.Duration `yaml:"warm_refresh" default:"10m"`
}

// 动态池大小计算
func (pc *PoolConfig) CalculateMaxActive() int {
    availableMemoryMB := getAvailableMemoryMB()
    availableCPUs := runtime.NumCPU()

    memoryBased := availableMemoryMB / pc.PerAgentMemoryMB
    cpuBased := availableCPUs * 4  // 每核心可支持约 4 个 I/O 密集型 Agent

    calculated := min(memoryBased, cpuBased)
    return min(calculated, pc.MaxActive) // 不超过配置上限
}
```

### 3.3 预热池 vs 冷启动

| 指标 | 预热池 Agent | 冷启动 Agent |
|------|-------------|-------------|
| 启动延迟 | < 500ms（直接分配） | 3-8s（Bridge 初始化 + SDK 加载） |
| 内存占用 | 每实例约 150MB（空闲态） | 按需分配 |
| 适用场景 | 高频任务分配、SLA 要求高 | 低频使用、资源紧张 |

### 3.4 Agent 实例复用策略

```go
type AgentPoolManager struct {
    mu          sync.RWMutex
    config      *PoolConfig
    active      map[string]*AgentInstance  // taskID → instance
    warmPool    []*AgentInstance            // 预热实例
    waitQueue   *redis.Client              // Redis Sorted Set

    bridge      pb.AgentBridgeClient
    worktreeMgr *WorktreeManager
    costTracker *CostTracker
    metrics     *PoolMetrics
}

// 获取 Agent 实例（优先从预热池取）
func (pm *AgentPoolManager) Acquire(ctx context.Context, task *Task) (*AgentInstance, error) {
    pm.mu.Lock()
    defer pm.mu.Unlock()

    // 1. 检查活跃池是否已满
    if len(pm.active) >= pm.config.MaxActive {
        // 加入等待队列
        err := pm.enqueue(ctx, task)
        if err != nil {
            return nil, fmt.Errorf("等待队列已满: %w", err)
        }
        return nil, ErrQueued // 调用方监听 Redis channel 等待调度
    }

    // 2. 尝试从预热池获取
    if len(pm.warmPool) > 0 {
        inst := pm.warmPool[len(pm.warmPool)-1]
        pm.warmPool = pm.warmPool[:len(pm.warmPool)-1]
        inst.TaskID = task.ID
        inst.Status = AgentStarting
        pm.active[task.ID] = inst

        pm.metrics.WarmPoolHit.Inc()
        go pm.replenishWarmPool() // 异步补充预热池
        return inst, nil
    }

    // 3. 冷启动新实例
    pm.metrics.ColdStart.Inc()
    inst, err := pm.createInstance(ctx, task)
    if err != nil {
        return nil, err
    }
    pm.active[task.ID] = inst
    return inst, nil
}

// 释放 Agent 实例
func (pm *AgentPoolManager) Release(taskID string) {
    pm.mu.Lock()
    inst, exists := pm.active[taskID]
    if exists {
        delete(pm.active, taskID)
    }
    pm.mu.Unlock()

    if !exists {
        return
    }

    // 清理 Agent 状态
    inst.Reset()

    // 如果预热池未满，回收到预热池（复用 Bridge 连接）
    pm.mu.Lock()
    if len(pm.warmPool) < pm.config.WarmPoolSize {
        pm.warmPool = append(pm.warmPool, inst)
        pm.mu.Unlock()
    } else {
        pm.mu.Unlock()
        inst.Destroy() // 销毁多余实例
    }

    // 检查等待队列，调度下一个任务
    go pm.scheduleNext()
}

// 定期清理空闲过久的预热池实例
func (pm *AgentPoolManager) cleanupWarmPool() {
    ticker := time.NewTicker(pm.config.WarmPoolRefreshInterval)
    defer ticker.Stop()

    for range ticker.C {
        pm.mu.Lock()
        now := time.Now()
        alive := pm.warmPool[:0]
        for _, inst := range pm.warmPool {
            if now.Sub(inst.LastUsed) > pm.config.WarmPoolRefreshInterval*2 {
                go inst.Destroy()
            } else {
                alive = append(alive, inst)
            }
        }
        pm.warmPool = alive
        pm.mu.Unlock()
    }
}
```

### 3.5 资源限制

每个 Agent 实例的资源边界通过 cgroup（Linux）或 Docker 资源限制实现：

```
单 Agent 资源包络:
┌─────────────────────────────────────┐
│  Memory:  512MB (hard limit)        │
│  CPU:     1 core (shares: 1024)     │
│  Disk IO: 100 weight (normal)       │
│  PIDs:    256 max                   │
│  Network: 白名单出站                │
│  Timeout: 30 min (execution)        │
│           5 min  (idle)             │
└─────────────────────────────────────┘

20 Agent 总资源需求:
  Memory:  ~10 GB
  CPU:     ~8 cores (I/O密集，实际远低于 20 cores)
  Disk:    ~20 GB (worktrees, 取决于仓库大小)
```

---

## 4. 任务到 Agent 分配流程

### 4.1 完整链路序列图

```
  User          IM Bridge       Task Service     Orchestrator     Agent Pool      Worktree Mgr    Bridge(TS)      Claude API
   │               │               │               │               │               │               │               │
   │  "修复 #42"   │               │               │               │               │               │               │
   │──────────────→│               │               │               │               │               │               │
   │               │  创建任务      │               │               │               │               │               │
   │               │──────────────→│               │               │               │               │               │
   │               │               │  保存到 PG     │               │               │               │               │
   │               │               │──┐             │               │               │               │               │
   │               │               │←─┘             │               │               │               │               │
   │               │               │               │               │               │               │               │
   │               │               │  分配给 Agent  │               │               │               │               │
   │               │               │──────────────→│               │               │               │               │
   │               │               │               │               │               │               │               │
   │               │               │               │  Acquire()    │               │               │               │
   │               │               │               │──────────────→│               │               │               │
   │               │               │               │               │               │               │               │
   │               │               │               │               │  Create()     │               │               │
   │               │               │               │               │──────────────→│               │               │
   │               │               │               │               │               │──┐ go-git:     │               │
   │               │               │               │               │               │  │ checkout    │               │
   │               │               │               │               │               │  │ branch      │               │
   │               │               │               │               │               │←─┘             │               │
   │               │               │               │               │               │               │               │
   │               │               │               │               │←── worktree OK │               │               │
   │               │               │               │               │               │               │               │
   │               │               │               │←── Agent 实例  │               │               │               │
   │               │               │               │               │               │               │               │
   │               │               │               │  gRPC Execute()                │               │               │
   │               │               │               │──────────────────────────────→│               │               │
   │               │               │               │               │               │               │               │
   │               │               │               │               │               │  query()      │               │
   │               │               │               │               │               │──────────────→│               │
   │               │               │               │               │               │               │  Messages API │
   │               │               │               │               │               │               │──────────────→│
   │               │               │               │               │               │               │←─ stream ─────│
   │               │               │               │               │               │               │               │
   │               │               │               │  ←── AgentEvent stream ────────│               │               │
   │               │               │               │               │               │               │               │
   │               │               │               │──→ WebSocket ──→ Frontend      │               │               │
   │               │               │               │──→ Redis Streams               │               │               │
   │               │               │               │──→ CostTracker                 │               │               │
   │               │               │               │               │               │               │               │
   │  实时进度推送  │               │               │               │               │               │               │
   │←─────────────│               │               │               │               │               │               │
   │               │               │               │               │               │               │               │
   │               │               │               │  ... Agent 编码完成 ...         │               │               │
   │               │               │               │               │               │               │               │
   │               │               │               │  StatusChange: completed        │               │               │
   │               │               │               │←──────────────────────────────│               │               │
   │               │               │               │               │               │               │               │
   │               │               │               │  创建 PR (go-git / gh CLI)     │               │               │
   │               │               │               │──┐             │               │               │               │
   │               │               │               │←─┘             │               │               │               │
   │               │               │               │               │               │               │               │
   │               │               │  更新任务状态   │               │               │               │               │
   │               │               │←──────────────│               │               │               │               │
   │               │               │               │  Release()    │               │               │               │
   │               │               │               │──────────────→│               │               │               │
   │               │               │               │               │  Cleanup()    │               │               │
   │               │               │               │               │──────────────→│               │               │
   │               │               │               │               │               │               │               │
   │  "PR #87 已就绪"             │               │               │               │               │               │
   │←─────────────│               │               │               │               │               │               │
```

### 4.2 分配决策逻辑

```go
func (o *Orchestrator) AssignToAgent(ctx context.Context, task *Task) error {
    // 1. 预算校验
    if task.BudgetUsd <= 0 {
        task.BudgetUsd = o.defaultBudget(task)
    }

    // 2. Sprint 预算检查
    if task.SprintID != "" {
        sprint, _ := o.sprintRepo.Get(ctx, task.SprintID)
        if sprint.SpentUsd >= sprint.TotalBudgetUsd {
            return ErrSprintBudgetExhausted
        }
    }

    // 3. 获取 Agent 实例
    inst, err := o.pool.Acquire(ctx, task)
    if errors.Is(err, ErrQueued) {
        task.Status = TaskStatusQueued
        o.taskRepo.Update(ctx, task)
        o.notify(task, "任务已进入队列，当前有 %d 个 Agent 在工作",
            o.pool.ActiveCount())
        return nil
    }
    if err != nil {
        return fmt.Errorf("获取 Agent 失败: %w", err)
    }

    // 4. 创建 worktree
    wt, err := o.worktreeMgr.Create(ctx, task.ID, task.Project.RepoBranch)
    if err != nil {
        o.pool.Release(task.ID)
        return fmt.Errorf("创建 worktree 失败: %w", err)
    }

    // 5. 构建执行上下文
    prompt := o.buildAgentPrompt(task)

    // 6. 启动执行（异步）
    go o.executeAndMonitor(ctx, inst, task, wt, prompt)

    // 7. 更新任务状态
    task.Status = TaskStatusInProgress
    task.AgentBranch = wt.Branch
    task.AgentWorktree = wt.Path
    o.taskRepo.Update(ctx, task)

    return nil
}
```

---

## 5. Git Worktree 管理

### 5.1 Worktree 设计

每个 Agent 任务在独立的 Git worktree 中执行，避免并发冲突：

```
/data/repos/<project-slug>/                  ← 主仓库 (bare clone)
    .git/
    ...

/data/worktrees/<project-slug>/
    <task-id-1>/                             ← Agent 1 的工作区
        src/
        tests/
        ...
    <task-id-2>/                             ← Agent 2 的工作区
        src/
        tests/
        ...
```

### 5.2 分支命名规范

```
格式:   agent/<task-id>
示例:   agent/550e8400-e29b
长格式: agent/550e8400-e29b-41d4-a716-446655440000

PR 标题格式: [Agent] <task-title>
Commit 格式: <type>: <description> [agent/<task-id>]
```

### 5.3 并行 Worktree 上限

```
默认上限: 20 (与 Agent 池 maxActive 一致)

资源估算:
  小型仓库 (< 100MB):   20 worktrees ≈  2 GB 磁盘
  中型仓库 (100MB-1GB):  20 worktrees ≈ 20 GB 磁盘
  大型仓库 (> 1GB):      10 worktrees ≈ 10 GB 磁盘 (浅克隆)

动态调整: 根据磁盘可用空间自动降低上限
```

### 5.4 go-git 代码示例

```go
package worktree

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "time"

    "github.com/go-git/go-git/v5"
    "github.com/go-git/go-git/v5/config"
    "github.com/go-git/go-git/v5/plumbing"
    "github.com/go-git/go-git/v5/plumbing/transport/http"
)

type WorktreeManager struct {
    mu           sync.Mutex
    repoPath     string                     // 主仓库路径
    worktreeBase string                     // worktree 存放根目录
    repo         *git.Repository
    worktrees    map[string]*WorktreeInfo   // taskID → info
    maxActive    int
    semaphore    chan struct{}               // 并发创建控制
    auth         *http.BasicAuth
}

type WorktreeInfo struct {
    TaskID    string
    Branch    string
    Path      string
    CreatedAt time.Time
    Status    string // "creating" | "active" | "cleaning"
}

func NewWorktreeManager(
    repoURL, repoPath, worktreeBase string,
    maxActive int, token string,
) (*WorktreeManager, error) {
    // Clone 或打开已有仓库
    repo, err := git.PlainOpen(repoPath)
    if err != nil {
        repo, err = git.PlainClone(repoPath, false, &git.CloneOptions{
            URL: repoURL,
            Auth: &http.BasicAuth{
                Username: "x-access-token",
                Password: token,
            },
        })
        if err != nil {
            return nil, fmt.Errorf("克隆仓库失败: %w", err)
        }
    }

    return &WorktreeManager{
        repoPath:     repoPath,
        worktreeBase: worktreeBase,
        repo:         repo,
        worktrees:    make(map[string]*WorktreeInfo),
        maxActive:    maxActive,
        semaphore:    make(chan struct{}, 3), // 最多 3 个并发创建
        auth: &http.BasicAuth{
            Username: "x-access-token", Password: token,
        },
    }, nil
}

// Create 创建新的 worktree 用于 Agent 任务
func (wm *WorktreeManager) Create(
    ctx context.Context, taskID, baseBranch string,
) (*WorktreeInfo, error) {
    // 获取信号量（限制并发创建数）
    select {
    case wm.semaphore <- struct{}{}:
        defer func() { <-wm.semaphore }()
    case <-ctx.Done():
        return nil, ctx.Err()
    }

    wm.mu.Lock()
    defer wm.mu.Unlock()

    // 检查上限
    if len(wm.worktrees) >= wm.maxActive {
        return nil, fmt.Errorf("已达 worktree 上限 (%d)", wm.maxActive)
    }

    // 拉取最新代码
    wt, err := wm.repo.Worktree()
    if err != nil {
        return nil, err
    }
    err = wt.Pull(&git.PullOptions{
        RemoteName: "origin",
        Auth:       wm.auth,
    })
    if err != nil && err != git.NoErrAlreadyUpToDate {
        return nil, fmt.Errorf("拉取最新代码失败: %w", err)
    }

    // 创建分支: agent/<task-id>
    branchName := fmt.Sprintf("agent/%s", taskID)
    branchRef := plumbing.NewBranchReferenceName(branchName)

    // 基于 baseBranch 创建新分支
    baseRef, err := wm.repo.Reference(
        plumbing.NewBranchReferenceName(baseBranch), true,
    )
    if err != nil {
        return nil, fmt.Errorf("找不到基础分支 %s: %w", baseBranch, err)
    }

    ref := plumbing.NewHashReference(branchRef, baseRef.Hash())
    err = wm.repo.Storer.SetReference(ref)
    if err != nil {
        return nil, fmt.Errorf("创建分支失败: %w", err)
    }

    // 创建 worktree 目录
    wtPath := filepath.Join(wm.worktreeBase, taskID)
    err = os.MkdirAll(wtPath, 0755)
    if err != nil {
        return nil, fmt.Errorf("创建 worktree 目录失败: %w", err)
    }

    // 使用 git worktree add
    // (go-git 对 worktree 支持有限，此处调用 git CLI)
    err = execGitWorktreeAdd(wm.repoPath, wtPath, branchName)
    if err != nil {
        os.RemoveAll(wtPath)
        return nil, fmt.Errorf("git worktree add 失败: %w", err)
    }

    info := &WorktreeInfo{
        TaskID:    taskID,
        Branch:    branchName,
        Path:      wtPath,
        CreatedAt: time.Now(),
        Status:    "active",
    }
    wm.worktrees[taskID] = info

    return info, nil
}

// Cleanup 清理指定任务的 worktree 和分支
func (wm *WorktreeManager) Cleanup(
    ctx context.Context, taskID string, deleteBranch bool,
) error {
    wm.mu.Lock()
    info, exists := wm.worktrees[taskID]
    if !exists {
        wm.mu.Unlock()
        return nil
    }
    info.Status = "cleaning"
    wm.mu.Unlock()

    // 1. 删除 worktree 目录
    if err := os.RemoveAll(info.Path); err != nil {
        return fmt.Errorf("删除 worktree 目录失败: %w", err)
    }

    // 2. 清理 git worktree 元数据
    _ = execGitWorktreePrune(wm.repoPath)

    // 3. 如果需要，删除远程分支（PR 已合并或任务取消）
    if deleteBranch {
        refName := plumbing.NewBranchReferenceName(info.Branch)
        _ = wm.repo.Storer.RemoveReference(refName)

        // 删除远程分支
        _ = wm.repo.Push(&git.PushOptions{
            RemoteName: "origin",
            RefSpecs: []config.RefSpec{
                config.RefSpec(":" + string(refName)),
            },
            Auth: wm.auth,
        })
    }

    wm.mu.Lock()
    delete(wm.worktrees, taskID)
    wm.mu.Unlock()

    return nil
}

// GarbageCollect 定时清理僵尸 worktree
func (wm *WorktreeManager) GarbageCollect(maxAge time.Duration) {
    wm.mu.Lock()
    defer wm.mu.Unlock()

    now := time.Now()
    for taskID, info := range wm.worktrees {
        if now.Sub(info.CreatedAt) > maxAge && info.Status == "active" {
            fmt.Printf("GC: 清理超龄 worktree task=%s age=%v\n",
                taskID, now.Sub(info.CreatedAt))
            go wm.Cleanup(context.Background(), taskID, false)
        }
    }
}
```

---

## 6. 成本控制实现

### 6.1 三层预算架构

```
┌────────────────────────────────────────────────────────────────────┐
│  Layer 3: 项目月度预算 (软限)                                       │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  budget: $2000/月                                            │  │
│  │  90% → 管理层通知   100% → 暂停所有 Agent                    │  │
│  │                                                              │  │
│  │  Layer 2: Sprint 预算 (软限)                                  │  │
│  │  ┌──────────────────────────────────────────────────────┐    │  │
│  │  │  budget: $500/sprint                                  │    │  │
│  │  │  80% → 通知技术负责人   100% → 新任务需审批            │    │  │
│  │  │                                                      │    │  │
│  │  │  Layer 1: 任务预算 (硬限)                              │    │  │
│  │  │  ┌──────────────────────────────────────────────┐    │    │  │
│  │  │  │  budget: $5.00 (默认, 可调)                   │    │    │  │
│  │  │  │  80% → 告警   100% → 立即中断 Agent           │    │    │  │
│  │  │  └──────────────────────────────────────────────┘    │    │  │
│  │  └──────────────────────────────────────────────────────┘    │  │
│  └──────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────┘
```

### 6.2 实时 Token 计数

Token 计数来自两个来源，按优先级选择：

1. **Claude API 响应中的 `usage` 字段**（精确值，优先采用）
2. **TS Bridge 侧的 tokenizer 估算**（作为 API usage 缺失时的回退）

```go
type CostTracker struct {
    mu sync.RWMutex

    pricing      map[string]*ModelPricing // "claude-sonnet-4" → pricing
    taskCosts    map[string]*TaskCost
    sprintCosts  map[string]*SprintCost
    projectCosts map[string]*ProjectCost

    redis  *redis.Client
    db     *gorm.DB
    notify func(event CostEvent)
}

type ModelPricing struct {
    Model            string
    InputPerMTok     float64 // $/百万 input tokens
    OutputPerMTok    float64 // $/百万 output tokens
    CacheReadPerMTok float64 // $/百万 cache read tokens
}

// 默认定价表 (2026-03)
var DefaultPricing = map[string]*ModelPricing{
    "claude-sonnet-4": {
        Model: "claude-sonnet-4",
        InputPerMTok: 3.0, OutputPerMTok: 15.0, CacheReadPerMTok: 0.30,
    },
    "claude-haiku-4": {
        Model: "claude-haiku-4",
        InputPerMTok: 0.80, OutputPerMTok: 4.0, CacheReadPerMTok: 0.08,
    },
    "claude-opus-4": {
        Model: "claude-opus-4",
        InputPerMTok: 15.0, OutputPerMTok: 75.0, CacheReadPerMTok: 1.50,
    },
}
```

### 6.3 maxBudgetUsd 执行流程

```go
// RecordUsage 是成本控制的核心方法，每次 CostUpdate 事件触发
func (ct *CostTracker) RecordUsage(
    taskID string, update *pb.CostUpdate,
) (CostAction, error) {
    ct.mu.Lock()
    defer ct.mu.Unlock()

    tc := ct.taskCosts[taskID]
    if tc == nil {
        return CostActionNone, ErrTaskNotTracked
    }

    // 1. 累加
    tc.InputTokens += update.InputTokens
    tc.OutputTokens += update.OutputTokens
    tc.CacheReadTokens += update.CacheReadTokens
    tc.SpentUsd += update.CostUsd
    tc.CallCount++

    // 2. 任务级预算检查 (硬限)
    ratio := tc.SpentUsd / tc.BudgetUsd

    if ratio >= 1.0 {
        go ct.notify(CostEvent{
            Type:   CostEventBudgetExceeded,
            TaskID: taskID,
            Spent:  tc.SpentUsd,
            Budget: tc.BudgetUsd,
        })
        return CostActionKill, nil
    }

    if ratio >= 0.8 && !tc.AlertSent80 {
        tc.AlertSent80 = true
        go ct.notify(CostEvent{
            Type:   CostEventBudgetWarning,
            TaskID: taskID,
            Spent:  tc.SpentUsd,
            Budget: tc.BudgetUsd,
            Message: fmt.Sprintf(
                "任务预算已用 %.0f%% ($%.2f/$%.2f)",
                ratio*100, tc.SpentUsd, tc.BudgetUsd,
            ),
        })
    }

    // 3. 级联更新 Sprint 和 Project 成本
    ct.cascadeUpdate(tc, update.CostUsd)

    // 4. 异步同步到 Redis（跨节点可见）
    go ct.syncToRedis(taskID, tc)

    return CostActionContinue, nil
}
```

### 6.4 超预算处理

```go
func (o *Orchestrator) handleBudgetExceeded(taskID string) {
    // 1. 请求 Bridge 保存会话快照后取消 (< 100ms)
    o.bridge.Cancel(context.Background(), &pb.CancelRequest{
        TaskId: taskID,
        Reason: "budget_exceeded",
    })

    // 2. 更新任务状态
    o.taskRepo.UpdateStatus(context.Background(), taskID, TaskStatusBudgetExceeded)

    // 3. WebSocket 通知前端
    o.wsHub.Broadcast("task:"+taskID, map[string]any{
        "type":   "agent.budget_exceeded",
        "taskId": taskID,
    })

    // 4. IM 通知任务分配者
    task, _ := o.taskRepo.Get(context.Background(), taskID)
    o.imBridge.Send(task.ReporterID, fmt.Sprintf(
        "任务「%s」的 Agent 预算已耗尽 ($%.2f/$%.2f)，需要增加预算或手动完成",
        task.Title, task.SpentUsd, task.BudgetUsd,
    ))
}
```

### 6.5 成本回调钩子

```go
type CostCallback func(event CostEvent)

type CostEvent struct {
    Type    CostEventType
    TaskID  string
    Spent   float64
    Budget  float64
    Message string
}

type CostEventType string

const (
    CostEventBudgetWarning  CostEventType = "budget_warning"   // 80%
    CostEventBudgetExceeded CostEventType = "budget_exceeded"  // 100%
    CostEventSprintWarning  CostEventType = "sprint_warning"   // Sprint 80%
    CostEventProjectWarning CostEventType = "project_warning"  // Project 90%
)
```

---

## 7. 会话管理

### 7.1 会话持久化——暂停/恢复

Agent 的会话状态通过 `SessionSnapshot` 机制实现跨重启持久化：

```
Agent 运行中
    │
    ├── 定期自动快照 (每 5 分钟)
    │   Bridge → AgentEvent.SessionSnapshot → Go → Redis + PG
    │
    ├── 主动暂停 (用户触发)
    │   Go → gRPC PauseTask → Bridge → 保存会话 → Snapshot → Go
    │
    └── 崩溃快照 (异常触发)
        Bridge SIGTERM → 优雅停机 → 最后一次 Snapshot → 退出
```

### 7.2 Redis 会话存储

```go
// 会话存储键设计
const (
    // 活跃会话状态（低延迟读取）
    keySessionState   = "session:%s:state"    // Hash
    // 会话快照（恢复用）
    keySessionSnap    = "session:%s:snapshot"  // String (JSON)
    // 会话历史（审计用）
    keySessionHistory = "session:%s:history"   // List
)

type SessionStore struct {
    redis *redis.Client
    db    *gorm.DB // PostgreSQL 持久化
}

// Save 保存会话快照（双写 Redis + PostgreSQL）
func (ss *SessionStore) Save(
    ctx context.Context, taskID string, snap *SessionSnapshot,
) error {
    snapJSON, err := json.Marshal(snap)
    if err != nil {
        return err
    }

    // 1. 写 Redis（低延迟，用于快速恢复）
    pipe := ss.redis.Pipeline()
    pipe.Set(ctx,
        fmt.Sprintf(keySessionSnap, taskID),
        snapJSON, 24*time.Hour,
    )
    pipe.HSet(ctx,
        fmt.Sprintf(keySessionState, taskID),
        map[string]interface{}{
            "turn_number": snap.TurnNumber,
            "spent_usd":   snap.SpentUsd,
            "updated_at":  time.Now().UnixMilli(),
        },
    )
    pipe.LPush(ctx,
        fmt.Sprintf(keySessionHistory, taskID), snapJSON,
    )
    pipe.LTrim(ctx,
        fmt.Sprintf(keySessionHistory, taskID), 0, 9,
    ) // 保留最近 10 个快照
    _, err = pipe.Exec(ctx)
    if err != nil {
        return fmt.Errorf("Redis 写入失败: %w", err)
    }

    // 2. 异步写 PostgreSQL（持久化，崩溃后可恢复）
    go func() {
        ss.db.Create(&AgentSessionSnapshot{
            TaskID:       taskID,
            TurnNumber:   snap.TurnNumber,
            SpentUsd:     snap.SpentUsd,
            SnapshotData: string(snapJSON),
            CreatedAt:    time.Now(),
        })
    }()

    return nil
}

// Restore 恢复会话（优先 Redis，回退 PostgreSQL）
func (ss *SessionStore) Restore(
    ctx context.Context, taskID string,
) (*SessionSnapshot, error) {
    // 1. 尝试从 Redis 恢复
    snapJSON, err := ss.redis.Get(ctx,
        fmt.Sprintf(keySessionSnap, taskID),
    ).Result()
    if err == nil {
        var snap SessionSnapshot
        json.Unmarshal([]byte(snapJSON), &snap)
        return &snap, nil
    }

    // 2. 回退到 PostgreSQL
    var record AgentSessionSnapshot
    err = ss.db.Where("task_id = ?", taskID).
        Order("created_at DESC").First(&record).Error
    if err != nil {
        return nil, fmt.Errorf("未找到会话快照: %w", err)
    }

    var snap SessionSnapshot
    json.Unmarshal([]byte(record.SnapshotData), &snap)

    // 回填到 Redis
    ss.redis.Set(ctx,
        fmt.Sprintf(keySessionSnap, taskID),
        record.SnapshotData, 24*time.Hour,
    )

    return &snap, nil
}
```

### 7.3 崩溃后恢复流程

```
TS Bridge 进程崩溃/重启
    │
    ▼
Go Orchestrator 检测到 gRPC 连接断开
    │
    ├── 1. 标记所有活跃 Agent 为 "recovering"
    ├── 2. 等待 Bridge 重启（健康检查轮询，每 5s）
    │       最多等待 60s，超时则告警
    │
    ▼
Bridge 恢复就绪 (HealthCheck → SERVING)
    │
    ▼
Go Orchestrator 逐个恢复 Agent
    │
    ├── 对每个 "recovering" 状态的 Agent:
    │   ├── 1. 从 Redis 加载最新 SessionSnapshot
    │   ├── 2. 检查 worktree 是否完整
    │   ├── 3. 发送 gRPC ResumeTask (带 snapshot)
    │   ├── 4. Bridge 用 snapshot 恢复 Agent SDK 会话
    │   └── 5. 更新状态为 "running"
    │
    └── 恢复失败的 Agent:
        ├── 重试计数 < 2 → 从头重试（复用 worktree）
        └── 重试计数 >= 2 → 标记为 "failed"，通知人工
```

---

## 8. 多 Agent 模式（P2）

### 8.1 角色定义

P2 阶段引入 Planner/Coder/Reviewer 三角色协作模式：

```
┌─────────────────────────────────────────────────────────────────┐
│                    Multi-Agent Orchestration                     │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    Planner Agent                           │  │
│  │  · 分析需求，分解子任务                                    │  │
│  │  · 为每个子任务选择最优 Coder                              │  │
│  │  · 监督进度，处理阻塞                                      │  │
│  │  · 模型: Claude Sonnet 4 (成本均衡)                        │  │
│  └───────────────────┬───────────────────────────────────────┘  │
│                      │ 委派任务                                  │
│          ┌───────────┼───────────┐                              │
│          ▼           ▼           ▼                              │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                       │
│  │ Coder A  │ │ Coder B  │ │ Coder C  │                       │
│  │          │ │          │ │          │                       │
│  │ 独立     │ │ 独立     │ │ 独立     │                       │
│  │ worktree │ │ worktree │ │ worktree │                       │
│  │          │ │          │ │          │                       │
│  │ Sonnet 4 │ │ Sonnet 4 │ │ Haiku 4  │ ← 简单任务用轻量模型  │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘                       │
│       │            │            │                              │
│       └────────────┼────────────┘                              │
│                    │ 提交 PR                                    │
│                    ▼                                            │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    Reviewer Agent                          │  │
│  │  · 审查代码质量、安全性、规范合规                           │  │
│  │  · 交叉验证减少假阳性                                      │  │
│  │  · 通过 → 合并；不通过 → 反馈给 Coder                      │  │
│  │  · 模型: Claude Sonnet 4                                   │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 8.2 Agent 间通信

Agent 间通过 Go Orchestrator 中转消息，不直接通信：

```
Planner                 Orchestrator              Coder A
   │                        │                        │
   │  委派子任务 #1          │                        │
   │  (通过 tool_call)      │                        │
   │───────────────────────→│                        │
   │                        │  Spawn Coder A         │
   │                        │───────────────────────→│
   │                        │                        │
   │                        │  ... Coder 编码中 ...   │
   │                        │                        │
   │                        │  Coder 完成，PR 已创建  │
   │                        │←───────────────────────│
   │                        │                        │
   │  收到子任务 #1 完成通知  │                        │
   │←───────────────────────│                        │
   │                        │                        │
   │  触发 Review            │                        │
   │───────────────────────→│                        │
   │                        │  Spawn Reviewer         │
   │                        │───────────────────────→ Reviewer
```

### 8.3 任务委派流程

```go
type MultiAgentOrchestrator struct {
    pool     *AgentPoolManager
    taskRepo TaskRepository
    bridge   pb.AgentBridgeClient
}

// PlannerDelegate 处理 Planner Agent 的委派请求
func (mao *MultiAgentOrchestrator) PlannerDelegate(
    ctx context.Context,
    parentTaskID string,
    subtask *SubtaskSpec,
) error {
    // 1. 创建子任务
    childTask := &Task{
        ParentID:     parentTaskID,
        Title:        subtask.Title,
        Description:  subtask.Description,
        BudgetUsd:    subtask.EstimatedBudget,
        Labels:       subtask.Labels,
        AssigneeType: "agent",
    }
    if err := mao.taskRepo.Create(ctx, childTask); err != nil {
        return err
    }

    // 2. 选择角色: coder 或 reviewer
    switch subtask.Role {
    case "coder":
        return mao.assignToCoder(ctx, childTask)
    case "reviewer":
        return mao.assignToReviewer(ctx, childTask)
    default:
        return mao.assignToCoder(ctx, childTask)
    }
}
```

### 8.4 Planner 工具注入

Planner Agent 通过自定义 MCP 工具与 Orchestrator 交互：

```typescript
// Planner 专用工具定义
const plannerTools = [
  {
    name: "delegate_task",
    description: "将子任务委派给 Coder Agent 执行",
    inputSchema: {
      type: "object",
      properties: {
        title: { type: "string", description: "子任务标题" },
        description: { type: "string", description: "详细描述和验收标准" },
        role: { type: "string", enum: ["coder", "reviewer"] },
        complexity: { type: "string", enum: ["low", "medium", "high"] },
        files: { type: "array", items: { type: "string" } },
        dependencies: { type: "array", items: { type: "string" } },
      },
      required: ["title", "description", "role"],
    },
  },
  {
    name: "check_subtask_status",
    description: "查询已委派子任务的执行状态",
    inputSchema: {
      type: "object",
      properties: {
        subtaskId: { type: "string" },
      },
      required: ["subtaskId"],
    },
  },
  {
    name: "request_review",
    description: "请求 Reviewer Agent 审查指定 PR",
    inputSchema: {
      type: "object",
      properties: {
        prUrl: { type: "string" },
        focusAreas: { type: "array", items: { type: "string" } },
      },
      required: ["prUrl"],
    },
  },
];
```

---

## 9. 竞品编排对比

### 9.1 对比总览表

| 维度 | AgentForge | Composio Orchestrator | OpenSwarm | SWE-agent (Open SWE) |
|------|-----------|----------------------|-----------|---------------------|
| **架构语言** | Go + TypeScript | TypeScript | TypeScript | Python (LangGraph) |
| **Agent 运行时** | Claude Agent SDK (TS Bridge) | 可插拔 (Claude Code/Codex/Aider) | Claude Code CLI | LangGraph + Daytona |
| **隔离方式** | Git worktree (go-git) | Git worktree (tmux/Docker) | Claude Code 内置 | Daytona 沙箱 VM |
| **任务管理** | 内建看板 + AI 分解 | 外部 (GitHub Issues/Linear) | Linear 集成 | GitHub Issues |
| **IM 集成** | 10+ 平台 (cc-connect) | Slack/Desktop 通知 | Discord 命令 | Slack 线程 |
| **成本控制** | 三层预算 + 实时追踪 | 基础追踪 | 无 | 无 |
| **会话恢复** | 快照持久化 + Redis | 无（重启重来） | 无 | Temporal 持久执行 |
| **审查流水线** | 三层 (CI+Review Agent+人工) | CI 自动修复 | Worker/Reviewer 配对 | 基础 |
| **多 Agent 协作** | Planner/Coder/Reviewer (P2) | Orchestrator Agent 统一调度 | Worker/Reviewer | Manager/Planner/Programmer |
| **可观测性** | Prometheus + Grafana + OTel | SSE Dashboard | 实时 Dashboard | LangSmith |
| **部署方式** | Docker Compose → K8s | npm 全局安装 (CLI) | Node.js 服务 | Docker + Daytona |

### 9.2 关键借鉴点

**从 Composio Agent Orchestrator 借鉴：**

- **8 插槽插件架构**：Runtime/Agent/Workspace/Tracker/SCM/Notifier/Terminal/Lifecycle 可插拔。AgentForge 应在 Agent Provider 和 Notifier 维度实现类似的插件化。
- **Reaction 系统**：CI 失败自动路由回 Agent 修复。AgentForge 的审查流水线应实现类似的自动化反馈循环。
- **Convention over Configuration**：自动推导路径、分支前缀。AgentForge 的 worktree 管理可借鉴其基于 hash 的命名空间方案。

**从 OpenSwarm 借鉴：**

- **Worker/Reviewer 配对流水线**：每个编码任务自带审查配对，比独立审查流水线更紧耦合，适合快速迭代。
- **LanceDB 长期记忆**：跨 session 的向量化记忆。AgentForge P2 的 Agent 记忆可参考。
- **Discord 命令界面**：验证了 IM 驱动 Agent 的可行性。

**从 SWE-agent (Open SWE) 借鉴：**

- **Manager/Planner/Programmer 分层**：加入了 Manager 角色负责更高层决策。
- **Daytona 沙箱**：90ms VM 创建，比 worktree 更强的隔离性。AgentForge Phase 3 可考虑引入。
- **企业级模式 (Stripe/Ramp/Coinbase)**：验证了大规模编码 Agent 在生产环境的可行性。

### 9.3 AgentForge 差异化优势

```
┌─────────────────────────────────────────────────────────────────┐
│  竞品普遍缺失，AgentForge 独有的能力：                            │
│                                                                 │
│  1. 全链路 IM → 任务 → 编码 → 审查 → 通知                       │
│     (竞品通常只覆盖编码环节)                                      │
│                                                                 │
│  2. 中国 IM 原生支持 (飞书/钉钉/企微/QQ)                          │
│     (竞品仅支持 Slack/Discord)                                   │
│                                                                 │
│  3. 三层成本控制 + 实时追踪                                       │
│     (竞品最多有基础追踪，无预算执行)                               │
│                                                                 │
│  4. 人机混合看板 (碳基 + 硅基统一管理)                             │
│     (竞品将 Agent 视为工具而非团队成员)                            │
│                                                                 │
│  5. 会话持久化 + 崩溃恢复                                         │
│     (竞品普遍不支持会话恢复)                                      │
└─────────────────────────────────────────────────────────────────┘
```

---

## 10. 可观测性

### 10.1 Agent 执行日志

所有 Agent 事件持久化到 PostgreSQL `agent_runs` 和 `agent_events` 表，同时写入结构化日志：

```go
type AgentLog struct {
    Timestamp  time.Time         `json:"ts"`
    Level      string            `json:"level"`
    TaskID     string            `json:"task_id"`
    SessionID  string            `json:"session_id"`
    EventType  string            `json:"event_type"`
    Content    string            `json:"content,omitempty"`
    ToolName   string            `json:"tool_name,omitempty"`
    TurnNumber int               `json:"turn,omitempty"`
    CostUsd    float64           `json:"cost_usd,omitempty"`
    Duration   time.Duration     `json:"duration,omitempty"`
    Error      string            `json:"error,omitempty"`
    Meta       map[string]string `json:"meta,omitempty"`
}
```

### 10.2 Prometheus 指标

```go
var (
    // === Agent 池指标 ===
    agentPoolActive = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "agentforge_agent_pool_active",
        Help: "当前活跃 Agent 数量",
    })
    agentPoolWarm = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "agentforge_agent_pool_warm",
        Help: "预热池中的 Agent 数量",
    })
    agentPoolQueueSize = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "agentforge_agent_pool_queue_size",
        Help: "等待队列中的任务数",
    })

    // === Agent 执行指标 ===
    agentExecutionDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "agentforge_agent_execution_duration_seconds",
            Help:    "Agent 任务执行时长",
            Buckets: []float64{30, 60, 120, 300, 600, 1200, 1800},
        }, []string{"status"},
    ) // status: completed | failed | cancelled

    agentTurnsTotal = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "agentforge_agent_turns_total",
            Help:    "Agent 对话轮次数",
            Buckets: []float64{1, 5, 10, 15, 20, 25, 30},
        }, []string{"status"},
    )

    // === 成本指标 ===
    agentCostUsd = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "agentforge_agent_cost_usd_total",
        Help: "Agent 累计花费（美元）",
    }, []string{"project", "model"})

    agentTokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "agentforge_agent_tokens_total",
        Help: "Agent 累计 token 使用量",
    }, []string{"project", "direction"})
    // direction: input | output | cache_read

    agentBudgetExceeded = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "agentforge_agent_budget_exceeded_total",
        Help: "预算超限次数",
    }, []string{"level"})
    // level: task | sprint | project

    // === Bridge 指标 ===
    bridgeGrpcLatency = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "agentforge_bridge_grpc_latency_seconds",
        Help:    "Go → TS Bridge gRPC 调用延迟",
        Buckets: prometheus.DefBuckets,
    })
    bridgeHealthStatus = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "agentforge_bridge_health_status",
        Help: "Bridge 健康状态 (1=SERVING, 0=NOT_SERVING)",
    })

    // === Worktree 指标 ===
    worktreeActive = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "agentforge_worktree_active",
        Help: "当前活跃 worktree 数量",
    })
    worktreeDiskUsageBytes = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "agentforge_worktree_disk_usage_bytes",
        Help: "所有 worktree 的总磁盘占用",
    })

    // === WebSocket 指标 ===
    wsConnections = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "agentforge_ws_connections",
        Help: "当前 WebSocket 连接数",
    })
    wsEventDropped = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "agentforge_ws_event_dropped_total",
        Help: "因背压丢弃的 WebSocket 事件数",
    }, []string{"task_id"})
)
```

### 10.3 分布式追踪

使用 OpenTelemetry 实现跨服务追踪，覆盖完整的任务执行链路：

```
Trace: task-execution-{taskID}
│
├── Span: task.assign (Go API)
│   └── Span: pool.acquire (Agent Pool Manager)
│       └── Span: worktree.create (Worktree Manager)
│
├── Span: bridge.execute (gRPC to TS Bridge)
│   ├── Span: sdk.query (Claude Agent SDK)
│   │   ├── Span: llm.call.1 (Claude API)
│   │   ├── Span: tool.read (Read file)
│   │   ├── Span: llm.call.2 (Claude API)
│   │   ├── Span: tool.edit (Edit file)
│   │   ├── Span: llm.call.3 (Claude API)
│   │   ├── Span: tool.bash (Run tests)
│   │   └── Span: llm.call.4 (Claude API)
│   │
│   └── Span: cost.record (CostTracker)
│
├── Span: git.commit (Worktree)
├── Span: git.push (Remote)
├── Span: pr.create (GitHub API)
│
├── Span: review.trigger (Review Pipeline)
│   ├── Span: review.ci (GitHub Actions)
│   └── Span: review.agent (Review Agent)
│
└── Span: notify (IM Bridge)
    └── Span: feishu.send (飞书 API)
```

### 10.4 Grafana Dashboard 设计

```
┌───────────────────────────────────────────────────────────────────────────┐
│                     AgentForge Orchestration Dashboard                     │
├───────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│  ┌─ Agent Pool Status ────────────────┐  ┌─ Cost Overview ──────────────┐ │
│  │                                    │  │                              │ │
│  │  Active:  12/20  [========..]      │  │  Today:     $47.23          │ │
│  │  Warm:    2/3    [======....]      │  │  This Week: $312.85        │ │
│  │  Queued:  3      [===.......]      │  │  Sprint:    $892/$1500     │ │
│  │                                    │  │  [=========....]  59%      │ │
│  │  Warm Hit Rate: 72%                │  │                              │ │
│  │  Avg Cold Start: 4.2s              │  │  Top Cost Tasks:            │ │
│  └────────────────────────────────────┘  │   #42 fix-auth    $8.30    │ │
│                                          │   #38 add-search  $6.15    │ │
│  ┌─ Execution Metrics ────────────────┐  │   #45 unit-tests  $3.20    │ │
│  │                                    │  └──────────────────────────────┘ │
│  │  Success Rate (24h):  87%          │                                   │
│  │  Avg Duration:        8.2 min      │  ┌─ Bridge Health ──────────────┐ │
│  │  Avg Turns:           12.5         │  │                              │ │
│  │  P95 Duration:        22 min       │  │  Status: SERVING            │ │
│  │                                    │  │  Active Agents: 12          │ │
│  │  Duration Distribution (24h):      │  │  gRPC Latency p99: 2.3ms   │ │
│  │   ..#####..                        │  │  Uptime: 3d 14h 22m        │ │
│  │   0  5  10  15  20  25 min         │  │  Last Restart: 3d ago       │ │
│  └────────────────────────────────────┘  └──────────────────────────────┘ │
│                                                                           │
│  ┌─ Token Usage (7d) ─────────────────────────────────────────────────┐   │
│  │                                                                    │   │
│  │  Input:   ========================  2.4M tokens                   │   │
│  │  Output:  ==========               1.1M tokens                    │   │
│  │  Cache:   ==============           1.6M tokens (saved $4.80)      │   │
│  │                                                                    │   │
│  │  Daily Token Usage:                                                │   │
│  │  500K |                     _                                      │   │
│  │  400K |              _     | |  _                                  │   │
│  │  300K |  _    _     | |   | | | |                                  │   │
│  │  200K | | |  | |    | |   | | | |                                  │   │
│  │  100K | | |  | |    | |   | | | |                                  │   │
│  │     0 +--+----+------+-----+---+--                                │   │
│  │        Mon  Tue  Wed  Thu  Fri  Sat  Sun                           │   │
│  └────────────────────────────────────────────────────────────────────┘   │
│                                                                           │
│  ┌─ Worktree Status ──────────────┐  ┌─ Active Agents ────────────────┐  │
│  │                                │  │                                │  │
│  │  Active:    12                 │  │  ID    Task       Status  Turn │  │
│  │  Disk Used: 8.3 GB             │  │  --- ----------- ------ ----- │  │
│  │  Disk Avail: 42 GB             │  │  #1  fix-auth    running  8   │  │
│  │  Zombie: 0                     │  │  #2  add-search  running  15  │  │
│  │  Last GC: 12 min ago           │  │  #3  unit-tests  running  3   │  │
│  │                                │  │  #4  refactor    paused   22  │  │
│  └────────────────────────────────┘  │  ...                          │  │
│                                      └────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────────────┘
```

### 10.5 告警规则

```yaml
# prometheus-alerts.yml
groups:
  - name: agentforge-orchestration
    rules:
      # Agent 池耗尽
      - alert: AgentPoolExhausted
        expr: agentforge_agent_pool_active / 20 > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Agent 池接近耗尽"

      # Bridge 不健康
      - alert: BridgeUnhealthy
        expr: agentforge_bridge_health_status == 0
        for: 30s
        labels:
          severity: critical
        annotations:
          summary: "Agent SDK Bridge 不可用"

      # Agent 执行成功率下降
      - alert: AgentSuccessRateLow
        expr: |
          rate(agentforge_agent_execution_duration_seconds_count{status="completed"}[1h])
          /
          rate(agentforge_agent_execution_duration_seconds_count[1h])
          < 0.7
        for: 30m
        labels:
          severity: warning
        annotations:
          summary: "Agent 成功率低于 70%"

      # 成本异常
      - alert: CostSpike
        expr: rate(agentforge_agent_cost_usd_total[1h]) > 50
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "成本异常增长 (>$50/h)"

      # Worktree 泄漏
      - alert: WorktreeLeak
        expr: agentforge_worktree_active > agentforge_agent_pool_active * 1.5
        for: 30m
        labels:
          severity: warning
        annotations:
          summary: "可能存在 worktree 泄漏"
```

---

## 附录 A：部署拓扑

```yaml
# docker-compose.yml (开发环境)
version: "3.8"
services:
  api:
    build: ./cmd/api
    ports: ["8080:8080"]
    depends_on:
      bridge:
        condition: service_healthy
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      - BRIDGE_GRPC_ADDR=bridge:50051
      - DATABASE_URL=postgres://agentforge:pass@postgres:5432/agentforge
      - REDIS_URL=redis://redis:6379
    volumes:
      - worktrees:/data/worktrees
      - repos:/data/repos

  bridge:
    build: ./services/agent-bridge
    ports: ["50051:50051"]
    healthcheck:
      test: ["CMD", "grpc_health_probe", "-addr=:50051"]
      interval: 5s
      retries: 3
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - MAX_CONCURRENT_AGENTS=10
    deploy:
      resources:
        limits:
          memory: 4G
          cpus: "4.0"
    volumes:
      - worktrees:/data/worktrees

  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: agentforge
      POSTGRES_USER: agentforge
      POSTGRES_PASSWORD: pass
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U agentforge"]
      interval: 5s

  redis:
    image: redis:7-alpine
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s

  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./deploy/prometheus.yml:/etc/prometheus/prometheus.yml
    ports: ["9090:9090"]

  grafana:
    image: grafana/grafana:latest
    ports: ["3001:3000"]
    volumes:
      - ./deploy/grafana/dashboards:/var/lib/grafana/dashboards

volumes:
  worktrees:
  repos:
  pgdata:
```

## 附录 B：配置参考

```yaml
# agentforge-config.yaml
orchestrator:
  agent_pool:
    max_active: 20
    warm_pool_size: 2
    max_queue_size: 100
    per_agent_memory_mb: 512
    idle_timeout: 5m
    max_execution_time: 30m

  worktree:
    base_path: /data/worktrees
    repo_path: /data/repos
    max_active: 20
    gc_interval: 1h
    gc_max_age: 2h

  cost:
    default_task_budget_usd: 5.00
    default_sprint_budget_usd: 500.00
    default_project_monthly_budget_usd: 2000.00
    alert_threshold_task: 0.80
    alert_threshold_sprint: 0.80
    alert_threshold_project: 0.90

  bridge:
    grpc_addr: bridge:50051
    health_check_interval: 5s
    max_reconnect_attempts: 10
    keepalive_time: 30s
    keepalive_timeout: 10s

  session:
    auto_snapshot_interval: 5m
    snapshot_retention_count: 10
    redis_ttl: 24h

  observability:
    prometheus_port: 9090
    log_level: info
    trace_sample_rate: 0.1
```

---

> **文档状态：** 初稿完成，待团队评审
>
> **关联文档：**
> - [PRD](../PRD.md)
> - [后端技术分析](./backend-tech-analysis.md)
> - [技术挑战全景分析](./TECHNICAL_CHALLENGES.md)
> - [审查流水线设计](./REVIEW_PIPELINE_DESIGN.md)
> - [数据架构与实时系统设计](./DATA_AND_REALTIME_DESIGN.md)
> - [cc-connect 复用指南](./CC_CONNECT_REUSE_GUIDE.md)
