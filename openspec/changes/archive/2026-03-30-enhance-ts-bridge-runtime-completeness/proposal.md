## Why

The TS Bridge currently provides basic execution for all three agent runtimes (Claude Code, Codex, OpenCode), but significantly under-utilizes each platform's capabilities. Claude Code SDK offers hooks, subagents, extended thinking, file checkpointing, structured output, and elicitation—none of which are wired through the Bridge. Codex CLI supports MCP servers, approval policies, file change events, reasoning items, fork/rollback, and structured output schemas—all unused. OpenCode exposes 40+ HTTP endpoints including session fork/revert, slash commands, agent/skill discovery, todo management, PTY access, diff retrieval, and granular permission responses—the Bridge only uses 6. Completing these integrations is necessary to deliver the multi-agent orchestration, deep review, and role-based execution features documented in AGENT_ORCHESTRATION.md.

## What Changes

### Claude Code Runtime
- Wire `agents` option for custom subagent definitions (planner/coder/reviewer) — enables multi-agent delegation
- Wire `hooks` system (PreToolUse, PostToolUse, SubagentStart/Stop, PermissionRequest) for tool interception and audit
- Wire `maxThinkingTokens` / extended thinking for Opus-class models
- Wire `enableFileCheckpointing` + expose `rewindFiles()` for checkpoint-based rollback
- Wire `outputFormat` for structured JSON output with schema validation
- Wire `onElicitation` for MCP server auth/form handling
- Wire `canUseTool` callback for dynamic permission decisions from Go orchestrator
- Wire `includePartialMessages` for streaming token feedback
- Wire `disallowedTools` for explicit tool blacklisting
- Wire `fallbackModel` for model failover
- Wire `env` for custom environment variable injection
- Handle `SDKRateLimitEvent`, `SDKToolProgressMessage`, `SDKCompactBoundaryMessage` message types
- Expose `Query` methods: `interrupt()`, `setModel()`, `setMaxThinkingTokens()`, `mcpServerStatus()`

### Codex Runtime
- Handle `item.updated` events for streaming progress (currently only item.started/completed)
- Handle `turn.started` and `turn.failed` events properly
- Parse `Reasoning`, `FileChange`, `McpToolCall`, `TodoList`, `WebSearch` item detail variants (currently only AgentMessage and CommandExecution)
- Wire `--output-schema` for structured output
- Wire `--image` flag for image attachment support
- Wire `--add-dir` for additional writable directories
- Wire `--search` for web search capability
- Support `fork` mode in addition to `resume` for session management
- Wire MCP server configuration passthrough (`--config` with mcp_servers)
- Expose `rollback` capability for turn-level undo

### OpenCode Runtime
- Wire `/session/:id/fork` for session forking
- Wire `/session/:id/revert` and `/session/:id/unrevert` for message-level undo
- Wire `/session/:id/diff` for file diff retrieval
- Wire `/session/:id/todo` for todo list sync
- Wire `/session/:id/message` (GET) for message history retrieval
- Wire `/session/:id/command` for slash command execution
- Wire `/session/:id/permissions/:permissionID` for permission response forwarding
- Wire `/agent` and `/skill` discovery endpoints
- Handle `session.status`, `todo.updated`, `message.updated`, `command.executed` SSE events (currently only 5 event types handled)
- Parse `ReasoningPart`, `FilePart`, `AgentPart`, `CompactionPart`, `SubtaskPart` part types (currently only TextPart and ToolPart)
- Wire `/config` PATCH for runtime configuration updates
- Add provider OAuth flow support via `/provider/{id}/oauth/*`

### Cross-Runtime
- Add structured output support to `ExecuteRequest` and response types
- Add image/file attachment support to `ExecuteRequest`
- Extend `AgentEvent` types for new event categories (reasoning, file_change, todo_update, progress, rate_limit)
- Add fork/rollback operations to session management layer
- Add tool permission callback pathway from Bridge → Go orchestrator → Bridge

## Capabilities

### New Capabilities
- `bridge-claude-advanced-features`: Hooks, subagents, thinking, file checkpointing, elicitation, structured output for Claude Code runtime
- `bridge-codex-advanced-features`: Full event parsing, structured output, image support, MCP passthrough, fork/rollback for Codex runtime
- `bridge-opencode-advanced-features`: Session fork/revert, diff, todo sync, command execution, permission forwarding, full event/part parsing for OpenCode runtime
- `bridge-cross-runtime-extensions`: Structured output, attachments, extended event types, fork/rollback, and tool permission callbacks shared across all runtimes

### Modified Capabilities
- `bridge-http-contract`: New routes for fork, rollback, revert, diff, structured output, and permission callback
- `agent-sdk-bridge-runtime`: Extended ExecuteRequest/AgentEvent types, new continuity fields, additional runtime adapter methods

## Impact

- **Code**: `src-bridge/src/handlers/claude-runtime.ts`, `codex-runtime.ts`, `opencode-runtime.ts`, `src-bridge/src/opencode/transport.ts`, `src-bridge/src/types.ts`, `src-bridge/src/schemas.ts`, `src-bridge/src/server.ts`, `src-bridge/src/session/manager.ts`
- **Types**: `ExecuteRequest`, `AgentEvent`, `AgentStatus`, continuity state types all extended
- **API**: New Bridge HTTP routes for fork/rollback/revert/diff/permissions
- **Dependencies**: No new packages needed — all features use existing `@anthropic-ai/claude-agent-sdk`, Codex CLI, and OpenCode HTTP API
- **Tests**: All 39 existing test files need updates; new test files for advanced features
- **Go Orchestrator**: Must be updated to send new request fields and handle new event types (separate change)
