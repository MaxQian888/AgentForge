# Claude CLI 能力盘点

**Benchmark：** Claude Code CLI（即 `claude` TTY 工具，不含 Agent SDK）
**证据快照日：** 2026-04-18
**来源规则：** 仅一手官方资料 (`code.claude.com/docs/en/*`, `github.com/anthropics/claude-code`, Anthropic 官方博客/changelog)。第三方博客、推文、社区教程一律不计入。
**来源编号：** `[cc-NN]` 详见 `sources.md` § Claude CLI 章节。

---

## 1. Session & Lifecycle

### 1.1 session:create → ✓
新 session 通过 `claude` 命令无 `--resume/--continue` 直接启动。CLI 首次运行即落盘 JSONL transcript 至 `~/.claude/projects/<encoded-cwd>/<session-uuid>.jsonl`。[cc-1][cc-2]

### 1.2 session:resume → ✓
`claude --resume` 弹出交互式选择器；`claude --resume <session-id>` 直接恢复；`claude --continue` 恢复当前 cwd 下最近一次 session。[cc-1][cc-2]

### 1.3 session:fork → ✓
`claude --resume <id> --fork-session` 基于原 session 分叉出新 session id；原 session 保持不变。[cc-2]

### 1.4 session:fork:cross-directory → ✗
文档未记录跨目录 fork；`--fork-session` 仅描述同 cwd 分叉，`--add-dir` 属运行期附加目录而非 fork 语义。未见一手证据支持"将原 session fork 到另一个工作目录"。[cc-2]

### 1.5 session:fork:by-message-id → ~
通过 `/rewind` 或 ESC+ESC 可回到任意消息锚点并继续，实际等价于按 message id fork。CLI 未直接暴露 `--fork-at-message`，但文档描述 rewind 支持跳到指定消息。[cc-3]

### 1.6 session:rewind → ✓
ESC+ESC 触发 rewind；`/rewind` 斜杠命令亦可；支持单独回退消息、单独回退文件、或两者同回。[cc-3][cc-4]

### 1.7 session:interrupt → ✓
交互模式下 ESC 中断当前 turn；文档明确记录 "press Escape at any time to interrupt"。[cc-5]

### 1.8 session:pause-resume → ~
Session 状态持久化到 JSONL，CLI 进程可随时退出并 `--resume`。但没有一等的 "pause" 状态机（例如锁定后台进程、保留 MCP 服务器连接活跃）——关闭 CLI 即释放所有 runtime 资源，重启通过 `--resume` 重建。[cc-1][cc-2]

### 1.9 session:snapshot-export → ~
JSONL transcript 路径可读（`transcript_path` 在 hooks 输入里暴露），技术上可作为快照导出。但 CLI 无一等的 `claude export` / `claude snapshot` 子命令。[cc-6]

---

## 2. Execution Control

### 2.1 exec:set-model → ✓
`claude --model <alias|full-id>` 启动时设置；`/model` 斜杠命令运行时切换。[cc-7][cc-8]

### 2.2 exec:thinking-budget → ~
`--effort` 标志或 `/effort` 接受 `low|medium|high`（枚举而非 token 数字），映射到 extended thinking 的档位。无法传任意 token 数。[cc-7]

### 2.3 exec:max-turns → ✓
`--max-turns <N>` 标志；`--max-budget-usd` 为成本维度的等效限制。[cc-7]

### 2.4 exec:temperature → ✗
CLI 未暴露 temperature 标志；Claude Code 文档、`--help` 输出均无 temperature 选项。

### 2.5 exec:permission-mode → ✓
`--permission-mode <mode>` 启动时设置；`/permissions` 运行时切换。[cc-7][cc-9]

### 2.6 exec:plan-mode → ✓
`--permission-mode plan` 进入计划模式；Shift+Tab 或 `/permissions` 运行时切换入/出。[cc-7][cc-9]

### 2.7 exec:plan-mode-tools → ✓
Plan 模式下 `ExitPlanMode` 作为工具暴露；agent 自己调用该工具即可退出。[cc-9]

---

## 3. Tool & File Permissions

### 3.1 perm:tool-allowlist → ✓
settings.json 或 `--allowedTools`（通过 `--tools` 合并） + `/permissions allow <Tool>` 动态添加。[cc-9][cc-10]

### 3.2 perm:tool-denylist → ✓
settings.json `deny` 数组 + `/permissions deny <Tool>`。[cc-9][cc-10]

### 3.3 perm:file-scope → ✓
Deny 规则支持按 path glob；例 `Read(./private/**)`。[cc-9][cc-10]

### 3.4 perm:network-policy → ~
WebFetch/WebSearch 的粒度在工具级（可整体 allow/deny）；没有针对 URL/域的 per-request 网络策略。shell sandbox 可限制网络，但不属同一策略引擎。[cc-9]

### 3.5 perm:shell-allowlist → ✓
Bash(`cmd` arg-pattern) 规则支持按命令前缀授权；例 `Bash(git diff:*)`。[cc-9][cc-10]

### 3.6 perm:write-confirm → ✓
默认模式下 Edit/Write 要求确认；`acceptEdits` 模式自动接受文件编辑；`bypassPermissions` 全跳过。[cc-9]

### 3.7 perm:modes-enum → ✓
文档明确列出：`default` / `acceptEdits` / `plan` / `bypassPermissions`（部分文档补充 `dontAsk`）。Mode 名已确立为官方枚举。[cc-9]

---

## 4. Context & Memory

### 4.1 ctx:project-instructions → ✓
CLAUDE.md 项目级；放到仓库根或子目录；自动加载。`AGENTS.md` 通过 `@AGENTS.md` 导入支持。[cc-11][cc-12]

### 4.2 ctx:user-instructions → ✓
`~/.claude/CLAUDE.md` 为用户全局；跨所有项目加载。[cc-11]

### 4.3 ctx:directory-hierarchy → ✓
三级合并：enterprise/managed → user (`~/.claude/CLAUDE.md`) → project (`CLAUDE.md`) → local (`CLAUDE.local.md`)。文档明确记录加载顺序和合并策略。[cc-11]

### 4.4 ctx:memory-read → ✓
`#` 快捷键读当前 memory；`/memory` 查看 / 编辑；`@path` 导入其他 md。[cc-11]

### 4.5 ctx:memory-write → ✓
`# <note>` 在交互中快速追加到 CLAUDE.md；`/memory edit` 打开编辑器。[cc-11]

### 4.6 ctx:memory-auto-load → ✓
Session 启动时自动扫描并注入三级 CLAUDE.md；用户无需 opt-in。[cc-11]

### 4.7 ctx:memory-compaction → ✓
`/compact` 斜杠命令压缩当前上下文；`--include-hook-events` 配合；`PreCompact` hook 可拦截。[cc-13]

---

## 5. Skills Registry

### 5.1 skill:discover → ✓
`~/.claude/skills/` + `<project>/.claude/skills/` 中的 SKILL.md 被自动扫描；`/skills` 斜杠命令列出可用技能。[cc-14]

### 5.2 skill:invoke → ✓
Skill 由 agent 判断何时触发（基于 SKILL.md 的 description 与 frontmatter）；也可通过 `/skill <name>` 显式调用。[cc-14]

### 5.3 skill:params → ✓
SKILL.md frontmatter 声明 allowed-tools、arguments；agent 传参结构化。[cc-14]

### 5.4 skill:nested → ✗
文档未描述 skill 调用另一 skill 的一等机制（可通过在 SKILL.md 中引用其他 md，但这不是 "nested invocation"，仅是资产复用）。

### 5.5 skill:version-dependency → ✗
SKILL.md frontmatter 无版本字段，亦无依赖声明；skill 注册表不做版本解析。

---

## 6. Subagents

### 6.1 sub:dispatch → ✓
通过 `Task` 工具或 `--agents` 启动；`.claude/agents/` 中的 Markdown 文件即 subagent 定义。[cc-15]

### 6.2 sub:isolated-context → ✓
Subagent 默认拥有独立 context window；parent 通过工具返回值聚合结果。[cc-15]

### 6.3 sub:result-return → ✓
Subagent 返回字符串给 parent（Task 工具的 return value）。[cc-15]

### 6.4 sub:parallel → ✓
Parent 可在同一 turn 中调用多个 Task，subagent 并行执行。[cc-15]

### 6.5 sub:nested → ✗
文档明确："Subagents cannot spawn other subagents."——禁止嵌套。[cc-15]

### 6.6 sub:tool-inheritance → ✓
Subagent 的 `tools:` frontmatter 声明可用工具；省略则继承 parent。[cc-15]

---

## 7. Slash / User Commands

### 7.1 cmd:builtin → ✓
内置 `/help`、`/clear`、`/compact`、`/model`、`/permissions`、`/rewind`、`/skills`、`/mcp`、`/cost`、`/memory` 等几十条；文档有完整列表。[cc-16]

### 7.2 cmd:user-register → ✓
自定义命令通过 SKILL.md（新架构：commands 合并进 skills）或 `.claude/commands/<name>.md`（遗留，仍支持）。[cc-16][cc-14]

### 7.3 cmd:param-schema → ✓
Command md 的 frontmatter 可声明 `argument-hint`、`allowed-tools` 等；agent 按 schema 调用。[cc-16]

### 7.4 cmd:execute → ✓
用户输入 `/name arg1 arg2` 即执行。[cc-16]

---

## 8. Hooks

### 8.1 hook:pre-tool-use → ✓
`PreToolUse` 事件；接收 tool name + input，可 deny / allow / modify。[cc-6]

### 8.2 hook:post-tool-use → ✓
`PostToolUse` 事件；接收 tool response。[cc-6]

### 8.3 hook:user-prompt-submit → ✓
`UserPromptSubmit` 事件；可改写或拦截 prompt。[cc-6]

### 8.4 hook:stop → ✓
`Stop` 事件；session 正常结束触发。文档还列 `SubagentStop`。[cc-6]

### 8.5 hook:notification → ✓
`Notification` 事件；系统发出通知时触发。[cc-6]

### 8.6 hook:session-start → ✓
`SessionStart` 事件；文档还列 `SessionEnd`。[cc-6]

### 8.7 hook:callback-management → ✓
Hooks 在 settings.json 声明并由 `/hooks` 斜杠命令查看/管理。[cc-6]

### 8.8 hook:failure-isolation → ✓
Hook 脚本失败不导致 CLI 崩溃；文档描述 hooks 有超时与错误隔离。[cc-6]

---

## 9. Streaming & Events

### 9.1 evt:text-delta → ✓
TTY 模式默认流式输出；`--output-format stream-json` 暴露结构化 text 增量。[cc-7]

### 9.2 evt:thinking-delta → ✓
Extended thinking 在 stream-json 中以独立 block 输出；TTY 展示灰色折叠区。[cc-7]

### 9.3 evt:tool-use → ✓
stream-json 事件流含 tool_use block。[cc-7]

### 9.4 evt:tool-result → ✓
stream-json 事件流含 tool_result block。[cc-7]

### 9.5 evt:subagent → ~
subagent 生命周期通过 `SubagentStart/Stop` hooks 暴露，但 stream-json 未把 subagent 列为一等事件类型——必须通过 hooks 捕获。[cc-6][cc-7]

### 9.6 evt:cost → ~
`/cost` 给会话总计；`--include-hook-events` 可让 stream-json 输出带 usage 字段；但没有一等的 "per-turn cost delta" 事件——usage 要自行 diff。[cc-17]

### 9.7 evt:todo-tracking → ✓
内置 TodoWrite 工具；每次 todo 更新会在流中以 tool_use/tool_result 体现。[cc-7]

---

## 10. Cost & Usage Accounting

### 10.1 cost:per-turn → ~
`/cost` 显示会话累计；stream-json usage 字段按 turn 输出，需自行计算 delta。无一等 per-turn 摘要命令。[cc-17]

### 10.2 cost:per-session → ✓
`/cost` 展示当前 session 累计 token + USD；`--max-budget-usd` 限制上限。[cc-17]

### 10.3 cost:cache-read → ✓
API 返回 usage 含 `cache_read_input_tokens`；stream-json 透传该字段。[cc-17]

### 10.4 cost:cache-creation → ✓
API usage 含 `cache_creation_input_tokens`；stream-json 透传。[cc-17]

### 10.5 cost:budget-alert → ~
`--max-budget-usd` 是硬 cap（超出即停），不是渐进告警；无 "剩 10%" 预警事件。[cc-7]

### 10.6 cost:audit-export → ~
stream-json 全量事件可落盘作审计；但 CLI 无 `claude export-cost` 一等命令。subscriber 可用 `/stats`。[cc-17]

---

## 11. File Checkpointing & Diff

### 11.1 ckpt:file-checkpoint → ✓
每次 Edit/Write 前自动快照；存于 `.claude/checkpoints/`。[cc-4]

### 11.2 ckpt:file-revert → ✓
`/rewind` 或 ESC+ESC 可单独 revert 文件到前一 checkpoint。[cc-3][cc-4]

### 11.3 ckpt:per-turn-diff → ✓
每 turn 结束显示涉及文件的 diff；checkpoint 粒度为 per-edit-tool-call。[cc-4]

### 11.4 ckpt:unified-diff → ✓
Diff 以 unified diff 格式呈现。[cc-4]

### 11.5 ckpt:git-integration → ✗
文档明确："Checkpointing is not a replacement for version control."——不与 git 集成，且不跟踪 Bash 命令对文件的改动。[cc-4]

---

## 12. MCP Integration

### 12.1 mcp:server-define → ✓
`claude mcp add <name> <cmd>` 子命令；或 `.mcp.json` 项目级文件；或 settings.json user/project 作用域。[cc-18]

### 12.2 mcp:server-lifecycle → ✓
`claude mcp list/get/remove` 管理；`/mcp` 斜杠命令显示状态。[cc-18]

### 12.3 mcp:tool-discovery → ✓
Server 声明的 tools 自动发现；支持 `list_changed` notification 做动态更新。[cc-18]

### 12.4 mcp:permission-gate → ✓
MCP tool 走同一权限引擎；可在 allow/deny 中以 `mcp__<server>__<tool>` 形式列出。[cc-18][cc-9]

### 12.5 mcp:status-logs → ✓
`/mcp` 显示 server 状态（connected/failed）、tool 列表；日志在 `~/.claude/logs/mcp/`。[cc-18]

### 12.6 mcp:dynamic-toggle → ✓
`/mcp` 可运行时 enable/disable 单个 server；`list_changed` 触发发现。[cc-18]

---

## 13. Launch & Environment

### 13.1 env:working-directory → ✓
启动 CLI 的 cwd 即主工作目录；所有相对路径以此解析。[cc-7]

### 13.2 env:additional-directories → ✓
`--add-dir <path>` 标志；`/add-dir` 斜杠命令运行时添加。[cc-7]

### 13.3 env:env-vars → ✓
尊重 `ANTHROPIC_*` 环境变量；settings.json 可声明 `env` 对象注入给 hooks/MCP。[cc-10]

### 13.4 env:cli-flag-translate → ✓
完整 CLI flag 表；`--help` 和 cli-reference.md 对齐。[cc-7]

### 13.5 env:diagnostics → ✓
`/doctor` 或 `claude doctor` 输出环境诊断（版本、配置位置、权限）。[cc-16]

### 13.6 env:profile-system → ✗
无一等命名 profile 概念；settings 分层 (managed/user/project/local) 但不是 "switch between named profiles" 模型。

---

## 14. Output Styles & Personality

### 14.1 style:output-format → ✓
`--output-format text|stream-json`（非交互模式）；交互模式永远 TTY。[cc-7]

### 14.2 style:system-prompt-override → ~
`--system-prompt` 标志仅在 `--print` 非交互模式可用（文档里只有 print mode 的 override 示例）；交互模式下通过 output styles 覆盖 persona 层，不能替换核心 system prompt。[cc-7][cc-19]

### 14.3 style:system-prompt-append → ✓
`--append-system-prompt` 标志，交互/非交互均可。[cc-7]

### 14.4 style:persona-inheritance → ✓
Output styles 系统（Default / Explanatory / Learning）+ 自定义 style 的 md 文件；`outputStyle` 设置；style 可继承/扩展。[cc-19]

---

# 脚注来源（归 sources.md 的 Claude CLI 表）

以下编号与 `sources.md` 的 cc-* 行一对一。

- [cc-1] Overview（sessions 基础） → code.claude.com/docs/en/overview
- [cc-2] CLI reference（--resume/--continue/--fork-session） → code.claude.com/docs/en/cli-reference
- [cc-3] Checkpointing + rewind → code.claude.com/docs/en/checkpointing
- [cc-4] Checkpointing detail（per-edit snapshot, revert, not git） → code.claude.com/docs/en/checkpointing
- [cc-5] Interactive mode（Escape 中断） → code.claude.com/docs/en/interactive-mode
- [cc-6] Hooks（事件类型、settings.json 声明、failure isolation） → code.claude.com/docs/en/hooks
- [cc-7] CLI reference 全量标志（--model/--max-turns/--output-format/--add-dir/etc.） → code.claude.com/docs/en/cli-reference
- [cc-8] Model picker（/model） → code.claude.com/docs/en/interactive-mode
- [cc-9] Permission modes（枚举、Plan mode、ExitPlanMode 工具） → code.claude.com/docs/en/permission-modes
- [cc-10] Settings（allow/deny/env/hooks 配置面） → code.claude.com/docs/en/settings
- [cc-11] Memory（CLAUDE.md 三级、/memory、#） → code.claude.com/docs/en/memory
- [cc-12] AGENTS.md 互通 → github.com/anthropics/claude-code README
- [cc-13] /compact + PreCompact hook → code.claude.com/docs/en/hooks
- [cc-14] Skills（SKILL.md frontmatter、自动发现、命令合并） → code.claude.com/docs/en/skills
- [cc-15] Subagents（.claude/agents/、parallel、cannot-nest） → code.claude.com/docs/en/sub-agents
- [cc-16] Slash commands（内置 + 自定义） → code.claude.com/docs/en/commands
- [cc-17] /cost + usage fields → code.claude.com/docs/en/costs
- [cc-18] MCP（claude mcp 子命令、/mcp、.mcp.json、list_changed） → code.claude.com/docs/en/mcp
- [cc-19] Output styles（Default/Explanatory/Learning + 自定义） → code.claude.com/docs/en/output-styles
