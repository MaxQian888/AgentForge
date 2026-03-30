## Context

The TS Bridge (`src-bridge/`) is the unified AI execution gateway for AgentForge. It delegates all LLM work through three pluggable runtimes — Claude Code (via `@anthropic-ai/claude-agent-sdk`), Codex (via CLI), and OpenCode (via HTTP). Each runtime adapter currently implements the minimum viable execution path: spawn → stream events → capture continuity state. However, each upstream platform exposes significantly richer capabilities that the Bridge ignores. This leaves documented features like multi-agent orchestration, deep review with tool interception, structured output, file rollback, and session forking inoperable at the Bridge layer.

**Current state per runtime:**
- **Claude Code**: Uses `query()` with 10 of 30+ available options. No hooks, no subagents, no thinking config, no file checkpointing, no elicitation, no structured output.
- **Codex**: Handles 4 of 8 event types. Parses 2 of 9 item detail variants. No MCP passthrough, no structured output, no image support, no fork/rollback.
- **OpenCode**: Uses 6 of 40+ HTTP endpoints. Handles 5 of 17+ SSE event types. Parses 2 of 7 part types. No fork/revert, no diff, no todo sync, no permission forwarding.

## Goals / Non-Goals

**Goals:**
- Wire all production-relevant SDK/CLI/API features through the Bridge so Go orchestrator can access them
- Maintain backward compatibility — existing ExecuteRequest payloads continue to work unchanged
- Extend type system (ExecuteRequest, AgentEvent, AgentStatus) to carry new capabilities
- Add new Bridge HTTP routes for operations that don't fit the execute flow (fork, rollback, revert, diff)
- Ensure every new feature has corresponding test coverage

**Non-Goals:**
- Implementing the Go orchestrator changes to send new fields (separate change)
- Building the frontend UI for new capabilities
- Changing the Bridge's HTTP framework or deployment model
- Adding new LLM providers or runtimes
- Implementing agent team coordination logic (P2 feature, depends on subagent support being wired first)

## Decisions

### D1: Additive extension of ExecuteRequest, not new request types

**Decision**: Extend `ExecuteRequest` with optional fields for advanced features rather than creating separate request types per runtime.

**Rationale**: The Bridge already normalizes runtime differences behind `ExecuteRequest`. Adding optional fields (`thinking_config?`, `output_schema?`, `hooks_config?`, `attachments?`, `file_checkpointing?`) keeps the single-entry-point pattern. Runtime adapters ignore fields they don't support.

**Alternative considered**: Per-runtime request types (`ClaudeExecuteRequest`, `CodexExecuteRequest`). Rejected because it would force the Go orchestrator to know runtime-specific details, breaking the abstraction.

### D2: Event type extension via new AgentEvent subtypes

**Decision**: Add new `AgentEvent.type` values: `reasoning`, `file_change`, `todo_update`, `progress`, `rate_limit`, `partial_message`, `permission_request`. Keep existing types unchanged.

**Rationale**: The WebSocket event stream to Go already handles arbitrary event types via the `type` discriminator. Go can ignore unknown types gracefully. This is fully backward-compatible.

### D3: Hook forwarding via callback URL, not inline callbacks

**Decision**: For Claude Code hooks and Codex approval policies, the Bridge exposes a local HTTP callback endpoint. When a hook fires, the Bridge POSTs to Go orchestrator's callback URL (provided in ExecuteRequest) and awaits a response.

**Rationale**: The Bridge runs as a subprocess — it cannot hold JS closures from Go. The callback pattern matches the existing WebSocket event flow direction. Go orchestrator already handles async request/response patterns.

**Alternative considered**: WebSocket bidirectional messaging. Rejected because the current WS channel is fire-and-forget (Bridge → Go). Adding reverse messaging would complicate the protocol.

### D4: Structured output as optional request field with runtime-specific mapping

**Decision**: Add `output_schema?: { type: "json_schema"; schema: object }` to ExecuteRequest. Map to Claude's `outputFormat`, Codex's `--output-schema`, and OpenCode's message parameters.

**Rationale**: All three runtimes support structured output but with different APIs. A single Bridge-level field provides uniform access.

### D5: Session operations (fork/rollback/revert) as separate Bridge routes

**Decision**: Add `POST /bridge/fork`, `POST /bridge/rollback`, `POST /bridge/revert`, `GET /bridge/diff/:id` as new Bridge HTTP routes rather than overloading ExecuteRequest.

**Rationale**: These are distinct lifecycle operations, not execution requests. They need different request/response shapes and don't produce streaming events.

### D6: Incremental implementation in 4 waves

**Decision**: Implement in priority order:
1. **Wave 1 — Types & Infrastructure**: Extend types, schemas, add new routes, hook callback endpoint
2. **Wave 2 — Claude Code advanced**: Hooks, thinking, subagents, file checkpointing, structured output, elicitation
3. **Wave 3 — Codex advanced**: Full event parsing, structured output, image support, MCP passthrough, fork
4. **Wave 4 — OpenCode advanced**: Fork/revert, diff, todo, command execution, full event/part parsing, permission forwarding

**Rationale**: Wave 1 establishes the shared foundation. Waves 2-4 are independent per-runtime and can be parallelized. Claude Code is highest priority as it's the default runtime.

## Risks / Trade-offs

**[Risk] Claude Agent SDK breaking changes** → Pin to exact version (0.2.81). Add integration smoke test that validates SDK imports and basic query still work. Upgrade separately.

**[Risk] Codex CLI output format changes** → Codex JSONL format is marked experimental. Add defensive parsing with unknown-event passthrough. Log but don't crash on unrecognized events.

**[Risk] OpenCode server API instability** → OpenCode is in active development. Use versioned endpoint paths where available. Add health probe version check to detect incompatible servers.

**[Risk] Hook callback latency** → Go orchestrator round-trip adds latency to every tool call when hooks are active. Mitigation: hooks are opt-in per request. Default is no hooks (zero overhead). Add configurable timeout (default 5s) for hook callbacks.

**[Risk] Type expansion bloat** → ExecuteRequest grows from ~15 to ~25 fields. Mitigation: all new fields are optional. Group related fields into sub-objects (`thinking_config`, `hooks_config`, `attachment_config`).

**[Trade-off] Callback URL vs WebSocket bidirectional** → Callback is simpler but adds a reverse HTTP dependency. Acceptable because Go orchestrator already runs an HTTP server.

**[Trade-off] Full event parsing vs passthrough** → Parsing all event subtypes means more code to maintain. But passthrough would lose the normalization benefit. Chose full parsing for consistency.

## Open Questions

1. **Hook callback authentication**: Should the Bridge → Go callback use a shared secret or rely on localhost-only binding?
2. **Codex MCP config format**: Should Bridge translate its plugin manifest format to Codex `config.toml` format, or expect Go to provide Codex-native config?
3. **OpenCode PTY support**: The `/pty` WebSocket endpoint enables interactive terminal sessions. Is this needed for any AgentForge use case, or can it be deferred?
