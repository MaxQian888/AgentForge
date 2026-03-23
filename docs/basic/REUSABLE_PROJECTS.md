# 可复用的开源项目调研报告 — Agent 驱动开发管理工具

> 调研日期：2026-03-22 | 7 个 Agent 并行调研 | 覆盖 100+ 项目

---

## 核心发现：已有 12+ 个开源项目直接做「Agent + 任务管理 + 编码」

这是最重要的发现 — 市场上已经涌现出大量将 AI 编码 agent 与任务管理结合的开源项目（大多诞生于 2026 年 1-3 月）：

---

## 一、最高价值项目（直接可复用/参考）

### 1. Composio Agent Orchestrator ⭐⭐⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [ComposioHQ/agent-orchestrator](https://github.com/ComposioHQ/agent-orchestrator) |
| 发布 | 2026.02（由 30 个并发 agent 在 8 天内构建） |
| 技术栈 | TypeScript, Node.js, tmux/Docker |
| License | 待确认 |

**这是什么：** 管理**并行 AI 编码 agent 舰队**的编排器。每个 agent 获得独立的 git worktree、分支和 PR。自主处理 CI 修复、合并冲突和代码审查。

**核心架构：**
- **Planner** — 分解任务
- **Executor** — 执行编码
- **Tracker** — 追踪进度（对接 GitHub Issues / Linear）
- **8 个可插拔插槽** — 插件化架构
- **SSE Dashboard** — 实时监控面板

**为什么重要：** 这几乎就是你要做的东西。Agent 无关（支持 Claude Code / Codex / Aider），Tracker 无关（GitHub Issues / Linear），有 Web Dashboard，有插件架构。

---

### 2. OpenSwarm ⭐⭐⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [Intrect-io/OpenSwarm](https://github.com/Intrect-io/OpenSwarm) |
| 发布 | 2026.03（HN launch） |
| 技术栈 | TypeScript, Claude Code CLI, LanceDB, Discord.js, Linear API |

**这是什么：** 编排多个 Claude Code 实例。从 Linear 拾取 issue → 运行 Worker/Reviewer 配对流水线 → 向 Discord 报告 → 通过 LanceDB 保留长期记忆。

**核心架构：**
- **Linear 集成** — 自动拾取 issue，更新状态
- **Worker/Reviewer 管道** — 编码 + 审查配对
- **Discord 命令界面** — 远程控制
- **实时 Dashboard** — repo 状态、pipeline 事件、实时日志、PR 处理器
- **LanceDB 长期记忆** — 跨 session 记忆

**为什么重要：** 最接近「团队通过 IM 管理 AI agent 做开发」的工作流。Linear（任务） + Discord（IM） + Dashboard（监控） = 完整闭环。

---

### 3. Dorothy ⭐⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [Charlie85270/Dorothy](https://github.com/Charlie85270/Dorothy) |
| 类型 | Desktop App |
| 技术栈 | Electron, 5 个 MCP server, 40+ 工具 |

**这是什么：** 桌面应用，用 **Kanban 看板** 编排多个 AI CLI agent。Super Agent 跨 agent 池委派任务。

**核心能力：**
- 内建 Kanban 看板 + 自动 agent 分配 + 技能匹配
- 支持 Claude Code, Codex, Gemini CLI 等多种 agent
- 远程控制 + GitHub PR/issue 触发器
- 5 个 MCP server 提供 40+ 工具

---

### 4. Vibe Kanban ⭐⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [BloopAI/vibe-kanban](https://github.com/BloopAI/vibe-kanban) |
| 技术栈 | Web 应用 |

**这是什么：** Kanban issue 规划 + Agent 工作区（每个有独立分支、终端、开发服务器）+ 内联评论审查 diff。

**核心能力：**
- 完整 Kanban 看板
- 支持 10+ 编码 agent（Claude Code, Codex, Gemini CLI 等）
- 每个任务独立工作区（分支 + 终端 + dev server）
- 内联 diff 审查

---

### 5. Mission Control ⭐⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [crshdn/mission-control](https://github.com/crshdn/mission-control) |
| 官网 | mc.builderz.dev |
| 技术栈 | Web 应用, 80+ API 端点 |

**这是什么：** AI Agent 系统的开源管理面板。6 列 Kanban（Inbox → Assigned → In Progress → Review → Quality Review → Done），流水线编排，token 用量追踪。

**核心能力：**
- 6 列 Kanban + 拖拽 + 优先级 + 线程评论
- 子 agent 生成
- 成本分析面板
- 通过 OpenClaw 网关集成 AI 编码
- 80+ API 端点

---

### 6. Open SWE (LangChain) ⭐⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [langchain-ai/open-swe](https://github.com/langchain-ai/open-swe) |
| Stars | ~7,500 |
| 发布 | **2026.03.17**（非常新！） |
| 技术栈 | Python, LangGraph, Daytona 沙箱 |
| License | MIT |

**这是什么：** 异步云端编码 agent。连接 GitHub repo，委派任务，自动创建 PR。模式来自 Stripe / Ramp / Coinbase 内部工具。

**核心能力：**
- 通过 Slack 线程 / Linear issue / GitHub 评论触发
- Manager / Planner / Programmer 多 agent 分解
- Daytona 隔离沙箱执行
- 企业级模式（来自 Stripe/Ramp/Coinbase）

---

## 二、成熟平台（有任务管理能力）

### 7. OpenHands ⭐⭐⭐⭐⭐（最成熟）

| 属性 | 详情 |
|------|------|
| GitHub | [OpenHands/OpenHands](https://github.com/OpenHands/OpenHands) |
| Stars | **~69,000** |
| 最新版 | v1.5.0 (2026.03.11) |
| License | MIT |

- v1.5.0 新增 **TaskTrackerTool** + 任务列表面板 + 规划 agent
- 完整沙箱（Docker/K8s）+ Web UI + REST API + WebSocket
- Software Agent SDK 支持构建自定义 agent
- GitHub/GitLab/Bitbucket 集成
- 100+ LLM 提供商

---

### 8. OpenCode ⭐⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [anomalyco/opencode](https://github.com/anomalyco/opencode) |
| Stars | **~126,000** |
| License | MIT |

- 开源 Claude Code 替代品
- Agent Teams 支持（build/plan agent + 子 agent）
- 5M 月活用户，800+ 贡献者
- 终端 + IDE + 桌面应用
- 75+ 模型支持

---

## 三、项目管理层（可作为任务管理基座）

### 9. Plane ⭐⭐⭐⭐⭐（最佳任务管理基座）

| 属性 | 详情 |
|------|------|
| GitHub | [makeplane/plane](https://github.com/makeplane/plane) |
| Stars | **~46,800** |
| 最新版 | v1.2.3 (2026.03.05) |
| 技术栈 | Python/Django + React/Next.js + PostgreSQL + Redis + RabbitMQ |
| License | AGPL-3.0 |

**为什么是最佳：**
- **官方 MCP Server** — [makeplane/plane-mcp-server](https://github.com/makeplane/plane-mcp-server)
- **完整 REST API** + Python/Node.js SDK
- **OAuth 应用框架** + 应用市场
- **Webhook** 支持项目/Issue/Cycle/Module 事件
- 工作项/Sprint/模块/文档/看板/时间线

---

### 10. Gitea ⭐⭐⭐⭐（最佳代码管理基座）

| 属性 | 详情 |
|------|------|
| GitHub | [go-gitea/gitea](https://github.com/go-gitea/gitea) |
| Stars | **~54,300** |
| 最新版 | v1.25.5 |
| 技术栈 | Go + Vue.js + PostgreSQL |
| License | MIT |

**为什么重要：**
- **GitHub 兼容 REST API**（Swagger 文档）
- **优秀的 Webhook 系统**（支持 Slack/Discord/Telegram/钉钉/飞书/Teams）
- **Gitea Actions**（GitHub Actions 兼容 CI/CD）
- **官方 MCP Server**
- 单二进制部署，512MB RAM 可运行

---

### 11. Huly ⭐⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [hcengineering/platform](https://github.com/hcengineering/platform) |
| Stars | **~25,100** |
| 技术栈 | TypeScript 全栈 + Svelte + MongoDB |
| License | Apache 2.0 |

**独特优势：**
- **深度插件架构** — 整个平台都是插件，可作为基座构建新应用
- **实时 WebSocket** — hulypulse 推送服务
- 内建：Issue Tracker + 文档(Notion 式) + 聊天(Slack 式) + 收件箱 + 日历
- GitHub 双向同步

---

## 四、工作流 & 自动化层

### 12. n8n ⭐⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [n8n-io/n8n](https://github.com/n8n-io/n8n) |
| Stars | **~180,000** |
| 技术栈 | TypeScript + Vue.js + PostgreSQL |
| License | Fair-code（非 OSI 开源） |

- 400+ 集成 + 原生 LangChain AI agent 工作流
- **MCP Client + Server 支持**
- 可视化工作流构建器

### 13. Temporal ⭐⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [temporalio/temporal](https://github.com/temporalio/temporal) |
| Stars | ~19,000 |
| License | MIT |

- **OpenAI Codex 的 agent 基础设施**
- 持久执行 — 工作流崩溃自动恢复
- 1.86 万亿次 AI 公司执行
- SDK: Go/Java/TypeScript/Python/.NET

### 14. Activepieces ⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [activepieces/activepieces](https://github.com/activepieces/activepieces) |
| Stars | ~21,300 |
| License | **MIT** |

- **400 个 MCP Server**（最大开源 MCP 工具集）
- AI agent 框架 + MCP 集成

---

## 五、AI Agent 管理平台

### 15. Dify ⭐⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [langgenius/dify](https://github.com/langgenius/dify) |
| Stars | **~134,000** |
| License | Apache 2.0 |

- 可视化 Agent 工作流构建器 + RAG + LLMOps
- 50+ 内建工具 + MCP 支持
- REST API + 多模型 + RBAC
- $30M 融资

### 16. Langflow ⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [langflow-ai/langflow](https://github.com/langflow-ai/langflow) |
| Stars | **~100,000** |
| License | **MIT** |

- 拖拽式可视化 agent 构建器 + MCP
- 代码级可定制（非沙箱）
- 桌面应用可用

---

## 六、基础设施层

### 17. E2B — Agent 代码沙箱

| 属性 | 详情 |
|------|------|
| GitHub | [e2b-dev/E2B](https://github.com/e2b-dev/E2B) |
| Stars | ~8,900 |
| License | Apache 2.0 |

- Firecracker 微 VM，150-200ms 启动
- 专为 AI 生成代码执行设计

### 18. Daytona — Agent 开发环境

| 属性 | 详情 |
|------|------|
| GitHub | [daytonaio/daytona](https://github.com/daytonaio/daytona) |
| License | AGPL |

- 90ms 环境创建 + Git 操作 + MCP server
- 被 OpenHands / Open SWE / Google ADK 使用

### 19. Composio — Agent 工具连接层

| 属性 | 详情 |
|------|------|
| GitHub | [ComposioHQ/composio](https://github.com/ComposioHQ/composio) |
| Stars | ~27,000 |
| License | MIT |

- 1000+ 工具套件连接 agent 到 500+ 应用
- 认证管理 + 沙箱工作台

---

## 七、其他值得关注的项目

| 项目 | Stars | 描述 | 复用价值 |
|------|-------|------|---------|
| **KaibanJS** | 增长中 | JS 原生多 agent + Kanban 看板 | 高（JS 团队） |
| **OpenKanban** | 新 | TUI Kanban + 每 ticket 独立 worktree | 中高 |
| **AI Team OS** | 新 | 40+ MCP 工具 + 22 agent 模板 + 任务墙 | 中高 |
| **claude-code-swarm** | 小 | 24/7 监听 GitHub issue → 派发 Claude Code → 创建 PR | 中 |
| **AiderDesk** | 增长中 | Aider 桌面 GUI + 子任务树 + 成本追踪 | 中 |
| **Plandex** | 15K | 终端 AI 编码 + 规划 + 回滚 | 中 |
| **Letta (MemGPT)** | 21.6K | 有状态 agent + 记忆管理 + Letta Code | 中 |
| **NocoBase** | 21.8K | 插件微内核 + AI Employees + MCP 插件 | 中（作为 UI 基座） |

---

## 补充：最后一轮调研新发现的关键项目

### Agent Swarm (desplega-ai) ⭐⭐⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [desplega-ai/agent-swarm](https://github.com/desplega-ai/agent-swarm) |
| 官网 | agent-swarm.dev |
| 技术栈 | TypeScript, Docker, SQLite, OpenAI Embeddings |
| License | MIT |

**这是什么：** Lead agent 接收任务（来自你/Slack/GitHub），分解后委派给 Docker 隔离的 worker agent。Worker 执行任务、报告进度、自主交付代码。

**核心架构：**
- **Lead Agent + Docker Worker 隔离** — 每个 worker 有独立容器
- **优先级任务队列** — 依赖追踪 + 暂停/恢复
- **复合记忆系统** — 可搜索的 embedding（SQLite + OpenAI）
- **Worker 持久身份** — SOUL.md / IDENTITY.md / TOOLS.md / CLAUDE.md
- **多渠道集成** — Slack, GitHub, GitLab, Email

**为什么重要：** Docker 隔离 + 优先级队列 + 持久记忆 + Slack/GitHub 集成 = 生产级 agent 编排。

---

### Ruflo (原 Claude Flow) ⭐⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [ruvnet/ruflo](https://github.com/ruvnet/ruflo) |
| Stars | ~20,700 |
| 最新版 | v3.5.0（55 次 alpha 迭代，5,800+ commits） |
| 技术栈 | TypeScript, MCP, WASM |

**核心能力：**
- **60+ 专项 Agent** + **215+ MCP 工具**
- 自学习神经路由 + 3 层模型路由（节省最多 75% API 成本）
- 双模式编排：Claude Code + OpenAI Codex worker 并行
- 13 个专项 GitHub agent（repo 管理/代码审查/发版协调）
- AgentDB v3 + 8 个控制器

---

### Overstory ⭐⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [jayminwest/overstory](https://github.com/jayminwest/overstory) |
| 最新版 | v0.5.6 |
| 技术栈 | TypeScript (Bun), SQLite, tmux |
| License | MIT |

**核心能力：**
- 单会话 → 多 agent 团队（Orchestrator → Team Lead → Specialist Workers）
- 每个 worker 在独立 git worktree + tmux 中运行
- SQLite 邮件系统协调 + FIFO 合并队列 + 4 层冲突解决
- **os-eco 生态系统**：Seeds（issue 追踪）+ Mulch（专长存储）+ Canopy（prompt 管理）

---

### Squad (Microsoft/Brady Gaster) ⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [bradygaster/squad](https://github.com/bradygaster/squad) |
| 最新版 | v0.5.2 (2026.03, alpha) |
| 技术栈 | TypeScript/Node.js |

**核心能力：**
- 在仓库中初始化预配置 AI 团队（lead + 前端 + 后端 + 测试）
- Agent 身份/记忆存储为 `.squad/` 纯文本文件，随代码版本化
- 与 GitHub Copilot Coding Agent 集成自动拾取 issue
- 单 32KB prompt 编排

---

### Claude MPM (Multi-Agent Project Manager) ⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [bobmatnyc/claude-mpm](https://github.com/bobmatnyc/claude-mpm) |
| 技术栈 | Python, MCP |

**核心能力：**
- 47+ 专项 agent + 智能 PM 编排 + 自动任务路由
- 知识图谱记忆管理（kuzu-memory）
- 语义代码搜索
- 集成：Google Workspace, Notion, Confluence, **Slack**

---

### Untether — 远程 Agent 手机控制 ⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [littlebearapps/untether](https://github.com/littlebearapps/untether) |

**核心能力：**
- **Telegram 桥接** Claude Code / Codex / OpenCode / Gemini CLI
- 手机远程任务分配 + 进度流式传输 + 交互式审批按钮
- **成本追踪**（每次运行 + 每日预算）
- 定时任务（cron/webhook）+ 语音输入

---

### OpenWork — 开源 Claude Cowork ⭐⭐⭐

| 属性 | 详情 |
|------|------|
| GitHub | [different-ai/openwork](https://github.com/different-ai/openwork) |
| 官网 | openwork.software |

**核心能力：**
- 开源 Claude Cowork 替代品，专为团队设计
- 会话管理 + 实时流 + 执行计划渲染 + 权限管理 + 模板
- 桌面 UI 连接本地栈

---

## 完整对比矩阵

| 项目 | Agent 编码 | 任务管理 | 远程分配 | 成本追踪 | Web UI | 团队 | IM 集成 |
|------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| **OpenHands** | ✅ | ✅(v1.5) | ✅(API/GUI/GH) | ✅ | ✅ | ✅(RBAC) | Slack,Jira |
| **Composio Orchestrator** | ✅ | ✅(有状态) | ✅(CLI/Dashboard) | ✅ | ✅ | ✅ | GH,Linear |
| **Agent Swarm** | ✅ | ✅(优先队列) | ✅(Slack/GH/Email) | 部分 | API | ✅ | Slack,GH,GL,Email |
| **OpenSwarm** | ✅ | ✅(Linear) | ✅(Discord) | 部分 | ✅ | ✅ | Discord,Linear |
| **Open SWE** | ✅ | ✅(issue 驱动) | ✅(Slack/Linear) | 部分 | ✅ | ✅ | Slack,Linear |
| **Ruflo** | ✅ | ✅(PM 路由) | ✅(MCP) | ✅(成本路由) | Dashboard | ✅ | MCP |
| **Dorothy** | ✅ | ✅(Kanban) | ✅ | - | 桌面 | - | GH triggers |
| **Vibe Kanban** | ✅ | ✅(Kanban) | ✅ | - | ✅ | 工作区 | - |
| **Mission Control** | ✅ | ✅(6 列 Kanban) | ✅ | ✅ | ✅ | ✅ | OpenClaw |
| **Overstory** | ✅ | ✅(Seeds) | ✅(CLI) | - | Dashboard | 多 agent | SQLite 邮件 |
| **Squad** | ✅ | ✅(团队文件) | ✅(GH issues) | - | via GH | ✅(repo 原生) | GH |
| **Claude MPM** | ✅ | ✅(47+ agents) | ✅(CLI) | - | Dashboard | ✅ | Slack,Notion |

---

## 八、推荐复用策略

### 方案 A：基于 Composio Agent Orchestrator 扩展（推荐）

```
Composio Agent Orchestrator (核心编排)
  + Plane (任务管理层，MCP 对接)
  + cc-connect platform/ (IM 桥接，10 平台)
  + claude-code-action (代码审查)
  + E2B/Daytona (沙箱执行)
```

**优势：** Orchestrator 已有插件架构（8 个可插拔插槽），agent 无关，tracker 无关
**工作量：** 写 Plane tracker 插件 + IM bridge 插件 + 定制 Dashboard

### 方案 B：基于 OpenSwarm 扩展

```
OpenSwarm (Claude Code 编排 + Linear + Discord)
  + 替换 Linear → Plane (更灵活)
  + 替换 Discord → cc-connect (多 IM 平台)
  + 增加 Web Dashboard (参考 Mission Control)
  + 增加 Review Pipeline
```

**优势：** 已有 Worker/Reviewer 配对、长期记忆、实时 Dashboard
**工作量：** Fork + 替换集成层 + 增强 Dashboard

### 方案 C：基于 OpenHands 扩展

```
OpenHands (最成熟的 AI 编码平台)
  + 增强 TaskTrackerTool (v1.5 已有基础)
  + 添加 Plane 集成 (任务同步)
  + 添加 IM Bridge (cc-connect/Channels)
  + 添加 Review Pipeline
```

**优势：** 69K stars，最成熟，完整沙箱，V1 SDK
**工作量：** 在已有平台上添加团队工作流层

### 方案 D：从零整合最佳组件

```
自建 Orchestrator (TypeScript/Node.js)
  + Claude Agent SDK (agent 运行时)
  + Plane API/MCP (任务管理)
  + Gitea API/MCP (代码管理)
  + cc-connect platform/ (IM 桥接)
  + claude-code-action (审查)
  + Temporal (持久执行)
  + E2B (沙箱)
  + Refine/Next.js (Dashboard)
```

**优势：** 最大灵活性，完全控制
**工作量：** 最大，但每个组件都是最佳选择

---

## 九、对比总结

| 维度 | 方案 A | 方案 B | 方案 C | 方案 D |
|------|--------|--------|--------|--------|
| **开发周期** | 3-4 周 | 3-4 周 | 4-6 周 | 8-12 周 |
| **灵活性** | 高（插件架构） | 中 | 中 | 最高 |
| **成熟度** | 低（新项目） | 低（新项目） | 高（69K stars） | 取决于实现 |
| **Agent 支持** | 多种 | Claude Code | 多种 | 多种 |
| **IM 支持** | 需添加 | Discord | 需添加 | 10+ 平台 |
| **任务管理** | 外部(GH/Linear) | Linear | 内建(基础) | Plane(完整) |
| **团队功能** | SSE Dashboard | Dashboard | Web UI | 自定义 |

---

## 十、参考资源列表

### 直接相关项目
- [Composio Agent Orchestrator](https://github.com/ComposioHQ/agent-orchestrator)
- [OpenSwarm](https://github.com/Intrect-io/OpenSwarm)
- [Dorothy](https://github.com/Charlie85270/Dorothy)
- [Vibe Kanban](https://github.com/BloopAI/vibe-kanban)
- [Mission Control](https://github.com/crshdn/mission-control)
- [Open SWE](https://github.com/langchain-ai/open-swe)
- [OpenHands](https://github.com/OpenHands/OpenHands)
- [OpenCode](https://github.com/anomalyco/opencode)

### 任务管理基座
- [Plane](https://github.com/makeplane/plane) + [Plane MCP Server](https://github.com/makeplane/plane-mcp-server)
- [Gitea](https://github.com/go-gitea/gitea) + [Gitea MCP](https://gitea.com/gitea/gitea-mcp)
- [Huly](https://github.com/hcengineering/platform)

### 工作流/基础设施
- [n8n](https://github.com/n8n-io/n8n)
- [Temporal](https://github.com/temporalio/temporal)
- [E2B](https://github.com/e2b-dev/E2B)
- [Daytona](https://github.com/daytonaio/daytona)
- [Composio](https://github.com/ComposioHQ/composio)
- [Activepieces](https://github.com/activepieces/activepieces)

### IM 桥接
- [cc-connect](https://github.com/chenhg5/cc-connect)
- [Claude Code Channels](https://code.claude.com/docs/en/channels)
- [OpenClaw](https://github.com/openclaw/openclaw)

### Agent 框架
- [Claude Agent SDK](https://github.com/anthropics/claude-agent-sdk-typescript)
- [LangGraph](https://github.com/langchain-ai/langgraph)
- [CrewAI](https://github.com/crewAIInc/crewAI)
- [Mastra](https://github.com/mastra-ai/mastra)

### Awesome Lists
- [awesome-ai-agents (e2b)](https://github.com/e2b-dev/awesome-ai-agents)
- [awesome-agent-orchestrators](https://github.com/andyrewlee/awesome-agent-orchestrators)
- [awesome-ai-software-development-agents](https://github.com/flatlogic/awesome-ai-software-development-agents)
