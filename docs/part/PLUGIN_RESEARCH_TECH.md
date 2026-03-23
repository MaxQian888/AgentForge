# AgentForge 插件系统技术实现方案调研

> 调研时间: 2026-03-23
> 架构背景: Go Orchestrator + TS Agent SDK Bridge，gRPC 通信，PostgreSQL + Redis

---

## 目录

1. [Go 侧插件运行时](#1-go-侧插件运行时)
2. [TS 侧插件运行时](#2-ts-侧插件运行时)
3. [跨语言插件方案](#3-跨语言插件方案)
4. [插件协议设计](#4-插件协议设计)
5. [插件分发和注册](#5-插件分发和注册)
6. [安全沙箱对比](#6-安全沙箱对比)
7. [开源参考实现](#7-开源参考实现)
8. [综合对比矩阵](#8-综合对比矩阵)
9. [推荐方案](#9-推荐方案)

---

## 1. Go 侧插件运行时

### 1.1 Go Native Plugin Package

Go 标准库 `plugin` 包，通过 `plugin.Open()` 加载 `.so` 共享库。

**优点：**
- 原生支持，无第三方依赖
- 直接函数调用，零 RPC 开销
- 共享内存空间，数据传递高效

**缺点：**
- **平台限制严重**: 仅支持 Linux、FreeBSD、macOS，不支持 Windows
- **版本耦合**: 插件和宿主必须用完全相同的 Go 版本、build tags、依赖版本编译
- **无法卸载**: 加载后无法 unload，内存无法释放
- **CGO 依赖**: 需要 CGO 编译工具链
- **Race Detector 支持差**: 竞态检测不可靠
- **安全性低**: 插件拥有宿主进程的完整内存访问权限
- **实际等于单体编译**: 所有限制意味着插件必须与宿主同时构建

**结论**: 不推荐。限制太多，不适合分布式插件生态。

### 1.2 HashiCorp go-plugin

HashiCorp 开发的 gRPC 插件框架，被 Terraform、Vault、Nomad、Boundary、Waypoint 广泛使用。插件作为独立子进程运行，通过 gRPC/net-rpc 通信。

**架构原理：**
```
┌─────────────┐     gRPC/localhost     ┌─────────────┐
│  Host App   │ ◄──────────────────► │  Plugin      │
│  (Client)   │     Protobuf msgs     │  (Server)    │
│             │                        │  子进程       │
└─────────────┘                        └─────────────┘
     │                                       │
     ├─ plugin.Client 启动子进程               ├─ plugin.Serve 注册 gRPC 服务
     ├─ MuxBroker 多路复用                     ├─ 实现 Go interface
     └─ GRPCBroker 连接代理                    └─ TLS 安全通信
```

**核心特性：**
- 插件是 Go interface 的实现，开发体验自然
- 进程隔离: 插件崩溃不影响宿主
- gRPC 支持跨语言 (任何支持 gRPC 的语言均可编写插件)
- MuxBroker 支持复杂参数（io.Reader/Writer 等接口传递）
- 内置 TLS 支持
- 协议版本协商 (major version 在 protobuf package name 中编码)
- 内置健康检查 (grpc_health_v1)
- Stdout/Stderr 流式转发

**缺点：**
- 每个插件一个进程，资源开销较大
- 需要实现 client/server 两端代码
- 插件分发需要编译好的二进制文件
- 非 Go 语言编写插件需重新实现协议层

**适用场景**: 需要稳定、成熟、进程隔离的 Go 插件系统。

**代码示例：**
```go
// 定义插件接口
type ToolPlugin interface {
    Execute(ctx context.Context, input *ToolInput) (*ToolOutput, error)
    Describe() *ToolDescription
}

// 插件侧 - 实现 gRPC server
type MyToolPlugin struct{}
func (p *MyToolPlugin) Execute(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
    return &ToolOutput{Result: "done"}, nil
}
func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: handshake,
        Plugins: map[string]plugin.Plugin{
            "tool": &ToolGRPCPlugin{Impl: &MyToolPlugin{}},
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}

// 宿主侧 - 启动插件
client := plugin.NewClient(&plugin.ClientConfig{
    HandshakeConfig: handshake,
    Plugins:         pluginMap,
    Cmd:             exec.Command("./my-plugin"),
    AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
})
rpcClient, _ := client.Client()
raw, _ := rpcClient.Dispense("tool")
tool := raw.(ToolPlugin)
result, _ := tool.Execute(ctx, input)
```

### 1.3 Yaegi (Go 解释器)

Traefik 团队开发的 Go 解释器，可在运行时动态执行 Go 源代码。

**优点：**
- **热加载/热更新**: 修改源码即生效，无需重编译
- **同语言脚本**: 插件用标准 Go 语法编写，无语言切换成本
- **跨平台**: 不依赖 CGO，任何 Go 支持的平台均可运行
- **沙箱控制**: 可限制插件访问的包和系统调用
- **简单 API**: 仅 `New()`, `Use()`, `Eval()` 三个核心 API
- **性能可接受**: gzip handler 解释执行仅比编译版慢 <10%

**缺点：**
- **计算密集场景慢**: 纯计算性能约为编译版的 1/4~1/5
- **不支持泛型**: Go 泛型语法尚未支持
- **不支持 Go Modules**: 模块系统不可用
- **第三方库受限**: 需预定义可访问的包符号表
- **动态接口限制**: 预编译代码使用的接口不能动态添加
- **类型表示差异**: reflect 和 %T 在编译/解释模式下可能不一致

**适用场景**: 轻量级脚本扩展、中间件插件、配置驱动的逻辑定制。

### 1.4 WASM 运行时 (wazero / wasmtime-go)

#### wazero (推荐)

纯 Go 实现的 WebAssembly 运行时，零外部依赖。

**优点：**
- **零依赖**: 纯 Go 实现，无 CGO，保持 Go 交叉编译能力
- **强安全沙箱**: 线性内存边界检查、能力模型、无直接 syscall
- **跨语言插件**: 任何编译到 WASM 的语言 (Rust, C/C++, Go, AssemblyScript, TinyGo) 均可编写插件
- **JIT 支持**: amd64/arm64 上有 JIT 编译器，性能可观
- **小体积**: ~5.5 MB ROM 占用
- **广泛平台**: Windows, macOS, Linux, FreeBSD, NetBSD, BSDs, illumos, Solaris
- **Go 惯用 API**: context 传播、并发安全

**缺点：**
- CPU 密集任务性能不如原生
- WASM 当前为单线程架构，不支持并行
- WASI 网络支持不完整 (wasip1 无完整 socket)
- 插件开发需 WASM 工具链 (对非 Rust/Go 开发者有门槛)

#### wasmtime-go

Rust 编写的 Wasmtime 的 Go 绑定，通过 CGO 调用。

**对比 wazero：**

| 维度 | wazero | wasmtime-go |
|------|--------|-------------|
| 依赖 | 零 (纯 Go) | CGO + Rust C API |
| 交叉编译 | 容易 | 困难 |
| CPU 密集性能 | 良好 | 略优 |
| 二进制体积 | ~5.5 MB | ~2-38 MB |
| Go 惯用性 | 原生 Go API | C API 包装 |
| 平台 | 极广 | Win/Mac/Linux/Android |

**结论**: 对 Go 项目而言，**wazero 是首选**，除非有极端 CPU 性能需求。

---

## 2. TS 侧插件运行时

### 2.1 isolated-vm (V8 隔离)

利用 V8 的 Isolate 机制在 Node.js 中创建安全隔离的 JavaScript 执行环境。

**优点：**
- **强安全隔离**: 无 Node.js API 访问，独立 GC 和堆
- **内存限制**: 内置堆溢出 DoS 防护
- **执行超时**: 可设置最大执行时间
- **Reference 对象**: 支持跨 isolate 的函数引用
- **多线程执行**: 利用 V8 多线程能力

**缺点：**
- **原生编译依赖**: 需要 C++ 编译器安装
- **Node 20+ 兼容问题**: 需传 `--no-node-snapshot` 参数
- **Linux 发行版问题**: 部分发行版会剥离 Node.js "内部"符号导致模块不可用
- **API 受限**: 只能执行纯 JS，无法直接使用 Node.js 模块

**适用场景**: 需要在 TS 服务中安全执行不可信 JS 代码。

### 2.2 Worker Threads (Node.js 原生)

Node.js 内置的多线程 API，每个 Worker 运行在独立 V8 isolate 中。

**优点：**
- **零额外依赖**: Node.js 内置
- **完整 Node.js 环境**: Worker 中可使用完整 Node.js API
- **结构化克隆**: 支持 ArrayBuffer 等可转移对象的零拷贝传递
- **稳定兼容**: 不受 Node.js 版本升级影响

**缺点：**
- **安全隔离弱**: Worker 拥有完整 Node.js 权限 (文件系统、网络等)
- **无内存限制**: 需手动管理
- **无执行超时**: 需外部机制控制
- **通信开销**: `postMessage` 序列化/反序列化大对象较慢
- **不适合不可信代码**: 仅适合并行执行可信插件

**适用场景**: 可信插件的并行执行、计算密集任务分流。

### 2.3 QuickJS / WASM 沙箱

在 Node.js 中通过 WebAssembly 运行 QuickJS 引擎，创建完全隔离的 JS 执行环境。

**代表项目**: `@sebastianwessel/quickjs`

**优点：**
- **极强隔离**: QuickJS 编译为 WASM，运行在 WASM 沙箱中，双重隔离
- **内存/时间限制**: 内置资源限制
- **虚拟文件系统**: 可挂载虚拟 FS
- **支持 TypeScript**: 可在沙箱中执行 TS 代码
- **自定义模块**: 可挂载自定义 Node 模块
- **Fetch 客户端**: 可提供受控的 HTTP 访问
- **超小体积**: MicroQuickJS 仅需 ~100 KB ROM, ~10 KB RAM

**缺点：**
- **性能较低**: QuickJS 是解释器，比 V8 JIT 慢数十倍
- **ES 标准支持滞后**: 不一定支持最新 ES 特性
- **生态较小**: 社区和维护不如 V8 方案成熟
- **需谨慎配置**: 暴露 Fetch/FS 可能破坏沙箱安全性

**适用场景**: 执行简单的用户自定义逻辑、规则引擎、数据转换脚本。

### 2.4 MCP Server 协议 (stdio/Streamable HTTP)

将插件作为 MCP Server 运行，通过标准 MCP 协议通信。

**优点：**
- **AI 原生**: 专为 AI 工具扩展设计，与 Agent 系统天然契合
- **标准化**: Anthropic 主导的开放标准，工具/资源/提示三大原语
- **语言无关**: 任何语言均可实现 MCP Server
- **传输灵活**: 本地用 stdio (零网络开销)，远程用 Streamable HTTP
- **进程隔离**: 每个 MCP Server 独立进程
- **生态丰富**: 已有大量现成 MCP Server 可复用

**缺点：**
- **协议开销**: JSON-RPC 2.0 序列化/反序列化
- **单连接模型**: 一个 MCP Client 对应一个 MCP Server
- **发现机制不够完善**: 缺乏标准的服务发现和注册
- **安全模型不够完善**: 依赖传输层安全，无内置沙箱

**适用场景**: Agent 工具扩展、外部数据源接入、AI 能力增强。

### TS 侧方案对比

| 维度 | isolated-vm | Worker Threads | QuickJS/WASM | MCP Server |
|------|-------------|---------------|--------------|------------|
| 安全隔离 | 强 (V8 isolate) | 弱 (完整 Node API) | 极强 (WASM 沙箱) | 强 (进程隔离) |
| 性能 | 高 (V8 JIT) | 高 | 低 (解释器) | 中 (IPC 开销) |
| 依赖 | C++ 编译器 | 无 | 无 | 无 |
| 不可信代码 | 适合 | 不适合 | 适合 | 适合 |
| 语言支持 | 仅 JS | 仅 JS/TS | JS/TS | 任意语言 |
| Agent 集成 | 需封装 | 需封装 | 需封装 | 天然适配 |

---

## 3. 跨语言插件方案

### 3.1 WASM 统一运行时 (Extism)

Extism 是跨语言 WASM 插件框架，提供统一的宿主-插件接口。

**架构：**
```
┌──────────────────────────────────────────┐
│              Host Application            │
│  (Go / Node.js / Python / Rust / ...)    │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │         Extism Host SDK            │  │
│  │   (wraps Wasmtime/Wazero/V8)      │  │
│  │                                    │  │
│  │  ┌──────────┐  ┌──────────┐      │  │
│  │  │ Plugin A │  │ Plugin B │      │  │
│  │  │ (Rust)   │  │ (Go/TS)  │      │  │
│  │  │ .wasm    │  │ .wasm    │      │  │
│  │  └──────────┘  └──────────┘      │  │
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘
```

**核心特性：**
- **Host SDK**: Go, Rust, Node.js, Python, Ruby, PHP, .NET, Java, Zig 等
- **Plugin PDK**: Rust, Go, C/C++, AssemblyScript, Haskell, Zig 等
- **bytes-in/bytes-out 接口**: 支持 JSON, Protobuf, Cap'n Proto 等序列化
- **持久化变量**: 模块级状态存储
- **HTTP 客户端**: 宿主控制的安全 HTTP 访问
- **资源限制**: 运行时计时器和限制器
- **宿主函数**: 宿主可向插件暴露自定义函数

**实际使用者**: Navidrome (音乐服务器), moonrepo (构建工具), Zed (编辑器)

**对 AgentForge 的意义：**
- Go Orchestrator 用 Extism Go SDK 加载 WASM 插件
- TS Bridge 用 Extism JS SDK 加载同一 WASM 插件
- 同一个 .wasm 文件双侧复用

### 3.2 WasmEdge

CNCF 项目，面向云原生/边缘的高性能 WASM 运行时。

**特色：**
- 比 Linux 容器启动快 100x，运行时快 20%
- 内置 JS 运行时 (可执行 ES6/CJS/NPM 模块)
- 网络 socket、Postgres/MySQL 驱动扩展
- Kubernetes 集成 (crun / containerd-shim via runwasi)
- LlamaEdge: 内置 GenAI 模型支持 (LLM, TTS, STT)
- 插件系统: 通过共享库加载原生宿主函数扩展
- 内存占用低至 2MB

**缺点：**
- 生态不如 Extism 成熟
- 插件系统偏底层 (C/C++ 共享库)
- SDK 覆盖语言少于 Extism

### 3.3 gRPC 插件协议

使用 gRPC 作为统一的跨语言插件通信协议。

**优点：**
- **成熟稳定**: 生产级协议，性能经过充分验证
- **跨语言**: 支持几乎所有主流编程语言
- **类型安全**: Protobuf 强类型定义
- **流式支持**: 支持双向流
- **与 AgentForge 架构一致**: 已使用 gRPC 做 Go-TS 通信

**缺点：**
- 每个插件需实现 gRPC server
- 二进制分发 (需为每个平台编译)
- 连接管理复杂度

**结合 Buf Connect：**
Connect (by Buf) 提供了更现代的 gRPC 开发体验:
- Go: 直接嵌入 net/http server，无需 gRPC 专用服务器
- TypeScript: 类型安全客户端/服务端，原生 fetch API 集成
- 同时支持 gRPC、gRPC-Web、Connect 三种协议
- 无需 Envoy 代理即可处理 gRPC-Web 请求
- 稳定版本，Go 和 TS 均有良好支持

### 3.4 JSON-RPC / MCP 统一协议

用 JSON-RPC 2.0 (MCP 的底层协议) 作为统一插件通信协议。

**优点：**
- 文本协议，调试简单
- 与 MCP 生态直接兼容
- 实现简单，任何语言几行代码即可
- 传输无关 (stdio, HTTP, WebSocket 均可)

**缺点：**
- 性能不如 gRPC (JSON 序列化开销)
- 无内置流支持 (需额外设计)
- 无代码生成 (需手动维护类型)

### 3.5 Container/Sidecar 模式

插件作为容器运行，通过 API 通信。

**优点：**
- 最强隔离 (独立文件系统、网络、进程空间)
- 语言完全无关
- 可独立扩缩容
- 与 Kubernetes 生态天然集成

**缺点：**
- 启动慢 (300-1000ms)
- 资源占用高 (100-300MB+)
- 运维复杂度高
- 过重，不适合轻量插件场景

---

## 4. 插件协议设计

### 4.1 MCP (Model Context Protocol) 深入分析

MCP 是 Anthropic 主导的开放标准，专为 AI 应用的工具/资源扩展设计。

**三大原语：**

| 原语 | 说明 | Agent 场景 |
|------|------|-----------|
| **Tools** | 可执行的函数 | Agent 调用外部 API、执行操作 |
| **Resources** | 可读取的数据源 | Agent 获取上下文、知识库 |
| **Prompts** | 预定义的提示模板 | Agent 的行为指导、角色定义 |

**协议栈：**
```
┌─────────────────┐
│   Application   │  (Claude Desktop, IDE, AgentForge)
├─────────────────┤
│   MCP Client    │  JSON-RPC 2.0 + 能力协商
├─────────────────┤
│   Transport     │  stdio (本地) / Streamable HTTP (远程)
├─────────────────┤
│   MCP Server    │  工具实现 + 资源提供 + 提示模板
└─────────────────┘
```

**生命周期：**
1. Client 发送 `initialize` (含协议版本 + 能力声明)
2. Server 响应 (含自身协议版本 + 能力声明)
3. Client 发送 `initialized` 通知
4. 正常 JSON-RPC 消息交换
5. 关闭: stdio 关闭 stdin → 等待退出 → SIGTERM → SIGKILL

**传输演进：**
- **stdio**: 本地进程通信，零网络开销，最低延迟
- **HTTP+SSE**: 已弃用 (2024-11-05 版本)
- **Streamable HTTP**: 现代标准 (2025-03-26 版本)，单端点 POST/GET，支持可选 SSE 流

**与 AgentForge 的契合度：**
- TS Agent SDK Bridge 天然适合做 MCP Client
- 插件以 MCP Server 形式提供工具 → Agent 可直接调用
- 已有海量开源 MCP Server 可集成 (GitHub, Slack, Notion, DB 等)
- 可复用 Claude Agent SDK 中的 MCP 客户端实现

### 4.2 HashiCorp go-plugin 协议分析

**协议特征：**
- 基于 gRPC，Protobuf 消息编码
- 握手使用 Magic Cookie (环境变量 `TF_PLUGIN_MAGIC_COOKIE`)
- 版本协商: major version 编码在 protobuf package name 中 (如 `tfplugin5`, `tfplugin6`)
- 多 major version 共存: 同一 server 可同时实现多版本服务
- 内置服务: GRPCBroker (连接代理), GRPCController (生命周期), GRPCStdio (日志流)

**对 AgentForge 的启示：**
- 版本协商机制值得借鉴
- Magic Cookie 防止非法进程连接
- GRPCBroker 的多路复用可用于复杂参数传递

### 4.3 OpenAPI/JSON Schema 描述插件接口

用 JSON Schema 定义插件接口规范，类似 OpenAPI 描述 REST API。

```json
{
  "name": "web-search",
  "version": "1.0.0",
  "tools": [
    {
      "name": "search",
      "description": "Search the web",
      "inputSchema": {
        "type": "object",
        "properties": {
          "query": { "type": "string" },
          "limit": { "type": "integer", "default": 10 }
        },
        "required": ["query"]
      },
      "outputSchema": {
        "type": "object",
        "properties": {
          "results": {
            "type": "array",
            "items": { "$ref": "#/definitions/SearchResult" }
          }
        }
      }
    }
  ]
}
```

**优点**: 自描述、语言无关、可自动生成文档和客户端代码、与 MCP tool schema 兼容
**缺点**: 仅描述接口，不包含运行时协议

### 4.4 Event-driven 插件 (Webhook/PubSub)

基于事件驱动的插件模型，插件通过订阅事件和发布事件与系统交互。

```
AgentForge 事件总线 (Redis Pub/Sub)
    │
    ├── agent.task.created    → Plugin A (任务审计)
    ├── agent.tool.called     → Plugin B (工具监控)
    ├── agent.response.ready  → Plugin C (内容审核)
    └── agent.error.occurred  → Plugin D (告警通知)
```

**优点：**
- 松耦合: 插件不需要知道系统内部结构
- 异步: 不阻塞主流程
- 可扩展: 新增插件只需订阅事件
- 与 Redis 结合天然

**缺点：**
- 不适合同步请求-响应场景 (如工具调用)
- 事件顺序保证复杂
- 调试困难

**适用**: 通知、审计、监控类插件；不适合工具类插件。

---

## 5. 插件分发和注册

### 5.1 npm Registry 模式

将插件作为 npm 包发布。

**优点：**
- TS/JS 生态天然适配
- 成熟的版本管理 (semver)
- 依赖解析、lock file
- 支持 scope package (`@agentforge/plugin-xxx`)
- 可用 Verdaccio 自建私有 registry

**缺点：**
- 仅适合 JS/TS 插件
- 无法分发编译后的二进制文件
- 安全审计依赖 npm audit

### 5.2 OCI Registry 模式

用 OCI Artifact 标准分发插件，复用容器镜像 registry 基础设施。

**优点：**
- **统一分发**: 可存储 WASM 模块、二进制文件、配置、SBOM 等任何类型
- **不可变性**: 内容寻址，安全可验证
- **签名验证**: 与容器镜像相同的签名/验证工具链 (cosign, notation)
- **成熟基础设施**: Docker Hub, GitHub Container Registry, Harbor 等均支持
- **ORAS 工具链**: CNCF 项目，专为非容器 OCI 制品设计

**缺点：**
- 对开发者不够友好 (相比 npm install)
- 搜索/发现机制不如 npm
- 需要额外工具 (oras, helm) 来 push/pull

**示例流程：**
```bash
# 发布插件
oras push registry.agentforge.dev/plugins/web-search:1.0.0 \
  --artifact-type application/vnd.agentforge.plugin.v1 \
  plugin.wasm:application/wasm \
  manifest.json:application/json

# 拉取插件
oras pull registry.agentforge.dev/plugins/web-search:1.0.0
```

### 5.3 Git-based (GitHub Release + go install)

通过 Git 仓库 + Release 分发。

**优点：**
- 开发者最熟悉的方式
- 自动构建 (GitHub Actions)
- Go 插件可用 `go install`
- 社区贡献友好 (fork → PR)

**缺点：**
- 版本管理不如专用 registry 规范
- 二进制分发需维护多平台构建
- 无依赖解析

### 5.4 自建 Plugin Registry

AgentForge 自建插件市场/注册中心。

**设计要素：**
```
┌─────────────────────────────────────┐
│       AgentForge Plugin Registry     │
├─────────────────────────────────────┤
│  - 插件元数据存储 (PostgreSQL)       │
│  - 插件制品存储 (S3/MinIO + OCI)    │
│  - 搜索和发现 (全文检索)             │
│  - 版本管理 (semver)                 │
│  - 安全扫描 (CI 集成)               │
│  - 评分和评论                        │
│  - 安装统计                          │
│  - CLI 工具 (agentforge plugin)      │
└─────────────────────────────────────┘
```

**优点**: 完全可控、定制化 DX、安全审计集成
**缺点**: 开发维护成本高、冷启动生态问题

### 分发方案对比

| 维度 | npm | OCI | Git-based | 自建 Registry |
|------|-----|-----|-----------|--------------|
| 复杂度 | 低 | 中 | 低 | 高 |
| 多语言支持 | JS/TS only | 任意 | 任意 | 任意 |
| 安全性 | 中 | 高 (签名) | 低 | 高 |
| DX | 优秀 | 一般 | 良好 | 可定制 |
| 生态成熟度 | 极高 | 高 | 高 | 需建设 |
| 适合 MVP | 是 | 否 | 是 | 否 |

---

## 6. 安全沙箱对比

### 全景对比

| 维度 | 进程隔离 + seccomp | WASM 沙箱 | V8 Isolate | Container |
|------|-------------------|-----------|------------|-----------|
| **安全级别** | 高 | 极高 | 高 | 高 |
| **隔离原理** | 内核命名空间 + syscall 过滤 | 线性内存 + 能力模型 | V8 isolate + 无 Node API | 命名空间 + cgroups |
| **攻击面** | 内核 syscall | WASM 运行时 bug | V8 bug | 共享内核 |
| **启动时间** | 快 (~ms) | 极快 (20-100ms) | 极快 (~ms) | 慢 (300-1000ms) |
| **内存开销** | 低 (~10MB) | 极低 (10-50MB) | 极低 (~MB) | 高 (100-300MB) |
| **CPU 限制** | cgroups | WASM 指令计数 | 超时控制 | cgroups |
| **内存限制** | cgroups | 线性内存上限 | isolate 堆限制 | cgroups |
| **网络控制** | seccomp/iptables | 默认无网络 | 默认无网络 | 网络命名空间 |
| **文件系统** | mount namespace | 默认无 FS | 默认无 FS | overlay FS |
| **跨平台** | Linux only | 全平台 | 全平台 (需 --no-node-snapshot) | Linux (主要) |
| **性能开销** | 极低 (<1%) | 低 (I/O 场景) | 极低 | 低 |
| **语言支持** | 任意 (独立进程) | WASM 编译语言 | 仅 JS | 任意 |

### seccomp 详解

seccomp (secure computing) 是 Linux 内核安全设施:
- **严格模式**: 仅允许 `exit()`, `sigreturn()`, `read()`, `write()`
- **BPF 过滤模式**: 可为每个 syscall 编写自定义过滤规则
- 几乎零性能开销
- Chrome、Firefox 均使用 seccomp 沙箱化插件
- 可结合 Linux namespace 实现多层隔离

**工具链**: Firejail, nsjail, Cloudflare Sandbox, ZeroBoot (CoW fork, 亚毫秒启动)

### 隔离级别递进

```
弱 ←──────────────────────────────────────────────────→ 强

 进程          namespace       seccomp        gVisor       MicroVM      WASM
(共享内核)    (受限可见性)   (受限syscall)  (用户态内核)  (独立内核)  (无syscall)
```

### 推荐组合

对 AgentForge:
- **WASM 插件**: wazero/Extism 内置沙箱 (已足够安全)
- **进程级插件** (go-plugin/MCP): + seccomp 过滤 + namespace 隔离
- **不可信用户代码**: WASM (首选) 或 V8 Isolate + 资源限制
- **深度防御**: WASM 沙箱 + 进程隔离 + seccomp (多层叠加)

---

## 7. 开源参考实现

### 7.1 Terraform Provider 架构

```
┌──────────────┐        gRPC           ┌──────────────────┐
│              │  ←───────────────→    │  Provider Plugin  │
│  Terraform   │   tfplugin5/6 proto   │  (独立二进制)      │
│  Core        │                        │                   │
│              │  Magic Cookie 握手     │  terraform-       │
│              │  版本协商              │  provider-aws     │
│              │  GRPCBroker            │                   │
└──────────────┘                        └──────────────────┘
```

**关键设计：**
- 每个 Provider 是独立 Go 二进制
- go-plugin 管理子进程生命周期
- Protobuf 定义强类型接口 (tfplugin5.proto / tfplugin6.proto)
- 版本协商: client/server 协商选择双方都支持的 major version
- Plugin Framework SDK: 提供脚手架，开发者只需实现业务逻辑
- Registry (registry.terraform.io): 集中式插件发现和分发

**AgentForge 可借鉴：**
- Protocol versioning 机制
- Plugin SDK 模式 (降低开发门槛)
- Registry + CLI 工具链

### 7.2 VS Code Extension Host

```
┌──────────────────────────────────────────────┐
│                VS Code Main Process          │
│  ┌─────────┐  ┌──────────┐  ┌────────────┐  │
│  │ Renderer│  │ Extension │  │ Extension  │  │
│  │ Process │  │ Host      │  │ Host       │  │
│  │ (UI)    │  │ (Local    │  │ (Remote    │  │
│  │         │  │  Node.js) │  │  Node.js)  │  │
│  └────┬────┘  └────┬─────┘  └────┬───────┘  │
│       │            │              │           │
│       └────────────┴──────────────┘           │
│            RPC (MainThread* ↔ ExtHost*)       │
└──────────────────────────────────────────────┘
```

**关键设计：**
- Extension Host 是独立 Node.js 进程，运行所有已激活扩展
- **懒加载**: 扩展声明 Activation Events，仅在需要时加载
- **多类型 Host**: Local (Node.js), Web (浏览器), Remote (容器/SSH)
- **DOM 隔离**: 扩展不能直接操作 DOM
- **RPC 通信**: renderer ↔ extension host 通过双向 RPC
- **Language Server Protocol**: 语言扩展通过 LSP (stdio/TCP) 运行在独立进程
- **Utility Process**: 迁移到 Electron Utility Process 实现沙箱化

**AgentForge 可借鉴：**
- 懒加载 + Activation Events 模式
- 分离扩展声明 (manifest) 和运行时
- RPC-based 通信确保稳定性

### 7.3 Grafana Plugin SDK

```
┌─────────────────────────────────────────┐
│              Grafana Server              │
│                                          │
│  ┌────────────┐    ┌──────────────────┐ │
│  │ Frontend   │    │  Backend Plugin  │ │
│  │ (React/TS) │    │  (Go binary)     │ │
│  │ SystemJS   │    │  gRPC 通信        │ │
│  │ 动态加载    │    │  go-plugin 框架   │ │
│  └────────────┘    └──────────────────┘ │
└─────────────────────────────────────────┘
```

**关键设计：**
- **前后端分离**: 前端 React/TS + 后端 Go，通过定义清晰的接口通信
- **后端插件 = go-plugin**: 与 Terraform 相同的 gRPC 子进程模式
- **稳定性保证**: 插件崩溃不影响 Grafana 主进程
- **实例管理**: SDK 提供实例缓存，支持连接池复用
- **内置指标**: Prometheus 格式的运行时和自定义指标
- **流式数据**: 支持 streaming 数据源
- **并发查询**: SDK 0.232.0+ 支持并发查询 (默认串行，最多 10 并发)

**AgentForge 可借鉴：**
- 前端 + 后端插件分离模式 (与 Go + TS 双进程架构天然匹配)
- 实例管理和连接池
- 内置 observability

### 7.4 Backstage Plugin System

```
┌──────────────────────────────────────────────┐
│              Backstage Backend               │
│                                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ Plugin A │  │ Plugin B │  │ Plugin C │  │
│  │          │  │          │  │          │  │
│  │ 独立微服务│  │ Extension│  │ 使用      │  │
│  │ 无直接    │  │ Points   │  │ Services  │  │
│  │ 代码通信  │  │          │  │          │  │
│  └──────────┘  └──────────┘  └──────────┘  │
│       ↕              ↕              ↕        │
│    Services (log, db, config, auth, ...)     │
└──────────────────────────────────────────────┘
```

**关键设计：**
- **插件即微服务**: 插件间只能通过网络通信，无直接代码调用
- **水平可扩展**: 插件必须无状态或使用外部存储
- **Extension Points**: 插件可暴露扩展点，供 Module 扩展
- **Module 系统**: Module 通过 Extension Point 为插件添加功能
- **Service 层**: 内置日志、数据库、配置、认证等服务，插件和 Module 共享
- **前端 Extension Tree**: 所有前端扩展组成树状结构
- **Utility APIs**: 插件间共享功能的标准方式
- **路由抽象**: 插件间路由不依赖具体 URL 路径

**AgentForge 可借鉴：**
- 插件即微服务的哲学 (强隔离)
- Extension Point + Module 模式 (灵活扩展)
- Service 注入 (标准化插件基础设施访问)
- 路由抽象 (松耦合)

### 7.5 Buf Connect

见 [3.3 gRPC 插件协议 - 结合 Buf Connect](#33-grpc-插件协议)。

---

## 8. 综合对比矩阵

### Go 侧运行时对比

| 维度 | Go Native Plugin | go-plugin | Yaegi | wazero (WASM) | Extism |
|------|-----------------|-----------|-------|---------------|--------|
| **实现复杂度** | 低 | 中 | 低 | 中 | 低 |
| **运行时性能** | 极高 (原生) | 高 (gRPC) | 中 (解释) | 高 (JIT) | 高 (JIT) |
| **安全沙箱** | 无 | 进程隔离 | 受限 | 极强 | 极强 |
| **DX (开发体验)** | 差 (版本耦合) | 良好 | 优秀 | 良好 | 优秀 |
| **跨语言** | 无 | gRPC 跨语言 | 仅 Go | 任意→WASM | 任意→WASM |
| **跨平台** | Linux/Mac | 全 | 全 | 全 | 全 |
| **热更新** | 无 | 重启进程 | 即时 | 重载模块 | 重载模块 |
| **生态成熟度** | 低 | 极高 (HashiCorp) | 中 (Traefik) | 高 | 中-高 |
| **AgentForge 兼容** | 差 | 优秀 | 良好 | 优秀 | 优秀 |

### 跨语言方案对比

| 维度 | Extism (WASM) | gRPC 协议 | MCP 协议 | Container |
|------|--------------|-----------|----------|-----------|
| **复杂度** | 中 | 中 | 低 | 高 |
| **性能** | 高 | 高 | 中 | 中 |
| **安全性** | 极高 | 中 (依赖进程) | 中 (依赖进程) | 高 |
| **DX** | 良好 | 良好 | 优秀 | 一般 |
| **跨语言** | WASM 编译语言 | 几乎所有 | 几乎所有 | 所有 |
| **Agent 集成** | 需封装 | 需封装 | 天然适配 | 需封装 |
| **生态** | 中 | 极高 | 快速增长 | 极高 |
| **启动速度** | 极快 (<100ms) | 快 (~ms) | 快 (~ms) | 慢 (300ms+) |
| **资源占用** | 极低 | 低 | 低 | 高 |

---

## 9. 推荐方案

### MVP 阶段 (0-6 个月)

**核心策略**: MCP 协议 + go-plugin，快速搭建插件基础设施。

```
┌─────────────────────────────────────────────────────┐
│                  AgentForge MVP                      │
│                                                      │
│  ┌──────────────┐         ┌───────────────────────┐ │
│  │ Go           │  gRPC   │ TS Agent SDK Bridge   │ │
│  │ Orchestrator │ ←─────→ │                       │ │
│  │              │         │  ┌──────────────────┐ │ │
│  │ ┌──────────┐ │         │  │ MCP Client       │ │ │
│  │ │go-plugin │ │         │  │ (Claude SDK内置)  │ │ │
│  │ │Manager   │ │         │  └────────┬─────────┘ │ │
│  │ └──┬───────┘ │         └───────────┼───────────┘ │
│  └────┼─────────┘                     │              │
│       │                               │              │
│  ┌────▼─────┐  ┌────▼─────┐  ┌──────▼───────┐     │
│  │Go Plugin │  │Go Plugin │  │MCP Server    │     │
│  │(gRPC子进程)│  │(gRPC子进程)│  │(stdio/HTTP)  │     │
│  │任务钩子    │  │自定义审批  │  │工具扩展      │     │
│  └──────────┘  └──────────┘  └──────────────┘     │
└─────────────────────────────────────────────────────┘
```

**选型理由：**

1. **MCP 协议作为 Agent 工具扩展标准**
   - TS Bridge 已使用 Claude Agent SDK → MCP Client 天然可用
   - 海量开源 MCP Server 可直接集成 (GitHub, Slack, Jira, DB 等)
   - AI 社区标准，开发者熟悉度快速增长
   - 与 Agent 工具调用模型完美匹配

2. **go-plugin 作为 Go 侧扩展机制**
   - 经 Terraform/Vault 等验证的生产级方案
   - 进程隔离保障稳定性
   - gRPC 通信与现有架构一致
   - 适合任务钩子、审批流程、数据处理等 Orchestrator 侧扩展

3. **JSON Schema 描述插件接口**
   - 与 MCP tool schema 兼容
   - 自描述、可自动生成文档

4. **Git-based + npm 分发**
   - Go 插件: GitHub Release 二进制
   - MCP Server: npm 包 或 GitHub 仓库
   - 开发者友好，零额外基础设施

**MVP 不做的事：**
- 不建 WASM 运行时 (复杂度太高)
- 不建自建 Registry (先用 GitHub + npm)
- 不建 Container 插件 (过重)

---

### 长期阶段 (6-18 个月)

**核心策略**: 引入 WASM 统一运行时 + 自建 Registry，打造完整插件生态。

```
┌──────────────────────────────────────────────────────────────┐
│                    AgentForge 完整插件架构                     │
│                                                               │
│  ┌──────────────────────────────────────────────────────────┐│
│  │                  Plugin Registry                          ││
│  │  (自建, OCI 兼容, 元数据 PostgreSQL, 制品 S3/MinIO)       ││
│  └──────────────────────────────────────────────────────────┘│
│                                                               │
│  ┌──────────────┐           ┌───────────────────────┐        │
│  │ Go           │   gRPC    │ TS Agent SDK Bridge   │        │
│  │ Orchestrator │ ←───────→ │                       │        │
│  │              │           │  ┌──────────────────┐ │        │
│  │ ┌──────────┐ │           │  │ MCP Client       │ │        │
│  │ │Extism    │ │           │  │ + Extism JS SDK  │ │        │
│  │ │(wazero)  │ │           │  └────────┬─────────┘ │        │
│  │ │WASM Host │ │           └───────────┼───────────┘        │
│  │ └──┬───────┘ │                       │                    │
│  │    │         │                       │                    │
│  │ ┌──┼────────┐│                       │                    │
│  │ │go-plugin  ││                       │                    │
│  │ │Manager    ││                       │                    │
│  │ └──┬────────┘│                       │                    │
│  └────┼─────────┘                       │                    │
│       │                                 │                    │
│  ┌────┼─────────────────────────────────┼──────────────┐     │
│  │    │         Plugin Runtime Layer     │              │     │
│  │    │                                 │              │     │
│  │ ┌──▼──────┐ ┌──────────┐ ┌──────────▼──┐ ┌──────┐ │     │
│  │ │go-plugin│ │WASM      │ │MCP Server   │ │Event │ │     │
│  │ │(gRPC)   │ │Plugin    │ │(stdio/HTTP) │ │Hook  │ │     │
│  │ │         │ │(.wasm)   │ │             │ │(Redis│ │     │
│  │ │Orch扩展  │ │跨语言工具 │ │AI 工具扩展   │ │PubSub│ │     │
│  │ └─────────┘ └──────────┘ └─────────────┘ └──────┘ │     │
│  └─────────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────────┘
```

**长期阶段新增：**

1. **Extism WASM 运行时**
   - Go Orchestrator 集成 Extism Go SDK (底层 wazero)
   - TS Bridge 集成 Extism JS SDK
   - 同一 .wasm 插件双侧可加载
   - 最强安全沙箱 + 跨语言支持
   - 插件用 Rust/Go/C/AssemblyScript 等编写

2. **自建 Plugin Registry**
   - OCI 兼容 (可用 ORAS 工具链)
   - 元数据存储 PostgreSQL (复用现有)
   - 制品存储 S3/MinIO
   - 安全扫描集成
   - CLI: `agentforge plugin install/publish/search`

3. **Event Hook 系统**
   - 基于 Redis Pub/Sub 的事件钩子
   - 审计、监控、通知类插件
   - 异步非阻塞

4. **Plugin SDK 发布**
   - Go Plugin SDK (封装 go-plugin + Extism PDK)
   - TS Plugin SDK (封装 MCP Server + Extism PDK)
   - 脚手架工具: `agentforge plugin create`
   - 文档站 + 示例库

### 技术选型总结

| 插件类型 | 运行时 | 协议 | 分发 | 安全 |
|---------|--------|------|------|------|
| **Agent 工具** | MCP Server | MCP (JSON-RPC) | npm / GitHub | 进程隔离 |
| **Orchestrator 扩展** | go-plugin | gRPC | GitHub Release | 进程隔离 |
| **跨语言安全插件** | Extism/wazero | WASM ABI | OCI Registry | WASM 沙箱 |
| **事件钩子** | Redis Pub/Sub | Event JSON | 内置配置 | 事件过滤 |
| **用户自定义脚本** | QuickJS WASM | 内置 API | 在线编辑器 | WASM 沙箱 |

---

## 参考资源

- [HashiCorp go-plugin](https://github.com/hashicorp/go-plugin) - gRPC 插件框架
- [Extism](https://extism.org/) - 跨语言 WASM 插件框架
- [wazero](https://wazero.io/) - 零依赖 Go WASM 运行时
- [MCP 架构概览](https://modelcontextprotocol.io/docs/learn/architecture) - Model Context Protocol
- [Terraform Plugin Protocol](https://github.com/hashicorp/terraform/blob/main/docs/plugin-protocol/README.md)
- [VS Code Extension Host](https://code.visualstudio.com/api/advanced-topics/extension-host)
- [Grafana Plugin SDK (Go)](https://github.com/grafana/grafana-plugin-sdk-go)
- [Backstage Backend Architecture](https://backstage.io/docs/backend-system/architecture/index/)
- [Connect RPC](https://connectrpc.com/) - Buf 的现代 gRPC 替代
- [Yaegi](https://github.com/traefik/yaegi) - Go 解释器
- [@sebastianwessel/quickjs](https://github.com/sebastianwessel/quickjs) - QuickJS WASM 沙箱
- [isolated-vm](https://github.com/laverdet/isolated-vm) - V8 隔离环境
- [WasmEdge](https://wasmedge.org/) - 云原生 WASM 运行时
- [ORAS](https://oras.land/) - OCI Registry as Storage
