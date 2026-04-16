# Bridge ACP Client — Phase 1 Implementation Plan

- **ID**: `2026-04-16-bridge-acp-client-phase1-impl`
- **Spec**: `docs/superpowers/specs/2026-04-16-bridge-acp-client-integration.md` (draft-1)
- **Goal**: Migrate four adapters (`claude_code / codex / opencode / cursor`) from per-adapter transports to a unified ACP client in `src-bridge/`.
- **Status**: ready-to-execute
- **Strategy**: Subagent-Driven Development on `master`. Each task executed by an implementer subagent + spec-compliance reviewer; code-quality review for code-heavy tasks only.
- **Non-goals**: gemini/qoder/iflow, ACP server role, unstable ACP features, new frontend UI.

---

## Invariants across all tasks

- **No behavior regression** for any of the four adapters while their `BRIDGE_ACP_<ADAPTER>` flag is `0` (legacy path). All new code is reachable only when the flag is `1`.
- **HTTP route boundary is frozen** — don't rename, remove, or change the response shape of any `/bridge/*` endpoint. Additive query params are OK.
- **Event contract is frozen** — the `AgentEventType` enum may gain new members (additive) but existing ones keep their current semantics. Frontend/Go must keep working against `master` without changes.
- **Tests run clean**: `cd src-bridge && pnpm test` must pass on every commit. Integration tests that touch real adapters are gated by `SKIP_ACP_INTEGRATION=1` in CI (local runs may opt-in).
- **Atomic commits** per task with the task ID in the commit message (`acp(client): T<N> — <subject>`). Each task is one commit unless the implementer has strong reason to split, in which case the last commit carries `T<N>` and interim commits note the split.

## Dependencies between tasks

```
T1 ─┐
    ├─ T2 ─ T3 ─┐
    │          ├─ T4 ─┐
    │          │      ├─ T6 ─┬─ T7 ─┐
    │          ├─ T5 ─┘      │      ├─ T8 ─┐
    │          └───────────  T6? no │      ├─ T9  ─┐
    │                               │      ├─ T10 ─┤
    │                               │      └─ T11 ─┼─ T12
    │                               │              │
    │                               └──────────────┘
    └───────────────── T13 (can start once T12 done) ── T14 (docs/cleanup)
```

Linearized execution order: T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8 → T9 → T10 → T11 → T12 → T13 → T14.

(T4 and T5 could run parallel after T3 but we serialize for simplicity and to keep review coherent.)

## Task roster

| # | Subject | Primary files | Needs code-quality review? |
|---|---|---|---|
| T1 | Dependencies + module scaffold | `src-bridge/package.json`, `src-bridge/src/runtime/acp/**` skeleton | no |
| T2 | NDJSON transport + JSON-RPC dispatcher | `runtime/acp/transport.ts`, tests | yes |
| T3 | Process supervisor + `AcpClient` lifecycle | `runtime/acp/process.ts`, `runtime/acp/client.ts`, mock agent fixture, tests | yes |
| T4 | fs + terminal + permission client handlers | `runtime/acp/handlers/*.ts`, tests | yes |
| T5 | `session/update` → `AgentEventType` translator | `runtime/acp/events/session-update.ts`, tests | yes |
| T6 | Adapter dispatcher + flag-gated routing | `runtime/registry.ts` (edits), `runtime/acp/registry.ts`, tests | yes |
| T7 | claude_code ACP adapter + smoke | `runtime/acp/adapters/claude-code.ts`, integration test | yes |
| T8 | codex ACP adapter + smoke | `runtime/acp/adapters/codex.ts`, integration test | yes |
| T9 | opencode ACP adapter + smoke | `runtime/acp/adapters/opencode.ts`, integration test | yes |
| T10 | cursor ACP adapter + smoke | `runtime/acp/adapters/cursor.ts`, integration test | yes |
| T11 | `live_controls` unification + fork/rollback remap | `runtime/agent-runtime.ts`, `handlers/*` callsites | yes |
| T12 | Cross-layer audit (Go + frontend) | `src-go/**`, `app/**`, docs | no |
| T13 | Clean-up of legacy paths behind OFF flag (guarded, not deleted) | removal commits gated by follow-up review | no |
| T14 | CLAUDE.md + bridge README + spec changelog bump to draft-2 | doc files | no |

## Validation

After each task:

```bash
cd D:/Project/AgentForge/src-bridge
pnpm typecheck           # tsc --noEmit
pnpm lint
pnpm test -- --run       # unit + component
# integration tests opt-in:
# SKIP_ACP_INTEGRATION=0 pnpm test:integration
```

Plus at project root:

```bash
pnpm exec tsc --noEmit   # frontend type-check — must stay clean after T12
```

---

## T1 — Dependencies + module scaffold

**Scope**: Install ACP SDK dependency and create the `runtime/acp/` module skeleton with empty exports. No logic yet.

**Steps**:
1. Add to `src-bridge/package.json` dependencies: `"@zed-industries/agent-client-protocol": "^0.3.0"` (pin to latest published on npm as of 2026-04-18; confirm version with `pnpm view @zed-industries/agent-client-protocol version` before pinning).
2. `pnpm install` — commit updated lockfile.
3. Create files (each with a single-line `export {}` or placeholder interface — no logic):
   - `src-bridge/src/runtime/acp/index.ts`
   - `src-bridge/src/runtime/acp/client.ts` — export class `AcpClient` with constructor-only stub + method signatures matching spec §4.2
   - `src-bridge/src/runtime/acp/registry.ts` — export `ACP_ADAPTERS` constant matching spec §5.1
   - `src-bridge/src/runtime/acp/process.ts`, `runtime/acp/transport.ts`, `runtime/acp/session.ts`, `runtime/acp/errors.ts` — empty with `export {}`
   - `src-bridge/src/runtime/acp/handlers/fs.ts`, `handlers/terminal.ts`, `handlers/permission.ts` — empty
   - `src-bridge/src/runtime/acp/events/session-update.ts` — empty
4. Verify typecheck passes: `pnpm typecheck`.

**Deliverables**:
- `package.json` + `pnpm-lock.yaml` with new dep
- Module skeleton (all files above)

**Acceptance**:
- `pnpm typecheck` green
- `import { AcpClient } from "./runtime/acp"` works from anywhere in `src-bridge`
- Commit: `acp(client): T1 — scaffold runtime/acp module and dep`

---

## T2 — NDJSON transport + JSON-RPC dispatcher

**Scope**: Implement `runtime/acp/transport.ts`: a JSON-RPC 2.0 client over NDJSON stdio. No process spawning yet — take `stdin: Writable, stdout: Readable` in constructor.

**Spec reference**: §4.3.

**Contract**:
```ts
export class JsonRpcTransport {
  constructor(input: Readable, output: Writable, logger?: Logger);
  request<TParams, TResult>(method: string, params: TParams): Promise<TResult>;
  notify<TParams>(method: string, params: TParams): void;
  on<TParams>(method: string, handler: (params: TParams) => Promise<unknown> | unknown): void; // handles incoming requests from agent
  onNotification<TParams>(method: string, handler: (params: TParams) => void): void;
  close(): Promise<void>;
}
```

**Requirements**:
- NDJSON framing: one JSON object per `\n`; tolerate partial reads from chunked input.
- Pending-request map keyed by numeric id (start at 1, increment); response with no pending id → protocol error logged + dropped.
- Incoming agent-initiated request → run registered `on(method)` handler → write response (success or JSON-RPC error) back on stdin.
- Write backpressure: `request()` waits for `drain` if stdin write returns false.
- Malformed JSON → log + drop + continue (don't crash).
- Close semantics: pending promises reject with `AcpTransportClosed`; handlers unregistered.

**Tests** (`src-bridge/tests/unit/runtime/acp/transport.test.ts`):
- request/response happy path
- concurrent requests resolve in correct order
- agent-initiated request → handler return value serialized correctly
- notification (no response expected)
- malformed JSON line ignored, subsequent valid line processed
- partial chunked reads reassembled across lines
- response id with no pending request is dropped + logged

**Deliverables**:
- `runtime/acp/transport.ts` (~150 LOC)
- `runtime/acp/errors.ts` — define `AcpTransportClosed, AcpProtocolError`
- `tests/unit/runtime/acp/transport.test.ts`

**Acceptance**:
- All new unit tests pass
- Tests use in-memory `PassThrough` streams, no spawned processes
- Commit: `acp(client): T2 — NDJSON JSON-RPC transport`

---

## T3 — Process supervisor + `AcpClient` lifecycle

**Scope**: Implement `runtime/acp/process.ts` (spawning + lifecycle) and `runtime/acp/client.ts` (glue). End state: `AcpClient` can spawn a mock agent fixture, `initialize`, `newSession`, run one `prompt`, `cancel`, `dispose`.

**Spec reference**: §4.2 + §4.4 + §7.1.

**Requirements**:
- `ProcessHost.spawn(cmd, args, env, cwd) → { transport, exitPromise }`
- `AcpClient` composes `ProcessHost` + `JsonRpcTransport` from T2.
- `initialize` sends `{ protocolVersion: 1, clientCapabilities: { fs: { readTextFile: true, writeTextFile: true }, terminal: true } }`; stores returned `agentCapabilities`.
- `newSession(cwd, mcpServers)` implements spec §5.4 — `mcpServers` passed through.
- `prompt(sessionId, content)`: pre-validates content blocks against stored `agentCapabilities.promptCapabilities.*`; serialize per session (reject overlapping prompts with `AcpConcurrentPrompt`).
- `cancel(sessionId)` sends notification, waits for in-flight prompt to resolve with `stopReason === "cancelled"` (2s timeout → `AcpCancelTimeout`).
- `dispose()` sends cancel for all open sessions → closes stdin → 2s wait → SIGTERM → 5s wait → SIGKILL.
- Exit-with-pending-prompt → all pending promises reject with `AcpProcessCrash` carrying last 2 KB stderr.

**Mock agent fixture** (`src-bridge/tests/fixtures/mock-acp-agent.ts`):
- Standalone Bun/Node script that speaks ACP JSON-RPC stubs: accepts `initialize`, `session/new`, `session/prompt` (replies with one `session/update` `agent_message_chunk` then returns `{ stopReason: "end_turn" }`), `session/cancel`. Configurable via env: `MOCK_CRASH_ON_PROMPT=1` etc.

**Tests** (`tests/component/acp/client.test.ts`):
- spawn mock → `initialize` → `newSession` → `prompt("hello")` → receive 1 update → stopReason=end_turn → dispose
- cancel mid-prompt
- concurrent prompt on same session rejected
- mock crash mid-prompt → `AcpProcessCrash`
- dispose times out → escalates to SIGKILL (simulated with a "ignoring stdin close" fixture mode)

**Deliverables**:
- `runtime/acp/process.ts`, `runtime/acp/client.ts`, `runtime/acp/session.ts`, `runtime/acp/errors.ts` (extend)
- `tests/fixtures/mock-acp-agent.ts`
- `tests/component/acp/client.test.ts`

**Acceptance**:
- Component tests pass
- Commit: `acp(client): T3 — process supervisor and client lifecycle`

---

## T4 — Client handlers: fs + terminal + permission

**Scope**: Implement the three client-side handler modules. Register them on `AcpClient` during `initialize` so the agent can call back.

**Spec reference**: §6.

### 4a. fs (`runtime/acp/handlers/fs.ts`)
- `createFsHandler(sandbox: FsSandbox)` returns `{ readTextFile, writeTextFile }`.
- `FsSandbox.resolve(sessionId, path)` uses `node:path.resolve` + `realpath` (follow symlinks) + prefix-check against `AcpClient.cwd`. Reject on escape with JSON-RPC error `{ code: -32602, message: "Invalid params", data: { reason: "path_escapes_worktree" } }`.
- `readTextFile` honors `line` (1-based) + `limit` (lines).
- `writeTextFile` writes UTF-8; creates parent dirs with `recursive: true`; fsyncs before returning.

### 4b. terminal (`runtime/acp/handlers/terminal.ts`)
- `TerminalManager` with per-task allocator, `maxConcurrentTerminals = 16`, per-PTY `outputByteLimit = 10 MB`.
- Use `node-pty` if available (dev desktop) else fall back to `Bun.spawn` for CI.
- `terminal/create / output / wait_for_exit / kill / release` as per spec §6.2.
- Output buffer is a ring buffer capped at `outputByteLimit`; `truncated` flag set when overrun.

### 4c. permission (`runtime/acp/handlers/permission.ts`)
- `PermissionRouter` with injected `EventStreamer` + `HookCallbackManager`.
- On `session/request_permission`: generate `request_id`, emit `permission_request` event with ACP `toolCall` + `options`, register resolver in `HookCallbackManager`.
- `/bridge/permission-response/:request_id` already resolves via `HookCallbackManager`; no server-side route change — verify it routes our resolver cleanly.
- 10-minute timeout → router resolves as `{ outcome: "cancelled" }` and logs.

**Tests**:
- `tests/unit/runtime/acp/handlers/fs.test.ts` — sandbox escape table (symlink escapes, `..` traversal, absolute path outside root, drive letter on win), happy read/write, line/limit slicing
- `tests/unit/runtime/acp/handlers/terminal.test.ts` — create/output/kill lifecycle, cap enforcement, ring-buffer truncation flag
- `tests/component/acp/handlers/permission.test.ts` — mock agent requests permission → event emitted → simulate HTTP response → router returns correct outcome; timeout path

**Deliverables**:
- Three handler files + their test files
- `runtime/acp/client.ts` — handler registration during `initialize`

**Acceptance**:
- All new tests pass
- Sandbox escape tests cover: `../..`, symlink pointing outside, absolute `/etc/passwd` analogue, same on Windows (`C:\\Windows\\System32`)
- Commit: `acp(client): T4 — fs + terminal + permission handlers`

---

## T5 — `session/update` → `AgentEventType` translator

**Scope**: Pure mapping function lives in `runtime/acp/events/session-update.ts`. Wire it into `AcpClient` so incoming `session/update` notifications are forwarded through `EventStreamer`.

**Spec reference**: §8 (table).

**Contract**:
```ts
export function translateSessionUpdate(
  params: { sessionId: SessionId; update: SessionUpdate },
  taskId: string,
): AgentEvent[];
```

One ACP update can expand to zero or multiple `AgentEvent` objects. Pass through `_meta` unchanged into `AgentEvent.metadata`.

**Tests** (`tests/unit/runtime/acp/events/session-update.test.ts`):
- One test per variant from spec table §8
- `tool_call_update.fields.status` transitions → correct `tool_result` vs `tool.status_change`
- `_meta` passthrough
- Unknown `sessionUpdate` discriminator → one `error` event logged + empty array returned (don't crash)

**Deliverables**:
- `runtime/acp/events/session-update.ts`
- Test file
- Wire-up in `AcpClient` (transport `onNotification("session/update", ...)`)

**Acceptance**:
- Unit tests pass
- Commit: `acp(client): T5 — session/update event translator`

---

## T6 — Adapter dispatcher + flag-gated routing

**Scope**: Modify `src-bridge/src/runtime/registry.ts` so the four target adapters (`claude_code / codex / opencode / cursor`) consult `BRIDGE_ACP_<ADAPTER>` env flags and delegate to a new `AcpAdapter` class when enabled, falling back to legacy behavior otherwise.

**Spec reference**: §3, §5.

**Key edits**:
- Introduce `runtime/acp/adapter.ts` — implements the 12-method `RuntimeAdapter` contract via `AcpClient`.
- `createClaudeCodeAdapter / createCodexAdapter / createOpenCodeReadinessAdapter / createCliRuntimeAdapter` (cursor branch only) are wrapped in a dispatcher:
  ```ts
  function dispatch(adapterId: AdapterId, legacy: RuntimeAdapter, acp: RuntimeAdapter): RuntimeAdapter {
    return process.env[`BRIDGE_ACP_${adapterId.toUpperCase()}`] === "1" ? acp : legacy;
  }
  ```
- For `cursor`: keep `createCliRuntimeAdapter` for gemini/qoder/iflow; wrap only the cursor registration.
- The 12 `RuntimeAdapter` methods in the ACP adapter translate to `AcpClient` calls. Unsupported methods (e.g., `setThinkingBudget` if agent caps don't include it) throw `UnsupportedOperationError` — same error type as today, no new error class.
- Document all four env flags in `src-bridge/README.md` (or create one if missing).

**Tests**:
- `tests/unit/runtime/acp/adapter.test.ts` — each of 12 methods dispatches to correct `AcpClient` call; unsupported methods throw `UnsupportedOperationError`
- `tests/unit/runtime/registry.test.ts` — dispatcher switches on env flag (spy on factories)

**Deliverables**:
- `runtime/acp/adapter.ts`
- Modified `runtime/registry.ts`
- Tests
- `src-bridge/README.md` documenting flags

**Acceptance**:
- Tests pass
- With all four flags `0`, legacy paths unchanged and tests currently passing continue to pass
- Commit: `acp(client): T6 — adapter dispatcher and flag-gated registry`

---

## T7 — claude_code ACP adapter + smoke

**Scope**: Wire the `claude_code` path through ACP end-to-end. Smoke-test with a real `@agentclientprotocol/claude-agent-acp` subprocess.

**Steps**:
1. In `AcpAdapter`'s `claude_code` branch (or via a small delta file `runtime/acp/adapters/claude-code.ts` if adapter-specific hooks are needed), implement any claude-code-specific request mappings (e.g., ANTHROPIC_API_KEY propagation, thinking-budget capability check).
2. Integration test: `tests/integration/acp/claude-code.test.ts` — requires `ANTHROPIC_API_KEY` or skips with clear message. Smoke: set `BRIDGE_ACP_CLAUDE_CODE=1`, spawn task, send "say hi", assert `output` event received, cancel test session.
3. Run with flag `1` and `0` — confirm both pass against their respective paths.

**Acceptance**:
- Integration test green locally with env vars set
- No regression in legacy path tests
- Commit: `acp(client): T7 — claude_code adapter over ACP`

---

## T8 — codex ACP adapter + smoke

**Scope**: Same as T7 but for `codex`. Requires OpenAI API key or codex CLI login.

**Adapter-specific notes**:
- Tauri sidecar: document that `@zed-industries/codex-acp` optional-dep native binary must resolve at runtime. Log a clear error if missing.
- `codex-acp` requires `codex` CLI on PATH; integration test skips if absent.

**Acceptance**:
- Integration test green with env
- Commit: `acp(client): T8 — codex adapter over ACP`

---

## T9 — opencode ACP adapter + smoke

**Scope**: `opencode acp` native subcommand.

**Adapter-specific notes**:
- Provider auth is on-disk state managed by `opencode` itself. Existing `/bridge/opencode/provider-auth/*` routes are unchanged — they prep state before spawning `opencode acp`.
- `/undo` and `/redo` slash commands are unsupported per opencode docs; integration test must not assume they work.

**Acceptance**:
- Integration test green when `opencode` on PATH
- Commit: `acp(client): T9 — opencode adapter over ACP`

---

## T10 — cursor ACP adapter + smoke

**Scope**: `cursor-agent acp` native subcommand.

**Adapter-specific notes**:
- Login required; integration test skips with clear message if `cursor-agent` reports auth error.
- Capability flag `cursorExtensions: true` is advertised by the agent but we don't implement extension handlers in this spec — log a warning on first `cursor/*` call and return JSON-RPC error `-32601 Method not found` from our side (agent-initiated extension requests). User-visible messaging tracked in open question §12.4 of the spec.

**Acceptance**:
- Integration test green when `cursor-agent` logged in
- Commit: `acp(client): T10 — cursor adapter over ACP`

---

## T11 — `live_controls` unification + fork/rollback remap

**Scope**:

1. `src-bridge/src/runtime/agent-runtime.ts:135-143` — drop the `runtime === "claude_code" && this.claudeQuery` gate. Populate `live_controls` for any runtime that has an active `AcpClient`. Capability-gate individual fields (`set_thinking_budget` only when agent advertises support).
2. `AcpAdapter.fork` / `rollback` / `revert` — implement replay-via-prompt strategy per spec §7.2 for adapters where `agentCapabilities.loadSession === true`; otherwise return `UnsupportedOperationError` (same as today for non-Claude).
3. Remove `AgentRuntime.claudeQuery` field **only if** no legacy path reads it (the legacy flag-off path still does). Instead: rename to `legacyClaudeQuery` and narrow scope to legacy runtime; ACP path never sets it.

**Tests**:
- Unit: `live_controls` populated for all four adapters when flag on
- Unit: `set_thinking_budget` absent when agent doesn't advertise
- Integration: fork against claude_code reproduces prior conversation up to target message

**Acceptance**:
- All adapter types see unified `live_controls` in tests
- Commit: `acp(client): T11 — unify live_controls and remap fork/rollback`

---

## T12 — Cross-layer audit (Go + frontend)

**Scope**: Grep Go and frontend for any remaining `runtime === "claude_code"` gates or assumptions about which events originate from which adapter. Document findings.

**Steps**:
1. `rg 'claude_code' src-go/` — for each hit, determine whether the Go code should now be adapter-agnostic.
2. `rg 'claude_code' app/ components/ lib/` — same for frontend.
3. File an issue comment block at the end of the spec §12 if any non-trivial changes needed; otherwise write a one-paragraph "clean" note.

**Deliverables**:
- `docs/superpowers/specs/2026-04-16-bridge-acp-client-integration/audit-notes.md` — findings
- No code changes in this task unless trivial

**Acceptance**:
- Audit notes file committed
- Commit: `acp(client): T12 — Go and frontend audit notes`

---

## T13 — Legacy path cleanup (guarded)

**Scope**: Do NOT delete legacy files yet. Instead:
- Add `// DEPRECATED: scheduled for removal after ACP rollout, see spec §11.1` header comments to the deletion-list files (spec §11.1).
- Add a CI check (script) that warns if any file in the deletion list is modified without a spec-link reference.

**Rationale**: The actual deletion happens after real-world validation (not this plan). We mark intent now so no one accidentally invests in those files.

**Deliverables**:
- Header comments on deprecated files
- `scripts/check-deprecated-legacy.mjs` and a doc pointer in `src-bridge/README.md`

**Acceptance**:
- CI script runs on `pnpm lint` pre-push (or documented standalone)
- Commit: `acp(client): T13 — mark legacy adapter paths as deprecated`

---

## T14 — Docs + spec changelog

**Scope**:
- `CLAUDE.md` (project root) — add one paragraph under "Architecture → Bridge Structure" explaining the ACP client + flag mechanism.
- `src-bridge/README.md` — full section on running the bridge in ACP mode, env flags, required binaries, troubleshooting.
- `docs/superpowers/specs/2026-04-16-bridge-acp-client-integration.md` §14 — bump to draft-2 with summary of implementation notes learned (adapter-specific quirks discovered during T7–T10).

**Deliverables**:
- Doc updates as listed
- Spec bump to draft-2

**Acceptance**:
- Rendered docs readable, link-checked
- Commit: `acp(client): T14 — docs and spec changelog draft-2`

---

## Open questions that may affect execution

1. **ACP SDK version pin** (T1): confirm latest stable published version of `@zed-industries/agent-client-protocol` at execution time; if >0.5.0 check for breaking changes vs acpx's usage.
2. **node-pty availability** (T4b): if `node-pty` is not in bridge deps, evaluate `Bun.spawn` PTY support or add a native dep (requires install-time build). Prefer `Bun.spawn` path if it supports PTY mode; defer to implementer judgment.
3. **Windows PTY** (T4b): `node-pty` on Windows uses ConPTY; verify our dev environment (Max Qian uses Windows per gitStatus). Codex integration tests may need Windows-specific skips.
4. **Cursor extension methods** (T10): if agent sends `cursor/*` requests during smoke test and we return `method not found`, does `cursor-agent` crash or degrade gracefully? Capture observed behavior in T10 report.
5. **Process pooling** (spec §12.2): not in plan scope; if a task reveals a hot-start requirement, file a follow-up spec.

---

## Changelog

- **2026-04-16** — v1 written after pivot from capability-universe spec. Aligned with spec `2026-04-16-bridge-acp-client-integration.md` draft-1.
