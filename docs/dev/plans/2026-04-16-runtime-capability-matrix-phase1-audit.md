# Runtime Capability Matrix — Phase 1: Audit & Matrix Filling — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md` 定义的 framework 转化为一份**完全填实**的能力矩阵——包含 7 个对标项目的能力盘点、AgentForge 三层自审、60–90 行 `matrix.csv`、逐条引用的 `sources.md`、`~`/`✗` 能力的分能力节、以及可进入 writing-plans 下一轮的 `gaps.yaml`。

**Architecture:** Phase 1 纯研究/文档产出，不修改 runtime 代码。流程分五阶段：（1）行骨架固定→（2）对标项目逐项调研（可并行）→（3）AgentForge 三层自审→（4）分能力节书写→（5）机器校验 + gaps.yaml 生成。每阶段结束都 commit，失败可回溯。

**Tech Stack:**
- **调研工具**：`mcp__deepwiki__ask_question`、`mcp__context7__query-docs`、`mcp__fetch__fetch`、`WebFetch`、`WebSearch`
- **代码审计**：`Grep`、`Glob`、`Read`（扫 `src-bridge/`、`src-go/`、`app/`、`lib/stores/`）
- **格式校验**：`awk` scripts（spec §5.5）
- **产出**：Markdown + CSV + YAML

**Spec reference:** `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md`（以下简称 SPEC）

**Constraint:** SPEC §9.2 证据快照日 = **2026-04-18**。在此日之前完成所有对标项目调研；此后上游更新只能进入 SPEC §13（开放问题），不再改既有单元格。

---

## File Structure

### 本 plan 产生 / 修改的文件

| 路径 | 角色 | 本 plan 责任 |
|---|---|---|
| `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md` | 主 spec | **追加** §6.* 分能力节；**更新** §14 变更记录 |
| `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/matrix.csv` | 机器矩阵 | **填实** 60–90 行 |
| `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/sources.md` | 脚注来源 | **填实** 所有 `[cc-*]`/`[sdk-*]`/`[cur-*]`/`[aid-*]`/`[cdx-*]`/`[oc-*]`/`[gmi-*]` 来源 |
| `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/gaps.yaml` | Gap 清单 | **填实** 初版 gap entries（P0-P3 标签） |
| `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/sources/screenshots/` | 归档截图 | **入库** 上游网页截图（档案化） |
| `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/research/` | 调研原始笔记 | **新建** 子目录存放 per-benchmark 研究 notes |
| `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/validate.sh` | 校验脚本 | **新建** 包装 SPEC §5.5 awk 脚本 |

### 不修改的文件

- `src-bridge/**`、`src-go/**`、`app/**`、`lib/**`：Phase 1 **只读**。任何代码改动归 Phase 2+（v2 接口重构 + per-adapter gap 修复，由后续 plan 覆盖）。
- 附件 `capability-matrix.d.ts` 等产物 TS 类型：归 Phase 2。

---

## Phase 1 任务分解

五个阶段，每阶段一到多个 Task。Task 内步骤为 2–5 分钟原子操作。

### Phase 结构

```
A. 骨架          → Task 1          (行枚举 + CSV 表头)
B. 对标调研      → Task 2–8        (7 benchmarks × 1 task; 可并行)
C. 自审          → Task 9–11       (FE / Go / Br 各 1 task)
D. 能力节书写    → Task 12–14      (§6 节；按分组批处理)
E. 汇总          → Task 15–17      (sources.md 规整; gaps.yaml 生成; 校验)
```

**并行性提示**：Task 2–8 彼此独立、可并行派发；Task 9–11 彼此独立、可并行派发。其余任务顺序执行。

---

## Task 1: 固定矩阵行骨架

**Files:**
- Modify: `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/matrix.csv`
- Create: `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/research/_skeleton-notes.md`

**目标**：把 SPEC §4.2 的 14 分组展开为 60–90 个原子能力行；每行只填 `row_key` + `capability` 两列，状态列留空白（填充在后续 task 完成）。

- [ ] **Step 1：枚举 Group 1–7 的原子能力**

参考 SPEC §4.2 + §4.3 粒度标准。对每一组，列出该组的 atomic capabilities（估计每组 3–8 行）。写入临时笔记：

```
# 1. Session & Lifecycle
1.1  session:create
1.2  session:resume
1.3  session:fork
1.4  session:fork:by-message-id
1.5  session:fork:cross-directory
1.6  session:rewind:to-message
1.7  session:rewind:to-checkpoint
1.8  session:interrupt
1.9  session:pause-resume
1.10 session:snapshot-export
1.11 session:checkpoint-list
# 2. Execution Control
2.1  exec:set-model
2.2  exec:set-thinking-budget
2.3  exec:set-max-turns
2.4  exec:set-temperature
2.5  exec:permission-mode:switch
2.6  exec:plan-mode:enter
2.7  exec:plan-mode:exit
... (以此类推至第 7 组)
```

存入 `research/_skeleton-notes.md`，作为后续 Task 2–8 的参考清单。

- [ ] **Step 2：枚举 Group 8–14 的原子能力**

同上，对剩余 7 组展开。特别注意：
- Group 8（Hooks）：每种 hook 类型一行（pre_tool_use / post_tool_use / user_prompt_submit / stop / notification / session_start / ...）
- Group 12（MCP）：server 定义 / 生命周期 / 工具发现 / 权限门控 / status / logs / 动态启停，逐项列
- Group 14（Output Styles）：按 SPEC §13.1 留有"待决"标签

- [ ] **Step 3：生成 CSV 骨架**

写入 `matrix.csv`：
- Line 1：header（按 SPEC §5.5 13 列，已存在）
- Line 2+：每行 `row_key,capability,,,,,,,,,,,§6.<组号>.<序号>`（状态列都空）

共 60–90 行。

- [ ] **Step 4：跑 anchor 完整性校验**

```bash
cd docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2
awk -F, 'NR>1 && $NF == "" { print "row " NR " missing anchor"; exit 1 }' matrix.csv
```

Expected：无输出，退出码 0。

- [ ] **Step 5：跑 row_key 唯一性校验**

```bash
awk -F, 'NR>1 { if (seen[$1]++) { print "duplicate row_key: " $1; exit 1 } }' matrix.csv
```

Expected：无输出，退出码 0。

- [ ] **Step 6：Commit**

```bash
rtk git add docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/matrix.csv \
           docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/research/_skeleton-notes.md
rtk git commit -m "docs(spec): add runtime capability row skeleton (60-90 rows)"
```

---

## Task 2: Claude CLI 能力盘点（调研 + 填列）

**Files:**
- Modify: `matrix.csv`（填 `claude_cli` 列，第 3 列）
- Create: `research/claude-cli.md`
- Modify: `sources.md`（追加 `cc-*` 条目）

**目标**：遍历 Task 1 的 60–90 行，逐行判定 Claude CLI 是否支持；`✓` / `~` / `✗` / `N-A` 四选一；`✓`/`~` 必须有脚注指向源头。

- [ ] **Step 1：扫官方 docs + help 输出**

```
# 使用工具
- mcp__fetch__fetch  https://docs.claude.com/en/docs/claude-code
- WebFetch           https://github.com/anthropics/claude-code
- Bash               claude --help > research/claude-cli-help.txt (如果 CLI 可用)
```

产出 `research/claude-cli.md`，结构：

```markdown
# Claude CLI 能力盘点

## Session & Lifecycle
- session:create → ✓ [cc-1]  https://docs.claude.com/.../sessions
- session:fork   → ✓ [cc-2]  https://docs.claude.com/.../fork
...

## Execution Control
...

# 脚注来源
[cc-1] https://docs.claude.com/en/docs/claude-code/sessions  accessed 2026-04-17  screenshot: cc-1.png
```

- [ ] **Step 2：补 hooks/skills/subagents/slash 专项**

Claude CLI 的 hooks / skills / subagents / slash commands 文档散落各处。专项查询：

```
mcp__deepwiki__ask_question {
  repoName: "anthropics/claude-code",
  question: "What hook types are supported? What are skills? How do subagents work?"
}
```

把回答归入 `research/claude-cli.md` 对应分组。

- [ ] **Step 3：归档网页截图**

对每个 `cc-*` 脚注对应的 URL，保存截图至 `sources/screenshots/cc-<n>.png`；或记录 `web.archive.org` 镜像 URL。

- [ ] **Step 4：填 CSV `claude_cli` 列**

打开 `matrix.csv`，按 Task 2 step 1-2 的判定逐行填第 3 列。

- [ ] **Step 5：更新 sources.md**

把 `research/claude-cli.md` 脚注表格合并到 `sources.md` 的 "Claude CLI" 小节。

- [ ] **Step 6：校验**

```bash
# 确保 cc 列无 ?
awk -F, 'NR>1 && $3 == "?" { print "row " NR; exit 1 }' matrix.csv
```

- [ ] **Step 7：Commit**

```bash
rtk git add docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/
rtk git commit -m "docs(spec): fill claude_cli capability column"
```

---

## Task 3: Claude Agent SDK 能力盘点

**Files:**
- Modify: `matrix.csv`（填 `claude_sdk` 列，第 4 列）
- Create: `research/claude-sdk.md`
- Modify: `sources.md`（追加 `sdk-*`）

**目标**：与 Task 2 结构同；数据源换成 `@anthropic-ai/claude-agent-sdk` 的 `types.d.ts` 与示例。

- [ ] **Step 1：获取 SDK 类型定义**

```
mcp__context7__resolve-library-id {
  libraryName: "Anthropic Claude Agent SDK",
  query: "runtime agent session hook cost"
}
→ 取到 libraryId

mcp__context7__query-docs {
  libraryId: "<resolved>",
  query: "query() Options hook callbacks interrupt setModel rewindFiles mcpServerStatus"
}
```

- [ ] **Step 2：查 GitHub 源码**

```
mcp__deepwiki__read_wiki_structure { repoName: "anthropics/claude-agent-sdk-typescript" }
mcp__deepwiki__read_wiki_contents  { repoName: "anthropics/claude-agent-sdk-typescript" }
```

- [ ] **Step 3：对照 Task 1 的行骨架逐一判定**

产出 `research/claude-sdk.md`，格式同 Task 2。

- [ ] **Step 4–7：截图 / 填 CSV / 更新 sources / 校验 / Commit**

步骤同 Task 2。Commit 信息：

```bash
rtk git commit -m "docs(spec): fill claude_sdk capability column"
```

---

## Task 4: Cursor 能力盘点

**Files:**
- Modify: `matrix.csv`（第 5 列）
- Create: `research/cursor.md`
- Modify: `sources.md`（追加 `cur-*`）

- [ ] **Step 1：搜集 Cursor docs + rules + agent mode**

```
WebFetch https://docs.cursor.com
WebSearch "Cursor agent mode hooks rules MCP"
```

重点关注：Rules 系统、`@` context、Agent mode、MCP、Tab completion、Composer 等。

- [ ] **Step 2：判定每行对 Cursor 的适用性**

Cursor 不是 CLI agent，大多能力 N-A（如 slash commands / hook types 可能不对等）。N-A 也要在脚注里写明理由。

- [ ] **Step 3–7：同 Task 2 的后续步骤**

Commit：

```bash
rtk git commit -m "docs(spec): fill cursor capability column"
```

---

## Task 5: Aider 能力盘点

**Files:**
- Modify: `matrix.csv`（第 6 列）
- Create: `research/aider.md`
- Modify: `sources.md`（追加 `aid-*`）

- [ ] **Step 1：Aider docs + GitHub**

```
mcp__deepwiki__ask_question {
  repoName: "paul-gauthier/aider",
  question: "What is the session model? Does Aider support hooks? How does /add, /commit work? What is the repo map?"
}
WebFetch https://aider.chat/docs
```

- [ ] **Step 2–7：同模板**

Commit：

```bash
rtk git commit -m "docs(spec): fill aider capability column"
```

---

## Task 6: Codex 能力盘点

**Files:**
- Modify: `matrix.csv`（第 7 列）
- Create: `research/codex.md`
- Modify: `sources.md`（追加 `cdx-*`）

- [ ] **Step 1：OpenAI Codex CLI docs + source**

```
WebFetch https://developers.openai.com/codex/cli
mcp__deepwiki__ask_question {
  repoName: "openai/codex",
  question: "approval flow; rollout; sandbox; model picker; MCP; session fork; resume"
}
```

- [ ] **Step 2：重点核查**

Codex 有 approval flow（vs. Claude hooks）、rollout 文件（vs. session）——需要细分多行以区分粒度。

- [ ] **Step 3–7：同模板**

Commit：

```bash
rtk git commit -m "docs(spec): fill codex capability column"
```

---

## Task 7: OpenCode 能力盘点

**Files:**
- Modify: `matrix.csv`（第 8 列）
- Create: `research/opencode.md`
- Modify: `sources.md`（追加 `oc-*`）

- [ ] **Step 1：OpenCode docs + source**

```
mcp__deepwiki__ask_question {
  repoName: "opencode-ai/opencode",  # 确认 repo name
  question: "terminal UI; session events; model picker; MCP; hooks"
}
```

- [ ] **Step 2：交叉验证**

既有 `src-bridge/src/opencode/` 目录下的 transport 实现，参考我们自己的集成来佐证 OpenCode 暴露的能力面。这是唯一一个"AgentForge 现状 vs 上游"可双向校验的 adapter。

- [ ] **Step 3–7：同模板**

Commit：

```bash
rtk git commit -m "docs(spec): fill opencode capability column"
```

---

## Task 8: Gemini CLI 能力盘点

**Files:**
- Modify: `matrix.csv`（第 9 列）
- Create: `research/gemini-cli.md`
- Modify: `sources.md`（追加 `gmi-*`）

- [ ] **Step 1：Gemini CLI docs + source**

```
mcp__deepwiki__ask_question {
  repoName: "google-gemini/gemini-cli",
  question: "extensions; MCP; session model; GEMINI.md memory; hooks; tool approval"
}
WebFetch https://github.com/google-gemini/gemini-cli
```

- [ ] **Step 2–7：同模板**

Commit：

```bash
rtk git commit -m "docs(spec): fill gemini_cli capability column"
```

---

## Task 9: AgentForge Frontend 自审

**Files:**
- Modify: `matrix.csv`（第 10 列 `fe`）
- Create: `research/agentforge-fe.md`
- No new sources.md entries（AgentForge 列不用脚注编号，用文件行号）

**目标**：根据 Task 1 行骨架，逐行判定 AgentForge 前端是否提供相应控制/展示。**证据必须是代码文件行号**，禁止"应该有"推断。

- [ ] **Step 1：枚举相关前端资产**

```bash
# 使用 Glob / Grep 扫以下路径
- lib/stores/*-store.ts  (重点：agent / session / role / plugin / cost / team / workflow / memory)
- app/(dashboard)/{agents,roles,plugins,memory,cost,workflow,settings}/**
- hooks/**
```

产出 `research/agentforge-fe.md`，列出每个 store 暴露的 action / state。

- [ ] **Step 2：逐行判定 + 填证据**

对每行 capability，在 FE 列填 `✓/~/✗/N-A`，并在 `research/agentforge-fe.md` 记录：
```
1.3 session:fork → ~  lib/stores/agent-store.ts:L210（仅基于 session_id，不支持 message_id）
8.3 hook:user_prompt_submit → ✗  未在任何 store 暴露
```

- [ ] **Step 3：填 CSV `fe` 列**

- [ ] **Step 4：校验**

```bash
awk -F, 'NR>1 && $10 == "?" { print "row " NR; exit 1 }' matrix.csv
```

- [ ] **Step 5：Commit**

```bash
rtk git add docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/
rtk git commit -m "docs(spec): audit AgentForge frontend layer"
```

---

## Task 10: AgentForge Go 自审

**Files:**
- Modify: `matrix.csv`（第 11 列 `go`）
- Create: `research/agentforge-go.md`

- [ ] **Step 1：枚举相关 Go 模块**

```bash
# Grep 扫：
- src-go/internal/role/**
- src-go/internal/plugin/**
- src-go/internal/memory/**
- src-go/internal/pool/**
- src-go/internal/ws/**
- src-go/internal/scheduler/**  (若能力涉及定时 hook)
```

对每模块列出：暴露的 HTTP endpoint、表/消息、关键类型。

- [ ] **Step 2–5：同 Task 9 pattern**

Commit：

```bash
rtk git commit -m "docs(spec): audit AgentForge go layer"
```

---

## Task 11: AgentForge Bridge 自审

**Files:**
- Modify: `matrix.csv`（第 12 列 `br`）
- Create: `research/agentforge-br.md`

- [ ] **Step 1：枚举 bridge 资产**

```bash
# Grep 扫：
- src-bridge/src/runtime/**
- src-bridge/src/handlers/*-runtime.ts
- src-bridge/src/mcp/**
- src-bridge/src/session/**
- src-bridge/src/filters/**
- src-bridge/src/plugins/**
- src-bridge/src/cost/**
- src-bridge/src/ws/**
```

- [ ] **Step 2：重点是 adapter 对比**

Bridge 列的特殊挑战：一行能力 per adapter 状态可能不同。本 plan 的 Bridge 列取的是 **"至少有一个 adapter 已实现"** 的聚合态。per-adapter 声明（例如 codex 是否支持 fork）进入 Task 12–14 的分能力节的 "各 adapter 声明" 小节。

- [ ] **Step 3–5：同 pattern**

Commit：

```bash
rtk git commit -m "docs(spec): audit AgentForge bridge layer"
```

---

## Task 12: 分能力节书写（Group 1–5）

**Files:**
- Modify: `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md`（追加 §6）

**目标**：对每个 matrix 中 `~` 或 `✗` 的 AgentForge 单元格对应的能力行，按 SPEC §6.1 八小节模板写节。本 task 覆盖 Group 1（Session）–5（Skills）。

**节排序**：严格按 `row_key` 升序。

- [ ] **Step 1：在 main spec 添加 §6 章节锚点**

在 SPEC 末尾（§13 前）插入：

```markdown
---

## 6. 分能力节（Per-capability sections）

> 以下节覆盖矩阵中所有 `~` / `✗` 的 AgentForge 能力行。按 row_key 升序排列。
> 全绿能力不展开；其状态只在矩阵中体现。
```

- [ ] **Step 2：逐节填写（Group 1 Session）**

对 Group 1 下所有 `~`/`✗` 行，按 §6.1 八小节模板写：
```markdown
### §6.1.3 session:fork

**1. 定义**（1-3 句）
**2. 对标证据**（表格，引用 sources.md 编号）
**3. AgentForge 现状**（FE/Go/Br 三层 + 代码行号）
**4. Gap 拆分**（可行动粒度）
**5. v2 接口提案**（TS 片段）
**6. 各 adapter 声明**（per-adapter 降级）
**7. 测试策略**（单元 / 集成 / 回归）
**8. 风险 / 开放问题**
```

- [ ] **Step 3：重复 Step 2 for Group 2 (Execution Control) / Group 3 (Permissions) / Group 4 (Memory) / Group 5 (Skills)**

- [ ] **Step 4：自查 8 小节完整性**

```bash
# grep 找所有 §6.*.* 节，确认每节都有 1-8 子标题
grep -E "^### §6\." docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md | wc -l
# 对每节，检查是否有 **1. 定义**、**2. 对标证据** ... **8. 风险**
```

- [ ] **Step 5：Commit**

```bash
rtk git commit -m "docs(spec): write per-capability sections for Groups 1-5"
```

---

## Task 13: 分能力节书写（Group 6–10）

**Files:** 同 Task 12

- [ ] **Step 1：重复 Task 12 Step 2 for Groups 6–10**

Groups：Subagents / Slash Commands / Hooks / Streaming & Events / Cost & Usage。

- [ ] **Step 2：自查**

- [ ] **Step 3：Commit**

```bash
rtk git commit -m "docs(spec): write per-capability sections for Groups 6-10"
```

---

## Task 14: 分能力节书写（Group 11–14 + 能力依赖图）

**Files:** 同 Task 12

- [ ] **Step 1：重复 Task 12 Step 2 for Groups 11–14**

Groups：File Checkpointing / MCP / Launch & Environment / Output Styles。

- [ ] **Step 2：在 §6 末尾追加能力依赖图（mermaid）**

```markdown
### §6.99 能力依赖图

> 标出哪些能力是其他能力的前置——指引后续 plan 的顺序。

\`\`\`mermaid
graph LR
  skills_discovery --> skills_invoke
  hooks_register --> hooks_preToolUse
  hooks_register --> hooks_postToolUse
  session_fork --> session_fork_by_message_id
  ...
\`\`\`
```

- [ ] **Step 3：全节号完整性自查**

确保 §6.<group>.<seq> 对矩阵 `~`/`✗` 行一一对应；无缺漏。

```bash
# 脚本：列矩阵里所有 ~/✗ 的 anchor（扫所有 10 列状态格：3-12），然后 grep 主 spec
# 必须每个 anchor 存在对应的 ### §6.X.Y 标题（§6.4 全绿例外除外——这里不会被选中）
awk -F, 'NR>1 { for (i=3; i<=12; i++) if ($i=="~" || $i=="✗") { print $NF; next } }' matrix.csv \
  | sort -u > /tmp/expected-anchors.txt
grep -E "^### §6\." docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md \
  | sed -E 's/^### (§[0-9.]+).*/\1/' | sort -u > /tmp/actual-anchors.txt
diff /tmp/expected-anchors.txt /tmp/actual-anchors.txt
```

> 注意：awk 条件**只要任意一列为 `~` 或 `✗` 即计数**（`next` 短路），覆盖所有 10 列状态格，包含 cursor/aider/codex/opencode/gemini_cli 等对标列。

Expected：diff 为空（或仅为正常的全绿行不生成节，见 SPEC §6.4）。

- [ ] **Step 4：Commit**

```bash
rtk git commit -m "docs(spec): write per-capability sections for Groups 11-14 + dependency graph"
```

---

## Task 15: 汇总 sources.md

**Files:**
- Modify: `sources.md`（从各 research/*.md 聚合脚注到规范表）
- Verify: 所有 `[cc-*]` 等引用在 sources.md 中可查到

- [ ] **Step 1：列主 spec 中所有脚注编号**

```bash
grep -oE '\[(cc|sdk|cur|aid|cdx|oc|gmi)-[0-9]+\]' docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md | sort -u > /tmp/used-refs.txt
```

- [ ] **Step 2：列 sources.md 中定义的脚注**

```bash
# 匹配 sources.md 表格行中的脚注 ID（形如 `| cc-1 | ...`），归一化为 [cc-1] 供 diff
grep -oE '\|[[:space:]]*((cc|sdk|cur|aid|cdx|oc|gmi)-[0-9]+)[[:space:]]*\|' \
  docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/sources.md \
  | sed -E 's/.*((cc|sdk|cur|aid|cdx|oc|gmi)-[0-9]+).*/[\1]/' \
  | sort -u > /tmp/defined-refs.txt
```

- [ ] **Step 3：Diff**

```bash
diff /tmp/used-refs.txt /tmp/defined-refs.txt
```

Expected：diff 空。如果主 spec 有引用但 sources.md 未定义 → 补 sources；反之（sources 多余）→ 删冗余。

- [ ] **Step 4：确保每条 sources 条目都有 URL + 访问日期 + 归档位置**

SPEC §9.2 要求：`URL | 访问日期 | 归档位置`。grep 检查：

```bash
awk -F'|' 'NR>1 && NF>=5 && $4!="" && $5!="" { next } NR>1 { print "incomplete source row: " NR; exit 1 }' sources.md
```

- [ ] **Step 5：Commit**

```bash
rtk git commit -m "docs(spec): consolidate sources.md with all benchmark citations"
```

---

## Task 16: 生成 `gaps.yaml`

**Files:**
- Modify: `gaps.yaml`（填实 entries）

**目标**：从 §6 分能力节的 "Gap 拆分" 小节抽取所有 gap，按 SPEC §8.6 schema 生成 YAML；按 SPEC §8.3 打 P0–P3 标签；按 SPEC §8.2 打四维。

- [ ] **Step 1：从主 spec 抽取 gap 条目**

对每个 §6.* 节，读取 "4. Gap 拆分" 小节。每个 `- [ ] gap-N: ...` 生成一个 YAML entry：
```yaml
- id: <ADAPTER_PREFIX>-<序号>    # CC=claude_code; CX=codex; OC=opencode; CS=cursor; AD=aider; GM=gemini; FE=前端; GO=Go; BR=bridge
  adapter: <runtime_key>
  capability: "<row_key>"
  axis:
    value: <blocker|high|medium|low>
    complexity: <S|M|L|XL>
    risk: <low|medium|high>
    stability: <stable|volatile>
  priority: <P0|P1|P2|P3>
  depends_on: [<其他 gap id>]
  roadmap_track: <A|B|C|D1|D2|D3|null>
  hint_plan_merge_group: "<adapter>:<group>"
```

- [ ] **Step 2：应用 P0 强制规则**

SPEC §8.3：当前 adapter 宣称支持（矩阵中 ✓）但实际是 stub → "诚信 gap"，强制 P0。

Task 11 应已发现这些：对 bridge 列 ✓ 但代码是 `throw new Error('TODO')` 的行做全局搜索：

```bash
rg -l "throw new Error\(['\"]TODO" src-bridge/src/
```

任何命中 → 标诚信 gap。

- [ ] **Step 3：填 `roadmap_track` 字段**

按 SPEC §10.2 表格映射：
- Hook-类 gap → Track C
- Skill-类 gap → Track D2
- NodeType / Function-类 gap → 与本 spec 无直接耦合，留空
- Frontend-扩展-类 gap → Track D1
- 其他 → null

- [ ] **Step 4：校验 YAML 语法**

```bash
# 用 node 或 python 简单 parse
python -c "import yaml; yaml.safe_load(open('docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/gaps.yaml'))"
```

Expected：无 exception。

- [ ] **Step 5：生成 Top-20 summary（§8.4 要求）**

按 P0 > P1 排序取前 20，写入主 spec §1 之后的 "Executive Summary" 小节：

```markdown
### 1.1 Top-20 全局 Gap（Executive Summary）

| # | Gap ID | Adapter | Capability | P | 估工 | Roadmap |
|---|---|---|---|---|---|---|
| 1 | CC-003 | claude_code | 6.8.3 hook:user_prompt_submit | P0 | M | C |
| 2 | ... |
```

- [ ] **Step 6：Commit**

```bash
rtk git add docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/gaps.yaml \
           docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md
rtk git commit -m "docs(spec): generate initial gaps.yaml + top-20 executive summary"
```

---

## Task 17: 最终校验 + 变更记录

**Files:**
- Create: `validate.sh`（封装 SPEC §5.5 + §9.3 的 8 条校验）
- Modify: `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md`（§14 追加 draft-2 条目）

- [ ] **Step 1：写 `validate.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail

BASE="docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2"
SPEC="docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md"

echo "→ [1/8] matrix.csv: no ? cells in columns 3-12"
awk -F, 'NR>1 { for (i=3; i<=12; i++) if ($i == "?") { print "row "NR" col "i; exit 1 } }' "$BASE/matrix.csv"

echo "→ [2/8] matrix.csv: every row has Anchor"
awk -F, 'NR>1 && $NF == "" { print "row "NR" missing anchor"; exit 1 }' "$BASE/matrix.csv"

echo "→ [3/8] matrix.csv: row_key uniqueness"
awk -F, 'NR>1 { if (seen[$1]++) { print "dup "$1; exit 1 } }' "$BASE/matrix.csv"

echo "→ [4/8] sources.md: used ⊆ defined"
grep -oE '\[(cc|sdk|cur|aid|cdx|oc|gmi)-[0-9]+\]' "$SPEC" | sort -u > /tmp/used.txt
grep -oE '\|[[:space:]]*((cc|sdk|cur|aid|cdx|oc|gmi)-[0-9]+)[[:space:]]*\|' "$BASE/sources.md" \
  | sed -E 's/.*((cc|sdk|cur|aid|cdx|oc|gmi)-[0-9]+).*/[\1]/' | sort -u > /tmp/defined.txt
missing=$(comm -23 /tmp/used.txt /tmp/defined.txt)
if [ -z "$missing" ]; then
  echo "  OK"
else
  echo "missing sources:"; echo "$missing"; exit 1
fi

echo "→ [5/8] gaps.yaml: valid YAML"
# 兼容 python3 / python
PY=$(command -v python3 || command -v python)
if [ -z "$PY" ]; then echo "python/python3 not on PATH"; exit 1; fi
"$PY" -c "import yaml; yaml.safe_load(open('$BASE/gaps.yaml'))"

echo "→ [6/8] §6 anchors cover all matrix ~/✗ rows (any of 10 status cols 3-12)"
awk -F, 'NR>1 { for (i=3; i<=12; i++) if ($i=="~" || $i=="✗") { print $NF; next } }' "$BASE/matrix.csv" \
  | sort -u > /tmp/expected.txt
grep -oE "^### §6\.[0-9]+\.[0-9]+" "$SPEC" | sed -E 's/^### //' | sort -u > /tmp/actual.txt
diff /tmp/expected.txt /tmp/actual.txt || { echo "anchor mismatch"; exit 1; }

echo "→ [7/8] every §6.X.Y section has all 8 subsections (1.-8.)"
# 状态机：每个 section 维护一个 8 位 "seen" 数组；进入新 section 或 EOF 时核验全 1
# 仅用 POSIX awk（避开 gawk 专有的 or/lshift）
awk '
  function check_complete(sect,    i) {
    if (sect == "") return
    for (i = 1; i <= 8; i++) {
      if (!(i in seen)) {
        print "incomplete: " sect " missing subsection " i
        exit 1
      }
    }
  }
  /^### §6\./ {
    check_complete(cur_sect)
    cur_sect = $0
    delete seen
  }
  /^\*\*[1-8]\. / {
    n = substr($0, 3, 1) + 0
    seen[n] = 1
  }
  END { check_complete(cur_sect) }
' "$SPEC"

echo "→ [8/8] no TBD/TODO in main spec body"
! grep -E '(TBD|XXX|TODO|PLACEHOLDER)' "$SPEC"

echo "✅ All 8 checks passed."
```

- [ ] **Step 2：跑 `validate.sh`**

```bash
chmod +x docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/validate.sh
./docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/validate.sh
```

Expected：8 项全绿，最终 `✅ All 8 checks passed.`

- [ ] **Step 3：修任何红灯，循环直到全绿**

- [ ] **Step 4：§14 变更记录追加**

```markdown
| 2026-04-18 | draft-2 | Max Qian | Phase 1 完成：矩阵 N 行填实；§6 能力节 M 节；gaps.yaml N 条 entries；validate.sh 8 项全绿 |
```

把 N / M 数字替换为实际。

- [ ] **Step 5：Final commit**

```bash
rtk git add docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/validate.sh \
           docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md
rtk git commit -m "docs(spec): Phase 1 complete — matrix, capability sections, gaps.yaml, validate.sh

- matrix.csv: N rows filled (7 benchmarks × 3 AgentForge layers)
- sources.md: all [cc|sdk|cur|aid|cdx|oc|gmi]-* footnotes resolved
- §6 per-capability sections: M nodes (8-subsection template enforced)
- gaps.yaml: N entries, Top-20 summary in §1.1
- validate.sh: 8-check pipeline green

Phase 2 (v2 interface refactor + per-gap plans) unblocked.
"
```

---

## 验收与下一步

### 本 plan 完成标准

1. `validate.sh` 8 项全绿
2. `matrix.csv` 无 `?` 单元格
3. `sources.md` 所有脚注可溯至 URL + 归档
4. `gaps.yaml` 至少有 N 条（N ≥ 20）entries，含 P0 标签
5. 主 spec §6 覆盖所有矩阵 `~`/`✗` AgentForge 行（SPEC §6.4 例外规则除外）
6. 所有 commit 信息以 `docs(spec):` 开头

### Phase 2+ 计划（不属本 plan）

`gaps.yaml` 产出后，按 SPEC §10.3 规则：
- 按 `hint_plan_merge_group` 聚合
- 每个 group → 一次 writing-plans 调用
- 第一个实施 plan 应是 **`RuntimeAdapterV2` 接口骨架 + 错误类型细化**（解除 AgentRuntime.claudeQuery 硬编码），之后才做具体 gap 修复

### 本 plan 的测试/回归影响

Phase 1 **纯文档**。不改任何 runtime / app 代码。现有 test suite 不受影响；CI 不需要额外配置。
