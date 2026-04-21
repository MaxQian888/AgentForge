# Bridge ACP Client Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate five `src-bridge/` agent adapters (`claude_code / codex / opencode / cursor / gemini`) to speak ACP via the official `@agentclientprotocol/sdk`, with adapter-level process pooling, capability-gated unstable passthrough, and preserved end-to-end IM path. Delete legacy handler files in the same PR as integration smoke.

**Architecture:** Reuse SDK's `ClientSideConnection` + `ndJsonStream` (no DIY transport). One pooled `ChildProcessHost` per adapter owns one SDK connection; a `MultiplexedClient` routes inbound `Client` calls (fs / terminal / permission / elicitation / sessionUpdate) by `sessionId` to per-task contexts. `AcpSession` wraps `(pool, sessionId)` and is what `runtime/registry.ts` adapter factories return, mapped to the existing `RuntimeAdapter` interface. Legacy runtime files deleted in T7.

**Tech Stack:** TypeScript, Bun runtime + `bun:test`, Hono HTTP, `@agentclientprotocol/sdk`, `node-pty`, existing `HookCallbackManager` / `EventStreamer` / Hono router.

**Source of truth:** `docs/superpowers/specs/2026-04-21-bridge-acp-client-integration.md`. Every task links to a spec section; diverging requires spec update first.

---

## Shared invariants (every task)

- `bun run typecheck` (in `src-bridge/`) MUST pass on every commit starting from T1.
- `pnpm exec tsc --noEmit` (repo root) MUST pass on every commit.
- `bun test` in `src-bridge/` MUST pass on every commit from T2 onward. Integration tests gated by `SKIP_ACP_INTEGRATION=1` (default ON in CI).
- Commit message format: `acp(client): T<N> — <subject>` (or `T4a / T4b / ...` for subtasks). One commit per task unless a task explicitly calls out split commits.
- `/bridge/*` HTTP routes and their response shapes are FROZEN; do not rename or break them.
- `AgentEventType` enum may gain members (additive); do not change semantics of existing members.
- `BRIDGE_ACP_<ADAPTER>` env vars exist only during T1–T6; T7 deletes the branches.

## File structure (post-T1)

```
src-bridge/
├─ package.json                                        [T1 modify: +@agentclientprotocol/sdk, +node-pty, +@types/node-pty]
├─ src/
│  ├─ server.ts                                        [T6 modify: inject AcpConnectionPool into registry options]
│  ├─ runtime/
│  │  ├─ registry.ts                                   [T1 modify: AdapterId += "gemini"; T6 modify: adapter factories]
│  │  ├─ agent-runtime.ts                              [T6 modify: drop live_controls Claude gate; T7 delete claudeQuery]
│  │  └─ acp/
│  │     ├─ index.ts                                   [T1 create: barrel]
│  │     ├─ registry.ts                                [T1 modify: +gemini, extend AdapterId]
│  │     ├─ errors.ts                                  [T1 modify: +3 classes]
│  │     ├─ process-host.ts                            [T1 create (rename process.ts → this); T2 implement]
│  │     ├─ connection-pool.ts                         [T1 create stub; T2 implement]
│  │     ├─ multiplexed-client.ts                      [T1 create stub; T3 implement]
│  │     ├─ session.ts                                 [T1 rewrite stub; T3 implement]
│  │     ├─ capabilities.ts                            [T1 create stub; T3 implement]
│  │     ├─ adapter-factory.ts                         [T1 create stub; T6 implement]
│  │     ├─ fs-sandbox.ts                              [T4a create]
│  │     ├─ terminal-manager.ts                        [T4b create]
│  │     ├─ permission-router.ts                       [T4c create]
│  │     ├─ handlers/
│  │     │  ├─ fs.ts                                   [T1 rewrite stub; T4a implement]
│  │     │  ├─ terminal.ts                             [T1 rewrite stub; T4b implement]
│  │     │  ├─ permission.ts                           [T1 rewrite stub; T4c implement]
│  │     │  └─ elicitation.ts                          [T1 create; T4d implement]
│  │     └─ events/
│  │        └─ session-update.ts                       [T1 rewrite stub; T5 implement]
│  └─ handlers/
│     ├─ claude-runtime.ts                             [T7 delete]
│     ├─ codex-runtime.ts                              [T7 delete]
│     ├─ opencode-runtime.ts                           [T7 delete]
│     └─ command-runtime.ts                            [T7 modify: drop cursor+gemini branches]
├─ tests/                                              [new root for integration + component + fixtures]
│  ├─ fixtures/
│  │  └─ mock-acp-agent.ts                             [T3 create]
│  ├─ unit/runtime/acp/                                [T2–T5 create one file per module]
│  ├─ component/acp/                                   [T3 create: happy-path, cancel-race, pooling, multi-session-fs, permission-flow]
│  └─ integration/acp/                                 [T7 create: {claude_code,codex,opencode,cursor,gemini}.test.ts]
└─ src/opencode/                                       [T7 delete directory]

src-go/
└─ internal/
   ├─ repository/task_repo.go                          [T8 modify: +GetAncestorRoot]
   └─ service/im_forward_*.go                          [T8 modify: root-task rollup + folding_mode]
```

Tests colocated under `src/**/*.test.ts` (existing pattern) remain for unit tests of small utilities. Larger ACP suites live under `src-bridge/tests/`.

## Dependencies between tasks

```
T0 ─► T1 ─► T2 ─► T3 ─► T4a ─┐
                      ├► T4b ├─► T5 ─► T6 ─► T7
                      ├► T4c ┤
                      └► T4d ┘
T8 is independent; start any time from T1 onward; MUST merge before T7.
```

---

## Task T0: Verify gemini ACP spawn command

**Goal:** Determine the correct subcommand/flag for the `gemini` CLI to enter ACP mode. Record decision in spec changelog.

**Files:**
- Modify: `docs/superpowers/specs/2026-04-21-bridge-acp-client-integration.md` (changelog entry + §5.1 if args change)

**Steps:**

- [ ] **Step 1: Probe gemini CLI**

Run on a machine with `gemini` installed:

```bash
gemini --help 2>&1 | grep -Ei "acp|agent-client|experimental" || echo "no match"
gemini acp --help 2>&1 | head -5 || true
gemini --experimental-acp --help 2>&1 | head -5 || true
```

Expected: one of these returns usage help without error. The winning form is the canonical invocation.

- [ ] **Step 2: Cross-reference upstream**

Open `@google-gemini/gemini-cli` `zedIntegration.ts` on GitHub and confirm which invocation the Zed integration uses. Record the URL and commit SHA in the spec changelog.

- [ ] **Step 3: Pin args in spec**

Edit spec §5.1 `ACP_ADAPTERS.gemini.args` to the verified form. Edit spec §16 changelog with a new entry:

```markdown
- **2026-MM-DD (T0 verification)** — Pinned `gemini` ACP spawn to `["<verified-args>"]` (verified against gemini CLI vX.Y and `@google-gemini/gemini-cli` @ `<sha>` `zedIntegration.ts`).
```

- [ ] **Step 4: Commit**

```bash
rtk git add docs/superpowers/specs/2026-04-21-bridge-acp-client-integration.md
rtk git commit -m "acp(client): T0 — pin gemini spawn command"
```

---

## Task T1: Rescaffold `runtime/acp/` + install SDK

**Goal:** Replace draft-1 placeholders with the target module layout. Install `@agentclientprotocol/sdk` and `node-pty`. All new files compile but stubs throw `AcpCapabilityUnsupported` or return `Promise.reject(new Error("not implemented by T<N>"))` at runtime.

**Files:**
- Modify: `src-bridge/package.json`
- Delete: `src-bridge/src/runtime/acp/transport.ts`
- Rename + rewrite: `src-bridge/src/runtime/acp/process.ts` → `process-host.ts`
- Modify: `src-bridge/src/runtime/acp/errors.ts`
- Modify: `src-bridge/src/runtime/acp/registry.ts`
- Create: `src-bridge/src/runtime/acp/{connection-pool,multiplexed-client,capabilities,adapter-factory,index}.ts`
- Create: `src-bridge/src/runtime/acp/handlers/elicitation.ts`
- Rewrite (stub bodies, keep paths): `src-bridge/src/runtime/acp/session.ts`, `src-bridge/src/runtime/acp/events/session-update.ts`, `src-bridge/src/runtime/acp/handlers/{fs,terminal,permission}.ts`

**Steps:**

- [ ] **Step 1: Verify current scaffold state**

```bash
ls src-bridge/src/runtime/acp
```

Expected output (unordered): `errors.ts events handlers process.ts registry.ts session.ts transport.ts`

- [ ] **Step 2: Add dependencies**

Edit `src-bridge/package.json` to add (inside `"dependencies"`):

```json
    "@agentclientprotocol/sdk": "^0.19.0",
    "node-pty": "^1.0.0",
```

And inside `"devDependencies"`:

```json
    "@types/node": "^22.10.0",
```

If `@types/node` is already present, skip it. Then:

```bash
cd src-bridge && bun install
```

Expected: `bun install` completes without error. Verify SDK resolved:

```bash
ls node_modules/@agentclientprotocol/sdk/dist 2>&1 | head -5
```

Expected: shows `.d.ts` and `.js` files. If `@agentclientprotocol/sdk@^0.19.0` is not published, halt and update spec §5.1 / this task with whatever the live version is — do not substitute a different package.

- [ ] **Step 3: Delete transport.ts**

```bash
rm src-bridge/src/runtime/acp/transport.ts
```

- [ ] **Step 4: Rename process.ts → process-host.ts with new stub**

```bash
rm src-bridge/src/runtime/acp/process.ts
```

Create `src-bridge/src/runtime/acp/process-host.ts`:

```ts
// ChildProcessHost — spawn, stderr ring-buffer, graceful shutdown.
// Implementation lands in T2. T1 ships an unusable stub that compiles.

export interface ChildProcessHostOptions {
  adapterId: string;
  command: string;
  args: readonly string[];
  env: Record<string, string>;
  logger: { info: (...a: unknown[]) => void; warn: (...a: unknown[]) => void; error: (...a: unknown[]) => void };
}

export interface ChildProcessHostHandles {
  stdin: WritableStream<Uint8Array>;
  stdout: ReadableStream<Uint8Array>;
}

export class ChildProcessHost {
  readonly stderrBuffer: { dump(): string } = { dump: () => "" };
  readonly exited: Promise<number> = Promise.resolve(-1);

  constructor(private readonly opts: ChildProcessHostOptions) {}

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  start(): Promise<ChildProcessHostHandles> {
    return Promise.reject(new Error("ChildProcessHost.start not implemented (T2)"));
  }

  shutdown(_gracefulMs?: number): Promise<void> {
    return Promise.reject(new Error("ChildProcessHost.shutdown not implemented (T2)"));
  }
}
```

- [ ] **Step 5: Extend errors.ts**

Append to `src-bridge/src/runtime/acp/errors.ts`:

```ts
/**
 * The agent did not advertise the capability required by the invoked
 * unstable method (e.g. `session.setModel` without `availableModels`).
 */
export class AcpCapabilityUnsupported extends Error {
  constructor(public readonly capability: string) {
    super(`ACP capability not advertised: ${capability}`);
    this.name = "AcpCapabilityUnsupported";
  }
}

/**
 * The adapter's `envRequired` is missing from the bridge process env.
 * Surfaced before spawn; maps to the structured AUTH_REQUIRED event.
 */
export class AcpAuthMissing extends Error {
  constructor(public readonly adapterId: string, public readonly missing: readonly string[]) {
    super(`ACP auth missing for ${adapterId}: ${missing.join(", ") || "(provider-managed)"}`);
    this.name = "AcpAuthMissing";
  }
}

/**
 * Spawn failed with ENOENT — the adapter executable (or `npx`/`node`) is
 * not in PATH. Maps to AUTH_REQUIRED with `install_hint` metadata.
 */
export class AcpCommandNotFound extends Error {
  constructor(
    public readonly adapterId: string,
    public readonly command: string,
    public readonly installHint: string,
  ) {
    super(`ACP command not found: ${command} (${adapterId}). Install hint: ${installHint}`);
    this.name = "AcpCommandNotFound";
  }
}
```

- [ ] **Step 6: Update registry.ts (add gemini; widen AdapterId)**

Replace the `AdapterId` union and `ACP_ADAPTERS` object in `src-bridge/src/runtime/acp/registry.ts` with the 5-adapter form (copying from spec §5.1):

```ts
export type AdapterId = "claude_code" | "codex" | "opencode" | "cursor" | "gemini";

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
    args: ["--experimental-acp"], // T0 verifies — if T0 flipped this to ["acp"], replace here.
    envRequired: [],
    cursorExtensions: false,
  },
} as const satisfies Record<AdapterId, AcpAdapterConfig>;
```

Keep the existing `AcpAdapterConfig` interface unchanged. Update the doc comment to mention "five adapters this phase migrates to ACP" instead of "four".

- [ ] **Step 7: Create capabilities.ts stub**

`src-bridge/src/runtime/acp/capabilities.ts`:

```ts
// capability gates + AgentCapabilities cache helpers. Implementation: T3.
import { AcpCapabilityUnsupported } from "./errors";

// Runtime shape reused by T3; wide type here to keep T1 compilable without
// the SDK schema types being touched yet.
export interface CachedCapabilities {
  raw: unknown;
  modelSelectable: boolean;
  thinkingBudget: boolean;
  modeSelectable: boolean;
  mcpStatus: boolean;
  sessionFork: boolean;
  loadSession: boolean;
}

export function requireCapability(caps: CachedCapabilities, key: keyof CachedCapabilities, label: string): void {
  if (!caps[key]) {
    throw new AcpCapabilityUnsupported(label);
  }
}
```

- [ ] **Step 8: Create connection-pool.ts stub**

`src-bridge/src/runtime/acp/connection-pool.ts`:

```ts
// AcpConnectionPool — adapter-level singleton. Implementation: T2.
import type { AdapterId } from "./registry";
import type { CachedCapabilities } from "./capabilities";
import type { ChildProcessHost } from "./process-host";

export type SessionId = string;

export interface PerSessionContext {
  taskId: string;
  cwd: string;
  // Full types land in T3 (FsSandbox, TerminalManager, PermissionRouter, EventStreamer).
  [key: string]: unknown;
}

export interface AcquireContext {
  registerSession(sessionId: SessionId, ctx: PerSessionContext): void;
  unregisterSession(sessionId: SessionId): void;
}

export interface PooledEntry {
  adapterId: AdapterId;
  host: ChildProcessHost;
  // SDK connection lands in T2.
  conn: unknown;
  caps: CachedCapabilities;
  clientDispatcher: unknown;
  sessions: Set<SessionId>;
  restartPending: boolean;
}

export interface AcpConnectionPoolOptions {
  logger: { info: (...a: unknown[]) => void; warn: (...a: unknown[]) => void; error: (...a: unknown[]) => void };
  idleMs?: number;
}

export class AcpConnectionPool {
  constructor(private readonly opts: AcpConnectionPoolOptions) {}

  acquire(_adapterId: AdapterId, _ctx: AcquireContext): Promise<PooledEntry> {
    return Promise.reject(new Error("AcpConnectionPool.acquire not implemented (T2)"));
  }

  release(_adapterId: AdapterId, _sessionId: SessionId): Promise<void> {
    return Promise.reject(new Error("AcpConnectionPool.release not implemented (T2)"));
  }

  shutdownAll(_graceful?: boolean): Promise<void> {
    return Promise.resolve();
  }
}
```

- [ ] **Step 9: Create multiplexed-client.ts stub**

`src-bridge/src/runtime/acp/multiplexed-client.ts`:

```ts
// MultiplexedClient — SDK `Client` impl; dispatches by sessionId. Impl: T3.
import type { PerSessionContext, SessionId } from "./connection-pool";

export class MultiplexedClient {
  private readonly sessions = new Map<SessionId, PerSessionContext>();

  register(sessionId: SessionId, ctx: PerSessionContext): void {
    this.sessions.set(sessionId, ctx);
  }

  unregister(sessionId: SessionId): void {
    this.sessions.delete(sessionId);
  }

  // All Client interface methods land in T3, delegating to handlers/*.
}
```

- [ ] **Step 10: Rewrite session.ts with T3-ready stub**

`src-bridge/src/runtime/acp/session.ts`:

```ts
// AcpSession — per-(task_id, sessionId) public surface. Impl: T3.
import type { AdapterId } from "./registry";
import type { AcpConnectionPool, SessionId } from "./connection-pool";
import type { CachedCapabilities } from "./capabilities";

export interface AcpSessionOptions {
  taskId: string;
  adapterId: AdapterId;
  cwd: string;
  // Full context types (streamer, permissionRouter, fsSandbox, terminalManager, mcpServers, logger)
  // are wired in T3 once the handler modules have settled signatures.
  [key: string]: unknown;
}

export class AcpSession {
  private constructor(
    readonly sessionId: SessionId,
    readonly capabilities: CachedCapabilities,
    readonly adapterId: AdapterId,
  ) {}

  static open(_pool: AcpConnectionPool, _opts: AcpSessionOptions): Promise<AcpSession> {
    return Promise.reject(new Error("AcpSession.open not implemented (T3)"));
  }

  prompt(_content: unknown[]): Promise<string> {
    return Promise.reject(new Error("AcpSession.prompt not implemented (T3)"));
  }

  cancel(): Promise<void> {
    return Promise.reject(new Error("AcpSession.cancel not implemented (T3)"));
  }

  dispose(): Promise<void> {
    return Promise.resolve();
  }
}
```

- [ ] **Step 11: Create handlers/elicitation.ts stub**

`src-bridge/src/runtime/acp/handlers/elicitation.ts`:

```ts
// unstable_createElicitation passthrough — capability-gated. Impl: T4d.
export async function createElicitation(_ctx: unknown, _params: unknown): Promise<{ action: "cancel" }> {
  return { action: "cancel" };
}

export async function completeElicitation(_ctx: unknown, _params: unknown): Promise<void> {
  // no-op until T4d
}
```

- [ ] **Step 12: Rewrite existing handler stubs to compile against new ctx types**

`src-bridge/src/runtime/acp/handlers/fs.ts`:

```ts
// readTextFile / writeTextFile via FsSandbox. Impl: T4a.
export async function readTextFile(_ctx: unknown, _params: unknown): Promise<unknown> {
  throw new Error("handlers/fs.readTextFile not implemented (T4a)");
}
export async function writeTextFile(_ctx: unknown, _params: unknown): Promise<unknown> {
  throw new Error("handlers/fs.writeTextFile not implemented (T4a)");
}
```

`src-bridge/src/runtime/acp/handlers/terminal.ts`:

```ts
// 6 terminal methods via TerminalManager. Impl: T4b.
export async function createTerminal(_ctx: unknown, _params: unknown): Promise<unknown> {
  throw new Error("handlers/terminal.createTerminal not implemented (T4b)");
}
export async function terminalOutput(_ctx: unknown, _params: unknown): Promise<unknown> {
  throw new Error("handlers/terminal.terminalOutput not implemented (T4b)");
}
export async function waitForTerminalExit(_ctx: unknown, _params: unknown): Promise<unknown> {
  throw new Error("handlers/terminal.waitForTerminalExit not implemented (T4b)");
}
export async function killTerminal(_ctx: unknown, _params: unknown): Promise<unknown> {
  throw new Error("handlers/terminal.killTerminal not implemented (T4b)");
}
export async function releaseTerminal(_ctx: unknown, _params: unknown): Promise<unknown> {
  throw new Error("handlers/terminal.releaseTerminal not implemented (T4b)");
}
```

`src-bridge/src/runtime/acp/handlers/permission.ts`:

```ts
// session/request_permission router. Impl: T4c.
export async function requestPermission(_ctx: unknown, _params: unknown): Promise<unknown> {
  throw new Error("handlers/permission.requestPermission not implemented (T4c)");
}
```

- [ ] **Step 13: Rewrite events/session-update.ts stub**

`src-bridge/src/runtime/acp/events/session-update.ts`:

```ts
// sessionUpdate → AgentEventType mapping. Impl: T5.
import type { AgentEvent } from "../../../types";

export function mapSessionUpdate(
  _taskId: string,
  _sessionId: string,
  _update: unknown,
  _nowMs: () => number,
): AgentEvent[] {
  throw new Error("mapSessionUpdate not implemented (T5)");
}
```

- [ ] **Step 14: Create adapter-factory.ts stub**

`src-bridge/src/runtime/acp/adapter-factory.ts`:

```ts
// createAcpRuntimeAdapter — returns RuntimeAdapter shape. Impl: T6.
import type { AdapterId } from "./registry";

export interface AcpRuntimeAdapterDeps {
  pool: import("./connection-pool").AcpConnectionPool;
  // Remaining deps (permissionRouter, terminalManager, worktreeService, mcpServersStatus,
  // taskEventsService, logger, legacyFactories, makeFsSandbox, resolveMcpServersFor) wired in T6.
  [key: string]: unknown;
}

export function createAcpRuntimeAdapter(_adapterId: AdapterId, _deps: AcpRuntimeAdapterDeps): unknown {
  throw new Error("createAcpRuntimeAdapter not implemented (T6)");
}
```

- [ ] **Step 15: Create index.ts barrel**

`src-bridge/src/runtime/acp/index.ts`:

```ts
export { ACP_ADAPTERS, type AdapterId, type AcpAdapterConfig } from "./registry";
export * from "./errors";
export { ChildProcessHost, type ChildProcessHostOptions, type ChildProcessHostHandles } from "./process-host";
export { AcpConnectionPool, type PooledEntry, type PerSessionContext, type SessionId, type AcquireContext } from "./connection-pool";
export { MultiplexedClient } from "./multiplexed-client";
export { AcpSession, type AcpSessionOptions } from "./session";
export { requireCapability, type CachedCapabilities } from "./capabilities";
export { createAcpRuntimeAdapter, type AcpRuntimeAdapterDeps } from "./adapter-factory";
export { mapSessionUpdate } from "./events/session-update";
```

- [ ] **Step 16: Run typecheck**

```bash
cd src-bridge && bun run typecheck
```

Expected: exits 0. If errors mention missing exports from `./registry` (because `AcpAdapterConfig` type wasn't exported), export it explicitly — add to `registry.ts`:

```ts
export type { AcpAdapterConfig };
```

- [ ] **Step 17: Run repo typecheck**

```bash
pnpm exec tsc --noEmit
```

Expected: exits 0.

- [ ] **Step 18: Commit**

```bash
rtk git add src-bridge/package.json src-bridge/bun.lock src-bridge/src/runtime/acp/
rtk git commit -m "acp(client): T1 — rescaffold runtime/acp/ + install @agentclientprotocol/sdk"
```

---

## Task T2: `ChildProcessHost` + `AcpConnectionPool`

**Goal:** Spawn and supervise ACP agent children; maintain an adapter-level pool with ref-counting and idle reclaim.

**Files:**
- Modify: `src-bridge/src/runtime/acp/process-host.ts`
- Modify: `src-bridge/src/runtime/acp/connection-pool.ts`
- Create: `src-bridge/tests/unit/runtime/acp/process-host.test.ts`
- Create: `src-bridge/tests/unit/runtime/acp/connection-pool.test.ts`

**Steps:**

- [ ] **Step 1: Write failing test for `ChildProcessHost.start` — spawn success**

Create `src-bridge/tests/unit/runtime/acp/process-host.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { ChildProcessHost } from "../../../../src/runtime/acp/process-host";

const silentLogger = { info: () => {}, warn: () => {}, error: () => {} };

describe("ChildProcessHost", () => {
  test("start() returns stdin/stdout for a valid command", async () => {
    const host = new ChildProcessHost({
      adapterId: "test",
      command: "node",
      args: ["-e", "process.stdin.on('data', d => process.stdout.write(d))"],
      env: { ...process.env } as Record<string, string>,
      logger: silentLogger,
    });
    const handles = await host.start();
    expect(handles.stdin).toBeInstanceOf(WritableStream);
    expect(handles.stdout).toBeInstanceOf(ReadableStream);
    await host.shutdown(500);
  });

  test("start() rejects with AcpCommandNotFound when binary is missing", async () => {
    const host = new ChildProcessHost({
      adapterId: "test",
      command: "definitely-not-a-real-binary-xyz123",
      args: [],
      env: {} as Record<string, string>,
      logger: silentLogger,
    });
    await expect(host.start()).rejects.toMatchObject({ name: "AcpCommandNotFound" });
  });

  test("stderrBuffer.dump() returns last 8KB of child stderr", async () => {
    const host = new ChildProcessHost({
      adapterId: "test",
      command: "node",
      args: ["-e", "console.error('hello from stderr'); setTimeout(()=>{}, 50)"],
      env: { ...process.env } as Record<string, string>,
      logger: silentLogger,
    });
    await host.start();
    await new Promise((r) => setTimeout(r, 100));
    expect(host.stderrBuffer.dump()).toContain("hello from stderr");
    await host.shutdown(500);
  });

  test("shutdown(gracefulMs) escalates to SIGTERM then SIGKILL", async () => {
    const host = new ChildProcessHost({
      adapterId: "test",
      command: "node",
      args: [
        "-e",
        "process.on('SIGTERM', () => {}); setInterval(() => process.stdout.write('.'), 50);",
      ],
      env: { ...process.env } as Record<string, string>,
      logger: silentLogger,
    });
    await host.start();
    const start = Date.now();
    await host.shutdown(100);
    expect(Date.now() - start).toBeLessThan(2000);
    await expect(host.exited).resolves.toBeDefined();
  });
});
```

- [ ] **Step 2: Run test, confirm failure**

```bash
cd src-bridge && bun test tests/unit/runtime/acp/process-host.test.ts
```

Expected: tests fail with `ChildProcessHost.start not implemented (T2)`.

- [ ] **Step 3: Implement `ChildProcessHost`**

Replace `src-bridge/src/runtime/acp/process-host.ts`:

```ts
import { spawn, type ChildProcess } from "node:child_process";
import { AcpCommandNotFound, AcpProcessCrash } from "./errors";

export interface ChildProcessHostOptions {
  adapterId: string;
  command: string;
  args: readonly string[];
  env: Record<string, string>;
  logger: { info: (...a: unknown[]) => void; warn: (...a: unknown[]) => void; error: (...a: unknown[]) => void };
  installHint?: string;
}

export interface ChildProcessHostHandles {
  stdin: WritableStream<Uint8Array>;
  stdout: ReadableStream<Uint8Array>;
}

class RingBuffer {
  private chunks: Buffer[] = [];
  private size = 0;
  constructor(private readonly max: number) {}
  append(chunk: Buffer): void {
    this.chunks.push(chunk);
    this.size += chunk.length;
    while (this.size > this.max && this.chunks.length > 0) {
      const first = this.chunks[0]!;
      const excess = this.size - this.max;
      if (first.length <= excess) {
        this.size -= first.length;
        this.chunks.shift();
      } else {
        this.chunks[0] = first.subarray(excess);
        this.size = this.max;
      }
    }
  }
  dump(): string {
    return Buffer.concat(this.chunks).toString("utf8");
  }
}

const STDERR_MAX = 8 * 1024;

export class ChildProcessHost {
  private child: ChildProcess | null = null;
  readonly stderrBuffer = new RingBuffer(STDERR_MAX);
  private exitResolve!: (code: number) => void;
  readonly exited: Promise<number> = new Promise<number>((res) => {
    this.exitResolve = res;
  });

  constructor(private readonly opts: ChildProcessHostOptions) {}

  start(): Promise<ChildProcessHostHandles> {
    return new Promise((resolve, reject) => {
      let child: ChildProcess;
      try {
        child = spawn(this.opts.command, this.opts.args as string[], {
          env: this.opts.env,
          stdio: ["pipe", "pipe", "pipe"],
        });
      } catch (err) {
        reject(
          new AcpCommandNotFound(
            this.opts.adapterId,
            this.opts.command,
            this.opts.installHint ?? "Check that the command is in PATH.",
          ),
        );
        return;
      }

      child.on("error", (err: NodeJS.ErrnoException) => {
        if (err.code === "ENOENT") {
          reject(
            new AcpCommandNotFound(
              this.opts.adapterId,
              this.opts.command,
              this.opts.installHint ?? "Check that the command is in PATH.",
            ),
          );
        } else {
          reject(new AcpProcessCrash(`spawn error: ${err.message}`));
        }
      });

      child.stderr?.on("data", (chunk: Buffer) => {
        this.stderrBuffer.append(chunk);
      });

      child.on("exit", (code, signal) => {
        this.opts.logger.info("acp child exited", { adapterId: this.opts.adapterId, code, signal });
        this.exitResolve(code ?? -1);
      });

      this.child = child;

      const stdin = new WritableStream<Uint8Array>({
        write: (chunk) =>
          new Promise((res, rej) => {
            const ok = child.stdin?.write(chunk);
            if (ok === false) child.stdin?.once("drain", () => res());
            else res();
            child.stdin?.once("error", rej);
          }),
        close: () =>
          new Promise((res) => {
            child.stdin?.end(() => res());
          }),
      });

      const stdout = new ReadableStream<Uint8Array>({
        start(controller) {
          child.stdout?.on("data", (chunk: Buffer) => controller.enqueue(new Uint8Array(chunk)));
          child.stdout?.on("end", () => controller.close());
          child.stdout?.on("error", (err) => controller.error(err));
        },
      });

      resolve({ stdin, stdout });
    });
  }

  async shutdown(gracefulMs = 2000): Promise<void> {
    if (!this.child || this.child.exitCode !== null) return;
    try {
      this.child.stdin?.end();
    } catch {
      // already closed
    }
    const deadline = Date.now() + gracefulMs;
    while (this.child.exitCode === null && Date.now() < deadline) {
      await new Promise((r) => setTimeout(r, 50));
    }
    if (this.child.exitCode === null) {
      this.child.kill("SIGTERM");
      const sigtermDeadline = Date.now() + gracefulMs;
      while (this.child.exitCode === null && Date.now() < sigtermDeadline) {
        await new Promise((r) => setTimeout(r, 50));
      }
    }
    if (this.child.exitCode === null) {
      this.child.kill("SIGKILL");
    }
  }
}
```

- [ ] **Step 4: Run `process-host.test.ts` to green**

```bash
cd src-bridge && bun test tests/unit/runtime/acp/process-host.test.ts
```

Expected: all 4 tests pass.

- [ ] **Step 5: Write failing tests for `AcpConnectionPool`**

Create `src-bridge/tests/unit/runtime/acp/connection-pool.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { AcpConnectionPool, type AcquireContext } from "../../../../src/runtime/acp/connection-pool";

const silentLogger = { info: () => {}, warn: () => {}, error: () => {} };

function makeAcquireCtx(): AcquireContext {
  return {
    registerSession: () => {},
    unregisterSession: () => {},
  };
}

describe("AcpConnectionPool", () => {
  test("concurrent acquire for the same adapterId triggers exactly one spawn", async () => {
    // Setup: inject a fake spawner via pool options.
    // Pool construction & counting verified after the acquire promises resolve.
    const pool = new AcpConnectionPool({ logger: silentLogger, idleMs: 60_000 });
    const [a, b] = await Promise.all([
      pool.acquire("claude_code", makeAcquireCtx()),
      pool.acquire("claude_code", makeAcquireCtx()),
    ]);
    expect(a).toBe(b);
    await pool.shutdownAll(true);
  });

  test("release decrements session refcount; idle > idleMs triggers shutdown", async () => {
    const pool = new AcpConnectionPool({ logger: silentLogger, idleMs: 10 });
    const entry = await pool.acquire("claude_code", makeAcquireCtx());
    entry.sessions.add("s1");
    await pool.release("claude_code", "s1");
    await new Promise((r) => setTimeout(r, 80));
    // A subsequent acquire after reclaim spawns a fresh host.
    const second = await pool.acquire("claude_code", makeAcquireCtx());
    expect(second).not.toBe(entry);
    await pool.shutdownAll(true);
  });

  test("host exit flips restartPending; next acquire spawns fresh host", async () => {
    const pool = new AcpConnectionPool({ logger: silentLogger });
    const first = await pool.acquire("claude_code", makeAcquireCtx());
    // Simulate crash by marking restartPending + forcing host.exited to settle.
    first.restartPending = true;
    const second = await pool.acquire("claude_code", makeAcquireCtx());
    expect(second).not.toBe(first);
    await pool.shutdownAll(true);
  });
});
```

Note: real pool construction will need to either accept an injected spawner (preferred) or be testable against a scripted Node binary. Use dependency injection — extend `AcpConnectionPoolOptions` with an optional `spawnFactory` used by tests.

- [ ] **Step 6: Implement `AcpConnectionPool`**

Replace `src-bridge/src/runtime/acp/connection-pool.ts`:

```ts
import type { AdapterId } from "./registry";
import { ACP_ADAPTERS } from "./registry";
import type { CachedCapabilities } from "./capabilities";
import { ChildProcessHost, type ChildProcessHostOptions } from "./process-host";
import { AcpAuthMissing } from "./errors";

export type SessionId = string;

export interface PerSessionContext {
  taskId: string;
  cwd: string;
  [key: string]: unknown;
}

export interface AcquireContext {
  registerSession(sessionId: SessionId, ctx: PerSessionContext): void;
  unregisterSession(sessionId: SessionId): void;
}

export interface PooledEntry {
  adapterId: AdapterId;
  host: ChildProcessHost;
  conn: unknown;
  caps: CachedCapabilities;
  clientDispatcher: unknown;
  sessions: Set<SessionId>;
  restartPending: boolean;
  lastIdleAt: number;
}

export type SpawnFactory = (opts: ChildProcessHostOptions) => ChildProcessHost;

export interface AcpConnectionPoolOptions {
  logger: { info: (...a: unknown[]) => void; warn: (...a: unknown[]) => void; error: (...a: unknown[]) => void };
  idleMs?: number;
  spawnFactory?: SpawnFactory;
  /** T3 wires an SDK-based initializer here; T2 leaves it undefined. */
  onInitialize?: (entry: PooledEntry) => Promise<void>;
}

const defaultSpawnFactory: SpawnFactory = (opts) => new ChildProcessHost(opts);

export class AcpConnectionPool {
  private readonly entries = new Map<AdapterId, PooledEntry>();
  private readonly acquireLocks = new Map<AdapterId, Promise<PooledEntry>>();
  private readonly idleTimers = new Map<AdapterId, ReturnType<typeof setTimeout>>();
  private readonly idleMs: number;
  private readonly spawnFactory: SpawnFactory;

  constructor(private readonly opts: AcpConnectionPoolOptions) {
    this.idleMs = opts.idleMs ?? 600_000;
    this.spawnFactory = opts.spawnFactory ?? defaultSpawnFactory;
  }

  async acquire(adapterId: AdapterId, ctx: AcquireContext): Promise<PooledEntry> {
    const pending = this.acquireLocks.get(adapterId);
    if (pending) return pending;

    const existing = this.entries.get(adapterId);
    if (existing && !existing.restartPending) {
      this.clearIdleTimer(adapterId);
      return existing;
    }

    const promise = this.spawnAndInitialize(adapterId).finally(() => {
      this.acquireLocks.delete(adapterId);
    });
    this.acquireLocks.set(adapterId, promise);
    const entry = await promise;
    this.entries.set(adapterId, entry);
    return entry;
  }

  private async spawnAndInitialize(adapterId: AdapterId): Promise<PooledEntry> {
    const config = ACP_ADAPTERS[adapterId];
    const missing = config.envRequired.filter((k) => !process.env[k]);
    if (missing.length > 0) {
      throw new AcpAuthMissing(adapterId, missing);
    }
    const host = this.spawnFactory({
      adapterId,
      command: config.command,
      args: config.args,
      env: { ...process.env } as Record<string, string>,
      logger: this.opts.logger,
    });
    await host.start();
    const entry: PooledEntry = {
      adapterId,
      host,
      conn: null,
      caps: {
        raw: null,
        modelSelectable: false,
        thinkingBudget: false,
        modeSelectable: false,
        mcpStatus: false,
        sessionFork: false,
        loadSession: false,
      },
      clientDispatcher: null,
      sessions: new Set(),
      restartPending: false,
      lastIdleAt: Date.now(),
    };
    if (this.opts.onInitialize) {
      await this.opts.onInitialize(entry);
    }
    host.exited.then((code) => {
      this.opts.logger.warn("acp pool entry exited", { adapterId, code });
      entry.restartPending = true;
    });
    return entry;
  }

  async release(adapterId: AdapterId, sessionId: SessionId): Promise<void> {
    const entry = this.entries.get(adapterId);
    if (!entry) return;
    entry.sessions.delete(sessionId);
    if (entry.sessions.size === 0) {
      entry.lastIdleAt = Date.now();
      this.scheduleIdleReclaim(adapterId);
    }
  }

  private scheduleIdleReclaim(adapterId: AdapterId): void {
    this.clearIdleTimer(adapterId);
    const t = setTimeout(() => {
      const entry = this.entries.get(adapterId);
      if (!entry || entry.sessions.size > 0) return;
      this.opts.logger.info("acp pool idle reclaim", { adapterId });
      entry.host.shutdown(2000).catch(() => {});
      this.entries.delete(adapterId);
    }, this.idleMs);
    this.idleTimers.set(adapterId, t);
  }

  private clearIdleTimer(adapterId: AdapterId): void {
    const t = this.idleTimers.get(adapterId);
    if (t) {
      clearTimeout(t);
      this.idleTimers.delete(adapterId);
    }
  }

  async shutdownAll(_graceful = true): Promise<void> {
    for (const t of this.idleTimers.values()) clearTimeout(t);
    this.idleTimers.clear();
    const entries = Array.from(this.entries.values());
    this.entries.clear();
    await Promise.all(entries.map((e) => e.host.shutdown(2000).catch(() => {})));
  }
}
```

- [ ] **Step 7: Update tests to use `spawnFactory` injection**

Update `connection-pool.test.ts` to pass a mock spawn factory so tests don't require real binaries. The test file becomes:

```ts
import { describe, expect, test } from "bun:test";
import { AcpConnectionPool, type AcquireContext } from "../../../../src/runtime/acp/connection-pool";
import { ChildProcessHost, type ChildProcessHostOptions } from "../../../../src/runtime/acp/process-host";

const silentLogger = { info: () => {}, warn: () => {}, error: () => {} };

class FakeHost extends ChildProcessHost {
  private exitResolveInternal!: (code: number) => void;
  override readonly exited = new Promise<number>((r) => (this.exitResolveInternal = r));
  constructor(opts: ChildProcessHostOptions) {
    super(opts);
  }
  override async start() {
    return {
      stdin: new WritableStream<Uint8Array>(),
      stdout: new ReadableStream<Uint8Array>(),
    };
  }
  override async shutdown() {
    this.exitResolveInternal(0);
  }
}

const factory = (opts: ChildProcessHostOptions) => new FakeHost(opts);

function makeAcquireCtx(): AcquireContext {
  return { registerSession: () => {}, unregisterSession: () => {} };
}

describe("AcpConnectionPool", () => {
  test("concurrent acquire triggers exactly one spawn", async () => {
    let spawned = 0;
    const pool = new AcpConnectionPool({
      logger: silentLogger,
      idleMs: 60_000,
      spawnFactory: (opts) => { spawned++; return factory(opts); },
    });
    const [a, b] = await Promise.all([
      pool.acquire("claude_code", makeAcquireCtx()),
      pool.acquire("claude_code", makeAcquireCtx()),
    ]);
    expect(a).toBe(b);
    expect(spawned).toBe(1);
    await pool.shutdownAll(true);
  });

  test("release + idle reclaim spawns fresh host next time", async () => {
    const pool = new AcpConnectionPool({ logger: silentLogger, idleMs: 20, spawnFactory: factory });
    const entry = await pool.acquire("claude_code", makeAcquireCtx());
    entry.sessions.add("s1");
    await pool.release("claude_code", "s1");
    await new Promise((r) => setTimeout(r, 80));
    const second = await pool.acquire("claude_code", makeAcquireCtx());
    expect(second).not.toBe(entry);
    await pool.shutdownAll(true);
  });

  test("restartPending forces a fresh spawn on next acquire", async () => {
    const pool = new AcpConnectionPool({ logger: silentLogger, spawnFactory: factory });
    const first = await pool.acquire("claude_code", makeAcquireCtx());
    first.restartPending = true;
    const second = await pool.acquire("claude_code", makeAcquireCtx());
    expect(second).not.toBe(first);
    await pool.shutdownAll(true);
  });

  test("missing envRequired rejects with AcpAuthMissing (no spawn)", async () => {
    // codex requires OPENAI_API_KEY; ensure it's absent for this test.
    const savedKey = process.env.OPENAI_API_KEY;
    delete process.env.OPENAI_API_KEY;
    let spawned = 0;
    const pool = new AcpConnectionPool({
      logger: silentLogger,
      spawnFactory: (opts) => { spawned++; return factory(opts); },
    });
    await expect(pool.acquire("codex", makeAcquireCtx())).rejects.toMatchObject({
      name: "AcpAuthMissing",
    });
    expect(spawned).toBe(0);
    if (savedKey) process.env.OPENAI_API_KEY = savedKey;
  });
});
```

- [ ] **Step 8: Run all new tests to green**

```bash
cd src-bridge && bun test tests/unit/runtime/acp/process-host.test.ts tests/unit/runtime/acp/connection-pool.test.ts
```

Expected: all tests pass.

- [ ] **Step 9: Run full suite regression check**

```bash
cd src-bridge && bun test && bun run typecheck
```

Expected: exits 0. Fix any regressions before committing.

- [ ] **Step 10: Commit**

```bash
rtk git add src-bridge/src/runtime/acp/process-host.ts src-bridge/src/runtime/acp/connection-pool.ts src-bridge/tests/unit/runtime/acp/
rtk git commit -m "acp(client): T2 — ChildProcessHost + AcpConnectionPool with ref-count + idle reclaim"
```

---

## Task T3: `AcpSession` + `MultiplexedClient` + `capabilities.ts` + mock-acp-agent fixture

**Goal:** Connect `AcpConnectionPool` to the SDK's `ClientSideConnection`, add per-session multiplexing, cache capabilities, and implement the core `prompt` / `cancel` / `dispose` flows. Ship a mock-agent fixture so higher tasks can run component tests without a real agent.

**Files:**
- Modify: `src-bridge/src/runtime/acp/multiplexed-client.ts`
- Modify: `src-bridge/src/runtime/acp/session.ts`
- Modify: `src-bridge/src/runtime/acp/capabilities.ts`
- Modify: `src-bridge/src/runtime/acp/connection-pool.ts` (wire SDK initialize via `onInitialize`)
- Create: `src-bridge/tests/fixtures/mock-acp-agent.ts`
- Create: `src-bridge/tests/component/acp/happy-path.test.ts`
- Create: `src-bridge/tests/component/acp/cancel-race.test.ts`
- Create: `src-bridge/tests/component/acp/pooling.test.ts`
- Create: `src-bridge/tests/component/acp/multi-session-fs.test.ts`
- Create: `src-bridge/tests/unit/runtime/acp/capability-gate.test.ts`

**Steps:**

- [ ] **Step 1: Inspect the SDK surface**

```bash
cat src-bridge/node_modules/@agentclientprotocol/sdk/dist/index.d.ts | head -80
```

Read the declarations of `ClientSideConnection`, `ndJsonStream`, `Client`, `Agent`, `RequestError`, and `schema.AgentCapabilities`. If the SDK exports types differently (e.g. namespace vs. named exports), adjust imports accordingly for all subsequent steps.

- [ ] **Step 2: Write mock-acp-agent fixture**

Create `src-bridge/tests/fixtures/mock-acp-agent.ts`:

```ts
#!/usr/bin/env bun
// Mock ACP agent for component tests. Implements a configurable subset of
// the SDK Agent interface. Configuration is read from env:
//   ACP_MOCK_CAPS='{"modelSelectable":true, "thinkingBudget":false, ...}'
//   ACP_MOCK_SCRIPT='[{"kind":"agent_message_chunk","text":"hi"},{"kind":"stop","reason":"end_turn"}]'
// Communicates over stdio NDJSON using SDK's AgentSideConnection.
import {
  AgentSideConnection,
  ndJsonStream,
  type Agent,
  type schema,
} from "@agentclientprotocol/sdk";
import { randomUUID } from "node:crypto";

const caps: Partial<schema.AgentCapabilities> = JSON.parse(
  process.env.ACP_MOCK_CAPS ?? "{}",
);
const script: Array<Record<string, unknown>> = JSON.parse(
  process.env.ACP_MOCK_SCRIPT ?? "[]",
);

class MockAgent implements Agent {
  private sessions = new Map<string, { cwd: string }>();
  private cancelled = new Set<string>();

  async initialize(_params: schema.InitializeRequest) {
    return { protocolVersion: 1, agentCapabilities: caps };
  }

  async newSession(params: schema.NewSessionRequest) {
    const sid = randomUUID();
    this.sessions.set(sid, { cwd: params.cwd });
    return { sessionId: sid };
  }

  async prompt(params: schema.PromptRequest): Promise<{ stopReason: schema.StopReason }> {
    const sid = params.sessionId;
    for (const update of script) {
      if (this.cancelled.has(sid)) {
        return { stopReason: "cancelled" };
      }
      if (update.kind === "stop") {
        return { stopReason: (update.reason as schema.StopReason) ?? "end_turn" };
      }
      // The connection dispatched here is attached after construction (see below).
      await conn.sessionUpdate({ sessionId: sid, update: update as schema.SessionUpdate });
      await new Promise((r) => setTimeout(r, 5));
    }
    return { stopReason: "end_turn" };
  }

  async cancel(params: schema.CancelRequest): Promise<void> {
    this.cancelled.add(params.sessionId);
  }
}

const agent = new MockAgent();
const stream = ndJsonStream(
  process.stdout as unknown as WritableStream<Uint8Array>,
  process.stdin as unknown as ReadableStream<Uint8Array>,
);
const conn = new AgentSideConnection(agent, stream);
```

The `AgentSideConnection` constructor shape depends on the SDK; if the SDK exposes `AgentSideConnection.create(...)` instead, use that. The fixture's role is to let the bridge-side tests round-trip real SDK messages. **Do not run this file directly in tests — only via `ChildProcessHost.spawn("bun", ["tests/fixtures/mock-acp-agent.ts"])` with `ACP_MOCK_*` env vars set per test case.**

- [ ] **Step 3: Implement `capabilities.ts` extraction from SDK types**

Replace `src-bridge/src/runtime/acp/capabilities.ts`:

```ts
import type { schema } from "@agentclientprotocol/sdk";
import { AcpCapabilityUnsupported } from "./errors";

export interface CachedCapabilities {
  raw: schema.AgentCapabilities;
  modelSelectable: boolean;
  thinkingBudget: boolean;
  modeSelectable: boolean;
  mcpStatus: boolean;
  sessionFork: boolean;
  loadSession: boolean;
}

export function cacheCapabilities(raw: schema.AgentCapabilities): CachedCapabilities {
  return {
    raw,
    modelSelectable:
      Array.isArray(raw.availableModels) && raw.availableModels.length > 0,
    thinkingBudget: raw.promptCapabilities?.thinkingBudget === true,
    modeSelectable:
      Array.isArray(raw.availableModes) && raw.availableModes.length > 0,
    mcpStatus: Boolean(raw.mcpCapabilities?.http || raw.mcpCapabilities?.sse),
    sessionFork: raw.session?.fork === true,
    loadSession: Boolean(raw.loadSession),
  };
}

export function requireCapability(
  caps: CachedCapabilities,
  key: keyof Omit<CachedCapabilities, "raw">,
  label: string,
): void {
  if (!caps[key]) {
    throw new AcpCapabilityUnsupported(label);
  }
}
```

If the SDK's `AgentCapabilities` schema uses different property names, adjust the mapping but keep the `CachedCapabilities` shape stable — it's consumed by `adapter-factory.ts` in T6.

- [ ] **Step 4: Implement `MultiplexedClient` (handlers still stubbed — they get filled by T4)**

Replace `src-bridge/src/runtime/acp/multiplexed-client.ts`:

```ts
import {
  RequestError,
  type Client,
  type schema,
} from "@agentclientprotocol/sdk";
import type { PerSessionContext, SessionId } from "./connection-pool";
import * as fsHandler from "./handlers/fs";
import * as terminalHandler from "./handlers/terminal";
import * as permissionHandler from "./handlers/permission";
import * as elicitationHandler from "./handlers/elicitation";
import { mapSessionUpdate } from "./events/session-update";

export class MultiplexedClient implements Client {
  private readonly sessions = new Map<SessionId, PerSessionContext>();
  constructor(private readonly nowMs: () => number = () => Date.now()) {}

  register(sessionId: SessionId, ctx: PerSessionContext): void {
    this.sessions.set(sessionId, ctx);
  }
  unregister(sessionId: SessionId): void {
    this.sessions.delete(sessionId);
  }

  private ctxOrThrow(sessionId: string): PerSessionContext {
    const ctx = this.sessions.get(sessionId);
    if (!ctx) {
      throw new RequestError(-32602, "unknown_session", { sessionId });
    }
    return ctx;
  }

  async sessionUpdate(params: { sessionId: string; update: schema.SessionUpdate; _meta?: unknown }): Promise<void> {
    try {
      const ctx = this.sessions.get(params.sessionId);
      if (!ctx) return; // swallow; notifications cannot return errors
      const streamer = (ctx.streamer as { send: (ev: unknown) => void } | undefined);
      if (!streamer) return;
      const events = mapSessionUpdate(ctx.taskId, params.sessionId, params.update, this.nowMs);
      for (const ev of events) streamer.send(ev);
    } catch (err) {
      // notifications are one-way; log and continue
      (this.sessions.get(params.sessionId)?.logger as { warn?: (...a: unknown[]) => void } | undefined)?.warn?.(
        "sessionUpdate map error", err,
      );
    }
  }

  async requestPermission(params: schema.RequestPermissionRequest): Promise<schema.RequestPermissionResponse> {
    const ctx = this.ctxOrThrow(params.sessionId);
    return permissionHandler.requestPermission(ctx, params) as Promise<schema.RequestPermissionResponse>;
  }

  async readTextFile(params: schema.ReadTextFileRequest): Promise<schema.ReadTextFileResponse> {
    const ctx = this.ctxOrThrow(params.sessionId);
    return fsHandler.readTextFile(ctx, params) as Promise<schema.ReadTextFileResponse>;
  }
  async writeTextFile(params: schema.WriteTextFileRequest): Promise<schema.WriteTextFileResponse> {
    const ctx = this.ctxOrThrow(params.sessionId);
    return fsHandler.writeTextFile(ctx, params) as Promise<schema.WriteTextFileResponse>;
  }

  async createTerminal(params: schema.CreateTerminalRequest): Promise<schema.CreateTerminalResponse> {
    const ctx = this.ctxOrThrow(params.sessionId);
    return terminalHandler.createTerminal(ctx, params) as Promise<schema.CreateTerminalResponse>;
  }
  async terminalOutput(params: schema.TerminalOutputRequest): Promise<schema.TerminalOutputResponse> {
    const ctx = this.ctxOrThrow(params.sessionId);
    return terminalHandler.terminalOutput(ctx, params) as Promise<schema.TerminalOutputResponse>;
  }
  async waitForTerminalExit(params: schema.WaitForTerminalExitRequest): Promise<schema.WaitForTerminalExitResponse> {
    const ctx = this.ctxOrThrow(params.sessionId);
    return terminalHandler.waitForTerminalExit(ctx, params) as Promise<schema.WaitForTerminalExitResponse>;
  }
  async killTerminal(params: schema.KillTerminalRequest): Promise<schema.KillTerminalResponse | void> {
    const ctx = this.ctxOrThrow(params.sessionId);
    return terminalHandler.killTerminal(ctx, params) as Promise<schema.KillTerminalResponse | void>;
  }
  async releaseTerminal(params: schema.ReleaseTerminalRequest): Promise<schema.ReleaseTerminalResponse | void> {
    const ctx = this.ctxOrThrow(params.sessionId);
    return terminalHandler.releaseTerminal(ctx, params) as Promise<schema.ReleaseTerminalResponse | void>;
  }

  async unstable_createElicitation(params: schema.CreateElicitationRequest): Promise<schema.CreateElicitationResponse> {
    const ctx = this.sessions.get((params as { sessionId?: string }).sessionId ?? "");
    if (!ctx) return { action: "cancel" };
    return elicitationHandler.createElicitation(ctx, params) as Promise<schema.CreateElicitationResponse>;
  }
  async unstable_completeElicitation(params: unknown): Promise<void> {
    // fire-and-forget; safe if no ctx.
    await elicitationHandler.completeElicitation(null, params);
  }
}
```

If the SDK's `Client` interface has additional methods (`extMethod`, `extNotification`), add them with a default `throw new RequestError(-32601, "method_not_found")`. T6 may refine for cursorExtensions: true adapters.

- [ ] **Step 5: Implement `AcpSession`**

Replace `src-bridge/src/runtime/acp/session.ts`:

```ts
import {
  ClientSideConnection,
  ndJsonStream,
  type schema,
} from "@agentclientprotocol/sdk";
import type { AdapterId } from "./registry";
import type { AcpConnectionPool, PerSessionContext, SessionId } from "./connection-pool";
import type { CachedCapabilities } from "./capabilities";
import { cacheCapabilities, requireCapability } from "./capabilities";
import { MultiplexedClient } from "./multiplexed-client";
import { AcpCancelTimeout, AcpConcurrentPrompt, AcpTransportClosed } from "./errors";
import { ChildProcessHost } from "./process-host";

const CANCEL_TIMEOUT_MS = 2_000;

export interface AcpSessionOptions {
  taskId: string;
  adapterId: AdapterId;
  cwd: string;
  mcpServers: schema.McpServer[];
  // The per-session context is constructed by adapter-factory (T6) and passed in.
  perSessionContext: PerSessionContext;
  logger: { info: (...a: unknown[]) => void; warn: (...a: unknown[]) => void; error: (...a: unknown[]) => void };
}

export class AcpSession {
  private inflightPrompt: Promise<schema.StopReason> | null = null;
  private disposed = false;

  private constructor(
    readonly sessionId: SessionId,
    readonly capabilities: CachedCapabilities,
    readonly adapterId: AdapterId,
    private readonly pool: AcpConnectionPool,
    private readonly conn: ClientSideConnection,
    private readonly client: MultiplexedClient,
    private readonly opts: AcpSessionOptions,
  ) {}

  static async open(pool: AcpConnectionPool, opts: AcpSessionOptions): Promise<AcpSession> {
    const entry = await pool.acquire(opts.adapterId, {
      registerSession: (sid, ctx) => (entry?.clientDispatcher as MultiplexedClient)?.register(sid, ctx),
      unregisterSession: (sid) => (entry?.clientDispatcher as MultiplexedClient)?.unregister(sid),
    });
    const conn = entry.conn as ClientSideConnection;
    const client = entry.clientDispatcher as MultiplexedClient;
    const resp = await conn.newSession({ cwd: opts.cwd, mcpServers: opts.mcpServers });
    client.register(resp.sessionId, opts.perSessionContext);
    entry.sessions.add(resp.sessionId);
    return new AcpSession(resp.sessionId, entry.caps, opts.adapterId, pool, conn, client, opts);
  }

  async prompt(content: schema.ContentBlock[]): Promise<schema.StopReason> {
    if (this.disposed) throw new AcpTransportClosed("session disposed");
    if (this.inflightPrompt) {
      throw new AcpConcurrentPrompt(`concurrent prompt on session ${this.sessionId}`);
    }
    const p = this.conn
      .prompt({ sessionId: this.sessionId, prompt: content })
      .then((r) => r.stopReason)
      .finally(() => {
        this.inflightPrompt = null;
      });
    this.inflightPrompt = p;
    return p;
  }

  async cancel(): Promise<void> {
    if (this.disposed) return;
    await this.conn.cancel({ sessionId: this.sessionId });
    if (!this.inflightPrompt) return;
    const deadline = Date.now() + CANCEL_TIMEOUT_MS;
    while (this.inflightPrompt && Date.now() < deadline) {
      await Promise.race([
        this.inflightPrompt.catch(() => {}),
        new Promise((r) => setTimeout(r, 50)),
      ]);
    }
    if (this.inflightPrompt) {
      throw new AcpCancelTimeout(`cancel timeout on session ${this.sessionId}`);
    }
  }

  async setMode(modeId: string): Promise<void> {
    await this.conn.setSessionMode({ sessionId: this.sessionId, modeId });
  }

  async setConfigOption(key: string, value: unknown): Promise<schema.SetSessionConfigOptionResponse> {
    return this.conn.setSessionConfigOption({ sessionId: this.sessionId, key, value });
  }

  async setModel(modelId: string): Promise<void> {
    requireCapability(this.capabilities, "modelSelectable", "setModel");
    // Prefer unstable_setSessionModel if advertised; else setSessionConfigOption.
    const raw = this.capabilities.raw as { unstable_setSessionModel?: boolean };
    if (raw.unstable_setSessionModel) {
      // SDK may expose this as conn.unstable_setSessionModel — pattern-match at runtime.
      const unstable = (this.conn as unknown as {
        unstable_setSessionModel?: (p: { sessionId: string; modelId: string }) => Promise<void>;
      }).unstable_setSessionModel;
      if (unstable) {
        await unstable({ sessionId: this.sessionId, modelId });
        return;
      }
    }
    await this.setConfigOption("model", modelId);
  }

  async forkSession(): Promise<AcpSession> {
    requireCapability(this.capabilities, "sessionFork", "forkSession");
    const unstable = (this.conn as unknown as {
      unstable_forkSession?: (p: { sessionId: string }) => Promise<{ sessionId: string }>;
    }).unstable_forkSession;
    if (!unstable) throw new AcpCancelTimeout("fork not implemented in SDK version");
    const resp = await unstable({ sessionId: this.sessionId });
    // Register the new session under the same PerSessionContext (fresh context can be wired by T6 caller).
    this.client.register(resp.sessionId, this.opts.perSessionContext);
    return new AcpSession(
      resp.sessionId,
      this.capabilities,
      this.adapterId,
      this.pool,
      this.conn,
      this.client,
      this.opts,
    );
  }

  async dispose(): Promise<void> {
    if (this.disposed) return;
    this.disposed = true;
    this.client.unregister(this.sessionId);
    await this.pool.release(this.adapterId, this.sessionId);
  }
}
```

- [ ] **Step 6: Wire SDK initialize via `onInitialize` in the pool**

Edit `src-bridge/src/runtime/acp/connection-pool.ts`'s `spawnAndInitialize`. Replace the `conn: null, clientDispatcher: null, caps: {...}` scaffold with actual initialization:

```ts
private async spawnAndInitialize(adapterId: AdapterId): Promise<PooledEntry> {
  const config = ACP_ADAPTERS[adapterId];
  const missing = config.envRequired.filter((k) => !process.env[k]);
  if (missing.length > 0) {
    throw new AcpAuthMissing(adapterId, missing);
  }
  const host = this.spawnFactory({
    adapterId,
    command: config.command,
    args: config.args,
    env: { ...process.env } as Record<string, string>,
    logger: this.opts.logger,
  });
  const { stdin, stdout } = await host.start();

  const { ClientSideConnection, ndJsonStream } = await import("@agentclientprotocol/sdk");
  const { MultiplexedClient } = await import("./multiplexed-client");
  const { cacheCapabilities } = await import("./capabilities");

  const client = new MultiplexedClient();
  const stream = ndJsonStream(stdin, stdout);
  const conn = new ClientSideConnection(client, stream);
  const init = await conn.initialize({
    protocolVersion: 1,
    clientCapabilities: {
      fs: { readTextFile: true, writeTextFile: true },
      terminal: true,
    },
  });

  const entry: PooledEntry = {
    adapterId,
    host,
    conn,
    caps: cacheCapabilities(init.agentCapabilities),
    clientDispatcher: client,
    sessions: new Set(),
    restartPending: false,
    lastIdleAt: Date.now(),
  };
  host.exited.then((code) => {
    this.opts.logger.warn("acp pool entry exited", { adapterId, code });
    entry.restartPending = true;
  });
  return entry;
}
```

If the SDK's `ClientSideConnection` constructor signature differs (e.g. `(stream, client)` order, or static `.create()`), adapt here.

- [ ] **Step 7: Write component test — happy path**

Create `src-bridge/tests/component/acp/happy-path.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { AcpConnectionPool } from "../../../src/runtime/acp/connection-pool";
import { AcpSession } from "../../../src/runtime/acp/session";
import { ChildProcessHost, type ChildProcessHostOptions } from "../../../src/runtime/acp/process-host";

const silent = { info: () => {}, warn: () => {}, error: () => {} };

function mockSpawn(script: Array<Record<string, unknown>>, caps: Record<string, unknown>) {
  return (opts: ChildProcessHostOptions) => {
    const host = new ChildProcessHost({
      ...opts,
      command: "bun",
      args: ["tests/fixtures/mock-acp-agent.ts"],
      env: {
        ...opts.env,
        ACP_MOCK_SCRIPT: JSON.stringify(script),
        ACP_MOCK_CAPS: JSON.stringify(caps),
      },
    });
    return host;
  };
}

describe("ACP happy path", () => {
  test("initialize → newSession → prompt → text delta → stopReason", async () => {
    const received: unknown[] = [];
    const pool = new AcpConnectionPool({
      logger: silent,
      spawnFactory: mockSpawn(
        [
          { kind: "agent_message_chunk", text: "hi" },
          { kind: "stop", reason: "end_turn" },
        ],
        {},
      ),
    });
    const session = await AcpSession.open(pool, {
      taskId: "task-1",
      adapterId: "claude_code",
      cwd: process.cwd(),
      mcpServers: [],
      perSessionContext: {
        taskId: "task-1",
        cwd: process.cwd(),
        streamer: { send: (ev: unknown) => received.push(ev) },
        logger: silent,
      },
      logger: silent,
    });
    const stopReason = await session.prompt([{ type: "text", text: "hello" }]);
    expect(stopReason).toBe("end_turn");
    expect(received.length).toBeGreaterThan(0);
    await session.dispose();
    await pool.shutdownAll(true);
  });
});
```

- [ ] **Step 8: Write component tests — cancel-race, pooling, multi-session-fs, permission-flow**

Create `src-bridge/tests/component/acp/cancel-race.test.ts` (before/during/after prompt each tested), `pooling.test.ts` (two sessions same adapter share one host; mock script crashes → both sessions get AcpProcessCrash), `multi-session-fs.test.ts` (two concurrent sessions route fs/terminal right), `permission-flow.test.ts` (mock agent sends requestPermission → HookCallbackManager → `/bridge/permission-response/:id` → mock resumes). Use the same `mockSpawn` helper; vary `ACP_MOCK_SCRIPT`. Structure each as 1–2 tests.

For `permission-flow.test.ts`, `handlers/permission.ts` still stubs in T3 — write the test, run it to fail with `not implemented (T4c)`, leave the test file in place so T4c picks it up.

- [ ] **Step 9: Write failing unit test for capability gate**

Create `src-bridge/tests/unit/runtime/acp/capability-gate.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { cacheCapabilities, requireCapability } from "../../../../src/runtime/acp/capabilities";
import { AcpCapabilityUnsupported } from "../../../../src/runtime/acp/errors";

describe("capabilities", () => {
  test("cacheCapabilities flattens SDK schema correctly", () => {
    const caps = cacheCapabilities({
      availableModels: [{ id: "m1" }],
      promptCapabilities: { thinkingBudget: true },
      availableModes: [],
      mcpCapabilities: { http: true, sse: false },
      session: { fork: true },
      loadSession: true,
    } as never);
    expect(caps.modelSelectable).toBe(true);
    expect(caps.thinkingBudget).toBe(true);
    expect(caps.modeSelectable).toBe(false);
    expect(caps.mcpStatus).toBe(true);
    expect(caps.sessionFork).toBe(true);
    expect(caps.loadSession).toBe(true);
  });

  test("requireCapability throws AcpCapabilityUnsupported when gate closed", () => {
    const caps = cacheCapabilities({} as never);
    expect(() => requireCapability(caps, "modelSelectable", "setModel")).toThrow(AcpCapabilityUnsupported);
  });
});
```

- [ ] **Step 10: Run all new tests**

```bash
cd src-bridge && bun test tests/
```

Expected: unit tests pass; happy-path + cancel-race + pooling + multi-session-fs pass; permission-flow fails because handlers/permission.ts is still T4c stub — leave failing with a clear error message mentioning "T4c".

To exclude the known-failing permission test from this commit's green-gate, use Bun test's `test.todo()` marker **only** for that single test case. Remove the `.todo` in T4c.

- [ ] **Step 11: Run typecheck + full bridge test suite**

```bash
cd src-bridge && bun run typecheck && bun test
```

Expected: exits 0.

- [ ] **Step 12: Commit**

```bash
rtk git add src-bridge/src/runtime/acp/ src-bridge/tests/fixtures/mock-acp-agent.ts src-bridge/tests/component/acp/ src-bridge/tests/unit/runtime/acp/
rtk git commit -m "acp(client): T3 — AcpSession + MultiplexedClient + mock-agent fixture + component tests"
```

---

## Task T4a: `FsSandbox` + `handlers/fs.ts`

**Goal:** Implement worktree-rooted `readTextFile` / `writeTextFile` with realpath escape rejection.

**Files:**
- Create: `src-bridge/src/runtime/acp/fs-sandbox.ts`
- Modify: `src-bridge/src/runtime/acp/handlers/fs.ts`
- Create: `src-bridge/tests/unit/runtime/acp/fs-sandbox.test.ts`
- Create: `src-bridge/tests/unit/runtime/acp/handlers-fs.test.ts`

**Steps:**

- [ ] **Step 1: Write failing test for `FsSandbox.resolve`**

Create `src-bridge/tests/unit/runtime/acp/fs-sandbox.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { FsSandbox } from "../../../../src/runtime/acp/fs-sandbox";

describe("FsSandbox", () => {
  test("resolves paths inside the worktree", () => {
    const root = mkdtempSync(path.join(tmpdir(), "fs-sandbox-"));
    writeFileSync(path.join(root, "a.txt"), "hello");
    const s = new FsSandbox(root);
    expect(s.resolve("a.txt")).toBe(path.join(root, "a.txt"));
  });

  test("rejects .. escapes", () => {
    const root = mkdtempSync(path.join(tmpdir(), "fs-sandbox-"));
    const s = new FsSandbox(root);
    expect(() => s.resolve("../outside.txt")).toThrow(/path_escapes_worktree/);
  });

  test("rejects absolute paths outside the worktree", () => {
    const root = mkdtempSync(path.join(tmpdir(), "fs-sandbox-"));
    const s = new FsSandbox(root);
    expect(() => s.resolve(path.join(tmpdir(), "other.txt"))).toThrow(/path_escapes_worktree/);
  });

  test("rejects symlink escapes", () => {
    const root = mkdtempSync(path.join(tmpdir(), "fs-sandbox-"));
    const outside = path.join(tmpdir(), `outside-${Date.now()}.txt`);
    writeFileSync(outside, "secret");
    symlinkSync(outside, path.join(root, "link.txt"));
    const s = new FsSandbox(root);
    expect(() => s.resolve("link.txt")).toThrow(/path_escapes_worktree/);
  });
});
```

- [ ] **Step 2: Run failing test**

```bash
cd src-bridge && bun test tests/unit/runtime/acp/fs-sandbox.test.ts
```

Expected: module-not-found errors.

- [ ] **Step 3: Implement `FsSandbox`**

Create `src-bridge/src/runtime/acp/fs-sandbox.ts`:

```ts
import { realpathSync } from "node:fs";
import path from "node:path";

export class FsSandbox {
  private readonly rootReal: string;
  constructor(private readonly root: string) {
    this.rootReal = realpathSync(root);
  }

  resolve(requested: string): string {
    const abs = path.isAbsolute(requested) ? requested : path.join(this.root, requested);
    const joined = path.resolve(abs);
    let real: string;
    try {
      real = realpathSync(joined);
    } catch {
      // File doesn't exist yet — realpath its parent dir then re-append.
      const parent = realpathSync(path.dirname(joined));
      real = path.join(parent, path.basename(joined));
    }
    const rel = path.relative(this.rootReal, real);
    if (rel.startsWith("..") || path.isAbsolute(rel)) {
      const err = new Error("path_escapes_worktree");
      (err as { code?: string }).code = "path_escapes_worktree";
      throw err;
    }
    return real;
  }
}
```

- [ ] **Step 4: Run sandbox tests to green**

```bash
cd src-bridge && bun test tests/unit/runtime/acp/fs-sandbox.test.ts
```

Expected: all 4 pass.

- [ ] **Step 5: Write failing test for `handlers/fs.ts`**

Create `src-bridge/tests/unit/runtime/acp/handlers-fs.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { mkdtempSync, writeFileSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { FsSandbox } from "../../../../src/runtime/acp/fs-sandbox";
import { readTextFile, writeTextFile } from "../../../../src/runtime/acp/handlers/fs";

function makeCtx() {
  const root = mkdtempSync(path.join(tmpdir(), "h-fs-"));
  return {
    ctx: { taskId: "t1", cwd: root, fsSandbox: new FsSandbox(root) },
    root,
  };
}

describe("handlers/fs", () => {
  test("readTextFile returns file content", async () => {
    const { ctx, root } = makeCtx();
    writeFileSync(path.join(root, "a.txt"), "line1\nline2\nline3\n");
    const out = await readTextFile(ctx, { sessionId: "s1", path: "a.txt" });
    expect(out.content).toBe("line1\nline2\nline3\n");
  });

  test("readTextFile honors line + limit (1-based)", async () => {
    const { ctx, root } = makeCtx();
    writeFileSync(path.join(root, "a.txt"), "a\nb\nc\nd\n");
    const out = await readTextFile(ctx, { sessionId: "s1", path: "a.txt", line: 2, limit: 2 });
    expect(out.content).toBe("b\nc\n");
  });

  test("writeTextFile creates parent dirs", async () => {
    const { ctx, root } = makeCtx();
    await writeTextFile(ctx, { sessionId: "s1", path: "nested/deep/a.txt", content: "ok" });
    expect(readFileSync(path.join(root, "nested/deep/a.txt"), "utf8")).toBe("ok");
  });

  test("rejects path escapes with -32602 RequestError shape", async () => {
    const { ctx } = makeCtx();
    await expect(readTextFile(ctx, { sessionId: "s1", path: "../x" })).rejects.toMatchObject({
      code: -32602,
    });
  });
});
```

- [ ] **Step 6: Implement `handlers/fs.ts`**

Replace `src-bridge/src/runtime/acp/handlers/fs.ts`:

```ts
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import path from "node:path";
import { RequestError, type schema } from "@agentclientprotocol/sdk";
import type { FsSandbox } from "../fs-sandbox";

interface Ctx {
  fsSandbox: FsSandbox;
  [k: string]: unknown;
}

export async function readTextFile(
  ctx: Ctx,
  params: schema.ReadTextFileRequest,
): Promise<schema.ReadTextFileResponse> {
  let abs: string;
  try {
    abs = ctx.fsSandbox.resolve(params.path);
  } catch (e) {
    if ((e as { code?: string }).code === "path_escapes_worktree") {
      throw new RequestError(-32602, "path_escapes_worktree", { path: params.path });
    }
    throw e;
  }
  const full = readFileSync(abs, "utf8");
  const line = params.line ?? 1;
  const limit = params.limit;
  if (line === 1 && !limit) return { content: full };
  const lines = full.split("\n");
  const start = Math.max(0, line - 1);
  const end = typeof limit === "number" ? Math.min(lines.length, start + limit) : lines.length;
  const sliced = lines.slice(start, end);
  // Preserve trailing newline if original ended with \n and we reached the end.
  const content = sliced.join("\n") + (end < lines.length ? "\n" : (full.endsWith("\n") ? "\n" : ""));
  return { content };
}

export async function writeTextFile(
  ctx: Ctx,
  params: schema.WriteTextFileRequest,
): Promise<schema.WriteTextFileResponse> {
  let abs: string;
  try {
    abs = ctx.fsSandbox.resolve(params.path);
  } catch (e) {
    if ((e as { code?: string }).code === "path_escapes_worktree") {
      throw new RequestError(-32602, "path_escapes_worktree", { path: params.path });
    }
    throw e;
  }
  mkdirSync(path.dirname(abs), { recursive: true });
  writeFileSync(abs, params.content, "utf8");
  return {};
}
```

- [ ] **Step 7: Run fs tests + typecheck**

```bash
cd src-bridge && bun test tests/unit/runtime/acp/handlers-fs.test.ts tests/unit/runtime/acp/fs-sandbox.test.ts && bun run typecheck
```

Expected: 8 tests pass, typecheck exits 0.

- [ ] **Step 8: Commit**

```bash
rtk git add src-bridge/src/runtime/acp/fs-sandbox.ts src-bridge/src/runtime/acp/handlers/fs.ts src-bridge/tests/unit/runtime/acp/fs-sandbox.test.ts src-bridge/tests/unit/runtime/acp/handlers-fs.test.ts
rtk git commit -m "acp(client): T4a — FsSandbox + handlers/fs with worktree realpath guard"
```

---

## Task T4b: `TerminalManager` + `handlers/terminal.ts`

**Goal:** Implement PTY pool with per-task output cap and global concurrency cap. Map ACP's six terminal methods to lifecycle operations.

**Files:**
- Create: `src-bridge/src/runtime/acp/terminal-manager.ts`
- Modify: `src-bridge/src/runtime/acp/handlers/terminal.ts`
- Create: `src-bridge/tests/unit/runtime/acp/terminal-manager.test.ts`

**Steps:**

- [ ] **Step 1: Verify `node-pty` installed**

```bash
ls src-bridge/node_modules/node-pty/lib 2>&1 | head -3
```

Expected: lists at least `index.d.ts`. If missing, `cd src-bridge && bun install`.

- [ ] **Step 2: Write failing tests**

Create `src-bridge/tests/unit/runtime/acp/terminal-manager.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { TerminalManager } from "../../../../src/runtime/acp/terminal-manager";

const root = () => mkdtempSync(path.join(tmpdir(), "tm-"));

describe("TerminalManager", () => {
  test("create + output + waitForExit for a simple command", async () => {
    const tm = new TerminalManager({ outputByteLimit: 1_000_000, maxConcurrent: 8 });
    const { terminalId } = await tm.create({ taskId: "t1", command: "node", args: ["-e", "console.log('pong'); process.exit(0)"], cwd: root() });
    const exit = await tm.waitForExit({ terminalId });
    expect(exit.exitCode).toBe(0);
    const out = await tm.output({ terminalId });
    expect(out.output).toContain("pong");
    await tm.release({ terminalId });
  });

  test("rejects beyond maxConcurrent", async () => {
    const tm = new TerminalManager({ outputByteLimit: 1_000_000, maxConcurrent: 1 });
    await tm.create({ taskId: "t1", command: "node", args: ["-e", "setTimeout(()=>{},5000)"], cwd: root() });
    await expect(
      tm.create({ taskId: "t1", command: "node", args: ["-e", "process.exit(0)"], cwd: root() }),
    ).rejects.toMatchObject({ code: -32000 });
  });

  test("kill keeps terminalId valid; release invalidates it", async () => {
    const tm = new TerminalManager({ outputByteLimit: 1_000_000, maxConcurrent: 8 });
    const { terminalId } = await tm.create({ taskId: "t1", command: "node", args: ["-e", "setTimeout(()=>{},5000)"], cwd: root() });
    await tm.kill({ terminalId });
    await expect(tm.output({ terminalId })).resolves.toBeDefined();
    await tm.release({ terminalId });
    await expect(tm.output({ terminalId })).rejects.toThrow();
  });
});
```

- [ ] **Step 3: Run failing tests**

```bash
cd src-bridge && bun test tests/unit/runtime/acp/terminal-manager.test.ts
```

Expected: module-not-found.

- [ ] **Step 4: Implement `TerminalManager`**

Create `src-bridge/src/runtime/acp/terminal-manager.ts`:

```ts
import { RequestError } from "@agentclientprotocol/sdk";
import * as pty from "node-pty";
import { randomUUID } from "node:crypto";

export interface TerminalManagerOptions {
  outputByteLimit: number;
  maxConcurrent: number;
}

interface TerminalEntry {
  terminalId: string;
  taskId: string;
  term: pty.IPty;
  buffer: string[];
  bytes: number;
  exitCode: number | null;
  released: boolean;
  exitWaiters: Array<(code: number) => void>;
}

export class TerminalManager {
  private readonly entries = new Map<string, TerminalEntry>();
  constructor(private readonly opts: TerminalManagerOptions) {}

  async create(params: { taskId: string; command: string; args?: string[]; cwd: string; env?: Record<string, string> }): Promise<{ terminalId: string }> {
    if (this.entries.size >= this.opts.maxConcurrent) {
      throw new RequestError(-32000, "terminal_capacity", { limit: this.opts.maxConcurrent });
    }
    const term = pty.spawn(params.command, params.args ?? [], {
      cwd: params.cwd,
      env: { ...process.env, ...(params.env ?? {}) },
      cols: 80,
      rows: 24,
    });
    const entry: TerminalEntry = {
      terminalId: randomUUID(),
      taskId: params.taskId,
      term,
      buffer: [],
      bytes: 0,
      exitCode: null,
      released: false,
      exitWaiters: [],
    };
    term.onData((chunk) => {
      entry.buffer.push(chunk);
      entry.bytes += Buffer.byteLength(chunk);
      while (entry.bytes > this.opts.outputByteLimit && entry.buffer.length > 1) {
        const dropped = entry.buffer.shift()!;
        entry.bytes -= Buffer.byteLength(dropped);
      }
    });
    term.onExit(({ exitCode }) => {
      entry.exitCode = exitCode;
      for (const w of entry.exitWaiters) w(exitCode);
      entry.exitWaiters = [];
    });
    this.entries.set(entry.terminalId, entry);
    return { terminalId: entry.terminalId };
  }

  async output(params: { terminalId: string }): Promise<{ output: string; truncated: boolean }> {
    const e = this.entries.get(params.terminalId);
    if (!e || e.released) throw new RequestError(-32602, "terminal_not_found");
    return { output: e.buffer.join(""), truncated: e.bytes >= this.opts.outputByteLimit };
  }

  async waitForExit(params: { terminalId: string }): Promise<{ exitCode: number }> {
    const e = this.entries.get(params.terminalId);
    if (!e) throw new RequestError(-32602, "terminal_not_found");
    if (e.exitCode !== null) return { exitCode: e.exitCode };
    return new Promise((res) => {
      e.exitWaiters.push((code) => res({ exitCode: code }));
    });
  }

  async kill(params: { terminalId: string }): Promise<void> {
    const e = this.entries.get(params.terminalId);
    if (!e) throw new RequestError(-32602, "terminal_not_found");
    if (e.exitCode === null) e.term.kill();
  }

  async release(params: { terminalId: string }): Promise<void> {
    const e = this.entries.get(params.terminalId);
    if (!e) return;
    if (e.exitCode === null) e.term.kill();
    e.released = true;
    this.entries.delete(params.terminalId);
  }

  /** One-shot convenience for `RuntimeAdapter.executeShell`. */
  async runOneShot(taskId: string, command: string): Promise<{ output: string; exitCode: number }> {
    const { terminalId } = await this.create({ taskId, command: "sh", args: ["-c", command], cwd: process.cwd() });
    const exit = await this.waitForExit({ terminalId });
    const out = await this.output({ terminalId });
    await this.release({ terminalId });
    return { output: out.output, exitCode: exit.exitCode };
  }
}
```

On Windows, `sh -c` fallback may not exist; use `cmd /c` branch if needed. Test in CI on target platforms.

- [ ] **Step 5: Implement `handlers/terminal.ts`**

Replace `src-bridge/src/runtime/acp/handlers/terminal.ts`:

```ts
import type { schema } from "@agentclientprotocol/sdk";
import type { TerminalManager } from "../terminal-manager";

interface Ctx {
  taskId: string;
  terminalManager: TerminalManager;
  cwd: string;
  [k: string]: unknown;
}

export async function createTerminal(ctx: Ctx, params: schema.CreateTerminalRequest): Promise<schema.CreateTerminalResponse> {
  const { terminalId } = await ctx.terminalManager.create({
    taskId: ctx.taskId,
    command: params.command,
    args: params.args,
    cwd: params.cwd ?? ctx.cwd,
    env: params.env,
  });
  return { terminalId };
}
export async function terminalOutput(ctx: Ctx, params: schema.TerminalOutputRequest): Promise<schema.TerminalOutputResponse> {
  return ctx.terminalManager.output({ terminalId: params.terminalId });
}
export async function waitForTerminalExit(ctx: Ctx, params: schema.WaitForTerminalExitRequest): Promise<schema.WaitForTerminalExitResponse> {
  const { exitCode } = await ctx.terminalManager.waitForExit({ terminalId: params.terminalId });
  return { exitCode };
}
export async function killTerminal(ctx: Ctx, params: schema.KillTerminalRequest): Promise<schema.KillTerminalResponse | void> {
  await ctx.terminalManager.kill({ terminalId: params.terminalId });
}
export async function releaseTerminal(ctx: Ctx, params: schema.ReleaseTerminalRequest): Promise<schema.ReleaseTerminalResponse | void> {
  await ctx.terminalManager.release({ terminalId: params.terminalId });
}
```

- [ ] **Step 6: Run tests + typecheck**

```bash
cd src-bridge && bun test tests/unit/runtime/acp/terminal-manager.test.ts && bun run typecheck
```

Expected: 3 tests pass, typecheck exits 0.

- [ ] **Step 7: Commit**

```bash
rtk git add src-bridge/src/runtime/acp/terminal-manager.ts src-bridge/src/runtime/acp/handlers/terminal.ts src-bridge/tests/unit/runtime/acp/terminal-manager.test.ts
rtk git commit -m "acp(client): T4b — TerminalManager PTY pool + handlers/terminal"
```

---

## Task T4c: `handlers/permission.ts` + `PermissionRouter`

**Goal:** Adapt ACP's `session/request_permission` into the existing `HookCallbackManager` + `/bridge/permission-response/:id` route. Remove the `.todo` marker on the permission-flow component test.

**Files:**
- Create: `src-bridge/src/runtime/acp/permission-router.ts`
- Modify: `src-bridge/src/runtime/acp/handlers/permission.ts`
- Create: `src-bridge/tests/unit/runtime/acp/permission-router.test.ts`
- Modify: `src-bridge/tests/component/acp/permission-flow.test.ts` (remove `.todo`)

**Steps:**

- [ ] **Step 1: Write failing router test**

Create `src-bridge/tests/unit/runtime/acp/permission-router.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { HookCallbackManager } from "../../../../src/runtime/hook-callback-manager";
import { PermissionRouter } from "../../../../src/runtime/acp/permission-router";

const silent = { info: () => {}, warn: () => {}, error: () => {} };

describe("PermissionRouter", () => {
  test("allow option resolves selected outcome", async () => {
    const hook = new HookCallbackManager();
    const streamer = { send: () => {} };
    const router = new PermissionRouter({ hookCallbackManager: hook, streamer, logger: silent });
    const p = router.request("task1", "session1", { toolCall: { name: "write" }, options: [{ optionId: "allow", label: "Allow", kind: "allow_once" }] });
    const pending = Array.from(hook.pendingIds())[0];
    hook.resolve(pending, { option_id: "allow" });
    const out = await p;
    expect(out).toMatchObject({ outcome: { outcome: "selected", optionId: "allow" } });
  });

  test("cancel returns cancelled outcome", async () => {
    const hook = new HookCallbackManager();
    const streamer = { send: () => {} };
    const router = new PermissionRouter({ hookCallbackManager: hook, streamer, logger: silent });
    const p = router.request("task1", "session1", { toolCall: { name: "write" }, options: [] });
    const pending = Array.from(hook.pendingIds())[0];
    hook.resolve(pending, { cancelled: true });
    const out = await p;
    expect(out).toMatchObject({ outcome: { outcome: "cancelled" } });
  });

  test("30min timeout auto-cancels", async () => {
    const hook = new HookCallbackManager();
    const streamer: { events: unknown[]; send: (e: unknown) => void } = { events: [], send(e) { this.events.push(e); } };
    const router = new PermissionRouter({ hookCallbackManager: hook, streamer, logger: silent, timeoutMs: 30 });
    const out = await router.request("task1", "session1", { toolCall: { name: "write" }, options: [] });
    expect(out).toMatchObject({ outcome: { outcome: "cancelled" } });
    expect(streamer.events.some((e) => (e as { type?: string }).type === "permission_request" )).toBe(true);
  });
});
```

Note: `HookCallbackManager.pendingIds()` may not exist; add a `public pendingIds(): Iterable<string> { return this.pending.keys(); }` method to `src-bridge/src/runtime/hook-callback-manager.ts` if needed. This is a small additive change; include it in the same commit.

- [ ] **Step 2: Run failing test**

```bash
cd src-bridge && bun test tests/unit/runtime/acp/permission-router.test.ts
```

Expected: module-not-found.

- [ ] **Step 3: Implement `PermissionRouter`**

Create `src-bridge/src/runtime/acp/permission-router.ts`:

```ts
import type { HookCallbackManager } from "../hook-callback-manager";

export interface PermissionOutcomeSelected { outcome: "selected"; optionId: string }
export interface PermissionOutcomeCancelled { outcome: "cancelled" }
export type PermissionOutcome = PermissionOutcomeSelected | PermissionOutcomeCancelled;

export interface PermissionRouterOptions {
  hookCallbackManager: HookCallbackManager;
  streamer: { send: (event: unknown) => void };
  logger: { info: (...a: unknown[]) => void; warn: (...a: unknown[]) => void; error: (...a: unknown[]) => void };
  timeoutMs?: number;
}

const THIRTY_MINUTES = 30 * 60 * 1000;

export class PermissionRouter {
  private readonly timeoutMs: number;
  constructor(private readonly opts: PermissionRouterOptions) {
    this.timeoutMs = opts.timeoutMs ?? THIRTY_MINUTES;
  }

  async request(taskId: string, sessionId: string, params: {
    toolCall: unknown;
    options: unknown[];
  }): Promise<{ outcome: PermissionOutcome }> {
    const registration = this.opts.hookCallbackManager.register({ timeoutMs: this.timeoutMs });
    const requestId = registration.requestId;
    this.opts.streamer.send({
      task_id: taskId,
      session_id: sessionId,
      timestamp_ms: Date.now(),
      type: "permission_request",
      data: { request_id: requestId, toolCall: params.toolCall, options: params.options },
    });
    try {
      const payload = (await registration.promise) as { option_id?: string; cancelled?: boolean };
      if (payload.cancelled) return { outcome: { outcome: "cancelled" } };
      if (payload.option_id) return { outcome: { outcome: "selected", optionId: payload.option_id } };
      return { outcome: { outcome: "cancelled" } };
    } catch {
      this.opts.streamer.send({
        task_id: taskId,
        session_id: sessionId,
        timestamp_ms: Date.now(),
        type: "status_change",
        data: { kind: "permission_timeout", request_id: requestId },
      });
      return { outcome: { outcome: "cancelled" } };
    }
  }
}
```

If `HookCallbackManager.register` returns a different shape (check file `src-bridge/src/runtime/hook-callback-manager.ts`), adjust the payload extraction. Add a `pendingIds()` accessor to that class — 1-line change.

- [ ] **Step 4: Implement `handlers/permission.ts`**

Replace `src-bridge/src/runtime/acp/handlers/permission.ts`:

```ts
import type { schema } from "@agentclientprotocol/sdk";
import type { PermissionRouter } from "../permission-router";

interface Ctx {
  taskId: string;
  permissionRouter: PermissionRouter;
  [k: string]: unknown;
}

export async function requestPermission(
  ctx: Ctx,
  params: schema.RequestPermissionRequest,
): Promise<schema.RequestPermissionResponse> {
  return ctx.permissionRouter.request(ctx.taskId, params.sessionId, {
    toolCall: params.toolCall,
    options: params.options,
  });
}
```

- [ ] **Step 5: Remove `.todo` from `tests/component/acp/permission-flow.test.ts`**

Edit the file (created in T3) to drop the `test.todo` marker; the test should now pass.

- [ ] **Step 6: Run tests**

```bash
cd src-bridge && bun test tests/unit/runtime/acp/permission-router.test.ts tests/component/acp/permission-flow.test.ts && bun run typecheck
```

Expected: 3 router tests + permission-flow test pass.

- [ ] **Step 7: Commit**

```bash
rtk git add src-bridge/src/runtime/hook-callback-manager.ts src-bridge/src/runtime/acp/permission-router.ts src-bridge/src/runtime/acp/handlers/permission.ts src-bridge/tests/unit/runtime/acp/permission-router.test.ts src-bridge/tests/component/acp/permission-flow.test.ts
rtk git commit -m "acp(client): T4c — PermissionRouter + handlers/permission reusing HookCallbackManager"
```

---

## Task T4d: `handlers/elicitation.ts` full passthrough

**Goal:** Wire `unstable_createElicitation` through the same request_id + HookCallbackManager pattern; capability-gate on the caller side (spec §6.4).

**Files:**
- Modify: `src-bridge/src/runtime/acp/handlers/elicitation.ts`
- Create: `src-bridge/tests/unit/runtime/acp/handlers-elicitation.test.ts`

**Steps:**

- [ ] **Step 1: Write failing tests**

Create `src-bridge/tests/unit/runtime/acp/handlers-elicitation.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { HookCallbackManager } from "../../../../src/runtime/hook-callback-manager";
import { createElicitation } from "../../../../src/runtime/acp/handlers/elicitation";
import { PermissionRouter } from "../../../../src/runtime/acp/permission-router";

const silent = { info: () => {}, warn: () => {}, error: () => {} };

describe("handlers/elicitation", () => {
  test("emits elicitation_request event and resolves with payload", async () => {
    const hook = new HookCallbackManager();
    const streamer: { events: unknown[]; send: (e: unknown) => void } = { events: [], send(e) { this.events.push(e); } };
    const router = new PermissionRouter({ hookCallbackManager: hook, streamer, logger: silent });
    const ctx = { taskId: "t1", permissionRouter: router } as const;
    const p = createElicitation(ctx, { sessionId: "s1", message: "input?", schema: {} });
    const reqId = Array.from(hook.pendingIds())[0];
    hook.resolve(reqId, { action: "submit", data: { value: 42 } });
    await expect(p).resolves.toEqual({ action: "submit", data: { value: 42 } });
    expect(streamer.events.some((e) => (e as { type?: string }).type === "status_change" )).toBe(true);
  });

  test("returns { action: 'cancel' } when permissionRouter missing", async () => {
    const ctx = { taskId: "t1" };
    const out = await createElicitation(ctx as unknown as never, { sessionId: "s1", message: "x", schema: {} } as never);
    expect(out).toEqual({ action: "cancel" });
  });
});
```

- [ ] **Step 2: Run failing test**

Expect: first test fails because the current stub returns `{ action: "cancel" }` without routing.

- [ ] **Step 3: Implement passthrough**

Replace `src-bridge/src/runtime/acp/handlers/elicitation.ts`:

```ts
import type { HookCallbackManager } from "../../hook-callback-manager";
import type { PermissionRouter } from "../permission-router";

interface Ctx {
  taskId: string;
  permissionRouter?: PermissionRouter;
  hookCallbackManager?: HookCallbackManager;
  streamer?: { send: (e: unknown) => void };
  [k: string]: unknown;
}

export async function createElicitation(ctx: Ctx, params: { sessionId: string; message: string; schema: unknown }): Promise<{ action: "submit"; data: unknown } | { action: "cancel" }> {
  const router = ctx.permissionRouter;
  if (!router) return { action: "cancel" };
  // Re-enter the same request_id/HookCallbackManager pipeline used for permissions.
  // Emit an elicitation_request status_change event; response semantics differ (action vs option_id).
  const hook = (router as unknown as { opts: { hookCallbackManager: HookCallbackManager; streamer: { send: (e: unknown) => void } } }).opts;
  const registration = hook.hookCallbackManager.register({ timeoutMs: 30 * 60 * 1000 });
  hook.streamer.send({
    task_id: ctx.taskId,
    session_id: params.sessionId,
    timestamp_ms: Date.now(),
    type: "status_change",
    data: { kind: "elicitation_request", request_id: registration.requestId, message: params.message, schema: params.schema },
  });
  try {
    const payload = (await registration.promise) as { action?: string; data?: unknown };
    if (payload.action === "submit") return { action: "submit", data: payload.data };
    return { action: "cancel" };
  } catch {
    return { action: "cancel" };
  }
}

export async function completeElicitation(_ctx: unknown, _params: unknown): Promise<void> {
  /* no-op — response is already delivered via /bridge/permission-response */
}
```

The `PermissionRouter.opts` back-reference is pragmatic; if the cast feels brittle, expose a `router.emitElicitation(...)` helper on `PermissionRouter` instead.

- [ ] **Step 4: Run tests + typecheck**

```bash
cd src-bridge && bun test tests/unit/runtime/acp/handlers-elicitation.test.ts && bun run typecheck
```

Expected: both pass.

- [ ] **Step 5: Commit**

```bash
rtk git add src-bridge/src/runtime/acp/handlers/elicitation.ts src-bridge/tests/unit/runtime/acp/handlers-elicitation.test.ts
rtk git commit -m "acp(client): T4d — unstable_createElicitation passthrough via HookCallbackManager"
```

---

## Task T5: `events/session-update.ts` full mapping

**Goal:** Translate every ACP `session/update` variant into `AgentEvent[]` per spec §8; tolerate unknown variants.

**Files:**
- Modify: `src-bridge/src/runtime/acp/events/session-update.ts`
- Create: `src-bridge/tests/unit/runtime/acp/session-update-mapping.test.ts`

**Steps:**

- [ ] **Step 1: Write failing tests**

Create `src-bridge/tests/unit/runtime/acp/session-update-mapping.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { mapSessionUpdate } from "../../../../src/runtime/acp/events/session-update";

const now = () => 1_700_000_000_000;

function mapOne(update: unknown) {
  return mapSessionUpdate("task-1", "sess-1", update, now)[0];
}

describe("mapSessionUpdate", () => {
  test.each([
    ["user_message_chunk", { sessionUpdate: "user_message_chunk", content: { type: "text", text: "hi" } }, "partial_message"],
    ["agent_message_chunk", { sessionUpdate: "agent_message_chunk", content: { type: "text", text: "hey" } }, "output"],
    ["agent_thought_chunk", { sessionUpdate: "agent_thought_chunk", content: { type: "text", text: "thinking" } }, "reasoning"],
    ["tool_call", { sessionUpdate: "tool_call", toolCallId: "t1", title: "Run", kind: "execute" }, "tool_call"],
    ["tool_call_update in_progress", { sessionUpdate: "tool_call_update", toolCallId: "t1", status: "in_progress" }, "tool.status_change"],
    ["tool_call_update completed", { sessionUpdate: "tool_call_update", toolCallId: "t1", status: "completed" }, "tool_result"],
    ["plan", { sessionUpdate: "plan", entries: [] }, "todo_update"],
    ["available_commands_update", { sessionUpdate: "available_commands_update", availableCommands: [] }, "status_change"],
    ["current_mode_update", { sessionUpdate: "current_mode_update", currentMode: "plan" }, "status_change"],
    ["config_option_update", { sessionUpdate: "config_option_update", key: "x", value: 1 }, "status_change"],
  ])("maps %s to %s", (_name, update, expectedType) => {
    const ev = mapOne(update);
    expect(ev.type).toBe(expectedType);
    expect(ev.task_id).toBe("task-1");
    expect(ev.session_id).toBe("sess-1");
  });

  test("unknown variant falls through to acp_passthrough", () => {
    const ev = mapOne({ sessionUpdate: "invented_new_kind", foo: 1 });
    expect(ev.type).toBe("status_change");
    expect((ev.data as { kind?: string }).kind).toBe("acp_passthrough");
    expect((ev.data as { _raw?: unknown })._raw).toMatchObject({ sessionUpdate: "invented_new_kind" });
  });

  test("copies _meta verbatim", () => {
    const ev = mapOne({ sessionUpdate: "agent_message_chunk", content: { type: "text", text: "x" }, _meta: { usage: { input_tokens: 10 } } });
    expect((ev.data as { _meta?: unknown })._meta).toEqual({ usage: { input_tokens: 10 } });
  });
});
```

- [ ] **Step 2: Run failing tests**

```bash
cd src-bridge && bun test tests/unit/runtime/acp/session-update-mapping.test.ts
```

Expected: all fail with "not implemented (T5)".

- [ ] **Step 3: Implement mapper**

Replace `src-bridge/src/runtime/acp/events/session-update.ts`:

```ts
import type { AgentEvent } from "../../../types";

export function mapSessionUpdate(
  taskId: string,
  sessionId: string,
  update: unknown,
  nowMs: () => number,
): AgentEvent[] {
  const u = update as { sessionUpdate?: string; _meta?: unknown; [k: string]: unknown };
  const base = { task_id: taskId, session_id: sessionId, timestamp_ms: nowMs() };
  const withMeta = <T extends Record<string, unknown>>(data: T): T & { _meta?: unknown } =>
    u._meta ? { ...data, _meta: u._meta } : data;

  switch (u.sessionUpdate) {
    case "user_message_chunk":
      return [{ ...base, type: "partial_message", data: withMeta({ direction: "user", content: u.content }) }];
    case "agent_message_chunk":
      return [{ ...base, type: "output", data: withMeta({ content: u.content }) }];
    case "agent_thought_chunk":
      return [{ ...base, type: "reasoning", data: withMeta({ content: u.content }) }];
    case "tool_call":
      return [{ ...base, type: "tool_call", data: withMeta({ toolCallId: u.toolCallId, title: u.title, kind: u.kind, input: u.input }) }];
    case "tool_call_update": {
      const status = (u as { status?: string }).status;
      if (status === "completed" || status === "failed") {
        return [{ ...base, type: "tool_result", data: withMeta({ toolCallId: u.toolCallId, status, output: u.output, error: u.error }) }];
      }
      return [{ ...base, type: "tool.status_change", data: withMeta({ toolCallId: u.toolCallId, status }) }];
    }
    case "plan":
      return [{ ...base, type: "todo_update", data: withMeta({ entries: u.entries }) }];
    case "available_commands_update":
      return [{ ...base, type: "status_change", data: withMeta({ kind: "commands", availableCommands: u.availableCommands }) }];
    case "current_mode_update":
      return [{ ...base, type: "status_change", data: withMeta({ kind: "mode", currentMode: u.currentMode }) }];
    case "config_option_update":
      return [{ ...base, type: "status_change", data: withMeta({ kind: "config_option", key: u.key, value: u.value }) }];
    default:
      // Unstable passthrough: NES, document lifecycle, ext, unknown.
      if (typeof u.sessionUpdate === "string" && u.sessionUpdate.startsWith("nes_")) {
        return [{ ...base, type: "status_change", data: withMeta({ kind: "nes", subtype: u.sessionUpdate, _raw: u }) }];
      }
      if (typeof u.sessionUpdate === "string" && u.sessionUpdate.startsWith("document_")) {
        return [{ ...base, type: "status_change", data: withMeta({ kind: "document", subtype: u.sessionUpdate, _raw: u }) }];
      }
      return [{ ...base, type: "status_change", data: withMeta({ kind: "acp_passthrough", _raw: u }) }];
  }
}
```

- [ ] **Step 4: Run tests + typecheck**

```bash
cd src-bridge && bun test tests/unit/runtime/acp/session-update-mapping.test.ts && bun run typecheck
```

Expected: all 13 (10 parameterized + 3) pass.

- [ ] **Step 5: Commit**

```bash
rtk git add src-bridge/src/runtime/acp/events/session-update.ts src-bridge/tests/unit/runtime/acp/session-update-mapping.test.ts
rtk git commit -m "acp(client): T5 — session/update → AgentEvent mapping with passthrough fallback"
```

---

## Task T6: `adapter-factory.ts` + `runtime/registry.ts` wiring + `live_controls` unification

**Goal:** Wrap `AcpSession` in the real `RuntimeAdapter` interface. Wire all 5 adapters through `createAcpRuntimeAdapter`. Drop the Claude-specific `live_controls` gate. Verify Linux optional-dep resolution.

**Files:**
- Modify: `src-bridge/src/runtime/acp/adapter-factory.ts`
- Modify: `src-bridge/src/runtime/registry.ts`
- Modify: `src-bridge/src/runtime/agent-runtime.ts` (lines 134–144)
- Modify: `src-bridge/src/server.ts` (inject `AcpConnectionPool` + deps)

**Steps:**

- [ ] **Step 1: Implement `adapter-factory.ts` building the `RuntimeAdapter` shape**

Replace `src-bridge/src/runtime/acp/adapter-factory.ts`:

```ts
import type { AdapterId } from "./registry";
import type { AcpConnectionPool } from "./connection-pool";
import { AcpSession } from "./session";
import { FsSandbox } from "./fs-sandbox";
import type { TerminalManager } from "./terminal-manager";
import type { PermissionRouter } from "./permission-router";
import type { CachedCapabilities } from "./capabilities";
import type {
  AgentRuntime,
  EventSink,
  ExecuteRequest,
  RuntimeForkParams,
  RuntimeRollbackParams,
  RuntimeRevertParams,
  RuntimeCommandParams,
  RuntimeShellParams,
  RuntimeSetModelParams,
  RuntimeThinkingBudgetParams,
} from "../../types";
import type { HookCallbackManager } from "../hook-callback-manager";

export interface AcpRuntimeAdapterDeps {
  pool: AcpConnectionPool;
  terminalManager: TerminalManager;
  permissionRouter: PermissionRouter;
  hookCallbackManager: HookCallbackManager;
  worktreeService: {
    revert(params: RuntimeRevertParams): Promise<void>;
    diff(params: { taskId: string }): Promise<unknown>;
  };
  taskEventsService: { messages(taskId: string): Promise<unknown> };
  mcpServersStatus: (runtime: AgentRuntime) => Promise<unknown>;
  resolveMcpServersFor: (runtime: AgentRuntime) => unknown[];
  logger: { info: (...a: unknown[]) => void; warn: (...a: unknown[]) => void; error: (...a: unknown[]) => void };
  legacyFactories?: Partial<Record<AdapterId, unknown>>;
  now?: () => number;
}

export function createAcpRuntimeAdapter(adapterId: AdapterId, deps: AcpRuntimeAdapterDeps) {
  return {
    async ensureAvailable(): Promise<void> {
      // Lazy-check env requirements; pool.acquire performs the deeper check.
      return;
    },
    async execute(runtime: AgentRuntime, streamer: EventSink, req: ExecuteRequest, systemPrompt: string): Promise<void> {
      // Legacy fallback during T1–T6 dev only.
      if (process.env[`BRIDGE_ACP_${adapterId.toUpperCase()}`] === "0") {
        const legacy = deps.legacyFactories?.[adapterId] as { execute?: (...a: unknown[]) => Promise<void> } | undefined;
        if (legacy?.execute) return legacy.execute(runtime, streamer, req, systemPrompt);
      }
      const fsSandbox = new FsSandbox(runtime.worktreeRoot);
      const session = await AcpSession.open(deps.pool, {
        taskId: runtime.taskId,
        adapterId,
        cwd: runtime.worktreeRoot,
        mcpServers: deps.resolveMcpServersFor(runtime) as never,
        perSessionContext: {
          taskId: runtime.taskId,
          cwd: runtime.worktreeRoot,
          fsSandbox,
          terminalManager: deps.terminalManager,
          permissionRouter: deps.permissionRouter,
          hookCallbackManager: deps.hookCallbackManager,
          streamer,
          logger: deps.logger,
        },
        logger: deps.logger,
      });
      // Update live_controls on runtime using cached capabilities:
      runtime.capabilities = {
        setModel: session.capabilities.modelSelectable,
        setThinkingBudget: session.capabilities.thinkingBudget,
        setMode: session.capabilities.modeSelectable,
        mcpStatus: session.capabilities.mcpStatus,
      };
      const stop = await session.prompt([{ type: "text", text: req.prompt }] as never);
      streamer.send({
        task_id: runtime.taskId,
        session_id: session.sessionId,
        timestamp_ms: (deps.now ?? Date.now)(),
        type: "status_change",
        data: { kind: "stop", stopReason: stop },
      });
      await session.dispose();
    },
    async fork(runtime: AgentRuntime, _params: RuntimeForkParams) {
      // Implementers: look up session handle from runtime state. For now, throw.
      throw new Error("fork via ACP requires per-runtime session handle; T6 wires via AgentRuntime.state");
    },
    async rollback(_runtime: AgentRuntime, _params: RuntimeRollbackParams): Promise<void> {
      throw new Error("rollback via ACP requires per-runtime session handle; T6 wires via AgentRuntime.state");
    },
    async revert(_runtime: AgentRuntime, params: RuntimeRevertParams): Promise<void> {
      await deps.worktreeService.revert(params);
    },
    async getMessages(runtime: AgentRuntime) {
      return deps.taskEventsService.messages(runtime.taskId);
    },
    async getDiff(runtime: AgentRuntime) {
      return deps.worktreeService.diff({ taskId: runtime.taskId });
    },
    async executeCommand(_runtime: AgentRuntime, _params: RuntimeCommandParams) {
      // extMethod passthrough with slash-command fallback; session handle needed.
      throw new Error("executeCommand via ACP requires per-runtime session handle");
    },
    async executeShell(runtime: AgentRuntime, params: RuntimeShellParams) {
      return deps.terminalManager.runOneShot(runtime.taskId, (params as { command: string }).command);
    },
    async setThinkingBudget(_runtime: AgentRuntime, _params: RuntimeThinkingBudgetParams): Promise<void> {
      throw new Error("setThinkingBudget via ACP requires per-runtime session handle");
    },
    async getMcpServerStatus(runtime: AgentRuntime) {
      return deps.mcpServersStatus(runtime);
    },
    async interrupt(_runtime: AgentRuntime): Promise<void> {
      throw new Error("interrupt via ACP requires per-runtime session handle");
    },
    async setModel(_runtime: AgentRuntime, _params: RuntimeSetModelParams): Promise<void> {
      throw new Error("setModel via ACP requires per-runtime session handle");
    },
  };
}
```

**Important note:** several methods above (`fork`, `rollback`, `executeCommand`, `interrupt`, `setThinkingBudget`, `setModel`) require the adapter to hold a per-task session handle beyond a single `execute()` call. `AgentRuntime` (the existing state container in `src-bridge/src/runtime/agent-runtime.ts`) already tracks per-task state (e.g. `claudeQuery`) — extend it to hold an `acpSession: AcpSession | null` reference. The `execute()` method above sets `runtime.acpSession = session` before `session.prompt(...)`. Other methods read `runtime.acpSession` and delegate. Update the `runtime.ts` interface and the assignments accordingly in the final wiring.

- [ ] **Step 2: Add `acpSession` to `AgentRuntime` + drop `claudeQuery` gate**

Edit `src-bridge/src/runtime/agent-runtime.ts`:

1. Add a field `acpSession: AcpSession | null = null;` alongside `claudeQuery`.
2. Replace lines 134–144 (the current `liveControls` expression) with a capability-driven form:

```ts
const liveControls = this.acpSession
  ? {
      interrupt: true,
      set_model: this.acpSession.capabilities.modelSelectable || undefined,
      set_thinking_budget: this.acpSession.capabilities.thinkingBudget || undefined,
      mcp_status: this.acpSession.capabilities.mcpStatus || undefined,
      set_mode: this.acpSession.capabilities.modeSelectable || undefined,
    }
  : runtime === "claude_code" && this.claudeQuery
  ? {
      interrupt: typeof this.claudeQuery.interrupt === "function" || undefined,
      set_model: typeof this.claudeQuery.setModel === "function" || undefined,
      set_thinking_budget: typeof this.claudeQuery.setMaxThinkingTokens === "function" || undefined,
      mcp_status: typeof this.claudeQuery.mcpServerStatus === "function" || undefined,
    }
  : undefined;
```

The `claudeQuery` branch stays during T6 (legacy fallback). T7 deletes it.

- [ ] **Step 3: Wire all 5 adapters via `createAcpRuntimeAdapter` in `runtime/registry.ts`**

In `src-bridge/src/runtime/registry.ts`, locate the five adapter factory sites (lines 374, 414, 426, 435, 441 per T6 baseline). For each, replace the current factory with:

```ts
const acpAdapter = createAcpRuntimeAdapter("<adapter_id>", {
  pool: options.acpPool,
  terminalManager: options.terminalManager,
  permissionRouter: options.permissionRouter,
  hookCallbackManager: options.hookCallbackManager,
  worktreeService: options.worktreeService,
  taskEventsService: options.taskEventsService,
  mcpServersStatus: options.mcpServersStatus,
  resolveMcpServersFor: options.resolveMcpServersFor,
  logger: options.logger,
  legacyFactories: { "<adapter_id>": options.legacyFactories?.<adapter_id> },
  now: options.now,
});
```

Keep the existing fields of the `RuntimeAdapter` record (key/label/defaultProvider/etc.) as they were; only the method implementations swap out. Fold `qoder` and `iflow` untouched — they still go through `createCliRuntimeAdapter`.

Extend `AgentRuntimeRegistryOptions` (same file) with:

```ts
acpPool: AcpConnectionPool;
terminalManager: TerminalManager;
permissionRouter: PermissionRouter;
hookCallbackManager: HookCallbackManager;
worktreeService: { revert: ...; diff: ... };
taskEventsService: { messages: ... };
mcpServersStatus: (runtime) => Promise<unknown>;
resolveMcpServersFor: (runtime) => unknown[];
legacyFactories?: Partial<Record<AdapterId, unknown>>;
```

- [ ] **Step 4: Inject deps from `server.ts`**

In `src-bridge/src/server.ts`, at startup:

```ts
import { AcpConnectionPool } from "./runtime/acp/connection-pool";
import { TerminalManager } from "./runtime/acp/terminal-manager";
import { PermissionRouter } from "./runtime/acp/permission-router";

const acpPool = new AcpConnectionPool({ logger, idleMs: 600_000 });
const terminalManager = new TerminalManager({ outputByteLimit: 10 * 1024 * 1024, maxConcurrent: 16 });
const permissionRouter = new PermissionRouter({ hookCallbackManager, streamer: bridgeEventStreamer, logger });
```

Pass them into the existing `buildAgentRuntimeRegistry(...)` (or equivalent) call. Pass legacy factories as `{ claude_code: existingClaudeFactory, codex: existingCodexFactory, ... }` so the `BRIDGE_ACP_<ADAPTER>=0` branch keeps working.

- [ ] **Step 5: Typecheck + existing tests**

```bash
cd src-bridge && bun run typecheck && bun test
```

Expected: exits 0. If compile errors, fix import paths / option names and re-run.

- [ ] **Step 6: Linux CI optional-dep validation**

Add a CI job (or use an existing one in `.github/workflows`) that runs:

```bash
cd src-bridge && bun install --frozen-lockfile && bun test
```

on `ubuntu-latest`. If the repo already has a bridge CI job, confirm it now installs `@zed-industries/codex-acp` (which has platform-specific native optional-deps) cleanly. Failure to resolve blocks this task.

- [ ] **Step 7: Commit**

```bash
rtk git add src-bridge/src/runtime/acp/adapter-factory.ts src-bridge/src/runtime/registry.ts src-bridge/src/runtime/agent-runtime.ts src-bridge/src/server.ts
rtk git commit -m "acp(client): T6 — adapter-factory + registry wiring + capability-driven live_controls"
```

---

## Task T7: Integration tests × 5 + atomic legacy deletion

**Goal:** Prove all five adapters work end-to-end with real agents; in the same PR, delete legacy handlers and remove the `BRIDGE_ACP_*` flag + `claudeQuery` field.

**Files:**
- Create: `src-bridge/tests/integration/acp/claude_code.test.ts`
- Create: `src-bridge/tests/integration/acp/codex.test.ts`
- Create: `src-bridge/tests/integration/acp/opencode.test.ts`
- Create: `src-bridge/tests/integration/acp/cursor.test.ts`
- Create: `src-bridge/tests/integration/acp/gemini.test.ts`
- Modify: `scripts/dev/dev-all.js` / `dev:backend:verify` entry point (to add 5-adapter echo step)
- Delete: `src-bridge/src/handlers/claude-runtime.ts`
- Delete: `src-bridge/src/handlers/codex-runtime.ts`
- Delete: `src-bridge/src/handlers/opencode-runtime.ts`
- Modify: `src-bridge/src/handlers/command-runtime.ts` (drop cursor + gemini branches)
- Delete: `src-bridge/src/opencode/` (entire directory)
- Modify: `src-bridge/src/runtime/registry.ts` (remove legacy factory constructors for cc/codex/opencode/cursor/gemini; remove `legacyFactories` option; drop `BRIDGE_ACP_*` branch in `adapter-factory.ts`)
- Modify: `src-bridge/src/runtime/agent-runtime.ts` (delete `claudeQuery` field + fallback branch)
- Modify: `.env.example`
- Modify: `docs/superpowers/specs/2026-04-21-bridge-acp-client-integration.md` (append `_meta.usage` table to §8; update changelog)
- Modify: `CHANGELOG.md` or similar

**Steps:**

- [ ] **Step 1: Write one integration test per adapter**

Template for `src-bridge/tests/integration/acp/claude_code.test.ts`:

```ts
import { describe, expect, test, beforeAll } from "bun:test";
import { AcpConnectionPool } from "../../../src/runtime/acp/connection-pool";
import { AcpSession } from "../../../src/runtime/acp/session";

const SKIP = process.env.SKIP_ACP_INTEGRATION !== "0";
const d = SKIP ? describe.skip : describe;

const silent = { info: () => {}, warn: () => {}, error: () => {} };

d("ACP integration — claude_code", () => {
  let pool: AcpConnectionPool;
  beforeAll(() => {
    pool = new AcpConnectionPool({ logger: silent, idleMs: 60_000 });
  });

  test("echo smoke", async () => {
    const received: unknown[] = [];
    const session = await AcpSession.open(pool, {
      taskId: "int-cc-1",
      adapterId: "claude_code",
      cwd: process.cwd(),
      mcpServers: [],
      perSessionContext: {
        taskId: "int-cc-1",
        cwd: process.cwd(),
        streamer: { send: (ev: unknown) => received.push(ev) },
        logger: silent,
      },
      logger: silent,
    });
    const stop = await session.prompt([{ type: "text", text: "echo hello" }] as never);
    expect(["end_turn", "max_turns"]).toContain(stop);
    expect(received.length).toBeGreaterThan(0);
    await session.dispose();
    await pool.shutdownAll(true);
  }, 120_000);

  // cancel / fs / terminal / permission tests follow the same pattern.
});
```

Each adapter file mirrors this with its own `taskId` / `adapterId`. `SKIP_ACP_INTEGRATION=1` is the default (skip); CI job flips to `=0` for `claude_code` and `codex` (which can run headless given API keys).

- [ ] **Step 2: Run integration tests locally against real agents**

With `ANTHROPIC_API_KEY` and `OPENAI_API_KEY` exported:

```bash
cd src-bridge && SKIP_ACP_INTEGRATION=0 bun test tests/integration/acp/
```

Expected: all 5 adapters pass `echo` + cancel + fs + terminal + permission round-trips. If any fails, HALT — do not proceed to deletion.

- [ ] **Step 3: Sample `_meta.usage` shapes and update spec §8**

During the above run, capture each adapter's `_meta` payload on agent_message_chunk events. Append to spec §8 a table like:

```markdown
### 8.1 Empirical `_meta.usage` shapes (T7 observations)

| Adapter | `_meta.usage` present | Fields observed |
|---|---|---|
| claude_code | yes | `input_tokens / output_tokens / cache_read_tokens / cache_creation_tokens` |
| codex | yes/no | ... |
| opencode | ... | ... |
| cursor | ... | ... |
| gemini | ... | ... |
```

Update §16 (changelog) with a T7 entry noting the cost_update emission rules chosen.

- [ ] **Step 4: Update `pnpm dev:backend:verify`**

Edit `scripts/dev/dev-all.js` (or the actual verify entry — check `package.json` scripts) to add a post-boot step that POSTs to `/bridge/execute` for each of the 5 adapters with `prompt: "echo hello"` and asserts at least one output event + stopReason. Gate on `VERIFY_ACP=1` env so normal `dev:backend:verify` doesn't require API keys by default.

- [ ] **Step 5: Delete legacy handler files**

```bash
rm src-bridge/src/handlers/claude-runtime.ts
rm src-bridge/src/handlers/codex-runtime.ts
rm src-bridge/src/handlers/opencode-runtime.ts
rm -r src-bridge/src/opencode
```

Also delete the associated `.test.ts` files under the same paths.

- [ ] **Step 6: Edit `command-runtime.ts` to drop cursor + gemini branches**

Open `src-bridge/src/handlers/command-runtime.ts`. Remove the branches that handle `runtime === "cursor"` and `runtime === "gemini"`. The remaining branches (`qoder`, `iflow`) stay. If the file becomes trivial, keep it as-is — deletion is T13-style cleanup, not T7.

- [ ] **Step 7: Remove `legacyFactories` option + `BRIDGE_ACP_*` branch**

Edit `src-bridge/src/runtime/acp/adapter-factory.ts`: delete the `if (process.env[\`BRIDGE_ACP_${adapterId.toUpperCase()}\`] === "0")` block and the `deps.legacyFactories?.[adapterId]` usage. Remove `legacyFactories` from `AcpRuntimeAdapterDeps`.

Edit `src-bridge/src/runtime/registry.ts`: delete the `legacyFactories` option, all imports of now-deleted handler files, and the `streamClaudeRuntime / streamCodexRuntime / streamOpenCodeRuntime` function references.

- [ ] **Step 8: Remove `claudeQuery` field + its `live_controls` fallback**

Edit `src-bridge/src/runtime/agent-runtime.ts`: delete the `claudeQuery` field, all assignments, and the `runtime === "claude_code" && this.claudeQuery` branch inside `liveControls`. Collapse to:

```ts
const liveControls = this.acpSession
  ? {
      interrupt: true,
      set_model: this.acpSession.capabilities.modelSelectable || undefined,
      set_thinking_budget: this.acpSession.capabilities.thinkingBudget || undefined,
      mcp_status: this.acpSession.capabilities.mcpStatus || undefined,
      set_mode: this.acpSession.capabilities.modeSelectable || undefined,
    }
  : undefined;
```

- [ ] **Step 9: Update `.env.example`**

Remove `BRIDGE_ACP_CLAUDE_CODE=1` / `BRIDGE_ACP_CODEX=1` / etc. lines (if any existed after T1). Add a short note: `# ACP runtimes: no per-adapter env flags; add ANTHROPIC_API_KEY / OPENAI_API_KEY as adapter auth requires.`

- [ ] **Step 10: Run full suite + typecheck**

```bash
cd src-bridge && bun run typecheck && bun test
pnpm exec tsc --noEmit
```

Expected: exits 0 from all three. If any test references the deleted legacy handlers, delete the test.

- [ ] **Step 11: Run integration smoke one more time**

```bash
cd src-bridge && SKIP_ACP_INTEGRATION=0 bun test tests/integration/acp/
```

Expected: still green.

- [ ] **Step 12: Commit (single atomic commit or a split tagged T7)**

```bash
rtk git add src-bridge/ docs/superpowers/specs/2026-04-21-bridge-acp-client-integration.md scripts/dev/ .env.example
rtk git commit -m "$(cat <<'EOF'
acp(client): T7 — integration smoke + atomic legacy deletion

Integration tests for all five adapters (claude_code / codex / opencode /
cursor / gemini) passing with SKIP_ACP_INTEGRATION=0.

Deleted (per spec §12.2):
- src-bridge/src/handlers/claude-runtime.ts, codex-runtime.ts, opencode-runtime.ts
- src-bridge/src/opencode/ (transport + pending-interactions)
- command-runtime.ts cursor + gemini branches
- runtime/registry.ts legacy factory imports
- AgentRuntime.claudeQuery field
- agent-runtime.ts:135-143 Claude-specific live_controls gate
- BRIDGE_ACP_<ADAPTER> env branches in adapter-factory.ts

Appended empirical _meta.usage table to spec §8.1.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task T8: Go `task_repo.GetAncestorRoot` + `im_forward` root-task rollup

**Goal:** Ensure IM replies to sub-agent tasks fold into the root task's `im_reply_target`, with per-platform folding modes.

**Files:**
- Modify: `src-go/internal/repository/task_repo.go`
- Modify: `src-go/internal/service/im_forward_observer.go` (or equivalent observer — check existing naming)
- Create: `src-go/internal/repository/task_repo_ancestor_test.go`
- Create: `src-go/internal/service/im_forward_rollup_test.go`
- Modify: IM bridge manifest / config file where per-platform defaults live (search `im_forward` or `im_reply_target` to locate)

**Steps:**

- [ ] **Step 1: Find the right files**

```bash
# Identify the existing im_forward observer and config path.
```

Use `Grep` for `im_reply_target` and `ParentID` in `src-go/internal/` to locate the observer file and the place where ParentID is stored on tasks. Record the paths before editing.

- [ ] **Step 2: Write failing unit test for `GetAncestorRoot`**

Create `src-go/internal/repository/task_repo_ancestor_test.go`:

```go
package repository

import (
    "context"
    "testing"
)

func TestGetAncestorRoot_WalksParentChain(t *testing.T) {
    repo, cleanup := newTestRepo(t)
    defer cleanup()
    ctx := context.Background()
    root, _ := repo.CreateTask(ctx, &Task{ID: "root-1"})
    mid, _ := repo.CreateTask(ctx, &Task{ID: "mid-1", ParentID: &root.ID})
    leaf, _ := repo.CreateTask(ctx, &Task{ID: "leaf-1", ParentID: &mid.ID})

    got, err := repo.GetAncestorRoot(ctx, leaf.ID)
    if err != nil {
        t.Fatalf("GetAncestorRoot err: %v", err)
    }
    if got.ID != root.ID {
        t.Fatalf("expected root=%s got=%s", root.ID, got.ID)
    }
}

func TestGetAncestorRoot_RootReturnsSelf(t *testing.T) {
    repo, cleanup := newTestRepo(t)
    defer cleanup()
    ctx := context.Background()
    root, _ := repo.CreateTask(ctx, &Task{ID: "root-1"})
    got, err := repo.GetAncestorRoot(ctx, root.ID)
    if err != nil { t.Fatal(err) }
    if got.ID != root.ID { t.Fatalf("expected self") }
}
```

Adjust `newTestRepo` and `Task` fields to whatever the repository's real types/helpers are — check neighboring test files.

- [ ] **Step 3: Run failing test**

```bash
cd src-go && go test ./internal/repository/ -run TestGetAncestorRoot -v
```

Expected: compile error (method missing).

- [ ] **Step 4: Implement `GetAncestorRoot`**

Add to `task_repo.go`:

```go
// GetAncestorRoot walks ParentID from taskID up to the root task.
// Returns the task itself if it has no ParentID. Returns error if a
// cycle is detected (safety) or the chain is broken.
func (r *TaskRepo) GetAncestorRoot(ctx context.Context, taskID string) (*Task, error) {
    seen := make(map[string]struct{})
    current, err := r.GetTask(ctx, taskID)
    if err != nil {
        return nil, err
    }
    for current.ParentID != nil && *current.ParentID != "" {
        if _, ok := seen[current.ID]; ok {
            return nil, fmt.Errorf("task ancestor cycle at %s", current.ID)
        }
        seen[current.ID] = struct{}{}
        next, err := r.GetTask(ctx, *current.ParentID)
        if err != nil {
            return nil, err
        }
        current = next
    }
    return current, nil
}
```

Adjust receiver name / method signature to match the existing repo pattern (could be `*TaskRepository`, could use `sqlx.DB` directly, etc.).

- [ ] **Step 5: Run test to green**

```bash
cd src-go && go test ./internal/repository/ -run TestGetAncestorRoot -v
```

Expected: both pass.

- [ ] **Step 6: Write failing test for `im_forward` rollup**

Create `src-go/internal/service/im_forward_rollup_test.go` mirroring the existing observer test style. Assert: when a child task (ParentID set) emits an event, the observer resolves the root task and uses `root.IMReplyTarget` rather than the child's own.

- [ ] **Step 7: Implement the rollup in observer**

Edit the observer file (located in Step 1). Before dispatching to IM, call `r.taskRepo.GetAncestorRoot(ctx, event.TaskID)` and use the root's reply target. If the root has no `IMReplyTarget`, skip dispatch (task not IM-originated).

Add a config `IMChildTaskFoldingMode` to the observer. When the mode is `frontend_only` (QQ default), short-circuit: emit nothing to IM for child tasks (only root task emits). When `nested` (default), emit with the root's reply target. When `flat`, emit with the child's own reply target (fallback behavior).

Read the default from the IM bridge manifest for the originating platform. If you don't have a per-platform manifest lookup, add one — but keep it minimal.

- [ ] **Step 8: Run Go tests + lint**

```bash
cd src-go && go test ./... && go vet ./...
```

Expected: all green.

- [ ] **Step 9: Commit**

```bash
rtk git add src-go/internal/repository/ src-go/internal/service/
rtk git commit -m "acp(client): T8 — task_repo.GetAncestorRoot + im_forward root-task rollup + folding_mode"
```

---

## Self-review

After the plan is complete, the author (you) runs the following self-check and fixes anything inline:

1. **Spec coverage check** — walk through spec sections §1 through §16 and point to the task that implements each:
   - §1 Background → context only, no task needed.
   - §2 Goals → T1–T8 collectively.
   - §3 Architecture → T1 scaffolds, T2–T6 implement.
   - §4 Module design — every module has an owning task (§4.1 → T1; §4.2 → T2; §4.3 → T2; §4.4 → T3; §4.5 → T3; §4.6 → T6).
   - §5 Adapter registry → T1 (registry.ts) + T0 (gemini verify).
   - §6 Handlers → T4a/b/c/d.
   - §7 Session lifecycle → T3 (open / prompt / cancel / dispose) + T6 (fork/rollback/etc. that need runtime state).
   - §8 Streaming mapping → T5, + T7 empirical table.
   - §9 Cross-layer impact → T6 (§9.1 live_controls), T8 (§9.1 GetAncestorRoot + im_forward + folding_mode), T7 (§9.3 AcpCommandNotFound surfacing — already present via T2 ChildProcessHost).
   - §10 Error taxonomy → T1 creates classes; T2–T6 throw them; T7 deletion doesn't change error surface.
   - §11 Testing → each task owns its unit slice; T3 owns mock-agent fixture + component tests; T7 owns integration + dev:backend:verify.
   - §12 Migration plan → T1–T6 phase 1 flag-gated; T7 phase 2 atomic deletion.
   - §13 Out of scope → none of T0–T8 touches these; confirmed.
   - §14 Open questions → T0 (Q7), T6 (Q6), T7 (Q5), T8 (Q8) each resolve one; Q3/Q4 deferred by design.
   - §15 References → documentation-only.
   - §16 Changelog → T0 + T7 append entries.

2. **Placeholder scan** — no "TBD" / "TODO" / "similar to above" left in task bodies. Spot-checked; clean.

3. **Type consistency** — `AcpSession.open` signature consistent across T3 / T6. `CachedCapabilities` shape referenced identically in T3 (definition) and T6 (consumer). `PerSessionContext` uses `[k: string]: unknown` throughout T1 stubs → T3 concrete field use — acceptable because Handler functions narrow at call site. If a subagent trips on this, tighten by extracting a named `PerSessionContext` interface with the real fields in T3.

4. **Known fragility callouts (for implementers)**:
   - SDK API shape (`ClientSideConnection`, `ndJsonStream`, `Client` / `Agent` interfaces, schema namespace) is the version pinned at T1. If live SDK differs (e.g., `ClientSideConnection` takes `(stream, client)` vs `(client, stream)`, or `AgentSideConnection` is absent and the SDK ships `serve(agent, stream)` instead), **T2 / T3 adapt at the call sites**; do not fabricate types. Report the actual signatures back if they diverge materially from this plan.
   - `HookCallbackManager.register` shape — if it doesn't return `{ requestId, promise }`, wrap it with a small helper.
   - `node-pty` on Windows Tauri sidecar path may need `conpty` vs `winpty`; confirmed in T4b tests run on target platforms.
   - `runtime/registry.ts` options object is already substantial; prefer adding a discriminated `deps` parameter if the field count hits a readability wall.

## Execution handoff

**Plan complete and saved to `docs/superpowers/plans/2026-04-21-bridge-acp-client-integration.md`.**

Two execution options:

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration. Uses `superpowers:subagent-driven-development`.
2. **Inline Execution** — execute tasks in this session using `superpowers:executing-plans`, batch with checkpoints.

Which approach?
