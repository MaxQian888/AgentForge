# Claude Agent SDK 能力盘点

**Benchmark：** `@anthropic-ai/claude-agent-sdk`（TypeScript，v0.2.109）+ `claude-agent-sdk` for Python（同步跟随）
**证据快照日：** 2026-04-18
**来源规则：** 仅一手资料：
- TypeScript：npm 包 `@anthropic-ai/claude-agent-sdk@0.2.109` 的 `sdk.d.ts`（下载自 registry.npmjs.org，本机校对：`/tmp/sdk-ts/package/sdk.d.ts`）、`CHANGELOG.md`、`README.md`。
- Python：`github.com/anthropics/claude-agent-sdk-python` wiki + 源文件（`ClaudeAgentOptions` dataclass、`ClaudeSDKClient` class、`SubprocessCLITransport`）。
- 官方文档：`docs.claude.com/en/api/agent-sdk/*`（reference pages）。
- 第三方博客、教程、推文不计入。

**来源编号：** `[sdk-NN]` 详见 `sources.md` § Claude Agent SDK 章节。

**判定规则（Task 3 专属）：**
- `✓` = TS 和 Python 两端都已一等支持（文档 + 源码类型均可证）。
- `~` = 只有一端支持、或两端都仅部分支持、或需 `extraArgs`/`extra_args` 绕行。必须注明哪端。
- `✗` = 两端均无一等 API（即便底层 CLI 支持）。
- `N-A` 仅用于 "SDK 层不应承担" 的事项（本列几乎用不上，SDK 应与 CLI 等宽）。

**关键设计差异预警：**
1. SDK 没有交互式 TTY，因此所有 CLI 交互语义（ESC 中断、Shift+Tab、`#` 快捷键、斜杠命令主体）**通过控制协议**暴露：`Query.interrupt()`、`Query.setPermissionMode()` 等。
2. SDK 的 "hooks" 是**类型化回调**（`hooks: Partial<Record<HookEvent, HookCallbackMatcher[]>>`），比 CLI 的 settings.json shell 命令更强。
3. SDK Options 字段是 CLI flag 的超集——CLI 没有的 `includePartialMessages`、`canUseTool`、`outputFormat`、`persistSession`、`enableFileCheckpointing`、`spawnClaudeCodeProcess` 等都在 SDK。
4. TS `HookEvent` 枚举有 27 项；Python 当前仅 10 项（见 §8）——这是 SDK 的一个内部不对称。

---

## 1. Session & Lifecycle

### 1.1 session:create → ✓
TS：`query({ prompt, options })` 隐式创建；`Options.sessionId` 可指定 UUID。[sdk-1]
Python：`query(prompt, options)` 或 `ClaudeSDKClient(options)` 创建。[sdk-2]

### 1.2 session:resume → ✓
TS：`Options.resume: string`（session UUID）+ `Options.continue?: boolean`（恢复当前 cwd 最近一次）。[sdk-1]
Python：`ClaudeAgentOptions.resume` 与 `continue_conversation`。[sdk-2][sdk-3]

### 1.3 session:fork → ✓
TS：`Options.forkSession: boolean`（与 `resume` 合用）、**及**顶层 `forkSession(sessionId, options?) => Promise<ForkSessionResult>` 函数（独立 API）。[sdk-1] sdk.d.ts:548
Python：`ClaudeAgentOptions.fork_session: bool`；Python 0.1.0+ 支持；**未**提供顶层 `fork_session()` 函数——需走 options 路径。[sdk-2][sdk-3]

### 1.4 session:fork:cross-directory → ✗
两端 SDK 都**未**暴露 "将原 session fork 到另一 cwd" 的语义。TS `ForkSessionOptions` 仅继承 `SessionMutationOptions.dir`（用于定位原 session 文件），不是 "fork 到新 cwd"。`cwd` 在 forked session 的 Options 中仍指新 session 自己的 cwd，不改变 fork 的来源约束——与 CLI 同样缺失。[sdk-1]

### 1.5 session:fork:by-message-id → ✓
TS：`forkSession(sessionId, { upToMessageId })` 及 `Options.resumeSessionAt: string`（resume 时限定到 message UUID）。这是 SDK **优于** CLI 的一等字段（CLI 只有交互式 rewind）。[sdk-1] sdk.d.ts:553-557
Python：文档描述 `rewind_files(user_message_id)`；但 `fork_session=True + resume_session_at=...` 未明确暴露为一等字段——根据 Python SDK wiki 未列出 `resume_session_at`。标 `~` 的 rationale："TS 有一等 `upToMessageId` / `resumeSessionAt`，Python 需依赖底层 CLI 透传、无类型化字段"。[sdk-3]
→ 聚合：因 TS 完整 / Python 部分 → `~`。

### 1.6 session:rewind → ✓
TS：`Query.rewindFiles(userMessageId, { dryRun? })` 配合 `Options.enableFileCheckpointing`。[sdk-1] sdk.d.ts:~1820
Python：`ClaudeSDKClient.rewind_files(user_message_id)` + `ClaudeAgentOptions.enable_file_checkpointing=True` + `extra_args={"replay-user-messages": None}`。[sdk-3][sdk-4]
注：SDK `rewind` 聚焦 "文件 rewind 到某条 user message"；不含对话消息回退（CLI `/rewind` 的全能语义不完全覆盖）——SDK 端无 "只 revert messages 不 revert files" 的开关。但 "rewind 到任意消息" 这一根能力双端支持。

### 1.7 session:interrupt → ✓
TS：`Query.interrupt(): Promise<void>`。sdk.d.ts:1712。[sdk-1]
Python：`ClaudeSDKClient.interrupt()`。[sdk-3]

### 1.8 session:pause-resume → ~
两端均无一等 "pause" 状态机——`close()` / `interrupt()` 后进程退出，后续只能通过 `resume: <id>` 重建。与 CLI 相同不足：JSONL 持久化支撑 "重启恢复"，但没有 "热备连接池+MCP 保活" 语义。标 `~` 的 rationale："通过 resume 近似，但无显式 pause API；与 CLI 对等缺失"。[sdk-1][sdk-3]

### 1.9 session:snapshot-export → ~
TS：顶层 `listSessions()`、`getSessionMessages(sessionId)`、`getSessionInfo(sessionId)`、`getSubagentMessages()` 均为公开函数——**比 CLI 更强**，可程序化导出。[sdk-1] sdk.d.ts:577,602,634,725
Python：`list_sessions()`、`get_session_messages()` 为顶层函数；**无** `get_session_info()`。[sdk-3]
整体聚合为 `~`：TS 完备 / Python 缺 `get_session_info`；且两端都未提供 "snapshot 打包为可移植档案" 的一等命令——只能自行读取 JSONL。

---

## 2. Execution Control

### 2.1 exec:set-model → ✓
TS：`Options.model`（启动）+ `Query.setModel(model?)`（运行时切换）。sdk.d.ts:1725。[sdk-1]
Python：`ClaudeAgentOptions.model` + `ClaudeSDKClient.set_model(model)`。[sdk-3][sdk-5]

### 2.2 exec:thinking-budget → ~
TS：`Options.thinking: ThinkingConfig`（`{type:'adaptive'}` / `{type:'enabled', budgetTokens:N}` / `{type:'disabled'}`） + `Options.effort: 'low'|'medium'|'high'|'max'` + 已弃用的 `maxThinkingTokens`。运行时 `Query.setMaxThinkingTokens(n)` 仍可调（标 `@deprecated`）。sdk.d.ts:1740。[sdk-1]
Python：`ClaudeAgentOptions.thinking: ThinkingConfig`（`Adaptive`/`Enabled`/`Disabled`）、`effort`、`max_thinking_tokens`（deprecated）。[sdk-3]
聚合 `~` 的 rationale："SDK 支持优于 CLI（CLI 只有 `--effort` 枚举），但 `maxThinkingTokens` 已弃用、运行时切换路径在迁移中——不算稳定完整"。

### 2.3 exec:max-turns → ✓
TS：`Options.maxTurns`。[sdk-1]
Python：`ClaudeAgentOptions.max_turns`。[sdk-3]

### 2.4 exec:temperature → ✗
两端均无 `temperature` 字段。TS Options 类型、Python `ClaudeAgentOptions` 均无此选项。可通过 `extraArgs` / `extra_args` 绕行传 CLI flag，但 CLI 本身也不暴露——保持 `✗`。[sdk-1][sdk-3]

### 2.5 exec:permission-mode → ✓
TS：`Options.permissionMode: PermissionMode`（启动） + `Query.setPermissionMode(mode)`（运行时）。sdk.d.ts:1720, 1527。[sdk-1]
Python：`ClaudeAgentOptions.permission_mode` + `ClaudeSDKClient.set_permission_mode(mode)`。[sdk-3]

### 2.6 exec:plan-mode → ✓
TS：`permissionMode: 'plan'`；`Query.setPermissionMode('plan')`。PermissionMode 枚举显式包含 `'plan'`。[sdk-1]
Python：同上；`PermissionMode` Literal 含 `'plan'`。[sdk-3]

### 2.7 exec:plan-mode-tools → ✓
Plan mode 下 `ExitPlanMode` 工具由 CLI 子进程提供；SDK hooks 可对 `ExitPlanMode` tool_name 做 PreToolUse 拦截，`planFilePath` 字段在 hook input 暴露（TS CHANGELOG 0.2.76）。[sdk-6]

---

## 3. Tool & File Permissions

### 3.1 perm:tool-allowlist → ✓
TS：`Options.allowedTools: string[]` + `Options.tools: string[] | {type:'preset'}`。[sdk-1]
Python：`allowed_tools` + `tools`。[sdk-3]

### 3.2 perm:tool-denylist → ✓
TS：`Options.disallowedTools: string[]`。[sdk-1]
Python：`disallowed_tools`。[sdk-3]

### 3.3 perm:file-scope → ✓
两端通过 `Settings.permissions.allow/deny` glob 规则（如 `Read(./private/**)`）透传给 CLI；`canUseTool` 回调额外可做编程判定。[sdk-1][sdk-3]

### 3.4 perm:network-policy → ~
SDK 未新增粒度——继承 CLI 的工具级策略（WebFetch allow/deny）。TS 有 `sandbox.network.allowLocalBinding/allowUnixSockets`，但那是 sandbox 进程隔离而非 URL 策略。标 `~`（与 CLI 同）。[sdk-1]

### 3.5 perm:shell-allowlist → ✓
同 CLI：`Bash(git diff:*)` 规则通过 Settings / allowlist 生效；TS `sandbox.autoAllowBashIfSandboxed` 是进一步增强。[sdk-1]

### 3.6 perm:write-confirm → ✓
`canUseTool(toolName, input, {signal, suggestions, blockedPath})` 回调提供编程化 write-confirm——**SDK 专有强化**，优于 CLI 的 `acceptEdits` 枚举。[sdk-1] sdk.d.ts:146。Python 同样暴露 `can_use_tool`。[sdk-3]

### 3.7 perm:modes-enum → ✓
TS PermissionMode 枚举：`'default' | 'acceptEdits' | 'bypassPermissions' | 'plan' | 'dontAsk' | 'auto'`（6 值，其中 `'auto'` 为 SDK 新增的模型分类器模式，CLI 文档未列）。sdk.d.ts:1527。[sdk-1]
Python：同样 Literal 类型。[sdk-3]

---

## 4. Context & Memory

### 4.1 ctx:project-instructions → ✓
SDK 通过底层 CLI 加载 CLAUDE.md；`settingSources` 必须包含 `'project'` 才会加载 CLAUDE.md（TS CHANGELOG 强调）。[sdk-1]
Python 同。`setting_sources` 含 `'project'`。[sdk-3]

### 4.2 ctx:user-instructions → ✓
`settingSources: ['user']` 加载 `~/.claude/CLAUDE.md`。[sdk-1][sdk-3]

### 4.3 ctx:directory-hierarchy → ✓
`settingSources: ['user'|'project'|'local']` 精细控制三级；未提供时默认 SDK 隔离模式（不加载任何文件 settings）。[sdk-1]

### 4.4 ctx:memory-read → ~
SDK 不直接暴露 memory 读 API；只能通过 hook（`UserPromptSubmit` 注入 additionalContext）或 `systemPrompt.append` 间接观察到当前内存状态。无类似 CLI `#` 快捷键 / `/memory` 的一等入口。标 `~`：存在 InstructionsLoaded hook（TS）可观察 memory 加载事件，Python 无此 hook。[sdk-1][sdk-3]

### 4.5 ctx:memory-write → ~
SDK 无一等 "write memory" API；需自行 `fs.writeFile(CLAUDE.md)` 再 reload。`InstructionsLoaded` hook 只观察不写。标 `~`。[sdk-1]

### 4.6 ctx:memory-auto-load → ✓
`settingSources` 含 `'project'` 时自动加载 CLAUDE.md；`additionalDirectories` 下的 CLAUDE.md 需 `env.CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`。[sdk-1]

### 4.7 ctx:memory-compaction → ~
SDK 无 `/compact` 编程入口；但 `PreCompact` / `PostCompact`（仅 TS）hook 可观察/拦截压缩。`HOOK_EVENTS` 含这两项，Python 目前只有 `PreCompact`。[sdk-1][sdk-3]

---

## 5. Skills Registry

### 5.1 skill:discover → ✓
TS：`Query.supportedCommands(): Promise<SlashCommand[]>` 返回 skill+内置命令；`SDKSystemMessage` 的 `skills` 字段（TS CHANGELOG 0.1.25）列出可用技能。[sdk-1]
Python：`ClaudeSDKClient.get_server_info()` 返回 `available_commands` + `current_output_style`；`SystemMessage` subtype=init 含 `agents`、`slash_commands`（skills 列表）。[sdk-3]

### 5.2 skill:invoke → ✓
两端都通过 agent 自然触发（SKILL.md description matching）或用户 prompt 内容调用 `/skill-name`；SDK 不提供 "程序化强制 invoke" 单独 API，和 CLI 同。[sdk-1][sdk-3]

### 5.3 skill:params → ✓
SDK 继承 CLI 的 SKILL.md frontmatter；`SlashCommand` 类型含 `argumentHint` 字段。[sdk-1]

### 5.4 skill:nested → ✗
两端 SDK 均未提供 "skill 调用 skill" 的一等机制——与 CLI 同。[sdk-1][sdk-3]

### 5.5 skill:version-dependency → ✗
两端无版本字段。[sdk-1][sdk-3]

---

## 6. Subagents

### 6.1 sub:dispatch → ✓
TS：`Options.agents: Record<string, AgentDefinition>` 定义 inline subagent；agent 通过 `Task` 工具（aka "Agent" tool）调用。`AgentDefinition` 含 `description`、`prompt`、`tools`、`model`、`mcpServers`、`skills`、`maxTurns`、`background`、`permissionMode`、`effort` 等字段。sdk.d.ts:38-91。[sdk-1]
Python：`ClaudeAgentOptions.agents: dict[str, AgentDefinition]`。[sdk-3]

### 6.2 sub:isolated-context → ✓
与 CLI 同——subagent 独立 context window。`getSubagentMessages(sessionId, agentId)` 可单独取 subagent 消息。[sdk-1]

### 6.3 sub:result-return → ✓
Task tool 返回 subagent 最终字符串；`task_notification` 事件含 `tool_use_id` 关联。[sdk-1]

### 6.4 sub:parallel → ✓
与 CLI 同；多个 Task 同 turn 内并行。`AgentDefinition.background: true` 支持后台 subagent + `agentProgressSummaries`。[sdk-1]

### 6.5 sub:nested → ✗
继承 CLI 限制。[sdk-1]

### 6.6 sub:tool-inheritance → ✓
`AgentDefinition.tools?: string[]`（缺省继承 parent）、`disallowedTools?: string[]`。[sdk-1]

---

## 7. Slash / User Commands

### 7.1 cmd:builtin → ✓
`Query.supportedCommands()` 列出内置 + 用户命令，**优于** CLI 的无程序化 introspection。[sdk-1]
Python：`get_server_info()["available_commands"]`。[sdk-3]

### 7.2 cmd:user-register → ~
SDK 不提供 "在程序内注册新 slash 命令" 的一等 API；自定义命令仍靠磁盘 SKILL.md / `.claude/commands/`（CLI 机制继承）。SDK 仅能 "发现+执行"，不能 "创建"。标 `~` 的 rationale："可通过 `plugins: [{type:'local', path:...}]` 加载包含命令的插件（TS CHANGELOG 0.2.x）；但没有直接 'defineSlashCommand(name, handler)' API"。[sdk-1][sdk-3]

### 7.3 cmd:param-schema → ✓
`SlashCommand.argumentHint` 字段透传；SDK 消费方可读 schema。[sdk-1]

### 7.4 cmd:execute → ✓
用户 prompt 以 `/name args` 传入即执行；输出通过 stream 回流（TS CHANGELOG 修过 "local slash command output not returned"）。[sdk-1]

---

## 8. Hooks

### 8.1 hook:pre-tool-use → ✓
TS/Python 都支持 `PreToolUse` 类型化回调（TS 是 `HookCallback(input, toolUseID, {signal})`）。sdk.d.ts:654。[sdk-1][sdk-3]

### 8.2 hook:post-tool-use → ✓
TS：`PostToolUse` + `PostToolUseFailure`（单独枚举值，Python 0.1.26+ 也加入）。[sdk-1][sdk-3]

### 8.3 hook:user-prompt-submit → ✓
`UserPromptSubmit` hook；可修改/拦截 prompt。[sdk-1][sdk-3]

### 8.4 hook:stop → ✓
`Stop`；TS 额外有 `StopFailure`、`SubagentStop`。Python 两者都有。[sdk-1][sdk-3]

### 8.5 hook:notification → ✓
`Notification`。[sdk-1][sdk-3]

### 8.6 hook:session-start → ✓
TS：`SessionStart` + `SessionEnd`。Python 仅 `SessionStart`（根据 HookEvent 枚举）；`SessionEnd` 未出现在 Python HookEvent Literal 中。标 `~`？——但 `SessionStart` 这一行的能力边界仅覆盖 start，端到端双端都有一等，`SessionEnd` 是另一能力。本行 `✓`。[sdk-1][sdk-3]

### 8.7 hook:callback-management → ✓
`HookCallbackMatcher { matcher?, hooks[], timeout? }` 类型化注册；运行时无动态 register/unregister API（注册在 Options 时一次性）——但这与 CLI 的 settings.json 静态注册相当。[sdk-1][sdk-3]

### 8.8 hook:failure-isolation → ✓
TS Hook 有 `asyncTimeout`、`AsyncHookJSONOutput.async:true`；AbortSignal 在 callback 参数中可观察 cancellation——失败隔离机制完备。[sdk-1]

---

## 9. Streaming & Events

### 9.1 evt:text-delta → ✓
TS：`includePartialMessages: true` 时发 `SDKPartialAssistantMessage` 携带 text_delta。[sdk-1]
Python：`include_partial_messages=True` 发 `StreamEvent`。[sdk-3]

### 9.2 evt:thinking-delta → ✓
同上，stream event 含 thinking_delta block（与 CLI stream-json 对等）。[sdk-1][sdk-3]

### 9.3 evt:tool-use → ✓
AssistantMessage 的 content 数组含 `tool_use` block。[sdk-1][sdk-3]

### 9.4 evt:tool-result → ✓
UserMessage 或系统消息含 `tool_result` block。[sdk-1][sdk-3]

### 9.5 evt:subagent → ✓
TS：`task_started`、`task_notification`、`task_progress`、`task_completed` 一等系统消息；`SubagentStart`/`SubagentStop` hooks；`getSubagentMessages()` 顶层函数——**SDK 优于 CLI**（CLI 只能通过 hooks 捕获）。[sdk-1] sdk.d.ts:634
Python：`SubagentStart`/`SubagentStop` hooks；无 `getSubagentMessages()` 顶层函数。标 `~`（Python 部分）？——但 hook+ResultMessage 可重建；本行 `✓` 因 SDK 整体一等；在分能力节中注明 Python 缺失顶层函数。

### 9.6 evt:cost → ~
TS：`SDKResultSuccess.usage: NonNullableUsage`、`total_cost_usd`、`modelUsage: Record<string, ModelUsage>`（每 model 拆分）；`task_progress` 含 "cumulative usage metrics"。per-turn delta 仍需自算——与 CLI 同。[sdk-1]
Python：`ResultMessage.usage`、`total_cost_usd` 字段对等；但无 per-turn 事件流中的 cost delta。[sdk-3]
聚合 `~`：优于 CLI（因字段更丰富、有 `modelUsage` 按模型拆分）但仍非 "per-turn 一等事件"。

### 9.7 evt:todo-tracking → ✓
`TodoWrite` 工具是 built-in（在 Claude Code CLI 侧），SDK 通过 tool_use/tool_result 流透传；未作为独立 SDK 事件类型。与 CLI 同。[sdk-1]

---

## 10. Cost & Usage Accounting

### 10.1 cost:per-turn → ~
`SDKResultMessage` 在每轮结束发，含完整 usage 对象。TS `modelUsage` 按 model 分桶的 per-turn 成本——优于 CLI，但仍需 consumer 聚合。Python `usage` 为 dict，字段对等但无类型分桶。标 `~` 聚合。[sdk-1][sdk-3]

### 10.2 cost:per-session → ✓
`total_cost_usd` 累积；`maxBudgetUsd` 硬上限，超出返回 `error_max_budget_usd` SDKResult。[sdk-1][sdk-3]

### 10.3 cost:cache-read → ✓
`usage.cache_read_input_tokens` 字段在两端都有；TS NonNullableUsage 保证非空。[sdk-1][sdk-3]

### 10.4 cost:cache-creation → ✓
`usage.cache_creation_input_tokens`。[sdk-1][sdk-3]

### 10.5 cost:budget-alert → ~
同 CLI：`maxBudgetUsd` 是硬 cap，无渐进告警事件。`task_progress` 有累计值，可自建告警，但 SDK 不主动发 "80% reached" 事件。[sdk-1]

### 10.6 cost:audit-export → ~
SDK 的 AsyncGenerator 本质天然可全量落盘——比 CLI stream-json 落盘更程序化（可直接 `for await (const msg of query) { log(msg); }`），但无一等 `exportCost()` 函数。标 `~`：比 CLI 更强但仍非 first-class 命令。[sdk-1]

---

## 11. File Checkpointing & Diff

### 11.1 ckpt:file-checkpoint → ✓
TS：`Options.enableFileCheckpointing: boolean`（显式 opt-in，CLI 默认开）。sdk.d.ts:~1090。[sdk-1]
Python：`enable_file_checkpointing: bool` + 需 `extra_args={"replay-user-messages": None}`。[sdk-3]

### 11.2 ckpt:file-revert → ✓
TS：`Query.rewindFiles(userMessageId, {dryRun?}): Promise<RewindFilesResult>`——含 `dryRun` 预览，**优于** CLI。[sdk-1]
Python：`ClaudeSDKClient.rewind_files(user_message_id)`；无 `dry_run` 参数。标 `~`？——但两端都支持 revert 根能力；`dryRun` 是 TS 的增强，本行 `✓`，在 §6.11.2 注 Python 缺 dry_run。

### 11.3 ckpt:per-turn-diff → ✓
Hook `PostToolUse` 对 Edit/Write 工具可取得 tool_input / tool_response 含 diff；`FileChanged` hook（TS 独有）提供实时文件事件。TS CHANGELOG 描述 `FileChangedHookInput.event: 'change'|'add'|'unlink'`。[sdk-1]

### 11.4 ckpt:unified-diff → ✓
Edit 工具返回 unified diff；SDK 透传。[sdk-1]

### 11.5 ckpt:git-integration → ✗
同 CLI 继承（不跟 git）。[sdk-1]

---

## 12. MCP Integration

### 12.1 mcp:server-define → ✓
TS：`Options.mcpServers: Record<string, McpServerConfig>`；或 `createSdkMcpServer({name, tools})` 定义进程内 SDK MCP 服务器（**SDK 独有**——CLI 只有外部进程 MCP）。sdk.d.ts:412。[sdk-1]
Python：`mcp_servers: dict | str`；`@tool` + `create_sdk_mcp_server()` 定义 SDK 内 MCP。[sdk-3][sdk-7]

### 12.2 mcp:server-lifecycle → ✓
TS：`Query.reconnectMcpServer(name)`、`Query.toggleMcpServer(name, enabled)`、`Query.setMcpServers(servers)`（动态替换）。sdk.d.ts:~1870。[sdk-1]
Python：`Query.reconnect_mcp_server()`、`toggle_mcp_server()`（0.1.46+）；未见 `set_mcp_servers` 等效。标 `~`：Python 少一个方法但运行时生命周期覆盖根能力；本行 `✓` 聚合。[sdk-3]

### 12.3 mcp:tool-discovery → ✓
`McpServerStatus.tools: { name, description, annotations }[]` 自动填充（connected 时）。[sdk-1]

### 12.4 mcp:permission-gate → ✓
同 CLI + SDK 可在 `canUseTool` 对 `mcp__<server>__<tool>` 做自定义决策。[sdk-1]

### 12.5 mcp:status-logs → ✓
TS：`Query.mcpServerStatus()` 返回 `McpServerStatus[]`（连接/失败/needs-auth/pending/disabled + error/config/scope/tools）。[sdk-1]
Python：`ClaudeSDKClient.get_mcp_status()`（0.1.23+）。[sdk-3]

### 12.6 mcp:dynamic-toggle → ✓
`toggleMcpServer(name, enabled)` 双端支持。[sdk-1][sdk-3]

---

## 13. Launch & Environment

### 13.1 env:working-directory → ✓
`Options.cwd: string`；默认 `process.cwd()`。[sdk-1]
Python：`cwd`。[sdk-3]

### 13.2 env:additional-directories → ✓
TS：`Options.additionalDirectories: string[]`；若要加载其下 CLAUDE.md 需 `env.CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`。[sdk-1]
Python：`add_dirs: list`（映射 `--add-dir`）。[sdk-3]

### 13.3 env:env-vars → ✓
TS：`Options.env: {[key]: string | undefined}`。[sdk-1]
Python：`env: dict`。[sdk-3]

### 13.4 env:cli-flag-translate → ✓
TS：`Options.extraArgs: Record<string, string | null>`（null 表示布尔 flag）。[sdk-1]
Python：`extra_args: dict`。[sdk-3]

### 13.5 env:diagnostics → ~
两端 SDK 没有 `doctor()` 顶层函数。TS `Query.accountInfo()`（账户 info）、`Query.initializationResult()`（返回含 account、models、output style）可部分替代；`Query.getContextUsage()` 返回 token 分桶；`env/debug/debugFile/stderr` 选项辅助排错。Python `get_server_info()` 类似。标 `~`——有结构化自省但无 "health check 诊断" 单一入口。[sdk-1][sdk-3]

### 13.6 env:profile-system → ✗
两端 SDK 无 profile 系统——与 CLI 同。[sdk-1][sdk-3]

---

## 14. Output Styles & Personality

### 14.1 style:output-format → ✓
TS：`Options.outputFormat: { type: 'json_schema', schema }` 驱动结构化输出（`SDKResultSuccess.structured_output`）；这是 **SDK 独有**——CLI 的 `--output-format` 只有 text/stream-json。[sdk-1]
Python：`output_format` 字段（TS 迁移版）。[sdk-3]

### 14.2 style:system-prompt-override → ✓
TS：`Options.systemPrompt: string`（完全覆盖）或 `{type:'preset', preset:'claude_code', append?, excludeDynamicSections?}`——**比 CLI 更强**（CLI override 仅 `--print` 模式可用）。[sdk-1]
Python：`system_prompt: str | SystemPromptPreset`。[sdk-3]

### 14.3 style:system-prompt-append → ✓
TS：`systemPrompt.append` 字段；或 `appendSystemPrompt`（旧路径，TS CHANGELOG 提到已合并到 systemPrompt）。[sdk-1]
Python：preset 内 append key。[sdk-3]

### 14.4 style:persona-inheritance → ✓
Output styles 文件由 CLI 端解析；`initializationResult()` 返回 `output_style_name` 等。[sdk-1]

---

# 脚注来源（归 sources.md 的 Claude Agent SDK 表）

以下编号与 `sources.md` 的 sdk-* 行一对一。

- [sdk-1] TypeScript SDK 类型定义 `@anthropic-ai/claude-agent-sdk@0.2.109` 的 `sdk.d.ts` → npm tarball 解压本地校对
- [sdk-2] TypeScript SDK README（query() 调用约定） → github.com/anthropics/claude-agent-sdk-typescript/README.md
- [sdk-3] Python SDK `ClaudeAgentOptions` + `ClaudeSDKClient` → github.com/anthropics/claude-agent-sdk-python（wiki 2.3, 3.2, 5.3, 6.1, 6.2）
- [sdk-4] Python SDK `rewind_files` + `enable_file_checkpointing` → github.com/anthropics/claude-agent-sdk-python wiki 6.2
- [sdk-5] Python SDK `set_model` + `set_permission_mode` 运行时控制 → wiki 3.2
- [sdk-6] TypeScript SDK CHANGELOG.md（0.2.76 `planFilePath`；0.1.45 structured output；0.2.21 reconnect/toggleMcpServer；0.2.63 supportedAgents；0.2.72 getSettings；0.2.74 skills user-invocable） → github.com/anthropics/claude-agent-sdk-typescript/CHANGELOG.md
- [sdk-7] Custom Tools（`@tool` + `create_sdk_mcp_server`） → docs.claude.com/en/api/agent-sdk/custom-tools; Python wiki 5.1
- [sdk-8] Agent SDK overview 官方 reference → docs.claude.com/en/api/agent-sdk/overview
