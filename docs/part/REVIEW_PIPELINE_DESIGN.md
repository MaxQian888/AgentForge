# 审查流水线技术设计文档

> 版本：v1.0 | 日期：2026-03-22
> 所属项目：AgentForge — Agent 驱动的开发管理平台

---

## 目录

1. [架构总览](#一架构总览)
2. [Layer 1 — 快速审查](#二layer-1--快速审查)
3. [Layer 2 — 深度审查](#三layer-2--深度审查)
4. [Layer 3 — 人工审批](#四layer-3--人工审批)
5. [审查结果聚合](#五审查结果聚合)
6. [假阳性管理](#六假阳性管理)
7. [与 Agent 工作流集成](#七与-agent-工作流集成)
8. [安全审查细节](#八安全审查细节)
9. [成本优化](#九成本优化)
10. [数据模型](#十数据模型)
11. [部署与监控](#十一部署与监控)

---

## 一、架构总览

### 1.1 三层审查架构图

```
Agent / 人类开发者 提交 PR
  │
  ▼
┌──────────────────────────────────────────────────────────────────┐
│ Layer 1: 快速审查（所有 PR，免费/低成本）                           │
│                                                                  │
│  ┌─────────────────────┐  ┌──────────────────────────────────┐  │
│  │ claude-code-action   │  │ CI Pipeline                      │  │
│  │ (GitHub Action v1)   │  │ ┌────────┬──────────┬─────────┐ │  │
│  │                      │  │ │ Lint   │ TypeCheck│ Test    │ │  │
│  │ · PR 代码审查        │  │ │(ESLint)│ (tsc)    │(Vitest) │ │  │
│  │ · Inline 注释        │  │ └────────┴──────────┴─────────┘ │  │
│  │ · 风格 & 最佳实践    │  │                                  │  │
│  └─────────────────────┘  └──────────────────────────────────┘  │
│                                                                  │
│  预计耗时: 2-5 分钟 | 触发条件: 所有 PR                           │
└────────────────────────────────┬─────────────────────────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │ 分级路由决策引擎          │
                    │                          │
                    │ 风险评分 = f(变更大小,    │
                    │   文件敏感度, PR 来源,    │
                    │   Layer1 发现数量)        │
                    │                          │
                    │ 低风险 → 直接合并候选     │
                    │ 中风险 → Layer 2          │
                    │ 高风险 → Layer 2 + 3      │
                    └────────────┬─────────────┘
                                 │
┌────────────────────────────────▼─────────────────────────────────┐
│ Layer 2: 深度审查（Agent PR / 重要变更，付费）                      │
│                                                                   │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌─────────┐ │
│  │ 逻辑正确性   │ │ 安全漏洞     │ │ 性能影响     │ │ 规范合规│ │
│  │ Review Agent │ │ Security     │ │ Performance  │ │ Style   │ │
│  │              │ │ Agent        │ │ Agent        │ │ Agent   │ │
│  └──────┬───────┘ └──────┬───────┘ └──────┬───────┘ └────┬────┘ │
│         │                │                │              │      │
│         └────────────────┴────────────────┴──────────────┘      │
│                              │                                   │
│                    ┌─────────▼──────────┐                        │
│                    │ 交叉验证 & 去重     │                        │
│                    │ 假阳性过滤引擎      │                        │
│                    └────────────────────┘                        │
│                                                                   │
│  预计耗时: 5-15 分钟 | 触发条件: Agent PR / 高风险变更              │
└────────────────────────────────┬─────────────────────────────────┘
                                 │
┌────────────────────────────────▼─────────────────────────────────┐
│ Layer 3: 人工审批（关键变更）                                       │
│                                                                   │
│  ┌────────────────────┐  ┌──────────────────────────────────┐   │
│  │ IM 推送审查摘要     │  │ 审批操作                          │   │
│  │ (飞书/钉钉/Slack)   │  │                                  │   │
│  │                     │  │ ✅ 一键审批 → 合并 PR             │   │
│  │ · 变更概要          │  │ ❌ 拒绝 → 关闭 PR + 通知         │   │
│  │ · 风险评估          │  │ 🔄 要求修改 → 转回编码 Agent     │   │
│  │ · 审查发现摘要      │  │                                  │   │
│  └────────────────────┘  └──────────────────────────────────┘   │
│                                                                   │
│  预计耗时: 取决于人工 | 触发条件: 高风险变更 / 关键文件变更          │
└──────────────────────────────────────────────────────────────────┘
```

### 1.2 设计原则

| 原则 | 描述 |
|------|------|
| **成本分级** | 低风险 PR 仅用免费的 Layer 1；高风险 PR 叠加付费 Layer 2/3 |
| **并行审查** | Layer 2 的多维度审查并行执行，不串行等待 |
| **假阳性最小化** | 交叉验证 + 历史学习，目标假阳性率 < 5% |
| **自动闭环** | 审查发现 → 变更请求 → Agent 自动修复 → 重新审查，减少人工介入 |
| **可插拔** | 审查维度可自定义扩展，新增审查 Agent 无需修改核心流程 |

---

## 二、Layer 1 — 快速审查

### 2.1 claude-code-action 集成

[claude-code-action](https://github.com/anthropics/claude-code-action) 是 Anthropic 官方的 GitHub Action（v1.0，MIT 协议），直接在 GitHub Runner 上运行 Claude Code，对 PR 进行代码审查、问答和修改建议。

**核心特性：**

- 智能模式检测：根据触发事件自动选择执行模式（PR 审查、@claude 提及、自动化任务）
- 内联注释：可在 PR diff 中精确标注问题代码行
- 进度追踪：实时更新审查进度（checkbox 追踪）
- 结构化输出：可返回 JSON 格式审查结果供后续流程消费
- 多 Provider 支持：Anthropic API / AWS Bedrock / Google Vertex AI / Microsoft Foundry
- 完全在自有 Runner 上执行，API 调用走用户选择的 Provider

**运行成本：** 使用用户自有 Anthropic API Key 按 Token 计费，单次 PR 审查约 $0.01–0.10（取决于 PR 大小和模型选择）。也可使用 Claude Code OAuth Token 作为替代认证方式。

### 2.2 GitHub Action 工作流配置

#### 基础 PR 审查（所有 PR 触发）

```yaml
# .github/workflows/review-layer1.yml
name: "AgentForge Review - Layer 1"

on:
  pull_request:
    types: [opened, synchronize, ready_for_review, reopened]
  issue_comment:
    types: [created]
  pull_request_review_comment:
    types: [created]

permissions:
  contents: read
  pull-requests: write
  issues: write
  id-token: write

jobs:
  quick-review:
    runs-on: ubuntu-latest
    # 跳过 draft PR
    if: github.event.pull_request.draft == false
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 1

      - uses: anthropics/claude-code-action@v1
        with:
          anthropic_api_key: ${{ secrets.ANTHROPIC_API_KEY }}
          trigger_phrase: "@claude"
          track_progress: true
          prompt: |
            REPO: ${{ github.repository }}
            PR NUMBER: ${{ github.event.pull_request.number }}

            请对此 PR 进行快速代码审查，重点关注：
            - 代码质量与最佳实践
            - 潜在 Bug 或逻辑错误
            - 明显的安全风险
            - 性能隐患

            PR 分支已 checkout 到当前工作目录。

            使用 `gh pr comment` 发表整体反馈。
            使用 `mcp__github_inline_comment__create_inline_comment`（设置 `confirmed: true`）在具体代码行标注问题。
            仅通过 GitHub 评论发表审查意见，不要在消息中输出审查文本。

            审查完成后，输出一个 JSON 格式的结构化摘要。

          claude_args: |
            --model claude-4-0-sonnet-20250805
            --max-turns 15
            --allowedTools "mcp__github_inline_comment__create_inline_comment,Bash(gh pr comment:*),Bash(gh pr diff:*),Bash(gh pr view:*),Read"
            --json-schema '{"type":"object","properties":{"risk_level":{"type":"string","enum":["low","medium","high","critical"]},"findings_count":{"type":"integer"},"categories":{"type":"array","items":{"type":"string"}},"summary":{"type":"string"},"needs_deep_review":{"type":"boolean"}},"required":["risk_level","findings_count","needs_deep_review","summary"]}'
```

#### CI 流水线（Lint / TypeCheck / Test）

```yaml
# .github/workflows/ci.yml
name: "AgentForge CI"

on:
  pull_request:
    types: [opened, synchronize]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-node@v4
        with:
          node-version: "22"
          cache: "pnpm"
      - run: pnpm install --frozen-lockfile
      - run: pnpm lint
        # ESLint 配置见项目 .eslintrc

  typecheck:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-node@v4
        with:
          node-version: "22"
          cache: "pnpm"
      - run: pnpm install --frozen-lockfile
      - run: pnpm typecheck
        # tsc --noEmit

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-node@v4
        with:
          node-version: "22"
          cache: "pnpm"
      - run: pnpm install --frozen-lockfile
      - run: pnpm test -- --coverage
        # Vitest + coverage

  # Go 后端 CI
  go-lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  go-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
      - run: go test ./... -race -coverprofile=coverage.out
```

### 2.3 触发规则

| 事件 | 触发动作 | 说明 |
|------|---------|------|
| `pull_request.opened` | Layer 1 完整审查 | 新 PR 打开时 |
| `pull_request.synchronize` | Layer 1 增量审查 | 推送新 commit 时 |
| `pull_request.ready_for_review` | Layer 1 完整审查 | Draft 转 Ready 时 |
| `issue_comment.created` | @claude 交互 | 评论中 @claude 时 |
| `pull_request_review_comment.created` | @claude 交互 | review 注释中 @claude 时 |

### 2.4 Layer 1 输出

Layer 1 审查完成后产出结构化 JSON（通过 `--json-schema` 参数获取），包含：

```json
{
  "risk_level": "medium",
  "findings_count": 3,
  "categories": ["security", "performance"],
  "summary": "发现 3 个问题：1 个 SQL 注入风险（高）、1 个未索引查询（中）、1 个未处理错误（低）",
  "needs_deep_review": true
}
```

此输出作为 **分级路由决策** 的输入，决定是否触发 Layer 2。

---

## 三、Layer 2 — 深度审查

### 3.1 自建 Review Agent 架构

Layer 2 使用 Claude Agent SDK（TypeScript）构建自定义 Review Agent，通过 Agent SDK Bridge（TS 子服务）与 Go 后端通信。

```
Go Review Service
  │
  │  gRPC / HTTP
  ▼
Agent SDK Bridge (TS 子服务)
  │
  ├── Review Orchestrator Agent
  │     │
  │     ├── spawn → Logic Review SubAgent     (逻辑正确性)
  │     ├── spawn → Security Review SubAgent  (安全漏洞)
  │     ├── spawn → Performance Review SubAgent (性能影响)
  │     └── spawn → Compliance Review SubAgent (规范合规)
  │
  │   ┌─────────────────────────────┐
  │   │  并行执行，各自独立上下文     │
  │   │  共享: PR diff, 项目规范文档  │
  │   └─────────────────────────────┘
  │
  ▼
Cross-Validation Engine (交叉验证引擎)
  │
  ▼
Aggregated Review Result (聚合审查结果)
```

### 3.2 多维度并行审查

#### 3.2.1 逻辑正确性审查

```typescript
// review-agents/logic-review.ts
import { claude } from "@anthropic-ai/claude-code";

async function reviewLogic(prContext: PRContext): Promise<ReviewFindings> {
  const result = await claude({
    prompt: `你是一个严格的代码逻辑审查专家。

审查以下 PR 变更的逻辑正确性：

## PR 信息
- 仓库: ${prContext.repo}
- PR: #${prContext.number}
- 标题: ${prContext.title}
- 描述: ${prContext.description}

## 变更文件
${prContext.diff}

## 审查维度
1. 边界条件处理（空值、零值、最大值、并发）
2. 错误处理完整性（是否遗漏错误路径）
3. 状态一致性（数据库事务、缓存一致性）
4. 算法正确性（逻辑是否符合意图）
5. 类型安全（类型断言、any 使用）

对每个发现项输出：
- severity: "critical" | "high" | "medium" | "low" | "info"
- category: "logic"
- subcategory: 具体维度
- file: 文件路径
- line: 行号
- message: 问题描述
- suggestion: 修复建议`,
    options: {
      maxTurns: 10,
      model: "claude-opus-4-1-20250805",
    },
  });

  return parseFindings(result);
}
```

#### 3.2.2 安全漏洞审查

安全审查基于 OWASP Top 10 框架，结合项目上下文进行针对性检查。

```typescript
// review-agents/security-review.ts
async function reviewSecurity(prContext: PRContext): Promise<ReviewFindings> {
  const result = await claude({
    prompt: `你是一个资深安全审查专家，专注于 Web 应用安全。

## OWASP Top 10 检查清单
对以下 PR 变更逐项检查：

### A01: 访问控制失效
- 是否存在越权访问风险
- RBAC/ABAC 规则是否正确实施
- API 端点是否有适当的认证/授权检查

### A02: 加密机制失效
- 敏感数据是否加密存储/传输
- 是否使用过时的加密算法
- 密钥管理是否安全

### A03: 注入
- SQL 注入（参数化查询检查）
- XSS（输出编码检查）
- 命令注入（Shell 命令拼接检查）
- LDAP/NoSQL/ORM 注入

### A04: 不安全设计
- 业务逻辑漏洞
- 缺少速率限制
- 缺少输入验证

### A05: 安全配置错误
- 默认凭据
- 不必要的功能启用
- 错误消息泄露内部信息

### A06: 易受攻击的组件
- 是否引入已知漏洞的依赖
- 依赖版本是否过旧

### A07: 认证失效
- 会话管理缺陷
- 密码策略不足
- 多因素认证缺失

### A08: 软件和数据完整性失效
- 不安全的反序列化
- 未验证的依赖来源

### A09: 日志和监控不足
- 敏感操作是否记录审计日志
- 日志中是否包含敏感信息

### A10: SSRF
- 服务端请求是否校验 URL
- 是否存在内网访问风险

## PR 变更
${prContext.diff}

对每个发现项标注 CWE 编号（如适用）。
severity 使用 CVSS 思路评估: critical(9-10), high(7-8.9), medium(4-6.9), low(0-3.9)。`,
    options: {
      maxTurns: 15,
      model: "claude-opus-4-1-20250805",
    },
  });

  return parseFindings(result);
}
```

#### 3.2.3 性能影响审查

```typescript
// review-agents/performance-review.ts
async function reviewPerformance(prContext: PRContext): Promise<ReviewFindings> {
  const result = await claude({
    prompt: `你是一个性能优化专家。

审查以下 PR 变更的性能影响：

## 审查维度
1. 数据库查询
   - N+1 查询问题
   - 缺少索引
   - 全表扫描风险
   - 未使用分页的大数据集查询

2. 算法复杂度
   - O(n²) 或更高复杂度的循环
   - 不必要的重复计算
   - 可缓存的计算结果

3. 内存使用
   - 大对象未释放
   - goroutine 泄漏（Go）
   - 内存中加载大数据集

4. 网络与 I/O
   - 不必要的串行请求（可并行）
   - 缺少超时设置
   - 未使用连接池

5. 前端性能
   - 大 bundle 引入
   - 不必要的重渲染
   - 缺少懒加载

## PR 变更
${prContext.diff}`,
    options: {
      maxTurns: 10,
      model: "claude-4-0-sonnet-20250805",
    },
  });

  return parseFindings(result);
}
```

#### 3.2.4 项目规范合规审查

```typescript
// review-agents/compliance-review.ts
async function reviewCompliance(prContext: PRContext): Promise<ReviewFindings> {
  // 从项目配置中加载规范文档
  const projectStandards = await loadProjectStandards(prContext.repo);

  const result = await claude({
    prompt: `你是项目规范审查员。

## 项目规范
${projectStandards}

## 审查维度
1. 命名规范（变量/函数/文件/分支）
2. 代码组织（目录结构、模块划分）
3. 注释和文档要求
4. 错误处理模式（项目约定的错误处理方式）
5. 日志规范（日志级别、格式）
6. API 设计规范（REST 约定、响应格式）
7. 测试规范（覆盖率要求、测试命名）
8. Git 提交规范（Conventional Commits）

## PR 变更
${prContext.diff}`,
    options: {
      maxTurns: 8,
      model: "claude-4-0-sonnet-20250805",
    },
  });

  return parseFindings(result);
}
```

### 3.3 并行执行架构

```typescript
// review-orchestrator.ts
async function executeDeepReview(prContext: PRContext): Promise<AggregatedResult> {
  // 并行启动所有审查维度
  const [logic, security, performance, compliance] = await Promise.all([
    reviewLogic(prContext),
    reviewSecurity(prContext),
    reviewPerformance(prContext),
    reviewCompliance(prContext),
  ]);

  // 交叉验证与去重
  const validated = await crossValidate({
    logic,
    security,
    performance,
    compliance,
  });

  // 假阳性过滤
  const filtered = await filterFalsePositives(validated, prContext.repo);

  // 聚合结果
  return aggregateResults(filtered);
}
```

### 3.4 触发条件

Layer 2 深度审查在以下情况触发：

| 条件 | 触发方式 | 说明 |
|------|---------|------|
| Agent 提交的 PR | 自动 | 所有 Agent 生成的 PR 强制深度审查 |
| Layer 1 判定 `needs_deep_review: true` | 自动 | Layer 1 发现高风险问题 |
| 风险评分 >= 中 | 自动 | 分级路由引擎评估 |
| 人工触发 | 手动 | 通过 IM 或 Dashboard 手动触发 `/review deep <pr-url>` |
| 敏感文件变更 | 自动 | 见安全审查触发规则 |

**Layer 2 GitHub Action 触发（由 Layer 1 驱动）：**

```yaml
# .github/workflows/review-layer2.yml
name: "AgentForge Review - Layer 2 Deep Review"

on:
  workflow_run:
    workflows: ["AgentForge Review - Layer 1"]
    types: [completed]

jobs:
  check-need-deep-review:
    runs-on: ubuntu-latest
    outputs:
      needs_deep: ${{ steps.check.outputs.needs_deep }}
    steps:
      - name: Check Layer 1 results
        id: check
        run: |
          # 从 Layer 1 的 structured_output 获取审查结果
          # 通过 GitHub API 获取 workflow run artifacts
          RESULT=$(gh api repos/${{ github.repository }}/actions/runs/${{ github.event.workflow_run.id }}/artifacts)
          # 解析 risk_level 和 needs_deep_review
          NEEDS_DEEP=$(echo "$RESULT" | jq -r '.needs_deep_review')
          echo "needs_deep=$NEEDS_DEEP" >> "$GITHUB_OUTPUT"

  deep-review:
    needs: check-need-deep-review
    if: needs.check-need-deep-review.outputs.needs_deep == 'true'
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
      id-token: write
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - name: Trigger AgentForge Deep Review
        run: |
          # 调用 AgentForge Review Service API 触发深度审查
          curl -X POST "${{ secrets.AGENTFORGE_API_URL }}/api/v1/reviews/trigger" \
            -H "Authorization: Bearer ${{ secrets.AGENTFORGE_TOKEN }}" \
            -H "Content-Type: application/json" \
            -d '{
              "pr_url": "${{ github.event.workflow_run.head_repository.full_name }}/pull/${{ github.event.workflow_run.pull_requests[0].number }}",
              "review_type": "deep",
              "dimensions": ["logic", "security", "performance", "compliance"]
            }'
```

---

## 四、Layer 3 — 人工审批

### 4.1 审查摘要推送

当 Layer 2 完成后，系统将审查结果聚合为结构化摘要，通过 IM 桥接服务推送给相关审批人。

**IM 推送消息模板（飞书卡片消息示例）：**

```json
{
  "msg_type": "interactive",
  "card": {
    "header": {
      "title": { "content": "PR 审查报告 #87", "tag": "plain_text" },
      "template": "orange"
    },
    "elements": [
      {
        "tag": "div",
        "text": {
          "tag": "lark_md",
          "content": "**仓库:** org/project\n**分支:** agent/task-123 → main\n**提交者:** Coding Agent\n**关联任务:** TASK-123 用户认证模块 Token 刷新\n\n---\n\n**风险评估:** 🟡 中等\n**变更规模:** +156 / -42 (6 文件)\n\n**审查发现:**\n🔴 高危 × 1: SQL 参数未转义 (auth/token.go:87)\n🟡 中危 × 2: 缺少并发锁 / 未设查询超时\n🟢 低危 × 1: 变量命名不符合规范"
        }
      },
      {
        "tag": "action",
        "actions": [
          {
            "tag": "button",
            "text": { "content": "✅ 审批通过", "tag": "plain_text" },
            "type": "primary",
            "value": { "action": "approve", "pr": 87, "review_id": "rev-xxx" }
          },
          {
            "tag": "button",
            "text": { "content": "❌ 拒绝", "tag": "plain_text" },
            "type": "danger",
            "value": { "action": "reject", "pr": 87, "review_id": "rev-xxx" }
          },
          {
            "tag": "button",
            "text": { "content": "🔄 要求修改", "tag": "plain_text" },
            "type": "default",
            "value": { "action": "request_changes", "pr": 87, "review_id": "rev-xxx" }
          },
          {
            "tag": "button",
            "text": { "content": "📋 查看详情", "tag": "plain_text" },
            "type": "default",
            "url": "https://github.com/org/project/pull/87"
          }
        ]
      }
    ]
  }
}
```

### 4.2 审批操作处理流程

```
用户在 IM 点击按钮
  │
  ├── ✅ 审批通过
  │     ├── 调用 GitHub API: 合并 PR
  │     ├── 更新任务状态: In Review → Done
  │     ├── 通知 Agent: 任务完成
  │     └── 记录审批日志
  │
  ├── ❌ 拒绝
  │     ├── 调用 GitHub API: 关闭 PR + 评论拒绝原因
  │     ├── 更新任务状态: In Review → Cancelled
  │     ├── 清理: 删除 Agent worktree + 分支
  │     └── 通知相关人员
  │
  └── 🔄 要求修改
        ├── 弹出输入框让用户填写修改说明
        ├── 在 PR 评论中添加修改请求
        ├── 更新任务状态: In Review → Changes Requested → In Progress
        ├── 触发 Agent 重新执行:
        │     ├── 恢复 Agent Session（session resume）
        │     ├── 注入修改请求作为新 prompt
        │     └── Agent 修改代码 → 推送到同一分支 → 重新触发审查
        └── 通知技术负责人
```

### 4.3 人工审批触发规则

不是所有 PR 都需要人工审批。触发条件：

| 条件 | 说明 |
|------|------|
| Layer 2 发现 critical/high 级别问题 | 存在高危发现 |
| 涉及认证/授权/支付等关键模块 | 敏感文件路径匹配 |
| 变更量 > 500 行 | 大规模变更 |
| Agent 首次处理该类型任务 | Agent 信任度尚未建立 |
| 项目配置要求人工审批 | 在项目 settings 中可配置 |

---

## 五、审查结果聚合

### 5.1 多层 Findings 合并

```typescript
interface ReviewFinding {
  id: string;
  layer: 1 | 2;
  dimension: "quick" | "logic" | "security" | "performance" | "compliance";
  severity: "critical" | "high" | "medium" | "low" | "info";
  category: string;
  subcategory?: string;
  file: string;
  line?: number;
  endLine?: number;
  message: string;
  suggestion?: string;
  cwe?: string;        // CWE 编号（安全类）
  confidence: number;  // 0-1 置信度
  isValidated: boolean; // 是否经过交叉验证
}

interface AggregatedReviewResult {
  prUrl: string;
  reviewId: string;
  overallRisk: "critical" | "high" | "medium" | "low";
  findings: ReviewFinding[];
  summary: string;          // 可操作摘要
  metrics: {
    totalFindings: number;
    bySeverity: Record<string, number>;
    byDimension: Record<string, number>;
    falsePositivesFiltered: number;
    duplicatesRemoved: number;
  };
  recommendation: "approve" | "request_changes" | "reject";
  estimatedFixTime: string; // 预估修复时间
}
```

### 5.2 去重策略

同一问题可能被多个审查维度重复发现。去重规则：

1. **精确去重**：同文件 + 同行号 + 相似消息（Levenshtein 距离 < 0.3）→ 合并，保留最高 severity
2. **语义去重**：使用 Embedding 计算 findings 间语义相似度，相似度 > 0.85 视为重复
3. **保留策略**：重复发现合并时，保留所有维度标签（如同一问题同时被 security 和 logic 发现）

### 5.3 严重性评分

最终严重性 = max(各维度评分) + 上下文加权

```
最终评分 = base_severity × context_multiplier

context_multiplier 规则:
  - 涉及认证模块: × 1.5
  - 涉及支付模块: × 2.0
  - 涉及用户数据: × 1.3
  - 被多个维度同时发现: × 1.2
  - 在主分支直接修改: × 1.5
```

### 5.4 可操作摘要生成

聚合引擎最终生成一段简洁的可操作摘要：

```
## 审查摘要 — PR #87

**风险等级: 🟡 中等** | 发现 4 个问题 (1 高 / 2 中 / 1 低) | 预估修复: 30 分钟

### 必须修复 (阻塞合并)
1. 🔴 [高] auth/token.go:87 — SQL 参数未转义，存在注入风险
   → 建议: 使用 sqlx.NamedQuery 替代字符串拼接

### 建议修复
2. 🟡 [中] auth/session.go:42 — 并发场景缺少 mutex 保护
3. 🟡 [中] auth/token.go:123 — 数据库查询未设超时

### 可选优化
4. 🟢 [低] auth/token.go:15 — 变量名 `t` 建议改为 `tokenRefreshInterval`

**建议操作: 要求修改（修复第 1 项后可合并）**
```

---

## 六、假阳性管理

### 6.1 交叉验证机制

当多个审查维度并行执行时，通过交叉验证降低假阳性：

```
┌──────────────────────────────────────────────────┐
│              交叉验证引擎                          │
│                                                    │
│  输入: 各维度的 findings 列表                       │
│                                                    │
│  规则 1: 单一维度发现 + confidence < 0.6           │
│          → 标记为"待验证"，降低 severity 一级       │
│                                                    │
│  规则 2: 两个以上维度同时发现同一问题               │
│          → 标记为"已验证"，confidence 提升           │
│                                                    │
│  规则 3: 安全类发现 + severity >= high              │
│          → 强制保留，不降级（宁可误报不可漏报）      │
│                                                    │
│  规则 4: 与历史假阳性库匹配                         │
│          → 自动过滤或降级为 info                    │
│                                                    │
│  输出: 验证后的 findings + 置信度更新               │
└──────────────────────────────────────────────────┘
```

### 6.2 历史假阳性学习

```sql
-- 假阳性记录表
CREATE TABLE false_positives (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID REFERENCES projects(id),
    pattern     TEXT NOT NULL,          -- 匹配模式（正则或语义描述）
    category    VARCHAR(50),            -- 审查维度
    file_pattern VARCHAR(255),          -- 文件路径模式
    reason      TEXT,                   -- 标记为假阳性的原因
    reporter_id UUID REFERENCES members(id),
    occurrences INTEGER DEFAULT 1,      -- 累计出现次数
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_fp_project ON false_positives(project_id);
CREATE INDEX idx_fp_category ON false_positives(category);
```

**学习流程：**

1. 审查人员在 PR 评论中标记假阳性：`@claude false-positive: 这个不是安全问题，因为...`
2. 系统提取假阳性模式，存入 `false_positives` 表
3. 后续审查时，匹配引擎自动检查新 findings 是否与已知假阳性模式匹配
4. 匹配到的 findings 自动降级为 `info` 或过滤
5. 当同一模式累计出现 >= 3 次，将其提升为"强假阳性"，未来自动过滤

### 6.3 用户反馈循环

```
审查发现 finding
  │
  ├── 用户认可 → 记录为"真阳性" → 增加该类型检查权重
  │
  ├── 用户标记假阳性
  │     ├── 记录模式到假阳性库
  │     ├── 调整该类型检查参数
  │     └── 如果同一 Agent 连续产出假阳性 → 调整 Agent prompt
  │
  └── 用户忽略（不操作）
        └── 30 天后自动标记为"已过期"，不影响学习
```

**假阳性率目标：** < 5%（即 100 个发现中假阳性不超过 5 个）

---

## 七、与 Agent 工作流集成

### 7.1 Agent PR 触发审查的完整流程

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. 任务分配                                                      │
│    用户通过 IM/Dashboard 创建任务 → 分配给 Agent                  │
│    任务状态: Assigned                                             │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 2. Agent 执行编码                                                 │
│    Agent SDK Bridge 启动 Agent → 独立 worktree                   │
│    Agent 分析 → 编码 → 测试 → commit                             │
│    任务状态: In Progress                                          │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 3. 创建 PR                                                       │
│    Agent 通过 `gh pr create` 创建 PR                             │
│    PR 标题自动关联任务: "fix(auth): token refresh logic [TASK-123]"│
│    任务状态: In Review                                            │
│    PR 标签: agent-generated, task-123                             │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 4. Layer 1 自动触发                                               │
│    GitHub Action 检测到 PR 事件 → 启动 claude-code-action          │
│    同时启动 CI (lint + typecheck + test)                          │
│    产出: 结构化审查结果 + CI 状态                                  │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 5. 分级路由                                                       │
│    agent-generated 标签 → 强制进入 Layer 2                        │
│    或 Layer 1 判定 needs_deep_review → Layer 2                    │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│ 6. Layer 2 深度审查                                               │
│    Review Service 启动 4 个并行 SubAgent                         │
│    交叉验证 → 假阳性过滤 → 聚合结果                              │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                    ┌───────┴────────┐
                    │                │
              无高危发现         有高危发现
                    │                │
                    ▼                ▼
             自动批准合并       Layer 3 人工审批
                    │                │
                    │         ┌──────┴──────┐
                    │     审批通过      要求修改
                    │         │            │
                    ▼         ▼            ▼
              任务完成     任务完成    Agent 自动修改
              (Done)      (Done)     (回到步骤 2)
```

### 7.2 审查结果回流到任务状态

```go
// Go 后端 Review Service - 审查结果处理
func (s *ReviewService) HandleReviewResult(ctx context.Context, result *AggregatedReviewResult) error {
    task, err := s.taskRepo.GetByPRUrl(ctx, result.PRUrl)
    if err != nil {
        return err
    }

    switch result.Recommendation {
    case "approve":
        // 自动合并 PR
        if err := s.githubClient.MergePR(ctx, result.PRUrl); err != nil {
            return err
        }
        // 更新任务状态为完成
        task.Status = "done"
        task.CompletedAt = time.Now()

    case "request_changes":
        // 更新任务状态为需要修改
        task.Status = "changes_requested"
        // 将修改请求转发给 Agent
        s.agentBridge.RequestChanges(ctx, task.AgentSessionID, ReviewChangesPrompt{
            Findings:    result.Findings,
            Summary:     result.Summary,
            TaskContext: task,
        })

    case "reject":
        // 关闭 PR
        s.githubClient.ClosePR(ctx, result.PRUrl, result.Summary)
        task.Status = "cancelled"
    }

    // 保存任务状态变更
    if err := s.taskRepo.Update(ctx, task); err != nil {
        return err
    }

    // 发送通知
    s.notifier.NotifyReviewComplete(ctx, task, result)

    // 记录审查到数据库
    return s.reviewRepo.Save(ctx, &Review{
        TaskID:   task.ID,
        PRUrl:    result.PRUrl,
        Reviewer: "review-pipeline",
        Status:   string(result.Recommendation),
        Findings: result.Findings,
        Summary:  result.Summary,
        CostUSD:  result.Cost,
    })
}
```

### 7.3 变更请求时自动重新执行

当审查要求修改时，系统自动将修改请求转回编码 Agent：

```typescript
// Agent SDK Bridge - 处理变更请求
async function handleChangesRequested(
  sessionId: string,
  changes: ReviewChangesPrompt,
): Promise<void> {
  // 尝试恢复已有 session（断点续做）
  const result = await claude({
    prompt: `之前的 PR 审查发现了以下问题，请逐一修复：

${changes.findings
  .filter((f) => f.severity === "critical" || f.severity === "high")
  .map(
    (f, i) => `
${i + 1}. [${f.severity.toUpperCase()}] ${f.file}:${f.line}
   问题: ${f.message}
   建议: ${f.suggestion || "请根据问题描述修复"}
`,
  )
  .join("\n")}

修复后请运行测试确保所有测试通过，然后 commit 并 push 到当前分支。`,
    options: {
      resume: sessionId, // 恢复之前的 session
      maxTurns: 20,
    },
  });

  // push 后 GitHub Action 自动重新触发 Layer 1
  // 形成闭环: 修改 → push → Layer 1 → (可能) Layer 2 → 合并/再次修改
}
```

---

## 八、安全审查细节

### 8.1 安全审查触发规则

安全审查（Layer 2 Security SubAgent）在以下情况 **强制触发**：

| 触发条件 | 匹配规则 | 说明 |
|---------|---------|------|
| 认证相关变更 | 文件路径匹配 `**/auth/**`, `**/login/**`, `**/session/**`, `**/oauth/**` | 认证流程变更 |
| 加密相关变更 | 文件路径匹配 `**/crypto/**`, `**/encrypt/**`；或 diff 包含 `crypto.`, `bcrypt`, `jwt.Sign` | 密码学操作 |
| 输入处理变更 | diff 包含 `req.Body`, `req.Query`, `req.Params`, `FormValue`, `innerHTML`, `dangerouslySetInnerHTML` | 用户输入处理 |
| 数据库操作 | diff 包含原始 SQL 字符串拼接（`"SELECT.*" +`）、新增 SQL 查询 | SQL 注入风险 |
| 依赖变更 | 修改 `package.json`, `go.mod`, `go.sum`, `pnpm-lock.yaml` | 供应链安全 |
| 配置变更 | 修改 `.env*`, `config/**`, `**/secrets/**`, `docker-compose*` | 安全配置 |
| API 端点变更 | 新增路由注册（`router.GET`, `app.Post`, `api.HandleFunc`） | 新攻击面 |
| 权限相关 | diff 包含 `role`, `permission`, `admin`, `sudo`, `elevated` | 权限提升风险 |

**GitHub Action 路径触发配置：**

```yaml
# .github/workflows/security-review.yml
name: "AgentForge Security Review"

on:
  pull_request:
    types: [opened, synchronize]
    paths:
      # 认证 & 授权
      - "src/**/auth/**"
      - "src/**/login/**"
      - "src/**/session/**"
      - "src/**/oauth/**"
      - "src/**/rbac/**"
      - "src/**/permission*/**"
      # 加密
      - "src/**/crypto/**"
      - "src/**/encrypt*/**"
      # 数据库
      - "src/**/model*/**"
      - "src/**/migration*/**"
      - "**/schema.sql"
      # 依赖
      - "package.json"
      - "pnpm-lock.yaml"
      - "go.mod"
      - "go.sum"
      # 配置
      - ".env*"
      - "config/**"
      - "docker-compose*"
      - "**/Dockerfile*"
      # API 路由
      - "src/**/route*/**"
      - "src/**/handler*/**"
      - "src/**/middleware*/**"

jobs:
  security-scan:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
      security-events: write
      id-token: write
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - uses: anthropics/claude-code-action@v1
        with:
          anthropic_api_key: ${{ secrets.ANTHROPIC_API_KEY }}
          prompt: |
            REPO: ${{ github.repository }}
            PR NUMBER: ${{ github.event.pull_request.number }}

            此 PR 修改了安全敏感文件，请进行深度安全审查。

            ## OWASP Top 10 逐项检查
            针对本次变更，逐项检查 OWASP Top 10 (2021)：
            A01-访问控制失效 / A02-加密失效 / A03-注入 / A04-不安全设计 /
            A05-安全配置错误 / A06-易受攻击组件 / A07-认证失效 /
            A08-完整性失效 / A09-日志监控不足 / A10-SSRF

            ## 额外检查
            - 硬编码凭据或密钥
            - 不安全的反序列化
            - 竞态条件
            - 路径遍历
            - 敏感数据日志泄露

            每个发现标注：severity, CWE 编号, 具体文件行号, 修复建议。
            使用 inline comment 标注具体代码问题。

          claude_args: |
            --model claude-opus-4-1-20250805
            --max-turns 20
            --allowedTools "mcp__github_inline_comment__create_inline_comment,Bash(gh pr comment:*),Bash(gh pr diff:*),Read"
```

### 8.2 依赖漏洞扫描

```yaml
# 作为 CI 的一部分
dependency-audit:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v6

    # Node.js 依赖审计
    - name: npm audit
      run: |
        pnpm audit --audit-level=moderate --json > npm-audit.json || true

    # Go 依赖审计
    - name: govulncheck
      run: |
        go install golang.org/x/vuln/cmd/govulncheck@latest
        govulncheck ./... 2>&1 | tee go-vuln.txt || true

    # 结果上报到 AgentForge Review Service
    - name: Report findings
      run: |
        curl -X POST "${{ secrets.AGENTFORGE_API_URL }}/api/v1/reviews/dependency-scan" \
          -H "Authorization: Bearer ${{ secrets.AGENTFORGE_TOKEN }}" \
          -F "npm_audit=@npm-audit.json" \
          -F "go_vuln=@go-vuln.txt" \
          -F "pr_number=${{ github.event.pull_request.number }}"
```

### 8.3 安全审查严重性映射

| 安全发现类型 | 默认 Severity | 上下文加权后最高 |
|-------------|--------------|---------------|
| SQL 注入 (CWE-89) | critical | critical |
| 认证绕过 (CWE-287) | critical | critical |
| 硬编码凭据 (CWE-798) | high | critical (生产配置) |
| XSS (CWE-79) | high | critical (用户数据页面) |
| SSRF (CWE-918) | high | critical (内网可达) |
| 路径遍历 (CWE-22) | high | high |
| 竞态条件 (CWE-362) | medium | high (支付/余额) |
| 日志泄露 (CWE-532) | medium | high (含密码/Token) |
| 缺少输入验证 (CWE-20) | medium | high (面向公网) |
| 弱加密 (CWE-327) | medium | medium |
| 依赖漏洞 (已知 CVE) | 按 CVSS 评分 | 按 CVSS 评分 |

---

## 九、成本优化

### 9.1 分级审查成本模型

```
┌───────────────────────────────────────────────────────────────┐
│                    成本分级策略                                  │
│                                                               │
│  所有 PR                                                      │
│  └── Layer 1: claude-code-action (Sonnet)                    │
│      成本: ~$0.01-0.10/PR                                    │
│      覆盖: 100% PR                                           │
│                                                               │
│  Agent PR + 中风险以上                                        │
│  └── Layer 2: 深度审查 (Opus for security, Sonnet for others)│
│      成本: ~$0.30-1.50/PR                                    │
│      覆盖: ~30-50% PR                                        │
│                                                               │
│  高风险 + 关键变更                                             │
│  └── Layer 3: 人工审批                                        │
│      成本: 人工时间，无 API 费用                               │
│      覆盖: ~10-20% PR                                        │
│                                                               │
│  预估月成本 (50 PR/月团队):                                    │
│  Layer 1: 50 × $0.05 = $2.50                                 │
│  Layer 2: 20 × $0.80 = $16.00                                │
│  Layer 3: 10 × $0.00 = $0.00 (人工成本另计)                  │
│  总计 API 成本: ~$18.50/月                                    │
└───────────────────────────────────────────────────────────────┘
```

### 9.2 模型选择策略

| 审查维度 | 推荐模型 | 原因 | 单次成本估算 |
|---------|---------|------|------------|
| Layer 1 快速审查 | claude-4-0-sonnet | 性价比最优，足够覆盖常见问题 | $0.01-0.10 |
| 逻辑正确性 | claude-4-0-sonnet | 中等复杂度即可 | $0.05-0.20 |
| 安全漏洞 | claude-opus-4-1 | 安全审查需要最强推理能力 | $0.15-0.60 |
| 性能影响 | claude-4-0-sonnet | 模式匹配为主 | $0.05-0.20 |
| 规范合规 | claude-haiku-4-5 | 规则匹配为主，轻量模型即可 | $0.01-0.05 |
| 交叉验证 | claude-haiku-4-5 | 简单比对任务 | $0.01-0.03 |

### 9.3 基于变更大小的分级

```typescript
function determineReviewLevel(pr: PRMetadata): ReviewLevel {
  const score = calculateRiskScore(pr);

  // 微小变更（< 20 行，仅文档/配置）→ Layer 1 only
  if (pr.additions + pr.deletions < 20 && isDocOrConfigOnly(pr.files)) {
    return { layers: [1], models: { layer1: "claude-haiku-4-5" } };
  }

  // 小变更（< 100 行，无敏感文件）→ Layer 1 with Sonnet
  if (pr.additions + pr.deletions < 100 && !hasSensitiveFiles(pr.files)) {
    return { layers: [1], models: { layer1: "claude-4-0-sonnet-20250805" } };
  }

  // Agent PR 或中等变更 → Layer 1 + 2
  if (pr.labels.includes("agent-generated") || score >= 0.5) {
    return {
      layers: [1, 2],
      models: {
        layer1: "claude-4-0-sonnet-20250805",
        security: "claude-opus-4-1-20250805",
        logic: "claude-4-0-sonnet-20250805",
        performance: "claude-4-0-sonnet-20250805",
        compliance: "claude-haiku-4-5-20251001",
      },
    };
  }

  // 大变更或高风险 → Layer 1 + 2 + 3
  if (score >= 0.8 || pr.additions + pr.deletions > 500) {
    return {
      layers: [1, 2, 3],
      models: {
        layer1: "claude-4-0-sonnet-20250805",
        security: "claude-opus-4-1-20250805",
        logic: "claude-opus-4-1-20250805",
        performance: "claude-4-0-sonnet-20250805",
        compliance: "claude-4-0-sonnet-20250805",
      },
    };
  }

  // 默认 Layer 1
  return { layers: [1], models: { layer1: "claude-4-0-sonnet-20250805" } };
}

function calculateRiskScore(pr: PRMetadata): number {
  let score = 0;
  const size = pr.additions + pr.deletions;

  // 变更大小
  if (size > 500) score += 0.3;
  else if (size > 200) score += 0.2;
  else if (size > 50) score += 0.1;

  // 敏感文件
  if (hasSensitiveFiles(pr.files)) score += 0.3;

  // Agent 生成
  if (pr.labels.includes("agent-generated")) score += 0.2;

  // 变更文件数
  if (pr.changedFiles > 10) score += 0.1;

  // Layer 1 发现数量
  if (pr.layer1Findings > 3) score += 0.2;

  return Math.min(score, 1.0);
}
```

### 9.4 成本预算控制

```go
// Go 后端 - 审查成本控制
type ReviewBudget struct {
    ProjectID     uuid.UUID
    MonthlyLimit  decimal.Decimal  // 月度审查预算上限
    CurrentSpent  decimal.Decimal  // 当月已用
    AlertAt       float64          // 告警阈值 (0.8 = 80%)
}

func (s *ReviewService) CheckBudget(ctx context.Context, projectID uuid.UUID, estimatedCost decimal.Decimal) error {
    budget, err := s.budgetRepo.Get(ctx, projectID)
    if err != nil {
        return err
    }

    newSpent := budget.CurrentSpent.Add(estimatedCost)

    // 超过预算上限 → 降级为 Layer 1 only
    if newSpent.GreaterThan(budget.MonthlyLimit) {
        return ErrBudgetExceeded
    }

    // 接近告警阈值 → 通知技术负责人
    ratio := newSpent.Div(budget.MonthlyLimit).InexactFloat64()
    if ratio >= budget.AlertAt {
        s.notifier.NotifyBudgetWarning(ctx, projectID, ratio)
    }

    return nil
}
```

---

## 十、数据模型

### 10.1 审查相关表结构

基于 PRD 中定义的 `reviews` 表，扩展以下表：

```sql
-- 审查记录（扩展 PRD 定义）
CREATE TABLE reviews (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id         UUID REFERENCES tasks(id),
    pr_url          VARCHAR(500) NOT NULL,
    pr_number       INTEGER,
    repo            VARCHAR(255),

    -- 审查层级和维度
    layer           SMALLINT NOT NULL,       -- 1, 2, 3
    dimension       VARCHAR(50),             -- 'quick', 'logic', 'security', 'performance', 'compliance', 'human'
    reviewer        VARCHAR(100) NOT NULL,   -- 'claude-code-action', 'logic-agent', 'security-agent', ...

    -- 结果
    status          VARCHAR(30) NOT NULL,    -- pending | running | completed | failed
    recommendation  VARCHAR(30),             -- approve | request_changes | reject
    risk_level      VARCHAR(20),             -- critical | high | medium | low
    findings        JSONB DEFAULT '[]',      -- [{severity, category, file, line, message, ...}]
    summary         TEXT,

    -- 成本
    model_used      VARCHAR(100),
    tokens_input    BIGINT DEFAULT 0,
    tokens_output   BIGINT DEFAULT 0,
    cost_usd        DECIMAL(8,4) DEFAULT 0,
    duration_sec    INTEGER,

    created_at      TIMESTAMPTZ DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

-- 聚合审查结果（多层合并后的最终结果）
CREATE TABLE review_aggregations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pr_url          VARCHAR(500) NOT NULL UNIQUE,
    task_id         UUID REFERENCES tasks(id),
    review_ids      UUID[] NOT NULL,         -- 关联的 reviews.id 列表

    -- 聚合结果
    overall_risk    VARCHAR(20) NOT NULL,
    recommendation  VARCHAR(30) NOT NULL,
    findings        JSONB NOT NULL,          -- 去重、排序后的最终 findings
    summary         TEXT NOT NULL,
    metrics         JSONB NOT NULL,          -- {totalFindings, bySeverity, ...}

    -- 人工审批
    human_decision  VARCHAR(30),             -- approve | reject | request_changes
    human_reviewer  UUID REFERENCES members(id),
    human_comment   TEXT,
    decided_at      TIMESTAMPTZ,

    total_cost_usd  DECIMAL(8,4) DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- 假阳性记录
CREATE TABLE false_positives (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID REFERENCES projects(id),
    pattern         TEXT NOT NULL,
    category        VARCHAR(50),
    file_pattern    VARCHAR(255),
    reason          TEXT,
    reporter_id     UUID REFERENCES members(id),
    occurrences     INTEGER DEFAULT 1,
    is_strong       BOOLEAN DEFAULT FALSE,  -- 强假阳性（>=3次确认）
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- 索引
CREATE INDEX idx_reviews_pr ON reviews(pr_url);
CREATE INDEX idx_reviews_task ON reviews(task_id);
CREATE INDEX idx_reviews_status ON reviews(status);
CREATE INDEX idx_review_agg_pr ON review_aggregations(pr_url);
CREATE INDEX idx_review_agg_task ON review_aggregations(task_id);
CREATE INDEX idx_fp_project ON false_positives(project_id);
CREATE INDEX idx_fp_category ON false_positives(category);
```

---

## 十一、部署与监控

### 11.1 GitHub Action 配置清单

| Secret / 变量 | 必需 | 说明 |
|---------------|------|------|
| `ANTHROPIC_API_KEY` | 是 | Anthropic API Key，用于 claude-code-action |
| `AGENTFORGE_API_URL` | 是 | AgentForge 后端 API 地址 |
| `AGENTFORGE_TOKEN` | 是 | AgentForge 服务认证 Token |
| `GITHUB_TOKEN` | 自动 | GitHub 自动注入，需 permissions 声明 |

### 11.2 监控指标

| 指标 | 目标 | 告警阈值 |
|------|------|---------|
| Layer 1 审查耗时 p95 | < 5 分钟 | > 8 分钟 |
| Layer 2 审查耗时 p95 | < 15 分钟 | > 25 分钟 |
| 假阳性率 | < 5% | > 10% |
| 审查覆盖率 | 100% Agent PR | < 95% |
| 月度审查成本 | 预算内 | > 80% 预算 |
| Layer 2 触发率 | 30-50% | > 70% (可能过度触发) |
| Agent 修改后通过率 | > 80% | < 60% (Agent 质量问题) |

### 11.3 Prometheus 指标

```
# 审查耗时
review_duration_seconds{layer="1|2|3", dimension="quick|logic|security|performance|compliance"}

# 审查计数
review_total{layer, status="completed|failed", recommendation="approve|request_changes|reject"}

# 假阳性
review_false_positives_total{project, category}

# 成本
review_cost_usd{layer, model, dimension}

# Agent 修改循环次数
review_change_request_cycles{task_id}
```

---

## 附录

### A. 关键技术依赖

| 组件 | 版本 | 用途 |
|------|------|------|
| [claude-code-action](https://github.com/anthropics/claude-code-action) | v1.0 | Layer 1 GitHub Action 审查 |
| [Claude Agent SDK](https://docs.anthropic.com/en/docs/agent-sdk/overview) | latest | Layer 2 自建 Review Agent |
| GitHub Actions | - | CI/CD 流水线 |
| cc-connect (Fork) | - | IM 桥接（Layer 3 审批通知） |

### B. 配置参考

claude-code-action 完整输入参数参考（v1.0）：

| 参数 | 默认值 | 说明 |
|------|-------|------|
| `anthropic_api_key` | - | Anthropic API 密钥 |
| `claude_code_oauth_token` | - | OAuth Token（替代 API Key） |
| `prompt` | - | 审查指令 / 自定义模板 |
| `trigger_phrase` | `@claude` | 评论触发关键词 |
| `track_progress` | `false` | 启用进度追踪评论 |
| `claude_args` | - | Claude CLI 参数（`--model`, `--max-turns`, `--allowedTools`, `--json-schema` 等） |
| `settings` | - | Claude Code 设置（JSON 或文件路径） |
| `use_bedrock` | `false` | 使用 AWS Bedrock |
| `use_vertex` | `false` | 使用 Google Vertex AI |
| `use_foundry` | `false` | 使用 Microsoft Foundry |
| `classify_inline_comments` | `true` | 缓冲 inline 评论并分类（过滤测试/探测评论） |
| `include_fix_links` | `true` | 在审查反馈中包含"修复此问题"链接 |
| `additional_permissions` | - | 额外 GitHub 权限（如 `actions: read`） |
| `plugins` | - | Claude Code 插件列表 |

### C. 术语表

| 术语 | 说明 |
|------|------|
| Finding | 审查中发现的单个问题项 |
| 假阳性 (False Positive) | 被错误标记为问题的正常代码 |
| 交叉验证 | 多个审查维度的结果互相验证，提高准确性 |
| 分级路由 | 根据 PR 风险等级决定触发哪些审查层 |
| Session Resume | Claude Agent SDK 的会话恢复能力，避免重启 Agent |

---

> **文档状态：** 初稿完成
> **作者：** AgentForge 架构组
> **下一步：** 团队评审 → Phase 1 实现 Layer 1 → Phase 2 实现 Layer 2/3
