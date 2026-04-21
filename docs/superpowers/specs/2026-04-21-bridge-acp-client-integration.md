# Bridge ACP Client Integration — Spec

- **ID**: `2026-04-21-bridge-acp-client-integration`
- **Author**: Claude (作业编排) + Max Qian
- **Status**: final
- **Scope**: `src-bridge/` runtime layer; `src-go/` IM forwarding observer (root-task rollup) and `internal/repository/task_repo` ancestor lookup; `src-bridge/src/runtime/agent-runtime.ts` `live_controls` unification
- **Evidence snapshot date**: 2026-04-21
- **Supersedes**: `docs/dev/specs/2026-04-16-bridge-acp-client-integration.md` (draft-2). The old spec remains as historical reference only; this document is the sole source of truth going forward.
- **Depends on**: none. Consumers of `/bridge/*` (Go `internal/bridge/client.go`, frontend stores) observe zero breaking change.

## 1. Background

`src-bridge/` today speaks seven different dialects to seven agent runtimes:

| Adapter | Transport | Current entry file |
|---|---|---|
| `claude_code` | Anthropic Agent SDK in-proc | `src-bridge/src/handlers/claude-runtime.ts` |
| `codex` | Codex CLI subprocess (custom protocol) | `src-bridge/src/handlers/codex-runtime.ts` |
| `opencode` | HTTP to OpenCode server | `src-bridge/src/handlers/opencode-runtime.ts` |
| `cursor` | stdio subprocess (JSON stream) | `src-bridge/src/handlers/command-runtime.ts` (cursor branch) |
| `gemini` | stdio subprocess (JSON stream) | `src-bridge/src/handlers/command-runtime.ts` (gemini branch) |
| `qoder` | stdio subprocess | `src-bridge/src/handlers/command-runtime.ts` (qoder branch) |
| `iflow` | stdio subprocess | `src-bridge/src/handlers/command-runtime.ts` (iflow branch) |

Each adapter re-implements message framing, streaming event translation, tool-call plumbing, permission prompting, and process supervision. Four concrete consequences motivate this spec:

1. **Claude-specific leak.** `agent-runtime.ts:135–143` hard-codes `runtime === "claude_code"` when exposing `live_controls`. `setModel` / `setThinkingBudget` / `mcpServerStatus` / `rewindFiles` only work for Claude because only Claude exposes a `ClaudeQueryControl` handle.
2. **Capability divergence.** `cursor / gemini / qoder / iflow` (via `command-runtime.ts`) throw `UnsupportedOperationError` for `fork / rollback / revert / getMessages / getDiff / executeCommand / executeShell / setThinkingBudget / getMcpServerStatus / interrupt / setModel` — the bridge silently downgrades most features.
3. **Five translation layers** converting adapter-native events into `AgentEventType`. Each one is a bug surface.
4. **Ecosystem alignment drift.** Five target adapters (`claude_code / codex / opencode / cursor / gemini`) already ship maintained ACP wrappers, and the official ACP TypeScript SDK implements JSON-RPC + NDJSON + typed `Agent`/`Client` interfaces. We pay the cost of maintaining five fork-specific pipelines against zero upstream reuse.

**Current scaffold state (2026-04-21 audit).** The directory `src-bridge/src/runtime/acp/` contains draft-1-era placeholders:

- `transport.ts` / `process.ts` / `session.ts` / `handlers/{fs,terminal,permission}.ts` / `events/session-update.ts` are all 3-line `// Placeholder for T<N>` + `export {}` stubs. `registry.ts` (50 LOC) lists spawn commands but is not referenced from `runtime/registry.ts`. `errors.ts` (65 LOC) defines six error classes only.
- The `@agentclientprotocol/sdk` package is **not** in `src-bridge/package.json` despite draft-2 claiming it was "installed".
- No imports of the SDK exist anywhere in the codebase.
- Nothing in `runtime/registry.ts` reaches the ACP module — the scaffold is unreachable dead code from commit `37c5a62` (2026-04-16).

This spec treats the existing scaffold as a starting point, not as completed work. T1 rescaffolds into the layout below.

## 2. Goals and non-goals

### Goals

1. Bridge becomes an **ACP client** that drives five target adapters (`claude_code / codex / opencode / cursor / gemini`) over stdio JSON-RPC using the **official `@agentclientprotocol/sdk`**. No DIY transport, no DIY `Agent` / `Client` interface, no DIY schema types.
2. Reuse shipped ACP agent wrappers — no DIY ACP agent implementations on our side.
3. The 12-method `RuntimeAdapter` surface and all `/bridge/*` HTTP routes stay **stable at the boundary**. `src-go/internal/bridge/client.go` does not change. Frontend observes only feature **gains** (`fork` / `rollback` / `setModel` / `setThinkingBudget` / `mcpServerStatus` / `interrupt` / `executeCommand` / `executeShell` / `getMessages` / `getDiff` start working for non-Claude adapters).
4. All five target adapters gain uniform support for: cancel, `setMode`, `setModel`, `setConfigOption`, permission dialog, structured tool-call streaming, `fs.read` / `fs.write`, terminal.
5. **Unstable ACP methods** (`unstable_forkSession` / `unstable_resumeSession` / `unstable_closeSession` / `unstable_setSessionModel` / `unstable_logout`, NES family, document lifecycle, `unstable_createElicitation`) are supported via **capability-gated passthrough** — the SDK exposes them; we wire them to `AcpSession` and fail with structured `AcpCapabilityUnsupported` when an agent does not advertise the capability. Frontend UI for these is out of scope.
6. **Adapter-level process pooling**: one child process per adapter shared across tasks using ACP's native `session/new` multi-session support. Per-task state lives on the `AcpSession` wrapper; the child lives on `AcpConnectionPool`.
7. Legacy Claude-specific code paths inside the bridge are retired. `live_controls` becomes adapter-agnostic, driven by `capabilities` cached on `PooledEntry`.
8. **Two-phase rollout** (tightened from draft-2's three-phase): implementation period is flag-gated per adapter (`BRIDGE_ACP_<ADAPTER>` defaults ON; `=0` emergency fallback to legacy). Once T7 smoke is green for all five adapters, **the same PR** deletes legacy handlers, drops the flag, and removes the Claude `live_controls` gate. No release-cycle buffer.
9. **End-to-end IM path preserved**: `@runtime <prompt>` in IM → Go task dispatch → Bridge ACP → `session/update` stream → events fan out (persistence, frontend WS, IM forward with root-task rollup for sub-agent tasks). Background tasks continue after IM client disconnects.

### Non-goals

- Bridge as an ACP **agent** (server). Zed-side integration is a future spec.
- Migrating `qoder` / `iflow` — they stay on `command-runtime.ts` until upstream ACP wrappers exist. `command-runtime.ts` loses its `cursor` and `gemini` branches in this spec (§12.2).
- Rewriting the WS event contract to Go. We map ACP events to the current `AgentEventType` set; new event types are additive (`status_change.kind="acp_passthrough"` fallback for unknown subtypes).
- New MCP runtime. We continue using `src-bridge/src/mcp/client-hub.ts` for legacy in-proc MCP; ACP agents get MCP server configs pushed through `session/new.mcpServers` — the agent talks to MCP directly.
- Frontend UI for mode / available commands / elicitation / NES. The wire is connected; visualization is a later spec.
- Persistence of `allow_always` / `reject_always` permission choices (treated as `_once` this phase).
- Per-adapter Cursor extensions (`cursor/ask_question` etc.). `extMethod` passthrough is wired; UI mapping is a later spec.
- Non-ACP cleanup items identified during the 2026-04-21 audit (cost duplication, review orchestration split, HTTP MCP transport stub, role injector coverage, session snapshot recovery expansion, Bridge test gap fills). See §15.

## 3. Architecture overview

### 3.1 End-to-end flow

```
┌──────────────────────┐
│ IM platforms         │
│ (feishu / dingtalk / │
│  slack / telegram /  │
│  discord / wecom /   │
│  qq / qqbot / ...)   │
└──────┬───────────────┘
       │ @claude <prompt>   (directRuntimeMentions → runtime id)
       ▼
┌──────────────────────┐
│ src-im-bridge        │
│   core/engine        │
└──────┬───────────────┘
       │ POST /api/v1/im/command { runtime, prompt, reply_target, bridge_id }
       ▼
┌─────────────────────────────────────────┐
│ src-go (Go orchestrator)                │
│   task_dispatch_service                 │
│     ├─ Create Task { runtime, provider, │
│     │   im_reply_target, parent_id? }   │
│     └─ eventbus: task.dispatch          │
│   workflow/nodetypes/llm_agent          │──► may spawn child task (ParentID=root)
│   agent_service                         │
│     ├─ POST /bridge/execute             │
│     └─ /bridge/stream (WS)              │
│   im_forward observer                   │
│     └─ GetAncestorRoot(taskID)          │──► root task's im_reply_target
└──────┬──────────────────────────────────┘
       │ POST /bridge/execute { task_id, runtime, prompt, ... }
       ▼
┌─────────────────────────────────────────────┐
│ src-bridge (Hono)                           │
│   server/routes → runtime/registry.ts        │
│     createAcpRuntimeAdapter(adapterId)       │
│                                             │
│   runtime/acp/                               │
│     AcpConnectionPool (per-adapter)          │
│       └─ ChildProcessHost                    │
│           + ClientSideConnection (SDK)       │
│                                             │
│     AcpSession (per task_id, per sessionId)  │
│       └─ prompt / cancel / setMode / ...     │
│       └─ unstable_* (capability-gated)       │
│                                             │
│     MultiplexedClient                        │
│       Map<SessionId, PerSessionContext>      │
│         ├─ FsSandbox (worktree-rooted)       │
│         ├─ TerminalManager (pty pool)        │
│         ├─ PermissionRouter                  │
│         └─ EventStreamer (per-task)          │
└──────┬──────────────────────────────────────┘
       │ NDJSON stdio (SDK ndJsonStream)
       ▼
┌─────────────────────────────────────────┐
│ ACP agent child (pooled; one-per-adapter)│
│   @agentclientprotocol/claude-agent-acp │
│   @zed-industries/codex-acp             │
│   opencode acp                          │
│   cursor-agent acp                      │
│   gemini --experimental-acp | acp       │ ← T0 verifies exact flag
│                                         │
│   (agent-internal sub-agents surface as │
│    tool_call + plan in session/update)  │
└─────────────────────────────────────────┘

  Reply path · incremental streaming + terminal state:
    session/update  ──► events/session-update.ts mapping
                   ──► EventStreamer.emit(AgentEvent)
                   ──► WS /bridge/stream
                   ──► Go ws.Hub / eventbus
                         ├─ task_events / task_comments persistence
                         ├─ frontend WS push
                         └─ im_forward (root-task rollup)
                              └─ IM platform reply/card update
```

### 3.2 Key shifts from the existing scaffold (draft-1 layout)

| Concern | Existing scaffold (draft-1 placeholders) | This spec |
|---|---|---|
| JSON-RPC + NDJSON transport | `transport.ts` placeholder (DIY plan) | **Delete.** Use SDK's `ndJsonStream()` + `ClientSideConnection`. |
| `Agent` RPC surface | planned DIY `AcpClient` class | Directly use SDK `ClientSideConnection` methods. |
| `Client` handler interface | (not defined) | Implement SDK's exported `Client` interface in `MultiplexedClient`. |
| Per-process scope | (not defined) | **One** `ChildProcessHost` + **one** `ClientSideConnection` per adapter, pooled. |
| Per-task scope | `session.ts` placeholder (mixed concerns) | `AcpSession` wraps `(PooledEntry, SessionId)`; per-task state lives on `PerSessionContext`. |
| Input/output routing | (not defined) | `MultiplexedClient` dispatches inbound `Client` calls by `params.sessionId` to the right `PerSessionContext`. |
| Unstable methods | (not defined) | Exposed on `AcpSession`; capability-gated with `AcpCapabilityUnsupported`. |
| Adapter coverage | 4 listed in `acp/registry.ts` (cc / codex / opencode / cursor) | 5 (cc / codex / opencode / cursor / gemini). |
| Feature flag | (not defined) | `BRIDGE_ACP_<ADAPTER>=0` as emergency fallback during implementation period only; removed at T7 PR. |
| IM sub-agent rollup | (not addressed) | Go-side `im_forward` resolves ancestor root; child tasks fold into root's `im_reply_target`. |

## 4. Module design

### 4.1 Target layout (post-T1)

```
src-bridge/src/runtime/acp/
├─ index.ts              barrel export
├─ registry.ts           ACP_ADAPTERS[5] (spawn command + envRequired + cursorExtensions)
├─ errors.ts             6 existing classes + AcpCapabilityUnsupported + AcpAuthMissing + AcpCommandNotFound
├─ process-host.ts       ChildProcessHost — spawn / stderr ring-buffer / graceful shutdown
├─ connection-pool.ts    AcpConnectionPool — per-adapter singleton (host + SDK connection)
├─ multiplexed-client.ts MultiplexedClient — implements SDK Client; dispatches by sessionId
├─ session.ts            AcpSession — per-(task_id, sessionId) public surface
├─ capabilities.ts       capability gates + AgentCapabilities cache helpers
├─ adapter-factory.ts    createAcpRuntimeAdapter(adapterId) — returned by runtime/registry factories
├─ handlers/
│   ├─ fs.ts             readTextFile / writeTextFile via FsSandbox
│   ├─ terminal.ts       6 terminal methods via TerminalManager
│   ├─ permission.ts     requestPermission via HookCallbackManager
│   └─ elicitation.ts    unstable_createElicitation / unstable_completeElicitation passthrough
└─ events/
    └─ session-update.ts sessionUpdate → AgentEventType mapping (single source of truth)
```

**T1 concrete changes from the 2026-04-21 scaffold state**:
- **Delete** `transport.ts` (SDK replaces it).
- **Rename** `process.ts` → `process-host.ts`; rewrite per §4.2.
- **Rewrite** `session.ts` per §4.5 (pure per-task wrapper, no transport concerns).
- **Add** `connection-pool.ts`, `multiplexed-client.ts`, `capabilities.ts`, `adapter-factory.ts`, `index.ts`, `handlers/elicitation.ts`.
- **Extend** `errors.ts` with `AcpCapabilityUnsupported`, `AcpAuthMissing`, `AcpCommandNotFound`.
- **Keep** `registry.ts` (5-adapter map stays; gemini args finalized at T0).
- **Keep** existing `handlers/{fs,terminal,permission}.ts` file paths; rewrite bodies per §6.
- **Keep** `events/session-update.ts` path; rewrite per §8.
- **Add** `@agentclientprotocol/sdk` to `src-bridge/package.json` (pin to the version available on npm at T1; current intent is `^0.19.0`, verified live).

### 4.2 `ChildProcessHost` (`process-host.ts`)

```ts
export interface ChildProcessHostOptions {
  adapterId: AdapterId;
  command: string;
  args: readonly string[];
  env: Record<string, string>;
  logger: Logger;
}

export class ChildProcessHost {
  constructor(opts: ChildProcessHostOptions);
  start(): Promise<{
    stdin: WritableStream<Uint8Array>;
    stdout: ReadableStream<Uint8Array>;
  }>;
  readonly stderrBuffer: RingBuffer;        // trailing 8 KB
  readonly exited: Promise<number>;          // resolves to exit code
  shutdown(gracefulMs?: number): Promise<void>; // close stdin → wait → SIGTERM → SIGKILL
}
```

Single responsibility: spawn, env merge, stderr ring-buffer, graceful shutdown. Does not touch JSON-RPC.

### 4.3 `AcpConnectionPool` (`connection-pool.ts`)

Singleton at bridge-process scope. One `PooledEntry` per `AdapterId`.

```ts
export class AcpConnectionPool {
  constructor(opts: { logger: Logger; idleMs?: number /* default 600_000 */ });
  acquire(adapterId: AdapterId, ctx: AcquireContext): Promise<PooledEntry>;
  release(adapterId: AdapterId, sessionId: SessionId): Promise<void>;
  shutdownAll(graceful?: boolean): Promise<void>;
}

interface PooledEntry {
  host: ChildProcessHost;
  conn: ClientSideConnection;           // SDK
  caps: schema.AgentCapabilities;        // cached from initialize response
  clientDispatcher: MultiplexedClient;   // the Client impl the SDK calls into
  sessions: Set<SessionId>;              // ref count for idle reclaim
  restartPending: boolean;
}

interface AcquireContext {
  registerSession(sessionId: SessionId, ctx: PerSessionContext): void;
  unregisterSession(sessionId: SessionId): void;
}
```

- `acquire` serializes per-adapter via a mutex to prevent double-spawn under concurrent first-use.
- If `restartPending` is true or `host.exited` has settled, spawn a fresh host and re-initialize (new `ClientSideConnection`).
- `release` decrements `sessions`; when empty and idle > `idleMs`, schedule shutdown.
- `shutdownAll` is called on bridge process shutdown.

### 4.4 `MultiplexedClient` (`multiplexed-client.ts`)

Implements SDK's exported `Client` interface. Single instance per `PooledEntry`, shared across all sessions on that pool entry.

```ts
export interface PerSessionContext {
  taskId: string;
  cwd: string;
  fsSandbox: FsSandbox;
  terminalManager: TerminalManager;
  permissionRouter: PermissionRouter;
  streamer: EventStreamer;
  logger: Logger;
}

export class MultiplexedClient implements Client {
  private sessions = new Map<SessionId, PerSessionContext>();

  register(sessionId: SessionId, ctx: PerSessionContext): void;
  unregister(sessionId: SessionId): void;

  // Client impl (dispatches by params.sessionId to sessions.get(sid)):
  async sessionUpdate(params): Promise<void>;
  async requestPermission(params): Promise<schema.RequestPermissionResponse>;
  async readTextFile(params): Promise<schema.ReadTextFileResponse>;
  async writeTextFile(params): Promise<schema.WriteTextFileResponse>;
  async createTerminal(params): Promise<schema.CreateTerminalResponse>;
  async terminalOutput(params): Promise<schema.TerminalOutputResponse>;
  async waitForTerminalExit(params): Promise<schema.WaitForTerminalExitResponse>;
  async killTerminal(params): Promise<schema.KillTerminalResponse | void>;
  async releaseTerminal(params): Promise<schema.ReleaseTerminalResponse | void>;
  async unstable_createElicitation?(params): Promise<schema.CreateElicitationResponse>;
  async unstable_completeElicitation?(params): Promise<void>;
  async extMethod?(method, params): Promise<Record<string, unknown>>;
  async extNotification?(method, params): Promise<void>;
}
```

- Unknown `params.sessionId` rejects with `RequestError(-32602, { reason: "unknown_session" })`.
- Per-method logic delegates to a handler module under `handlers/`.
- For `sessionUpdate` (notification), errors are logged and swallowed — we cannot respond with a JSON-RPC error (notifications are one-way) and we MUST keep receiving subsequent updates per ACP cancellation semantics.

### 4.5 `AcpSession` (`session.ts`)

Per-(task, session) public surface. What `adapter-factory.ts` holds and maps to the `RuntimeAdapter` 12-method face.

```ts
export interface AcpSessionOptions {
  taskId: string;
  adapterId: AdapterId;
  cwd: string;
  streamer: EventStreamer;
  permissionRouter: PermissionRouter;
  fsSandbox: FsSandbox;
  terminalManager: TerminalManager;
  mcpServers: schema.McpServer[];
  logger: Logger;
}

export class AcpSession {
  static async open(pool: AcpConnectionPool, opts: AcpSessionOptions): Promise<AcpSession>;

  readonly sessionId: schema.SessionId;
  readonly capabilities: schema.AgentCapabilities;

  // stable
  prompt(content: schema.ContentBlock[]): Promise<schema.StopReason>;
  cancel(): Promise<void>;
  setMode(modeId: string): Promise<void>;
  setConfigOption(key: string, value: unknown): Promise<schema.SetSessionConfigOptionResponse>;
  authenticate(methodId: string): Promise<void>;

  // unstable (capability-gated; throws AcpCapabilityUnsupported otherwise)
  forkSession(): Promise<AcpSession>;
  resumeSession(): Promise<void>;
  setModel(modelId: string): Promise<void>;
  closeSession(): Promise<void>;
  logout(): Promise<void>;                  // unstable_logout
  startNes(params): Promise<schema.StartNesResponse>;
  suggestNes(params): Promise<schema.SuggestNesResponse>;
  closeNes(): Promise<void>;
  didOpenDocument(params): Promise<void>;
  didChangeDocument(params): Promise<void>;
  didCloseDocument(params): Promise<void>;
  didSaveDocument(params): Promise<void>;
  didFocusDocument(params): Promise<void>;
  acceptNes(params): Promise<void>;
  rejectNes(params): Promise<void>;
  extMethod(method: string, params: Record<string, unknown>): Promise<Record<string, unknown>>;
  extNotification(method: string, params: Record<string, unknown>): Promise<void>;

  // lifecycle
  dispose(): Promise<void>;  // pool.release + unregister in MultiplexedClient (idempotent)
}
```

Invariants:
- One `AcpSession` wraps exactly one `(pooledEntry, sessionId)` pair.
- `prompt` serializes per session; concurrent call rejects immediately with `AcpConcurrentPrompt`.
- All capability checks read `this.capabilities` (cached on `AcpSession.open`). No live re-query.
- `dispose` is idempotent; double-dispose is a no-op with a debug log.

### 4.6 `adapter-factory.ts`

```ts
export function createAcpRuntimeAdapter(adapterId: AdapterId): RuntimeAdapterFactory {
  return async (task, streamer, deps) => {
    if (process.env[`BRIDGE_ACP_${adapterId.toUpperCase()}`] === "0") {
      return deps.legacyFactories[adapterId](task, streamer, deps); // emergency fallback (impl period only)
    }
    const session = await AcpSession.open(deps.pool, {
      taskId: task.id,
      adapterId,
      cwd: task.worktreeRoot,
      streamer,
      permissionRouter: deps.permissionRouter,
      fsSandbox: deps.makeFsSandbox(task.worktreeRoot),
      terminalManager: deps.terminalManager,
      mcpServers: deps.resolveMcpServersFor(task),
      logger: deps.logger.child({ taskId: task.id, adapterId }),
    });

    return {
      execute:          (req) => session.prompt(toContentBlocks(req)).then(mapStopReason),
      cancel:           ()    => session.cancel(),
      interrupt:        ()    => session.cancel(),                 // alias
      setModel:         (m)   => session.setModel(m),              // throws AcpCapabilityUnsupported if unsupported
      setMode:          (m)   => session.setMode(m),
      setConfigOption:  (k, v)=> session.setConfigOption(k, v),
      setThinkingBudget:(n)   =>
        session.capabilities.promptCapabilities?.thinkingBudget
          ? session.setConfigOption("thinking_budget", n)
          : throwUnsupported("setThinkingBudget"),
      fork:             ()    => session.forkSession().then(wrapAsAdapter),
      rollback:         (a)   => rollbackViaReplay(session, a),    // uses loadSession if capable
      revert:           (a)   => deps.worktreeService.revert(a),   // not via ACP
      getMessages:      ()    => deps.taskEventsService.messages(task.id), // not via ACP
      getDiff:          (a)   => deps.worktreeService.diff(a),
      executeCommand:   (c)   => session.extMethod("agent/executeCommand", { command: c })
                                   .catch(() => session.prompt([{ type:"text", text:`/run ${c}` }])),
                                   // best-effort: prefer agent extension; fall back to slash-command prompt
      executeShell:     (c)   => deps.terminalManager.runOneShot(task.id, c),
      getMcpServerStatus:()   => deps.mcpServersStatus(task),
      dispose:          ()    => session.dispose(),
    };
  };
}
```

`live_controls` (at `agent-runtime.ts:135-143`) drops the `runtime === "claude_code"` gate. Fields populate from `session.capabilities`:

- `setModel`: populated if `capabilities.availableModels` is non-empty **and** either `setSessionConfigOption("model", ...)` or `unstable_setSessionModel` is advertised.
- `setThinkingBudget`: populated if `promptCapabilities.thinkingBudget === true`.
- `setMode`: populated if `availableModes` is non-empty.
- `mcpServerStatus`: populated if `mcpCapabilities.http || mcpCapabilities.sse`.
- `rewindFiles`: maps to `revert` (worktree service, not ACP).

## 5. Adapter registry

### 5.1 Target commands (5 adapters)

```ts
export const ACP_ADAPTERS = {
  claude_code: {
    command: "npx",
    args: ["-y", "@agentclientprotocol/claude-agent-acp@^0.28.0"],
    envRequired: ["ANTHROPIC_API_KEY"],
    cursorExtensions: false,
  },
  codex: {
    command: "npx",
    args: ["-y", "@zed-industries/codex-acp@^0.11.1"],
    envRequired: ["OPENAI_API_KEY"],
    cursorExtensions: false,
  },
  opencode: {
    command: "opencode",
    args: ["acp"],
    envRequired: [],
    cursorExtensions: false,
  },
  cursor: {
    command: "cursor-agent",
    args: ["acp"],
    envRequired: [],
    cursorExtensions: true,
  },
  gemini: {
    command: "gemini",
    args: ["--experimental-acp"],        // T0 verifies; may become ["acp"] if CLI stabilized
    envRequired: [],                     // gemini handles own auth
    cursorExtensions: false,
  },
} as const satisfies Record<AdapterId, AcpAdapterConfig>;
```

**T0 verification**: the exact flag/subcommand for `gemini` may be `acp` rather than `--experimental-acp`. Pin the correct form during implementation before shipping; record decision in this spec's changelog.

### 5.2 Deprecation caveat

The old `@zed-industries/claude-code-acp` package is deprecated and re-routed to `@agentclientprotocol/claude-agent-acp`. We pin the new name.

### 5.3 Auth passthrough

- **claude_code**: `ANTHROPIC_API_KEY` (or Claude Console login) read from bridge env; forwarded unchanged.
- **codex**: `@zed-industries/codex-acp` shells out to `codex` CLI; CLI's own auth applies (API key env var or OAuth via `codex login`).
- **opencode**: existing `/bridge/opencode/provider-auth/*` routes stay; spawned `opencode acp` picks up on-disk auth.
- **cursor**: `cursor-agent` refuses to start if not logged in; bridge surfaces `AcpAuthMissing(cursor, [])` on spawn failure — mapped to existing `AUTH_REQUIRED` structured event.
- **gemini**: `gemini` CLI maintains its own auth; bridge forwards env unchanged.

Missing `envRequired` variables short-circuit in `pool.acquire` with `AcpAuthMissing(adapterId, missingEnv)` — no spawn attempt.

### 5.4 Capability flags

During `initialize` we declare:

```ts
clientCapabilities: {
  fs: { readTextFile: true, writeTextFile: true },
  terminal: true,
}
```

Agent replies with its own `agentCapabilities`. Cached on `PooledEntry.caps` and re-used by every `AcpSession.open` on that pool entry.

## 6. Client-side handlers

### 6.1 `fs/*` handler (`handlers/fs.ts`)

```ts
export function readTextFile(ctx: PerSessionContext, params: schema.ReadTextFileRequest): Promise<schema.ReadTextFileResponse>;
export function writeTextFile(ctx: PerSessionContext, params: schema.WriteTextFileRequest): Promise<schema.WriteTextFileResponse>;
```

**Sandbox rule**: paths escaping the session's worktree `cwd` (after `realpath`) reject with `RequestError(-32602, { reason: "path_escapes_worktree" })`. No fallback to the outer filesystem. `line + limit` are 1-based line numbers, limit in lines, per ACP schema. Binary writes are out of scope this phase.

### 6.2 `terminal/*` handler (`handlers/terminal.ts`)

Reuses `TerminalManager` (PTY pool). `node-pty` on desktop (Tauri sidecar); `Bun.spawn` fallback in web dev.

Capacity: per-task default `outputByteLimit = 10 MB`; global `maxConcurrentTerminals = 16`. Over-capacity allocation rejects with `RequestError(-32000, { reason: "terminal_capacity" })`.

Lifecycle maps 1:1 to ACP `createTerminal` / `terminalOutput` / `waitForTerminalExit` / `killTerminal` / `releaseTerminal`. `killTerminal` keeps id valid; `releaseTerminal` frees the slot and invalidates the id. Tool calls referencing a released id keep their last-known output snapshot.

### 6.3 `session/request_permission` handler (`handlers/permission.ts`)

Reuses existing `HookCallbackManager` and `/bridge/permission-response/:request_id` route — no new frontend surface.

Flow:
1. Agent sends `session/request_permission { sessionId, toolCall, options }`.
2. `PermissionRouter.request(taskId, toolCall, options)` generates a `request_id`, emits `permission_request` event through `EventStreamer`, registers resolver in `HookCallbackManager`.
3. Frontend / IM → Go → `POST /bridge/permission-response/:request_id` with `{ option_id }` or `{ cancelled: true }`.
4. Handler resolves pending promise; router returns `{ outcome: "selected", optionId }` or `{ outcome: "cancelled" }`.

Timeout: 30 min no-response → auto-cancel (`{ outcome: "cancelled" }` returned to agent; `permission_timeout` event emitted). `allow_always` / `reject_always` currently treated as `_once` (no persistence this phase).

### 6.4 `unstable_createElicitation` handler (`handlers/elicitation.ts`)

Capability-gated passthrough. Reuses `PermissionRouter`'s `request_id` + `HookCallbackManager` pattern, but event subtype is `elicitation_request` and response shape is a schema-typed payload rather than an option id.

Frontend has no UI this phase; bridge registers the `unstable_createElicitation` method on `MultiplexedClient` unconditionally (SDK `Client` interface marks it optional — agents that do not support elicitation simply never call it). If invoked and the session's `ctx.permissionRouter` is unavailable (unexpected state), resolve with `{ action: "cancel" }` per schema rather than throwing.

## 7. Session lifecycle

### 7.1 Per-task state machine

```
new task → AcpSession.open:
            pool.acquire(adapterId, ctx)
            → on pool miss: ChildProcessHost.start + ndJsonStream + new ClientSideConnection
            → conn.initialize (first session on this host only)
            → conn.newSession({ cwd, mcpServers })
            → MultiplexedClient.register(sessionId, perSessionContext)
            → returns AcpSession
        ↓
     (prompt | setMode | setConfigOption | unstable_*)* → dispose
             ↓
       cancel → await stopReason=cancelled (2 s limit → AcpCancelTimeout)
```

- Prompt turns serialize per session (ACP disallows concurrent prompts on the same session).
- `/bridge/cancel` → `AcpSession.cancel()` → SDK `conn.cancel({sessionId})`; awaits current prompt's `stopReason=cancelled`.
- Task archival / terminal state triggers `AcpSession.dispose()` → `pool.release(adapterId, sessionId)` → if ref count hits zero and idle > `POOL_IDLE_MS`, `ChildProcessHost.shutdown()`.

### 7.2 Fork / rollback / revert mapping

- **fork**:
  1. If `capabilities.session.fork === true` → `session.forkSession()` → SDK `unstable_forkSession` → new `AcpSession`.
  2. Else if `capabilities.loadSession === true` → fresh `newSession` + replay prior user messages via successive `prompt` calls up to the ancestor point.
  3. Else → reject with `UnsupportedOperationError` (no regression vs legacy).
- **rollback**: same three-tier ladder (fork-if-capable → loadSession replay → reject).
- **revert**: file-level revert handled by worktree service, not ACP. Unchanged.

### 7.3 `live_controls` unification

Drop the `runtime === "claude_code"` gate at `agent-runtime.ts:135-143`. Replace with capability-driven population as described in §4.6. Frontend already tolerates field absence.

### 7.4 Sub-agents

Two distinct concepts, handled separately:

- **Agent-internal sub-agents** (e.g., Claude's Task tool, Codex internal planner): represented in ACP as nested `tool_call` / `tool_call_update` / `plan` updates within the parent session's `session/update` stream. Bridge does nothing special — they flow through `session-update.ts` mapping like any other tool call.
- **Orchestrator-level child tasks** (e.g., workflow `llm_agent` node spawning a new `Task` with `ParentID` set): each child task calls `/bridge/execute` independently. `pool.acquire(adapterId)` reuses the already-running host; `conn.newSession()` produces a new `sessionId`. The child task's `AcpSession` registers with the same `MultiplexedClient` under its own `sessionId`. Parent and child run concurrently on the pooled connection.

### 7.5 Background task continuity

- `AcpSession` lifetime = task lifetime. Independent of IM subscriptions.
- Bridge WS disconnect (frontend closed) does not affect Go's `/bridge/stream` subscription — Go is the event hub; frontend and IM are downstream.
- Task completion → Go `task_service.Complete` → `im_forward` observer reads `im_reply_target` from the **root task** (sub-agent rollup) → IM platform reply API posts result. User need not be online.

## 8. Streaming: `session/update` → `AgentEventType`

Single translation table in `runtime/acp/events/session-update.ts`. Stable variants:

| `SessionUpdate.sessionUpdate` | `AgentEventType` | Notes |
|---|---|---|
| `user_message_chunk` | `partial_message` | direction=user |
| `agent_message_chunk` | `output` | text delta |
| `agent_thought_chunk` | `reasoning` | |
| `tool_call` | `tool_call` | |
| `tool_call_update` (pending/in_progress) | `tool.status_change` | |
| `tool_call_update` (completed/failed) | `tool_result` | preserves frontend expectation |
| `plan` | `todo_update` | |
| `available_commands_update` | `status_change{kind:"commands"}` | additive |
| `current_mode_update` | `status_change{kind:"mode"}` | additive |
| `config_option_update` | `status_change{kind:"config_option"}` | additive |

Unstable / extension variants (capability-gated; wire only, no frontend work this phase):

| Unstable | Mapping |
|---|---|
| NES lifecycle events | `status_change{kind:"nes", subtype:<event>}` |
| document lifecycle (open/change/close/save/focus) | `status_change{kind:"document", subtype:<event>}` |
| `extMethod` / `extNotification` | `status_change{kind:"ext", method:<name>}` |
| Any unknown `sessionUpdate` variant | `status_change{kind:"acp_passthrough"}` + `metadata._raw = <original>` |

Permission requests flow through §6.3, not this table.

`_meta` on every variant is copied verbatim into `AgentEvent.metadata._meta` so downstream consumers can opt into richer fields without a schema change. Usage / cost: emit `cost_update` when `_meta.usage` is present. T7 integration tests produce the empirical `_meta.usage` shape per adapter; the resulting table is appended to this spec's appendix before T7 PR merges (see §14 Q5).

## 9. Cross-layer impact

### 9.1 Go orchestrator

- **Add `task_repo.GetAncestorRoot(taskID)`** (if not already equivalent): walks `ParentID` chain to the root task. Used by `im_forward`.
- **`im_forward` observer**: for every event, resolve `event.task_id → ancestor root → root.im_reply_target`. Sub-agent progress/results fold into the root's IM thread. Task with no `im_reply_target` (non-IM-originated) stays invisible to IM bridge as today.
- **New config `im_child_task_folding_mode`** (`nested | flat | frontend_only`): default `nested` for feishu / slack / telegram / discord; default `frontend_only` for QQ platforms that do not support nested cards; default `flat` for anything unidentified. Platform-specific defaults set in IM bridge manifest.
- **No other API changes.** `/bridge/*` routes and WS event shapes preserved.
- Observable: `live_controls` populates for non-Claude adapters. Go code already treats `live_controls` as present-or-absent — verified during T6.
- Cost events arrive for codex / opencode / cursor / gemini where previously only `claude_code` emitted. No downstream breakage expected.
- `src-go/internal/bridge/client.go` is **unchanged**.

### 9.2 Frontend

- No required change. New `status_change.kind` values (`mode / commands / config_option / nes / document / ext / acp_passthrough`) are backwards-compatible — existing handlers ignore unknown kinds.
- Optional follow-up (out of scope): UI for mode / available commands / elicitation / NES.

### 9.3 Tauri

- **Packaging policy**: Node.js 22+ must be in PATH at runtime. Documented as a desktop prerequisite in installation docs. **No** Node bundle / delayed download / agent binary prepackaging this phase — those are tracked as a separate "desktop distribution readiness" spec in §15.
- **Spawn failures**: `ChildProcessHost.start()` catches `ENOENT` on the adapter's executable and maps to `AcpCommandNotFound(adapterId, command, installHint)`. Hint URLs: Node → `https://nodejs.org/`; `cursor-agent` → `https://cursor.sh/`; `opencode` / `gemini` → respective project install docs. Surfaced to frontend via structured event → existing `AUTH_REQUIRED` adjacent channel.
- `@zed-industries/codex-acp` has platform-specific optional dependencies — Tauri build resolves correct native binary per target triple. Validated during T6 on Linux CI.
- `cursor-agent` / `opencode` / `gemini` are user-installed CLIs; `AcpCommandNotFound` gives the install hint in the failure event.

### 9.4 IM Bridge

- `directRuntimeMentions` already includes all 5 runtime mentions (`@claude` / `@claudecode` / `@claude-code`, `@codex`, `@opencode`, `@cursor`, `@gemini`). No change.
- Card renderers already handle `tool_call / tool_result / reasoning / output / permission_request / todo_update` (T8 verifies no regression). Additive `status_change{kind:...}` subtypes fall through the default-renderer branch.
- Sub-agent rollup: child task progress appears folded in the root's thread per platform-specific renderer. QQ (no nested cards) falls back to `frontend_only` folding (no IM message for child tasks; only root task emits IM updates).

## 10. Error taxonomy

| Error class | Trigger point | Maps to `AgentEventType` | Visible to Go / frontend |
|---|---|---|---|
| `AcpProtocolError` | SDK throws `RequestError` (JSON-RPC layer) | `error` + `metadata.class="AcpProtocolError"` | Yes |
| `AcpProcessCrash` | host exits unexpectedly (not via `dispose`) | `error` + `metadata.stderr_tail=<last 8KB>`; triggers `pool.restartPending` | Yes |
| `AcpCancelTimeout` | `cancel()` does not receive `stopReason=cancelled` within 2 s | `error`; session force-disposed | Yes |
| `AcpTransportClosed` | write to stdin fails because host already exited | `error`; converts to `AcpProcessCrash` flow | Yes |
| `AcpConcurrentPrompt` | same session receives concurrent `prompt` | `error`; does not interrupt current prompt | Yes |
| `AcpCapabilityUnsupported` | call to unstable method not advertised by agent | Equivalent to legacy `UnsupportedOperationError` | Yes (Go already tolerates) |
| `AcpAuthMissing` | `envRequired` missing / cursor not logged in | `AUTH_REQUIRED` (structured event, reused) | Yes |
| `AcpCommandNotFound` | `npx` / `node` / `cursor-agent` / `opencode` / `gemini` not in PATH | `AUTH_REQUIRED` variant + `metadata.install_hint=<URL>` | Yes (frontend / Tauri UI renders install hint) |

**Recovery strategies**:
- **Host crash**: `pool.restartPending=true`; next `acquire` spawns a fresh host. Current sessions receive `AcpProcessCrash`. Task-level retry is the upper layer's decision — bridge does not auto-retry.
- **Prompt mid-flight crash**: no auto-retry (side-effect-bearing `tool_call`s already emitted are not safely replayable). Task marked failed; upper layer decides re-dispatch.
- **Cancel timeout**: `AcpSession.dispose()` force-releases the pool slot and logs a warning; the underlying session may remain in an indeterminate state in the agent, but on the bridge side state is reset.

## 11. Testing strategy

### 11.1 Unit tests (`src-bridge/tests/unit/runtime/acp/`)

No external agent dependency. Gated by standard `bun test` / `pnpm test`.

- `process-host.test.ts` — spawn failure, stderr ring overflow, graceful shutdown escalation (close stdin → SIGTERM → SIGKILL) under timeouts.
- `connection-pool.test.ts` — concurrent `acquire` single-spawn, ref count, idle reclaim, crash → `restartPending` → respawn.
- `multiplexed-client.test.ts` — `sessionId` routing, unknown `sessionId` → `-32602`, idempotent register / unregister.
- `capability-gate.test.ts` — unstable method call without advertised capability → `AcpCapabilityUnsupported`.
- `fs-sandbox.test.ts` — `..` / symlink / absolute-path escape rejection.
- `permission-router.test.ts` — allow / reject / timeout / cancel race.
- `session-update-mapping.test.ts` — all 12 stable variants + 3+ unstable passthroughs + unknown-variant fallback.
- `session.test.ts` — concurrent prompt → `AcpConcurrentPrompt`; `dispose` idempotent; `cancel` 2 s timeout → `AcpCancelTimeout`.

### 11.2 Component tests (`src-bridge/tests/component/acp/`)

Uses `tests/fixtures/mock-acp-agent.ts` — a standalone Bun script implementing `AgentSideConnection` from the SDK, with configurable capabilities and trigger hooks for each client callback.

- `happy-path.test.ts` — initialize → newSession → prompt → text delta → tool_call → stopReason.
- `cancel-race.test.ts` — cancel before / during / after prompt.
- `pooling.test.ts` — two tasks share one host; stub crash → both sessions receive `AcpProcessCrash`.
- `multi-session-fs.test.ts` — two concurrent sessions route fs / terminal requests correctly.
- `permission-flow.test.ts` — mock agent triggers permission request → `HookCallbackManager` → `/bridge/permission-response/:id` → agent resumes.

### 11.3 Integration tests (`src-bridge/tests/integration/acp/<adapter>.test.ts`)

One file each for `claude_code` / `codex` / `opencode` / `cursor` / `gemini`. Requires real agents installed.

Each runs: smoke (`echo hello`) / cancel / fs read+write / terminal `echo pong` / permission round-trip.

Gated by `SKIP_ACP_INTEGRATION=1` (default ON in CI; OFF for T7 smoke gate and local verification).

**CI gating checkpoints**:
- T6 PR: Linux runner runs `bun install` to validate `@zed-industries/codex-acp` optional-dep resolution. Failure blocks merge.
- T7 PR: Linux runner runs integration suite with `SKIP_ACP_INTEGRATION=0` for at least `claude_code` and `codex`; cursor / gemini may require local-only verification depending on CI auth constraints (record decision in T7 commit message).

### 11.4 End-to-end smoke

`pnpm dev:backend:verify` gains a 5-adapter echo prompt. Each adapter receives `"echo hello"` through `/bridge/execute`; assertion is at least one `partial_message` + one `output` + terminal `stopReason=end_turn`. Not CI-gated (requires real agent installs). Serves as manual verification entry point for T7.

### 11.5 Type regression

- `bun run typecheck` (in `src-bridge/`) green on every task PR, starting from T1 (SDK install).
- `pnpm exec tsc --noEmit` (repo root) green on every task PR.
- `schema.*` coverage in `events/session-update.ts` is verified by `session-update-mapping.test.ts` being exhaustive over `schema.SessionUpdate` union members.

## 12. Migration plan

### 12.1 Rollout phases (two phases; simplified from draft-2's three)

**Phase 1 — Implementation (T2 through T6)**
- `BRIDGE_ACP_<ADAPTER>` default ON.
- `BRIDGE_ACP_<ADAPTER>=0` returns the legacy factory (emergency fallback, per-adapter granularity).
- Legacy handler files still exist and compile.
- Local developers and `dev:backend:verify` exercise ACP path by default.

**Phase 2 — T7 PR merge (single atomic migration)**
- All five adapters pass integration smoke with `SKIP_ACP_INTEGRATION=0`.
- **Same PR** executes §12.2 deletion list, removes `BRIDGE_ACP_*` env branches, drops the `agent-runtime.ts:135-143` Claude gate, updates docs / changelog / `.env.example`.
- `BRIDGE_ACP_<ADAPTER>=0` becomes a no-op with a deprecation log warning (kept for one cleanup commit to give any out-of-band consumers a clean signal; removed entirely at T10 housekeeping).

**Rationale for dropping draft-2's "release-cycle" buffer**: the project is in internal testing (per project memory: "breaking changes freely permitted"). A release-cycle guard for emergency rollback has no audience.

### 12.2 Deletion list (executed in T7 PR)

After T7 smoke green, in the same PR:
- `src-bridge/src/handlers/claude-runtime.ts` (full file)
- `src-bridge/src/handlers/codex-runtime.ts` (full file)
- `src-bridge/src/handlers/opencode-runtime.ts` (full file)
- `src-bridge/src/handlers/command-runtime.ts` — delete `cursor` and `gemini` adapter factory branches; retain `qoder` and `iflow`.
- `src-bridge/src/opencode/transport.ts` + `src-bridge/src/opencode/pending-interactions.ts` — delete the entire `opencode/` directory.
- `src-bridge/src/runtime/registry.ts` — legacy adapter factories for cc / codex / opencode / cursor / gemini; the `BRIDGE_ACP_<ADAPTER>` env check in `adapter-factory.ts` (branch that returns legacy).
- `AgentRuntime.claudeQuery` field + `agent-runtime.ts:135-143` `runtime === "claude_code"` gate.
- Legacy tests covering only the deleted handlers (new ACP tests cover equivalent behavior).

### 12.3 Retained

- `EventStreamer`, `SessionManager`, `RuntimePoolManager`, `HookCallbackManager`, `MCPClientHub`, Hono routes, `AgentRuntime` state container, per-adapter `continuity` shapes (still used for state snapshotting across restart).
- `qoder` / `iflow` via `command-runtime.ts`.
- All `/bridge/*` HTTP routes.

### 12.4 Task sequence (input to writing-plans)

1. **T0** — verify `gemini` ACP spawn command; pin exact args in `ACP_ADAPTERS.gemini`; record in spec changelog.
2. **T1** — rescaffold: delete `transport.ts`, rename `process.ts` → `process-host.ts`, add `connection-pool.ts` / `multiplexed-client.ts` / `capabilities.ts` / `adapter-factory.ts` / `index.ts` / `handlers/elicitation.ts`; extend `errors.ts` with 3 classes; add `@agentclientprotocol/sdk` to `package.json` and `bun install`; `bun run typecheck` green.
3. **T2** — `process-host.ts` + `connection-pool.ts` + unit tests (§11.1 first two entries).
4. **T3** — `session.ts` + `multiplexed-client.ts` + `capabilities.ts` + `tests/fixtures/mock-acp-agent.ts` + component tests (§11.2).
5. **T4a** — `handlers/fs.ts` + sandbox unit tests.
6. **T4b** — `handlers/terminal.ts` (reuse `TerminalManager`) + capacity tests.
7. **T4c** — `handlers/permission.ts` (reuse `HookCallbackManager`) + router tests.
8. **T4d** — `handlers/elicitation.ts` passthrough + capability gate tests.
9. **T5** — `events/session-update.ts` full mapping + `session-update-mapping.test.ts`.
10. **T6** — `adapter-factory.ts` + wire into `runtime/registry.ts` × 5 + drop Claude-specific `live_controls` gate + `AcpCommandNotFound` surfacing; Linux CI optional-dep validation.
11. **T7** — integration tests × 5 (§11.3) + `pnpm dev:backend:verify` 5-adapter echo + **§12.2 deletion in same PR** + `_meta.usage` empirical table appended to spec + docs / changelog / `.env.example`.
12. **T8** — Go `task_repo.GetAncestorRoot` + `im_forward` root-task rollup + `im_child_task_folding_mode` per-platform defaults. Can run in parallel with T2–T6; must be merged before T7.

**Dependencies**:
- T0 → T1 → T2 → T3 → (T4a, T4b, T4c, T4d parallel, all depend on T3) → T5 → T6 → T7
- T8 is independent; must complete before T7 merges.

## 13. Out of scope — related but separate concerns

The 2026-04-21 TS Bridge audit identified several issues adjacent to ACP but intentionally excluded from this spec. Each gets its own future brainstorm:

1. **Cost duplication** — `src-bridge/src/cost/{accounting.ts,calculator.ts}` calculates costs; `src-go/internal/cost/tracker.go` also tracks and enforces budget. Two pricing sources risk inconsistency. Target: Go owns pricing + thresholds; Bridge reports raw token counts only. Future spec: `bridge-cost-ownership-realignment`.

2. **Review orchestration split** — `src-bridge/src/review/{orchestrator.ts,aggregator.ts}` + `src-go/internal/service/review_service.go` both participate in review aggregation. Builtin Bridge scanners also use naive string matching (e.g., `eval` in comments). Target: Go selects plugins and aggregates; Bridge executes plugins. Future spec: `bridge-review-orchestration-boundary`.

3. **HTTP MCP transport** — `src-bridge/src/mcp/http-transport.ts` throws `"HTTP MCP transport not yet implemented"`; plugin manager attempting HTTP transport crashes. Future spec or issue: implement or formally deprecate.

4. **Role injector coverage** — `src-bridge/src/role/injector.ts` is only invoked from `claude-runtime.ts`; codex / opencode / cursor / gemini bypass role enforcement. ACP unification naturally migrates all runtimes to the same `AcpSession.prompt` path, so this spec **positions the single injection point in `adapter-factory.ts`** but does not rewrite role semantics. Future spec: `bridge-role-enforcement-uniform-coverage`.

5. **Session snapshot recovery** — Currently only opencode can resume from a snapshot (`handlers/opencode-runtime.ts:50–76` + `runtime/registry.ts:403–449` specialcasing). Post-ACP, `AcpSession` lifecycle covers fork / rollback / resume for all adapters. Quickwin after ACP merges; independent spec.

6. **Bridge test gap fills** — `runtime/backend-profiles.ts`, `role/injector.ts`, `scheduler/bun-cron-adapter.ts`, `ws/event-stream.ts`, `providers/*` have no test coverage. Non-spec; tracked as issues.

7. **Permissions-UX** — `allow_always` / `reject_always` persistence across sessions. Independent spec.

8. **Frontend UI** — for mode / available commands / elicitation / NES. Independent spec.

9. **Cursor `cursor/*` extensions UI mapping** — `ask_question` / `create_plan` / `update_todos` / `task` / `generate_image`. Independent spec.

10. **Tauri desktop distribution readiness** — Node bundle / delayed download / agent binary prepackaging. Triggered when desktop distribution becomes a delivery priority.

## 14. Open questions

Draft-2 left eight open questions. Each gets final disposition here.

1. ~~Unstable fork adoption~~ — **resolved (draft-2)**: capability-gated passthrough (§4.5, §7.2).
2. ~~Process pooling~~ — **resolved (draft-2)**: adapter-level pool (§4.3).
3. `allow_always` / `reject_always` persistence — **deferred**: treated as `_once` this phase; belongs to permissions-UX spec (§13 item 7).
4. Cursor-specific `cursor/*` extensions UI mapping — **deferred**: `extMethod` passthrough wired; UI mapping is a separate spec (§13 item 9).
5. Usage / cost telemetry outside Claude — **resolved by T7 deliverable**: codex / opencode / cursor / gemini `_meta.usage` shapes are sampled during T7 integration runs; each adapter that emits a `usage` payload gets a uniform `cost_update` event. Adapters without `_meta.usage` stay silent. T7 PR appends the empirical shape table to this spec before merge.
6. Linux build paths for `@zed-industries/codex-acp` — **resolved by T6 gate**: CI Linux runner runs `bun install` at T6. Platform optional-dep resolution failure blocks T6 merge.
7. Gemini ACP spawn flag — **resolved by T0**: T0 verifies the exact flag (`--experimental-acp` vs `acp` vs other) and pins `ACP_ADAPTERS.gemini.args`. Decision recorded in this spec's changelog.
8. Sub-agent IM folding per platform — **resolved by T8 + config**: new `im_child_task_folding_mode` per-platform default (`nested` for feishu / slack / telegram / discord, `flat` as fallback, `frontend_only` for QQ-family platforms). T8 wires per-platform defaults in IM bridge manifest.

## 15. References

- Superseded draft: `docs/dev/specs/2026-04-16-bridge-acp-client-integration.md` (historical reference only).
- Superseded plans: `docs/dev/plans/2026-04-16-bridge-acp-client-integration.md`, `docs/dev/plans/2026-04-16-bridge-acp-client-phase1-impl.md`.
- Capability-reference appendix (preserved): `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md`.
- Matrix skeleton + claude_cli / claude_sdk research (preserved): `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/`.
- ACP schema: https://github.com/agentclientprotocol/agent-client-protocol — pin to latest stable tag at T1 commit; record SHA in commit message.
- Official SDK: https://www.npmjs.com/package/@agentclientprotocol/sdk — exports `ClientSideConnection`, `ndJsonStream`, `Client` / `Agent` interfaces, `schema.*`, `RequestError`.
- Adapter wrappers: `@agentclientprotocol/claude-agent-acp@^0.28.0`, `@zed-industries/codex-acp@^0.11.1`, `opencode acp` (native), `cursor-agent acp` (native), `gemini --experimental-acp` / `acp` (T0 verifies).
- Gemini CLI ACP integration (SDK canonical reference): `@google-gemini/gemini-cli` `zedIntegration.ts`.

## 16. Changelog

- **2026-04-21 (this spec, v1)** — final spec superseding 2026-04-16 draft-2.
  - Scaffold correction: `src-bridge/src/runtime/acp/` contents are draft-1-era placeholders; T1 rescaffolds to the draft-2 layout described here.
  - SDK dependency: `@agentclientprotocol/sdk` not yet in `package.json`; T1 installs. Draft-2's claim that the SDK was "installed" was incorrect at the time of writing.
  - Migration plan tightened to two phases (was three): T7 PR merges integration tests + legacy deletion + flag removal atomically. No release-cycle buffer.
  - Tauri policy fixed: Node.js as prerequisite (documentation + structured error with install hint); no bundle / delayed download / binary prepackaging this phase.
  - Error taxonomy section (§10) added: 8 error classes enumerated with triggers, mappings, and recovery strategies.
  - Open questions 3, 4, 5, 6, 7, 8 given concrete disposition (§14).
  - Out-of-scope section (§13) explicitly lists 10 non-ACP TS Bridge concerns found during the 2026-04-21 audit, each with a named future spec.
  - `im_child_task_folding_mode` config added (§9.1, §9.4) to address QQ nested-card incompatibility.
  - `AcpCommandNotFound` error class added to cover `node` / `npx` / `cursor-agent` missing in PATH on desktop.
- **Pre-2026-04-21** — see `docs/dev/specs/2026-04-16-bridge-acp-client-integration.md` §14 for draft-1 → draft-2 history.
