# AgentForge 技术挑战全景分析

> 版本：v1.0 | 日期：2026-03-22
> 基于 PRD v1.0 及技术选型分析（Go 后端 + TypeScript Agent SDK Bridge）
>
> v1.2 live contract update: 当前 Go↔TS Bridge 实现以 HTTP 命令 + WebSocket 事件流为准，canonical route family 为 `/bridge/*`。本文中保留的 gRPC / proto 讨论仅作历史方案参考，不能视为当前实现入口。

---

## 当前实现快照（2026-03-29）

这份文档分析的是技术难点全景，但当前仓库里已经有几条明确的落地答案，不应再把它们写成悬而未决：

- Go↔TS Bridge 的 live contract 已经稳定为 HTTP `/bridge/*` 命令面 + WebSocket 事件流，兼容 alias 只是历史迁移层。
- TS Bridge 已有多 runtime catalog 与 continuity 语义，不再只是 Claude 单一路径；当前真实运行时包括 `claude_code`、`codex`、`opencode`。
- 实时事件广播当前由 Go 侧 WebSocket hub 承担 project-scoped fan-out，任务、审查、插件、调度和文档相关事件都走这条主链。
- 审查工作区、`pending_human` 状态、项目设置 runtime catalog、docs/wiki、以及桌面 window chrome 都已经有真实实现，因此后续章节中把这些面写成纯规划时，应优先服从当前代码真相。

---

## 目录

1. [Go + TypeScript Agent SDK Bridge 双栈通信](#1-go--typescript-agent-sdk-bridge-双栈通信)
2. [多 Agent 并发隔离](#2-多-agent-并发隔离)
3. [实时 WebSocket 架构](#3-实时-websocket-架构)
4. [Token 成本控制](#4-token-成本控制)
5. [Agent 生命周期管理](#5-agent-生命周期管理)
6. [AI 任务分解可靠性](#6-ai-任务分解可靠性)
7. [安全沙箱](#7-安全沙箱)
8. [IM 自然语言解析](#8-im-自然语言解析)

---

## 1. Go + TypeScript Agent SDK Bridge 双栈通信

### 1.1 问题描述（为什么难）

AgentForge 的核心矛盾：**Go 是最佳后端语言（goroutine 并发模型、go-git 纯 Go 实现），但 Claude Agent SDK 只有 TypeScript 版本**。这意味着系统必须运行两个异构运行时，并在它们之间建立高效、可靠的通信桥梁。

> **架构决策更新**：Bridge 不仅负责 Agent 编码执行，还是**所有后端 AI 调用的统一出口**（包括 AI 任务分解、IM 意图识别）。Go 侧不再依赖 LangChainGo，所有 LLM 调用统一经过 Bridge。这增加了 Bridge 的重要性，同时也意味着 Bridge 的通信可靠性和性能更加关键。

具体难点：

- **跨进程边界的流式数据传输**：Agent SDK 的 `query()` 产生的是持续数秒到数分钟的流式输出（token-by-token），不是简单的请求-响应。Go 后端需要实时中继这些流式数据到前端 WebSocket，任何中间缓冲都会增加延迟。
- **错误语义的跨语言映射**：TypeScript 的异常体系（Error/TypeError/RangeError）与 Go 的 `error` 接口截然不同。Agent SDK 的错误可能包含 rate limit 信息、token 用量、会话状态等结构化数据，需要完整传递到 Go 侧进行业务决策。
- **会话状态的跨进程共享**：Agent SDK 的 session resume 功能依赖内存中的会话状态。当 TS Bridge 进程重启时，这些状态必须能够恢复。
- **部署耦合**：Go 二进制和 Node.js 运行时的部署、健康检查、版本升级必须协调。任一侧宕机都会影响整个 Agent 执行流水线。
- **序列化开销**：Agent 输出包含代码片段、diff、日志等大文本。即使当前 live contract 已改为 HTTP + WebSocket，跨进程 JSON 传输与事件分发仍然需要关注峰值大小、分片和缓冲策略。

### 1.2 技术方案

#### 通信协议选择：HTTP 命令 + WebSocket 事件流（当前 live contract）

```
Go Backend (Fiber/Echo)
    │
    ├── HTTP Client ──────────── HTTP Server (TS Bridge)
    │   ├── POST /bridge/execute
    │   ├── GET  /bridge/status/:id
    │   ├── POST /bridge/cancel
    │   ├── POST /bridge/pause
    │   ├── POST /bridge/resume
    │   └── GET  /bridge/health
    │
    └── WebSocket Event Relay ── WS Bridge Stream
        └── TS Bridge 将执行事件主动推回 Go
```

**为什么当前 live contract 选择 HTTP + WebSocket：**

1. `/bridge/*` JSON 接口更接近当前仓库的真实实现和调试方式，Go 与 TS 两侧都已围绕它稳定落地。
2. WebSocket 足以承载 TS→Go 的事件流与心跳，不需要把命令面和事件面强行塞进同一协议。
3. 对本仓库的桌面模式与单 Pod 部署而言，可观测性和调试便利性优先于 protobuf 带来的微小序列化收益。
4. 如需保留 proto / gRPC 讨论，它们应当被视为历史方案参考，而不是当前 live contract。

#### 历史 Proto 参考（非 live contract）

```protobuf
syntax = "proto3";
package agentforge.bridge;

service AgentBridge {
  // 双向流：Go 发送指令，TS 返回 Agent 输出
  rpc Execute(stream AgentCommand) returns (stream AgentEvent);
  // 一元：查询 Agent 状态
  rpc GetStatus(StatusRequest) returns (AgentStatus);
  // 一元：终止 Agent
  rpc Cancel(CancelRequest) returns (CancelResponse);
  // 一元：健康检查
  rpc HealthCheck(HealthRequest) returns (HealthResponse);
}

message AgentCommand {
  string task_id = 1;
  string session_id = 2;
  oneof command {
    ExecuteTask execute = 3;
    PauseTask pause = 4;
    ResumeTask resume = 5;
    ProvideInput provide_input = 6;  // 人机交互输入
  }
}

message AgentEvent {
  string task_id = 1;
  string session_id = 2;
  int64 timestamp_ms = 3;
  oneof event {
    AgentOutput output = 4;          // 流式文本输出
    ToolCall tool_call = 5;          // Agent 调用工具
    ToolResult tool_result = 6;      // 工具返回结果
    StatusChange status_change = 7;  // 状态变更
    CostUpdate cost_update = 8;      // 成本更新
    AgentError error = 9;            // 错误
    SessionSnapshot snapshot = 10;   // 会话快照（用于持久化）
  }
}

message AgentOutput {
  string content = 1;
  string content_type = 2;  // "text" | "code" | "diff" | "markdown"
  int32 turn_number = 3;
}

message CostUpdate {
  int64 input_tokens = 1;
  int64 output_tokens = 2;
  double cost_usd = 3;
  double budget_remaining_usd = 4;
}

message AgentError {
  string code = 1;       // "RATE_LIMIT" | "BUDGET_EXCEEDED" | "SESSION_EXPIRED" ...
  string message = 2;
  map<string, string> metadata = 3;  // 结构化错误上下文
  bool retryable = 4;
}
```

#### 错误传播策略

```
TS Bridge 侧（捕获 Agent SDK 错误）:
  try {
    await query(...)
  } catch (e) {
    // 分类映射为 AgentError proto
    if (e instanceof RateLimitError) → code: "RATE_LIMIT", retryable: true
    if (e instanceof BudgetError)    → code: "BUDGET_EXCEEDED", retryable: false
    if (e instanceof AuthError)      → code: "AUTH_FAILED", retryable: false
    // 未知错误保留原始堆栈
    else → code: "INTERNAL", message: e.message, metadata: {stack: e.stack}
  }

Go 侧（接收并处理）:
  switch agentErr.Code {
  case "RATE_LIMIT":
      // 指数退避重试，通知调度器降低并发
  case "BUDGET_EXCEEDED":
      // 标记任务失败，通知用户
  case "AUTH_FAILED":
      // 告警 + 阻止后续调用
  default:
      // 记录日志，根据 retryable 决定是否重试
  }
```

#### 流式输出中继架构

```
Claude API ──token──→ Agent SDK ──AgentEvent──→ WS 事件流 ──→ Go Backend
                       (TS Bridge)                                  │
                                                                    ├──→ Redis Pub/Sub
                                                                    │       │
                                                                    │       ├──→ WebSocket Hub → 前端
                                                                    │       └──→ IM Bridge → 飞书/钉钉
                                                                    │
                                                                    └──→ PostgreSQL (持久化日志)
```

关键实现：Go 侧使用 goroutine 从 TS Bridge 的事件流读取，通过 channel 扇出到多个消费者（WebSocket、IM、DB），避免任一消费者阻塞影响流式接收。

```go
func (s *AgentService) relayStream(stream pb.AgentBridge_ExecuteClient, taskID string) {
    eventCh := make(chan *pb.AgentEvent, 256) // 带缓冲，防背压

    // 消费者 goroutine
    go s.wsHub.BroadcastEvents(taskID, eventCh)      // WebSocket
    go s.imBridge.ForwardEvents(taskID, eventCh)      // IM
    go s.agentRunRepo.PersistEvents(taskID, eventCh)  // DB

    // 生产者：从 TS Bridge 事件流读取
    for {
        event, err := stream.Recv()
        if err == io.EOF {
            close(eventCh)
            return
        }
        if err != nil {
            // 处理连接断开、超时等
            s.handleStreamError(taskID, err)
            close(eventCh)
            return
        }
        eventCh <- event  // 非阻塞写入（缓冲区满则丢弃旧消息并告警）
    }
}
```

#### 健康检查与故障转移

```
Go Backend                          TS Bridge
    │                                   │
    ├── gRPC HealthCheck (每 5s) ──────→│
    │   ← SERVING / NOT_SERVING         │
    │                                   │
    ├── TCP 存活检测 (每 10s) ──────────→│
    │                                   │
    └── 连续 3 次失败 → 触发告警 + 尝试重启 TS Bridge
        + 将排队任务标记为"等待恢复"
        + 已在执行的任务等待 session resume
```

#### 部署策略

```yaml
# docker-compose.yml
services:
  api:
    image: agentforge/api:latest       # Go 后端
    depends_on:
      bridge:
        condition: service_healthy
    environment:
      - BRIDGE_GRPC_ADDR=bridge:50051

  bridge:
    image: agentforge/bridge:latest    # TS Bridge (Node.js)
    healthcheck:
      test: ["CMD", "grpc_health_probe", "-addr=:50051"]
      interval: 5s
      retries: 3
    deploy:
      replicas: 2                       # 多实例，负载均衡
      resources:
        limits:
          memory: 2G                    # Agent 执行消耗内存较多
```

### 1.3 关键实现细节

- **Proto 文件版本管理**：如果保留 proto 作为历史架构描述，应明确它不是 live contract；当前测试和运维都以 `/bridge/*` 路由与 JSON schema 为准。
- **大消息分片**：Agent 输出中的代码片段和日志仍可能很大（>1MB）。当前风险点不再是 gRPC 默认 4MB 限制，而是 HTTP/WS JSON 事件的缓冲、摘要化和客户端渲染成本。
- **连接复用**：当前重点是复用 HTTP client 与 TS→Go WebSocket 长连接，而不是设计新的 gRPC 连接池。
- **优雅停机**：TS Bridge 收到 SIGTERM 后，等待所有进行中的 Agent `query()` 调用发送 SessionSnapshot，然后才退出。Go 侧在 Bridge 重启后用 snapshot 恢复会话。
- **超时配置**：Agent 执行可能持续数分钟，WebSocket 连接的 keepalive 设置需要适配长时间连接（`ping interval: 30s, pong timeout: 10s`）。

### 1.4 风险与缓解

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| WS 事件流在长时间执行中断开 | 中 | 高 | keepalive + 自动重连 + session resume |
| TS Bridge OOM（多 Agent 同时执行） | 中 | 高 | 限制单 Bridge 实例并发数 + 多实例水平扩展 |
| proto 版本不一致导致序列化失败 | 低 | 高 | CI 强制检查 + proto lint |
| Go/TS 时区差异导致时间戳不一致 | 低 | 低 | 统一使用 UTC 毫秒时间戳 |

---

## 2. 多 Agent 并发隔离

### 2.1 问题描述（为什么难）

PRD 要求支持 **20+ 并发 Agent**，每个 Agent 都在同一个 Git 仓库上独立编码。核心矛盾：**多个 Agent 可能同时修改同一仓库的不同文件，甚至同一文件的不同区域**。

具体难点：

- **Git worktree 的并发限制**：`git worktree` 不允许两个 worktree 检出同一个分支。worktree 的创建和清理本身不是原子操作，并发创建可能导致 `.git/worktrees/` 目录下的元数据冲突。
- **文件系统资源竞争**：20 个 worktree 意味着 20 份代码副本。对于大型仓库（>1GB），磁盘 I/O 和存储空间成为瓶颈。Agent 执行时还会安装依赖（node_modules、go mod download），进一步放大磁盘压力。
- **合并冲突不可避免**：当两个 Agent 修改了有交叉的代码区域，后合并的 PR 必然冲突。自动解决合并冲突是一个未解决的软件工程难题。
- **Git 锁竞争**：go-git 对同一 `.git` 目录的并发操作（push、fetch）需要序列化。Git 的引用更新使用文件锁（`refs/heads/*.lock`），高并发下容易出现锁争用。
- **worktree 泄漏**：Agent 崩溃、被 kill 或超时后，遗留的 worktree 和分支需要清理，否则会持续消耗磁盘空间。

### 2.2 技术方案

#### branch-per-agent 命名策略

```
分支命名：agent/<task-id>/<短hash>
示例：agent/550e8400-e29b/a1b2c3

Worktree 路径：/data/worktrees/<project-slug>/<task-id>/
示例：/data/worktrees/agentforge/550e8400-e29b/
```

使用 task-id 而非 agent-id 作为分支标识，原因：
- 一个 Agent 可能执行多个任务（重试场景）
- 任务与分支一一对应，方便追踪

#### Worktree 生命周期管理器

```go
type WorktreeManager struct {
    mu          sync.Mutex
    repo        *git.Repository
    basePath    string
    worktrees   map[string]*WorktreeInfo  // taskID → info
    maxActive   int                        // 并行上限
    semaphore   chan struct{}               // 控制并发创建
}

type WorktreeInfo struct {
    TaskID    string
    Branch    string
    Path      string
    CreatedAt time.Time
    Status    WorktreeStatus  // Creating | Active | Merging | Cleaning
    PID       int             // Agent 进程 PID（用于僵尸检测）
}

// 创建 worktree（带并发控制）
func (wm *WorktreeManager) Create(ctx context.Context, taskID string, baseBranch string) (*WorktreeInfo, error) {
    // 1. 获取信号量（限制同时创建数）
    select {
    case wm.semaphore <- struct{}{}:
        defer func() { <-wm.semaphore }()
    case <-ctx.Done():
        return nil, ctx.Err()
    }

    wm.mu.Lock()
    defer wm.mu.Unlock()

    // 2. 检查是否已达并行上限
    activeCount := wm.countActive()
    if activeCount >= wm.maxActive {
        return nil, ErrMaxWorktreesReached
    }

    // 3. 创建分支和 worktree
    branchName := fmt.Sprintf("agent/%s/%s", taskID, shortHash())
    wtPath := filepath.Join(wm.basePath, taskID)

    // go-git 创建 worktree
    wt, err := wm.repo.Worktree()  // 主 worktree
    // ... 创建新分支、添加 worktree ...

    info := &WorktreeInfo{
        TaskID:    taskID,
        Branch:    branchName,
        Path:      wtPath,
        CreatedAt: time.Now(),
        Status:    WorktreeActive,
    }
    wm.worktrees[taskID] = info
    return info, nil
}

// 清理 worktree（带安全检查）
func (wm *WorktreeManager) Cleanup(taskID string) error {
    wm.mu.Lock()
    info, exists := wm.worktrees[taskID]
    wm.mu.Unlock()

    if !exists {
        return nil
    }

    // 1. 确保 Agent 进程已终止
    if info.PID > 0 && processAlive(info.PID) {
        return ErrAgentStillRunning
    }

    // 2. 删除 worktree 目录
    os.RemoveAll(info.Path)

    // 3. 清理 git worktree 元数据
    // git worktree prune

    // 4. 如果 PR 已合并或任务已取消，删除远程分支
    // git push origin --delete <branch>

    wm.mu.Lock()
    delete(wm.worktrees, taskID)
    wm.mu.Unlock()

    return nil
}
```

#### 并行执行上限与排队机制

```
                   任务队列 (Redis Sorted Set)
                   ┌─────────────────────────┐
                   │ priority:1 task-A        │
                   │ priority:2 task-B        │
                   │ priority:3 task-C        │
                   │ ...                      │
                   └───────────┬─────────────┘
                               │
                   ┌───────────▼─────────────┐
                   │   Agent Pool Scheduler    │
                   │   maxConcurrent: 20       │
                   │   current: 18             │
                   │   available: 2            │
                   └───────────┬─────────────┘
                               │
               ┌───────────────┼───────────────┐
               │               │               │
          ┌────▼────┐    ┌────▼────┐    ┌────▼────┐
          │ Agent 1  │    │ Agent 2  │    │ ...     │
          │ wt: /t1  │    │ wt: /t2  │    │         │
          └──────────┘    └──────────┘    └─────────┘
```

并发上限的计算依据：
- 每个 Agent worktree 约占 100MB-1GB 磁盘（取决于仓库大小）
- 每个 Agent 进程（Node.js）约占 200-500MB 内存
- 20 个 Agent ≈ 10-20GB 磁盘 + 4-10GB 内存
- 建议初始上限 10，根据服务器配置动态调整

#### 合并冲突解决策略

```
                    Agent A 完成 → PR #1 → 审查通过
                                                │
                                        尝试合并到 main
                                                │
                                           合并成功 ✓
                                                │
                    Agent B 完成 → PR #2 → 审查通过
                                                │
                                        尝试合并到 main
                                                │
                                   ┌─── 冲突检测 ───┐
                                   │                 │
                              无冲突 ✓          有冲突 ✗
                              直接合并           │
                                           ┌─────▼──────┐
                                           │ 自动解决尝试 │
                                           │ (git merge  │
                                           │  --strategy) │
                                           └─────┬──────┘
                                                 │
                                        ┌────────┼────────┐
                                        │                 │
                                   自动解决成功       自动解决失败
                                   重新审查             │
                                                   ┌─────▼──────┐
                                                   │ 派 Agent    │
                                                   │ 重新 rebase │
                                                   │ + 修复冲突   │
                                                   └─────┬──────┘
                                                         │
                                                    ┌────┼────┐
                                                    │         │
                                               修复成功   修复失败
                                               重新审查   通知人工
```

实现要点：
- **合并队列**：使用 Redis 分布式锁实现 FIFO 合并队列，同一项目同一时间只允许一个 PR 合并到 main
- **冲突预检**：在 Agent 提交 PR 前，先 `git merge --no-commit --no-ff main` 预检是否有冲突
- **自动 rebase**：冲突时，优先让 Agent 重新 rebase 到最新 main，在 rebase 过程中由 Agent 解决冲突（利用 LLM 理解两侧代码意图）

#### Git 锁竞争解决

```go
// 所有 Git 写操作通过统一的锁管理器
type GitLockManager struct {
    locks map[string]*sync.Mutex  // repoURL → mutex
    mu    sync.RWMutex
}

func (glm *GitLockManager) WithLock(repoURL string, fn func() error) error {
    glm.mu.RLock()
    lock, exists := glm.locks[repoURL]
    glm.mu.RUnlock()

    if !exists {
        glm.mu.Lock()
        lock = &sync.Mutex{}
        glm.locks[repoURL] = lock
        glm.mu.Unlock()
    }

    lock.Lock()
    defer lock.Unlock()
    return fn()
}

// 使用：所有 push 操作序列化
glm.WithLock(repoURL, func() error {
    return repo.Push(&git.PushOptions{...})
})
```

### 2.3 关键实现细节

- **Worktree 共享 .git 目录**：所有 worktree 共享同一个 `.git` 目录，只有工作区文件被复制。这大幅减少磁盘占用，但也意味着 `.git/index.lock` 是全局瓶颈——所有 worktree 的 `git add`/`git commit` 实际上使用各自独立的 index 文件（`$GIT_DIR/worktrees/<name>/index`），不会冲突。
- **浅克隆优化**：对大型仓库，Agent worktree 可以基于浅克隆（`--depth=1`）创建，减少初始检出时间和磁盘占用。
- **依赖缓存共享**：多个 worktree 的 `node_modules`/Go module cache 可以通过符号链接或硬链接共享只读部分，减少磁盘 I/O。
- **定时清理 cron**：每小时运行一次僵尸 worktree 检测（创建超过 2 小时且 Agent 已无活动的 worktree 自动清理）。

### 2.4 风险与缓解

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| 磁盘空间耗尽 | 中 | 高 | 监控磁盘使用 + 自动清理 + 浅克隆 |
| 合并冲突导致 PR 积压 | 中 | 中 | 合并队列 + Agent 自动 rebase |
| Git 锁争用导致推送失败 | 中 | 低 | 操作级锁管理 + 重试 |
| Worktree 泄漏积累 | 中 | 中 | 定时清理 + 任务完成 hook |

---

## 3. 实时 WebSocket 架构

### 3.1 问题描述（为什么难）

PRD 要求支持 **1000+ 并发 WebSocket 连接**，且 **Agent 输出延迟 < 100ms**。核心挑战在于：

- **扇出放大**：一个 Agent 的流式输出需要同时推送给所有订阅了该任务的前端连接和 IM 频道。如果 50 个用户同时查看同一个 Agent 的实时输出，一条 Agent 消息需要扇出为 50 条 WebSocket 消息。
- **消息有序性**：Agent 的 token-by-token 输出必须按顺序到达前端。TCP 保证了单连接的有序性，但当消息经过 Redis Pub/Sub 中转时，多个消费者节点可能以不同顺序收到消息。
- **背压处理**：慢速客户端（如手机弱网环境）无法及时消费消息，会导致服务端 WebSocket 写缓冲膨胀，最终 OOM。
- **重连状态恢复**：客户端断开后重连，需要接收断开期间的缺失消息，否则 Agent 输出不完整。
- **多节点部署**：API 服务水平扩展后，同一个用户的 WebSocket 可能连接到不同节点。Agent 事件需要跨节点广播。

### 3.2 技术方案

#### 分层消息架构

> live contract note: 下方以 Redis Streams 为中心的设计更适合作为多实例扩展方案参考。当前仓库主路径已经先落在 Go 内部 WebSocket hub + project-scoped broadcast，而不是把 Redis Streams 作为实时主总线。

```
Agent 事件源                    分发层                     消费层
┌──────────┐              ┌──────────────┐           ┌──────────────┐
│ Agent 1  │──AgentEvent─→│              │──ws──→    │ 前端连接 ×50  │
│ Agent 2  │──AgentEvent─→│ Redis Streams│──ws──→    │ 前端连接 ×30  │
│ Agent 3  │──AgentEvent─→│ (持久化消息)  │──http──→  │ IM Bridge     │
│ ...      │              │              │──pg──→    │ PostgreSQL    │
└──────────┘              └──────────────┘           └──────────────┘
```

**为什么选 Redis Streams 而非 Pub/Sub：**
- Redis Streams 持久化消息，支持按 ID 范围读取（解决重连补发问题）
- 支持消费者组（Consumer Group），多个 API 节点可以分摊消费负载
- 消息自带递增 ID，天然保证有序性

#### WebSocket Hub 实现

```go
type WSHub struct {
    // 连接管理
    connections map[string]*WSConn              // connID → conn
    // 订阅管理：一个 topic 可能有多个连接订阅
    subscriptions map[string]map[string]*WSConn // topic → {connID → conn}

    mu sync.RWMutex

    // 背压控制
    maxBufferSize int  // 每连接最大缓冲消息数
}

type WSConn struct {
    ID        string
    UserID    string
    Conn      *websocket.Conn
    SendCh    chan []byte     // 带缓冲的发送通道
    Topics    map[string]bool // 已订阅的 topic
    LastMsgID string          // 最后接收的消息 ID（用于重连补发）
    CreatedAt time.Time
}

// 消息广播（带背压保护）
func (h *WSHub) Broadcast(topic string, msg []byte) {
    h.mu.RLock()
    conns := h.subscriptions[topic]
    h.mu.RUnlock()

    for _, conn := range conns {
        select {
        case conn.SendCh <- msg:
            // 正常写入
        default:
            // 缓冲区满 → 慢速客户端
            // 策略：丢弃最旧消息 + 发送"消息跳过"标记
            h.handleSlowClient(conn, msg)
        }
    }
}

// 每个连接的写 goroutine
func (h *WSHub) writePump(conn *WSConn) {
    ticker := time.NewTicker(30 * time.Second) // ping 间隔
    defer ticker.Stop()

    for {
        select {
        case msg, ok := <-conn.SendCh:
            if !ok {
                conn.Conn.WriteMessage(websocket.CloseMessage, nil)
                return
            }
            conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if err := conn.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
                return // 写入失败，关闭连接
            }
            conn.LastMsgID = extractMsgID(msg)

        case <-ticker.C:
            conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if err := conn.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}
```

#### 重连消息补发

```
客户端重连流程：

1. 客户端发送：{ "type": "reconnect", "lastMsgID": "1679825410123-5" }
2. 服务端从 Redis Streams 读取该 ID 之后的所有消息
3. 批量发送缺失消息（压缩为单次 batch）
4. 恢复实时订阅

Redis Streams 保留策略：
  - MAXLEN ~10000  每个 topic 保留最近 10000 条消息
  - 或 MINID 保留最近 1 小时的消息
  - 超出部分自动裁剪
```

```go
func (h *WSHub) handleReconnect(conn *WSConn, lastMsgID string) error {
    // 从 Redis Streams 读取缺失消息
    messages, err := h.redis.XRange(ctx,
        topicStreamKey(conn.CurrentTopic),
        lastMsgID, "+",  // 从 lastMsgID 到最新
    ).Result()

    if err != nil {
        return err
    }

    // 批量补发
    batch := make([]json.RawMessage, 0, len(messages))
    for _, msg := range messages {
        batch = append(batch, json.RawMessage(msg.Values["data"].(string)))
    }

    batchPayload, _ := json.Marshal(map[string]interface{}{
        "type":     "batch",
        "messages": batch,
        "count":    len(batch),
    })

    conn.SendCh <- batchPayload
    return nil
}
```

#### 多节点消息同步

```
                Redis Streams (消息总线)
                ┌────────────────────┐
                │ stream:agent:task1 │
                │ stream:agent:task2 │
                │ stream:project:p1  │
                └──────┬─────────────┘
                       │
         ┌─────────────┼─────────────┐
         │             │             │
    ┌────▼────┐   ┌────▼────┐   ┌────▼────┐
    │ API 节点1│   │ API 节点2│   │ API 节点3│
    │ 消费者组 │   │ 消费者组 │   │ 消费者组 │
    │ WSHub    │   │ WSHub    │   │ WSHub    │
    │ 300 conn │   │ 350 conn │   │ 350 conn │
    └─────────┘   └─────────┘   └─────────┘
```

每个 API 节点独立运行一个 Redis Streams 消费者，读取所有相关 topic 的消息，然后只推送给本节点上有对应订阅的 WebSocket 连接。

当前仓库真相补充：

- Go 侧当前已经有 `internal/ws/hub.go` 作为统一广播中心，并支持按 `project_id` 过滤。
- TS Bridge 通过自己的 `EventStreamer` 反向连接 Go WebSocket，而不是等待 Go 侧订阅一个 gRPC stream。
- 如果后续真的进入多节点大规模部署，再把 Redis Streams/NATS 这类持久化消息总线提升为实时主链会更合理；当前不应把它写成“已经采用”的默认结论。

### 3.3 关键实现细节

- **消息压缩**：对 Agent 输出（尤其是代码块）启用 WebSocket permessage-deflate 扩展，减少 30-50% 带宽。
- **心跳检测**：服务端每 30 秒 Ping，客户端必须在 10 秒内 Pong，否则认为连接死亡，清理资源。
- **订阅粒度**：支持 project 级（接收项目内所有事件）和 task 级（仅接收特定任务事件）订阅。前端根据当前页面自动切换订阅粒度。
- **消息聚合**：对高频 Agent 输出（token-by-token），在服务端做 50ms 窗口聚合，将多个 token 合并为一条消息发送，减少消息数量。
- **连接数限制**：每用户最多 5 个并发 WebSocket 连接（防止标签页泄漏导致连接数爆炸）。

### 3.4 风险与缓解

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| 慢客户端导致内存泄漏 | 中 | 高 | 背压机制 + 消息丢弃策略 + 连接超时断开 |
| Redis 单点故障 | 低 | 高 | Redis Sentinel / Redis Cluster 高可用 |
| 消息乱序 | 低 | 中 | Redis Streams ID 天然有序 + 客户端排序验证 |
| 重连风暴（服务端重启后所有客户端同时重连） | 低 | 高 | 客户端随机退避重连（1-5s） + 服务端连接限速 |

---

## 4. Token 成本控制

### 4.1 问题描述（为什么难）

Token 成本是 AgentForge 运营中最不可预测的变量。一个"修复小 bug"的任务可能花费 $0.05，也可能因为 Agent 陷入循环而花费 $50。

具体难点：

- **成本估算的不确定性**：任务开始前无法精确知道 Agent 会消耗多少 token。Agent 可能需要阅读大量代码文件（大 context），也可能一次就解决问题。估算误差可能达到 10 倍。
- **实时预算执行**：Agent SDK 的 `query()` 调用是异步流式的，token 消耗在调用过程中逐步增加。需要在 token 消耗达到预算上限时**立即**中断执行，而非等调用完成后才发现超支。
- **多 Provider 成本归一化**：Claude/GPT-4/Gemini 的定价模型不同（每百万 token 价格、输入输出差异、缓存折扣），需要统一的成本计算框架。
- **成本归属**：一个 Agent 任务可能包含多次 LLM 调用（分析代码、编码、运行测试、修复错误），成本需要准确归属到具体任务和子步骤。
- **预算告警的时效性**：从检测到预算即将耗尽到中断 Agent 执行，延迟必须 < 1 秒，否则可能多花费显著金额。

### 4.2 技术方案

#### 三层成本控制架构

```
┌─────────────────────────────────────────────────┐
│ Layer 1: 任务级预算（硬限）                        │
│   每个任务有 maxBudgetUsd (默认 $5.00)             │
│   达到 80% → 告警                                 │
│   达到 100% → 立即中断 Agent                       │
├─────────────────────────────────────────────────┤
│ Layer 2: Sprint 级预算（软限）                      │
│   Sprint 总预算 (如 $500)                          │
│   达到 80% → 通知技术负责人                         │
│   达到 100% → 新任务需人工审批                      │
├─────────────────────────────────────────────────┤
│ Layer 3: 项目级预算（软限）                          │
│   月度总预算 (如 $2000)                             │
│   达到 90% → 升级到管理层                           │
│   达到 100% → 暂停所有 Agent                       │
└─────────────────────────────────────────────────┘
```

#### 实时成本追踪器

```go
type CostTracker struct {
    mu sync.RWMutex

    // 价格表（定期从配置刷新）
    pricing map[string]*ProviderPricing  // provider → pricing

    // 实时累计
    taskCosts    map[string]*TaskCost     // taskID → cost
    sprintCosts  map[string]*SprintCost   // sprintID → cost
    projectCosts map[string]*ProjectCost  // projectID → cost
}

type ProviderPricing struct {
    Provider       string
    Model          string
    InputPerMToken float64  // 输入 token 单价（每百万 token 美元）
    OutputPerMToken float64 // 输出 token 单价
    CacheReadDiscount float64 // 缓存读取折扣（如 Claude 的 prompt caching）
    UpdatedAt      time.Time
}

type TaskCost struct {
    TaskID         string
    BudgetUsd      float64
    SpentUsd       float64
    InputTokens    int64
    OutputTokens   int64
    CacheReadTokens int64
    Calls          int32          // LLM 调用次数
    AlertSent      bool           // 80% 告警是否已发送

    // 按步骤分拆（分析/编码/测试/修复）
    StepCosts      map[string]*StepCost
}

// 核心：每次 Agent 事件中的 CostUpdate 触发此方法
func (ct *CostTracker) RecordUsage(taskID string, update *pb.CostUpdate) error {
    ct.mu.Lock()
    defer ct.mu.Unlock()

    task := ct.taskCosts[taskID]
    if task == nil {
        return ErrTaskNotTracked
    }

    // 1. 累加 token 使用量
    task.InputTokens += update.InputTokens
    task.OutputTokens += update.OutputTokens

    // 2. 计算成本（根据 Provider 定价）
    cost := ct.calculateCost(update)
    task.SpentUsd += cost

    // 3. 检查预算
    ratio := task.SpentUsd / task.BudgetUsd

    if ratio >= 1.0 {
        // 立即中断！
        return ErrBudgetExceeded
    }

    if ratio >= 0.8 && !task.AlertSent {
        // 异步发送告警（不阻塞主流程）
        go ct.sendBudgetAlert(taskID, task.SpentUsd, task.BudgetUsd)
        task.AlertSent = true
    }

    // 4. 级联更新 Sprint 和 Project 成本
    ct.updateSprintCost(task.SprintID, cost)
    ct.updateProjectCost(task.ProjectID, cost)

    // 5. 写入 Redis（异步，用于跨节点同步）
    go ct.syncToRedis(taskID, task)

    return nil
}

// 成本计算
func (ct *CostTracker) calculateCost(update *pb.CostUpdate) float64 {
    pricing := ct.pricing[update.Provider]
    if pricing == nil {
        // 回退到默认定价
        pricing = ct.pricing["default"]
    }

    inputCost := float64(update.InputTokens) / 1_000_000 * pricing.InputPerMToken
    outputCost := float64(update.OutputTokens) / 1_000_000 * pricing.OutputPerMToken

    // 缓存折扣
    if update.CacheReadTokens > 0 {
        cacheSaved := float64(update.CacheReadTokens) / 1_000_000 * pricing.InputPerMToken * pricing.CacheReadDiscount
        inputCost -= cacheSaved
    }

    return inputCost + outputCost
}
```

#### 预算中断流程

```
CostTracker.RecordUsage() 返回 ErrBudgetExceeded
    │
    ▼
Agent Service 捕获错误
    │
    ├── 1. 发送 gRPC Cancel 到 TS Bridge（< 100ms）
    │       → TS Bridge 调用 Agent SDK 的 abort/cancel
    │
    ├── 2. 发送 SessionSnapshot 请求（保存当前进度）
    │       → 用于后续手动增加预算后的 session resume
    │
    ├── 3. 更新任务状态为 "budget_exceeded"
    │
    ├── 4. WebSocket 推送中断事件到前端
    │       → { type: "agent.budget_exceeded", taskId, spent, budget }
    │
    └── 5. IM 通知任务分配者
            → "任务 #{title} 的 Agent 预算已耗尽（${spent}/${budget}），需要增加预算或手动完成"
```

#### 执行前成本估算

```go
type CostEstimator struct {
    // 历史数据统计
    historyRepo *AgentRunRepository
}

type CostEstimate struct {
    EstimatedUsd    float64
    ConfidenceLevel string  // "low" | "medium" | "high"
    Breakdown       map[string]float64  // 各步骤估算
    SimilarTasks    []TaskReference      // 参考历史任务
    Recommendation  string               // "建议预算" 文本
}

func (ce *CostEstimator) Estimate(task *Task) (*CostEstimate, error) {
    // 1. 查找同项目、同类型的历史任务成本
    similar, err := ce.historyRepo.FindSimilarTasks(task.ProjectID, task.Labels, 20)

    // 2. 计算统计值
    costs := extractCosts(similar)
    p50 := percentile(costs, 50)
    p90 := percentile(costs, 90)

    // 3. 根据任务描述复杂度调整
    complexity := ce.assessComplexity(task.Description)
    multiplier := complexityMultiplier(complexity) // 1.0 / 1.5 / 2.5

    estimated := p50 * multiplier

    // 4. 推荐预算 = p90 × 1.2（留 20% 余量）
    recommended := p90 * multiplier * 1.2

    confidence := "medium"
    if len(similar) < 5 {
        confidence = "low"
    } else if len(similar) > 20 {
        confidence = "high"
    }

    return &CostEstimate{
        EstimatedUsd:    estimated,
        ConfidenceLevel: confidence,
        Recommendation:  fmt.Sprintf("建议预算 $%.2f（基于 %d 个类似任务的历史数据）", recommended, len(similar)),
    }, nil
}
```

### 4.3 关键实现细节

- **Token 计数的来源**：优先使用 Claude API 响应中的 `usage` 字段（精确值），而非客户端 tokenizer 估算。TS Bridge 在每次 API 调用后立即报告 `CostUpdate` 事件。
- **价格表更新**：Provider 定价随时可能变化。价格表存储在 PostgreSQL 中，管理后台可手动更新，同时提供 API 端点用于自动化更新脚本。
- **成本的最终一致性**：实时追踪使用 Redis（低延迟），定期（每 5 分钟）与 PostgreSQL 同步（持久化）。两者偶尔的不一致在最终同步时修正。
- **预算溢出容忍**：由于 CostUpdate 不是实时到达的（Agent SDK 可能批量报告），实际花费可能略微超过预算。设计上接受 5-10% 的超支容忍度。

### 4.4 风险与缓解

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| Agent 在预算检查间隙大量消耗 token | 中 | 中 | 减小 CostUpdate 报告间隔 + 超支容忍 |
| Provider 定价变更未及时同步 | 低 | 低 | 价格表变更告警 + 人工确认 |
| 历史数据不足导致估算偏差大 | 高（初期） | 低 | 冷启动期使用保守默认值 + 标记"low confidence" |
| 多节点成本追踪不一致 | 低 | 中 | Redis 原子操作 + 定期 PostgreSQL 校准 |

---

## 5. Agent 生命周期管理

### 5.1 问题描述（为什么难）

Agent 不是简单的无状态函数调用——它是一个**有状态、长时间运行、可能产生副作用的进程**。管理 Agent 的完整生命周期（spawn → monitor → pause → resume → kill）面临以下挑战：

- **进程模型复杂**：Claude Agent SDK 通过 `query()` 调用执行，每次调用是一个长时间运行的异步操作。SDK 可能在内部管理子进程（如执行 shell 命令、运行测试）。暂停一个 Agent 意味着需要暂停整个进程树。
- **会话持久化**：Agent SDK 支持 session resume，但这要求在暂停/崩溃时正确保存会话状态（包括 conversation history、tool results、正在进行的操作上下文）。
- **崩溃恢复的不确定性**：Agent 崩溃后恢复执行，代码仓库的状态可能已经改变（其他 Agent 合并了代码）。恢复的 Agent 需要感知环境变化并调整行为。
- **超时处理的粒度**：简单的"30 分钟超时 kill"太粗暴——Agent 可能在超时前 1 秒刚开始一个关键操作。需要区分"Agent 空转"（真正卡住）和"Agent 正在执行耗时操作"（正常）。
- **僵尸检测**：Agent 进程可能存在但不产生任何输出和活动（死循环、等待永远不会到达的输入）。需要启发式方法区分"正在思考"和"已经卡死"。

### 5.2 技术方案

#### Agent 状态机

```
                    ┌──────────────────────────────────┐
                    │                                  │
                    ▼                                  │
             ┌──────────┐                              │
     spawn → │ Starting │                              │
             └────┬─────┘                              │
                  │ Agent SDK 就绪                      │
                  ▼                                    │
             ┌──────────┐     pause      ┌──────────┐ │
             │ Running  │ ──────────────→│ Paused   │ │
             │          │                │          │ │
             │          │ ←──────────────│          │ │
             └────┬─────┘     resume     └────┬─────┘ │
                  │                           │       │
         ┌───────┼───────┐              kill / │       │
         │       │       │              timeout│       │
    完成  │  失败  │  kill  │                    │       │
         │       │       │                    │       │
         ▼       ▼       ▼                    ▼       │
    ┌─────┐ ┌───────┐ ┌──────────┐    ┌──────────┐   │
    │Done │ │Failed │ │Cancelled │    │Cancelled │   │
    └─────┘ └───┬───┘ └──────────┘    └──────────┘   │
                │                                     │
                │ retryable?                          │
                │ retries < maxRetries?                │
                └── yes ──────────────────────────────┘
```

#### Agent 进程管理器

```go
type AgentManager struct {
    agents    map[string]*AgentInstance  // taskID → instance
    mu        sync.RWMutex

    bridge    AgentBridgeClient          // gRPC client to TS Bridge
    wsHub     *WSHub
    costTracker *CostTracker
    wtManager *WorktreeManager

    // 配置
    maxRetries        int           // 最大重试次数（默认 2）
    idleTimeout       time.Duration // 空转超时（默认 5 分钟）
    maxExecutionTime  time.Duration // 最大执行时间（默认 30 分钟）
    heartbeatInterval time.Duration // 心跳检测间隔（默认 30 秒）
}

type AgentInstance struct {
    TaskID      string
    SessionID   string
    Status      AgentStatus

    // 活动追踪
    LastActivity time.Time         // 最后一次输出/工具调用时间
    LastOutput   string            // 最后一条输出内容
    TurnCount    int               // 当前对话轮次
    MaxTurns     int               // 最大轮次限制

    // 恢复相关
    RetryCount   int
    Snapshots    []*SessionSnapshot // 会话快照（用于恢复）

    // 取消控制
    cancel       context.CancelFunc
    done         chan struct{}
}

// 启动 Agent
func (am *AgentManager) Spawn(ctx context.Context, task *Task) (*AgentInstance, error) {
    // 1. 创建 worktree
    wt, err := am.wtManager.Create(ctx, task.ID, task.Project.RepoBranch)
    if err != nil {
        return nil, fmt.Errorf("创建 worktree 失败: %w", err)
    }

    // 2. 初始化 Agent 实例
    agentCtx, cancel := context.WithCancel(ctx)
    instance := &AgentInstance{
        TaskID:       task.ID,
        Status:       AgentStarting,
        LastActivity: time.Now(),
        MaxTurns:     task.MaxTurns,
        cancel:       cancel,
        done:         make(chan struct{}),
    }

    am.mu.Lock()
    am.agents[task.ID] = instance
    am.mu.Unlock()

    // 3. 启动执行 goroutine
    go am.executeAgent(agentCtx, instance, task, wt)

    // 4. 启动监控 goroutine
    go am.monitorAgent(agentCtx, instance)

    return instance, nil
}

// Agent 执行主循环
func (am *AgentManager) executeAgent(ctx context.Context, inst *AgentInstance, task *Task, wt *WorktreeInfo) {
    defer close(inst.done)

    // 构建 Agent 执行请求
    stream, err := am.bridge.Execute(ctx)
    if err != nil {
        inst.Status = AgentFailed
        return
    }

    // 发送执行命令
    stream.Send(&pb.AgentCommand{
        TaskId: task.ID,
        Command: &pb.AgentCommand_Execute{
            Execute: &pb.ExecuteTask{
                Prompt:       task.Description,
                WorktreePath: wt.Path,
                BranchName:   wt.Branch,
                MaxTurns:     int32(task.MaxTurns),
                BudgetUsd:    task.BudgetUsd,
            },
        },
    })

    inst.Status = AgentRunning

    // 读取事件流
    for {
        event, err := stream.Recv()
        if err == io.EOF {
            inst.Status = AgentDone
            return
        }
        if err != nil {
            if ctx.Err() != nil {
                inst.Status = AgentCancelled
            } else {
                inst.Status = AgentFailed
                // 尝试重试
                am.maybeRetry(inst, task, wt)
            }
            return
        }

        // 更新活动时间
        inst.LastActivity = time.Now()

        // 处理各类事件
        switch e := event.Event.(type) {
        case *pb.AgentEvent_Output:
            inst.LastOutput = e.Output.Content
            inst.TurnCount = int(e.Output.TurnNumber)
            am.wsHub.Broadcast("task:"+task.ID, eventToJSON(event))

        case *pb.AgentEvent_CostUpdate:
            if err := am.costTracker.RecordUsage(task.ID, e.CostUpdate); err != nil {
                if errors.Is(err, ErrBudgetExceeded) {
                    inst.cancel()
                    inst.Status = AgentFailed
                    return
                }
            }

        case *pb.AgentEvent_Snapshot:
            inst.Snapshots = append(inst.Snapshots, e.Snapshot)

        case *pb.AgentEvent_Error:
            if e.Error.Retryable {
                // 可重试错误（如 rate limit），让 TS Bridge 内部处理
                continue
            }
            inst.Status = AgentFailed
            return
        }
    }
}
```

#### 僵尸检测与超时处理

```go
func (am *AgentManager) monitorAgent(ctx context.Context, inst *AgentInstance) {
    ticker := time.NewTicker(am.heartbeatInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-inst.done:
            return
        case <-ticker.C:
            am.checkAgentHealth(inst)
        }
    }
}

func (am *AgentManager) checkAgentHealth(inst *AgentInstance) {
    now := time.Now()
    idleDuration := now.Sub(inst.LastActivity)
    totalDuration := now.Sub(inst.CreatedAt)

    // 1. 空转超时检测
    if idleDuration > am.idleTimeout {
        // Agent 超过 5 分钟没有任何输出
        // 先查询 Bridge 侧的 Agent 状态
        status, err := am.bridge.GetStatus(context.Background(), &pb.StatusRequest{
            TaskId: inst.TaskID,
        })

        if err != nil || status.State == "stuck" {
            // 确认卡死，发送警告
            am.sendAlert(inst, "Agent 已空转 %v，可能卡住", idleDuration)

            // 超过 2 倍空转超时 → 强制终止
            if idleDuration > am.idleTimeout*2 {
                am.Kill(inst.TaskID, "idle_timeout")
            }
        }
        // 如果 Bridge 报告 Agent 仍在执行工具（如运行测试），则不视为空转
    }

    // 2. 总执行时间超时
    if totalDuration > am.maxExecutionTime {
        am.Kill(inst.TaskID, "max_execution_time")
    }

    // 3. 轮次超限
    if inst.TurnCount >= inst.MaxTurns {
        am.Kill(inst.TaskID, "max_turns_exceeded")
    }
}
```

#### 会话持久化与恢复

```go
// 暂停 Agent（保存会话状态）
func (am *AgentManager) Pause(taskID string) error {
    inst := am.getAgent(taskID)
    if inst == nil || inst.Status != AgentRunning {
        return ErrInvalidState
    }

    // 1. 请求 TS Bridge 保存 session snapshot
    _, err := am.bridge.Execute(context.Background())
    // 发送 PauseTask command → Bridge 触发 SDK session save

    // 2. 等待 snapshot 返回
    // ... (通过 AgentEvent_Snapshot 接收)

    // 3. 持久化 snapshot 到 PostgreSQL
    am.snapshotRepo.Save(taskID, inst.Snapshots[len(inst.Snapshots)-1])

    // 4. 更新状态
    inst.Status = AgentPaused

    return nil
}

// 恢复 Agent
func (am *AgentManager) Resume(taskID string) error {
    inst := am.getAgent(taskID)
    if inst == nil || inst.Status != AgentPaused {
        return ErrInvalidState
    }

    // 1. 从 PostgreSQL 加载最新 snapshot
    snapshot, _ := am.snapshotRepo.GetLatest(taskID)

    // 2. 发送恢复命令到 TS Bridge
    // Bridge 使用 Agent SDK 的 session resume 功能
    // 传入 snapshot 中的 session_id + conversation history

    // 3. 重启执行和监控 goroutine
    go am.executeAgent(ctx, inst, task, wt)
    go am.monitorAgent(ctx, inst)

    inst.Status = AgentRunning
    return nil
}
```

### 5.3 关键实现细节

- **进程树管理**：Agent 可能通过 shell 工具启动子进程（如 `npm test`）。Kill Agent 时必须终止整个进程组（`kill -PGID`），否则子进程成为孤儿进程。
- **Session Resume 的限制**：Claude Agent SDK 的 session resume 依赖 conversation history。如果会话过长（超过 context window），resume 后 Agent 可能丢失早期上下文。需要在 snapshot 中保留关键上下文摘要。
- **重试策略**：失败重试时，Agent 从上一个 snapshot 恢复，而非从头开始。如果没有 snapshot，则从头重试但复用已有的 worktree（已完成的代码修改仍然存在）。
- **优雅终止 vs 强制终止**：优雅终止给 Agent 10 秒时间保存状态并提交已完成的工作。超时后强制终止进程。

### 5.4 风险与缓解

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| Session resume 后 Agent 行为异常 | 中 | 中 | 恢复后自动运行 sanity check + 人工确认 |
| 子进程泄漏 | 中 | 低 | 进程组管理 + 定时扫描孤儿进程 |
| 僵尸检测误判（正常耗时操作被 kill） | 低 | 高 | 查询 Bridge 侧实际状态 + 多级超时 |
| 并发 Kill + Resume 竞态 | 低 | 中 | 操作级互斥锁 + 状态机严格校验 |

---

## 6. AI 任务分解可靠性

### 6.1 问题描述（为什么难）

PRD 要求任务自动分解准确率 > 80%。"准确"的定义本身就很模糊：

- **需求的歧义性**：用户通过 IM 发送的需求描述通常是非结构化的自然语言，可能缺少上下文、包含隐含假设、甚至自相矛盾。例如"优化搜索功能"——是优化性能？优化 UI？还是增加搜索范围？
- **拆解粒度的判断**：什么粒度的子任务适合 Agent？太粗（"实现用户认证模块"）Agent 难以一次完成；太细（"在第 42 行添加一个 if 判断"）失去了 AI 分解的意义。
- **技术上下文的获取**：分解任务需要理解代码库结构、技术栈、已有实现。LLM 的 context window 无法容纳整个代码库，需要智能选择相关文件。
- **人-Agent 任务分类**：判断一个子任务适合人还是 Agent 需要考虑复杂度、安全性、创造性等多维因素。这本身就是一个需要领域知识的决策。
- **评估反馈闭环**：如何知道分解结果是否"好"？只有在 Agent 实际执行后才能验证，但此时成本已经产生。

### 6.2 技术方案

#### 分层分解架构

```
用户输入（自然语言需求）
        │
        ▼
┌───────────────────────────────┐
│ Step 1: 需求澄清              │
│   - 识别模糊点                 │
│   - 通过 IM 追问（如有必要）    │
│   - 提取结构化需求             │
└───────────────┬───────────────┘
                │
                ▼
┌───────────────────────────────┐
│ Step 2: 代码上下文获取         │
│   - 根据需求关键词搜索代码库   │
│   - 获取相关文件和函数签名      │
│   - 获取项目结构和依赖关系      │
└───────────────┬───────────────┘
                │
                ▼
┌───────────────────────────────┐
│ Step 3: 任务分解（LLM）        │
│   - 输入：需求 + 代码上下文    │
│   - 输出：子任务列表           │
│   - 每个子任务标注：           │
│     · 标题和描述               │
│     · 估计复杂度               │
│     · 推荐执行者（人/Agent）   │
│     · 依赖关系                │
│     · 涉及文件列表             │
└───────────────┬───────────────┘
                │
                ▼
┌───────────────────────────────┐
│ Step 4: 分解验证               │
│   - 子任务覆盖度检查           │
│   - 依赖关系合理性             │
│   - 粒度合理性（太粗/太细）     │
│   - 总成本估算                 │
└───────────────┬───────────────┘
                │
                ▼
┌───────────────────────────────┐
│ Step 5: 人工确认（可选）       │
│   - 推送分解结果到 IM          │
│   - 用户一键确认/调整/拒绝     │
│   - 调整后的结果反馈给 AI      │
└───────────────────────────────┘
```

#### 分解 Prompt 工程

```go
type DecompositionRequest struct {
    Requirement    string            // 原始需求
    ProjectContext *ProjectContext    // 项目信息
    CodeContext    []*CodeSnippet    // 相关代码片段
    PastExamples   []*DecompositionExample // 历史成功分解案例（few-shot）
}

type ProjectContext struct {
    TechStack    []string          // ["Go", "PostgreSQL", "Redis"]
    Structure    string            // 项目目录结构摘要
    Conventions  string            // 编码规范摘要
    TeamSkills   map[string][]string // member → skills
}

// 构建 system prompt
func buildDecompositionPrompt(req *DecompositionRequest) string {
    return fmt.Sprintf(`你是 AgentForge 的任务分解引擎。你的职责是将用户需求拆解为可独立执行的子任务。

## 项目上下文
- 技术栈：%s
- 项目结构：
%s

## 编码规范
%s

## 相关代码
%s

## 分解规则
1. 每个子任务应该是一个独立的、可验证的工作单元
2. 子任务粒度指南：
   - 适合 Agent：修复明确的 bug、实现 CRUD 接口、编写单元测试、添加文档
   - 适合人工：架构设计、安全审计、性能优化、复杂算法、跨模块重构
3. 标注依赖关系（哪些任务必须先完成）
4. 每个子任务包含涉及的文件列表（即使是预估）
5. 估计每个子任务的 Agent 执行成本（low/medium/high）

## 输出格式
返回 JSON 数组，每个元素包含：
{
  "title": "简短标题",
  "description": "详细描述，包含验收标准",
  "executor": "agent" | "human",
  "executor_reason": "为什么选择这个执行者",
  "complexity": "low" | "medium" | "high",
  "estimated_cost": "low" | "medium" | "high",
  "dependencies": ["task-index-0", "task-index-1"],
  "files": ["path/to/file1.go", "path/to/file2.go"],
  "labels": ["bug", "feature", "test", "docs"]
}

## 历史成功案例（参考）
%s

## 用户需求
%s`,
        strings.Join(req.ProjectContext.TechStack, ", "),
        req.ProjectContext.Structure,
        req.ProjectContext.Conventions,
        formatCodeContext(req.CodeContext),
        formatExamples(req.PastExamples),
        req.Requirement,
    )
}
```

#### 准确率追踪与反馈循环

```go
type DecompositionFeedback struct {
    DecompositionID string
    TaskID          string

    // Agent 执行结果反馈
    AgentSucceeded  bool     // Agent 是否成功完成
    ActualCost      float64  // 实际成本
    ActualFiles     []string // 实际修改的文件

    // 人工反馈
    UserRating      int      // 1-5 星评分（可选）
    UserAdjustments []string // 用户做了哪些调整
}

// 定期计算准确率
func (ds *DecompositionService) CalculateAccuracy(projectID string, period time.Duration) float64 {
    feedbacks := ds.repo.GetFeedbacks(projectID, time.Now().Add(-period), time.Now())

    total := len(feedbacks)
    if total == 0 {
        return 0
    }

    successful := 0
    for _, f := range feedbacks {
        // "准确" = Agent 成功完成 + 成本未超估 2 倍 + 文件列表覆盖度 > 70%
        if f.AgentSucceeded &&
           f.ActualCost <= f.EstimatedCost*2 &&
           fileCoverage(f.ActualFiles, f.EstimatedFiles) > 0.7 {
            successful++
        }
    }

    return float64(successful) / float64(total)
}
```

### 6.3 关键实现细节

- **代码上下文检索**：使用 go-git 获取项目文件树，结合关键词匹配和 embedding 相似度搜索，选择最相关的 5-10 个文件的摘要（函数签名 + 注释）作为 context。
- **Few-shot 学习**：存储历史上成功的分解案例（用户确认且 Agent 成功执行），作为 few-shot 示例提供给 LLM，持续提升分解质量。
- **渐进式分解**：对于大型需求，先做粗粒度分解（3-5 个模块级任务），每个模块级任务开始执行前再做细粒度分解。避免一次性分解失控。
- **人机协同兜底**：当 AI 分解置信度低时（如需求非常模糊），自动通知人工审查。用户可以在 IM 中直接修改分解结果。

### 6.4 风险与缓解

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| 分解遗漏关键子任务 | 中 | 高 | 覆盖度检查 + 人工确认步骤 |
| Agent/人工分类错误 | 中 | 中 | 基于历史数据训练分类规则 + 回退到人工 |
| 依赖关系错误导致执行顺序混乱 | 低 | 高 | DAG 验证（无环） + 分解后模拟执行 |
| 冷启动期准确率低 | 高（初期） | 低 | 初期全部人工确认 + 积累数据 |

---

## 7. 安全沙箱

### 7.1 问题描述（为什么难）

Agent 本质上是一个**拥有代码执行能力的 AI**。如果不加限制，Agent 可能：

- **读取/泄露敏感信息**：环境变量中的 API key、`.env` 文件、私钥、数据库凭证。
- **执行恶意命令**：`rm -rf /`、挖矿程序、反弹 shell。
- **网络外联**：将代码或凭证发送到外部服务器。
- **资源耗尽**：无限循环消耗 CPU、分配大量内存、填满磁盘。
- **提权攻击**：利用 Agent 运行时的权限执行超出预期的操作。

真实案例参考：OpenClaw 曾发生过 API key 泄露事件（PRD 风险 #10），说明 Agent 安全不是理论问题。

### 7.2 技术方案

#### 三层沙箱架构

```
┌─────────────────────────────────────────────────────┐
│ Layer 1: 工具白名单（Agent SDK 层）                    │
│   - 只允许预定义的工具集                               │
│   - 文件操作限制在 worktree 目录内                     │
│   - Shell 命令白名单                                  │
├─────────────────────────────────────────────────────┤
│ Layer 2: 进程隔离（操作系统层）                         │
│   - 独立 Linux 用户（最小权限）                        │
│   - cgroup 资源限制（CPU/内存/磁盘 IO）                │
│   - 网络策略（限制外联范围）                           │
├─────────────────────────────────────────────────────┤
│ Layer 3: 容器隔离（部署层）— Phase 2                   │
│   - 每个 Agent 独立 Docker 容器                       │
│   - 只读根文件系统 + 可写 worktree volume              │
│   - seccomp 限制系统调用                              │
└─────────────────────────────────────────────────────┘
```

#### Layer 1: 工具白名单

```typescript
// TS Bridge 中的 Agent 配置
const agentConfig = {
  tools: [
    // 文件操作：限制在 worktree 路径
    createFileTool({
      allowedPaths: [worktreePath],
      blockedPatterns: [
        '**/.env*',          // 环境变量文件
        '**/*.pem',          // 私钥
        '**/*.key',          // 密钥
        '**/credentials*',   // 凭证文件
        '**/secrets*',       // 密钥配置
      ],
    }),

    // Shell 命令：白名单
    createShellTool({
      allowedCommands: [
        'npm test',
        'npm run build',
        'npm run lint',
        'go test ./...',
        'go build ./...',
        'go vet ./...',
        'git add',
        'git commit',
        'git diff',
        'git log',
        'git status',
        'cat', 'ls', 'find', 'grep',  // 只读命令
      ],
      blockedPatterns: [
        /curl\s.*\|.*sh/,    // 防止 curl | sh 攻击
        /wget/,               // 防止下载执行
        /rm\s+-rf/,           // 防止递归删除
        /chmod\s+[0-7]*s/,   // 防止 SUID 设置
        /ssh\s/,              // 防止 SSH 连接
        /nc\s/,               // 防止 netcat
      ],
      workingDirectory: worktreePath,
      timeout: 120_000, // 单命令 2 分钟超时
    }),

    // Git 操作：仅限当前仓库
    createGitTool({
      repoPath: worktreePath,
      allowPush: true,
      allowForcePush: false,
    }),
  ],
};
```

#### Layer 2: 进程隔离

```go
// 启动 Agent 进程时的隔离配置
type SandboxConfig struct {
    // Linux 用户隔离
    User    string // "agentforge-agent" (低权限用户)
    Group   string // "agentforge-agents"

    // cgroup 资源限制
    CPUShares    int64  // 1024 (一个核心)
    MemoryLimit  int64  // 2GB
    DiskIOWeight int    // 100 (正常优先级)
    PidsMax      int64  // 256 (最大进程数)

    // 网络限制
    NetworkPolicy NetworkPolicy

    // 文件系统
    ReadOnlyPaths []string // ["/usr", "/lib", "/bin"]
    WritablePaths []string // [worktreePath, "/tmp/agent-<id>"]
    MaskedPaths   []string // ["/proc/kcore", "/proc/keys"]
}

type NetworkPolicy struct {
    AllowOutbound []string // 允许的出站目标
    // 例如：["api.anthropic.com:443", "api.openai.com:443",
    //        "github.com:443", "registry.npmjs.org:443"]
    DenyAll       bool     // Phase 2: 默认拒绝所有出站
}
```

#### 凭证隔离

```
Agent 看不到的敏感信息：
┌─────────────────────────────────────────────┐
│ 1. LLM API Key     → 仅 TS Bridge 持有     │
│    Agent 通过 Bridge 调用 LLM，无需直接持有 key│
│                                             │
│ 2. GitHub Token     → 仅 Go Backend 持有    │
│    Agent 通过工具间接操作 Git，无需持有 token  │
│                                             │
│ 3. Database 凭证   → 仅 Go Backend 持有      │
│    Agent 无数据库访问权限                     │
│                                             │
│ 4. 项目 .env 文件   → 从 worktree 中排除     │
│    .gitignore + 工具白名单双重保护            │
└─────────────────────────────────────────────┘
```

#### 审计追踪

```go
type AuditLogger struct {
    store AuditStore // PostgreSQL
}

type AuditEntry struct {
    ID          string
    TaskID      string
    AgentID     string
    Timestamp   time.Time
    Action      string          // "file_read" | "file_write" | "shell_exec" | "git_push" | ...
    Target      string          // 文件路径 / 命令 / URL
    Result      string          // "allowed" | "blocked" | "error"
    Details     json.RawMessage // 额外上下文
}

// 所有 Agent 操作都通过审计中间件
func (al *AuditLogger) LogAndCheck(entry *AuditEntry) error {
    // 1. 持久化日志
    al.store.Insert(entry)

    // 2. 实时异常检测
    if al.isAnomaly(entry) {
        // 例如：Agent 在短时间内读取大量不相关文件
        // 或者尝试访问被阻止的路径多次
        al.sendSecurityAlert(entry)
    }

    return nil
}

// 异常检测规则示例
func (al *AuditLogger) isAnomaly(entry *AuditEntry) bool {
    // 规则 1：单分钟内 > 50 次文件读取
    // 规则 2：连续 > 3 次尝试访问被阻止的路径
    // 规则 3：尝试执行不在白名单中的命令
    // ...
    return false
}
```

### 7.3 关键实现细节

- **MVP 阶段聚焦 Layer 1**：工具白名单是最快实现且最有效的安全措施。Agent SDK 本身支持自定义工具集，直接在 TS Bridge 中配置。
- **环境变量清洗**：Agent 进程的环境变量只保留最小集（`PATH`、`HOME`、`NODE_PATH`），所有敏感变量（`ANTHROPIC_API_KEY`、`DATABASE_URL` 等）不传递到 Agent 进程。
- **输出审查**：Agent 提交的代码在 PR 审查阶段会被 claude-code-security-review 扫描，检测是否引入了硬编码凭证、不安全的配置等。
- **定期安全审计**：每周生成 Agent 操作审计报告，人工审查异常模式。

### 7.4 风险与缓解

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| Agent 通过代码注入绕过白名单 | 低 | 高 | 多层防御 + 输出审查 + 异常检测 |
| 白名单过严影响 Agent 能力 | 中 | 中 | 按项目可配置 + 逐步放宽 |
| 容器逃逸（Phase 2） | 极低 | 极高 | seccomp + 定期安全更新 |
| 审计日志被篡改 | 极低 | 高 | 日志写入独立存储 + 追加写（不可修改） |

---

## 8. IM 自然语言解析

### 8.1 问题描述（为什么难）

用户通过 IM 用自然语言与 AgentForge 交互。这不是简单的命令解析——需要理解意图、保持上下文、处理歧义。

具体难点：

- **意图分类的边界模糊**：
  - "帮我看看 auth 模块" → 代码审查？Bug 查找？文档生成？
  - "这个太慢了" → 性能优化？前端加载优化？数据库查询优化？
- **多轮上下文追踪**：
  - 用户："修复 #42"
  - Agent："您指的是 issue #42 '登录失败'吗？"
  - 用户："对"
  - 两条消息之间可能间隔数分钟，中间有其他人的消息穿插。
- **多语言支持**：团队可能使用中文、英文、甚至混合（"帮我 fix 这个 bug"）。意图分类需要语言无关。
- **命令消歧**：
  - "@AgentForge 把这个分给小李" → "这个"指什么？最近的 issue？当前讨论的话题？
  - "@AgentForge 测试一下" → 运行全部测试？运行特定文件的测试？
- **群聊噪声过滤**：在群聊中，大量消息与 AgentForge 无关。需要只响应明确指向 AgentForge 的消息，避免误触发。

### 8.2 技术方案

#### 分层解析架构

```
IM 消息到达
    │
    ▼
┌───────────────────────────────┐
│ Layer 1: 触发检测              │
│   - @AgentForge 提及           │
│   - / 斜杠命令                 │
│   - 私聊（直接触发）           │
│   - 非触发消息 → 忽略          │
└───────────────┬───────────────┘
                │
                ▼
┌───────────────────────────────┐
│ Layer 2: 命令路由              │
│   - 斜杠命令 → 直接路由       │
│   - 自然语言 → 意图分类       │
└───────┬───────────┬───────────┘
        │           │
   斜杠命令     自然语言
        │           │
        ▼           ▼
┌──────────┐  ┌──────────────────┐
│ 命令解析器│  │ 意图分类器 (LLM) │
│ (正则+   │  │                  │
│  模板)   │  │  意图：           │
│          │  │  · create_task    │
│          │  │  · assign_task    │
│          │  │  · query_status   │
│          │  │  · run_agent      │
│          │  │  · review_code    │
│          │  │  · clarify (追问) │
│          │  │  · unknown        │
└────┬─────┘  └────────┬─────────┘
     │                 │
     └────────┬────────┘
              │
              ▼
┌───────────────────────────────┐
│ Layer 3: 参数提取              │
│   - 从消息中提取实体            │
│     · issue/PR 编号            │
│     · 人名 → 成员 ID           │
│     · 项目名                   │
│     · 代码引用                 │
│   - 缺失参数 → 追问           │
└───────────────┬───────────────┘
                │
                ▼
┌───────────────────────────────┐
│ Layer 4: 执行 + 响应           │
│   - 调用对应 API               │
│   - 格式化响应消息             │
│   - 更新上下文                 │
└───────────────────────────────┘
```

#### 意图分类实现

```go
type IntentClassifier struct {
    bridge     BridgeClient    // gRPC client → Agent SDK Bridge (统一 AI 出口)
    contextMgr *ContextManager // 多轮上下文
}

type ClassificationResult struct {
    Intent     string            // "create_task" | "assign_task" | ...
    Confidence float64           // 0.0 - 1.0
    Params     map[string]string // 提取的参数
    NeedMore   []string          // 需要追问的参数
}

func (ic *IntentClassifier) Classify(ctx context.Context, msg *IMMessage) (*ClassificationResult, error) {
    // 1. 获取会话上下文（最近 10 条消息）
    history := ic.contextMgr.GetHistory(msg.ChannelID, msg.UserID, 10)

    // 2. 构建分类 prompt
    prompt := fmt.Sprintf(`你是 AgentForge 的意图分类器。根据用户消息和对话历史，判断用户意图。

## 可用意图
- create_task: 创建新任务（需要：description）
- assign_task: 分配任务（需要：task_id, assignee）
- query_status: 查询状态（需要：task_id 或 project_id）
- run_agent: 直接执行 Agent 编码（需要：prompt, 可选 repo/branch）
- review_code: 触发代码审查（需要：pr_url 或 task_id）
- decompose_task: 分解任务（需要：task_id 或 description）
- list_tasks: 列出任务（可选：filter）
- sprint_status: Sprint 状态
- clarify: 需要用户澄清
- unknown: 无法识别

## 对话历史
%s

## 当前消息
用户 [%s]: %s

## 输出格式（JSON）
{
  "intent": "意图名称",
  "confidence": 0.95,
  "params": {"key": "value"},
  "need_more": ["缺失的参数名"],
  "clarification_question": "如果需要追问，这里是追问内容"
}`,
        formatHistory(history),
        msg.UserName,
        msg.Content,
    )

    // 3. 调用 LLM（使用轻量模型，如 Claude Haiku，降低成本和延迟）
    result, err := ic.llm.Generate(ctx, prompt, &llm.Options{
        Model:       "claude-haiku-4-5",  // 快速 + 低成本
        MaxTokens:   200,
        Temperature: 0.1,  // 低温度提高确定性
    })

    // 4. 解析 JSON 结果
    var classification ClassificationResult
    json.Unmarshal([]byte(result), &classification)

    // 5. 低置信度检测
    if classification.Confidence < 0.6 {
        classification.Intent = "clarify"
        classification.NeedMore = append(classification.NeedMore, "intent")
    }

    return &classification, nil
}
```

#### 多轮上下文管理

```go
type ContextManager struct {
    // Redis 存储会话上下文
    redis *redis.Client

    // 上下文过期时间（默认 30 分钟）
    ttl time.Duration
}

type ConversationContext struct {
    ChannelID    string
    UserID       string
    Messages     []*ContextMessage  // 最近 N 条消息
    PendingTask  *PendingTaskState  // 正在进行中的操作（等待确认/追问）
    LastIntent   string             // 上次识别的意图
    LastEntitys  map[string]string  // 上次提取的实体
}

type PendingTaskState struct {
    Intent     string            // 待完成的意图
    Params     map[string]string // 已收集的参数
    MissingParams []string       // 待收集的参数
    CreatedAt  time.Time
    ExpiresAt  time.Time         // 超时自动取消
}

// 处理"对"、"是的"、"确认"等简短回复
func (cm *ContextManager) HandleConfirmation(msg *IMMessage) (*Action, error) {
    ctx := cm.GetContext(msg.ChannelID, msg.UserID)

    if ctx.PendingTask == nil {
        return nil, nil // 没有待确认的操作
    }

    // 检查是否是确认性回复
    if isConfirmation(msg.Content) {
        // 执行待确认的操作
        action := &Action{
            Intent: ctx.PendingTask.Intent,
            Params: ctx.PendingTask.Params,
        }
        ctx.PendingTask = nil
        cm.SaveContext(ctx)
        return action, nil
    }

    // 检查是否是取消性回复
    if isCancellation(msg.Content) {
        ctx.PendingTask = nil
        cm.SaveContext(ctx)
        return &Action{Intent: "cancelled"}, nil
    }

    // 否则作为参数补充处理
    return nil, nil
}

// 确认性回复的多语言检测
func isConfirmation(text string) bool {
    confirmPatterns := []string{
        "对", "是", "是的", "确认", "好", "好的", "行", "可以", "ok",
        "yes", "yep", "sure", "confirm", "right", "correct",
    }
    normalized := strings.TrimSpace(strings.ToLower(text))
    for _, p := range confirmPatterns {
        if normalized == p {
            return true
        }
    }
    return false
}
```

#### 命令消歧

```go
// 处理指代消歧（"这个"、"那个"、"上面的"）
func (ic *IntentClassifier) resolveReference(msg *IMMessage, ctx *ConversationContext) string {
    // 策略 1：查找最近的 issue/PR/task 引用
    // 如果上一条消息提到了 #42，那么"这个"大概率指 #42

    // 策略 2：查找 IM 消息中的引用/回复
    // 如果用户回复了一条包含 issue 链接的消息，"这个"指那个 issue

    // 策略 3：查找当前频道最近讨论的任务
    // 上下文窗口内最后被提及的任务

    // 策略 4：无法消歧 → 追问
    return ""
}
```

### 8.3 关键实现细节

- **LLM 模型选择**：意图分类使用 Haiku（快速 + 低成本），复杂的参数提取使用 Sonnet。分层调用减少总成本。
- **命令快捷方式缓存**：高频命令（如 `/task list`）直接正则匹配，不经过 LLM，响应时间 < 50ms。
- **群聊优化**：只处理 @AgentForge 或 / 前缀的消息，大幅减少 LLM 调用量。私聊模式下所有消息都处理。
- **响应格式适配**：不同 IM 平台的富文本格式不同（飞书 Markdown vs 钉钉 ActionCard vs Slack Block Kit）。cc-connect 的 platform 层已经抽象了这些差异，但响应内容构建需要感知平台特性。
- **限流保护**：每用户每分钟最多 20 条命令消息，防止刷屏导致 LLM 调用成本失控。

### 8.4 风险与缓解

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| 意图分类错误导致错误操作 | 中 | 中 | 关键操作前确认 + 低置信度追问 |
| 多轮上下文丢失 | 低 | 低 | Redis 持久化 + 30 分钟 TTL |
| 群聊误触发 | 低 | 低 | 严格触发条件 + @提及 |
| LLM 分类延迟影响 IM 体验 | 中 | 中 | Haiku 快速分类 + 快捷方式缓存 |

---

## 附录：挑战间的交叉依赖

```
┌─────────────────────────────────────────────────────────────────┐
│                        AgentForge 技术挑战依赖图                 │
│                                                                 │
│   IM 解析 ──→ 任务分解 ──→ Agent 生命周期 ──→ 多 Agent 隔离    │
│     (8)         (6)           (5)                (2)            │
│                  │             │                  │             │
│                  │             ▼                  ▼             │
│                  │         成本控制 ←────── 双栈通信             │
│                  │            (4)              (1)              │
│                  │             │                  │             │
│                  │             ▼                  ▼             │
│                  └──────→ WebSocket ←────── 安全沙箱           │
│                             (3)               (7)              │
└─────────────────────────────────────────────────────────────────┘

实施优先级建议（按依赖拓扑排序）：
  Phase 1 (MVP): 1 → 5 → 3 → 4 → 7(Layer 1) → 8(基础命令)
  Phase 2:       2 → 6 → 7(Layer 2) → 8(自然语言)
  Phase 3:       7(Layer 3) → 8(高级)
```

---

> **文档状态：** 初稿完成，待团队评审
>
> **关联文档：**
> - [PRD](../PRD.md)
> - [后端技术分析](./backend-tech-analysis.md)
> - [可复用项目调研](../REUSABLE_PROJECTS.md)
