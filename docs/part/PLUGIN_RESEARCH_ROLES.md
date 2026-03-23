# AgentForge 数字员工角色自定义 - 调研报告

> 调研时间: 2026-03-23
> 调研范围: AI Agent 角色定义、能力组合、协作模式、知识管理、安全约束、现有产品分析

---

## 目录

1. [角色定义模式](#1-角色定义模式)
2. [能力组合机制](#2-能力组合机制)
3. [工作流中的角色协作](#3-工作流中的角色协作)
4. [角色专属知识和记忆](#4-角色专属知识和记忆)
5. [行为约束和安全](#5-行为约束和安全)
6. [现有产品的角色系统](#6-现有产品的角色系统)
7. [设计模式总结](#7-设计模式总结)
8. [AgentForge 推荐方案](#8-agentforge-推荐方案)
9. [角色定义 Schema 草案](#9-角色定义-schema-草案)

---

## 1. 角色定义模式

### 1.1 System Prompt 模板化（GPTs、Coze Bot）

最基础的角色定义方式，通过系统提示词定义角色的身份、行为和约束。

**核心要素：**
- **身份声明**：角色名称、职责描述
- **行为规则**：输出格式、语气风格、限制条件
- **上下文注入**：领域知识、参考文档
- **变量替换**：`{topic}`、`{user_name}` 等动态占位符

**OpenAI GPTs 的实现：**
- Instructions 字段定义行为规则（等效于 API 的 system prompt）
- Knowledge Files 上传参考文档（最多 20 个文件，单文件 512MB）
- Capabilities 开关控制功能（Web Search、Code Interpreter、Image Generation）
- Actions 连接外部 API
- Prompt Starters 提供交互示例

**优势：** 简单直观，无需编码，快速迭代
**劣势：** 缺乏结构化，难以版本控制和复用，能力边界模糊

### 1.2 角色卡片/Profile（CrewAI Agent 定义）

将角色抽象为结构化配置，包含明确的字段定义。

**CrewAI YAML Agent 定义：**
```yaml
researcher:
  role: '{topic} Senior Data Researcher'
  goal: 'Uncover cutting-edge developments in {topic}'
  backstory: >
    You are a seasoned researcher with expertise in
    identifying emerging trends and analyzing complex data.
  tools:
    - SerperDevTool
    - WebScraperTool
  verbose: true
  allow_delegation: true
  max_iter: 15
  reasoning: true        # 2025新增: 启用策略规划
  inject_date: true      # 自动注入日期上下文
```

**核心字段：**
| 字段 | 说明 |
|------|------|
| `role` | 角色职能标题 |
| `goal` | 角色目标（驱动决策的核心） |
| `backstory` | 背景故事（提供上下文和个性） |
| `tools` | 可用工具列表 |
| `allow_delegation` | 是否允许委派任务 |
| `max_iter` | 最大推理迭代次数 |

**优势：** 结构化、可版本控制、支持变量替换、代码与配置分离
**劣势：** 表达能力有限，复杂行为需要补充代码

### 1.3 技能树/能力图谱

将角色能力组织为层次化的树状结构，支持按需加载。

**SKILL.md 模式（2025-2026 新兴标准）：**
```
project/
  skills/
    data_science/
      SKILL.md           # 元数据 + 入口指令
      pandas_expert/
        SKILL.md          # 子技能
      visualization/
        SKILL.md
      statistical_analysis/
        SKILL.md
```

**关键设计原则：**
- **渐进式披露（Progressive Disclosure）**：先加载轻量元数据，匹配后再注入完整指令
- **避免"上帝技能"反模式**：每个技能聚焦单一职责
- **确定性 vs 智能分离**：模板/规则用确定性逻辑，推理/创造用 LLM
- **可组合性**：技能可嵌套、可链式调用

**五种技能设计模式（Google ADK 生态总结）：**
1. **Tool Wrapper**：为特定库/API 提供按需上下文
2. **Generator**：使用模板和参考强制一致输出
3. **Reviewer**：将检查清单与检查方法分离
4. **Pipeline**：多步骤工作流，带显式门控条件
5. **Inversion**：先收集变量，再填充模板

### 1.4 角色继承和组合

通过继承基础角色并叠加特定能力来创建新角色。

**模式示例：**
```
BaseAgent（基础能力：对话、推理）
  ├── DeveloperAgent（+代码生成、+代码审查）
  │     ├── FrontendDeveloper（+React、+CSS）
  │     └── BackendDeveloper（+API设计、+数据库）
  ├── DesignerAgent（+UI设计、+原型）
  └── PMAgent（+需求分析、+项目管理）
```

**实现方式：**
- Prompt 拼接：基础 prompt + 角色专属 prompt + 任务 prompt
- 配置合并：基础配置 deep merge 角色配置
- Mixin 组合：能力包按需组合（类似 trait/mixin 模式）

---

## 2. 能力组合机制

### 2.1 Tool 到角色的映射

借鉴 RBAC 思想，将工具权限组织为权限组，按角色分配。

**分层模型：**
```
工具（Tool）→ 权限（Permission）→ 能力包（Capability Pack）→ 角色（Role）

示例:
  [基础能力包]
    - text_generation
    - web_search
    - file_read

  [开发能力包] extends [基础能力包]
    - code_execution
    - git_operations
    - terminal_access
    - code_review

  [前端开发角色] = [开发能力包] + [设计能力包(部分)]
    - code_execution
    - git_operations
    - figma_integration (from 设计能力包)
```

### 2.2 预设能力包 vs 自由组合

| 模式 | 优势 | 劣势 | 适用场景 |
|------|------|------|----------|
| **预设能力包** | 开箱即用、经过验证 | 灵活性低 | 标准角色、快速启动 |
| **自由组合** | 完全灵活 | 配置复杂、可能冲突 | 高级用户、特殊需求 |
| **混合模式** | 平衡灵活与便捷 | 实现复杂 | 推荐方案 |

**推荐：混合模式** — 提供预设角色模板，允许用户在模板基础上增删能力。

### 2.3 能力依赖和冲突管理

```yaml
capabilities:
  code_execution:
    requires: [terminal_access]
    conflicts_with: [sandbox_only_mode]
  database_write:
    requires: [database_read]
    risk_level: high
    approval_required: true
```

**冲突类型：**
- **互斥冲突**：两个能力不可同时启用（如 sandbox_only 与 full_access）
- **依赖冲突**：能力 A 需要能力 B，但 B 被禁用
- **资源冲突**：多个能力竞争同一资源（如文件锁）

---

## 3. 工作流中的角色协作

### 3.1 CrewAI Process 模式

CrewAI 提供两种核心编排模式：

**Sequential（顺序执行）：**
```python
crew = Crew(
    agents=[researcher, writer, editor],
    tasks=[research_task, write_task, edit_task],
    process=Process.sequential  # 任务按顺序流转
)
```

**Hierarchical（层级管理）：**
```python
crew = Crew(
    agents=[researcher, writer, editor],
    tasks=[research_task, write_task, edit_task],
    process=Process.hierarchical,
    manager_llm=ChatOpenAI(model="gpt-4")  # 自动创建管理者
)
```

### 3.2 AutoGen 群聊协作

AutoGen 的群聊架构支持动态多方对话：

**核心模式：**
- **RoundRobinGroupChat**：轮询制，每个 Agent 依次发言
- **SelectorGroupChat**：由 LLM 或规则选择下一个发言者
- **嵌套群聊**：群聊可作为参与者嵌套到更高层群聊

**架构特点：**
- Core 层基于 Actor 模型的异步消息传递
- AgentChat 层提供预构建 Agent 类型（AssistantAgent、UserProxyAgent、WebSurferAgent）
- 2025 演进为 Microsoft Agent Framework，融合 Semantic Kernel

### 3.3 LangGraph 状态图编排

LangGraph 将工作流建模为有向图：

**核心概念：**
- **节点（Node）**：Agent、函数、决策点
- **边（Edge）**：数据流向，支持条件路由
- **状态（State）**：TypedDict 定义的共享状态，贯穿整个图

**关键模式：**
| 模式 | 说明 |
|------|------|
| Pipeline | 顺序移交 |
| Hub-and-Spoke | 中心协调器分派任务 |
| 条件路由 | 基于 Agent 输出动态选路 |
| 并行执行 | 多 Agent 同时处理，结果在下游合并 |
| Human-in-the-Loop | 暂停等待人工审核 |

**状态管理：**
- 显式 reducer 驱动的状态 schema
- 检查点机制（Checkpointing）支持持久化和恢复
- 与 LangSmith 集成提供可观测性

### 3.4 角色间通信协议对比

| 框架 | 通信模式 | 状态管理 | 人工介入 |
|------|----------|----------|----------|
| CrewAI | 任务委派 + 结果传递 | 任务级上下文 | Callback |
| AutoGen | 消息传递（Actor 模型） | 会话历史 | UserProxyAgent |
| LangGraph | 共享状态图 | 显式 TypedDict | 检查点暂停 |
| Dify | 节点间变量传递 | 工作流变量 | 审批节点 |

**AgentForge 启示：** 推荐采用混合模式 —— 结构化任务用状态图编排，开放式讨论用群聊模式，日常任务用顺序流程。

---

## 4. 角色专属知识和记忆

### 4.1 RAG 集成模式

**知识库绑定方式：**
```yaml
agent:
  name: "法务顾问"
  knowledge_bases:
    - id: "legal_kb"
      type: "vector"
      description: "公司法律文档库"
      access: "read"
      retrieval_strategy: "hybrid"  # dense + sparse
    - id: "contract_templates"
      type: "structured"
      description: "合同模板库"
      access: "read"
```

**检索策略演进：**
- **Vanilla RAG**：查询 → 检索 → 生成
- **Agentic RAG**：Agent 智能决策何时检索、检索什么
- **GraphRAG**：基于知识图谱的多跳推理检索

### 4.2 长期记忆机制

**记忆类型分层：**

| 记忆类型 | 说明 | 存储方式 | 生命周期 |
|----------|------|----------|----------|
| **短期/工作记忆** | 当前会话上下文 | 上下文窗口 | 会话内 |
| **情景记忆** | 过往交互记录 | 向量库 + 元数据 | 跨会话 |
| **语义记忆** | 结构化事实知识 | 知识图谱/向量库 | 持久 |
| **程序记忆** | 技能和行为模式 | 规则库/代码 | 持久 |

**实现推荐：**
- 使用 MCP（Model Context Protocol）作为知识访问的标准化接口
- 会话摘要自动生成并存储为长期记忆
- 经验累积通过反馈循环更新语义记忆

### 4.3 团队共享知识 vs 角色私有知识

```
知识层次:
  ├── 全局知识（所有角色共享）
  │     ├── 公司文化/规范
  │     ├── 产品文档
  │     └── 通用工具使用手册
  ├── 团队知识（同组角色共享）
  │     ├── 项目上下文
  │     ├── 团队约定
  │     └── 历史决策记录
  └── 角色私有知识
        ├── 专业领域文档
        ├── 个人工作记忆
        └── 技能特定参考资料
```

---

## 5. 行为约束和安全

### 5.1 角色权限边界

借鉴 RBAC 但增强为动态、上下文感知的权限模型：

**四层权限模型：**
1. **身份认证（Identity）**：Agent 身份验证
2. **角色权限（Role Permissions）**：基于角色的静态权限
3. **上下文策略（Context Policies）**：基于运行时条件的动态权限
4. **审计日志（Audit Trail）**：所有操作可追溯

**关键原则：**
- **最小权限原则**：仅授予完成任务所需的最小权限集
- **时间限定权限**：任务完成后自动回收临时权限
- **任务范围权限**：权限与具体任务绑定，任务结束即失效

### 5.2 输出过滤和审查

```yaml
output_policies:
  - type: "content_filter"
    rules:
      - no_pii_exposure          # 不泄露个人信息
      - no_credential_output     # 不输出凭证
      - language_appropriate     # 语言得体
  - type: "action_review"
    rules:
      - destructive_actions_require_approval  # 破坏性操作需审批
      - external_api_calls_logged             # 外部 API 调用记录
  - type: "quality_gate"
    rules:
      - code_must_pass_lint      # 代码必须通过 lint
      - responses_must_cite_sources  # 回复需引用来源
```

### 5.3 预算和资源限制

```yaml
resource_limits:
  token_budget:
    per_task: 50000
    per_day: 500000
    per_month: 10000000
  api_calls:
    per_minute: 10
    per_hour: 100
  execution_time:
    per_task: "30m"
    per_day: "8h"
  cost_limit:
    per_task: "$5"
    per_day: "$50"
    alert_threshold: 0.8  # 80% 时告警
```

### 5.4 安全策略模板

```yaml
security_profiles:
  standard:
    network_access: "restricted"
    file_access: "project_scope"
    code_execution: "sandboxed"
    external_apis: "whitelist_only"

  high_security:
    network_access: "none"
    file_access: "read_only"
    code_execution: "disabled"
    external_apis: "none"
    human_approval: "all_actions"

  development:
    network_access: "full"
    file_access: "project_scope"
    code_execution: "sandboxed"
    external_apis: "whitelist_only"
```

---

## 6. 现有产品的角色系统

### 6.1 OpenAI GPTs

| 维度 | 实现 |
|------|------|
| 角色定义 | Instructions 文本框 + Knowledge Files |
| 能力配置 | 开关式（Web Search / Code Interpreter / Image Gen） |
| 工具扩展 | Actions（OpenAPI Schema 定义的外部 API） |
| 知识管理 | 文件上传（20 个文件，单文件 512MB） |
| 协作 | 不支持多 Agent 协作 |
| 限制 | 无结构化角色定义、无版本控制、无团队协作 |

### 6.2 Coze Bot

| 维度 | 实现 |
|------|------|
| 角色定义 | Prompt + 自动生成头像/名称/描述 |
| 能力配置 | Plugins + Workflows + Knowledge + Variables |
| 工具扩展 | 插件市场 + 自定义 API 插件 |
| 知识管理 | 文本/表格/图片知识库 + RAG 检索节点 |
| 协作 | Multi-Agent Mode（专业化分工团队） |
| 策略 | Agent Strategy 插件（CoT/ToT/GoT/ReAct） |
| 特色 | 开源版 Coze Studio（Apache 2.0），可视化工作流，MCP 支持 |

### 6.3 Claude Projects

| 维度 | 实现 |
|------|------|
| 角色定义 | Custom Instructions（等效 system prompt） |
| 知识管理 | Knowledge Files（200K token 上下文窗口） |
| 文件支持 | PDF/DOCX/CSV/TXT/HTML 等，单文件 30MB |
| 协作 | 不支持多 Agent，但支持项目内多会话 |
| 特色 | RAG 增强（Pro/Team 用户），项目级隔离 |
| 限制 | 无工具扩展 API，无结构化角色定义 |

### 6.4 Dify Agent 配置

| 维度 | 实现 |
|------|------|
| 角色定义 | Agent Node（Instruction + Model + Tools） |
| 能力配置 | 可视化拖拽工具选择 + Agent Strategy 插件 |
| 工作流 | 可视化画布 + 条件/循环/HTTP/代码节点 |
| 知识管理 | Knowledge Pipeline + 向量检索 |
| 策略 | Function Calling / ReAct / 自定义策略插件 |
| 特色 | MCP 支持、OAuth、工作流触发器、开源可自部署 |

### 6.5 CrewAI YAML Agent

| 维度 | 实现 |
|------|------|
| 角色定义 | YAML 配置（role/goal/backstory） |
| 能力配置 | Tools 列表 + 代码注册 |
| 工作流 | Sequential / Hierarchical Process |
| 知识管理 | 工具集成 RAG |
| 特色 | 代码与配置分离、变量替换、2025 新增推理模式 |
| 限制 | 缺乏可视化、知识管理相对简单 |

### 6.6 企业 RPA（UiPath / Blue Prism）

| 维度 | 实现 |
|------|------|
| Bot 定义 | 可视化流程设计器 + 预建模板 |
| 能力配置 | Activity/Action 库 + 自定义组件 |
| 工作流 | 状态机 + 流程图 + 序列 |
| 治理 | 企业级审批流、审计日志、合规控制 |
| 特色 | UiPath — 复杂多系统集成；Blue Prism — 治理强、合规优先 |
| 趋势 | 从规则 Bot 向 AI 驱动自主 Agent 演进 |

---

## 7. 设计模式总结

### 模式 1: Template-Profile 模式（模板-档案）

**核心思想：** 角色 = 系统提示模板 + 结构化配置档案

```yaml
# 角色模板 (可继承)
base_template: "digital_employee_v1"
profile:
  name: "Alex"
  role: "Senior Frontend Developer"
  personality: "detail-oriented, collaborative"
  language: "zh-CN"
  response_style: "concise, technical"
```

**适用场景：** 快速创建标准化角色
**代表产品：** OpenAI GPTs, Claude Projects

---

### 模式 2: Capability-Composition 模式（能力组合）

**核心思想：** 角色 = 基础身份 + 可插拔能力包

```yaml
role: "全栈工程师"
capabilities:
  base: ["reasoning", "communication"]
  packages:
    - name: "frontend_dev"
      tools: ["code_editor", "browser_preview", "figma_viewer"]
      knowledge: ["react_docs", "css_reference"]
    - name: "backend_dev"
      tools: ["code_editor", "database_client", "api_tester"]
      knowledge: ["nodejs_docs", "postgresql_docs"]
  custom_tools:
    - my_internal_api_tool
```

**适用场景：** 灵活的角色定制，企业级权限管理
**代表产品：** Coze Bot, Dify Agent

---

### 模式 3: Graph-Orchestrated 模式（图编排）

**核心思想：** 角色在状态图中充当节点，通过边和条件路由协作

```python
# LangGraph 风格
workflow = StateGraph(ProjectState)
workflow.add_node("pm", pm_agent)
workflow.add_node("developer", dev_agent)
workflow.add_node("reviewer", review_agent)
workflow.add_edge("pm", "developer")
workflow.add_conditional_edges("developer", review_router,
    {"approved": END, "rejected": "developer"})
```

**适用场景：** 复杂多步骤工作流，需要条件分支和循环
**代表产品：** LangGraph, Dify Workflow

---

### 模式 4: Hierarchical-Delegation 模式（层级委派）

**核心思想：** Manager Agent 自动分解任务并委派给专业 Agent

```yaml
team:
  manager:
    role: "Project Manager"
    process: hierarchical
    delegation_strategy: "skill_match"
  members:
    - role: "Researcher"
      speciality: "data_gathering"
    - role: "Developer"
      speciality: "code_implementation"
    - role: "QA Engineer"
      speciality: "testing"
```

**适用场景：** 大型项目自动任务分解和团队协调
**代表产品：** CrewAI Hierarchical, AutoGen GroupChat

---

### 模式 5: Skill-Tree Progressive 模式（技能树渐进）

**核心思想：** 角色能力以树状结构组织，按需加载

```
roles/
  frontend_developer/
    ROLE.md                    # 角色元数据
    skills/
      react/
        SKILL.md               # React 开发技能
        hooks/SKILL.md         # 子技能: Hooks
        performance/SKILL.md   # 子技能: 性能优化
      css/
        SKILL.md
        animation/SKILL.md
      testing/
        SKILL.md
```

```yaml
# SKILL.md frontmatter
---
name: react
description: React component development and best practices
requires: [javascript_basics]
tools: [code_editor, browser_preview]
---
# Instructions
When working with React components...
```

**适用场景：** 大规模知识管理，避免上下文窗口溢出
**代表产品：** Claude Code Skills, Block 内部 Skills 市场

---

### 模式 6: Event-Driven Reactive 模式（事件驱动响应）

**核心思想：** 角色定义触发条件和响应行为，按事件激活

```yaml
role: "DevOps Guardian"
triggers:
  - event: "ci_pipeline_failed"
    action: "diagnose_and_fix"
    auto_execute: true
  - event: "pr_created"
    action: "auto_review"
    auto_execute: true
  - event: "security_alert"
    action: "assess_and_notify"
    requires_approval: true
  - event: "deployment_request"
    action: "execute_deployment"
    requires_approval: true
```

**适用场景：** 自动化运维、持续集成、监控响应
**代表产品：** Dify Workflow Triggers, GitHub Actions + AI

---

## 8. AgentForge 推荐方案

### 8.1 总体架构

推荐采用 **"Template-Profile + Capability-Composition + Skill-Tree"** 的三层混合架构：

```
┌─────────────────────────────────────────────┐
│            角色定义层 (Role Definition)        │
│  Template-Profile: 身份、个性、基础行为        │
├─────────────────────────────────────────────┤
│           能力组合层 (Capability Layer)        │
│  预设能力包 + 自由工具组合 + RBAC 权限         │
├─────────────────────────────────────────────┤
│           技能树层 (Skill Tree Layer)          │
│  按需加载专业知识、渐进式披露                   │
├─────────────────────────────────────────────┤
│           知识与记忆层 (Knowledge & Memory)    │
│  共享知识库 + 私有知识 + 多层记忆系统           │
├─────────────────────────────────────────────┤
│           安全与治理层 (Security & Governance) │
│  权限边界 + 输出过滤 + 预算控制 + 审计          │
└─────────────────────────────────────────────┘
```

### 8.2 核心设计决策

| 决策点 | 推荐方案 | 理由 |
|--------|----------|------|
| 角色定义格式 | YAML 配置 + Markdown 技能文件 | 可版本控制，对开发者友好，支持 AI 辅助生成 |
| 能力组合方式 | 预设模板 + 自由组合混合 | 平衡易用性和灵活性 |
| 协作编排 | 状态图为主 + 群聊为辅 | 结构化任务用图编排，开放讨论用群聊 |
| 知识管理 | MCP 协议 + 分层知识库 | 标准化接口，支持未来扩展 |
| 权限模型 | 动态 RBAC + 上下文策略 | 超越静态 RBAC，适应 Agent 动态特性 |
| 记忆系统 | 四层记忆（短期/情景/语义/程序） | 全面覆盖不同记忆需求 |

### 8.3 用户体验设计

- **角色市场（Role Marketplace）**：预建角色模板，一键使用
- **可视化角色编辑器**：拖拽式能力组合 + 实时预览
- **角色测试沙盒**：创建后可立即测试角色行为
- **版本管理**：角色配置 Git 化，支持回滚和分支
- **团队共享**：角色模板可在团队间共享和 fork

---

## 9. 角色定义 Schema 草案

### 9.1 YAML Schema

```yaml
# AgentForge Role Definition Schema v1.0
# 文件路径: roles/{role_id}/role.yaml

apiVersion: agentforge/v1
kind: Role
metadata:
  id: "frontend-developer"
  name: "前端开发工程师"
  version: "1.2.0"
  author: "team-admin"
  tags: ["development", "frontend", "web"]
  icon: "code-bracket"
  description: "专业的前端开发工程师，擅长 React/Vue 生态系统"
  created_at: "2026-01-15T10:00:00Z"
  updated_at: "2026-03-20T14:30:00Z"

# 基础身份
identity:
  role: "Senior Frontend Developer"
  personality: "detail-oriented, collaborative, patient"
  language: "zh-CN"
  response_style:
    tone: "professional"
    verbosity: "concise"
    format_preference: "markdown"

# 系统提示模板
system_prompt: |
  你是一位资深前端开发工程师，拥有 8 年以上的 Web 开发经验。
  你擅长 React/Vue 生态系统，对性能优化和用户体验有深入理解。

  ## 工作原则
  - 代码质量优先，遵循团队规范
  - 先理解需求，再动手编码
  - 主动提出更优方案

# 能力配置
capabilities:
  # 预设能力包
  packages:
    - "web-development"
    - "code-review"
    - "testing"

  # 工具列表
  tools:
    built_in:
      - code_editor
      - terminal
      - browser_preview
      - git_client
    external:
      - figma_viewer
      - npm_registry
    mcp_servers:
      - url: "http://localhost:3001/mcp"
        name: "project-tools"

  # 自定义技能树
  skills:
    - path: "skills/react"
      auto_load: true
    - path: "skills/typescript"
      auto_load: true
    - path: "skills/css"
      auto_load: false  # 按需加载
    - path: "skills/testing"
      auto_load: false

# 知识与记忆
knowledge:
  # 共享知识库
  shared:
    - id: "company-standards"
      type: "vector"
      access: "read"
    - id: "product-docs"
      type: "vector"
      access: "read"

  # 角色私有知识
  private:
    - id: "frontend-best-practices"
      type: "vector"
      sources:
        - "knowledge/react-patterns.md"
        - "knowledge/performance-guide.md"

  # 记忆配置
  memory:
    short_term:
      max_tokens: 128000
    episodic:
      enabled: true
      retention_days: 90
    semantic:
      enabled: true
      auto_extract: true
    procedural:
      enabled: true
      learn_from_feedback: true

# 协作配置
collaboration:
  # 可委派的角色
  can_delegate_to:
    - "backend-developer"
    - "designer"
  # 接受委派来源
  accepts_delegation_from:
    - "project-manager"
    - "tech-lead"
  # 通信偏好
  communication:
    preferred_channel: "structured"  # structured | chat | both
    report_format: "markdown"
    escalation_policy: "auto"  # auto | manual

# 安全与约束
security:
  # 安全配置档
  profile: "development"  # standard | high_security | development

  # 权限边界
  permissions:
    file_access:
      allowed_paths: ["src/", "public/", "tests/"]
      denied_paths: [".env", "secrets/"]
    network:
      allowed_domains: ["github.com", "npmjs.com", "*.cdn.com"]
    code_execution:
      sandbox: true
      allowed_languages: ["javascript", "typescript", "shell"]

  # 输出过滤
  output_filters:
    - no_credentials
    - no_pii
    - code_lint_check

  # 资源限制
  resource_limits:
    token_budget:
      per_task: 100000
      per_day: 1000000
    api_calls:
      per_minute: 20
    execution_time:
      per_task: "60m"
    cost_limit:
      per_day: "$30"

# 行为触发器
triggers:
  - event: "pr_created"
    action: "auto_review"
    condition: "pr.files.any(f => f.path.startsWith('src/frontend/'))"
  - event: "issue_assigned"
    action: "analyze_and_plan"
    condition: "issue.labels.includes('frontend')"

# 继承（可选）
extends: "base-developer"
overrides:
  identity.role: "Senior Frontend Developer"
  capabilities.packages:
    add: ["design-integration"]
    remove: ["backend-development"]
```

### 9.2 JSON Schema (用于 API 和验证)

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "AgentForge Role Definition",
  "type": "object",
  "required": ["apiVersion", "kind", "metadata", "identity", "capabilities"],
  "properties": {
    "apiVersion": {
      "type": "string",
      "pattern": "^agentforge/v\\d+$"
    },
    "kind": {
      "type": "string",
      "enum": ["Role"]
    },
    "metadata": {
      "type": "object",
      "required": ["id", "name", "version"],
      "properties": {
        "id": { "type": "string", "pattern": "^[a-z0-9-]+$" },
        "name": { "type": "string" },
        "version": { "type": "string", "pattern": "^\\d+\\.\\d+\\.\\d+$" },
        "author": { "type": "string" },
        "tags": { "type": "array", "items": { "type": "string" } },
        "description": { "type": "string" }
      }
    },
    "identity": {
      "type": "object",
      "required": ["role"],
      "properties": {
        "role": { "type": "string" },
        "personality": { "type": "string" },
        "language": { "type": "string" },
        "response_style": { "type": "object" }
      }
    },
    "capabilities": {
      "type": "object",
      "properties": {
        "packages": { "type": "array", "items": { "type": "string" } },
        "tools": { "type": "object" },
        "skills": { "type": "array" }
      }
    },
    "knowledge": { "type": "object" },
    "security": { "type": "object" },
    "collaboration": { "type": "object" },
    "triggers": { "type": "array" },
    "extends": { "type": "string" }
  }
}
```

---

## 参考来源

### 框架与产品文档
- [CrewAI Agents 文档](https://docs.crewai.com/en/concepts/agents)
- [CrewAI YAML 配置教程](https://codesignal.com/learn/courses/getting-started-with-crewai-agents-and-tasks/lessons/configuring-crewai-agents-and-tasks-with-yaml-files)
- [AutoGen Multi-Agent Conversation](https://microsoft.github.io/autogen/0.2/docs/Use-Cases/agent_chat/)
- [AutoGen Group Chat 设计模式](https://microsoft.github.io/autogen/stable//user-guide/core-user-guide/design-patterns/group-chat.html)
- [LangGraph 架构指南 2025](https://latenode.com/blog/ai-frameworks-technical-infrastructure/langgraph-multi-agent-orchestration/langgraph-ai-framework-2025-complete-architecture-guide-multi-agent-orchestration-analysis)
- [LangGraph 状态管理 2025](https://sparkco.ai/blog/mastering-langgraph-state-management-in-2025)
- [Dify Agent Node 文档](https://docs.dify.ai/versions/3-0-x/en/user-guide/workflow/node/agent)
- [Dify 工作流介绍](https://dify.ai/blog/dify-ai-workflow)
- [OpenAI GPTs 创建指南](https://help.openai.com/en/articles/8554397-creating-a-gpt)
- [OpenAI Instructions 最佳实践](https://help.openai.com/en/articles/9358033-key-guidelines-for-writing-instructions-for-custom-gpts)
- [Coze 快速入门](https://www.coze.com/open/docs/guides/quickstart)
- [Coze Multi-Agent Mode](https://cozehq.medium.com/building-an-all-in-one-personal-assistant-with-cozes-multi-agent-mode-e0f695137edf)
- [Claude Projects 指南](https://medium.com/@melissaonwuka/claude-projects-complete-guide-setup-tutorial-2025-3b9a60033b59)

### 设计模式与架构
- [Google Cloud Agentic AI 设计模式](https://docs.cloud.google.com/architecture/choose-design-pattern-agentic-ai-system)
- [SKILL.md 模式](https://bibek-poudel.medium.com/the-skill-md-pattern-how-to-write-ai-agent-skills-that-actually-work-72a3169dd7ee)
- [Block 工程博客 - 技能设计三原则](https://engineering.block.xyz/blog/3-principles-for-designing-agent-skills)
- [Spring AI Agent Skills](https://spring.io/blog/2026/01/13/spring-ai-generic-agent-skills/)
- [Azure AI Agent 编排模式](https://learn.microsoft.com/en-us/azure/architecture/ai-ml/guide/ai-agent-design-patterns)
- [AutoGen Multi-Agent Patterns 2025](https://sparkco.ai/blog/deep-dive-into-autogen-multi-agent-patterns-2025)

### 安全与权限
- [Sendbird AI Agent RBAC](https://sendbird.com/blog/ai-agent-role-based-access-control)
- [WorkOS AI Agent Access Control](https://workos.com/blog/ai-agent-access-control)
- [为何 RBAC 不足以管控 AI Agent](https://www.osohq.com/learn/why-rbac-is-not-enough-for-ai-agents)
- [Auth0 AI 时代的访问控制](https://auth0.com/blog/access-control-in-the-era-of-ai-agents/)

### 知识与记忆
- [IBM Agentic RAG](https://www.ibm.com/think/topics/agentic-rag)
- [IBM AI Agent Memory](https://www.ibm.com/think/topics/ai-agent-memory)
- [Agent Memory 演进: RAG → Agentic RAG → Agent Memory](https://yugensys.com/2025/11/19/evolution-of-rag-agentic-rag-and-agent-memory/)
- [AI Agent 知识库架构](https://www.infoworld.com/article/4091400/anatomy-of-an-ai-agent-knowledge-base.html)

### RPA 参考
- [UiPath RPA 平台](https://www.uipath.com/rpa/robotic-process-automation)
- [SS&C Blue Prism](https://www.blueprism.com/)
- [RPA 工具对比 2025](https://www.signitysolutions.com/blog/rpa-tools-comparison)
