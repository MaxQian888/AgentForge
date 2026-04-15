# Skeleton Notes — Row Enumeration Reasoning

Task 1 of Phase 1. Enumerating 60-90 atomic capability rows for matrix.csv.

## Granularity Calibration

Per SPEC section 4.3: split when adapters diverge on sub-capability; merge when they converge.
Per section 4.4: Plan Mode -> Group 2, Todo -> Group 9, Permission modes enum -> Group 3, EnterPlanMode/ExitPlanMode -> Group 2.

Row key format: `<group>.<seq>` (integer seq, not zero-padded).
Capability naming: `group_concept:modifier` style, lowercase, colons as separators.

## Per-Group Enumeration

### Group 1 — Session & Lifecycle (9 rows)
1.1 session:create — new session from scratch
1.2 session:resume — resume existing session by ID
1.3 session:fork — fork session (same directory)
1.4 session:fork:cross-directory — fork into different working directory
1.5 session:fork:by-message-id — fork from specific conversation point
1.6 session:rewind — rewind to prior message/checkpoint
1.7 session:interrupt — cancel in-flight execution
1.8 session:pause-resume — suspend and later resume
1.9 session:snapshot-export — export session state for portability

### Group 2 — Execution Control (7 rows)
2.1 exec:set-model — change model mid-session
2.2 exec:thinking-budget — set/adjust thinking token budget
2.3 exec:max-turns — limit autonomous turn count
2.4 exec:temperature — set sampling temperature
2.5 exec:permission-mode — set permission mode (bypass/plan/default etc.)
2.6 exec:plan-mode — enter/exit plan mode (section 4.4 boundary)
2.7 exec:plan-mode-tools — EnterPlanMode/ExitPlanMode as callable tools (section 4.4 boundary)

### Group 3 — Tool & File Permissions (7 rows)
3.1 perm:tool-allowlist — restrict to named tools only
3.2 perm:tool-denylist — block specific tools
3.3 perm:file-scope — restrict file read/write to paths
3.4 perm:network-policy — allow/deny network access
3.5 perm:shell-allowlist — restrict shell commands
3.6 perm:write-confirm — require confirmation before file writes
3.7 perm:modes-enum — permission modes enumeration (bypass/acceptEdits/plan/default/dontAsk) (section 4.4 boundary)

### Group 4 — Context & Memory (7 rows)
4.1 ctx:project-instructions — CLAUDE.md / AGENTS.md / GEMINI.md project-level instructions
4.2 ctx:user-instructions — user-level global instructions
4.3 ctx:directory-hierarchy — three-level directory hierarchy auto-merge
4.4 ctx:memory-read — read persisted memory entries
4.5 ctx:memory-write — create/update memory entries
4.6 ctx:memory-auto-load — auto-load relevant memories on session start
4.7 ctx:memory-compaction — compress/summarize old memories

### Group 5 — Skills Registry (5 rows)
5.1 skill:discover — list/search available skills
5.2 skill:invoke — trigger a skill by name
5.3 skill:params — pass structured parameters to skill
5.4 skill:nested — invoke skill from within another skill
5.5 skill:version-dependency — declare/resolve skill version constraints

### Group 6 — Subagents (6 rows)
6.1 sub:dispatch — spawn a subagent for a task
6.2 sub:isolated-context — subagent runs with restricted context
6.3 sub:result-return — collect structured result from subagent
6.4 sub:parallel — run multiple subagents concurrently
6.5 sub:nested — subagent spawns its own subagents
6.6 sub:tool-inheritance — subagent inherits/restricts parent tool set

### Group 7 — Slash / User Commands (4 rows)
7.1 cmd:builtin — built-in slash commands (/help, /clear, etc.)
7.2 cmd:user-register — register custom user commands
7.3 cmd:param-schema — declare parameter schema for commands
7.4 cmd:execute — execute registered command by name

### Group 8 — Hooks (8 rows)
8.1 hook:pre-tool-use — callback before tool execution
8.2 hook:post-tool-use — callback after tool execution
8.3 hook:user-prompt-submit — callback on user prompt submission
8.4 hook:stop — callback when agent stops
8.5 hook:notification — callback for notification events
8.6 hook:session-start — callback on session start
8.7 hook:callback-management — register/unregister/list hooks
8.8 hook:failure-isolation — hook failure does not crash agent

### Group 9 — Streaming & Events (7 rows)
9.1 evt:text-delta — streaming text token events
9.2 evt:thinking-delta — streaming thinking/reasoning events
9.3 evt:tool-use — tool invocation event
9.4 evt:tool-result — tool result event
9.5 evt:subagent — subagent lifecycle events
9.6 evt:cost — per-turn cost event
9.7 evt:todo-tracking — todo/task tracking events (section 4.4 boundary)

### Group 10 — Cost & Usage Accounting (6 rows)
10.1 cost:per-turn — per-turn token/cost breakdown
10.2 cost:per-session — cumulative session cost
10.3 cost:cache-read — cache read token accounting
10.4 cost:cache-creation — cache creation token accounting
10.5 cost:budget-alert — alert when approaching budget threshold
10.6 cost:audit-export — export cost/usage data for auditing

### Group 11 — File Checkpointing & Diff (5 rows)
11.1 ckpt:file-checkpoint — snapshot file state at a point
11.2 ckpt:file-revert — revert files to prior checkpoint
11.3 ckpt:per-turn-diff — show diff of files changed per turn
11.4 ckpt:unified-diff — produce unified diff format output
11.5 ckpt:git-integration — integrate checkpoints with git (stash/commit)

### Group 12 — MCP Integration (6 rows)
12.1 mcp:server-define — declare MCP server connection
12.2 mcp:server-lifecycle — start/stop/restart MCP server
12.3 mcp:tool-discovery — discover tools from MCP server
12.4 mcp:permission-gate — gate MCP tool access via permissions
12.5 mcp:status-logs — view MCP server status and logs
12.6 mcp:dynamic-toggle — enable/disable MCP servers at runtime

### Group 13 — Launch & Environment (6 rows)
13.1 env:working-directory — set primary working directory
13.2 env:additional-directories — add secondary directories
13.3 env:env-vars — pass environment variables
13.4 env:cli-flag-translate — translate CLI flags to API params
13.5 env:diagnostics — runtime environment diagnostics
13.6 env:profile-system — named configuration profiles

### Group 14 — Output Styles & Personality (4 rows)
14.1 style:output-format — set output style (concise/verbose/json etc.)
14.2 style:system-prompt-override — override system prompt entirely
14.3 style:system-prompt-append — append to system prompt
14.4 style:persona-inheritance — inherit persona/tone from role definition

## Totals

| Group | Rows |
|-------|------|
| 1     | 9    |
| 2     | 7    |
| 3     | 7    |
| 4     | 7    |
| 5     | 5    |
| 6     | 6    |
| 7     | 4    |
| 8     | 8    |
| 9     | 7    |
| 10    | 6    |
| 11    | 5    |
| 12    | 6    |
| 13    | 6    |
| 14    | 4    |
| **Total** | **87** |

87 rows. Within the 60-90 target range.

## Calibration Notes

- Group 1 Session: split fork into 3 sub-rows per SPEC section 4.3 explicit example. Dropped "checkpoint list" as a separate row — folded into rewind (same adapter boundary). Dropped "revert" as separate from "rewind" since they share the same adapter behavior.
- Group 2: Plan mode and plan mode tools are separate rows per section 4.4.
- Group 3: Permission modes enum is a row per section 4.4.
- Group 8: Each hook type is its own row per SPEC section 4.3 explicit example. Added callback-management and failure-isolation as cross-cutting rows since adapters diverge on these.
- Group 9: Todo/task tracking placed here per section 4.4.
- Group 10: Did not split "multi-model mixed" into separate row — folded into per-turn since adapters that support multi-model always report it per-turn.
