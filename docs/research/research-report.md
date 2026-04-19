# Agent 驱动开发管理工具 — 技术调研报告

> 调研日期：2026-03-22

---

## 一、项目目标

搭建一套 **Agent 驱动的开发管理工具**，核心能力：
1. **AI 任务管理** — 让 AI 管理和追踪团队任务（分解、分配、进度跟踪）
2. **远程 Agent 编程** — 团队成员通过 IM/Web 远程调用 Agent 进行编程作业
3. **代码审查集成** — 接入 Claude Reviewer 等自动化审查流水线
4. **组件复用** — 复用 cc-connect 的 IM 连接器、Agent 适配器等成熟模块

---

## 二、技术生态全景

### 2.1 Anthropic 官方工具栈

| 工具 | 用途 | 状态 |
|------|------|------|
| **Claude Agent SDK** (`@anthropic-ai/claude-agent-sdk`) | 编程式 Agent 运行时，`query()` API | 稳定，TS v0.2.71 / Py v0.1.48 |
| **Claude Code CLI** (`@anthropic-ai/claude-code`) | Agent 底层运行时（SDK 依赖） | 稳定 |
| **Claude Code Review** | 多 Agent 并行代码审查（Team/Enterprise） | 研究预览，$15-25/次 |
| **claude-code-action** | 免费 GitHub Action，响应 `@claude` | 开源，稳定 |
| **claude-code-security-review** | 安全漏洞专项审查 Action | 开源 |
| **Remote Control** | 手机/Web 远程控制本地 Claude Code | 研究预览，需 Max 计划 |
| **Agent Teams** | 多 Agent 协作（team lead + teammates） | 实验性 |
| **MCP** | Agent-工具通信协议 | 稳定，1000+ 社区服务器 |

### 2.2 社区工具

| 工具 | 用途 | 技术栈 |
|------|------|--------|
| **cc-connect** | 本地 Agent ↔ IM 平台桥接 | Go，MIT，2.4k stars |
| **claude-review-loop** | 多 Agent 交叉审查（Claude + Codex） | Claude Code 插件 |
| **Claude-to-IM-skill** | Claude Code IM 技能（Telegram/Discord/飞书/QQ） | Claude Code Skill |
| **claude-code-router** | 多模型提供商路由 | 与 cc-connect 集成 |

### 2.3 竞品参考

| 平台 | 架构特点 |
|------|----------|
| **Linear + AI** | 项目管理 hub + Agent 委派，MCP 集成，Agent 作为 delegate |
| **Cursor Background Agents** | 三层架构：Root Planner → Sub-Planner → Worker，隔离上下文 |
| **GitHub Copilot Coding Agent** | Issue → 自动编码 → PR → 响应审查反馈 |
| **Devin** | 全自主 AI 工程师，云端 IDE，可并行多实例 |

---

## 三、核心技术组件深度分析

### 3.1 Claude Agent SDK — 编程式 Agent 控制

```typescript
import { query } from "@anthropic-ai/claude-agent-sdk";

// 启动一个 agent 会话
for await (const message of query({
  prompt: "实现用户认证模块",
  options: {
    allowedTools: ["Read", "Edit", "Write", "Bash", "Glob", "Grep"],
    cwd: "/path/to/project",
    maxTurns: 20,
    maxBudgetUsd: 5.0,
    permissionMode: "bypassPermissions",
    allowDangerouslySkipPermissions: true,
  }
})) {
  if ("result" in message) console.log(message.result);
}
```

**关键能力：**
- `query()` 返回 AsyncGenerator，流式输出 agent 消息
- **Subagents**：内置多 agent 机制，支持并发、独立上下文
- **Session 管理**：`resume`/`sessionId`/`forkSession` 支持会话持久化
- **MCP 集成**：`mcpServers` 参数直接挂载自定义工具
- **Hooks**：`PreToolUse`/`PostToolUse`/`Stop` 生命周期回调
- **成本控制**：`maxBudgetUsd`/`maxTurns` 硬性上限
- **自定义工具**：`createSdkMcpServer` + Zod schema 创建进程内工具

### 3.2 cc-connect — 可复用组件

```
cc-connect/
├── agent/       # Agent 适配器（Claude/Codex/Cursor/Gemini/Qoder/OpenCode/iFlow）
├── platform/    # IM 平台连接器（飞书/钉钉/Telegram/Slack/Discord/企微/LINE/QQ/微信）
├── core/        # 核心桥接逻辑
├── daemon/      # 守护进程模式
├── config/      # TOML 配置系统
└── cmd/         # CLI 入口
```

**可复用部分：**
1. **platform/ 连接器** — 10 个 IM 平台的成熟适配器（WebSocket/长轮询/Webhook/Stream）
2. **agent/ 适配器** — 统一的 Agent 通信接口（spawn subprocess + stdin/stdout）
3. **会话管理** — 每用户独立会话、历史记录、上下文压缩
4. **多项目架构** — 一个进程管理多项目，各有独立 Agent + 平台组合
5. **守护进程** — 后台持续运行

### 3.3 Claude Reviewer 体系

**三层审查方案：**

| 层级 | 方案 | 适用场景 |
|------|------|----------|
| **轻量** | `claude-code-action` GitHub Action | 所有 PR，免费，单 Agent |
| **深度** | Claude Code Review（官方多 Agent） | 关键 PR，付费，多 Agent 并行 + 验证 |
| **自定义** | Agent SDK 构建专项审查 Agent | 特殊需求（安全/性能/合规） |

**Claude Code Review 架构（官方）：**
```
PR 开启
  ↓
并行派发专项 Agent（逻辑错误/边界条件/API 误用/认证漏洞/规范违反）
  ↓
验证步骤：尝试反驳每个发现（假阳性过滤）
  ↓
去重 + 严重性排序
  ↓
发布 inline PR 评论
```

---

## 四、推荐系统架构

### 4.1 整体架构：Orchestrator-Worker + IM Bridge

```
┌─────────────────────────────────────────────────────────────┐
│                    Web Dashboard / Admin UI                   │
│              （任务看板、进度追踪、审查报告、成本监控）          │
└──────────────────────────┬──────────────────────────────────┘
                           │ REST/WebSocket API
┌──────────────────────────▼──────────────────────────────────┐
│                   Orchestrator Service                        │
│  ┌─────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────┐  │
│  │Task Mgr │  │Agent Pool│  │Session   │  │Cost/Budget  │  │
│  │(分解/    │  │Manager   │  │Store     │  │Controller   │  │
│  │ 分配/    │  │(spawn/   │  │(Redis)   │  │(maxBudget   │  │
│  │ 追踪)   │  │ monitor) │  │          │  │ tracking)   │  │
│  └─────────┘  └──────────┘  └──────────┘  └─────────────┘  │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              Task Queue (Redis Streams)                  │ │
│  └─────────────────────────────────────────────────────────┘ │
└──────┬──────────┬──────────┬──────────┬─────────────────────┘
       │          │          │          │
  ┌────▼───┐ ┌───▼────┐ ┌───▼────┐ ┌───▼────┐
  │Worker  │ │Worker  │ │Worker  │ │Review  │
  │Agent 1 │ │Agent 2 │ │Agent 3 │ │Agent   │
  │(SDK    │ │(SDK    │ │(SDK    │ │(SDK    │
  │query())│ │query())│ │query())│ │query())│
  └────┬───┘ └───┬────┘ └───┬────┘ └───┬────┘
       │         │          │          │
       └─────────┴──────────┴──────────┘
                      │
              ┌───────▼───────┐
              │  Git Repos     │
              │  (branch-per-  │
              │   agent)       │
              └───────────────┘

┌─────────────────────────────────────────────────────────────┐
│                 IM Bridge (复用 cc-connect)                   │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────┐  │
│  │ 飞书   │ │ 钉钉   │ │Telegram│ │ Slack  │ │ 企业微信 │  │
│  └────────┘ └────────┘ └────────┘ └────────┘ └──────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 核心模块设计

#### 模块 1：Task Manager（任务管理器）

```typescript
interface Task {
  id: string;
  title: string;
  description: string;
  status: "pending" | "assigned" | "in_progress" | "review" | "done";
  assignee: "human" | AgentId;
  priority: "critical" | "high" | "medium" | "low";
  parentTask?: string;          // 支持任务分解
  blockedBy?: string[];         // 依赖关系
  branch?: string;              // 关联 Git 分支
  sessionId?: string;           // Agent SDK session ID（可恢复）
  budgetUsd?: number;           // 成本预算
  maxTurns?: number;            // Agent 循环上限
  reviewRequired: boolean;      // 是否需要审查
  metadata: Record<string, any>;
}
```

**能力：**
- AI 自动分解大任务为子任务（用 Claude API 分析需求文档）
- 自动分配：根据任务类型选择合适的 Agent 配置（Subagent 模式）
- 依赖追踪：blockedBy 机制，前置任务完成后自动触发后续
- 成本追踪：汇总每个任务/Sprint 的 token 消耗和美元成本

#### 模块 2：Agent Pool Manager（Agent 池管理器）

```typescript
class AgentPoolManager {
  // 启动 coding agent
  async spawnCodingAgent(task: Task): Promise<AgentSession> {
    const session = query({
      prompt: buildTaskPrompt(task),
      options: {
        allowedTools: ["Read", "Edit", "Write", "Bash", "Glob", "Grep", "Agent"],
        cwd: task.repoPath,
        maxTurns: task.maxTurns ?? 30,
        maxBudgetUsd: task.budgetUsd ?? 5.0,
        agents: this.getSubagentConfig(task),
        hooks: { Stop: [this.onAgentComplete.bind(this)] },
        mcpServers: this.getMcpServers(),
      }
    });
    return this.trackSession(task.id, session);
  }

  // 启动 review agent
  async spawnReviewAgent(prUrl: string): Promise<ReviewResult> {
    const result = query({
      prompt: `Review this PR: ${prUrl}. Focus on logic errors, security, and edge cases.`,
      options: {
        allowedTools: ["Read", "Grep", "Glob", "Bash"],
        outputFormat: reviewResultSchema, // Zod schema → 结构化输出
        agents: {
          "security-reviewer": { description: "安全审查", prompt: "...", tools: ["Read", "Grep"] },
          "logic-reviewer": { description: "逻辑审查", prompt: "...", tools: ["Read", "Grep", "Glob"] },
        }
      }
    });
    return parseReviewResult(result);
  }
}
```

#### 模块 3：IM Bridge（IM 桥接，复用 cc-connect）

**复用策略：**
- 直接复用 cc-connect 的 `platform/` 连接器（Go 模块）
- 替换 cc-connect 的 `agent/` 层，改为对接我们的 Orchestrator API
- 或者：Fork cc-connect，将 `core/` 层改为通过 HTTP/gRPC 调用 Orchestrator

```
用户在飞书发消息: "帮我修复 auth 模块的 bug #123"
  ↓
cc-connect platform/feishu 接收
  ↓
转发到 Orchestrator API（替代原 agent/ 层）
  ↓
Orchestrator 创建 Task → 分配 Agent → 执行 → 流式返回结果
  ↓
cc-connect 回传到飞书
```

#### 模块 4：Review Pipeline（审查流水线）

```
Agent 完成编码
  ↓
自动创建 PR (gh pr create)
  ↓
触发审查流水线：
  ├── claude-code-action (轻量审查，所有 PR)
  ├── claude-code-security-review (安全扫描)
  └── 自定义 Review Agent (项目规范审查)
  ↓
汇总审查结果
  ↓
通过 IM Bridge 通知任务创建者
  ↓
人工决策：合并 / 要求修改 / 拒绝
```

### 4.3 技术选型

| 组件 | 推荐技术 | 理由 |
|------|----------|------|
| **后端框架** | Node.js (Fastify/Hono) | 与 Agent SDK (TS) 同生态，async generator 原生支持 |
| **任务队列** | Redis Streams | 消费者组、消息确认、持久化，轻量级 |
| **会话存储** | Redis + SQLite | Redis 缓存活跃会话，SQLite 持久化历史 |
| **IM 桥接** | cc-connect (Go) | 成熟的 10 平台适配器，守护进程模式 |
| **代码审查** | claude-code-action + Agent SDK 自定义 | 免费层 + 深度层组合 |
| **前端面板** | Next.js | 任务看板 + 实时状态 + 成本监控 |
| **Agent 运行时** | Claude Agent SDK (TS) | 官方支持，subagent/session/hook 完整 |
| **协议层** | MCP | 标准化工具集成 |
| **版本控制** | branch-per-agent | 避免冲突，合并时审查 |

---

## 五、实施路径（建议分三期）

### Phase 1：MVP — 核心 Agent 编程能力（2-3 周）

- [ ] 搭建 Node.js 后端 + Redis
- [ ] 封装 Agent SDK `query()` 为 REST API
- [ ] 实现基本任务 CRUD + 状态流转
- [ ] 接入 1 个 IM 平台（飞书或 Slack，复用 cc-connect 连接器）
- [ ] 用户通过 IM 发送编程指令 → Agent 执行 → 返回结果

### Phase 2：任务管理 + 审查流水线（2-3 周）

- [ ] AI 任务分解（大任务 → 子任务）
- [ ] 多 Agent 并行执行（Agent Pool）
- [ ] 接入 claude-code-action 进行自动 PR 审查
- [ ] 自定义 Review Agent（项目规范）
- [ ] Web Dashboard（任务看板 + 成本监控）

### Phase 3：团队协作 + 高级功能（3-4 周）

- [ ] 多 IM 平台支持（扩展到钉钉/企微/Telegram）
- [ ] Agent Teams 集成（多 Agent 协作模式）
- [ ] RBAC 权限控制（谁能触发哪些 Agent）
- [ ] 成本预算管理（团队/项目/个人维度）
- [ ] 审查报告汇总 + 质量趋势分析
- [ ] Session 恢复 + 任务断点续做

---

## 六、关键风险与对策

| 风险 | 影响 | 对策 |
|------|------|------|
| Agent SDK 每次 query 启动子进程 | 资源消耗大 | Agent 池复用 + 会话恢复避免重复启动 |
| Token 成本不可控 | 预算超支 | `maxBudgetUsd` 硬限 + 实时监控告警 |
| Agent 代码质量不稳定 | 引入 bug | 强制审查流水线 + 测试必须通过 |
| IM 平台 API 变更 | 连接器失效 | 复用 cc-connect 社区维护，及时跟进 |
| 多 Agent 并发写同一文件 | 冲突 | branch-per-agent + 文件锁 + 空间隔离 |
| Windows 命令行长度限制 (8191 char) | 长 prompt 失败 | 使用 prompt 文件而非命令行参数 |
| 模型锁定 Claude | 供应商依赖 | cc-connect 已支持多 Agent，可扩展 |

---

## 七、参考资源

### 官方文档
- [Claude Agent SDK](https://platform.claude.com/docs/en/agent-sdk/overview)
- [Claude Code Review](https://code.claude.com/docs/en/code-review)
- [claude-code-action](https://github.com/anthropics/claude-code-action)
- [Claude Code Remote Control](https://code.claude.com/docs/en/remote-control)
- [Claude Code Agent Teams](https://code.claude.com/docs/en/agent-teams)
- [MCP 协议](https://modelcontextprotocol.io/)

### 社区项目
- [cc-connect](https://github.com/chenhg5/cc-connect) — IM 桥接（Go, MIT）
- [claude-review-loop](https://github.com/hamelsmu/claude-review-loop) — 多 Agent 交叉审查
- [claude-code-router](https://github.com/musistudio/claude-code-router) — 多模型路由

### 竞品参考
- [Linear AI Agents](https://linear.app/ai) — 项目管理 + Agent 委派
- [Cursor Background Agents](https://docs.cursor.com/en/background-agent) — 三层 Planner-Worker 架构
- [GitHub Copilot Coding Agent](https://github.com/features/copilot) — Issue → PR 自动化
