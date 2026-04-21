> **Status: Superseded by [`docs/superpowers/specs/2026-04-21-bridge-acp-client-integration.md`](../../../superpowers/specs/2026-04-21-bridge-acp-client-integration.md).**
> Kept for historical reference. Do not edit; do not cite as source of truth.

# Bridge ACP Client Integration — Spec (draft-2)

- **ID**: `2026-04-16-bridge-acp-client-integration`
- **Author**: Claude (作业编排) + Max Qian
- **Status**: draft-2
- **Scope**: `src-bridge/` runtime layer, `src-go/` IM forwarding observer (root-task rollup), `agent-runtime.ts` live_controls unification
- **Evidence snapshot date**: 2026-04-16
- **Supersedes (not deletes)**: `2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md` — kept as capability-reference appendix
- **Primary references**:
  - ACP schema: https://github.com/agentclientprotocol/agent-client-protocol @ `12cb17d` (2026-04-15)
  - ACP site: https://agentclientprotocol.com
  - **Official ACP TypeScript SDK**: `@agentclientprotocol/sdk@0.19.0` — installed, provides `ClientSideConnection`, `ndJsonStream`, `Client`/`Agent` interfaces, `schema.*` types, `RequestError`. This spec's central premise is to **reuse** it, not re-implement.
  - `openclaw/acpx` — reference implementation we originally mirrored (MIT, TypeScript); reduced relevance now that the official SDK provides the plumbing
  - Gemini CLI ACP integration: `@google-gemini/gemini-cli` `zedIntegration.ts` — SDK README's canonical "production implementation" reference

---

## 1. Background

`src-bridge/` today speaks seven different languages to seven agent runtimes:

| Adapter | Transport | Entry file |
|---|---|---|
| `claude_code` | Anthropic Agent SDK in-proc | `src-bridge/src/handlers/claude-runtime.ts` |
| `codex` | Codex CLI subprocess (custom protocol) | `src-bridge/src/handlers/codex-runtime.ts` |
| `opencode` | HTTP to OpenCode server | `src-bridge/src/handlers/opencode-runtime.ts` |
| `cursor` | stdio subprocess (Cursor CLI JSON stream) | `src-bridge/src/handlers/command-runtime.ts` |
| `gemini` | stdio subprocess (gemini CLI JSON stream) | same |
| `qoder` | stdio subprocess | same |
| `iflow` | stdio subprocess | same |

Every adapter re-implements its own message framing, streaming event translation, tool-call plumbing, permission prompting, and process supervision. Four concrete consequences:

1. **Claude-specific leak.** `agent-runtime.ts:135-143` hardcodes `runtime === "claude_code"` when exposing `live_controls`. `setModel / setThinkingBudget / mcpServerStatus / rewindFiles` only work for Claude because only Claude exposes a `ClaudeQueryControl` handle.
2. **Capability divergence.** `cursor / gemini / qoder / iflow` wrapped by `command-runtime.ts` throw `UnsupportedOperationError` for `fork / rollback / revert / getMessages / getDiff / executeCommand / executeShell / setThinkingBudget / getMcpServerStatus / interrupt / setModel` — the bridge silently downgrades most features.
3. **Four separate translation layers** converting adapter-native events into `AgentEventType`. Each one is a bug surface.
4. **Zed ecosystem alignment drift.** Five agents we care about (`claude_code / codex / opencode / cursor / gemini`) already ship maintained ACP agent wrappers, and the **official ACP TypeScript SDK** already implements the client-side JSON-RPC layer, NDJSON framing, and typed Agent/Client interfaces. We pay the cost of maintaining seven fork-specific pipelines against the cost of zero upstream reuse.

## 2. Goals and non-goals

### Goals

1. Bridge becomes an **ACP client** that drives five target adapters (`claude_code / codex / opencode / cursor / gemini`) over stdio JSON-RPC using the **official `@agentclientprotocol/sdk`**. No DIY transport, no DIY Agent/Client interface, no DIY schema types.
2. Reuse shipped ACP agents — no DIY ACP agent wrappers on our side.
3. The 12-method `RuntimeAdapter` surface and all `/bridge/*` HTTP routes stay **stable at the boundary** (Go orchestrator and frontend unchanged except where a currently-unsupported feature starts working).
4. All five target adapters gain uniform support for: cancel, set_mode, set_model, set_config_option, permission dialog, structured tool-call streaming, fs.read/write, terminal.
5. **Unstable ACP methods** (`unstable_forkSession / unstable_resumeSession / unstable_closeSession / unstable_setSessionModel / unstable_logout`, NES family, document lifecycle, `unstable_createElicitation`) are supported via **capability-gated passthrough** — the SDK exposes them; we wire them to `AcpSession` and fail with structured `AcpCapabilityUnsupported` when an agent does not advertise the capability. We do not yet surface them in frontend UI.
6. **Adapter-level process pooling**: one child process per adapter shared across tasks using ACP's native `session/new` multi-session support. Per-task state lives on the `AcpSession` wrapper; the child lives on `AcpConnectionPool`.
7. Legacy Claude-specific code paths inside the bridge are retired. `live_controls` becomes adapter-agnostic.
8. Feature-flagged rollout: per-adapter flag `BRIDGE_ACP_<ADAPTER>` defaults **ON**; `=0` is an emergency fallback to legacy for one release cycle. Once smoke passes across all five, legacy deletion list (§11.2) executes in the same PR sequence.
9. **End-to-end IM path preserved**: `@runtime <prompt>` in IM → Go task dispatch → Bridge ACP → `session/update` stream → events fan out (persistence, frontend WS, IM forward with root-task rollup for sub-agent tasks). Background tasks continue after IM client disconnects.

### Non-goals (this spec)

- Bridge as an ACP **agent** (server). Zed-side integration is a future spec.
- Migrating `qoder / iflow` — they stay on `command-runtime.ts` until upstream ACP wrappers exist. `command-runtime.ts` loses its `cursor` and `gemini` branches in this spec (§11.2).
- Rewriting the WS event contract to Go. We map ACP events to the current `AgentEventType` set; new event types are additive (`status_change.kind="acp_passthrough"` fallback for unknown subtypes).
- New MCP runtime. We continue using `src-bridge/src/mcp/client-hub.ts` for legacy in-proc MCP; ACP agents get MCP server configs pushed through `session/new.mcpServers` — the agent talks to MCP directly.
- Frontend UI for mode / available commands / elicitation / NES. The wire is connected; visualization is a later spec.
- Persistence of `allow_always / reject_always` permission choices (treated as `_once` this phase).
- Per-adapter Cursor extensions (`cursor/ask_question` etc.). `extMethod` passthrough is wired; UI mapping is a later spec.

## 3. Architecture overview

### 3.1 End-to-end flow

```
┌──────────────────────┐
│ IM 平台 (feishu/      │
│  dingtalk/slack/      │
│  telegram/discord/   │
│  wecom/qq/qqbot/...)  │
└──────┬───────────────┘
       │ @claude <prompt>   (directRuntimeMentions → runtime id)
       ▼
┌──────────────────────┐
│ src-im-bridge        │
│   core/engine        │
└──────┬───────────────┘
       │ POST /api/v1/im/command { runtime, prompt, reply_target, bridge_id }
       ▼
┌──────────────────────────────────────┐
│ src-go (Go orchestrator)             │
│   task_dispatch_service              │
│     ├─ 创建 Task { runtime, provider,│
│     │    im_reply_target, parent_id? }│
│     └─ eventbus: task.dispatch       │
│   workflow/nodetypes/llm_agent        │──► 可派生 child task (ParentID=root)
│   agent_service                      │
│     ├─ POST /bridge/execute          │
│     └─ /bridge/stream (WS)           │
└──────┬───────────────────────────────┘
       │ POST /bridge/execute { task_id, runtime, prompt, ... }
       ▼
┌──────────────────────────────────────────────┐
│ src-bridge (Hono)                            │
│   server/routes → runtime/registry.ts         │
│     createAcpRuntimeAdapter(adapterId)        │
│                                              │
│   runtime/acp/                                │
│     AcpConnectionPool (per-adapter)           │
│       └─ ChildProcessHost + ClientSideConnection│
│                                              │
│     AcpSession (per task_id, per sessionId)   │
│       └─ prompt / cancel / setMode / ...      │
│       └─ unstable_* (capability-gated)        │
│                                              │
│     MultiplexedClient                         │
│       Map<SessionId, PerSessionContext>       │
│         ├─ FsSandbox (worktree-rooted)        │
│         ├─ TerminalManager (pty pool)         │
│         ├─ PermissionRouter                   │
│         └─ EventStreamer (per-task)           │
└──────┬───────────────────────────────────────┘
       │ NDJSON stdio (ndJsonStream)
       ▼
┌──────────────────────────────────────┐
│ ACP agent child (池化; one-per-adapter)│
│   @agentclientprotocol/claude-agent-acp│
│   @zed-industries/codex-acp           │
│   opencode acp                        │
│   cursor-agent acp                    │
│   gemini --experimental-acp           │
│                                      │
│   (agent-internal subagents surface as│
│    tool_call + plan in session/update)│
└──────────────────────────────────────┘

  返回路径 · step-by-step 推送 + 终态:
    session/update  ──► events/session-update.ts 映射
                   ──► EventStreamer.emit(AgentEvent)
                   ──► WS /bridge/stream
                   ──► Go ws.Hub / eventbus
                         ├─ task_events / task_comments 持久化
                         ├─ 前端 WS 推送
                         └─ im_forward (root-task rollup)
                              └─ IM 平台 reply/card update
```

### 3.2 Key architectural shifts from draft-1

| Concern | draft-1 | draft-2 |
|---|---|---|
| JSON-RPC + NDJSON transport | DIY (`transport.ts` with request ID counter, pending map, line buffer, protocol validation) | `ndJsonStream() + new ClientSideConnection()` from SDK |
| `Agent` RPC surface | DIY `AcpClient` class (§4.2) | Directly use SDK `ClientSideConnection` methods |
| `Client` handler interface | DIY types | Implement SDK's exported `Client` interface |
| Per-process scope | One `AcpClient` per task | One `ChildProcessHost` + one `ClientSideConnection` per adapter, pooled |
| Per-task scope | `AcpClient.sessions` map | `AcpSession` wrapper holding a `SessionId` on the pooled connection |
| Input/output routing | N/A | `MultiplexedClient` dispatches inbound `Client` calls by `sessionId` to the right per-task context |
| Unstable methods | Excluded | Exposed on `AcpSession` with `AcpCapabilityUnsupported` gate |
| Adapter coverage | 4 (cc/codex/opencode/cursor) | 5 (+ gemini via `gemini --experimental-acp`) |
| Feature flag default | OFF during dev | ON; `=0` is emergency fallback |
| IM sub-agent rollup | Not addressed | Root-task rollup in `im_forward`; child tasks fold into root's `reply_target` |

## 4. ACP client module design

### 4.1 Module layout

```
src-bridge/src/runtime/acp/
├─ registry.ts           ACP_ADAPTERS[5]  (existing, cursor+gemini already in scope)
├─ errors.ts             (existing) + AcpCapabilityUnsupported, AcpAuthMissing, AcpCommandNotFound
├─ process-host.ts       ChildProcessHost — spawn, stderr ring-buffer, graceful shutdown
├─ connection-pool.ts    AcpConnectionPool — per-adapter singleton; holds host + SDK connection
├─ multiplexed-client.ts MultiplexedClient — implements SDK `Client`; dispatches by sessionId
├─ session.ts            AcpSession — per-(task_id, sessionId) public surface; stable + unstable
├─ capabilities.ts       capability gates, AgentCapabilities cache helpers
├─ handlers/
│   ├─ fs.ts             read/writeTextFile via FsSandbox
│   ├─ terminal.ts       6 terminal methods via TerminalManager
│   ├─ permission.ts     requestPermission via HookCallbackManager
│   └─ elicitation.ts    unstable_createElicitation passthrough
├─ events/
│   └─ session-update.ts sessionUpdate → AgentEventType mapping (single source of truth)
├─ adapter-factory.ts    createAcpRuntimeAdapter(adapterId) — returned by runtime/registry factories
└─ index.ts              barrel
```

**Removed from draft-1 layout**: `transport.ts` (replaced by SDK `ndJsonStream`), `client.ts` monolithic class (split into `process-host.ts` + `connection-pool.ts` + `session.ts` + `multiplexed-client.ts` to reflect pooling).

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
  readonly exited: Promise<number>;         // resolves to exit code
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
  caps: schema.AgentCapabilities;       // cached from initialize response
  clientDispatcher: MultiplexedClient;  // the `Client` impl the SDK calls into
  sessions: Set<SessionId>;             // ref count for idle reclaim
  restartPending: boolean;
}

interface AcquireContext {              // supplied by AcpSession.open
  registerSession(sessionId: SessionId, ctx: PerSessionContext): void;
  unregisterSession(sessionId: SessionId): void;
}
```

- `acquire` serializes per-adapter via a mutex to prevent double-spawn under concurrent first-use.
- If `restartPending` or `host.exited` is settled, spawn a fresh host and re-initialize.
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

- If `params.sessionId` is unknown, reject with `RequestError(-32602, "unknown_session")`.
- Per-method logic delegates to a handler module under `handlers/`.
- For `sessionUpdate` (notification), errors are logged and swallowed — we cannot respond with a JSON-RPC error (it is one-way) and we MUST continue receiving subsequent updates per ACP cancellation semantics.

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
- `prompt` serializes per-session; concurrent call rejects immediately with `AcpConcurrentPrompt`.
- All capability checks read `this.capabilities` (cached on `AcpSession.open`). No live re-query.

### 4.6 `adapter-factory.ts`

```ts
export function createAcpRuntimeAdapter(adapterId: AdapterId): RuntimeAdapterFactory {
  return async (task, streamer, deps) => {
    if (process.env[`BRIDGE_ACP_${adapterId.toUpperCase()}`] === "0") {
      return deps.legacyFactories[adapterId](task, streamer, deps); // emergency fallback
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
      execute: (req) => session.prompt(toContentBlocks(req)).then(mapStopReason),
      cancel:  () => session.cancel(),
      interrupt: () => session.cancel(),               // alias
      setModel: (m) => session.setModel(m),             // throws AcpCapabilityUnsupported if unsupported
      setMode:  (m) => session.setMode(m),
      setConfigOption: (k, v) => session.setConfigOption(k, v),
      setThinkingBudget: (n) =>
        session.capabilities.promptCapabilities?.thinkingBudget
          ? session.setConfigOption("thinking_budget", n)
          : throwUnsupported("setThinkingBudget"),
      fork: () => session.forkSession().then(wrapAsAdapter),
      rollback: (args) => rollbackViaReplay(session, args),     // uses loadSession if capable
      revert: (args) => deps.worktreeService.revert(args),      // not via ACP
      getMessages: () => deps.taskEventsService.messages(task.id), // not via ACP
      getDiff: (args) => deps.worktreeService.diff(args),
      executeCommand: (c) => session.extMethod("agent/executeCommand", { command: c })
                                   .catch(() => session.prompt([{ type:"text", text:`/run ${c}` }])),
                                   // best-effort: prefer agent extension; fall back to slash-command prompt
      executeShell: (c) => deps.terminalManager.runOneShot(task.id, c),
      getMcpServerStatus: () => deps.mcpServersStatus(task),
      dispose: () => session.dispose(),
    };
  };
}
```

`live_controls` (in `agent-runtime.ts:135-143`) drops the `runtime === "claude_code"` gate. Fields populate based on `session.capabilities`:

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
    args: ["--acp"],                     // verified: upstream deprecated --experimental-acp in favor of --acp
    envRequired: [],                     // gemini handles own auth
    cursorExtensions: false,
  },
} as const satisfies Record<AdapterId, AcpAdapterConfig>;
```

T0 verification (2026-04-16): google-gemini/gemini-cli `packages/cli/src/config/config.ts` defines two flags — `--acp` (current) and `--experimental-acp` (deprecated alias). Our registry uses `--acp`.

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
export function readTextFile(ctx: PerSessionContext, params: schema.ReadTextFileRequest): Promise<schema.ReadTextFileResponse> {
  const abs = ctx.fsSandbox.resolve(params.path); // realpath + startsWith(cwd)
  // apply line/limit slicing; UTF-8; reject binary per ACP schema
}
export function writeTextFile(ctx: PerSessionContext, params: schema.WriteTextFileRequest): Promise<schema.WriteTextFileResponse>;
```

**Sandbox rule**: paths escaping the session's worktree `cwd` (after `realpath`) reject with `RequestError(-32602, { reason: "path_escapes_worktree" })`. No fallback to the outer filesystem. `line + limit` are 1-based line numbers, limit in lines, per ACP schema. Binary writes are out of scope this phase.

### 6.2 `terminal/*` handler (`handlers/terminal.ts`)

Reuses `TerminalManager` (PTY pool). `node-pty` on desktop (Tauri sidecar), `Bun.spawn` fallback for web-dev.

Capacity: per-task default `outputByteLimit = 10 MB`, global `maxConcurrentTerminals = 16`. Over-capacity allocation rejects with `RequestError(-32000, { reason: "terminal_capacity" })`.

Lifecycle maps 1:1 to ACP `createTerminal / terminalOutput / waitForTerminalExit / killTerminal / releaseTerminal`. `killTerminal` keeps id valid; `releaseTerminal` frees slot and invalidates id. Tool calls referencing a released id keep their last-known output snapshot.

### 6.3 `session/request_permission` handler (`handlers/permission.ts`)

Reuses existing `HookCallbackManager` and `/bridge/permission-response/:request_id` route — no new FE surface.

Flow:
1. Agent sends `session/request_permission { sessionId, toolCall, options }`.
2. `PermissionRouter.request(taskId, toolCall, options)` generates a `request_id`, emits `permission_request` event through `EventStreamer`, registers resolver in `HookCallbackManager`.
3. Frontend / IM → Go → `POST /bridge/permission-response/:request_id` with `{ option_id }` or `{ cancelled: true }`.
4. Handler resolves pending promise; router returns `{ outcome: "selected", optionId }` or `{ outcome: "cancelled" }`.

Timeout: 30 min no-response → auto-cancel (`{ outcome: "cancelled" }` returned to agent; `permission_timeout` event emitted). `allow_always / reject_always` currently treated as `_once` (no persistence this phase).

### 6.4 `unstable_createElicitation` handler (`handlers/elicitation.ts`)

Capability-gated passthrough. Reuses `PermissionRouter`'s request_id + `HookCallbackManager` pattern, but event subtype is `elicitation_request` and response shape is a schema-typed payload rather than an option id.

Frontend has no UI this phase; bridge registers the `unstable_createElicitation` method on `MultiplexedClient` unconditionally (SDK `Client` interface marks it optional — agents that do not support elicitation simply never call it). If invoked and the session's `ctx.permissionRouter` is unavailable (unexpected state), resolve with `{ action: "cancel" }` per schema rather than throwing.

## 7. Session lifecycle

### 7.1 Per-task state machine

```
new-task → AcpSession.open:
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

Drop `runtime === "claude_code"` gate at `agent-runtime.ts:135-143`. Replace with capability-driven population as described in §4.6. Frontend already tolerates field absence.

### 7.4 Sub-agents

Two distinct concepts, handled separately:

- **Agent-internal subagents** (e.g., Claude's Task tool, Codex internal planner): represented in ACP as nested `tool_call` / `tool_call_update` / `plan` updates within the parent session's `session/update` stream. Bridge does nothing special — they flow through `session-update.ts` mapping like any other tool call.
- **Orchestrator-level child tasks** (e.g., workflow `llm_agent` node spawning a new `Task` with `ParentID` set): each child task calls `/bridge/execute` independently. `pool.acquire(adapterId)` reuses the already-running host; `conn.newSession()` produces a new sessionId. The child task's `AcpSession` registers with the same `MultiplexedClient` under its own sessionId. Parent and child run concurrently on the pooled connection.

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
| `tool_call_update` (completed/failed) | `tool_result` | preserves FE expectation |
| `plan` | `todo_update` | |
| `available_commands_update` | `status_change{kind:"commands"}` | additive |
| `current_mode_update` | `status_change{kind:"mode"}` | additive |
| `config_option_update` | `status_change{kind:"config_option"}` | additive |

Unstable / extension variants (capability-gated; wire only, no FE work this phase):

| Unstable | Mapping |
|---|---|
| NES lifecycle events | `status_change{kind:"nes", subtype:<event>}` |
| document lifecycle (open/change/close/save/focus) | `status_change{kind:"document", subtype:<event>}` |
| `extMethod` / `extNotification` | `status_change{kind:"ext", method:<name>}` |
| Any unknown `sessionUpdate` variant | `status_change{kind:"acp_passthrough"}` + `metadata._raw = <original>` |

Permission requests flow through §6.3, not this table.

`_meta` on every variant is copied verbatim into `AgentEvent.metadata._meta` so downstream consumers can opt into richer fields without a schema change. Usage / cost: emit `cost_update` when `_meta.usage` is present (Claude emits this; other adapters TBD — Phase T7 integration tests produce the data to decide, tracked §12).

## 9. Cross-layer impact

### 9.1 Go orchestrator

- **Add `task_repo.GetAncestorRoot(taskID)`** (if not already equivalent): walks `ParentID` chain to the root task. Used by `im_forward`.
- **`im_forward` observer**: for every event, resolve `event.task_id → ancestor root → root.im_reply_target`. Sub-agent progress/results fold into the root's IM thread. Task with no `im_reply_target` (non-IM-originated) stays invisible to IM bridge as today.
- **No other API changes.** `/bridge/*` routes and WS event shapes preserved.
- Observable: `live_controls` populates for non-Claude adapters. Go code already treats `live_controls` as present-or-absent — verified during T6.
- Cost events arrive for codex/opencode/cursor/gemini where previously only claude_code emitted. No downstream breakage expected.

### 9.2 Frontend

- No required change. New `status_change.kind` values (`mode / commands / config_option / nes / document / ext / acp_passthrough`) are backwards-compatible — existing handlers ignore unknown kinds.
- Optional follow-up (out of scope): UI for mode / available commands / elicitation.

### 9.3 Tauri

- Sidecar packaging requirement: `npx` (Node.js) must be available in the desktop runtime. Document as install requirement; optionally ship Node.js in the Tauri bundle.
- `@zed-industries/codex-acp` has platform-specific optional dependencies — Tauri build resolves correct native binary per target triple. Validated during T6.
- `cursor-agent` / `opencode` / `gemini` are user-installed CLIs. `pool.acquire` failure surfaces `AcpCommandNotFound(adapterId, command)` → mapped to structured `AUTH_REQUIRED`-adjacent event.

### 9.4 IM Bridge

- `directRuntimeMentions` already includes all 5 runtime mentions (`@claude`/`@claudecode`/`@claude-code`, `@codex`, `@opencode`, `@cursor`, `@gemini`). No change.
- Card renderers already handle `tool_call / tool_result / reasoning / output / permission_request / todo_update` (T8 verifies no regression). Additive `status_change{kind:...}` subtypes fall through the default-renderer branch.
- Sub-agent rollup: child task progress appears folded in the root's thread per existing platform-specific renderer (飞书嵌套卡片 / Slack thread / Telegram reply chain).

## 10. Testing strategy

Matches the depth selected in brainstorm problem 5 (B: unit + component + per-adapter smoke).

1. **Unit** (`src-bridge/tests/unit/runtime/acp/`):
   - `fs-sandbox.test.ts` — absolute path / `..` / symlink escape rejection.
   - `multiplexed-client.test.ts` — sessionId routing; unknown sessionId rejection.
   - `session-update-mapping.test.ts` — every stable variant + unstable passthrough + unknown-variant fallback.
   - `permission-router.test.ts` — allow/reject, timeout, cancel race.
   - `capability-gate.test.ts` — unstable method without advertised capability → `AcpCapabilityUnsupported`.
   - `connection-pool.test.ts` — concurrent acquire, ref counting, idle reclaim, crash → `restartPending`.
2. **Component** (`src-bridge/tests/component/acp/`):
   - `mock-acp-agent.ts` fixture (standalone script): initialize, session/new, session/prompt, session/update, session/request_permission, fs/*, terminal/*.
   - `happy-path.test.ts` — initialize → newSession → prompt → text delta → tool call → stopReason.
   - `cancel-race.test.ts` — cancel before/during/after prompt.
   - `pooling.test.ts` — two tasks share one host; one crash notifies both.
   - `multi-session-fs.test.ts` — two concurrent sessions' fs/terminal requests route correctly.
3. **Integration** — per adapter (`src-bridge/tests/integration/acp/<adapter>.test.ts`), one file each for `claude_code / codex / opencode / cursor / gemini`:
   - smoke: `prompt("echo hello")` → receives text delta.
   - cancel: `prompt + cancel` → `stopReason=cancelled`.
   - fs: agent reads/writes a temp file.
   - terminal: agent runs `echo pong`.
   - permission: agent triggers a permission-requiring tool.
   - Skippable via `SKIP_ACP_INTEGRATION=1`.
4. **End-to-end smoke** — `pnpm dev:backend:verify` gains a 5-adapter echo prompt.

Type regression: `pnpm exec tsc --noEmit` must pass; `schema.*` coverage in `events/session-update.ts` verified.

## 11. Migration plan

### 11.1 Rollout phases

Rollout progresses through three phases within this spec's execution:

1. **Dev** (T2–T7): flags default ON in local dev; legacy factories still present and reachable via `BRIDGE_ACP_<ADAPTER>=0`. Integration tests skippable. `pnpm dev:backend:verify` smokes all five.
2. **Smoke green** (end of T7): all five adapters pass smoke on trunk. Flag defaults stay ON; `=0` fallback retained for one release cycle as emergency lever.
3. **Cleanup** (T9): legacy files deleted per §11.2. `BRIDGE_ACP_<ADAPTER>=0` becomes a no-op (logged warning).

### 11.2 Deletion list (executed in T9)

After phase 2 and the emergency-fallback window closes:
- `src-bridge/src/handlers/claude-runtime.ts`
- `src-bridge/src/handlers/codex-runtime.ts`
- `src-bridge/src/handlers/opencode-runtime.ts`
- `src-bridge/src/handlers/command-runtime.ts` — delete `cursor` and `gemini` adapter factory branches; keep `qoder` and `iflow`.
- `src-bridge/src/opencode/` legacy transport module.
- `src-bridge/src/runtime/registry.ts` — legacy adapter factories for cc/codex/opencode/cursor/gemini.
- `AgentRuntime.claudeQuery` field + `agent-runtime.ts:135-143` `runtime === "claude_code"` gate.

### 11.3 Retained

- `EventStreamer`, `SessionManager`, `RuntimePoolManager`, `HookCallbackManager`, `MCPClientHub`, Hono routes, `AgentRuntime` state container, per-adapter `continuity` shapes (still used for state snapshotting across restart).

### 11.4 Task sequence (input to writing-plans)

1. **T0**: verify `gemini` ACP spawn command (pin exact args).
2. **T1**: (done) scaffolding — `registry.ts`, `errors.ts`.
3. **T2**: `process-host.ts` + `connection-pool.ts` + unit tests.
4. **T3**: `session.ts` + `multiplexed-client.ts` + `capabilities.ts` + mock-agent fixture + happy/cancel/pooling component tests.
5. **T4a**: `handlers/fs.ts` + sandbox.
6. **T4b**: `handlers/terminal.ts` (reuse `TerminalManager`).
7. **T4c**: `handlers/permission.ts` (reuse `HookCallbackManager`).
8. **T4d**: `handlers/elicitation.ts` passthrough.
9. **T5**: `events/session-update.ts` full mapping + tests.
10. **T6**: `adapter-factory.ts` + wire into `runtime/registry.ts` × 5 + drop Claude-specific `live_controls` gate.
11. **T7**: integration tests × 5 (skippable via env) + `pnpm dev:backend:verify` echo.
12. **T8**: Go `task_repo.GetAncestorRoot` + `im_forward` root-task rollup.
13. **T9**: legacy deletion (list §11.2).
14. **T10**: docs/PRD update + changelog + `BRIDGE_ACP_*` env in `.env.example`.

## 12. Open questions

1. ~~Unstable fork adoption~~ — **resolved**: capability-gated passthrough (§4.5, §7.2).
2. ~~Process pooling~~ — **resolved**: adapter-level pool (§4.3).
3. `allow_always / reject_always` persistence — still open; belongs to a permissions-UX spec.
4. Cursor-specific `cursor/*` extensions (`ask_question / create_plan / update_todos / task / generate_image`) — passthrough via `extMethod` this phase; UI mapping later spec.
5. Usage / cost telemetry outside Claude — codex/opencode/cursor/gemini `_meta.usage` payloads need inspection in T7 to decide uniform `cost_update` emission.
6. Linux build paths for `@zed-industries/codex-acp` — platform optionalDep resolution in CI + Tauri. Validate during T6.
7. ~~Gemini ACP spawn flag~~ — **resolved** (T0, 2026-04-16): `--acp` is current (`--experimental-acp` is deprecated alias).
8. Sub-agent IM folding per platform — 飞书 interactive card嵌套、Slack thread、Telegram reply chain 差异由 IM bridge 既有 per-platform renderer 承接；若差异过大（例如 QQ 不支持嵌套），是否引入"子任务只走前端不发 IM"开关？待 T8 审视 `im_forward` 后定。

## 13. References

- Capability-reference appendix (preserved): `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2-design.md`
- Matrix skeleton + claude_cli / claude_sdk research (preserved): `docs/superpowers/specs/2026-04-16-runtime-capability-matrix-and-adapter-v2/`
- ACP schema (stable): https://github.com/agentclientprotocol/agent-client-protocol/tree/12cb17d/schema
- Official SDK: https://www.npmjs.com/package/@agentclientprotocol/sdk — `ClientSideConnection`, `ndJsonStream`, `Client`/`Agent` interfaces, `schema.*`, `RequestError`.
- `openclaw/acpx` (reduced relevance): https://github.com/openclaw/acpx
- Adapter wrappers: `@agentclientprotocol/claude-agent-acp@^0.28.0`, `@zed-industries/codex-acp@^0.11.1`, `opencode acp` (native), `cursor-agent acp` (native), `gemini --acp` (native; `--experimental-acp` deprecated alias).
- Gemini CLI ACP integration (SDK README's canonical reference): `@google-gemini/gemini-cli` `zedIntegration.ts`.

## 14. Changelog

- **draft-2** (2026-04-16) — substantive rework around SDK reuse + pooling + scope expansion:
  - §2: unstable methods dropped from non-goals; capability-gated passthrough added. Gemini added to target adapters (5 total). Feature-flag default flipped to ON.
  - §3: end-to-end architecture diagram covers IM → Go → Bridge → agent → back; draft-1 → draft-2 shift table added.
  - §4: `transport.ts` and monolithic `AcpClient` removed in favor of `@agentclientprotocol/sdk` reuse. New module split: `process-host.ts`, `connection-pool.ts`, `multiplexed-client.ts`, `session.ts`, `capabilities.ts`, `adapter-factory.ts`. `live_controls` capability-driven population detailed.
  - §5: registry grows to 5 adapters (gemini added).
  - §6: elicitation handler added; sandbox and terminal semantics clarified.
  - §7: lifecycle rewritten around pooling; sub-agent (§7.4) and background task (§7.5) sections new.
  - §8: unstable and unknown-variant passthrough rows added to mapping table.
  - §9: root-task rollup requirement for `im_forward` codified.
  - §10-11: testing tiers and migration sequence re-keyed to the new module boundaries; legacy deletion list updated (cursor+gemini branches in `command-runtime.ts` included).
  - §12: questions 1-2 closed; questions 7-8 added.
- **draft-1** (2026-04-16) — initial spec after brainstorm pivot from capability-universe matrix to ACP client integration. Four target adapters: `claude_code / codex / opencode / cursor`. Zero DIY ACP wrappers required (but transport was still DIY — corrected in draft-2).
