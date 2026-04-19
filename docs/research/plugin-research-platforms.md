# 主流 Agent 平台插件/扩展架构调研报告

> 调研时间：2026-03-23
> 调研范围：9 个主流 Agent 平台的插件/扩展机制

---

## 目录

1. [Claude Agent SDK / MCP](#1-claude-agent-sdk--mcp)
2. [LangChain / LangGraph](#2-langchain--langgraph)
3. [Dify](#3-dify)
4. [Coze (扣子)](#4-coze-扣子)
5. [OpenAI Assistants / GPTs / Agents SDK](#5-openai-assistants--gpts--agents-sdk)
6. [CrewAI](#6-crewai)
7. [AutoGen / AG2](#7-autogen--ag2)
8. [Composio](#8-composio)
9. [n8n / Zapier](#9-n8n--zapier)
10. [对比分析表](#10-对比分析表)
11. [关键发现与趋势总结](#11-关键发现与趋势总结)

---

## 1. Claude Agent SDK / MCP

### 概述

Model Context Protocol (MCP) 是 Anthropic 于 2024 年 11 月推出的开放标准，用于标准化 AI 系统与外部工具/数据源的集成。2025 年 12 月，Anthropic 将 MCP 捐赠给 Linux 基金会下的 Agentic AI Foundation (AAIF)，由 Anthropic、Block 和 OpenAI 共同创立。

### 插件/扩展定义方式

**Tool 定义**：基于 JSON Schema (draft-07) 定义工具输入/输出契约。每个 Tool 包含 `name`（唯一标识）、`description`（可选描述）和 `inputSchema`（JSON Schema 参数定义）。

**三种 MCP Server 类型**：

| 类型 | 传输协议 | 适用场景 |
|------|----------|----------|
| SDK MCP Server (In-Process) | 进程内调用 | 自定义工具直接嵌入应用 |
| External MCP Server | stdio / HTTP+SSE / Streamable HTTP | 本地或远程独立服务 |
| MCP Connector | HTTP (API 直连) | 无需额外 MCP 客户端，直接通过 Messages API 连接 |

**MCP Apps**：允许开发者构建交互式 UI（图表、表单、仪表盘），在 Claude/ChatGPT 等兼容客户端中内联渲染。

### 注册与发现机制

- **工具发现**：客户端通过 `tools/list` 端点列出可用工具
- **工具调用**：通过 `tools/call` 端点调用，服务器执行并返回结果
- **MCP Tool Search**：动态按需加载工具（而非预加载全部），避免上下文窗口被工具定义占满。需要 Sonnet 4+ 或 Opus 4+ 模型支持

### 生命周期管理

MCP 是有状态协议，生命周期包含：
1. **初始化**：客户端发送 `initialize` 请求，包含实现信息、协议版本和能力声明
2. **能力协商**：协商客户端和服务器支持的功能
3. **正常通信**：工具调用、资源访问等
4. **连接终止**：有序断开

通信基于 JSON-RPC 2.0 协议。

### 权限与隔离机制

- OAuth 2.1 授权支持
- 2025 年 6 月更新的授权规范将 MCP Server 归类为 OAuth 资源服务器
- 要求客户端实现 Resource Indicators (RFC 8707)
- 工具代表任意代码执行，需谨慎对待

### 市场/分享模式

- MCP Apps 市场 (apps.extensions.modelcontextprotocol.io)
- 社区开发的 MCP Server 通过 GitHub 分享
- 无官方集中式插件市场

### 开发者体验 (DX)

- TypeScript/Python SDK 完善
- 最小化抽象，开发者友好
- 协议标准化程度高，跨客户端兼容
- 文档完善，社区活跃

### 优缺点

| 优点 | 缺点 |
|------|------|
| 开放标准，跨平台兼容 | 协议仍在演进中，不同客户端支持不同传输协议 |
| 被 OpenAI、Google DeepMind 等广泛采用 | 安全模型依赖实现方，无内置沙箱 |
| 灵活的传输层选择 | 无官方插件市场 |
| JSON Schema 标准化工具定义 | 有状态协议增加了实现复杂度 |

---

## 2. LangChain / LangGraph

### 概述

LangChain 1.0 和 LangGraph 1.0 已发布。LangChain 专注于高层 Agent 构建，LangGraph 提供低层图基执行模型。两者月下载量超 9000 万，被 Uber、JP Morgan、Cisco 等企业使用。

### 插件/扩展定义方式

**Tool 定义**：基于 `@tool` 装饰器的函数式定义：

```python
@tool
def search(query: str) -> str:
    """Search the web."""
    return web_search(query)
```

**扩展模式层次**：
- **LangChain 高层 API**：`create_agent` 快速构建 Agent
- **LangGraph 低层 API**：图基状态机，支持自定义节点和边
- **可组合性**：LangChain Agent 可嵌入 LangGraph 工作流

**集成生态**：
- 可插拔工具包：FAISS、Chroma、SQL、Python、Shell 等
- MCP 支持：通过 `langchain-mcp-adapters` 适配
- Deep Agents：新扩展，支持规划、文件访问、子 Agent、上下文管理

### 注册与发现机制

- 装饰器自动注册工具
- Agent 运行时自动发现已注册工具
- 支持动态工具绑定

### 生命周期管理

- **LangGraph 持久状态**：Agent 执行状态自动持久化，可中断后恢复
- **检查点**：支持在任意点保存和恢复 Agent 工作流
- **Human-in-the-Loop**：可暂停执行等待人工审查

### 权限与隔离机制

- 无内置沙箱或权限系统
- 依赖开发者自行实现安全边界
- 框架级别无隔离

### 市场/分享模式

- 开源社区贡献
- LangChain Hub（Prompt 分享）
- 无正式插件市场

### 开发者体验 (DX)

- Python/TypeScript 双语言支持
- 抽象层设计良好，学习曲线适中
- LangSmith 提供可观测性（跟踪、评估）
- PyCharm AI Agents Debugger 支持

### 优缺点

| 优点 | 缺点 |
|------|------|
| 最成熟的 Agent 框架生态 | 抽象层可能过重 |
| 高层到低层无缝切换 | 无内置安全/隔离机制 |
| MCP 适配器支持 | 版本迭代频繁，API 不够稳定 |
| 强大的多 Agent 编排能力 | 主要面向 Python 开发者 |

---

## 3. Dify

### 概述

Dify v1.0.0（2025 年 2 月）引入插件优先架构。此前模型和工具与核心平台紧耦合，v1.0 将所有模型和工具迁移为插件，实现解耦。

### 插件/扩展定义方式

**五种核心插件类型**：

| 类型 | 用途 |
|------|------|
| Models | AI 模型管理，跨 chatbot/agent/workflow 使用 |
| Tools | 领域能力（数据分析、内容翻译、自定义集成） |
| Agent Strategies | Agent 推理策略（CoT、ToT、Function Call、ReAct） |
| Extensions | 扩展 Dify 内部生态（如 IM 集成） |
| Bundles | 插件组合包 |

**Manifest 结构** (YAML)：
```yaml
version: "1.0.0"
type: tool
author: developer_name
name: my_plugin
label: My Plugin
resource:
  memory: 256  # MB
permissions:
  tool: true
  model: false
  endpoint: true
  storage: true
meta:
  arch: [amd64, arm64]
runner:
  language: python
  version: "3.12"
  entrypoint: main
```

**Endpoint 插件**：支持类 Serverless 的灵活扩展，将更多真实场景集成到 Dify 内部生态。

### 注册与发现机制

- **Dify Marketplace**：官方插件市场，提供策展、搜索和安装
- **GitHub 社区分发**：开发者可通过 GitHub 自由分享
- **本地部署**：企业可私有部署，`.difypkg` 格式分发
- 发布流程：开发 → 测试 → 隐私策略编写 → 打包为 `.difypkg` → 提交审核

### 生命周期管理

**四种运行时模式**：

| 模式 | 通信方式 | 适用场景 |
|------|----------|----------|
| Local Runtime | STDIN/STDOUT 子进程 | 开发环境 |
| Debug Runtime | TCP + Redis 状态管理 | 开发调试（支持热重载） |
| Serverless Runtime | AWS Lambda | SaaS 部署（自动扩缩） |
| Enterprise Runtime | 受控环境 | 私有部署 |

Plugin Daemon 管理插件生命周期和远程安装。

### 权限与隔离机制

**安全模型 = 密码学签名 + 权限声明 + 沙箱隔离**：

1. **签名机制**：通过审核的插件用私钥签名标记为"已认证"，未签名插件显示"不安全"警告
2. **权限声明**：插件必须在 Manifest 中显式声明功能权限，未声明的权限直接拒绝
3. **DifySandbox**：基于 Linux 原生能力的沙箱运行时
   - **系统调用隔离**：Seccomp 白名单策略
   - **文件系统隔离**：虚拟化文件系统
   - **网络隔离**：独立 Sandbox 网络 + 代理容器
   - **权限隔离**：最低权限原则
   - 支持 Python 和 Node.js

### 市场/分享模式

- **Dify Marketplace**：120+ 官方/社区插件
- 合作伙伴计划
- 严格代码审查流程
- 隐私策略审核

### 开发者体验 (DX)

- 低代码/无代码 + 代码扩展双模式
- 可视化工作流编排
- Docker Compose 一键部署
- 插件开发者指南完善

### 优缺点

| 优点 | 缺点 |
|------|------|
| 最完善的插件市场和生态 | 插件类型和格式与 Dify 平台强耦合 |
| 多层安全机制（签名+沙箱+权限） | 自定义插件需要遵循严格规范 |
| 四种运行时适配不同场景 | 沙箱限制了某些依赖包的使用 |
| 企业级部署支持 | Python/Node.js 沙箱，不支持 Go 等 |

---

## 4. Coze (扣子)

### 概述

Coze 是字节跳动的 AI Agent 开发平台，2024 年 2 月上线，2025 年 7 月开源（Apache 2.0）。开源了 Coze Studio 和 Coze Loop 两个核心项目。

### 插件/扩展定义方式

**技术栈**：Go (后端) + React/TypeScript (前端)，微服务架构，领域驱动设计 (DDD)。

**插件系统**：
- 开放的插件定义、调用和管理机制
- 可将任意第三方 API 或私有能力包装为插件
- 内置插件：网页搜索、计算器、代码执行、图片生成、文件处理
- 开源版插件数量少于云版，需开发者自行扩展

**工作流节点扩展**：
- **前端**：在 React 中注册新节点类型，每个节点有唯一 type（需与后端约定）
- **后端**：基于 Eino 框架构建 DAG（有向无环图），包含控制流和数据流
- 支持分支选择（Branch 机制）、异常处理（超时、重试、降级、异常分支）

**四大核心模块**：Bot 创建 + Workflow 设计 + 知识库集成 + 多平台部署

### 注册与发现机制

- 统一的插件管理界面
- 可视化拖拽节点构建工作流
- API 和 SDK 支持 (Go, Python, Java, JS)
- ChatSDK 集成

### 生命周期管理

- Docker 容器化部署
- 微服务架构（MySQL, Redis, Milvus, NSQ 等）
- 工作流支持超时、重试、降级策略
- 一键部署脚本（最低双核 CPU + 4GB 内存）

### 权限与隔离机制

- 代码节点支持沙箱执行
- 微服务间网络隔离
- API 认证：Personal Access Token
- 具体沙箱细节开源代码中可见

### 市场/分享模式

- 云版有插件市场
- 开源版依赖社区贡献和自行开发
- GitHub 开源社区

### 开发者体验 (DX)

- "AI 的 Figma + Bubble"：拖拽 + 自然语言构建
- 可视化工作流编排
- 低代码/无代码友好
- Go + React 技术栈，适合企业二次开发
- 完整的 Prompt/RAG/Plugin/Workflow 核心技术栈

### 优缺点

| 优点 | 缺点 |
|------|------|
| Go + React 架构，高性能可扩展 | 开源版功能少于云版 |
| 完整的 Agent 开发技术栈 | 生态主要面向中国市场 |
| DDD 微服务设计，二次开发友好 | 插件系统文档有待完善 |
| Apache 2.0 开源 | 国际社区相对较小 |

---

## 5. OpenAI Assistants / GPTs / Agents SDK

### 概述

OpenAI 的扩展体系经历了重大演变：GPT Actions (2024, 已废弃) → Assistants API → Responses API + Agents SDK (2025.03)。当前主推 Responses API 作为统一接口。

### 插件/扩展定义方式

**Function Calling**（核心扩展机制）：
- JSON Schema 定义函数参数
- Strict 模式保证输出严格符合 Schema
- Structured Outputs (2024.06) 保证模型输出 100% 匹配定义

**Responses API 内置工具**：
- Web Search、File Search、Computer Use
- 单次 API 调用可使用多个工具和模型轮次

**Agents SDK**（轻量级 Agent 框架）：
- 核心原语：Agents（LLM + 指令 + 工具）、Handoffs（Agent 间委托）、Guardrails（输入/输出验证）
- Python 和 TypeScript 双语言支持
- 自动 Agent 循环、工具调用、多轮执行
- 内置 Tracing 可观测性
- 兼容非 OpenAI 模型（需 Chat Completions 兼容 API）

**GPT Actions**（已废弃）：
- 基于 OpenAPI Spec 的 REST API 调用
- 自然语言 → JSON Schema → API 调用
- 已被更统一的 Function Calling + Agent 框架替代

### 注册与发现机制

- Function Calling 通过 API 参数注册
- Agents SDK 通过代码定义 Agent 和工具
- 无动态发现机制（工具在创建时静态绑定）

### 生命周期管理

- Responses API：`previous_response_id` 隐式状态管理
- Conversations API：持久线程和可重放状态
- Agents SDK：自动处理 Agent 循环和工具执行

### 权限与隔离机制

- API Key 认证
- 无内置沙箱
- 依赖开发者实现安全边界
- Guardrails 提供输入/输出验证

### 市场/分享模式

- GPT Store（自定义 GPTs 分享）
- 无 Agents SDK 插件市场

### 开发者体验 (DX)

- Responses API 统一且简洁
- Agents SDK 极简抽象，学习曲线平缓
- 双语言 SDK (Python/TypeScript)
- 内置 Tracing 和调试
- 文档和示例丰富

### 优缺点

| 优点 | 缺点 |
|------|------|
| Function Calling 成为行业标准 | 平台锁定风险 |
| Responses API 统一 Chat + Assistants | 无开源，依赖 OpenAI 服务 |
| Agents SDK 极简但强大 | GPT Actions 废弃带来迁移成本 |
| Strict 模式保证可靠性 | 自定义 Agent 能力有限 |

---

## 6. CrewAI

### 概述

CrewAI 是轻量级 Python 框架，用于编排角色扮演的自治 AI Agent 团队。核心理念是模拟现实组织结构：Manager、Worker、Researcher 等角色协作完成任务。

### 插件/扩展定义方式

**两种 Tool 定义方式**：

1. **`@tool` 装饰器**（轻量/函数式）：
```python
@tool("Search Tool")
def search(query: str) -> str:
    """Search the web for information."""
    return web_search(query)
```

2. **`BaseTool` 子类化**（复杂/类式）：
```python
class MyTool(BaseTool):
    name: str = "My Tool"
    description: str = "Does something."
    args_schema: Type[BaseModel] = MyToolInput
    def _run(self, argument: str) -> str:
        return "result"
```

**Agent 配置**（YAML 声明式 + Python 程序式）：
```yaml
agents:
  researcher:
    role: "Research Analyst"
    goal: "Find comprehensive information"
    backstory: "Expert researcher with..."
    tools: [search_tool, scrape_tool]
```

**Crew 编排模式**：顺序、层级、混合流程，支持 guardrails、callbacks、human-in-the-loop。

**CrewAI Flows**：事件驱动的企业级编排引擎，协调多个 Crew 和任务。

### 注册与发现机制

- 工具通过装饰器或类定义自动注册
- Agent 在 YAML 或代码中绑定工具
- 100+ 开箱即用的开源工具
- MCP 支持：`pip install crewai-tools[mcp]`

### 生命周期管理

- CrewAI Studio：可视化管理 Crew
- Control Plane：集中管理、监控和扩展
- Tracing & Observability：实时监控
- 工具结果自动缓存

### 权限与隔离机制

- 角色级别的工具访问控制
- Agent 间任务委托机制
- 无系统级沙箱
- 依赖开发者实现安全边界

### 市场/分享模式

- crewai-tools 开源工具库 (GitHub)
- CrewAI Studio 提供 SaaS 工具集成（Gmail、Teams、Notion 等）
- 无正式插件市场

### 开发者体验 (DX)

- YAML + Python 双模式配置
- 角色隐喻直觉（role/goal/backstory）
- 30 分钟内可运行多 Agent 流水线
- Pydantic 类型安全
- 企业控制面板

### 优缺点

| 优点 | 缺点 |
|------|------|
| 角色隐喻极其直觉 | 仅 Python |
| YAML 声明式 + Python 程序式灵活切换 | 企业功能需付费 |
| 100+ 内置工具 + MCP 支持 | 无系统级安全隔离 |
| Flows 支持复杂事件驱动编排 | 社区规模小于 LangChain |

---

## 7. AutoGen / AG2

### 概述

AG2（前身为 Microsoft AutoGen）是开源 Agent OS，核心概念是"可对话 Agent"，通过结构化对话协作解决问题。正在从 v0.2 过渡到 v1.0。

### 插件/扩展定义方式

**工具/技能定义**：
- 将可调用函数注册为 Agent 可用工具
- ConversableAgent 是基础构建块，处理消息交换和响应生成
- 自定义回复方法：注册自定义回复函数，定制 Agent 行为

**编排模式**：
- Group Chat：多 Agent 共享消息线程
- Swarm：蜂群模式
- Nested Chat：嵌套对话
- Sequential Chat：顺序对话
- 自定义编排：注册自定义回复方法

**Agent 角色**：
- Writer、Illustrator、Editor 等专业化 Agent
- 每个 Agent 订阅和发布到共同 topic

### 注册与发现机制

- 函数注册为工具
- Agent 在对话中动态发现和使用工具
- 支持 LLM、non-LLM 工具和人类输入

### 生命周期管理

- 正在向 v1.0 过渡
- `autogen.beta` 将成为正式版本
- 当前框架通过弃用逐步清理

### 权限与隔离机制

- 无内置安全机制
- 对话级别的权限控制
- 依赖开发者实现

### 市场/分享模式

- 完全开源 (GitHub)
- 无插件市场
- 无付费层

### 开发者体验 (DX)

- 对话驱动的 Agent 交互模型
- 适合学术研究和原型开发
- 文档和社区资源参差不齐（AutoGen → AG2 分裂）
- Python 3.10+ 要求

### 优缺点

| 优点 | 缺点 |
|------|------|
| 完全免费开源 | 非生产就绪 |
| 对话驱动模型独特 | AutoGen → AG2 生态碎片化 |
| 灵活的群聊和编排模式 | 文档和社区资源不稳定 |
| 学术研究和原型开发优秀 | v1.0 尚未发布 |

---

## 8. Composio

### 概述

Composio 是专为 AI Agent 设计的集成平台，提供 MCP 兼容的工具网关，连接 500+ 业务工具。定位为 "AI Agent 时代的集成层"。

### 插件/扩展定义方式

**MCP Gateway 架构**：
- 充当 AI Agent 和工具之间的反向代理
- Agent 连接单一网关端点，而非直接连接多个工具
- 有状态、会话感知，专为 AI Agent 的双向通信设计

**集成方式**：
- MCP Server (Rube)：可安装在 Cursor、Claude Desktop、VS Code 等客户端
- 直接 API 调用
- Python SDK (3.10+) 和 TypeScript SDK

**工具管理**：
- 500+ 预构建工具包（Slack、GitHub、Notion、Google Workspace、Microsoft 等）
- 每个工具已预配置认证、错误处理和维护
- 支持自定义工具集成

### 注册与发现机制

- MCP 标准的 `tools/list` 发现
- 统一端点暴露工具目录
- Agent 通过网关发现和调用工具
- 框架兼容：LangChain、CrewAI、LlamaIndex

### 生命周期管理

- 托管认证和会话管理
- 工具自动配置和维护
- 无需开发者管理单个工具连接

### 权限与隔离机制

- OAuth 认证托管
- 每个工具独立的认证流程
- 网关级别的访问控制

### 市场/分享模式

- 预构建工具目录
- 无社区插件市场
- SaaS 模式

### 开发者体验 (DX)

- "零代码"工具集成
- MCP 兼容，跨客户端使用
- 认证复杂度抽象
- 快速开发和部署

### 优缺点

| 优点 | 缺点 |
|------|------|
| 500+ 预构建工具，即用即得 | SaaS 依赖，非自托管 |
| MCP 兼容，标准化 | 自定义工具能力有限 |
| 托管认证消除最大痛点 | 主要是集成层，非 Agent 框架 |
| 多框架兼容 | 定价可能随使用量增长 |

---

## 9. n8n / Zapier

### n8n

#### 概述

n8n 是开源的工作流自动化平台，2025 年 10 月 C 轮融资 1.8 亿美元，估值 25 亿美元。已从纯自动化工具转型为 AI-Native 自动化平台。

#### 插件/扩展定义方式

**节点 (Node) 系统**：
- 400+ 原生节点 + 600+ 社区节点
- 自定义节点开发和发布
- HTTP Request 节点可连接任何 REST API
- JavaScript/Python 代码节点

**AI Agent 架构**（基于 LangChain JS）：
- 70+ AI 专用节点
- AI Agent 节点作为编排层
- 子节点提供：LLM 集成、记忆、工具
- 支持 RAG、Tool Use、多 Agent 编排

**支持的 AI 模型**：OpenAI、Anthropic Claude、Google Gemini、Ollama（本地）、Hugging Face 等。

#### 注册与发现机制

- 可视化节点库
- 社区节点仓库
- npm 包分发

#### 生命周期管理

- Docker/K8s 自托管
- 按工作流执行计费（非按步骤）
- 自托管完全免费，无限执行
- 云版本提供托管服务

#### 权限与隔离机制

- 自托管提供完全控制
- 网络隔离可配置
- 凭证加密存储

#### 市场/分享模式

- 社区节点库
- 工作流模板分享
- GitHub + Discord + Discourse 社区

### Zapier

#### 概述

工作流自动化领域的"家喻户晓"品牌，8000+ 应用集成。

#### 插件/扩展定义方式

- 8000+ 预构建应用连接器
- 轻量级 JavaScript 代码步骤
- CLI 支持私有应用开发
- Zapier AI Actions（AI 触发工作流）
- 支持 MCP

#### 注册与发现机制

- 应用目录搜索
- Zap 模板
- 无开放插件生态

#### 权限与隔离机制

- 云端托管，Zapier 管理安全
- 无自托管选项
- 严格执行限制

### n8n vs Zapier 对比

| 维度 | n8n | Zapier |
|------|-----|--------|
| 集成数量 | 400+ 原生 + 600+ 社区 | 8000+ |
| AI 能力 | 70+ AI 节点，原生 Agent 支持 | AI 步骤，基础 Agent |
| 定价模式 | 按工作流执行，自托管免费 | 按步骤计费 |
| 可扩展性 | 自定义节点，开源 fork | CLI 私有应用，有限 |
| 适用人群 | 开发者和技术团队 | 非技术用户和 SMB |

### 优缺点

| 平台 | 优点 | 缺点 |
|------|------|------|
| n8n | 开源自托管、AI 原生、高度可扩展 | 集成数量少于 Zapier、学习曲线较陡 |
| Zapier | 8000+ 集成、极易上手 | 闭源、按步骤计费贵、AI 能力基础 |

---

## 10. 对比分析表

| 维度 | Claude/MCP | LangChain/LangGraph | Dify | Coze | OpenAI | CrewAI | AG2 | Composio | n8n/Zapier |
|------|-----------|---------------------|------|------|--------|--------|-----|----------|-----------|
| **扩展方式** | MCP Server (JSON Schema) | Python 装饰器/类 | 插件包 (YAML Manifest) | API 包装 + DAG 节点 | Function Calling (JSON Schema) | 装饰器/BaseTool + YAML | 函数注册 | MCP Gateway + SDK | 可视化节点 |
| **隔离级别** | OAuth，无沙箱 | 无 | Seccomp 沙箱 + 签名 | 代码节点沙箱 | 无 | 无 | 无 | 网关级隔离 | 自托管级 |
| **DX 评分** | ★★★★☆ | ★★★★☆ | ★★★★★ | ★★★★☆ | ★★★★★ | ★★★★☆ | ★★★☆☆ | ★★★★☆ | ★★★★☆ |
| **生态成熟度** | ★★★★☆ (快速增长) | ★★★★★ | ★★★★☆ | ★★★☆☆ | ★★★★★ | ★★★☆☆ | ★★☆☆☆ | ★★★☆☆ | ★★★★★ (Zapier) |
| **多语言支持** | TS/Python | Python/TS | Python/Node.js (插件) | Go/Python/Java/JS (SDK) | Python/TS | Python | Python | Python/TS | JS/Python (节点) |
| **市场/社区** | MCP Apps + GitHub | Hub + GitHub | Marketplace (120+) | 云版市场 + 开源社区 | GPT Store | crewai-tools + Studio | GitHub 仅 | 工具目录 | 社区节点 + 模板 |
| **自托管** | N/A (协议) | 是 (框架) | 是 | 是 | 否 | 部分 | 是 | 否 | 是 (n8n) |
| **MCP 兼容** | 原生 | 适配器 | 未知 | 未知 | 采用中 | 支持 | 未知 | 原生 | 部分 (Zapier) |
| **与 AgentForge 兼容性** | ★★★★★ | ★★★★☆ | ★★★★☆ | ★★★★★ | ★★★☆☆ | ★★★☆☆ | ★★☆☆☆ | ★★★☆☆ | ★★★☆☆ |

### 与 AgentForge 兼容性说明

- **Claude/MCP ★★★★★**：MCP 作为开放标准，AgentForge 可直接实现 MCP Server，获得跨客户端兼容。JSON Schema Tool 定义与 Go 后端天然匹配。
- **Coze ★★★★★**：同为 Go + TS 双进程架构，DDD 微服务设计理念一致，工作流节点扩展模式可直接借鉴。
- **LangChain ★★★★☆**：Agent 编排模式成熟可参考，但 Python 生态需要适配层。
- **Dify ★★★★☆**：插件系统设计最完善，Manifest/权限/安全模型可直接参考，但 Python 沙箱需要替换为 Go 兼容方案。
- **OpenAI ★★★☆☆**：Function Calling JSON Schema 可复用，但闭源平台无法深度参考架构。

---

## 11. 关键发现与趋势总结

### 1. MCP 成为事实标准

MCP 已被 Anthropic、OpenAI、Google DeepMind 等采用，成为 AI Agent 与工具交互的事实标准。AgentForge 应将 MCP 兼容作为核心设计目标。

**关键设计启示**：
- JSON Schema 工具定义是行业共识
- 支持 stdio/HTTP+SSE/Streamable HTTP 多传输协议
- 有状态协议需要生命周期管理
- OAuth 2.1 成为认证标准

### 2. 插件系统向"解耦+市场"演进

Dify v1.0 的经验证明：从核心平台紧耦合到插件化解耦是必经之路。插件市场 + 审核机制 + 签名认证是成熟路径。

**关键设计启示**：
- Manifest (YAML/JSON) 声明式插件元数据
- 显式权限声明 + 密码学签名
- 多运行时模式（本地/调试/Serverless/企业）
- `.pkg` 格式分发

### 3. 角色驱动的 Agent 定义成为主流

CrewAI 的 role/goal/backstory 模式证明：角色隐喻比纯技术配置更直觉。LangGraph 的图基模型提供更精确的控制。

**对 AgentForge 的启示**：
- 数字员工角色 = CrewAI 的 role + goal + backstory + tools
- 工作流编排 = LangGraph 的图基状态机 + CrewAI Flows 的事件驱动
- 两层抽象：高层角色声明 + 低层图基控制

### 4. Go + TS 双进程架构有成功先例

Coze Studio 采用 Go (后端) + React/TS (前端) + DDD 微服务，与 AgentForge 架构高度匹配。其工作流节点扩展机制（前端注册 + 后端 DAG 执行）可直接参考。

### 5. 安全隔离是企业级必需

| 安全层次 | 代表方案 | AgentForge 建议 |
|----------|----------|-----------------|
| 无隔离 | LangChain, CrewAI, AG2 | 不可接受 |
| 签名认证 | Dify (密码学签名) | 基础要求 |
| 沙箱执行 | Dify (Seccomp), Coze | 代码执行必需 |
| 网关隔离 | Composio (MCP Gateway) | 外部工具调用 |
| 权限声明 | Dify (Manifest 权限), MCP (OAuth) | 必须实现 |

### 6. 低代码/可视化是 DX 差异化关键

Dify、Coze、n8n 的成功证明：可视化工作流编排大幅降低使用门槛。AgentForge 应提供：
- 代码定义（开发者）+ 可视化编排（运营人员）双模式
- YAML 声明式配置作为中间层

### 7. 多 Agent 编排趋向标准化

三种主流编排模式已稳定：
- **顺序**：Agent A → Agent B → Agent C
- **层级**：Manager Agent 分配任务给 Worker Agents
- **事件驱动**：CrewAI Flows, LangGraph 的消息订阅

### 8. 工具集成平台化

Composio 模式证明：Agent 不需要直接管理 500+ 工具的认证和集成。MCP Gateway 作为中间层，统一管理工具发现、认证和调用，是更可持续的架构。

---

### 对 AgentForge 插件系统的综合建议

基于以上调研，AgentForge 插件系统应：

1. **原生支持 MCP**：作为核心扩展协议，兼容整个 AI Agent 生态
2. **借鉴 Dify 的 Manifest + 安全模型**：YAML 元数据声明 + 权限系统 + 签名认证
3. **参考 Coze 的 Go + TS 节点扩展**：前端节点注册 + 后端 DAG 执行
4. **采用 CrewAI 的角色隐喻**：数字员工定义 = role + goal + backstory + tools
5. **实现多运行时**：本地开发 / Docker 沙箱 / Serverless 部署
6. **构建插件市场**：审核 + 签名 + 社区分发
7. **提供双模式 DX**：代码定义 + 可视化编排
