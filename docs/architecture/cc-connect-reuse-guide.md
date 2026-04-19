# cc-connect Fork 与复用技术指南

> AgentForge IM Bridge 层设计文档 | 基于 cc-connect v1.2.x
>
> cc-connect: <https://github.com/chenhg5/cc-connect> | MIT License | 2.4k Stars | Go 99.4%

---

## 当前实现快照（2026-03-30）

这份指南的核心判断仍然成立：AgentForge 复用了 cc-connect 的 `platform/` 连接器价值，而不是复用它的本地 CLI agent 执行模型。但当前仓库真相已经比最初草案更进一步：

- 当前 IM Bridge 仓库已经有真实 platform registry 与 control-plane wiring，而不只是 Feishu-first 草图。
- 当前受支持的 operator-facing 平台集合已经覆盖 `feishu`、`dingtalk`、`slack`、`telegram`、`discord`、`wecom`、`qq`、`qqbot`。
- 与 AgentForge 后端的主通信模型已经收敛为 HTTP control-plane + WebSocket 事件流，而不是待选的 `HTTP+SSE vs gRPC vs WebSocket`。
- Feishu 的 delayed update / native card 能力、以及 QQ / QQ Bot / WeCom 的平台覆盖都已经进入当前仓库资产，而不是停留在后续列表里。

---

## 目录

1. [cc-connect 架构分析](#一cc-connect-架构分析)
2. [保留部分：直接复用的模块](#二保留部分直接复用的模块)
3. [替换部分：需要重写的模块](#三替换部分需要重写的模块)
4. [集成架构：Fork 后与 AgentForge 通信](#四集成架构fork-后与-agentforge-通信)
5. [IM 命令协议实现](#五im-命令协议实现)
6. [平台特殊考虑](#六平台特殊考虑)
7. [迁移策略](#七迁移策略)
8. [潜在陷阱与应对](#八潜在陷阱与应对)

---

## 一、cc-connect 架构分析

### 1.1 项目定位

cc-connect 是一个 **IM-to-Agent 桥接器**，将本地运行的 AI 编码 Agent（Claude Code、Codex、Gemini CLI 等）连接到 10 个聊天平台。它的核心是一个 **Hub-and-Spoke** 架构：多个 IM 平台适配器通过统一的 Engine 路由消息到 Agent 子进程。

### 1.2 目录结构总览

```
cc-connect/
├── cmd/cc-connect/        # 程序入口 main.go：配置加载、daemon 管理、启动/关闭
├── core/                  # 核心抽象层：接口定义、注册表、消息类型、Engine 路由引擎
│   ├── interfaces.go      # Platform / Agent / AgentSession 接口定义
│   ├── registry.go        # 插件注册表（PlatformFactory / AgentFactory）
│   ├── message.go         # 统一消息类型 Message / Event / ImageAttachment 等
│   ├── engine.go          # 中央路由引擎 Engine：消息过滤 → 命令分发 → Agent 交互
│   ├── session.go         # 会话管理：历史记录持久化（JSON 文件）
│   ├── i18n.go            # 国际化（5 种语言）
│   └── speech.go          # 语音转文字
├── platform/              # IM 平台适配器（10 个）
│   ├── feishu/            # 飞书/Lark —— WebSocket 长连接
│   ├── dingtalk/          # 钉钉 —— Stream 模式
│   ├── slack/             # Slack —— Socket Mode
│   ├── telegram/          # Telegram —— Long Polling
│   ├── discord/           # Discord —— Gateway
│   ├── wecom/             # 企业微信 —— WebSocket / Webhook
│   ├── wechat/            # 微信个人号(ilink) —— HTTP Long Polling (Beta)
│   ├── line/              # LINE —— Webhook（需公网 IP）
│   ├── qq/                # QQ (NapCat/OneBot) —— WebSocket
│   └── qqbot/             # QQ Bot 官方 —— WebSocket
├── agent/                 # AI Agent 适配器（7 个）
│   ├── claudecode/        # Claude Code CLI
│   ├── codex/             # OpenAI Codex CLI
│   ├── cursor/            # Cursor Agent
│   ├── gemini/            # Gemini CLI
│   ├── qoder/             # Qoder CLI
│   ├── opencode/          # OpenCode (Crush)
│   └── iflow/             # iFlow CLI
├── config/                # TOML 配置加载/保存/热重载
├── daemon/                # 守护进程管理（install/start/stop/status）
├── docs/                  # 各平台接入指南
├── tests/                 # 测试
└── config.example.toml    # 配置模板（1179 行，非常详尽）
```

### 1.3 核心接口

cc-connect 的架构围绕三个核心接口构建，定义在 `core/interfaces.go`：

**Platform 接口** — 抽象 IM 平台：

```go
type Platform interface {
    Name() string
    Start(handler MessageHandler) error  // 启动平台，注册消息处理器
    Reply(ctx context.Context, replyCtx any, content string) error
    Send(ctx context.Context, replyCtx any, content string) error
    Stop() error
}

// 消息处理回调
type MessageHandler func(p Platform, msg *Message)
```

**Agent 接口** — 抽象 AI Agent：

```go
type Agent interface {
    Name() string
    StartSession(ctx context.Context, sessionID string) (AgentSession, error)
    ListSessions(ctx context.Context) ([]AgentSessionInfo, error)
    Stop() error
}
```

**AgentSession 接口** — 运行中的 Agent 会话：

```go
type AgentSession interface {
    Send(prompt string, images []ImageAttachment, files []FileAttachment) error
    RespondPermission(requestID string, result PermissionResult) error
    Events() <-chan Event       // 流式事件通道
    CurrentSessionID() string
    Alive() bool
    Close() error
}
```

### 1.4 插件注册机制

cc-connect 使用工厂模式 + `init()` 注册，定义在 `core/registry.go`：

```go
// 工厂类型
type PlatformFactory func(opts map[string]any) (Platform, error)
type AgentFactory    func(opts map[string]any) (Agent, error)

// 全局注册表
var platformFactories = make(map[string]PlatformFactory)
var agentFactories    = make(map[string]AgentFactory)

func RegisterPlatform(name string, factory PlatformFactory) {
    platformFactories[name] = factory
}

func RegisterAgent(name string, factory AgentFactory) {
    agentFactories[name] = factory
}
```

每个平台适配器在 `init()` 中注册自己：

```go
// platform/feishu/feishu.go
func init() {
    core.RegisterPlatform("feishu", New)
}

func New(opts map[string]any) (core.Platform, error) {
    appID := opts["app_id"].(string)
    appSecret := opts["app_secret"].(string)
    // ... 初始化飞书客户端
}
```

### 1.5 消息流转全景

```
用户在飞书发消息
    ↓
platform/feishu/ 的 WebSocket 长连接接收事件
    ↓
将原始消息转换为 core.Message {
    SessionKey: "feishu:{chatID}:{userID}",
    Content:    "帮我修复 auth 模块的 bug",
    Images:     [...],
    ReplyCtx:   feishuSpecificContext,
}
    ↓
调用 handler(platform, message)  // handler = engine.handleMessage
    ↓
Engine.handleMessage():
    ├── AllowList 过滤（检查 allow_from 白名单）
    ├── RateLimit 限频
    ├── BannedWords 过滤
    ├── Alias 别名解析
    ├── 检测斜杠命令（/new, /model, /mode 等）
    │   └── 命令匹配 → 执行命令 → Reply 返回结果
    └── 非命令消息 → processInteractiveMessage()
            ├── 查找或创建 AgentSession
            ├── session.Send(prompt, images, files)
            ├── 监听 session.Events() 通道
            │   ├── EventThinking  → Reply("思考中...")
            │   ├── EventToolUse   → Reply("执行: Bash(ls)")
            │   ├── EventText      → Reply(内容)
            │   ├── EventResult    → Reply(最终结果)
            │   └── EventPermissionRequest → 发送权限确认按钮
            └── 完成后更新会话历史
```

### 1.6 统一消息类型

```go
type Message struct {
    SessionKey string            // "feishu:{chatID}:{userID}" 唯一标识
    Platform   string            // "feishu" / "telegram" / ...
    MessageID  string            // 平台消息 ID（用于追踪）
    UserID     string
    UserName   string
    ChatName   string            // 群名（可选）
    Content    string            // 文本内容
    Images     []ImageAttachment // 图片附件
    Files      []FileAttachment  // 文件附件
    Audio      *AudioAttachment  // 语音消息
    ReplyCtx   any               // 平台特定的回复上下文（不透明类型）
    FromVoice  bool              // 是否来自语音转写
}

type Event struct {
    Type      EventType  // text / tool_use / tool_result / result / error / permission_request / thinking
    Content   string
    ToolName  string
    SessionID string
    RequestID string     // 权限请求 ID
    Done      bool
    Error     error
}
```

---

## 二、保留部分：直接复用的模块

### 2.1 platform/ 层 — 10 个 IM 平台适配器

**这是 cc-connect 最核心的复用价值。** 每个适配器都是生产就绪的，处理了各平台 SDK 的认证、连接管理、消息收发、重连等复杂逻辑。

| 平台 | 连接方式 | 是否需要公网 IP | 优先级 | 备注 |
|------|---------|:---:|:---:|------|
| **飞书 (Feishu/Lark)** | WebSocket 长连接 | 否 | **P0** | 支持卡片消息、交互按钮、Thread 隔离 |
| **钉钉 (DingTalk)** | Stream 模式 | 否 | P1 | 国内企业广泛使用 |
| **Slack** | Socket Mode | 否 | P1 | 海外团队首选 |
| **Telegram** | Long Polling | 否 | P1 | 支持内联按钮、流式编辑 |
| **Discord** | Gateway | 否 | P2 | 支持 @everyone/@here |
| **企业微信 (WeCom)** | WebSocket/Webhook | 否(WS) | P1 | 国内企业常用 |
| **微信个人号 (Weixin)** | HTTP Long Polling (ilink) | 否 | P3 | Beta，不稳定 |
| **LINE** | Webhook | **是** | P3 | 日本/东南亚市场 |
| **QQ (NapCat/OneBot)** | WebSocket | 否 | P2 | 通过 OneBot 协议 |
| **QQ Bot 官方** | WebSocket | 否 | P2 | 官方 API |

**复用策略：原封不动保留所有 platform/ 代码。** 这些适配器只依赖 `core.Platform` 接口和 `core.Message` 类型，与 agent/ 层完全解耦。

### 2.2 platform/ 适配器可选接口

cc-connect 通过可选接口（Optional Interface Pattern）支持平台差异化能力：

```go
// 流式消息更新（Telegram、Discord、飞书支持）
type MessageUpdater interface {
    UpdateMessage(ctx context.Context, replyCtx any, content string) error
}

// 图片发送
type ImageSender interface {
    SendImage(ctx context.Context, replyCtx any, img ImageAttachment) error
}

// 文件发送
type FileSender interface {
    SendFile(ctx context.Context, replyCtx any, file FileAttachment) error
}

// 内联按钮（Telegram 支持）
type InlineButtonSender interface {
    SendWithButtons(ctx context.Context, replyCtx any, content string,
        buttons [][]ButtonOption) error
}

// 富文本卡片（飞书支持）
type CardSender interface {
    SendCard(ctx context.Context, replyCtx any, card *Card) error
    ReplyCard(ctx context.Context, replyCtx any, card *Card) error
}

// 卡片导航（飞书卡片内动态更新）
type CardNavigable interface {
    SetCardNavigationHandler(h CardNavigationHandler)
}

// 打字指示器
type TypingIndicator interface {
    StartTyping(ctx context.Context, replyCtx any) (stop func())
}
```

**对 AgentForge 的意义：** 这些可选接口让我们可以根据平台能力发送不同富度的消息。例如飞书可发送交互卡片（任务状态、审批按钮），Telegram 可发送内联按钮，不支持的平台自动回退纯文本。

### 2.3 daemon/ 层 — 守护进程

`daemon/` 包提供系统服务管理能力：

- `install` — 注册为系统服务
- `uninstall` — 卸载服务
- `start` / `stop` / `restart` — 生命周期管理
- `status` — 状态查询
- 日志文件管理（大小限制）

**复用策略：** 直接保留。AgentForge 的 IM Bridge 服务需要以守护进程方式运行。

### 2.4 config/ 层 — TOML 配置体系

cc-connect 的配置系统非常成熟（config.example.toml 1179 行），支持：

- 全局设置（语言、日志级别、速率限制）
- 多项目配置（`[[projects]]` 数组，每个项目独立绑定 agent + platforms）
- 热重载（`/reload` 命令运行时更新配置）
- 每项目独立 Agent + 多平台

**配置结构示例（保留的部分）：**

```toml
# 全局设置
language = "zh"
data_dir = "/var/lib/agentforge-bridge"

[log]
level = "info"

[rate_limit]
max_messages = 30
window_secs = 60

# 项目配置
[[projects]]
name = "my-team"
allow_from = "user1,user2,user3"    # 白名单（保留）
admin_from = "admin1"               # 管理员白名单（保留）

# Agent 配置 → 需要替换
[projects.agent]
type = "agentforge"                 # 替换为我们的 Agent 类型

[projects.agent.options]
api_base = "http://localhost:8080"  # AgentForge API 地址
project_id = "uuid-xxx"            # AgentForge 项目 ID
api_key = "af_xxx"                 # AgentForge API Key

# 平台配置 → 直接保留
[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "cli_axxxxxxxxxxxx"
app_secret = "QhkMpxxxxxxxxxxxxxxxxxxxx"
enable_feishu_card = true
thread_isolation = true

[[projects.platforms]]
type = "dingtalk"

[projects.platforms.options]
app_key = "dingxxxxxxxxx"
app_secret = "xxxxxxxxxxxxxxxxxx"
```

**复用策略：** 保留配置框架和平台配置部分，替换 `[projects.agent]` 的配置结构。

---

## 三、替换部分：需要重写的模块

### 3.1 agent/ 层 → HTTP 调用 AgentForge Orchestrator API

**原有逻辑：** cc-connect 的 agent/ 层通过 `exec.Command` 在本地启动 AI CLI 子进程（如 `claude`），通过 stdin/stdout 管道与子进程通信。

**替换为：** 不再本地启动 Agent，而是通过 AgentForge 后端的 HTTP control-plane 与 WebSocket 事件流完成消息投递、命令执行和状态回传。IM Bridge 变为一个纯粹的 **消息翻译层**，不运行任何 Agent 逻辑。

**新建 `agent/agentforge/agentforge.go`：**

```go
package agentforge

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "sync"
    "time"

    "github.com/chenhg5/cc-connect/core"
)

func init() {
    core.RegisterAgent("agentforge", New)
}

// Config 从 TOML 配置中读取
type Config struct {
    APIBase   string // AgentForge API 网关地址
    ProjectID string // 项目 ID
    APIKey    string // 认证 API Key
}

type Agent struct {
    config Config
    client *http.Client
}

func New(opts map[string]any) (core.Agent, error) {
    cfg := Config{
        APIBase:   opts["api_base"].(string),
        ProjectID: opts["project_id"].(string),
        APIKey:    opts["api_key"].(string),
    }
    return &Agent{
        config: cfg,
        client: &http.Client{Timeout: 30 * time.Second},
    }, nil
}

func (a *Agent) Name() string { return "agentforge" }

func (a *Agent) StartSession(ctx context.Context, sessionID string) (core.AgentSession, error) {
    return &Session{
        agent:     a,
        sessionID: sessionID,
        events:    make(chan core.Event, 100),
    }, nil
}

func (a *Agent) ListSessions(ctx context.Context) ([]core.AgentSessionInfo, error) {
    // 调用 AgentForge API 获取会话列表
    return nil, nil
}

func (a *Agent) Stop() error { return nil }

// Session 通过 HTTP/SSE 与 AgentForge 后端通信
type Session struct {
    agent     *Agent
    sessionID string
    events    chan core.Event
    mu        sync.Mutex
    closed    bool
}

func (s *Session) Send(prompt string, images []core.ImageAttachment, files []core.FileAttachment) error {
    // 构建请求
    body := map[string]any{
        "project_id":  s.agent.config.ProjectID,
        "session_key": s.sessionID,
        "content":     prompt,
        "source":      "im_bridge",
    }

    jsonBody, _ := json.Marshal(body)
    req, err := http.NewRequest("POST",
        s.agent.config.APIBase+"/api/v1/im/message",
        bytes.NewReader(jsonBody))
    if err != nil {
        return err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+s.agent.config.APIKey)

    // 使用 SSE 接收流式响应
    go s.streamResponse(req)
    return nil
}

func (s *Session) streamResponse(req *http.Request) {
    resp, err := s.agent.client.Do(req)
    if err != nil {
        s.events <- core.Event{Type: core.EventError, Error: err}
        return
    }
    defer resp.Body.Close()

    // 解析 SSE 流
    decoder := json.NewDecoder(resp.Body)
    for {
        var event core.Event
        if err := decoder.Decode(&event); err != nil {
            if err == io.EOF {
                s.events <- core.Event{Type: core.EventResult, Done: true}
                return
            }
            s.events <- core.Event{Type: core.EventError, Error: err}
            return
        }
        s.events <- event
    }
}

func (s *Session) RespondPermission(requestID string, result core.PermissionResult) error {
    // 转发权限决策到 AgentForge 后端
    body := map[string]any{
        "request_id": requestID,
        "result":     result,
    }
    jsonBody, _ := json.Marshal(body)
    req, _ := http.NewRequest("POST",
        s.agent.config.APIBase+"/api/v1/agents/permission",
        bytes.NewReader(jsonBody))
    req.Header.Set("Authorization", "Bearer "+s.agent.config.APIKey)
    _, err := s.agent.client.Do(req)
    return err
}

func (s *Session) Events() <-chan core.Event { return s.events }
func (s *Session) CurrentSessionID() string  { return s.sessionID }
func (s *Session) Alive() bool               { return !s.closed }
func (s *Session) Close() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.closed = true
    close(s.events)
    return nil
}
```

**关键变化：**

| 维度 | cc-connect 原有 | AgentForge Fork |
|------|----------------|-----------------|
| Agent 运行位置 | 本地子进程 | 远程 AgentForge 服务器 |
| 通信方式 | stdin/stdout 管道 | HTTP/SSE 或 gRPC |
| 会话管理 | 本地 JSON 文件 | AgentForge 后端数据库 |
| Agent 类型 | CLI 工具（claude, codex...） | AgentForge Agent 池 |
| 成本控制 | 无 | AgentForge 预算系统 |

### 3.2 core/ 层 → 重写为任务管理语义

**原有逻辑：** core/engine.go 的 Engine 将 IM 消息**直接转发**给 Agent 子进程，本质是一个聊天代理。

**替换为：** Engine 需要理解 AgentForge 任务管理语义，支持：

1. **斜杠命令** → 映射到 AgentForge API（`/task create` → `POST /api/v1/projects/:pid/tasks`）
2. **自然语言** → 先发送到 AI 意图识别，再路由到对应 API
3. **通知推送** → AgentForge 后端主动推送消息到 IM（任务状态变更、审查完成等）

**Engine 改造重点：**

```go
// core/engine.go 新增命令处理

func (e *Engine) registerAgentForgeCommands() {
    // 任务管理命令
    e.RegisterCommand("/task", e.handleTaskCommand)
    e.RegisterCommand("/agent", e.handleAgentCommand)
    e.RegisterCommand("/review", e.handleReviewCommand)
    e.RegisterCommand("/sprint", e.handleSprintCommand)

    // 保留原有命令
    // /new, /list, /switch, /mode, /model 等继续可用
}

func (e *Engine) handleTaskCommand(p core.Platform, msg *core.Message, args string) {
    parts := strings.SplitN(args, " ", 2)
    subCmd := parts[0]

    switch subCmd {
    case "create":
        // POST /api/v1/projects/:pid/tasks
        // 将 parts[1] 作为任务描述
    case "assign":
        // POST /api/v1/tasks/:id/assign
    case "status":
        // GET /api/v1/tasks/:id
    case "list":
        // GET /api/v1/projects/:pid/tasks?assignee=me
    case "decompose":
        // POST /api/v1/tasks/:id/decompose
    }
}
```

**保留 core/ 中的通用模块：**

- `core/interfaces.go` — Platform 接口不变，Agent 接口小改
- `core/registry.go` — 注册机制不变
- `core/message.go` — Message / Event 类型不变
- `core/session.go` — 本地会话历史可保留作为缓存
- `core/i18n.go` — 国际化保留
- `core/speech.go` — 语音转文字保留

**重写/大改的部分：**

- `core/engine.go` — 消息路由逻辑需要大改，添加命令系统和 API 调用
- 新增 `core/api_client.go` — AgentForge HTTP 客户端封装
- 新增 `core/notification.go` — 接收后端推送并转发到 IM

---

## 四、集成架构：Fork 后与 AgentForge 通信

### 4.1 整体架构

```
                        用户层
  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
  │ 飞书     │  │ 钉钉     │  │ Slack    │  │ Telegram │  ...
  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘
       │             │             │             │
  ┌────▼─────────────▼─────────────▼─────────────▼──────────┐
  │               cc-connect Fork (IM Bridge)                 │
  │                                                           │
  │  ┌─────────────────────────────────────────────────────┐ │
  │  │ platform/  (保留，10 个 IM 适配器原封不动)            │ │
  │  └────────────────────┬────────────────────────────────┘ │
  │                       │ core.Message                      │
  │  ┌────────────────────▼────────────────────────────────┐ │
  │  │ core/engine.go  (改造)                               │ │
  │  │  ├── 斜杠命令解析 (/task, /agent, /review, /sprint) │ │
  │  │  ├── @AgentForge 自然语言意图识别                    │ │
  │  │  └── 通知推送（从后端接收 → 转发到 IM）             │ │
  │  └────────────────────┬────────────────────────────────┘ │
  │                       │ HTTP / SSE / WebSocket            │
  │  ┌────────────────────▼────────────────────────────────┐ │
  │  │ agent/agentforge/  (新建)                            │ │
  │  │  ├── HTTP Client → AgentForge API Gateway            │ │
  │  │  ├── SSE 接收 Agent 实时输出                         │ │
  │  │  └── WebSocket 接收通知推送                          │ │
  │  └─────────────────────────────────────────────────────┘ │
  └─────────────────────────┬─────────────────────────────────┘
                            │
                            │ HTTP REST / SSE / WebSocket
                            │
  ┌─────────────────────────▼─────────────────────────────────┐
  │                 AgentForge API Gateway                      │
  │                 (Go · Fiber/Echo)                           │
  │                                                            │
  │  POST /api/v1/im/message    ← IM 消息入口                 │
  │  POST /api/v1/im/command    ← 斜杠命令入口                │
  │  POST /api/v1/im/send       ← 主动发消息到 IM             │
  │  POST /api/v1/im/notify     ← 发通知到 IM                 │
  │  POST /api/v1/im/action     ← 交互动作入口                │
  │  POST /api/v1/im/bridge/*   ← Bridge 控制面注册/心跳      │
  │  WS   /ws/im-bridge         ← 定向投递/回放/进度流        │
  └────────────────────────────────────────────────────────────┘
```

### 4.2 消息流向详解

**场景 A：用户通过 IM 发送自然语言指令**

```
用户在飞书: "@AgentForge 把用户认证模块拆解一下，简单的分给 Agent"
    ↓
飞书适配器 → core.Message{Content: "@AgentForge 把用户认证模块拆解一下..."}
    ↓
Engine.handleMessage()
    ├── 检测到 @AgentForge 前缀
    └── 调用 agent/agentforge 的 Session.Send()
            ↓
        POST /api/v1/im/message {
            "project_id": "uuid",
            "session_key": "feishu:{chatID}:{userID}",
            "content": "把用户认证模块拆解一下，简单的分给 Agent",
            "source": "im_bridge",
            "platform": "feishu",
            "user_id": "xxx",
            "user_name": "陈震烨"
        }
            ↓
AgentForge 后端:
    ├── AI 分析意图 → 任务分解
    ├── 创建 5 个子任务
    ├── 3 个标记 "适合 Agent"，2 个标记 "需要人工"
    └── 通过 IM Control Plane / WebSocket 推送事件流:
        Event{Type: "text", Content: "已拆解为 5 个子任务:\n1. ..."}
        Event{Type: "result", Done: true}
            ↓
IM Bridge 接收事件 → Engine 转发到飞书
    ├── 如果支持 CardSender → 发送飞书交互卡片（含"分配"按钮）
    └── 否则 → 发送纯文本
```

**场景 B：AgentForge 后端主动推送通知**

```
Agent 完成编码 → PR #87 已创建 → 审查通过
    ↓
AgentForge 后端:
    POST /api/v1/im/notify → 推送到 IM Bridge
    或通过 WebSocket /ws/im-bridge 推送 notification.new / progress 事件
    {
        "type": "review_complete",
        "target_session": "feishu:{chatID}:{userID}",
        "title": "PR #87 审查通过",
        "content": "任务「修复 auth token 刷新逻辑」的 PR 已通过审查...",
        "actions": [
            {"text": "查看 PR", "url": "https://github.com/..."},
            {"text": "合并", "data": "merge:pr:87"}
        ]
    }
    ↓
IM Bridge 接收通知 → 路由到对应平台
    ├── 飞书 → SendCard() 发送交互卡片（含"合并"按钮）
    ├── Telegram → SendWithButtons() 发送内联按钮
    └── 其他平台 → Reply() 发送纯文本 + URL
```

### 4.3 IM Bridge 注册接口

IM Bridge 启动时需要向 AgentForge 后端注册自己：

```go
// agent/agentforge/register.go

// RegisterBridge 在启动时向 AgentForge 后端注册 IM Bridge 实例
func (a *Agent) RegisterBridge(ctx context.Context) error {
    body := map[string]any{
        "bridge_id":  a.bridgeID,
        "platforms":  a.activePlatforms, // ["feishu", "dingtalk", "slack"]
        "callback":   a.callbackURL,     // 后端推送通知的回调地址
        "started_at": time.Now(),
    }
    // POST /api/v1/im/bridge/register
    return a.postJSON(ctx, "/api/v1/im/bridge/register", body)
}
```

### 4.4 双向通信实现

IM Bridge 需要同时支持：

1. **IM → AgentForge**（用户消息/命令）：HTTP POST 请求
2. **AgentForge → IM**（通知/Agent 输出）：两种方式
   - **SSE (Server-Sent Events)**：Agent 实时输出流
   - **WebSocket**：双向通信，接收通知推送

```go
// agent/agentforge/notification_listener.go

// ListenNotifications 启动 WebSocket 连接，接收后端推送
func (a *Agent) ListenNotifications(ctx context.Context, handler core.MessageHandler) {
    wsURL := strings.Replace(a.config.APIBase, "http", "ws", 1) + "/ws/v1/bridge"

    for {
        conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
            HTTPHeader: http.Header{
                "Authorization": []string{"Bearer " + a.config.APIKey},
                "X-Bridge-ID":   []string{a.bridgeID},
            },
        })
        if err != nil {
            slog.Error("WebSocket 连接失败，5秒后重试", "error", err)
            time.Sleep(5 * time.Second)
            continue
        }

        a.handleWebSocket(ctx, conn, handler)
    }
}

func (a *Agent) handleWebSocket(ctx context.Context, conn *websocket.Conn, handler core.MessageHandler) {
    defer conn.Close(websocket.StatusNormalClosure, "")

    for {
        _, data, err := conn.Read(ctx)
        if err != nil {
            return // 触发重连
        }

        var notification Notification
        json.Unmarshal(data, &notification)

        // 路由到对应的 IM 平台
        a.routeNotification(notification)
    }
}
```

---

## 五、IM 命令协议实现

### 5.1 命令总览

```
AgentForge IM 命令协议:

/task create <description>           创建任务
/task list [--mine|--agent|--all]    查看任务列表
/task status <id>                    查看任务状态
/task assign <id> @agent|@user       分配任务
/task decompose <id>                 AI 分解任务

/agent run <prompt>                  直接执行 Agent 指令
/agent status                        Agent 池状态
/agent logs <id>                     查看 Agent 日志

/review <pr-url>                     触发代码审查
/review status <id>                  审查状态

/sprint status                       Sprint 概览
/sprint burndown                     燃尽图

@AgentForge <自然语言>                AI 理解意图并执行
```

### 5.2 命令注册实现

```go
// core/agentforge_commands.go

func (e *Engine) registerAgentForgeCommands(apiClient *agentforge.Client) {
    // /task 命令
    e.RegisterCommand("/task", func(p core.Platform, msg *core.Message, args string) {
        parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
        subCmd := parts[0]
        subArgs := ""
        if len(parts) > 1 {
            subArgs = parts[1]
        }

        switch subCmd {
        case "create":
            resp, err := apiClient.CreateTask(msg.SessionKey, subArgs)
            if err != nil {
                p.Reply(context.Background(), msg.ReplyCtx,
                    fmt.Sprintf("创建任务失败: %v", err))
                return
            }
            // 尝试发送卡片消息
            if cs, ok := p.(core.CardSender); ok {
                card := buildTaskCard(resp)
                cs.ReplyCard(context.Background(), msg.ReplyCtx, card)
            } else {
                p.Reply(context.Background(), msg.ReplyCtx,
                    fmt.Sprintf("已创建任务 #%s: %s\n状态: %s\n优先级: %s",
                        resp.ID[:8], resp.Title, resp.Status, resp.Priority))
            }

        case "list":
            tasks, err := apiClient.ListTasks(msg.SessionKey, subArgs)
            if err != nil {
                p.Reply(context.Background(), msg.ReplyCtx,
                    fmt.Sprintf("获取任务列表失败: %v", err))
                return
            }
            p.Reply(context.Background(), msg.ReplyCtx, formatTaskList(tasks))

        case "assign":
            // 解析 "/task assign abc123 @agent"
            assignParts := strings.SplitN(subArgs, " ", 2)
            if len(assignParts) < 2 {
                p.Reply(context.Background(), msg.ReplyCtx,
                    "用法: /task assign <task-id> @agent|@用户名")
                return
            }
            err := apiClient.AssignTask(assignParts[0], assignParts[1])
            if err != nil {
                p.Reply(context.Background(), msg.ReplyCtx,
                    fmt.Sprintf("分配失败: %v", err))
                return
            }
            p.Reply(context.Background(), msg.ReplyCtx,
                fmt.Sprintf("已将任务 %s 分配给 %s", assignParts[0], assignParts[1]))

        case "status":
            task, err := apiClient.GetTask(subArgs)
            if err != nil {
                p.Reply(context.Background(), msg.ReplyCtx,
                    fmt.Sprintf("查询失败: %v", err))
                return
            }
            if cs, ok := p.(core.CardSender); ok {
                cs.ReplyCard(context.Background(), msg.ReplyCtx, buildTaskDetailCard(task))
            } else {
                p.Reply(context.Background(), msg.ReplyCtx, formatTaskDetail(task))
            }

        case "decompose":
            // 异步操作：AI 分解任务
            p.Reply(context.Background(), msg.ReplyCtx,
                "正在分解任务，请稍候...")
            go func() {
                subtasks, err := apiClient.DecomposeTask(subArgs)
                if err != nil {
                    p.Reply(context.Background(), msg.ReplyCtx,
                        fmt.Sprintf("分解失败: %v", err))
                    return
                }
                p.Reply(context.Background(), msg.ReplyCtx,
                    formatDecomposition(subtasks))
            }()
        }
    })

    // /agent 命令
    e.RegisterCommand("/agent", func(p core.Platform, msg *core.Message, args string) {
        parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
        switch parts[0] {
        case "run":
            // 创建任务 + 立即分配给 Agent
            resp, err := apiClient.QuickAgentRun(msg.SessionKey, parts[1])
            if err != nil {
                p.Reply(context.Background(), msg.ReplyCtx,
                    fmt.Sprintf("执行失败: %v", err))
                return
            }
            p.Reply(context.Background(), msg.ReplyCtx,
                fmt.Sprintf("Agent 任务已启动 (ID: %s)，我会在完成后通知你", resp.TaskID[:8]))
        case "status":
            status, _ := apiClient.GetAgentPoolStatus()
            p.Reply(context.Background(), msg.ReplyCtx, formatAgentPool(status))
        }
    })

    // /review 命令
    e.RegisterCommand("/review", func(p core.Platform, msg *core.Message, args string) {
        resp, _ := apiClient.TriggerReview(args)
        p.Reply(context.Background(), msg.ReplyCtx,
            fmt.Sprintf("审查已触发 (ID: %s)，预计 %d 分钟完成", resp.ID[:8], resp.EstMinutes))
    })

    // /sprint 命令
    e.RegisterCommand("/sprint", func(p core.Platform, msg *core.Message, args string) {
        switch strings.TrimSpace(args) {
        case "status":
            sprint, _ := apiClient.GetCurrentSprint()
            p.Reply(context.Background(), msg.ReplyCtx, formatSprintStatus(sprint))
        case "burndown":
            data, _ := apiClient.GetBurndown()
            // 生成燃尽图图片
            imgPath := renderBurndownChart(data)
            if is, ok := p.(core.ImageSender); ok {
                imgData, _ := os.ReadFile(imgPath)
                is.SendImage(context.Background(), msg.ReplyCtx, core.ImageAttachment{
                    MimeType: "image/png",
                    Data:     imgData,
                    FileName: "burndown.png",
                })
            }
        }
    })
}
```

### 5.3 @AgentForge 自然语言处理

当消息以 `@AgentForge` 开头时，整条消息发送到后端 AI 进行意图识别：

```go
func (e *Engine) handleNaturalLanguage(p core.Platform, msg *core.Message) {
    // 去掉 @AgentForge 前缀
    content := strings.TrimPrefix(msg.Content, "@AgentForge")
    content = strings.TrimSpace(content)

    // 发送到 AgentForge 后端的 AI 意图识别接口
    resp, err := e.apiClient.ProcessNaturalLanguage(msg.SessionKey, content, ProcessOptions{
        Platform: msg.Platform,
        UserID:   msg.UserID,
        UserName: msg.UserName,
        ChatName: msg.ChatName,
    })
    if err != nil {
        p.Reply(context.Background(), msg.ReplyCtx,
            fmt.Sprintf("处理失败: %v", err))
        return
    }

    // 后端会返回 SSE 事件流（和 Agent 执行一样的 Event 格式）
    // 由 agentforge Session 处理并流式转发到 IM
}
```

---

## 六、平台特殊考虑

### 6.1 飞书 (Feishu) — 当前最完整平台

飞书是 AgentForge MVP 的首选 IM 平台，原因：

- WebSocket 长连接，无需公网 IP
- 支持**交互卡片**（Interactive Card）：可嵌入按钮、表单、下拉选择
- 支持 Thread 隔离（`thread_isolation = true`）
- 国内企业使用率高
- cc-connect 的飞书适配器最成熟

**飞书卡片消息能力利用：**

```go
// 任务状态卡片
func buildTaskCard(task *Task) *core.Card {
    card := core.NewCard().
        SetTitle(fmt.Sprintf("任务 #%s", task.ID[:8])).
        AddField("标题", task.Title).
        AddField("状态", task.Status).
        AddField("分配给", task.AssigneeName).
        AddField("优先级", task.Priority)

    if task.Status == "inbox" {
        card.AddButton("分配给 Agent", "act:assign-agent:"+task.ID).
             AddButton("AI 分解", "act:decompose:"+task.ID)
    }

    if task.PRUrl != "" {
        card.AddButton("查看 PR", "link:"+task.PRUrl).
             AddButton("审批通过", "act:approve:"+task.ID).
             AddButton("要求修改", "act:request-changes:"+task.ID)
    }

    return card
}
```

**飞书卡片回调处理（CardNavigable 接口）：**

```go
// 飞书卡片按钮点击回调
func (e *Engine) handleCardAction(action string, sessionKey string) *core.Card {
    parts := strings.SplitN(action, ":", 3)
    switch parts[0] {
    case "act":
        switch parts[1] {
        case "assign-agent":
            taskID := parts[2]
            e.apiClient.AssignTask(taskID, "@agent")
            return buildTaskCard(/* 更新后的任务 */)
        case "decompose":
            taskID := parts[2]
            go e.apiClient.DecomposeTask(taskID)
            return core.NewCard().SetTitle("正在分解任务...").AddField("状态", "处理中")
        case "approve":
            taskID := parts[2]
            e.apiClient.ApproveReview(taskID)
            return core.NewCard().SetTitle("已审批通过").AddField("状态", "Approved")
        }
    }
    return nil
}
```

**飞书 Webhook 配置要点：**

1. 创建企业自建应用（个人开发者也可创建）
2. 获取 App ID + App Secret
3. 启用机器人能力
4. 配置权限：`contact:user.base:readonly`, `im:message.group:receive`, `im:message.p2p:receive`, `im:message:send_as_bot`
5. 事件订阅选择**长连接模式**
6. 添加 `im.message.receive_v1` 事件
7. 发布应用

cc-connect 提供一键配置命令：

```bash
cc-connect feishu setup --project my-team
```

### 6.2 钉钉 (DingTalk) — 当前已纳入平台覆盖

- Stream 模式（类似飞书 WebSocket），无需公网 IP
- 支持 ActionCard 卡片消息（可嵌入按钮）
- 企业内部应用需要管理员开通
- 消息长度限制 20,000 字符

### 6.3 Slack — 当前已纳入平台覆盖

- Socket Mode，无需公网 IP
- 支持 Block Kit（丰富的消息布局组件）
- 支持 Interactive Messages（按钮、菜单、表单）
- 消息长度限制 40,000 字符
- Markdown 使用 mrkdwn 方言（cc-connect 已通过 `FormattingInstructionProvider` 处理）

### 6.4 Telegram — 当前已纳入平台覆盖

- Long Polling，无需公网 IP
- 支持 Inline Keyboard（按钮）
- 支持消息编辑（流式更新）
- 消息长度限制 4,096 字符（较短，需要分段发送）
- Bot Token 通过 @BotFather 获取

### 6.5 企业微信 (WeCom) — 当前已纳入平台覆盖

- WebSocket 模式无需公网 IP
- 支持 Markdown 消息和 Template Card
- 国内企业使用率高，与钉钉互补

### 6.6 Discord — 当前已纳入平台覆盖

- Gateway 连接，无需公网 IP
- 支持 Embed（嵌入式富文本）和 Components（按钮、下拉菜单）
- 消息长度限制 2,000 字符
- 适合开发者社区

### 6.7 QQ / QQ Bot — 当前已纳入平台覆盖

- QQ (NapCat / OneBot) 已进入当前 platform registry，并区分 stub / live transport。
- QQ Bot 官方路径也已进入当前 platform registry，与其他平台一样由统一 control plane 管理。
- 这意味着平台覆盖范围已经不再是“飞书 MVP，其他待后续”，而是“前端和 control plane 已对齐多平台，只是不同平台的富消息能力深度不同”。

### 6.8 各平台消息格式差异汇总

| 平台 | 最大长度 | 富消息 | 按钮 | 消息编辑 | Markdown |
|------|:---:|------|:---:|:---:|------|
| 飞书 | 30K | Interactive Card | 是 | 是 | 标准 |
| 钉钉 | 20K | ActionCard | 是 | 否 | 标准 |
| Slack | 40K | Block Kit | 是 | 是 | mrkdwn |
| Telegram | 4K | - | Inline Keyboard | 是 | MarkdownV2 |
| Discord | 2K | Embed | Components | 是 | 标准 |
| 企微 | 10K | Template Card | 是 | 否 | 标准 |

---

## 七、迁移策略

### 7.1 分步迁移计划

```
Phase 1: Fork + 最小化改造（1 周）
├── Fork cc-connect 仓库
├── 删除 agent/ 下所有原有适配器（claudecode/ codex/ 等）
├── 新建 agent/agentforge/ HTTP 客户端适配器
├── 保留 platform/ 全部代码
├── 保留 daemon/ 和 config/ 全部代码
├── 修改 cmd/cc-connect/main.go 的 import（移除原有 agent，添加 agentforge）
└── 验证：飞书消息 → Bridge → 简单 HTTP Echo → 飞书回复

Phase 2: 命令系统 + 任务管理（1 周）
├── 在 core/engine.go 中注册 AgentForge 命令
├── 实现 /task create|list|status|assign|decompose
├── 实现 /agent run|status
├── 实现 @AgentForge 自然语言转发
└── 验证：飞书 /task create → AgentForge API → 任务创建成功

Phase 3: 通知推送 + 富消息（1 周）
├── 实现 WebSocket 通知监听
├── 实现飞书交互卡片（任务状态、审批按钮）
├── 实现 Telegram 内联按钮
├── 实现卡片回调处理
└── 验证：Agent 完成任务 → 飞书收到卡片通知 → 点击"合并"按钮

Phase 4: 扩展其他平台（持续）
├── 钉钉 ActionCard 适配
├── Slack Block Kit 适配
├── 企微 Template Card 适配
└── 验证：多平台消息收发正常
```

### 7.2 具体操作步骤

**Step 1: Fork 仓库**

```bash
git clone https://github.com/chenhg5/cc-connect.git agentforge-im-bridge
cd agentforge-im-bridge

# 修改 go.mod 模块名
sed -i 's|github.com/chenhg5/cc-connect|github.com/agentforge/im-bridge|g' go.mod
# 全局替换导入路径
find . -name "*.go" -exec sed -i 's|github.com/chenhg5/cc-connect|github.com/agentforge/im-bridge|g' {} \;
```

**Step 2: 剥离原有 Agent 层**

```bash
# 删除所有原有 agent 适配器
rm -rf agent/claudecode agent/codex agent/cursor agent/gemini
rm -rf agent/qoder agent/opencode agent/iflow

# 新建 AgentForge Agent 适配器
mkdir -p agent/agentforge
# 创建 agentforge.go（见上文代码示例）
```

**Step 3: 修改 main.go 导入**

```go
// cmd/cc-connect/main.go
import (
    // 移除原有 agent 导入
    // _ "github.com/agentforge/im-bridge/agent/claudecode"
    // _ "github.com/agentforge/im-bridge/agent/codex"

    // 添加 AgentForge agent
    _ "github.com/agentforge/im-bridge/agent/agentforge"

    // 保留所有 platform 导入
    _ "github.com/agentforge/im-bridge/platform/feishu"
    _ "github.com/agentforge/im-bridge/platform/dingtalk"
    _ "github.com/agentforge/im-bridge/platform/slack"
    _ "github.com/agentforge/im-bridge/platform/telegram"
    _ "github.com/agentforge/im-bridge/platform/discord"
    _ "github.com/agentforge/im-bridge/platform/wecom"
    _ "github.com/agentforge/im-bridge/platform/qq"
    _ "github.com/agentforge/im-bridge/platform/qqbot"
    _ "github.com/agentforge/im-bridge/platform/line"
    _ "github.com/agentforge/im-bridge/platform/wechat"
)
```

**Step 4: 更新配置模板**

```toml
# config.example.toml
[[projects]]
name = "my-team"
allow_from = "*"

[projects.agent]
type = "agentforge"

[projects.agent.options]
api_base = "http://localhost:8080"
project_id = "your-project-uuid"
api_key = "af_your_api_key"

[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "cli_axxxxxxxxxxxx"
app_secret = "QhkMpxxxxxxxxxxxxxxxxxxxx"
enable_feishu_card = true
```

**Step 5: 构建验证**

```bash
make build
./cc-connect -config config.toml
# 检查日志输出：
# level=INFO msg="platform started" project=my-team platform=feishu
# level=INFO msg="cc-connect is running" projects=1
```

### 7.3 保持上游同步

cc-connect 社区活跃（483 commits, 221 forks），平台适配器会持续更新。建议：

```bash
# 添加上游 remote
git remote add upstream https://github.com/chenhg5/cc-connect.git

# 定期同步 platform/ 目录的更新
git fetch upstream
git checkout -b sync-upstream upstream/main

# 只合并 platform/ 和 daemon/ 的变更
git checkout main
git checkout sync-upstream -- platform/ daemon/
git commit -m "sync: update platform adapters from upstream cc-connect"
```

---

## 八、潜在陷阱与应对

### 8.1 IM 平台 API 限频

| 平台 | 限频策略 | 影响 | 应对 |
|------|---------|------|------|
| **飞书** | 应用级别 50 QPS，消息 API 5 QPS/用户 | Agent 长输出流式更新可能触发 | 控制 StreamPreview 更新频率（默认 1500ms 间隔） |
| **钉钉** | 企业内部应用 40 次/秒 | 群消息通知可能触发 | 消息合并，延迟发送 |
| **Slack** | Web API 1 req/sec（Tier 2），Socket Mode 无限制 | 消息更新触发 | 使用 Socket Mode 接收，控制 Web API 调用频率 |
| **Telegram** | 同一聊天 1 msg/sec，广播 30 msg/sec | 长输出分段发送触发 | 消息合并，编辑替代发送 |
| **Discord** | 全局 50 req/sec，通道 5 msg/sec | 多通道通知触发 | 消息队列缓冲 |

**通用应对策略：**

```go
// core/rate_limiter.go
// cc-connect 已内建速率限制，配置项：
[rate_limit]
max_messages = 20    # 每个窗口期最大消息数
window_secs = 60     # 窗口期秒数

[stream_preview]
interval_ms = 1500     # 流式预览最小更新间隔
min_delta_chars = 30   # 最少新增字符数才触发更新
max_chars = 2000       # 预览最大长度
```

### 8.2 消息格式差异

**陷阱：** 同一条消息在不同平台的 Markdown 渲染差异很大。

| 场景 | 飞书 | Slack | Telegram | Discord |
|------|------|-------|----------|---------|
| 代码块 | `` ```go `` | `` ```go `` | `` ```go `` | `` ```go `` |
| 粗体 | `**text**` | `*text*` | `*text*` | `**text**` |
| 链接 | `[text](url)` | `<url\|text>` | `[text](url)` | `[text](url)` |
| @用户 | `<at id="xxx"/>` | `<@U123>` | `@username` | `<@123>` |

**cc-connect 的解决方案：**

cc-connect 通过 `FormattingInstructionProvider` 接口让各平台提供格式化指导：

```go
// Slack 平台会返回 mrkdwn 格式说明
type FormattingInstructionProvider interface {
    FormattingInstructions() string
}
```

Engine 会将格式指导注入到 Agent 的系统提示中，确保输出符合目标平台格式。

**AgentForge 额外策略：** 在 IM Bridge 端做消息格式统一转换，而不是依赖 Agent 输出正确格式。

### 8.3 各平台认证流程差异

| 平台 | 认证方式 | 注意事项 |
|------|---------|---------|
| **飞书** | App ID + App Secret，可一键 `cc-connect feishu setup` | 个人开发者可创建，无需企业认证 |
| **钉钉** | App Key + App Secret，需企业管理后台开通 | 需企业管理员权限，测试用可创建组织 |
| **Slack** | Bot Token + App Token (Socket Mode) | 需创建 Slack App，启用 Socket Mode |
| **Telegram** | Bot Token (via @BotFather) | 最简单，分钟级接入 |
| **Discord** | Bot Token (Developer Portal) | 需启用 MESSAGE_CONTENT Intent |
| **企微** | Corp ID + Agent ID + Secret | 需企业管理员权限 |
| **QQ Bot** | App ID + App Secret | 需 QQ 开放平台审核 |

**建议：** MVP 阶段只接入飞书（P0），Telegram 作为调试辅助（最快接入），后续再扩展。

### 8.4 Webhook 安全

**陷阱：** 如果 IM Bridge 暴露 HTTP 端点接收后端通知，需要防止伪造请求。

```go
// cc-connect 的 Webhook 安全实现可参考：
[webhook]
enabled = true
port = 9111
token = "your-secret-token"    # 共享密钥认证
path = "/hook"

// 请求验证
req.Header.Get("Authorization") == "Bearer " + token
```

**AgentForge 应对：**

1. **共享密钥：** Bridge 和后端使用相同 API Key 互相认证
2. **内网部署：** Bridge 和后端在同一内网，不暴露到公网
3. **请求签名：** 后端推送通知时附带 HMAC 签名
4. **TLS：** 生产环境使用 HTTPS

### 8.5 长连接断线重连

**陷阱：** WebSocket/Long Polling 连接可能因网络波动断开，消息可能丢失。

cc-connect 已内建重连机制：

```go
// 飞书适配器自动重连
// Telegram Long Polling 自动重试
// Slack Socket Mode 自动重连
// Discord Gateway Resume
```

**AgentForge 额外考虑：**

- IM Bridge 与 AgentForge 后端的 WebSocket 也需要重连机制
- 断线期间的通知需要缓存，重连后补发
- 使用 Redis Streams 作为通知消息的持久化队列

### 8.6 多实例部署

**陷阱：** 如果部署多个 IM Bridge 实例，同一条消息可能被多个实例处理。

**应对：**

- 每个 Bridge 实例注册唯一 `bridge_id`
- 后端通知指定目标 `bridge_id`
- 或使用 Redis 分布式锁确保消息只被处理一次
- cc-connect 的 Management API（端口 9820）可用于多实例协调

### 8.7 Agent 执行超时

**陷阱：** Agent 编码任务可能执行很长时间（数十分钟），IM 平台可能超时。

```toml
# cc-connect 已有超时配置
[display]
idle_timeout_mins = 120   # Agent 空闲超时（默认 2 小时）
```

**AgentForge 策略：**

1. Agent 启动后立即回复 IM："任务已开始，我会在完成后通知你"
2. 通过 SSE/WebSocket 持续推送进度（每 30 秒一次心跳）
3. 完成后通过通知推送结果
4. 用户可随时 `/task status <id>` 查询进度

---

## 附录

### A. cc-connect 原有斜杠命令（保留）

| 命令 | 功能 | 保留 |
|------|------|:---:|
| `/new [name]` | 新建会话 | 是 |
| `/list` | 列出所有会话 | 是 |
| `/switch <id>` | 切换会话 | 是 |
| `/current` | 当前会话信息 | 是 |
| `/mode` | 权限模式管理 | 改造 |
| `/model` | 模型切换 | 改造 |
| `/quiet` | 静默模式切换 | 是 |
| `/memory` | Agent 记忆管理 | 改造 |
| `/cron` | 定时任务 | 是 |
| `/reload` | 热重载配置 | 是 |
| `/dir` | 工作目录管理 | 移除 |
| `/shell` | Shell 命令 | 移除 |

### B. AgentForge API Client 接口

```go
// agent/agentforge/client.go

type Client struct {
    baseURL string
    apiKey  string
    http    *http.Client
}

// 任务管理
func (c *Client) CreateTask(sessionKey, description string) (*Task, error)
func (c *Client) ListTasks(sessionKey, filter string) ([]Task, error)
func (c *Client) GetTask(taskID string) (*Task, error)
func (c *Client) AssignTask(taskID, assignee string) error
func (c *Client) DecomposeTask(taskID string) ([]Task, error)

// Agent 操作
func (c *Client) QuickAgentRun(sessionKey, prompt string) (*AgentRunResponse, error)
func (c *Client) GetAgentPoolStatus() (*AgentPoolStatus, error)

// 审查
func (c *Client) TriggerReview(prURL string) (*ReviewResponse, error)
func (c *Client) ApproveReview(taskID string) error

// Sprint
func (c *Client) GetCurrentSprint() (*Sprint, error)
func (c *Client) GetBurndown() (*BurndownData, error)

// 自然语言处理
func (c *Client) ProcessNaturalLanguage(sessionKey, content string, opts ProcessOptions) (*Stream, error)
```

### C. 参考链接

| 资源 | URL |
|------|-----|
| cc-connect 仓库 | <https://github.com/chenhg5/cc-connect> |
| cc-connect 配置模板 | <https://github.com/chenhg5/cc-connect/blob/main/config.example.toml> |
| 飞书接入指南 | <https://github.com/chenhg5/cc-connect/blob/main/docs/feishu.md> |
| 飞书开放平台 | <https://open.feishu.cn/> |
| 钉钉开放平台 | <https://open.dingtalk.com/> |
| Slack API | <https://api.slack.com/> |
| Telegram Bot API | <https://core.telegram.org/bots/api> |
| Discord 开发者文档 | <https://discord.com/developers/docs> |
| AgentForge PRD | 见 /PRD.md |

---

> **文档状态：** 已根据当前仓库实现做过一轮 live contract 同步，但仍包含部分长期架构与迁移草案。
>
> **当前仍值得单独评审的点：**
>
> 1. 多 Bridge 实例部署时，control-plane 和通知补发应继续沿现有后端注册/心跳模型演进，还是引入额外协调层
> 2. 是否保留 cc-connect 的部分管理/Webhook 能力作为辅助运维入口，而不是主业务入口
> 3. 各平台富消息能力矩阵是否要进一步文档化为统一 rendering profile / downgrade contract
