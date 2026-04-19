---
title: Agent Runtime 能力矩阵 + Adapter v2 设计
date: 2026-04-16
status: draft
owner: Max Qian
scope: bridge + go + frontend
benchmarks: [claude-cli, claude-sdk, cursor, aider, codex, opencode, gemini-cli]
related-tracks: [C, D2]
supersedes: []
blocks: []
---

# Agent Runtime 能力矩阵 + Adapter v2 设计

## 1. 背景与目标

AgentForge 在 `src-bridge` 已实装 8 个 runtime adapter（`claude_code` / `codex` / `opencode` / `command` / `cursor` / `gemini` / `qoder` / `iflow`），但当前实现处于**单点参照**阶段：
- `AgentRuntime.claudeQuery: ClaudeQueryControl` 和 `AgentStatus.live_controls` 等关键字段硬编码 `runtime === "claude_code"`（见 `src-bridge/src/runtime/agent-runtime.ts:134-144`）；
- `registry.ts:144-175` 已具备 14 操作 `RuntimeAdapter` 接口骨架，但对 skills / subagents / slash commands / plan mode / memory / output styles 这些 Claude CLI 一等能力**尚未建模**；
- 非 claude adapter 的 "完整度" 没有可核验的对齐基线，不知道哪里是"不支持"、哪里是"应支持但没做"。

本 spec 通过 **能力矩阵 + 统一接口 v2 + Per-adapter gap 清单** 三件套，把"和 ClaudeCode 之类交互是否完整"的追问转成**可机器审计的硬契约**。

### 目标

1. **横向盘点**：Claude Code CLI / Claude Agent SDK / Cursor / Aider / Codex / OpenCode / Gemini CLI 的能力并集；
2. **纵向盘点**：AgentForge 三层（Frontend / Go / Bridge）对每个能力的当前支持状态；
3. **推导 v2**：由并集自底向上推导 `RuntimeAdapterV2` 接口，并为每个 adapter 标注 per-capability 声明（full / partial / none）；
4. **拆解 gap**：生成可直接进入 `writing-plans` 阶段的条目化 gap 清单（带四维标签与 P0–P3 优先级）。

### 非目标

- 不做商业化、SaaS 化、计费分层设计；
- 不重写 DAG workflow 引擎本体（归 Plugin Extensibility Roadmap Track A/B）；
- 不做多 adapter 协作编排（已由 workflow 解决）；
- 不引入新的 runtime（Aider/Devin 等只作为对标来源，不承诺接入）；
- 不做 UX 设计（本 spec 只规定前端"有没有该控制/数据"，不规定"长什么样"）。

---

## 2. 范围

### 2.1 In

| 子系统 | 关键路径 |
|---|---|
| **Bridge runtime 抽象** | `src-bridge/src/runtime/**`、`src-bridge/src/handlers/*-runtime.ts` |
| **Bridge 相关模块** | `src-bridge/src/mcp/**`、`src-bridge/src/session/**`、`src-bridge/src/filters/**`、`src-bridge/src/plugins/**`、`src-bridge/src/cost/**` |
| **Go 编排相关** | `src-go/internal/role`、`src-go/internal/plugin`、`src-go/internal/memory`、`src-go/internal/pool`、`src-go/internal/ws`（仅与 adapter 能力相关部分） |
| **Frontend 控制面** | `lib/stores/{agent,session,role,plugin,cost,team,workflow,memory}-store.ts`、`app/(dashboard)/{agents,roles,plugins,memory,cost,workflow,settings}/` |
| **Adapter 枚举** | `claude_code` / `codex` / `opencode` / `command` / `cursor` / `gemini` / `qoder` / `iflow`（8 个，含 `command` 通用 CLI） |

### 2.2 Out

明确排除于本 spec：

- DAG workflow 引擎本体（NodeTypeRegistry / FunctionRegistry，归 Roadmap Track A/B）；
- 插件 marketplace 商业化、定价、许可；
- 多 agent 间直接通信（仅允许通过 workflow 协作）；
- 新 runtime 接入（如 Aider、Devin、LangChain Agents）；
- UX 视觉设计、信息架构、交互动效。

### 2.3 边界说明

- Roadmap Track **C（Hook 系统）** 和 **D2（Skill Registry）** 与本 spec §4.8（Hooks）、§4.5（Skills）强耦合。本 spec **阻塞**这两条 track 的 brainstorm/plan 起步。
- Roadmap Track **A / B / D1 / D3** 不与本 spec 耦合，可并行推进。

---

## 3. 原则

1. **契约第一**：矩阵列轴与 v2 接口签名先冻结，再谈实现改动。Claude CLI 是"最大超集"参考源，但不是唯一真相。
2. **不搞可选方法**：v2 接口沿用现有 `UnsupportedOperationError` 模式——所有方法恒存在，不支持时抛结构化异常（见 §7.4）。禁止 `adapter.fork?.()` 到处判空。
3. **降级显式**：调用方在调用前查 `adapter.capabilities`；调用结果里带 `degraded_by` 字段说明为什么回退。
4. **不保证向后兼容**（依据 MEMORY：AgentForge 处内部测试，breaking 自由）。
5. **"不省略"硬约束**：矩阵每个单元格必须落定（`✓` / `~` / `✗` / `N-A`），不允许 `?`；`~` 必须在分能力节展开；gap 必须可行动粒度。
6. **学习但不照抄**：对标能力分三档——
   - **采纳**：有价值且符合 AgentForge 架构 → 进接口；
   - **留档**：有趣但场景外 → 矩阵标注但声明"暂不支持"；
   - **明确拒绝**：与 AgentForge 架构冲突 → 写明理由。

---

## 4. 能力分类法（矩阵行轴）

### 4.1 分组原则

- **原子性**：每行能力小到 "adapter 要么实现要么不实现"；禁止 "Session 管理" 这类泛词；
- **正交性**：能力属于且仅属于一个分组；跨组时在副标题注明主组并交叉引用；
- **对标可溯**：每行都能指回至少一个对标项目的文档/代码源。

### 4.2 14 个一级分组

| # | 分组 | 主要内容 | 主要对标源 |
|---|---|---|---|
| 1 | **Session & Lifecycle** | create / resume / fork / rewind(to message) / interrupt / pause & resume / revert / checkpoint list / session snapshot export | Claude CLI + SDK；Codex rollout |
| 2 | **Execution Control** | setModel / setThinkingBudget / setMaxTurns / setTemperature / permission_mode / plan mode | Claude SDK；Cursor agent mode |
| 3 | **Tool & File Permissions** | tool allowlist/denylist / file scope / network policy / shell allowlist / write confirm level | Claude CLI；Codex approval |
| 4 | **Context & Memory** | CLAUDE.md / AGENTS.md / GEMINI.md 三级 / memory CRUD API / auto-load 规则 / 记忆压缩 | Claude CLI memory；Cursor rules |
| 5 | **Skills Registry** | skill 发现 / 触发 / 参数传递 / 嵌套 / 版本依赖 | Claude CLI skills；Roadmap D2 |
| 6 | **Subagents** | 派发 / 隔离上下文 / 结果回传 / 并行 / 嵌套层级 / 工具能力继承 | Claude CLI subagent；Aider |
| 7 | **Slash / User Commands** | 注册 / 参数 schema / 执行 / 内置 vs 用户 | Claude CLI slash commands |
| 8 | **Hooks** | PreToolUse / PostToolUse / UserPromptSubmit / Stop / Notification / SessionStart 等类型；回调管理；失败隔离；优先级 | Claude CLI hooks |
| 9 | **Streaming & Events** | tool_use / tool_result / thinking_delta / text_delta / subagent / hook / cost / MCP events；事件 schema；传输协议 | Claude SDK；Codex events |
| 10 | **Cost & Usage Accounting** | per-turn / per-session / cache_read / cache_creation / 多模型混合 / 预算告警 / 审计导出 | Claude SDK usage |
| 11 | **File Checkpointing & Diff** | 文件 checkpoint / rewind 影响的文件 revert / per-turn diff / unified-diff / 与 git 关系 | Claude CLI rewind；Aider |
| 12 | **MCP Integration** | server 定义 / 生命周期 / 工具发现 / 权限门控 / status / logs / 动态启停 | Claude CLI MCP；Codex |
| 13 | **Launch & Environment** | working_directory / additional_directories / env / CLI flag 转译 / ensureAvailable / 诊断 / profile 系统 | Claude CLI flags；Gemini extensions |
| 14 | **Output Styles & Personality** | output style / system prompt override/append / 角色人格继承 / tone/language 约束 | Claude CLI output styles；Cursor rules |

### 4.3 粒度标准

能力粒度以 **"adapter 行为会出现差异的边界"** 为准。示例：
- "Session fork" 是一行；"Session fork 跨目录"另一行；"Session fork 基于 message_id"另一行——因为 Codex 支持前两者但不支持第三者；
- "Hook：pre_tool_use" 一行；"Hook：post_tool_use" 另一行；"Hook：user_prompt_submit" 再一行。

**预估矩阵行数：60–90 个原子能力**。

### 4.4 边界级能力（不独立成组，单列入最接近的分组）

| 能力 | 归属分组 | 原因 |
|---|---|---|
| Plan Mode | 2（Execution Control） | 实现是"权限模式 + 交互循环"，不成组 |
| Todo / Task tracking | 9（Streaming） | 作为事件流项目处理 |
| Permission modes 枚举（bypass/acceptEdits/plan/default/dontAsk） | 3（Permissions） | 是 3 的细分项 |
| ExitPlanMode / EnterPlanMode tool | 2 | 作为特殊 tool，不在 3（tool permission） |

---

## 5. 矩阵格式

### 5.1 列轴（固定 10 列）

| 列组 | 列 | 语义 |
|---|---|---|
| **对标项目（7 列）** | Claude CLI / Claude SDK / Cursor / Aider / Codex / OpenCode / Gemini CLI | 该项目的用户能否通过文档化方式使用该能力 |
| **AgentForge 三层（3 列）** | FE / Go / Br | 代码**已合入 master 且非 dead code**——design-stage artifacts 不算 |

### 5.2 符号集

| 符号 | 语义 | 何时用 |
|---|---|---|
| `✓` | 完整支持 | 文档 + 代码 + 测试三者都能验证 |
| `~` | 部分支持 | 存在但功能缺失或有已知限制（必须在分能力节展开） |
| `✗` | 明确不支持 | 代码/文档未体现；显式拒绝 |
| `N-A` | 不适用 | 该层不应承担（如 cost accounting 在 Frontend 列常 N-A） |
| `?` | **禁用** | 所有格子必须落定；review 前必须消除 |

### 5.3 脚注系统

- 对标项目列的 `✓`/`~` 必须附脚注编号指向 `sources.md`，格式：`[<prefix>-<n>]`，前缀：
  - `cc-` Claude CLI
  - `sdk-` Claude Agent SDK
  - `cur-` Cursor
  - `aid-` Aider
  - `cdx-` Codex
  - `oc-` OpenCode
  - `gmi-` Gemini CLI
- AgentForge 列的 `~`/`✗` 必须指向 repo 内文件路径 + 行号或 issue 编号，证明现状真的是这样。

### 5.4 分组子表格式

不用一张大表（防滚屏找行），每分组一张 14 张子表。每表顶部 1–2 段前言界定本组边界；每表底部"小结"给出本组 AgentForge gap 行数。

示例（分组 8 缩略版）：

```
## 8. Hooks
前言：由 adapter 在特定生命周期事件时执行用户回调的机制。
      回调可修改行为（拦截/改写）或仅观察。

行                              | CC | SDK | Cur | Aid | Cdx | OC  | Gmi | FE | Go | Br | Anchor
8.1 hook:pre-tool-use           | ✓  | ✓   | ✗   | ✗   | ~   | ✗   | ✗   | ✓  | ✓  | ✓  | →§6.8.1
8.2 hook:post-tool-use          | ✓  | ✓   | ✗   | ✗   | ~   | ✗   | ✗   | ~  | ✓  | ✓  | →§6.8.2
8.3 hook:user-prompt-submit     | ✓  | ✓   | ✗   | ✗   | ✗   | ✗   | ✗   | ✗  | ✗  | ~  | →§6.8.3
8.4 hook:session-start          | ✓  | ✓   | ✗   | ✗   | ✗   | ✗   | ✗   | ✗  | ✗  | ✓  | →§6.8.4
... (完整版 10-12 行)

小结：11 行；AgentForge gap 3 行（8.3, 8.5, 8.9）。
```

### 5.5 机器可读导出

Spec 同时导出 `matrix.csv`（见附件目录）。CSV 在 §5.4 的"行"列基础上**拆两列**以便 awk/jq/sql 处理：

| CSV 列序 | 列名 | 对应 §5.4 子表位置 | 语义 |
|---|---|---|---|
| 1 | `row_key` | "行"列的编号部分，如 `8.1` | group.seq，唯一键 |
| 2 | `capability` | "行"列的命名部分，如 `hook:pre-tool-use` | 可读能力名（纯文本，非状态格）；必须用 `namespace:kebab-slug` 格式，禁止 snake_case 或空格 |
| 3–9 | `claude_cli` / `claude_sdk` / `cursor` / `aider` / `codex` / `opencode` / `gemini_cli` | 对标 7 列 | 状态格 `✓/~/✗/N-A` |
| 10–12 | `fe` / `go` / `br` | AgentForge 3 列 | 状态格 `✓/~/✗/N-A` |
| 13 | `anchor` | "Anchor"列，如 `§6.8.1` | 指向分能力节 |

状态格列位因此从**第 3 列开始**（第 1-2 列是文本键）。

**完整性验证脚本**（spec 附带，review 流程强制跑）：

注：CSV 不得包含注释行或空行；header 固定单行，header 之后全部是数据行。

```bash
# 确保无 ? 单元格（仅扫描状态列 3-12，跳过 row_key / capability / anchor）
awk -F, 'NR>1 { for (i=3; i<=12; i++) if ($i == "?") { print NR":"i; exit 1 } }' \
    2026-04-16-runtime-capability-matrix-and-adapter-v2/matrix.csv

# 确保每行都有 Anchor（最后一列）
awk -F, 'NR>1 && $NF == "" { print "row " NR " missing anchor"; exit 1 }' \
    2026-04-16-runtime-capability-matrix-and-adapter-v2/matrix.csv

# 确保 row_key 全唯一
awk -F, 'NR>1 { if (seen[$1]++) { print "duplicate row_key: " $1; exit 1 } }' \
    2026-04-16-runtime-capability-matrix-and-adapter-v2/matrix.csv
```

---

## 6. 分能力节模板

矩阵里每个 `~` 或 `✗` 行在此展开。**8 小节固定，缺一不可**；spec review 拿这个做 checklist。

### 6.1 节模板

~~~markdown
### §6.<组号>.<序号> <能力简称>

**1. 定义**（1-3 句）
<能力边界。概念混淆时必须切清。>

**2. 对标证据**
| 项目 | 支持 | 能力形态 | 来源 |
|---|---|---|---|
| Claude CLI | ✓ | ... | [cc-12] docs URL |
| Claude SDK | ✓ | ... | [sdk-7] types.ts#L200 |
| Cursor | ✗ | — | [cur-na] |
| Aider | ~ | ... | [aid-3] |
| Codex | ~ | ... | [cdx-5] |
| OpenCode | ✗ | — | [oc-na] |
| Gemini CLI | ✗ | — | [gmi-na] |

**3. AgentForge 现状**
| 层 | 状态 | 证据文件:行 | 说明 |
|---|---|---|---|
| Frontend | ✓/~/✗ | `lib/stores/agent-store.ts:L150` | ... |
| Go | ✓/~/✗ | `src-go/internal/role/...` | ... |
| Bridge | ✓/~/✗ | `src-bridge/src/runtime/hook-callback-manager.ts` | ... |

**4. Gap 拆分**（逐条；每条可能独立成 plan）
- [ ] gap-1: <具体缺失点，可执行粒度>
- [ ] gap-2: ...

**5. v2 接口提案**
```ts
// 本能力对应的方法签名/类型（从 §7 摘录 + 细化）
```

**6. 各 adapter 声明**
| Adapter | 声明 | 不支持时的降级建议 |
|---|---|---|
| claude_code | full | — |
| codex | partial | ... |
| opencode | none | 抛 UnsupportedOperationError(reason:"runtime_limitation") |
| cursor/gemini/qoder/iflow | none | 同上 |

**7. 测试策略**
- 单元：<具体测试文件 + 命名建议>
- 集成：<e2e 场景 + 最小重现>
- 回归：<现有哪些 test 会被影响>

**8. 风险 / 开放问题**
- <已识别的设计不确定性>
- <策略未定事项>
~~~

### 6.2 硬规则

1. **8 小节必填**：空缺 = review 驳回；
2. **对标证据必须有 URL/文件行号**：禁止"据我所知"；
3. **Gap 可行动粒度**：单条 gap 能独立成 plan（估工量 ≤ XL）；禁止"重写整块"这类抽象 gap；
4. **测试策略三档必填**：即使 N-A 也要写理由；
5. **"风险"节不得为空**：可以写 "no known risks" 但需要一句论证。

### 6.3 排序与交叉引用

- 节按 §4.2 分组号 + 组内序号排列；
- 跨组依赖用 `→ §6.2.3` 锚点显式交叉引用；
- Spec 末尾有 **"能力依赖图"**（mermaid），标出哪些 gap 是前置。

### 6.4 节选裁剪规则

**矩阵全绿（三列 ✓ + 对标集里无 `~`/`✗` 争议）的能力不生成节**，仅在矩阵中标注。预计 30–50 个能力需展开，总量约 50–80 页 markdown。

---

## 7. RuntimeAdapter v2 接口

### 7.1 设计风格

采用 **Fat Interface + Declarative Capabilities + Typed UnsupportedOperationError**：
- 沿用 `registry.ts:144-175` 现有骨架；
- 所有方法恒存在；不支持时抛结构化 `UnsupportedOperationError`；
- adapter 自报 `capabilities: RuntimeCapabilityMatrix`，启动时与全局矩阵交叉校验。

### 7.2 接口定义（TS 草案）

```ts
export interface RuntimeAdapterV2 {
  // --- Identity & Catalog ---
  readonly key: AgentRuntimeKey;
  readonly label: string;
  readonly capabilities: RuntimeCapabilityMatrix;
  readonly launchContract: RuntimeLaunchContract;
  readonly lifecycle: RuntimeLifecycleMetadata;
  getDiagnostics(): Promise<RuntimeDiagnostic[]>;
  ensureAvailable(): Promise<void>;

  // --- Group 1: Session & Lifecycle ---
  execute(ctx: ExecuteCtx): Promise<void>;
  interrupt(ctx: RuntimeCtx): Promise<void>;
  fork(ctx: RuntimeCtx, params: ForkParams): Promise<ForkResult>;
  rewind(ctx: RuntimeCtx, params: RewindParams): Promise<void>;     // 原 rollback+revert 合并
  listCheckpoints(ctx: RuntimeCtx): Promise<Checkpoint[]>;
  snapshot(ctx: RuntimeCtx): Promise<SessionSnapshot>;

  // --- Group 2: Execution Control ---
  setModel(ctx: RuntimeCtx, params: SetModelParams): Promise<void>;
  setThinkingBudget(ctx: RuntimeCtx, params: ThinkingBudgetParams): Promise<void>;
  setPermissionMode(ctx: RuntimeCtx, mode: PermissionMode): Promise<void>;
  enterPlanMode(ctx: RuntimeCtx): Promise<void>;
  exitPlanMode(ctx: RuntimeCtx, params: ExitPlanParams): Promise<void>;

  // --- Group 8: Hooks ---
  registerHook(ctx: RuntimeCtx, hook: HookBinding): Promise<HookHandle>;
  unregisterHook(ctx: RuntimeCtx, handle: HookHandle): Promise<void>;

  // --- Group 5: Skills ---
  listSkills(ctx: RuntimeCtx): Promise<SkillDescriptor[]>;
  invokeSkill(ctx: RuntimeCtx, params: InvokeSkillParams): Promise<unknown>;

  // --- Group 6: Subagents ---
  spawnSubagent(ctx: RuntimeCtx, params: SpawnSubagentParams): Promise<SubagentHandle>;

  // --- Group 7: Slash Commands ---
  listCommands(ctx: RuntimeCtx): Promise<CommandDescriptor[]>;
  executeCommand(ctx: RuntimeCtx, params: CommandParams): Promise<unknown>;

  // --- Group 11: File Checkpointing & Diff ---
  getDiff(ctx: RuntimeCtx, params?: DiffParams): Promise<Diff>;
  rewindFiles(ctx: RuntimeCtx, params: RewindFilesParams): Promise<RewindFilesResult>;

  // --- Group 12: MCP ---
  getMcpStatus(ctx: RuntimeCtx): Promise<MCPStatus>;

  // --- Groups 9, 10: Messages / Cost ---
  getMessages(ctx: RuntimeCtx): Promise<MessageLog>;
  getCost(ctx: RuntimeCtx): Promise<CostSnapshot>;

  // --- Group 13: Launch ---
  executeShell(ctx: RuntimeCtx, params: ShellParams): Promise<unknown>;
}
```

共 **22 个方法**（较当前 14 个扩充 8 个）。所有类型声明（`HookBinding` / `SkillDescriptor` / `PermissionMode` 等）由分能力节细化。

#### 7.2.1 为何 Group 3 / Group 4 不出现独立方法签名

- **Group 3（Tool & File Permissions）**：tool allowlist/denylist / file scope / network policy 是**启动期配置**而非可变操作，通过 `ExecuteCtx` 的 `ExecuteRequest` 字段与 `launchContract` 供给；`setPermissionMode`（运行期切换）在 Group 2 已覆盖。
- **Group 4（Context & Memory）**：`CLAUDE.md` / `AGENTS.md` / `GEMINI.md` 三级记忆通过**环境自动加载**（工作目录层级扫描）而非 adapter API；程序化的 memory CRUD 属于 Go 侧 `internal/memory` 的能力，不是 bridge adapter 的职责；如果后续需要暴露 adapter 层 memory 查询接口，将在分能力节 §6.4.* 讨论并酌情扩充 v2 方法集。

矩阵仍然对 Group 3 / 4 独立列出行；其状态体现在 `RuntimeCapabilityMatrix.permissions` / `.memory` 自报字段与对应的 §6 能力节中。

### 7.3 RuntimeCapabilityMatrix 自报

```ts
export type SupportLevel = "full" | "partial" | "none";

export interface RuntimeCapabilityMatrix {
  session:    { fork: SupportLevel; rewind: SupportLevel; snapshot: SupportLevel; /* ... */ };
  execution:  { setModel: SupportLevel; setThinkingBudget: SupportLevel; planMode: SupportLevel; /* ... */ };
  permissions:{ toolAllowlist: SupportLevel; fileScope: SupportLevel; /* ... */ };
  memory:     { claudeMd: SupportLevel; agentsMd: SupportLevel; geminiMd: SupportLevel; /* ... */ };
  skills:     { discovery: SupportLevel; invoke: SupportLevel; nesting: SupportLevel; /* ... */ };
  subagents:  { spawn: SupportLevel; parallel: SupportLevel; nested: SupportLevel; /* ... */ };
  commands:   { registration: SupportLevel; execution: SupportLevel; /* ... */ };
  hooks:      { preToolUse: SupportLevel; postToolUse: SupportLevel; userPromptSubmit: SupportLevel; /* ... */ };
  events:     { toolUse: SupportLevel; thinkingDelta: SupportLevel; todoWrite: SupportLevel; /* ... */ };
  cost:       { perTurn: SupportLevel; cacheAccounting: SupportLevel; /* ... */ };
  fileCheckpoint: { diff: SupportLevel; rewindFiles: SupportLevel; /* ... */ };
  mcp:        { serverConfig: SupportLevel; dynamicStart: SupportLevel; /* ... */ };
  launch:     { additionalDirectories: SupportLevel; profileSystem: SupportLevel; /* ... */ };
  outputStyle:{ systemPromptOverride: SupportLevel; append: SupportLevel; /* ... */ };
}
```

**约束**：
- 全局矩阵（§5）是 `capabilities` 的规范真相；
- Spec 附带 helper：`validateAdapterAgainstMatrix(adapter)`——启动时校验 adapter 自报 与 全局矩阵一致，不一致直接拒绝注册；
- adapter 声明 `partial` 必须在 adapter 文件顶部注释说明缺失点；
- `/* ... */` 省略的 key 集合**由矩阵派生**：每个原子能力 row 对应一个 key；矩阵填充完成后自动导出完整 TS 类型声明（脚本见 `validateAdapterAgainstMatrix` 附带工具）；
- **依赖 §13.1**：Group 14（Output Styles）是否保留独立分组待矩阵填充时观察；若合并到 Group 4，`outputStyle` 键整体下沉到 `memory.outputStyle.*`，其余 adapter 实现不受影响。

### 7.4 错误类型细化

```ts
export class UnsupportedOperationError extends Error {
  readonly operation: RuntimeOperationName;
  readonly runtime: AgentRuntimeKey;
  readonly reason: "not_implemented" | "runtime_limitation" | "policy_disabled";
  readonly hint?: string;
}

export class PartialCapabilityError extends Error {
  readonly operation: RuntimeOperationName;
  readonly runtime: AgentRuntimeKey;
  readonly missing_feature: string;
}
```

调用方可根据 `reason` 选择降级路径或生成用户文案。

### 7.5 Ctx 参数化

当前 `adapter.fork(runtime: AgentRuntime, params)` 耦合全量 `AgentRuntime`。v2 引入：

```ts
export interface RuntimeCtx {
  runtime: AgentRuntime;
  streamer: EventSink;
  continuity: RuntimeContinuityState | null;
}
```

配套变更：
- **移除** `AgentRuntime.claudeQuery: ClaudeQueryControl`（硬编码 Claude 专用控制）；
- **改为** 每 adapter 私有的 "control handle"，通过 adapter-scoped `ControlRegistry` 解藕；
- `AgentStatus.live_controls` 的计算改为 **读自 adapter.capabilities + handle 存在性**，而非 `runtime === "claude_code"`。

### 7.6 HTTP 契约

新增（与现有 `/runtime/catalog` 并列，读写分离）：

```
GET /runtime/capabilities               -> RuntimeCapabilityMatrix 全量 + adapter 索引
GET /runtime/capabilities/:adapter      -> 单 adapter capability
```

前端控件（中断 / 设模型 / 进入 plan mode 等）**必须**查询此 endpoint 决定是否渲染。

---

## 8. Gap 清单与优先级

### 8.1 Per-adapter Gap 表

每 adapter 一张表（8 张），由分能力节聚合：

```
Adapter: claude_code
Gap ID  | 能力锚点 | 类别   | 当前 | 目标 | P   | 估工 | 依赖   | Roadmap
CC-001  | §6.5.1   | Skills | ~    | full | P0 | M   | -      | D2
CC-002  | §6.6.3   | Sub    | ~    | full | P1 | L   | CC-001 | -
CC-003  | §6.8.3   | Hooks  | ~    | full | P2 | S   | -      | C
```

### 8.2 四维评分（不加权）

| 维度 | 取值 |
|---|---|
| `axis-1` 用户可见价值 | blocker / high / medium / low |
| `axis-2` 实现复杂度 | S (1-2 day) / M (3-5 day) / L (1-2 week) / XL (>2 week) |
| `axis-3` 风险 | low / medium / high（breaking / 涉及生产数据） |
| `axis-4` 契约稳定度 | stable / volatile（SDK 上游还在变） |

**不给总分**——项目内部测试阶段需要人判断，综合分会误导。

### 8.3 P0–P3 定义

| Label | 语义 | 选择条件 |
|---|---|---|
| **P0** | 必须下个 sprint 做 | `axis-1=blocker` 或 **诚信 gap**（当前 adapter 宣称支持但实为 stub） |
| **P1** | 下个里程碑 | `axis-1=high` 且 `axis-2 ≤ M` |
| **P2** | Backlog | `axis-1=medium` 或 `axis-2=L` |
| **P3** | 明确延后 | `axis-1=low` 或 `axis-4=volatile` |

**诚信 gap** 强制 P0：不允许 "矩阵里宣称支持但实际是 `throw new Error('TODO')`"。

### 8.4 Executive Summary

Spec 顶部（§1 后）给一张 **"Top-20 全局 gap"**——从 8 张表里按 P0 > P1 排序取前 20。

### 8.5 Gap → Plan 转换规则

- **一对一不强制**：多个相关 gap 可合并为一个 plan；
- **合并条件**：同一 adapter + 同一一级分组 + 累计工作量 ≤ XL → 允许合并；
- **跨 adapter 的同类 gap**（如 "所有 adapter 补 fork 的 message_id 支持"）**不得**合并——每 adapter 独立走 plan，防回归风险。

### 8.6 `gaps.yaml` Schema

```yaml
gaps:
  - id: CC-001
    adapter: claude_code
    capability: "6.5.1"
    axis:
      value: high
      complexity: M
      risk: low
      stability: stable
    priority: P0
    depends_on: []
    roadmap_track: D2
    hint_plan_merge_group: "claude_code:skills"
```

---

## 9. 调研纪律与 Review 流程

### 9.1 调研源表

| 项目 | 主源 | 辅源 | 工具 |
|---|---|---|---|
| Claude CLI | 官方 docs + claude-code GitHub | `claude --help` | fetch / deepwiki |
| Claude SDK | `@anthropic-ai/claude-agent-sdk` types.d.ts + 示例 | changelog | context7 |
| Cursor | Cursor docs + Release notes | GitHub discussions | fetch |
| Aider | GitHub README + docs | — | deepwiki |
| Codex | OpenAI Codex CLI docs + source | — | fetch / deepwiki |
| OpenCode | OpenCode docs + source | — | deepwiki |
| Gemini CLI | GitHub + Gemini CLI docs | — | fetch / deepwiki |

### 9.2 调研纪律

1. **禁止凭记忆写对标证据**——每条 `✓`/`~` 必有真实引用；
2. **使用 MCP**：`mcp__deepwiki__ask_question` 用于快速扫 GitHub repo；`mcp__context7__query-docs` 用于查 SDK 版本接口；`mcp__fetch__fetch` 用于抓 docs；
3. **证据快照日**：写作期间定一天（建议 `2026-04-18`）为快照日；之后上游更新只在 §8 "风险/开放问题" 提及，不改既有单元格——防追活靶子；
4. **网页归档**：URL 脚注同时记录 `web.archive.org` 镜像 或 截图文件名（存入附件目录 `sources/screenshots/`）。

### 9.3 Review Checklist

spec-document-reviewer subagent 的 review 必须显式覆盖：

1. 矩阵是否 **无 `?`** 单元格？
2. 分能力节是否 **八小节齐全**？
3. 对标证据是否有 **URL/文件行号**？
4. Gap 是否 **可行动粒度**（估工量 ≤ XL）？
5. 每 gap 是否 **覆盖回归测试策略**？
6. 与 Plugin Roadmap 6 track 的 **交叉引用完整**？
7. v2 接口是否 **未引入 claude_code-only 类型** 到公共类型层？
8. CSV / YAML 附件是否与主文档一致？

最多 3 轮 review 循环；第 3 轮未过则上报用户决策。

### 9.4 User Review Gate

spec 写完并通过内部 review 后，告知用户 "spec 已写入 `<path>`，请 review 后再进入 writing-plans 阶段"。用户要求改动 → 改完重跑 review 循环。

---

## 10. 交付与时序

### 10.1 文件结构

```
docs/superpowers/specs/
├── 2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md   ← 主 spec
└── 2026-04-16-runtime-capability-matrix-and-adapter-v2/             ← 附件目录
    ├── matrix.csv              机器可读矩阵
    ├── gaps.yaml               Gap 清单（8.6 schema）
    ├── sources.md              所有对标证据脚注来源表
    └── sources/screenshots/    网页归档截图（调研纪律）
```

### 10.2 与现有工作的时序

| 工作 | 关系 | 时序约束 |
|---|---|---|
| Event Bus（已 approved） | 独立 | 不阻塞 |
| Feishu Phase 1 | 独立 | 不阻塞 |
| Plugin Roadmap Track A (NodeTypeRegistry) | 无直接耦合 | 可并行 |
| Plugin Roadmap Track B (FunctionRegistry) | 无直接耦合 | 可并行 |
| Plugin Roadmap Track C (Hook 系统) | **强耦合** §4.8 / §6.8.* | **阻塞**——C 的 brainstorm/plan 必须在本 spec 定稿后 |
| Plugin Roadmap Track D1 (前端扩展槽) | 弱耦合 §4.14 / §7.6 | 可并行，注意 HTTP 契约对齐 |
| Plugin Roadmap Track D2 (Skill Registry) | **强耦合** §4.5 / §6.5.* | **阻塞**——D2 的 brainstorm/plan 必须在本 spec 定稿后 |
| Plugin Roadmap Track D3 (脚手架 CLI) | 参考本 spec 的 v2 模板生成 | 等 v2 稳定 |

### 10.3 后续 Plan 生成

本 spec 定稿后：
1. 读取 `gaps.yaml`；
2. 按 `hint_plan_merge_group` 聚合；
3. 每 merge group 作为 `writing-plans` 的一次输入，产出一份 plan；
4. **不要求一次生成全部 plans**——按 P0 → P1 顺序逐个产出、实施、合入。

### 10.4 非功能性约束

- **字数上限**：主 spec 正文 ≤ 25,000 字；附件不限；
- **结构稳定性**：§1–§12 的目录结构冻结，便于 review 对照；
- **语言**：中文为主；接口签名/代码 / 脚注前缀为英文。

---

## 11. 与 Plugin Extensibility Roadmap 的关系

复述 §10.2 并额外澄清边界：

- 本 spec **不是 Roadmap 的 Track**——它是跨 Track 的**审计与契约层**；
- Roadmap 里 "track C / D2" 的 brainstorm 在本 spec 定稿后再启动，届时参照本 spec 的 §4.5（Skills）/ §4.8（Hooks）章节作为能力定义源；
- 本 spec 产生的 v2 接口变更**不计入** Roadmap 进度表——是独立的一次性改造；
- 本 spec 产生的 gap 修复 plan **会部分归入** Roadmap（如 hook 相关 gap 在 Track C 的 plan 里一并实施），规则为：
  - Gap 的 `roadmap_track` 字段非空 → 由该 track 承接；
  - `roadmap_track` 为空 → 作为独立 plan 实施。

---

## 12. 附件占位

本次 brainstorm 仅定义**框架**。下列附件的实际内容由后续 `writing-plans` 产生的第一份 plan 负责填充（"对标项目能力调研 + 矩阵填充"）：

- `matrix.csv` — 模板头行已确定（13 列：`row_key` / `capability` / 对标 7 列 / AgentForge 3 列 / `anchor`，详见 §5.5）；行内容待填；
- `gaps.yaml` — schema 已定（§8.6）；entries 待填；
- `sources.md` — 表头已定（`[prefix-n]` 编号 / URL / 访问日期 / 归档位置）；内容待填；
- `sources/screenshots/` — 空目录；调研期间入库。

---

## 13. 开放问题

在 brainstorm 阶段未解决、留给 spec 填充阶段或后续 plan 决策的事项：

1. **Output Styles 分组（§4.14）是否独立保留**？Claude CLI 有明确的 output styles，其他对标项目以 "rules / system prompt" 形式提供——可能合并到 §4.4 (Memory)，留待矩阵填充时观察行数是否足够支撑独立分组。
2. **`command` runtime（通用 CLI）的能力列**是否单列？当前它与具名 adapter 的关系是 "兜底"，可能不参与对齐，仅作 launch profile 入口。
3. **MCP 服务能力（§4.12）是 adapter 的还是 bridge 的**？现有 `src-bridge/src/mcp/` 是全局 hub，不属某个 adapter。本 spec 如果把 "getMcpStatus" 作为 adapter 方法，意味着 claude_code 之外的 adapter 如何暴露 MCP 能力需要明确。
4. **Plan Mode 的"跨 adapter 抽象"**：Claude CLI 的 plan mode 是 permission_mode + tool gating 的组合；其他 adapter 可能没有直接对等物。是强制所有 adapter 实现 `enterPlanMode` 并抛 `UnsupportedOperationError`，还是作为 Claude-only 能力单列？
5. **Cost accounting 的统一**：§4.10 需要覆盖 OpenAI tokens / Anthropic tokens / cache tiers；如果某 adapter（如 command）没有原生 cost 反馈，如何填 §4.10？

上述问题在矩阵填充时必须定论，不得带入后续 plan。

---

## 14. 变更记录

| 日期 | 版本 | 作者 | 变更 |
|---|---|---|---|
| 2026-04-16 | draft-0 | Max Qian | 初稿（brainstorm 输出） |
| 2026-04-16 | draft-1 | Max Qian | Spec-review 修订：§1 adapter 计数 7→8；§5.5 CSV 13 列与 awk 扫描区间显式化、新增 row_key 唯一性校验；新增 §7.2.1 解释 Group 3/4 不占独立方法签名；§7.3 补 `/* ... */` 键集派生规则与 §13.1 依赖链；§12 对齐 13 列措辞 |
