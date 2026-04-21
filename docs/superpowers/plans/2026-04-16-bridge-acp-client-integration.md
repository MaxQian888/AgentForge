> **Status: Superseded.** This plan was written against draft-2 of the ACP integration spec, which has been superseded by [`docs/superpowers/specs/2026-04-21-bridge-acp-client-integration.md`](../../../superpowers/specs/2026-04-21-bridge-acp-client-integration.md). A new implementation plan will be generated from the 2026-04-21 spec via superpowers:writing-plans. Do not execute this plan.

# Bridge ACP Client Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate five `src-bridge/` agent adapters (`claude_code / codex / opencode / cursor / gemini`) to speak ACP via the official `@agentclientprotocol/sdk`, with adapter-level process pooling, capability-gated unstable method passthrough, and preserved end-to-end IM path.

**Architecture:** Reuse SDK's `ClientSideConnection` + `ndJsonStream` (no DIY transport). One pooled `ChildProcessHost` per adapter owns one SDK connection; a `MultiplexedClient` routes inbound `Client` calls (fs / terminal / permission / elicitation / sessionUpdate) by `sessionId` to per-task contexts. `AcpSession` wraps `(pool, sessionId)` and is what `runtime/registry.ts` adapter factories return, mapped to the existing `RuntimeAdapter` 12-method face. Legacy runtime files deleted after smoke green.

**Tech Stack:** TypeScript, Bun runtime, Hono HTTP, `@agentclientprotocol/sdk@0.19.0`, `node-pty`, existing `HookCallbackManager` / `EventStreamer` / `TerminalManager` / `FsSandbox`.

**Source of truth:** `docs/superpowers/specs/2026-04-16-bridge-acp-client-integration.md` (draft-2). Every task links to a section; diverging requires spec update first.

---

## File Structure

### New files (src-bridge/src/runtime/acp/)
| File | Responsibility |
|---|---|
| `process-host.ts` | Spawn + graceful shutdown + stderr ring-buffer |
| `connection-pool.ts` | Per-adapter singleton: host + SDK `ClientSideConnection` + ref count |
| `multiplexed-client.ts` | Implements SDK `Client`; sessionId routing |
| `session.ts` | `AcpSession` — per-(task, sessionId) public face (stable + unstable) |
| `capabilities.ts` | Capability helpers + `AcpCapabilityUnsupported` guard |
| `handlers/fs.ts` | readTextFile / writeTextFile via FsSandbox |
| `handlers/terminal.ts` | 6 terminal methods via TerminalManager |
| `handlers/permission.ts` | requestPermission via HookCallbackManager |
| `handlers/elicitation.ts` | unstable_createElicitation passthrough |
| `events/session-update.ts` | `SessionNotification` → `AgentEventType` mapping |
| `adapter-factory.ts` | `createAcpRuntimeAdapter(adapterId)` returning `RuntimeAdapter` |
| `index.ts` | Public barrel |

### Test files (src-bridge/tests/)
| File | Covers |
|---|---|
| `unit/runtime/acp/fs-sandbox.test.ts` | Path escape rejection |
| `unit/runtime/acp/multiplexed-client.test.ts` | sessionId routing |
| `unit/runtime/acp/session-update-mapping.test.ts` | Every stable + unstable variant + unknown fallback |
| `unit/runtime/acp/permission-router.test.ts` | Allow/reject/timeout/cancel |
| `unit/runtime/acp/capability-gate.test.ts` | Unstable gated methods |
| `unit/runtime/acp/connection-pool.test.ts` | Concurrent acquire, ref count, idle reclaim, crash |
| `component/acp/mock-acp-agent.ts` | Standalone JSON-RPC fixture |
| `component/acp/happy-path.test.ts` | init → newSession → prompt → stop |
| `component/acp/cancel-race.test.ts` | Cancel before/during/after |
| `component/acp/pooling.test.ts` | Two tasks share host; crash propagates |
| `component/acp/multi-session-fs.test.ts` | Concurrent sessions' fs/terminal routing |
| `integration/acp/<adapter>.test.ts` (×5) | Real adapter: smoke/cancel/fs/terminal/permission |

### Modified files
| File | Change |
|---|---|
| `src-bridge/src/runtime/acp/registry.ts` | Add `gemini` entry (5 total) |
| `src-bridge/src/runtime/acp/errors.ts` | Add `AcpCapabilityUnsupported`, `AcpAuthMissing`, `AcpCommandNotFound` |
| `src-bridge/src/runtime/registry.ts` | Wire `createAcpRuntimeAdapter` for 5 adapters; keep legacy fallback behind `BRIDGE_ACP_<ADAPTER>=0` |
| `src-bridge/src/runtime/agent-runtime.ts` L135-143 | Drop `runtime === "claude_code"` gate; capability-driven `live_controls` |
| `src-bridge/src/handlers/command-runtime.ts` | Remove `cursor` + `gemini` adapter branches (keep qoder/iflow) |
| `src-go/internal/repository/task_repo.go` | Add `GetAncestorRoot(taskID)` |
| `src-go/internal/service/im_*_execution.go` or forwarder | Use root task's `im_reply_target` |
| `pnpm-workspace scripts` (`dev:backend:verify`) | 5-adapter echo |
| `.env.example` | Document `BRIDGE_ACP_*` flags |
| `docs/PRD.md` | Note ACP migration status |

### Deleted files (T9, after smoke green)
- `src-bridge/src/handlers/claude-runtime.ts`
- `src-bridge/src/handlers/codex-runtime.ts`
- `src-bridge/src/handlers/opencode-runtime.ts`
- `src-bridge/src/opencode/` (legacy transport)
- Legacy adapter factories in `src-bridge/src/runtime/registry.ts` for cc/codex/opencode/cursor/gemini
- `AgentRuntime.claudeQuery` field and its single-runtime gate

---

## Task 0: Verify Gemini ACP spawn command

**Files:**
- Modify: `src-bridge/src/runtime/acp/registry.ts` (pin exact `gemini` args)

Spec §5.1 + §12.7. The `gemini` CLI may expose ACP as `--experimental-acp`, `acp`, or require a different subcommand. Pin correct form before integration tests depend on it.

- [ ] **Step 1: Check if `gemini` CLI is installed and inspect help**

Run:
```bash
which gemini && gemini --help 2>&1 | head -40
```
Expected: either lists an `acp` / `--experimental-acp` option, or returns "command not found".

If not found, install per https://github.com/google-gemini/gemini-cli and rerun.

- [ ] **Step 2: Probe subcommand help for ACP hint**

Run:
```bash
gemini acp --help 2>&1 || gemini --experimental-acp --help 2>&1 || echo "neither form works"
```
Expected: one form prints ACP-specific help (mentions stdio / agent client protocol). Note which form.

- [ ] **Step 3: Cross-check against Gemini CLI source**

Open https://github.com/google-gemini/gemini-cli/blob/main/packages/cli/src/zed-integration/zedIntegration.ts. Note the entry the CLI uses for Zed integration — this is the authoritative spawn form.

- [ ] **Step 4: Update `registry.ts` with verified args**

If the correct form turns out to be `gemini acp` (no `--experimental-` prefix), edit `src-bridge/src/runtime/acp/registry.ts` line 54 equivalent:

```ts
  gemini: {
    command: "gemini",
    args: ["acp"],                       // verified from Step 2/3
    envRequired: [],
    cursorExtensions: false,
  },
```

- [ ] **Step 5: Commit**

```bash
git add src-bridge/src/runtime/acp/registry.ts
git commit -m "feat(acp): add gemini adapter to ACP registry with verified spawn args"
```

---

## Task 1 (scaffolding — already done)

`src-bridge/src/runtime/acp/errors.ts` and `registry.ts` exist from prior commit `ac8da9b`. Task 0 added gemini. No further work here — proceed to T2.

---

## Task 2a: Extend errors with new classes

**Files:**
- Modify: `src-bridge/src/runtime/acp/errors.ts`
- Test: `src-bridge/src/runtime/acp/errors.test.ts` (new)

Spec §4.1 requires `AcpCapabilityUnsupported`, `AcpAuthMissing`, `AcpCommandNotFound`. Spec §4.4 requires `AcpProcessCrash` to carry stderr + exit code.

- [ ] **Step 1: Write failing test**

Create `src-bridge/src/runtime/acp/errors.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import {
  AcpCapabilityUnsupported,
  AcpAuthMissing,
  AcpCommandNotFound,
  AcpProcessCrash,
} from "./errors.js";

describe("acp errors (extended)", () => {
  test("AcpCapabilityUnsupported carries method and reason", () => {
    const err = new AcpCapabilityUnsupported("setModel", "no_rpc_advertised");
    expect(err.name).toBe("AcpCapabilityUnsupported");
    expect(err.method).toBe("setModel");
    expect(err.reason).toBe("no_rpc_advertised");
  });

  test("AcpAuthMissing lists missing env vars", () => {
    const err = new AcpAuthMissing("claude_code", ["ANTHROPIC_API_KEY"]);
    expect(err.name).toBe("AcpAuthMissing");
    expect(err.adapterId).toBe("claude_code");
    expect(err.missingEnv).toEqual(["ANTHROPIC_API_KEY"]);
  });

  test("AcpCommandNotFound carries adapterId and command", () => {
    const err = new AcpCommandNotFound("gemini", "gemini");
    expect(err.name).toBe("AcpCommandNotFound");
    expect(err.adapterId).toBe("gemini");
    expect(err.command).toBe("gemini");
  });

  test("AcpProcessCrash carries stderr tail and exit info", () => {
    const err = new AcpProcessCrash("child exited", {
      stderrTail: "oom",
      exitCode: 137,
      signal: "SIGKILL",
    });
    expect(err.stderrTail).toBe("oom");
    expect(err.exitCode).toBe(137);
    expect(err.signal).toBe("SIGKILL");
  });
});
```

- [ ] **Step 2: Run test (should fail: classes not exported / signatures wrong)**

Run: `cd src-bridge && bun test src/runtime/acp/errors.test.ts`
Expected: 4 failures — missing exports / missing fields on `AcpProcessCrash`.

- [ ] **Step 3: Extend `errors.ts`**

Append to `src-bridge/src/runtime/acp/errors.ts`:

```ts
/**
 * A method was invoked on an `AcpSession` that the negotiated agent
 * capabilities do not advertise. Callers map this to structured
 * `{support_state:"unsupported", reason_code}` per runtime-capability
 * contract.
 */
export class AcpCapabilityUnsupported extends Error {
  readonly method: string;
  readonly reason: string;
  constructor(method: string, reason: string) {
    super(`ACP capability unsupported: ${method} (${reason})`);
    this.name = "AcpCapabilityUnsupported";
    this.method = method;
    this.reason = reason;
  }
}

/**
 * Adapter cannot be spawned because required env variables are absent.
 * Raised inside `AcpConnectionPool.acquire` before any process is forked.
 */
export class AcpAuthMissing extends Error {
  readonly adapterId: string;
  readonly missingEnv: string[];
  constructor(adapterId: string, missingEnv: string[]) {
    super(`ACP auth missing for ${adapterId}: ${missingEnv.join(", ")}`);
    this.name = "AcpAuthMissing";
    this.adapterId = adapterId;
    this.missingEnv = missingEnv;
  }
}

/**
 * `ChildProcessHost.start()` failed because the binary resolved from
 * `ACP_ADAPTERS[adapterId].command` is not on PATH. User must install it.
 */
export class AcpCommandNotFound extends Error {
  readonly adapterId: string;
  readonly command: string;
  constructor(adapterId: string, command: string) {
    super(`ACP command not found for ${adapterId}: ${command}`);
    this.name = "AcpCommandNotFound";
    this.adapterId = adapterId;
    this.command = command;
  }
}
```

Then replace the existing `AcpProcessCrash` class (around line 24) with:

```ts
export interface AcpProcessCrashDetails {
  stderrTail: string;
  exitCode: number;
  signal?: string;
}

export class AcpProcessCrash extends Error {
  readonly stderrTail: string;
  readonly exitCode: number;
  readonly signal?: string;
  constructor(message: string, details: AcpProcessCrashDetails) {
    super(message);
    this.name = "AcpProcessCrash";
    this.stderrTail = details.stderrTail;
    this.exitCode = details.exitCode;
    this.signal = details.signal;
  }
}
```

- [ ] **Step 4: Run test — expect PASS**

Run: `cd src-bridge && bun test src/runtime/acp/errors.test.ts`
Expected: 4 passes.

- [ ] **Step 5: Commit**

```bash
git add src-bridge/src/runtime/acp/errors.ts src-bridge/src/runtime/acp/errors.test.ts
git commit -m "feat(acp): extend error vocabulary with capability/auth/command/crash details"
```

---

## Task 2b: `ChildProcessHost`

**Files:**
- Create: `src-bridge/src/runtime/acp/process-host.ts`
- Test: `src-bridge/src/runtime/acp/process-host.test.ts`

Spec §4.2. Owns spawning, stderr ring-buffer, graceful shutdown. Does not touch JSON-RPC.

- [ ] **Step 1: Write failing tests**

Create `src-bridge/src/runtime/acp/process-host.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { ChildProcessHost } from "./process-host.js";

describe("ChildProcessHost", () => {
  test("spawns child and exposes stdin/stdout streams", async () => {
    const host = new ChildProcessHost({
      adapterId: "claude_code",
      command: "node",
      args: ["-e", "process.stdin.pipe(process.stdout)"],
      env: {},
      logger: console as any,
    });
    const io = await host.start();
    expect(io.stdin).toBeDefined();
    expect(io.stdout).toBeDefined();
    await host.shutdown(100);
  });

  test("captures stderr into ring buffer", async () => {
    const host = new ChildProcessHost({
      adapterId: "claude_code",
      command: "node",
      args: ["-e", "console.error('boom'); setInterval(()=>{},1000)"],
      env: {},
      logger: console as any,
    });
    await host.start();
    await new Promise((r) => setTimeout(r, 200));
    expect(host.stderrBuffer.tail()).toContain("boom");
    await host.shutdown(100);
  });

  test("throws AcpCommandNotFound when binary missing", async () => {
    const host = new ChildProcessHost({
      adapterId: "claude_code",
      command: "definitely-not-a-real-binary-xyz",
      args: [],
      env: {},
      logger: console as any,
    });
    await expect(host.start()).rejects.toMatchObject({
      name: "AcpCommandNotFound",
    });
  });

  test("shutdown: closes stdin then SIGTERM then SIGKILL", async () => {
    const host = new ChildProcessHost({
      adapterId: "claude_code",
      command: "node",
      args: ["-e", "setInterval(()=>{},1000)"],
      env: {},
      logger: console as any,
    });
    await host.start();
    const t0 = Date.now();
    await host.shutdown(50);
    expect(Date.now() - t0).toBeLessThan(3000);
    expect(await host.exited).toBeGreaterThanOrEqual(0);
  });
});
```

- [ ] **Step 2: Run tests — expect FAIL (module missing)**

Run: `cd src-bridge && bun test src/runtime/acp/process-host.test.ts`
Expected: import error — `process-host.ts` does not exist.

- [ ] **Step 3: Implement `process-host.ts`**

Create `src-bridge/src/runtime/acp/process-host.ts`:

```ts
import { AcpCommandNotFound } from "./errors.js";
import type { AdapterId } from "./registry.js";

export interface Logger {
  debug(msg: string, meta?: unknown): void;
  warn(msg: string, meta?: unknown): void;
  error(msg: string, meta?: unknown): void;
}

export interface ChildProcessHostOptions {
  adapterId: AdapterId;
  command: string;
  args: readonly string[];
  env: Record<string, string>;
  logger: Logger;
}

export class RingBuffer {
  private parts: string[] = [];
  private bytes = 0;
  constructor(private readonly limit: number) {}
  append(chunk: string): void {
    this.parts.push(chunk);
    this.bytes += chunk.length;
    while (this.bytes > this.limit && this.parts.length > 1) {
      this.bytes -= this.parts.shift()!.length;
    }
  }
  tail(): string {
    return this.parts.join("");
  }
}

export class ChildProcessHost {
  readonly stderrBuffer = new RingBuffer(8 * 1024);
  private proc?: ReturnType<typeof Bun.spawn>;
  private resolveExit!: (code: number) => void;
  readonly exited = new Promise<number>((r) => {
    this.resolveExit = r;
  });
  private startingEnv: Record<string, string>;

  constructor(private readonly opts: ChildProcessHostOptions) {
    this.startingEnv = { ...process.env, ...opts.env } as Record<string, string>;
  }

  async start(): Promise<{
    stdin: WritableStream<Uint8Array>;
    stdout: ReadableStream<Uint8Array>;
  }> {
    try {
      this.proc = Bun.spawn([this.opts.command, ...this.opts.args], {
        stdin: "pipe",
        stdout: "pipe",
        stderr: "pipe",
        env: this.startingEnv,
      });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      if (/ENOENT|not found|not recognized/i.test(msg)) {
        throw new AcpCommandNotFound(this.opts.adapterId, this.opts.command);
      }
      throw err;
    }

    // Drain stderr into ring buffer
    (async () => {
      const reader = (this.proc!.stderr as ReadableStream<Uint8Array>).getReader();
      const dec = new TextDecoder();
      for (;;) {
        const { value, done } = await reader.read();
        if (done) return;
        this.stderrBuffer.append(dec.decode(value));
      }
    })().catch((e) => this.opts.logger.warn("stderr drain error", e));

    this.proc.exited.then((code) => this.resolveExit(code as number));

    return {
      stdin: this.proc.stdin as unknown as WritableStream<Uint8Array>,
      stdout: this.proc.stdout as unknown as ReadableStream<Uint8Array>,
    };
  }

  async shutdown(gracefulMs = 2000): Promise<void> {
    if (!this.proc) return;
    try {
      // Close stdin (signals EOF to agents that watch it)
      try {
        await (this.proc.stdin as any).end?.();
      } catch {
        /* already closed */
      }
      const raced = await Promise.race([
        this.exited,
        new Promise<null>((r) => setTimeout(() => r(null), gracefulMs)),
      ]);
      if (raced === null) {
        this.proc.kill("SIGTERM");
        const killed = await Promise.race([
          this.exited,
          new Promise<null>((r) => setTimeout(() => r(null), 3000)),
        ]);
        if (killed === null) this.proc.kill("SIGKILL");
      }
    } finally {
      try {
        await this.exited;
      } catch {
        /* swallowed */
      }
    }
  }
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `cd src-bridge && bun test src/runtime/acp/process-host.test.ts`
Expected: 4 passes.

- [ ] **Step 5: Type check**

Run: `cd src-bridge && bun run typecheck`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add src-bridge/src/runtime/acp/process-host.ts src-bridge/src/runtime/acp/process-host.test.ts
git commit -m "feat(acp): add ChildProcessHost with stderr ring buffer and graceful shutdown"
```

---

## Task 2c: `AcpConnectionPool`

**Files:**
- Create: `src-bridge/src/runtime/acp/connection-pool.ts`
- Test: `src-bridge/src/runtime/acp/connection-pool.test.ts`

Spec §4.3. Singleton holding per-adapter `PooledEntry` (host + SDK conn + caps + ref count).

- [ ] **Step 1: Write failing tests**

Create `src-bridge/src/runtime/acp/connection-pool.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { AcpConnectionPool, type PooledEntryFactory } from "./connection-pool.js";

function stubFactory(): PooledEntryFactory {
  let counter = 0;
  return async (adapterId) => {
    const id = ++counter;
    return {
      host: { shutdown: async () => {}, exited: Promise.resolve(0) } as any,
      conn: {} as any,
      caps: {} as any,
      clientDispatcher: { register: () => {}, unregister: () => {} } as any,
      sessions: new Set<string>(),
      restartPending: false,
      _id: id,
    } as any;
  };
}

describe("AcpConnectionPool", () => {
  test("concurrent acquire → single spawn (mutex)", async () => {
    const factory = stubFactory();
    const pool = new AcpConnectionPool({ logger: console as any, factory });
    const [a, b] = await Promise.all([
      pool.acquire("claude_code"),
      pool.acquire("claude_code"),
    ]);
    expect((a as any)._id).toBe((b as any)._id);
  });

  test("release decrements ref count and schedules idle shutdown", async () => {
    const factory = stubFactory();
    const pool = new AcpConnectionPool({
      logger: console as any,
      factory,
      idleMs: 50,
    });
    const entry = await pool.acquire("claude_code");
    entry.sessions.add("s1");
    await pool.release("claude_code", "s1");
    await new Promise((r) => setTimeout(r, 100));
    // After idle timeout, next acquire should spawn a new entry
    const again = await pool.acquire("claude_code");
    expect((again as any)._id).not.toBe((entry as any)._id);
  });

  test("acquire on restartPending entry spawns fresh", async () => {
    const factory = stubFactory();
    const pool = new AcpConnectionPool({ logger: console as any, factory });
    const first = await pool.acquire("claude_code");
    (first as any).restartPending = true;
    const second = await pool.acquire("claude_code");
    expect((second as any)._id).not.toBe((first as any)._id);
  });
});
```

- [ ] **Step 2: Run tests — expect FAIL (module missing)**

Run: `cd src-bridge && bun test src/runtime/acp/connection-pool.test.ts`
Expected: import error.

- [ ] **Step 3: Implement `connection-pool.ts`**

Create `src-bridge/src/runtime/acp/connection-pool.ts`:

```ts
import type { ClientSideConnection, Client, schema } from "@agentclientprotocol/sdk";
import type { ChildProcessHost, Logger } from "./process-host.js";
import type { AdapterId } from "./registry.js";

export interface PooledEntry {
  host: ChildProcessHost;
  conn: ClientSideConnection;
  caps: schema.AgentCapabilities;
  clientDispatcher: Client & {
    register(sid: string, ctx: unknown): void;
    unregister(sid: string): void;
  };
  sessions: Set<string>;
  restartPending: boolean;
}

export type PooledEntryFactory = (adapterId: AdapterId) => Promise<PooledEntry>;

export interface AcpConnectionPoolOptions {
  logger: Logger;
  factory: PooledEntryFactory;
  idleMs?: number;
}

export class AcpConnectionPool {
  private entries = new Map<AdapterId, PooledEntry>();
  private mutex = new Map<AdapterId, Promise<PooledEntry>>();
  private idleTimers = new Map<AdapterId, ReturnType<typeof setTimeout>>();
  private readonly idleMs: number;

  constructor(private readonly opts: AcpConnectionPoolOptions) {
    this.idleMs = opts.idleMs ?? 600_000;
  }

  async acquire(adapterId: AdapterId): Promise<PooledEntry> {
    this.cancelIdle(adapterId);
    const existing = this.entries.get(adapterId);
    if (existing && !existing.restartPending) return existing;

    const pending = this.mutex.get(adapterId);
    if (pending) return pending;

    const p = (async () => {
      if (existing?.restartPending) {
        await existing.host.shutdown(500).catch(() => {});
        this.entries.delete(adapterId);
      }
      const fresh = await this.opts.factory(adapterId);
      this.entries.set(adapterId, fresh);
      return fresh;
    })().finally(() => this.mutex.delete(adapterId));

    this.mutex.set(adapterId, p);
    return p;
  }

  async release(adapterId: AdapterId, sessionId: string): Promise<void> {
    const entry = this.entries.get(adapterId);
    if (!entry) return;
    entry.sessions.delete(sessionId);
    if (entry.sessions.size === 0) this.scheduleIdle(adapterId);
  }

  async shutdownAll(graceful = true): Promise<void> {
    for (const [id, t] of this.idleTimers) {
      clearTimeout(t);
      this.idleTimers.delete(id);
    }
    const entries = Array.from(this.entries.values());
    this.entries.clear();
    await Promise.all(entries.map((e) => e.host.shutdown(graceful ? 2000 : 0).catch(() => {})));
  }

  private scheduleIdle(adapterId: AdapterId): void {
    this.cancelIdle(adapterId);
    const t = setTimeout(() => {
      const entry = this.entries.get(adapterId);
      if (entry && entry.sessions.size === 0) {
        this.entries.delete(adapterId);
        entry.host.shutdown(2000).catch(() => {});
      }
      this.idleTimers.delete(adapterId);
    }, this.idleMs);
    this.idleTimers.set(adapterId, t);
  }

  private cancelIdle(adapterId: AdapterId): void {
    const t = this.idleTimers.get(adapterId);
    if (t) {
      clearTimeout(t);
      this.idleTimers.delete(adapterId);
    }
  }
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `cd src-bridge && bun test src/runtime/acp/connection-pool.test.ts`
Expected: 3 passes.

- [ ] **Step 5: Commit**

```bash
git add src-bridge/src/runtime/acp/connection-pool.ts src-bridge/src/runtime/acp/connection-pool.test.ts
git commit -m "feat(acp): add AcpConnectionPool with per-adapter mutex and idle reclaim"
```

---

## Task 3a: Capability helpers

**Files:**
- Create: `src-bridge/src/runtime/acp/capabilities.ts`
- Test: `src-bridge/src/runtime/acp/capabilities.test.ts`

Spec §4.5, §4.6. Central guards for unstable-method gating; `live_controls` feature detection.

- [ ] **Step 1: Write failing tests**

Create `src-bridge/src/runtime/acp/capabilities.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { gateUnstable, liveControlsFor } from "./capabilities.js";
import { AcpCapabilityUnsupported } from "./errors.js";

describe("capabilities.gateUnstable", () => {
  test("passes when capability advertised", () => {
    const caps = { session: { fork: true } } as any;
    expect(() => gateUnstable(caps, "fork", "session.fork")).not.toThrow();
  });

  test("throws AcpCapabilityUnsupported when missing", () => {
    const caps = { session: {} } as any;
    expect(() => gateUnstable(caps, "fork", "session.fork")).toThrow(
      AcpCapabilityUnsupported,
    );
  });

  test("dotted path navigation", () => {
    const caps = { a: { b: { c: true } } } as any;
    expect(() => gateUnstable(caps, "x", "a.b.c")).not.toThrow();
    expect(() => gateUnstable(caps, "x", "a.b.d")).toThrow();
  });
});

describe("capabilities.liveControlsFor", () => {
  test("populates setModel when availableModels + setConfigOption advertised", () => {
    const caps = {
      availableModels: [{ id: "claude-opus" }],
      promptCapabilities: { thinkingBudget: false },
      availableModes: [],
      mcpCapabilities: {},
    } as any;
    const lc = liveControlsFor(caps);
    expect(lc.setModel).toBe(true);
    expect(lc.setThinkingBudget).toBe(false);
  });

  test("sets setThinkingBudget when advertised", () => {
    const caps = {
      availableModels: [],
      promptCapabilities: { thinkingBudget: true },
      availableModes: [],
      mcpCapabilities: {},
    } as any;
    expect(liveControlsFor(caps).setThinkingBudget).toBe(true);
  });
});
```

- [ ] **Step 2: Run tests — expect FAIL**

Run: `cd src-bridge && bun test src/runtime/acp/capabilities.test.ts`
Expected: import error.

- [ ] **Step 3: Implement `capabilities.ts`**

Create `src-bridge/src/runtime/acp/capabilities.ts`:

```ts
import type { schema } from "@agentclientprotocol/sdk";
import { AcpCapabilityUnsupported } from "./errors.js";

export function gateUnstable(
  caps: schema.AgentCapabilities,
  method: string,
  path: string,
): void {
  const segments = path.split(".");
  let cur: any = caps;
  for (const s of segments) {
    if (cur == null || typeof cur !== "object" || !(s in cur)) {
      throw new AcpCapabilityUnsupported(method, `no_capability_${path}`);
    }
    cur = cur[s];
  }
  if (!cur) throw new AcpCapabilityUnsupported(method, `no_capability_${path}`);
}

export interface LiveControlsFlags {
  setModel: boolean;
  setMode: boolean;
  setThinkingBudget: boolean;
  setConfigOption: boolean;
  mcpServerStatus: boolean;
}

export function liveControlsFor(caps: schema.AgentCapabilities): LiveControlsFlags {
  const hasModels = Array.isArray(caps.availableModels) && caps.availableModels.length > 0;
  const hasModes = Array.isArray(caps.availableModes) && caps.availableModes.length > 0;
  const thinkingBudget = caps.promptCapabilities?.thinkingBudget === true;
  const mcp = caps.mcpCapabilities ?? {};
  return {
    setModel: hasModels,
    setMode: hasModes,
    setThinkingBudget: thinkingBudget,
    setConfigOption: true, // stable method; always available once session exists
    mcpServerStatus: Boolean(mcp.http || mcp.sse),
  };
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `cd src-bridge && bun test src/runtime/acp/capabilities.test.ts`
Expected: 5 passes.

- [ ] **Step 5: Commit**

```bash
git add src-bridge/src/runtime/acp/capabilities.ts src-bridge/src/runtime/acp/capabilities.test.ts
git commit -m "feat(acp): add capability gates and live_controls feature detection"
```

---

## Task 3b: `MultiplexedClient`

**Files:**
- Create: `src-bridge/src/runtime/acp/multiplexed-client.ts`
- Test: `src-bridge/src/runtime/acp/multiplexed-client.test.ts`

Spec §4.4. Implements SDK `Client`; dispatches inbound calls by `sessionId`. Handlers delegate to per-session `ctx`.

- [ ] **Step 1: Write failing tests**

Create `src-bridge/src/runtime/acp/multiplexed-client.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { MultiplexedClient } from "./multiplexed-client.js";

function makeCtx() {
  const calls: string[] = [];
  return {
    ctx: {
      taskId: "t1",
      cwd: "/tmp/wt",
      fsSandbox: {
        resolve: (_sid: string, p: string) => {
          calls.push(`fs:${p}`);
          return "/tmp/wt/" + p;
        },
      } as any,
      terminalManager: { createTerminal: async () => ({ terminalId: "tx1" }) } as any,
      permissionRouter: {
        request: async () => ({ outcome: "selected", optionId: "allow" }),
      } as any,
      streamer: { emit: (ev: unknown) => calls.push(`emit:${(ev as any).type}`) } as any,
      logger: console as any,
    },
    calls,
  };
}

describe("MultiplexedClient", () => {
  test("routes fs/read_text_file by sessionId", async () => {
    const mc = new MultiplexedClient({ logger: console as any });
    const { ctx, calls } = makeCtx();
    mc.register("s1", ctx);
    await expect(
      mc.readTextFile!({ sessionId: "s1", path: "foo.ts" } as any),
    ).rejects.toThrow(/ENOENT|not found/i); // fs.readFile will fail on stub path
    expect(calls.some((c) => c === "fs:foo.ts")).toBe(true);
  });

  test("rejects unknown sessionId with unknown_session", async () => {
    const mc = new MultiplexedClient({ logger: console as any });
    await expect(
      mc.readTextFile!({ sessionId: "unknown", path: "x" } as any),
    ).rejects.toMatchObject({ code: -32602 });
  });

  test("sessionUpdate errors are swallowed (notification)", async () => {
    const mc = new MultiplexedClient({ logger: console as any });
    // No session registered — sessionUpdate MUST NOT throw
    await expect(
      mc.sessionUpdate({ sessionId: "unknown", update: {} as any } as any),
    ).resolves.toBeUndefined();
  });

  test("unregister removes session", async () => {
    const mc = new MultiplexedClient({ logger: console as any });
    const { ctx } = makeCtx();
    mc.register("s2", ctx);
    mc.unregister("s2");
    await expect(
      mc.readTextFile!({ sessionId: "s2", path: "x" } as any),
    ).rejects.toMatchObject({ code: -32602 });
  });
});
```

- [ ] **Step 2: Run tests — expect FAIL**

Run: `cd src-bridge && bun test src/runtime/acp/multiplexed-client.test.ts`
Expected: module missing.

- [ ] **Step 3: Implement `multiplexed-client.ts`**

Create `src-bridge/src/runtime/acp/multiplexed-client.ts`:

```ts
import { RequestError, type Client, type schema } from "@agentclientprotocol/sdk";
import type { Logger } from "./process-host.js";
import * as fsH from "./handlers/fs.js";
import * as termH from "./handlers/terminal.js";
import * as permH from "./handlers/permission.js";
import * as elicH from "./handlers/elicitation.js";
import { mapSessionUpdate } from "./events/session-update.js";

export interface PerSessionContext {
  taskId: string;
  cwd: string;
  fsSandbox: {
    resolve(sessionId: string, path: string): string;
  };
  terminalManager: unknown; // typed in handlers/terminal.ts
  permissionRouter: {
    request(
      taskId: string,
      toolCall: schema.ToolCallUpdate,
      options: schema.PermissionOption[],
    ): Promise<schema.RequestPermissionOutcome>;
  };
  streamer: { emit(event: unknown): void };
  logger: Logger;
}

export class MultiplexedClient implements Client {
  private sessions = new Map<string, PerSessionContext>();
  constructor(private readonly opts: { logger: Logger }) {}

  register(sessionId: string, ctx: PerSessionContext): void {
    this.sessions.set(sessionId, ctx);
  }
  unregister(sessionId: string): void {
    this.sessions.delete(sessionId);
  }

  private require(sid: string): PerSessionContext {
    const ctx = this.sessions.get(sid);
    if (!ctx) {
      throw new RequestError(-32602, "unknown_session", { sessionId: sid });
    }
    return ctx;
  }

  // ── notifications (errors swallowed) ────────────────────────────────────
  async sessionUpdate(params: schema.SessionNotification): Promise<void> {
    try {
      const ctx = this.sessions.get((params as any).sessionId);
      if (!ctx) return;
      const ev = mapSessionUpdate(params);
      ctx.streamer.emit(ev);
    } catch (err) {
      this.opts.logger.warn("sessionUpdate handler failed", err);
    }
  }

  // ── requests ────────────────────────────────────────────────────────────
  async requestPermission(params: schema.RequestPermissionRequest) {
    const ctx = this.require((params as any).sessionId);
    return permH.handle(ctx, params);
  }
  async readTextFile(params: schema.ReadTextFileRequest) {
    const ctx = this.require((params as any).sessionId);
    return fsH.readTextFile(ctx, params);
  }
  async writeTextFile(params: schema.WriteTextFileRequest) {
    const ctx = this.require((params as any).sessionId);
    return fsH.writeTextFile(ctx, params);
  }
  async createTerminal(params: schema.CreateTerminalRequest) {
    const ctx = this.require((params as any).sessionId);
    return termH.createTerminal(ctx, params);
  }
  async terminalOutput(params: schema.TerminalOutputRequest) {
    const ctx = this.require((params as any).sessionId);
    return termH.terminalOutput(ctx, params);
  }
  async waitForTerminalExit(params: schema.WaitForTerminalExitRequest) {
    const ctx = this.require((params as any).sessionId);
    return termH.waitForExit(ctx, params);
  }
  async killTerminal(params: schema.KillTerminalRequest) {
    const ctx = this.require((params as any).sessionId);
    return termH.kill(ctx, params);
  }
  async releaseTerminal(params: schema.ReleaseTerminalRequest) {
    const ctx = this.require((params as any).sessionId);
    return termH.release(ctx, params);
  }
  async unstable_createElicitation(params: schema.CreateElicitationRequest) {
    const ctx = this.require((params as any).sessionId);
    return elicH.createElicitation(ctx, params);
  }
}
```

Note: handler modules (`handlers/*.ts` and `events/session-update.ts`) are stubs now; later tasks (T4, T5) fill them. Create empty placeholders so imports resolve:

`handlers/fs.ts`:
```ts
import type { schema } from "@agentclientprotocol/sdk";
export async function readTextFile(_ctx: unknown, _p: schema.ReadTextFileRequest): Promise<schema.ReadTextFileResponse> {
  throw new Error("fs.readTextFile not yet implemented (T4a)");
}
export async function writeTextFile(_ctx: unknown, _p: schema.WriteTextFileRequest): Promise<schema.WriteTextFileResponse> {
  throw new Error("fs.writeTextFile not yet implemented (T4a)");
}
```

`handlers/terminal.ts`:
```ts
import type { schema } from "@agentclientprotocol/sdk";
export async function createTerminal(_c: unknown, _p: schema.CreateTerminalRequest): Promise<schema.CreateTerminalResponse> {
  throw new Error("terminal.create not yet implemented (T4b)");
}
export async function terminalOutput(_c: unknown, _p: schema.TerminalOutputRequest): Promise<schema.TerminalOutputResponse> {
  throw new Error("not yet (T4b)");
}
export async function waitForExit(_c: unknown, _p: schema.WaitForTerminalExitRequest): Promise<schema.WaitForTerminalExitResponse> {
  throw new Error("not yet (T4b)");
}
export async function kill(_c: unknown, _p: schema.KillTerminalRequest): Promise<schema.KillTerminalResponse | void> {
  throw new Error("not yet (T4b)");
}
export async function release(_c: unknown, _p: schema.ReleaseTerminalRequest): Promise<schema.ReleaseTerminalResponse | void> {
  throw new Error("not yet (T4b)");
}
```

`handlers/permission.ts`:
```ts
import type { schema } from "@agentclientprotocol/sdk";
export async function handle(_c: unknown, _p: schema.RequestPermissionRequest): Promise<schema.RequestPermissionResponse> {
  throw new Error("permission.handle not yet implemented (T4c)");
}
```

`handlers/elicitation.ts`:
```ts
import type { schema } from "@agentclientprotocol/sdk";
export async function createElicitation(_c: unknown, _p: schema.CreateElicitationRequest): Promise<schema.CreateElicitationResponse> {
  throw new Error("elicitation not yet implemented (T4d)");
}
```

`events/session-update.ts`:
```ts
import type { schema } from "@agentclientprotocol/sdk";
export function mapSessionUpdate(_n: schema.SessionNotification): unknown {
  return { type: "status_change", kind: "acp_passthrough" };
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `cd src-bridge && bun test src/runtime/acp/multiplexed-client.test.ts`
Expected: 4 passes (the `readTextFile` test expects rejection from the stub handler's throw; update the assertion to match `throw.*not yet implemented` if Bun doesn't forward the fs module error):

If first test fails because the stub throws "not yet implemented" instead of "ENOENT", edit the test to:
```ts
await expect(mc.readTextFile!({ sessionId: "s1", path: "foo.ts" } as any))
  .rejects.toThrow(/not yet implemented|ENOENT/i);
```

- [ ] **Step 5: Typecheck**

Run: `cd src-bridge && bun run typecheck`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add src-bridge/src/runtime/acp/multiplexed-client.ts src-bridge/src/runtime/acp/multiplexed-client.test.ts src-bridge/src/runtime/acp/handlers/ src-bridge/src/runtime/acp/events/
git commit -m "feat(acp): add MultiplexedClient with sessionId routing and handler stubs"
```

---

## Task 3c: `AcpSession` + factory wire-up

**Files:**
- Create: `src-bridge/src/runtime/acp/session.ts`
- Modify: `src-bridge/src/runtime/acp/connection-pool.ts` (add real factory)
- Test: `src-bridge/src/runtime/acp/session.test.ts`
- Test: `src-bridge/tests/component/acp/mock-acp-agent.ts` (fixture)
- Test: `src-bridge/tests/component/acp/happy-path.test.ts`

Spec §4.5, §7.1. Per-task wrapper over `(pool, sessionId)`; thin delegating face.

- [ ] **Step 1: Write failing happy-path component test**

Create `src-bridge/tests/component/acp/mock-acp-agent.ts`:

```ts
#!/usr/bin/env node
// Minimal ACP agent stub for component tests. Speaks stable JSON-RPC over stdio.
// Read lines → respond to initialize/newSession/prompt/cancel; emit session/update text delta.

import { ndJsonStream, AgentSideConnection } from "@agentclientprotocol/sdk";

const agent: any = {
  async initialize(req: any) {
    return {
      protocolVersion: req.protocolVersion ?? 1,
      agentCapabilities: {
        loadSession: false,
        availableModes: [],
        availableModels: [],
        promptCapabilities: { thinkingBudget: false },
        mcpCapabilities: {},
      },
      authMethods: [],
    };
  },
  async newSession(_req: any) {
    return { sessionId: "mock-session-1" };
  },
  async prompt(req: any) {
    await conn.sessionUpdate({
      sessionId: req.sessionId,
      update: { sessionUpdate: "agent_message_chunk", content: { type: "text", text: "hello" } },
    });
    return { stopReason: "end_turn" };
  },
  async cancel(_req: any) {
    /* notification */
  },
  async setSessionMode() { return {}; },
  async setSessionConfigOption() { return { options: [] }; },
  async authenticate() { return {}; },
};

const stream = ndJsonStream(process.stdout as any, process.stdin as any);
const conn = new AgentSideConnection(() => agent as any, stream);
await conn.closed;
```

Create `src-bridge/tests/component/acp/happy-path.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { AcpConnectionPool } from "../../../src/runtime/acp/connection-pool.js";
import { AcpSession } from "../../../src/runtime/acp/session.js";
import { createPooledEntryFactory } from "../../../src/runtime/acp/connection-pool-factory.js";
import { MultiplexedClient } from "../../../src/runtime/acp/multiplexed-client.js";

describe("ACP happy path with mock agent", () => {
  test("initialize → newSession → prompt → text delta → stop_reason", async () => {
    const events: any[] = [];
    const mc = new MultiplexedClient({ logger: console as any });
    const factory = createPooledEntryFactory({
      logger: console as any,
      resolveSpawn: () => ({
        command: "bun",
        args: ["tests/component/acp/mock-acp-agent.ts"],
        env: {},
      }),
      clientDispatcher: mc,
    });
    const pool = new AcpConnectionPool({ logger: console as any, factory });

    const session = await AcpSession.open(pool, {
      taskId: "t1",
      adapterId: "claude_code" as any,
      cwd: process.cwd(),
      streamer: { emit: (e: any) => events.push(e) } as any,
      permissionRouter: {} as any,
      fsSandbox: { resolve: (_s: string, p: string) => p } as any,
      terminalManager: {} as any,
      mcpServers: [],
      logger: console as any,
      multiplexedClient: mc,
    });

    const stop = await session.prompt([{ type: "text", text: "hi" } as any]);
    expect(stop).toBe("end_turn");
    expect(events.some((e) => e.type === "output")).toBe(true);
    await session.dispose();
    await pool.shutdownAll();
  }, 15_000);
});
```

- [ ] **Step 2: Run tests — expect FAIL (modules missing)**

Run: `cd src-bridge && bun test tests/component/acp/happy-path.test.ts`
Expected: module `session.js` missing / `connection-pool-factory.js` missing.

- [ ] **Step 3: Implement `session.ts`**

Create `src-bridge/src/runtime/acp/session.ts`:

```ts
import type { ClientSideConnection, schema } from "@agentclientprotocol/sdk";
import type { AcpConnectionPool, PooledEntry } from "./connection-pool.js";
import type { MultiplexedClient, PerSessionContext } from "./multiplexed-client.js";
import type { AdapterId } from "./registry.js";
import { gateUnstable } from "./capabilities.js";
import { AcpConcurrentPrompt } from "./errors.js";
import type { Logger } from "./process-host.js";

export interface AcpSessionOptions {
  taskId: string;
  adapterId: AdapterId;
  cwd: string;
  streamer: { emit(event: unknown): void };
  permissionRouter: PerSessionContext["permissionRouter"];
  fsSandbox: PerSessionContext["fsSandbox"];
  terminalManager: unknown;
  mcpServers: schema.McpServer[];
  logger: Logger;
  multiplexedClient: MultiplexedClient;
}

export class AcpSession {
  private promptInFlight = false;

  static async open(pool: AcpConnectionPool, opts: AcpSessionOptions): Promise<AcpSession> {
    const entry = await pool.acquire(opts.adapterId);
    const res = await entry.conn.newSession({
      cwd: opts.cwd,
      mcpServers: opts.mcpServers,
    });
    entry.sessions.add(res.sessionId);
    opts.multiplexedClient.register(res.sessionId, {
      taskId: opts.taskId,
      cwd: opts.cwd,
      fsSandbox: opts.fsSandbox,
      terminalManager: opts.terminalManager as any,
      permissionRouter: opts.permissionRouter,
      streamer: opts.streamer,
      logger: opts.logger,
    });
    return new AcpSession(pool, entry, res.sessionId, opts);
  }

  private constructor(
    private readonly pool: AcpConnectionPool,
    private readonly entry: PooledEntry,
    readonly sessionId: string,
    private readonly opts: AcpSessionOptions,
  ) {}

  get capabilities(): schema.AgentCapabilities {
    return this.entry.caps;
  }
  private get conn(): ClientSideConnection {
    return this.entry.conn;
  }

  // stable
  async prompt(content: schema.ContentBlock[]): Promise<schema.StopReason> {
    if (this.promptInFlight) throw new AcpConcurrentPrompt("prompt already in flight");
    this.promptInFlight = true;
    try {
      const res = await this.conn.prompt({ sessionId: this.sessionId, prompt: content });
      return res.stopReason;
    } finally {
      this.promptInFlight = false;
    }
  }
  cancel(): Promise<void> {
    return this.conn.cancel({ sessionId: this.sessionId });
  }
  async setMode(modeId: string): Promise<void> {
    await this.conn.setSessionMode({ sessionId: this.sessionId, modeId });
  }
  async setConfigOption(key: string, value: unknown) {
    return this.conn.setSessionConfigOption({ sessionId: this.sessionId, optionId: key, value } as any);
  }
  async authenticate(methodId: string): Promise<void> {
    await this.conn.authenticate({ methodId } as any);
  }

  // unstable (gated)
  async forkSession(): Promise<AcpSession> {
    gateUnstable(this.capabilities, "forkSession", "session.fork");
    const res = await this.conn.unstable_forkSession({ sessionId: this.sessionId } as any);
    // The new session shares the same pool entry; wrap it.
    this.entry.sessions.add(res.sessionId);
    this.opts.multiplexedClient.register(res.sessionId, {
      taskId: this.opts.taskId,
      cwd: this.opts.cwd,
      fsSandbox: this.opts.fsSandbox,
      terminalManager: this.opts.terminalManager as any,
      permissionRouter: this.opts.permissionRouter,
      streamer: this.opts.streamer,
      logger: this.opts.logger,
    });
    return new AcpSession(this.pool, this.entry, res.sessionId, this.opts);
  }
  async resumeSession(): Promise<void> {
    gateUnstable(this.capabilities, "resumeSession", "session.resume");
    await this.conn.unstable_resumeSession({ sessionId: this.sessionId } as any);
  }
  async setModel(modelId: string): Promise<void> {
    gateUnstable(this.capabilities, "setModel", "session.setModel");
    await this.conn.unstable_setSessionModel({ sessionId: this.sessionId, modelId } as any);
  }
  async closeSession(): Promise<void> {
    gateUnstable(this.capabilities, "closeSession", "session.close");
    await this.conn.unstable_closeSession({ sessionId: this.sessionId } as any);
  }
  async logout(): Promise<void> {
    gateUnstable(this.capabilities, "logout", "authentication.logout");
    await this.conn.unstable_logout({} as any);
  }
  async extMethod(method: string, params: Record<string, unknown>): Promise<Record<string, unknown>> {
    return this.conn.extMethod(method, { sessionId: this.sessionId, ...params });
  }
  async extNotification(method: string, params: Record<string, unknown>): Promise<void> {
    await this.conn.extNotification(method, { sessionId: this.sessionId, ...params });
  }

  // lifecycle
  async dispose(): Promise<void> {
    this.opts.multiplexedClient.unregister(this.sessionId);
    await this.pool.release(this.opts.adapterId, this.sessionId);
  }
}
```

- [ ] **Step 4: Implement `connection-pool-factory.ts`**

Create `src-bridge/src/runtime/acp/connection-pool-factory.ts`:

```ts
import { ClientSideConnection, ndJsonStream } from "@agentclientprotocol/sdk";
import { ChildProcessHost, type Logger } from "./process-host.js";
import type { PooledEntry, PooledEntryFactory } from "./connection-pool.js";
import type { MultiplexedClient } from "./multiplexed-client.js";
import type { AdapterId } from "./registry.js";
import { ACP_ADAPTERS } from "./registry.js";
import { AcpAuthMissing } from "./errors.js";

export interface PooledEntryFactoryOpts {
  logger: Logger;
  clientDispatcher: MultiplexedClient;
  resolveSpawn?: (adapterId: AdapterId) => {
    command: string;
    args: readonly string[];
    env: Record<string, string>;
  };
  resolveEnv?: (adapterId: AdapterId) => Record<string, string>;
}

export function createPooledEntryFactory(opts: PooledEntryFactoryOpts): PooledEntryFactory {
  return async (adapterId: AdapterId): Promise<PooledEntry> => {
    const spec = ACP_ADAPTERS[adapterId];
    const spawnSpec = opts.resolveSpawn?.(adapterId) ?? {
      command: spec.command,
      args: spec.args,
      env: opts.resolveEnv?.(adapterId) ?? {},
    };
    const missing = spec.envRequired.filter((k) => !(k in spawnSpec.env) && !(k in process.env));
    if (missing.length > 0) throw new AcpAuthMissing(adapterId, missing);

    const host = new ChildProcessHost({
      adapterId,
      command: spawnSpec.command,
      args: spawnSpec.args,
      env: spawnSpec.env,
      logger: opts.logger,
    });
    const io = await host.start();
    const stream = ndJsonStream(io.stdin, io.stdout);
    const conn = new ClientSideConnection(() => opts.clientDispatcher, stream);
    const initRes = await conn.initialize({
      protocolVersion: 1,
      clientCapabilities: {
        fs: { readTextFile: true, writeTextFile: true },
        terminal: true,
      },
    } as any);
    return {
      host,
      conn,
      caps: initRes.agentCapabilities,
      clientDispatcher: opts.clientDispatcher as any,
      sessions: new Set<string>(),
      restartPending: false,
    };
  };
}
```

- [ ] **Step 5: Run component test — expect PASS**

Run: `cd src-bridge && bun test tests/component/acp/happy-path.test.ts`
Expected: 1 pass.

- [ ] **Step 6: Commit**

```bash
git add src-bridge/src/runtime/acp/session.ts src-bridge/src/runtime/acp/connection-pool-factory.ts src-bridge/src/runtime/acp/session.test.ts src-bridge/tests/component/acp/
git commit -m "feat(acp): add AcpSession + pooled entry factory wiring with happy-path component test"
```

---

## Task 3d: Pooling + cancel-race component tests

**Files:**
- Test: `src-bridge/tests/component/acp/pooling.test.ts`
- Test: `src-bridge/tests/component/acp/cancel-race.test.ts`
- Test: `src-bridge/tests/component/acp/multi-session-fs.test.ts`

Spec §10 tier 2. All use the mock-acp-agent fixture from T3c.

- [ ] **Step 1: Extend the mock agent to support multiple sessions + cancellable prompts + fs**

Edit `src-bridge/tests/component/acp/mock-acp-agent.ts`, add per-session state and cancel awareness:

```ts
// (in agent object)
const sessions = new Map<string, { cancelled: boolean; counter: number }>();
let sessionCounter = 0;

async newSession(_req: any) {
  const id = `mock-session-${++sessionCounter}`;
  sessions.set(id, { cancelled: false, counter: 0 });
  return { sessionId: id };
},
async prompt(req: any) {
  const st = sessions.get(req.sessionId)!;
  st.cancelled = false;
  for (let i = 0; i < 5; i++) {
    if (st.cancelled) return { stopReason: "cancelled" };
    await conn.sessionUpdate({
      sessionId: req.sessionId,
      update: { sessionUpdate: "agent_message_chunk", content: { type: "text", text: `chunk${i}` } },
    });
    await new Promise((r) => setTimeout(r, 50));
  }
  return { stopReason: "end_turn" };
},
async cancel(req: any) {
  const st = sessions.get(req.sessionId);
  if (st) st.cancelled = true;
},
async readTextFile(req: any) {
  // Handled by client — mock wouldn't call this; keep as no-op if called.
  throw new Error("agent doesn't read");
},
```

(Prompt needs to request fs/terminal from the client side to test routing. Add a mode:)

```ts
async prompt(req: any) {
  const firstBlock = req.prompt?.[0] as any;
  const marker = firstBlock?.type === "text" ? firstBlock.text : "";
  if (marker === "FS_READ") {
    const res = await (conn as any).readTextFile({ sessionId: req.sessionId, path: "mock.txt" });
    await conn.sessionUpdate({
      sessionId: req.sessionId,
      update: { sessionUpdate: "agent_message_chunk", content: { type: "text", text: `got:${res.content}` } },
    });
    return { stopReason: "end_turn" };
  }
  // (... existing iterations loop)
},
```

- [ ] **Step 2: Write pooling test**

Create `src-bridge/tests/component/acp/pooling.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { AcpConnectionPool } from "../../../src/runtime/acp/connection-pool.js";
import { AcpSession } from "../../../src/runtime/acp/session.js";
import { createPooledEntryFactory } from "../../../src/runtime/acp/connection-pool-factory.js";
import { MultiplexedClient } from "../../../src/runtime/acp/multiplexed-client.js";

describe("pooling", () => {
  test("two tasks share one host; different sessionIds", async () => {
    const mc = new MultiplexedClient({ logger: console as any });
    const factory = createPooledEntryFactory({
      logger: console as any,
      resolveSpawn: () => ({
        command: "bun",
        args: ["tests/component/acp/mock-acp-agent.ts"],
        env: {},
      }),
      clientDispatcher: mc,
    });
    const pool = new AcpConnectionPool({ logger: console as any, factory });

    const mkSession = async (tid: string) =>
      AcpSession.open(pool, {
        taskId: tid,
        adapterId: "claude_code" as any,
        cwd: process.cwd(),
        streamer: { emit: () => {} } as any,
        permissionRouter: {} as any,
        fsSandbox: { resolve: (_s: string, p: string) => p } as any,
        terminalManager: {} as any,
        mcpServers: [],
        logger: console as any,
        multiplexedClient: mc,
      });

    const s1 = await mkSession("t1");
    const s2 = await mkSession("t2");
    expect(s1.sessionId).not.toBe(s2.sessionId);
    await s1.dispose();
    await s2.dispose();
    await pool.shutdownAll();
  }, 15_000);
});
```

- [ ] **Step 3: Write cancel-race test**

Create `src-bridge/tests/component/acp/cancel-race.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { AcpConnectionPool } from "../../../src/runtime/acp/connection-pool.js";
import { AcpSession } from "../../../src/runtime/acp/session.js";
import { createPooledEntryFactory } from "../../../src/runtime/acp/connection-pool-factory.js";
import { MultiplexedClient } from "../../../src/runtime/acp/multiplexed-client.js";

describe("cancel race", () => {
  test("cancel mid-prompt → stopReason=cancelled", async () => {
    const mc = new MultiplexedClient({ logger: console as any });
    const pool = new AcpConnectionPool({
      logger: console as any,
      factory: createPooledEntryFactory({
        logger: console as any,
        resolveSpawn: () => ({
          command: "bun",
          args: ["tests/component/acp/mock-acp-agent.ts"],
          env: {},
        }),
        clientDispatcher: mc,
      }),
    });
    const session = await AcpSession.open(pool, {
      taskId: "t1",
      adapterId: "claude_code" as any,
      cwd: process.cwd(),
      streamer: { emit: () => {} } as any,
      permissionRouter: {} as any,
      fsSandbox: { resolve: (_s: string, p: string) => p } as any,
      terminalManager: {} as any,
      mcpServers: [],
      logger: console as any,
      multiplexedClient: mc,
    });
    const promptP = session.prompt([{ type: "text", text: "hi" } as any]);
    await new Promise((r) => setTimeout(r, 80));
    await session.cancel();
    const stop = await promptP;
    expect(stop).toBe("cancelled");
    await session.dispose();
    await pool.shutdownAll();
  }, 15_000);
});
```

- [ ] **Step 4: Run the two tests — expect PASS**

Run: `cd src-bridge && bun test tests/component/acp/pooling.test.ts tests/component/acp/cancel-race.test.ts`
Expected: 2 passes.

- [ ] **Step 5: Commit**

```bash
git add src-bridge/tests/component/acp/pooling.test.ts src-bridge/tests/component/acp/cancel-race.test.ts src-bridge/tests/component/acp/mock-acp-agent.ts
git commit -m "test(acp): component tests for pooling and cancel race"
```

---

## Task 4a: Fs handler + sandbox

**Files:**
- Modify: `src-bridge/src/runtime/acp/handlers/fs.ts` (replace stub)
- Create: `src-bridge/src/runtime/acp/fs-sandbox.ts` (if not already in bridge codebase)
- Test: `src-bridge/src/runtime/acp/fs-sandbox.test.ts`
- Test: `src-bridge/src/runtime/acp/handlers/fs.test.ts`

Spec §6.1. Worktree-rooted path resolution with realpath escape rejection; 1-based line/limit slicing.

- [ ] **Step 1: Write failing sandbox test**

Check whether `FsSandbox` already exists in the bridge codebase:

Run: `rg -l "class FsSandbox|FsSandbox[^=]*=" src-bridge/src`
If found, reuse and wire in. If not, create new:

Create `src-bridge/src/runtime/acp/fs-sandbox.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { FsSandbox } from "./fs-sandbox.js";
import { mkdtempSync, symlinkSync, writeFileSync, realpathSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

describe("FsSandbox", () => {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "sandbox-")));
  const outsideRoot = realpathSync(mkdtempSync(join(tmpdir(), "outside-")));
  writeFileSync(join(root, "in.txt"), "content");
  writeFileSync(join(outsideRoot, "secret.txt"), "secret");

  test("resolves file inside worktree", () => {
    const sb = new FsSandbox(root);
    expect(sb.resolve("s1", "in.txt")).toBe(join(root, "in.txt"));
  });

  test("rejects absolute path outside worktree", () => {
    const sb = new FsSandbox(root);
    expect(() => sb.resolve("s1", join(outsideRoot, "secret.txt"))).toThrow(
      /path_escapes_worktree/,
    );
  });

  test("rejects ../ traversal", () => {
    const sb = new FsSandbox(root);
    expect(() => sb.resolve("s1", "../secret.txt")).toThrow(/path_escapes_worktree/);
  });

  test("rejects symlink that resolves outside worktree", () => {
    symlinkSync(outsideRoot, join(root, "link-out"));
    const sb = new FsSandbox(root);
    expect(() => sb.resolve("s1", "link-out/secret.txt")).toThrow(/path_escapes_worktree/);
  });
});
```

- [ ] **Step 2: Implement `FsSandbox`**

Create `src-bridge/src/runtime/acp/fs-sandbox.ts`:

```ts
import { realpathSync, existsSync } from "node:fs";
import { isAbsolute, resolve as resolvePath, dirname } from "node:path";
import { RequestError } from "@agentclientprotocol/sdk";

export class FsSandbox {
  constructor(private readonly worktreeRoot: string) {}

  resolve(_sessionId: string, requested: string): string {
    const abs = isAbsolute(requested)
      ? requested
      : resolvePath(this.worktreeRoot, requested);
    // realpath on existing ancestor to follow symlinks
    let check = abs;
    while (!existsSync(check) && dirname(check) !== check) {
      check = dirname(check);
    }
    const real = existsSync(check) ? realpathSync(check) : check;
    const rootReal = realpathSync(this.worktreeRoot);
    if (!real.startsWith(rootReal)) {
      throw new RequestError(-32602, "path_escapes_worktree", {
        requested,
        resolved: real,
      });
    }
    return abs;
  }
}
```

- [ ] **Step 3: Run sandbox test — expect PASS**

Run: `cd src-bridge && bun test src/runtime/acp/fs-sandbox.test.ts`
Expected: 4 passes.

- [ ] **Step 4: Write fs handler test**

Create `src-bridge/src/runtime/acp/handlers/fs.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { readTextFile, writeTextFile } from "./fs.js";
import { FsSandbox } from "../fs-sandbox.js";
import { mkdtempSync, writeFileSync, readFileSync, realpathSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

function mkCtx(root: string) {
  return {
    taskId: "t1",
    cwd: root,
    fsSandbox: new FsSandbox(root),
    terminalManager: {} as any,
    permissionRouter: {} as any,
    streamer: { emit: () => {} } as any,
    logger: console as any,
  };
}

describe("fs handler", () => {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "fs-h-")));
  writeFileSync(join(root, "a.txt"), "line1\nline2\nline3\n");

  test("readTextFile returns entire UTF-8 content", async () => {
    const ctx = mkCtx(root);
    const res = await readTextFile(ctx, { sessionId: "s1", path: "a.txt" } as any);
    expect(res.content).toBe("line1\nline2\nline3\n");
  });

  test("readTextFile honors 1-based line+limit slice", async () => {
    const ctx = mkCtx(root);
    const res = await readTextFile(ctx, { sessionId: "s1", path: "a.txt", line: 2, limit: 1 } as any);
    expect(res.content).toBe("line2\n");
  });

  test("writeTextFile creates file inside worktree", async () => {
    const ctx = mkCtx(root);
    await writeTextFile(ctx, { sessionId: "s1", path: "b.txt", content: "hello" } as any);
    expect(readFileSync(join(root, "b.txt"), "utf8")).toBe("hello");
  });
});
```

- [ ] **Step 5: Implement handler**

Replace `src-bridge/src/runtime/acp/handlers/fs.ts`:

```ts
import { readFile, writeFile } from "node:fs/promises";
import type { schema } from "@agentclientprotocol/sdk";
import type { PerSessionContext } from "../multiplexed-client.js";

export async function readTextFile(
  ctx: PerSessionContext,
  params: schema.ReadTextFileRequest,
): Promise<schema.ReadTextFileResponse> {
  const abs = ctx.fsSandbox.resolve((params as any).sessionId, params.path);
  const full = await readFile(abs, "utf8");
  const line = (params as any).line as number | undefined;
  const limit = (params as any).limit as number | undefined;
  if (line == null && limit == null) return { content: full };
  const lines = full.split("\n");
  const start = Math.max(0, (line ?? 1) - 1);
  const end = limit != null ? start + limit : lines.length;
  const slice = lines.slice(start, end).join("\n");
  const trailing = end < lines.length ? "\n" : "";
  return { content: slice + trailing };
}

export async function writeTextFile(
  ctx: PerSessionContext,
  params: schema.WriteTextFileRequest,
): Promise<schema.WriteTextFileResponse> {
  const abs = ctx.fsSandbox.resolve((params as any).sessionId, params.path);
  await writeFile(abs, params.content, "utf8");
  return {} as schema.WriteTextFileResponse;
}
```

- [ ] **Step 6: Run tests — expect PASS**

Run: `cd src-bridge && bun test src/runtime/acp/handlers/fs.test.ts src/runtime/acp/fs-sandbox.test.ts`
Expected: 7 passes.

- [ ] **Step 7: Commit**

```bash
git add src-bridge/src/runtime/acp/fs-sandbox.ts src-bridge/src/runtime/acp/fs-sandbox.test.ts src-bridge/src/runtime/acp/handlers/fs.ts src-bridge/src/runtime/acp/handlers/fs.test.ts
git commit -m "feat(acp): implement fs handler with FsSandbox (path escape rejection)"
```

---

## Task 4b: Terminal handler

**Files:**
- Modify: `src-bridge/src/runtime/acp/handlers/terminal.ts`
- Create: `src-bridge/src/runtime/acp/terminal-manager.ts`
- Test: `src-bridge/src/runtime/acp/handlers/terminal.test.ts`

Spec §6.2. Reuses existing TerminalManager if present; otherwise thin pty pool wrapper. 10MB per-task, 16 global cap.

- [ ] **Step 1: Check for existing TerminalManager in bridge**

Run: `rg -l "TerminalManager|pty\.spawn|node-pty" src-bridge/src`
If an existing module like `src-bridge/src/runtime/terminal-manager.ts` exists, reuse; otherwise create.

- [ ] **Step 2: Write failing test**

Create `src-bridge/src/runtime/acp/handlers/terminal.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { createTerminal, terminalOutput, waitForExit, kill, release } from "./terminal.js";
import { TerminalManager } from "../terminal-manager.js";

function mkCtx() {
  return {
    taskId: "t1",
    cwd: process.cwd(),
    fsSandbox: {} as any,
    terminalManager: new TerminalManager({ perTaskByteLimit: 1024, maxConcurrent: 2 }),
    permissionRouter: {} as any,
    streamer: { emit: () => {} } as any,
    logger: console as any,
  };
}

describe("terminal handler", () => {
  test("create → output → wait → release", async () => {
    const ctx = mkCtx();
    const cr = await createTerminal(ctx, { sessionId: "s1", command: "node", args: ["-e", "console.log('pong')"] } as any);
    expect(cr.terminalId).toMatch(/.+/);
    const exit = await waitForExit(ctx, { sessionId: "s1", terminalId: cr.terminalId } as any);
    expect(exit.exitCode).toBe(0);
    const out = await terminalOutput(ctx, { sessionId: "s1", terminalId: cr.terminalId } as any);
    expect(out.output).toContain("pong");
    await release(ctx, { sessionId: "s1", terminalId: cr.terminalId } as any);
  }, 10_000);

  test("rejects when maxConcurrent exceeded", async () => {
    const ctx = mkCtx();
    const c1 = await createTerminal(ctx, { sessionId: "s1", command: "node", args: ["-e", "setInterval(()=>{},1000)"] } as any);
    const c2 = await createTerminal(ctx, { sessionId: "s1", command: "node", args: ["-e", "setInterval(()=>{},1000)"] } as any);
    await expect(
      createTerminal(ctx, { sessionId: "s1", command: "node", args: ["-e", "1"] } as any),
    ).rejects.toMatchObject({ code: -32000 });
    await kill(ctx, { sessionId: "s1", terminalId: c1.terminalId } as any);
    await kill(ctx, { sessionId: "s1", terminalId: c2.terminalId } as any);
    await release(ctx, { sessionId: "s1", terminalId: c1.terminalId } as any);
    await release(ctx, { sessionId: "s1", terminalId: c2.terminalId } as any);
  }, 10_000);
});
```

- [ ] **Step 3: Implement `TerminalManager`**

Create `src-bridge/src/runtime/acp/terminal-manager.ts`:

```ts
import { RequestError } from "@agentclientprotocol/sdk";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { randomUUID } from "node:crypto";

export interface TerminalManagerOpts {
  perTaskByteLimit?: number;
  maxConcurrent?: number;
}

interface TerminalEntry {
  proc: ChildProcessWithoutNullStreams;
  buffer: string[];
  bytes: number;
  exited: Promise<{ exitCode: number; signal?: string }>;
  truncated: boolean;
}

export class TerminalManager {
  private terminals = new Map<string, TerminalEntry>();
  private readonly byteLimit: number;
  private readonly maxConcurrent: number;

  constructor(opts: TerminalManagerOpts = {}) {
    this.byteLimit = opts.perTaskByteLimit ?? 10 * 1024 * 1024;
    this.maxConcurrent = opts.maxConcurrent ?? 16;
  }

  create(spec: { command: string; args?: string[]; cwd?: string; env?: Record<string, string> }): string {
    if (this.terminals.size >= this.maxConcurrent) {
      throw new RequestError(-32000, "terminal_capacity", { max: this.maxConcurrent });
    }
    const proc = spawn(spec.command, spec.args ?? [], {
      cwd: spec.cwd,
      env: { ...process.env, ...(spec.env ?? {}) },
      stdio: ["pipe", "pipe", "pipe"],
    });
    const entry: TerminalEntry = {
      proc,
      buffer: [],
      bytes: 0,
      truncated: false,
      exited: new Promise((r) =>
        proc.on("exit", (code, sig) => r({ exitCode: code ?? -1, signal: sig ?? undefined })),
      ),
    };
    const sink = (chunk: Buffer) => {
      const s = chunk.toString("utf8");
      entry.buffer.push(s);
      entry.bytes += s.length;
      while (entry.bytes > this.byteLimit && entry.buffer.length > 1) {
        entry.bytes -= entry.buffer.shift()!.length;
        entry.truncated = true;
      }
    };
    proc.stdout.on("data", sink);
    proc.stderr.on("data", sink);
    const id = randomUUID();
    this.terminals.set(id, entry);
    return id;
  }

  getOutput(id: string): { output: string; truncated: boolean; exitStatus?: { exitCode: number; signal?: string } } {
    const e = this.mustGet(id);
    const exitStatus = e.proc.exitCode != null ? { exitCode: e.proc.exitCode, signal: e.proc.signalCode ?? undefined } : undefined;
    return { output: e.buffer.join(""), truncated: e.truncated, exitStatus };
  }

  async waitForExit(id: string): Promise<{ exitCode: number; signal?: string }> {
    const e = this.mustGet(id);
    return e.exited;
  }

  kill(id: string): void {
    const e = this.mustGet(id);
    if (e.proc.exitCode == null) e.proc.kill("SIGKILL");
  }

  release(id: string): void {
    const e = this.terminals.get(id);
    if (!e) return;
    if (e.proc.exitCode == null) e.proc.kill("SIGKILL");
    this.terminals.delete(id);
  }

  private mustGet(id: string): TerminalEntry {
    const e = this.terminals.get(id);
    if (!e) throw new RequestError(-32602, "unknown_terminal", { terminalId: id });
    return e;
  }
}
```

- [ ] **Step 4: Implement handler**

Replace `src-bridge/src/runtime/acp/handlers/terminal.ts`:

```ts
import type { schema } from "@agentclientprotocol/sdk";
import type { PerSessionContext } from "../multiplexed-client.js";
import type { TerminalManager } from "../terminal-manager.js";

function tm(ctx: PerSessionContext): TerminalManager {
  return ctx.terminalManager as unknown as TerminalManager;
}

export async function createTerminal(
  ctx: PerSessionContext,
  p: schema.CreateTerminalRequest,
): Promise<schema.CreateTerminalResponse> {
  const id = tm(ctx).create({
    command: p.command,
    args: (p as any).args,
    cwd: (p as any).cwd ?? ctx.cwd,
    env: (p as any).env,
  });
  return { terminalId: id };
}

export async function terminalOutput(
  ctx: PerSessionContext,
  p: schema.TerminalOutputRequest,
): Promise<schema.TerminalOutputResponse> {
  const o = tm(ctx).getOutput(p.terminalId);
  return { output: o.output, truncated: o.truncated, exitStatus: o.exitStatus as any };
}

export async function waitForExit(
  ctx: PerSessionContext,
  p: schema.WaitForTerminalExitRequest,
): Promise<schema.WaitForTerminalExitResponse> {
  const x = await tm(ctx).waitForExit(p.terminalId);
  return { exitCode: x.exitCode, signal: x.signal } as any;
}

export async function kill(
  ctx: PerSessionContext,
  p: schema.KillTerminalRequest,
): Promise<schema.KillTerminalResponse | void> {
  tm(ctx).kill(p.terminalId);
}

export async function release(
  ctx: PerSessionContext,
  p: schema.ReleaseTerminalRequest,
): Promise<schema.ReleaseTerminalResponse | void> {
  tm(ctx).release(p.terminalId);
}
```

- [ ] **Step 5: Run tests — expect PASS**

Run: `cd src-bridge && bun test src/runtime/acp/handlers/terminal.test.ts`
Expected: 2 passes.

- [ ] **Step 6: Commit**

```bash
git add src-bridge/src/runtime/acp/terminal-manager.ts src-bridge/src/runtime/acp/handlers/terminal.ts src-bridge/src/runtime/acp/handlers/terminal.test.ts
git commit -m "feat(acp): implement terminal handler backed by TerminalManager"
```

---

## Task 4c: Permission handler

**Files:**
- Modify: `src-bridge/src/runtime/acp/handlers/permission.ts`
- Test: `src-bridge/src/runtime/acp/handlers/permission.test.ts`

Spec §6.3. Generates request_id, emits event, waits via `HookCallbackManager`. 30 min timeout.

- [ ] **Step 1: Write failing test**

Create `src-bridge/src/runtime/acp/handlers/permission.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { handle } from "./permission.js";
import { HookCallbackManager } from "../../hook-callback-manager.js";

function mkCtx(mgr: HookCallbackManager, emitted: any[]) {
  return {
    taskId: "t1",
    cwd: "/tmp",
    fsSandbox: {} as any,
    terminalManager: {} as any,
    permissionRouter: {
      async request(taskId, toolCall, options) {
        const { id } = mgr.register({ taskId, kind: "permission" });
        emitted.push({ request_id: id, tool_call: toolCall, options });
        return mgr.waitFor(id, 30 * 60_000) as any;
      },
    },
    streamer: { emit: (ev: any) => emitted.push(ev) } as any,
    logger: console as any,
  };
}

describe("permission handler", () => {
  test("returns selected when resolved", async () => {
    const mgr = new HookCallbackManager({});
    const emitted: any[] = [];
    const ctx = mkCtx(mgr, emitted);
    const p = handle(ctx as any, {
      sessionId: "s1",
      toolCall: { name: "Write" } as any,
      options: [{ id: "allow", label: "Allow", kind: "allow_once" }] as any,
    } as any);
    // resolve
    const registered = emitted[0];
    mgr.resolve(registered.request_id, { outcome: "selected", optionId: "allow" });
    const res = await p;
    expect((res as any).outcome).toBe("selected");
  });

  test("returns cancelled on timeout (fast clock)", async () => {
    const mgr = new HookCallbackManager({ defaultTimeoutMs: 10 });
    const emitted: any[] = [];
    const ctx = mkCtx(mgr, emitted);
    ctx.permissionRouter.request = async (taskId, toolCall, options) => {
      const { id } = mgr.register({ taskId, kind: "permission" });
      emitted.push({ request_id: id });
      return mgr.waitFor(id, 10) as any;
    };
    const res = await handle(ctx as any, {
      sessionId: "s1",
      toolCall: {} as any,
      options: [],
    } as any);
    expect((res as any).outcome).toBe("cancelled");
  });
});
```

- [ ] **Step 2: Implement handler**

Replace `src-bridge/src/runtime/acp/handlers/permission.ts`:

```ts
import type { schema } from "@agentclientprotocol/sdk";
import type { PerSessionContext } from "../multiplexed-client.js";

export async function handle(
  ctx: PerSessionContext,
  params: schema.RequestPermissionRequest,
): Promise<schema.RequestPermissionResponse> {
  const outcome = await ctx.permissionRouter.request(
    ctx.taskId,
    (params as any).toolCall,
    (params as any).options ?? [],
  );
  return { outcome } as schema.RequestPermissionResponse;
}
```

- [ ] **Step 3: Run test — expect PASS**

Run: `cd src-bridge && bun test src/runtime/acp/handlers/permission.test.ts`
Expected: 2 passes.

- [ ] **Step 4: Commit**

```bash
git add src-bridge/src/runtime/acp/handlers/permission.ts src-bridge/src/runtime/acp/handlers/permission.test.ts
git commit -m "feat(acp): wire permission handler to HookCallbackManager"
```

---

## Task 4d: Elicitation handler

**Files:**
- Modify: `src-bridge/src/runtime/acp/handlers/elicitation.ts`
- Test: `src-bridge/src/runtime/acp/handlers/elicitation.test.ts`

Spec §6.4. Capability-gated passthrough; no frontend UI. Emits `elicitation_request` event and returns `{action:"cancel"}` when no router path available.

- [ ] **Step 1: Implement handler**

Replace `src-bridge/src/runtime/acp/handlers/elicitation.ts`:

```ts
import type { schema } from "@agentclientprotocol/sdk";
import type { PerSessionContext } from "../multiplexed-client.js";

export async function createElicitation(
  ctx: PerSessionContext,
  params: schema.CreateElicitationRequest,
): Promise<schema.CreateElicitationResponse> {
  ctx.streamer.emit({
    type: "elicitation_request",
    session_id: (params as any).sessionId,
    payload: params,
  });
  // No FE this phase — default to cancel per schema.
  return { action: "cancel" } as schema.CreateElicitationResponse;
}
```

- [ ] **Step 2: Write test**

Create `src-bridge/src/runtime/acp/handlers/elicitation.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { createElicitation } from "./elicitation.js";

describe("elicitation handler", () => {
  test("emits event and returns cancel by default", async () => {
    const emitted: any[] = [];
    const ctx = {
      streamer: { emit: (e: any) => emitted.push(e) },
    } as any;
    const res = await createElicitation(ctx, { sessionId: "s1" } as any);
    expect((res as any).action).toBe("cancel");
    expect(emitted[0].type).toBe("elicitation_request");
  });
});
```

- [ ] **Step 3: Run test — expect PASS**

Run: `cd src-bridge && bun test src/runtime/acp/handlers/elicitation.test.ts`
Expected: 1 pass.

- [ ] **Step 4: Commit**

```bash
git add src-bridge/src/runtime/acp/handlers/elicitation.ts src-bridge/src/runtime/acp/handlers/elicitation.test.ts
git commit -m "feat(acp): elicitation passthrough returning cancel by default"
```

---

## Task 5: `session/update` → `AgentEventType` mapping

**Files:**
- Modify: `src-bridge/src/runtime/acp/events/session-update.ts`
- Test: `src-bridge/src/runtime/acp/events/session-update.test.ts`

Spec §8. All stable variants + unstable passthrough + unknown fallback.

- [ ] **Step 1: Write comprehensive mapping test**

Create `src-bridge/src/runtime/acp/events/session-update.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { mapSessionUpdate } from "./session-update.js";

const base = (update: any) => ({ sessionId: "s1", update } as any);

describe("mapSessionUpdate", () => {
  test("agent_message_chunk → output", () => {
    const ev = mapSessionUpdate(
      base({ sessionUpdate: "agent_message_chunk", content: { type: "text", text: "hi" } }),
    ) as any;
    expect(ev.type).toBe("output");
    expect(ev.text).toBe("hi");
  });

  test("agent_thought_chunk → reasoning", () => {
    expect(
      (mapSessionUpdate(base({ sessionUpdate: "agent_thought_chunk", content: { type: "text", text: "think" } })) as any).type,
    ).toBe("reasoning");
  });

  test("user_message_chunk → partial_message direction user", () => {
    const ev = mapSessionUpdate(
      base({ sessionUpdate: "user_message_chunk", content: { type: "text", text: "hi" } }),
    ) as any;
    expect(ev.type).toBe("partial_message");
    expect(ev.direction).toBe("user");
  });

  test("tool_call → tool_call", () => {
    expect(
      (mapSessionUpdate(base({ sessionUpdate: "tool_call", toolCallId: "1", title: "Write" })) as any).type,
    ).toBe("tool_call");
  });

  test("tool_call_update completed → tool_result", () => {
    expect(
      (mapSessionUpdate(base({ sessionUpdate: "tool_call_update", status: "completed", toolCallId: "1" })) as any).type,
    ).toBe("tool_result");
  });

  test("tool_call_update in_progress → tool.status_change", () => {
    expect(
      (mapSessionUpdate(base({ sessionUpdate: "tool_call_update", status: "in_progress", toolCallId: "1" })) as any).type,
    ).toBe("tool.status_change");
  });

  test("plan → todo_update", () => {
    expect(
      (mapSessionUpdate(base({ sessionUpdate: "plan", entries: [] })) as any).type,
    ).toBe("todo_update");
  });

  test("current_mode_update → status_change kind=mode", () => {
    const ev = mapSessionUpdate(base({ sessionUpdate: "current_mode_update", currentModeId: "ask" })) as any;
    expect(ev.type).toBe("status_change");
    expect(ev.kind).toBe("mode");
  });

  test("unknown sessionUpdate → acp_passthrough with _raw", () => {
    const ev = mapSessionUpdate(base({ sessionUpdate: "future_variant_xyz", payload: "abc" })) as any;
    expect(ev.type).toBe("status_change");
    expect(ev.kind).toBe("acp_passthrough");
    expect(ev.metadata._raw.sessionUpdate).toBe("future_variant_xyz");
  });

  test("_meta copied verbatim to metadata._meta", () => {
    const ev = mapSessionUpdate({
      sessionId: "s1",
      update: { sessionUpdate: "agent_message_chunk", content: { type: "text", text: "hi" }, _meta: { usage: { inputTokens: 10 } } },
    } as any) as any;
    expect(ev.metadata?._meta?.usage?.inputTokens).toBe(10);
  });
});
```

- [ ] **Step 2: Implement mapping**

Replace `src-bridge/src/runtime/acp/events/session-update.ts`:

```ts
import type { schema } from "@agentclientprotocol/sdk";

export interface MappedEvent {
  type: string;
  session_id: string;
  metadata?: { _meta?: unknown; _raw?: unknown };
  [k: string]: unknown;
}

export function mapSessionUpdate(n: schema.SessionNotification): MappedEvent {
  const sessionId = (n as any).sessionId as string;
  const upd = (n as any).update as any;
  const meta = upd?._meta;
  const metaBag = meta !== undefined ? { _meta: meta } : {};

  switch (upd?.sessionUpdate) {
    case "agent_message_chunk":
      return { type: "output", session_id: sessionId, text: upd.content?.text ?? "", metadata: metaBag };
    case "agent_thought_chunk":
      return { type: "reasoning", session_id: sessionId, text: upd.content?.text ?? "", metadata: metaBag };
    case "user_message_chunk":
      return { type: "partial_message", session_id: sessionId, direction: "user", text: upd.content?.text ?? "", metadata: metaBag };
    case "tool_call":
      return { type: "tool_call", session_id: sessionId, tool_call_id: upd.toolCallId, title: upd.title, raw: upd, metadata: metaBag };
    case "tool_call_update": {
      const terminal = upd.status === "completed" || upd.status === "failed";
      return {
        type: terminal ? "tool_result" : "tool.status_change",
        session_id: sessionId,
        tool_call_id: upd.toolCallId,
        status: upd.status,
        content: upd.content,
        metadata: metaBag,
      };
    }
    case "plan":
      return { type: "todo_update", session_id: sessionId, entries: upd.entries, metadata: metaBag };
    case "available_commands_update":
      return { type: "status_change", session_id: sessionId, kind: "commands", value: upd.commands, metadata: metaBag };
    case "current_mode_update":
      return { type: "status_change", session_id: sessionId, kind: "mode", value: upd.currentModeId, metadata: metaBag };
    case "config_option_update":
      return { type: "status_change", session_id: sessionId, kind: "config_option", value: upd.option, metadata: metaBag };
    // unstable passthrough buckets
    case "nes_suggestion":
    case "nes_closed":
      return { type: "status_change", session_id: sessionId, kind: "nes", subtype: upd.sessionUpdate, value: upd, metadata: metaBag };
    case "document_opened":
    case "document_changed":
    case "document_closed":
    case "document_saved":
    case "document_focused":
      return { type: "status_change", session_id: sessionId, kind: "document", subtype: upd.sessionUpdate, value: upd, metadata: metaBag };
    default:
      return {
        type: "status_change",
        session_id: sessionId,
        kind: "acp_passthrough",
        metadata: { ...metaBag, _raw: upd },
      };
  }
}
```

- [ ] **Step 3: Run tests — expect PASS**

Run: `cd src-bridge && bun test src/runtime/acp/events/session-update.test.ts`
Expected: 10 passes.

- [ ] **Step 4: Commit**

```bash
git add src-bridge/src/runtime/acp/events/session-update.ts src-bridge/src/runtime/acp/events/session-update.test.ts
git commit -m "feat(acp): session/update → AgentEventType mapping (stable + unstable + passthrough)"
```

---

## Task 6a: `adapter-factory.ts`

**Files:**
- Create: `src-bridge/src/runtime/acp/adapter-factory.ts`
- Create: `src-bridge/src/runtime/acp/index.ts`

Spec §4.6. Single factory builds an `AcpSession` per task and maps it to the `RuntimeAdapter` face.

- [ ] **Step 1: Implement `adapter-factory.ts`**

Create `src-bridge/src/runtime/acp/adapter-factory.ts`:

```ts
import { AcpSession } from "./session.js";
import type { AcpConnectionPool } from "./connection-pool.js";
import type { MultiplexedClient } from "./multiplexed-client.js";
import type { AdapterId } from "./registry.js";
import { liveControlsFor } from "./capabilities.js";
import { AcpCapabilityUnsupported } from "./errors.js";
import type { Logger } from "./process-host.js";

export interface AcpDeps {
  pool: AcpConnectionPool;
  multiplexedClient: MultiplexedClient;
  makeFsSandbox(worktreeRoot: string): import("./multiplexed-client.js").PerSessionContext["fsSandbox"];
  terminalManager: unknown;
  permissionRouter: import("./multiplexed-client.js").PerSessionContext["permissionRouter"];
  resolveMcpServersFor(task: { id: string }): Parameters<typeof AcpSession.open>[1]["mcpServers"];
  worktreeService: { revert(args: unknown): Promise<unknown>; diff(args: unknown): Promise<unknown> };
  taskEventsService: { messages(taskId: string): Promise<unknown> };
  mcpServersStatus(task: unknown): unknown;
  legacyFactories: Partial<Record<AdapterId, (task: any, streamer: any, deps: any) => Promise<any>>>;
  logger: Logger;
}

export function createAcpRuntimeAdapter(adapterId: AdapterId) {
  return async function adapterFactory(task: { id: string; worktreeRoot: string }, streamer: { emit: (e: unknown) => void }, deps: AcpDeps) {
    const flag = process.env[`BRIDGE_ACP_${adapterId.toUpperCase()}`];
    if (flag === "0" && deps.legacyFactories[adapterId]) {
      return deps.legacyFactories[adapterId]!(task, streamer, deps);
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
      logger: deps.logger,
      multiplexedClient: deps.multiplexedClient,
    });

    const lc = liveControlsFor(session.capabilities);
    const throwUnsupported = (m: string) => {
      throw new AcpCapabilityUnsupported(m, "not_advertised");
    };

    return {
      liveControls: lc,
      execute: async (req: { prompt: string }) =>
        session.prompt([{ type: "text", text: req.prompt } as any]).then((stopReason) => ({ stopReason })),
      cancel: () => session.cancel(),
      interrupt: () => session.cancel(),
      setModel: (m: string) => (lc.setModel ? session.setModel(m) : throwUnsupported("setModel")),
      setMode: (m: string) => (lc.setMode ? session.setMode(m) : throwUnsupported("setMode")),
      setConfigOption: (k: string, v: unknown) => session.setConfigOption(k, v),
      setThinkingBudget: (n: number) =>
        lc.setThinkingBudget
          ? session.setConfigOption("thinking_budget", n)
          : throwUnsupported("setThinkingBudget"),
      fork: async () => session.forkSession(),
      rollback: async () => {
        throw new AcpCapabilityUnsupported("rollback", "replay_not_yet_implemented");
      },
      revert: (args: unknown) => deps.worktreeService.revert(args),
      getMessages: () => deps.taskEventsService.messages(task.id),
      getDiff: (args: unknown) => deps.worktreeService.diff(args),
      executeCommand: async (c: string) => {
        try {
          return await session.extMethod("agent/executeCommand", { command: c });
        } catch {
          return session.prompt([{ type: "text", text: `/run ${c}` } as any]);
        }
      },
      executeShell: () => {
        throw new AcpCapabilityUnsupported("executeShell", "use_terminal_tool");
      },
      getMcpServerStatus: () => deps.mcpServersStatus(task),
      dispose: () => session.dispose(),
    };
  };
}
```

- [ ] **Step 2: Create barrel `index.ts`**

Create `src-bridge/src/runtime/acp/index.ts`:

```ts
export { ACP_ADAPTERS, type AdapterId, type AcpAdapterConfig } from "./registry.js";
export {
  AcpProtocolError,
  AcpProcessCrash,
  AcpCancelTimeout,
  AcpTransportClosed,
  AcpConcurrentPrompt,
  AcpCapabilityUnsupported,
  AcpAuthMissing,
  AcpCommandNotFound,
} from "./errors.js";
export { ChildProcessHost } from "./process-host.js";
export { AcpConnectionPool } from "./connection-pool.js";
export { createPooledEntryFactory } from "./connection-pool-factory.js";
export { MultiplexedClient, type PerSessionContext } from "./multiplexed-client.js";
export { AcpSession, type AcpSessionOptions } from "./session.js";
export { createAcpRuntimeAdapter, type AcpDeps } from "./adapter-factory.js";
export { FsSandbox } from "./fs-sandbox.js";
export { TerminalManager } from "./terminal-manager.js";
export { liveControlsFor, gateUnstable } from "./capabilities.js";
export { mapSessionUpdate } from "./events/session-update.js";
```

- [ ] **Step 3: Typecheck**

Run: `cd src-bridge && bun run typecheck`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add src-bridge/src/runtime/acp/adapter-factory.ts src-bridge/src/runtime/acp/index.ts
git commit -m "feat(acp): adapter-factory mapping AcpSession to RuntimeAdapter face"
```

---

## Task 6b: Wire into `runtime/registry.ts`; drop Claude-only live_controls gate

**Files:**
- Modify: `src-bridge/src/runtime/registry.ts`
- Modify: `src-bridge/src/runtime/agent-runtime.ts` lines 135-143

Spec §4.6, §7.3. Replace 5 legacy adapter factories with `createAcpRuntimeAdapter(id)`; fallback preserved via `legacyFactories` for `BRIDGE_ACP_<ADAPTER>=0`.

- [ ] **Step 1: Read current registry factory structure**

Run:
```bash
rg -n "claude_code|codex|opencode|cursor|gemini" src-bridge/src/runtime/registry.ts | head -40
```
Identify the 5 adapter factory blocks (by runtime id) within `AgentRuntimeRegistry` or `createRuntimeRegistry`.

- [ ] **Step 2: Add `AcpConnectionPool` + `MultiplexedClient` wiring to registry init**

Edit `src-bridge/src/runtime/registry.ts` — near the top of `createRuntimeRegistry` (or constructor of `AgentRuntimeRegistry`), instantiate:

```ts
import { AcpConnectionPool, MultiplexedClient, createPooledEntryFactory, createAcpRuntimeAdapter, TerminalManager, FsSandbox } from "./acp/index.js";

// Inside the factory / constructor, alongside other deps:
const multiplexedClient = new MultiplexedClient({ logger: deps.logger });
const acpPool = new AcpConnectionPool({
  logger: deps.logger,
  factory: createPooledEntryFactory({
    logger: deps.logger,
    clientDispatcher: multiplexedClient,
    resolveEnv: (adapterId) => {
      // pull relevant env per adapter (ANTHROPIC_API_KEY, OPENAI_API_KEY, ...)
      switch (adapterId) {
        case "claude_code": return { ANTHROPIC_API_KEY: process.env.ANTHROPIC_API_KEY ?? "" };
        case "codex":       return { OPENAI_API_KEY: process.env.OPENAI_API_KEY ?? "" };
        default:            return {};
      }
    },
  }),
});
const terminalManager = new TerminalManager();
const acpDeps = {
  pool: acpPool,
  multiplexedClient,
  makeFsSandbox: (root: string) => new FsSandbox(root),
  terminalManager,
  permissionRouter: {
    async request(taskId, toolCall, options) {
      const reg = deps.hookCallbackManager.register({ taskId, kind: "permission" });
      deps.eventStreamer.emit({ type: "permission_request", request_id: reg.id, tool_call: toolCall, options });
      return await deps.hookCallbackManager.waitFor(reg.id, 30 * 60_000) as any;
    },
  },
  resolveMcpServersFor: (_task) => [] /* pull from existing MCP registry */,
  worktreeService: deps.worktreeService,
  taskEventsService: deps.taskEventsService,
  mcpServersStatus: (task) => deps.mcpClientHub.status(task),
  legacyFactories: { claude_code: legacyClaudeFactory, codex: legacyCodexFactory, opencode: legacyOpenCodeFactory, cursor: legacyCursorFactory, gemini: legacyGeminiFactory },
  logger: deps.logger,
};
```

(Adapt parameter names to what already exists in the file — this is illustrative scaffolding; the engineer maps `deps.hookCallbackManager`, `deps.eventStreamer`, `deps.worktreeService`, `deps.mcpClientHub` to the actual field names present.)

- [ ] **Step 3: Replace each of the 5 adapter factories to use the ACP factory**

Inside whichever `switch` / map creates adapters by runtime id, replace the factory body for each of the five ids. For example (shape; match the switch/map structure already in `registry.ts`):

```ts
case "claude_code":
  return createAcpRuntimeAdapter("claude_code")(task, streamer, acpDeps);
case "codex":
  return createAcpRuntimeAdapter("codex")(task, streamer, acpDeps);
case "opencode":
  return createAcpRuntimeAdapter("opencode")(task, streamer, acpDeps);
case "cursor":
  return createAcpRuntimeAdapter("cursor")(task, streamer, acpDeps);
case "gemini":
  return createAcpRuntimeAdapter("gemini")(task, streamer, acpDeps);
```

Keep the original closures exposed inside `acpDeps.legacyFactories` under the same five keys so `BRIDGE_ACP_<ADAPTER>=0` can still fall back. `qoder` and `iflow` cases stay on `command-runtime.ts` unchanged.

- [ ] **Step 4: Drop Claude-only `live_controls` gate in `agent-runtime.ts`**

Run: `rg -n "runtime === \"claude_code\"" src-bridge/src/runtime/agent-runtime.ts`
Locate the `if (runtime === "claude_code" && this.claudeQuery)` branch at ~line 135-143.

Replace with capability-driven population. Conceptual shape:

```ts
// agent-runtime.ts — replace the claude-only gate
// before: if (runtime === "claude_code" && this.claudeQuery) { live_controls = {...} }
// after:
const lc = (this.adapter as any)?.liveControls;
if (lc) {
  live_controls = {
    setModel: lc.setModel ? (m: string) => this.adapter.setModel(m) : undefined,
    setMode: lc.setMode ? (m: string) => this.adapter.setMode(m) : undefined,
    setThinkingBudget: lc.setThinkingBudget ? (n: number) => this.adapter.setThinkingBudget(n) : undefined,
    setConfigOption: lc.setConfigOption ? (k: string, v: unknown) => this.adapter.setConfigOption(k, v) : undefined,
    getMcpServerStatus: lc.mcpServerStatus ? () => this.adapter.getMcpServerStatus() : undefined,
  };
}
```

- [ ] **Step 5: Typecheck**

Run: `cd src-bridge && bun run typecheck`
Expected: no errors. Fix by adjusting types as needed.

- [ ] **Step 6: Run existing registry tests**

Run: `cd src-bridge && bun test src/runtime/registry.test.ts`
Expected: all existing tests still pass (legacy factories still reachable; ACP-driven defaults exercised for each of the 5 ids).

- [ ] **Step 7: Run full bridge unit + component suite**

Run: `cd src-bridge && bun test`
Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add src-bridge/src/runtime/registry.ts src-bridge/src/runtime/agent-runtime.ts
git commit -m "feat(acp): wire 5 adapters through createAcpRuntimeAdapter; unify live_controls"
```

---

## Task 7: Integration tests × 5 + dev:backend:verify echo

**Files:**
- Create: `src-bridge/tests/integration/acp/<adapter>.test.ts` (5 files)
- Modify: `scripts/dev-backend-verify.ts` (or equivalent) to include 5-adapter echo

Spec §10 tier 3 + tier 4. Skippable via `SKIP_ACP_INTEGRATION=1`.

- [ ] **Step 1: Create `integration/acp/claude_code.test.ts`**

```ts
import { describe, expect, test } from "bun:test";
import { AcpConnectionPool, MultiplexedClient, createPooledEntryFactory, AcpSession, FsSandbox, TerminalManager } from "../../../src/runtime/acp/index.js";

const skip = process.env.SKIP_ACP_INTEGRATION === "1" || !process.env.ANTHROPIC_API_KEY;

describe.skipIf(skip)("claude_code ACP integration", () => {
  test("smoke prompt echo hello", async () => {
    const mc = new MultiplexedClient({ logger: console as any });
    const pool = new AcpConnectionPool({
      logger: console as any,
      factory: createPooledEntryFactory({
        logger: console as any,
        clientDispatcher: mc,
        resolveEnv: () => ({ ANTHROPIC_API_KEY: process.env.ANTHROPIC_API_KEY! }),
      }),
    });
    const events: any[] = [];
    const session = await AcpSession.open(pool, {
      taskId: "it-claude",
      adapterId: "claude_code",
      cwd: process.cwd(),
      streamer: { emit: (e: any) => events.push(e) } as any,
      permissionRouter: { async request() { return { outcome: "selected", optionId: "allow" }; } } as any,
      fsSandbox: new FsSandbox(process.cwd()),
      terminalManager: new TerminalManager(),
      mcpServers: [],
      logger: console as any,
      multiplexedClient: mc,
    });
    const stop = await session.prompt([{ type: "text", text: "Respond with just the word: hello" } as any]);
    expect(stop).toMatch(/end_turn|cancelled|max/);
    expect(events.some((e) => e.type === "output" && /hello/i.test(e.text))).toBe(true);
    await session.dispose();
    await pool.shutdownAll();
  }, 60_000);
});
```

- [ ] **Step 2: Create `codex.test.ts`**

```ts
import { describe, expect, test } from "bun:test";
import { AcpConnectionPool, MultiplexedClient, createPooledEntryFactory, AcpSession, FsSandbox, TerminalManager } from "../../../src/runtime/acp/index.js";

const skip = process.env.SKIP_ACP_INTEGRATION === "1" || !process.env.OPENAI_API_KEY;

describe.skipIf(skip)("codex ACP integration", () => {
  test("smoke prompt echo hello", async () => {
    const mc = new MultiplexedClient({ logger: console as any });
    const pool = new AcpConnectionPool({
      logger: console as any,
      factory: createPooledEntryFactory({
        logger: console as any,
        clientDispatcher: mc,
        resolveEnv: () => ({ OPENAI_API_KEY: process.env.OPENAI_API_KEY! }),
      }),
    });
    const events: any[] = [];
    const session = await AcpSession.open(pool, {
      taskId: "it-codex",
      adapterId: "codex",
      cwd: process.cwd(),
      streamer: { emit: (e: any) => events.push(e) } as any,
      permissionRouter: { async request() { return { outcome: "selected", optionId: "allow" }; } } as any,
      fsSandbox: new FsSandbox(process.cwd()),
      terminalManager: new TerminalManager(),
      mcpServers: [],
      logger: console as any,
      multiplexedClient: mc,
    });
    const stop = await session.prompt([{ type: "text", text: "Respond with just the word: hello" } as any]);
    expect(stop).toMatch(/end_turn|cancelled|max/);
    expect(events.some((e) => e.type === "output" && /hello/i.test(e.text))).toBe(true);
    await session.dispose();
    await pool.shutdownAll();
  }, 60_000);
});
```

- [ ] **Step 3: Create `opencode.test.ts` / `cursor.test.ts` / `gemini.test.ts`**

All three follow the same template as `codex.test.ts` above, but with CLI-presence skip guards instead of env var guards. Here is `opencode.test.ts`:

```ts
import { describe, expect, test } from "bun:test";
import { AcpConnectionPool, MultiplexedClient, createPooledEntryFactory, AcpSession, FsSandbox, TerminalManager } from "../../../src/runtime/acp/index.js";

const skip = process.env.SKIP_ACP_INTEGRATION === "1" || !Bun.which("opencode");

describe.skipIf(skip)("opencode ACP integration", () => {
  test("smoke prompt echo hello", async () => {
    const mc = new MultiplexedClient({ logger: console as any });
    const pool = new AcpConnectionPool({
      logger: console as any,
      factory: createPooledEntryFactory({ logger: console as any, clientDispatcher: mc }),
    });
    const events: any[] = [];
    const session = await AcpSession.open(pool, {
      taskId: "it-opencode",
      adapterId: "opencode",
      cwd: process.cwd(),
      streamer: { emit: (e: any) => events.push(e) } as any,
      permissionRouter: { async request() { return { outcome: "selected", optionId: "allow" }; } } as any,
      fsSandbox: new FsSandbox(process.cwd()),
      terminalManager: new TerminalManager(),
      mcpServers: [],
      logger: console as any,
      multiplexedClient: mc,
    });
    const stop = await session.prompt([{ type: "text", text: "Respond with just the word: hello" } as any]);
    expect(stop).toMatch(/end_turn|cancelled|max/);
    expect(events.some((e) => e.type === "output" && /hello/i.test(e.text))).toBe(true);
    await session.dispose();
    await pool.shutdownAll();
  }, 60_000);
});
```

For `cursor.test.ts`: identical shape, `adapterId: "cursor"`, `!Bun.which("cursor-agent")` in skip.
For `gemini.test.ts`: identical shape, `adapterId: "gemini"`, `!Bun.which("gemini")` in skip.

- [ ] **Step 3: Run integration suite (with env set up)**

```bash
cd src-bridge && ANTHROPIC_API_KEY=... bun test tests/integration/acp/claude_code.test.ts
```

Expected (when env valid): 1 pass.

On CI without keys:
```bash
cd src-bridge && SKIP_ACP_INTEGRATION=1 bun test tests/integration/acp/
```
Expected: all skipped.

- [ ] **Step 4: Add 5-adapter echo to `dev:backend:verify`**

Find the verify script (likely `scripts/dev-backend-verify.ts` or referenced in root `package.json` under `dev:backend:verify`). Add a block after existing checks:

```ts
// 5-adapter ACP echo (best effort; skipped when env/binary missing)
for (const adapterId of ["claude_code", "codex", "opencode", "cursor", "gemini"] as const) {
  try {
    await echoViaAcp(adapterId);
    console.log(`✓ ACP ${adapterId}: echo ok`);
  } catch (e) {
    console.warn(`- ACP ${adapterId}: skipped (${(e as Error).message})`);
  }
}
```

`echoViaAcp` spawns pool, opens session, prompts "echo ok", disposes.

- [ ] **Step 5: Commit**

```bash
git add src-bridge/tests/integration/acp/ scripts/dev-backend-verify.ts
git commit -m "test(acp): integration tests for 5 adapters + dev:backend:verify echo"
```

---

## Task 8: Go `im_forward` root-task rollup

**Files:**
- Modify: `src-go/internal/repository/task_repo.go`
- Test: `src-go/internal/repository/task_repo_test.go`
- Modify: `src-go/internal/service/im_*_forward.go` or equivalent observer wiring
- Test: corresponding `_test.go`

Spec §3.1, §7.5, §9.1. Ensure progress/result from child tasks reaches root task's IM reply target.

- [ ] **Step 1: Inspect current task_repo + im_forward**

Run:
```bash
rg -n "GetAncestorRoot|ParentID|parent_id" src-go/internal/repository/task_repo.go | head
rg -n "im_forward|ImForward|imForwarder" src-go/internal/service | head
```

Identify: the repository's current method set, and the observer/service that today takes an event and finds `im_reply_target` on the task.

- [ ] **Step 2: Write failing test for `GetAncestorRoot`**

Add to `src-go/internal/repository/task_repo_test.go`:

```go
func TestTaskRepo_GetAncestorRoot(t *testing.T) {
    // Arrange: root (no parent) → child1 → child2
    repo := newTestRepo(t)
    root := createTask(t, repo, nil)
    child1 := createTask(t, repo, &root.ID)
    child2 := createTask(t, repo, &child1.ID)

    // Act
    r1, err := repo.GetAncestorRoot(context.Background(), child2.ID)

    // Assert
    if err != nil { t.Fatal(err) }
    if r1.ID != root.ID {
        t.Fatalf("expected root %v, got %v", root.ID, r1.ID)
    }
}
```

If `createTask(t, repo, parentID)` does not exist in the test utilities, add this helper to the same test file:

```go
func createTask(t *testing.T, repo model.TaskRepository, parentID *uuid.UUID) *model.Task {
    t.Helper()
    task := &model.Task{
        ID:       uuid.New(),
        ParentID: parentID,
        Title:    "test task",
    }
    if err := repo.Create(context.Background(), task); err != nil {
        t.Fatalf("create task: %v", err)
    }
    return task
}
```

- [ ] **Step 3: Run test — expect FAIL (method missing)**

Run: `cd src-go && go test ./internal/repository/...`
Expected: `repo.GetAncestorRoot undefined`.

- [ ] **Step 4: Implement `GetAncestorRoot`**

Add to `src-go/internal/repository/task_repo.go`:

```go
// GetAncestorRoot walks ParentID to the topmost task (where ParentID is nil).
// Depth is bounded at 32 to prevent pathological cycles.
func (r *taskRepo) GetAncestorRoot(ctx context.Context, taskID uuid.UUID) (*model.Task, error) {
    current := taskID
    for depth := 0; depth < 32; depth++ {
        t, err := r.GetByID(ctx, current)
        if err != nil {
            return nil, err
        }
        if t.ParentID == nil {
            return t, nil
        }
        current = *t.ParentID
    }
    return nil, fmt.Errorf("task ancestor chain exceeds depth 32 starting from %s", taskID)
}
```

And expose it on the `TaskRepository` interface.

- [ ] **Step 5: Run test — expect PASS**

Run: `cd src-go && go test ./internal/repository/... -run TestTaskRepo_GetAncestorRoot`
Expected: PASS.

- [ ] **Step 6: Write failing test for im_forward rollup**

Locate the observer — likely `src-go/internal/service/im_forward_observer.go` or inside `im_service.go`. Add a test that:
1. Creates a root task with `im_reply_target = {chat: "C1", msg: "M1"}`.
2. Creates a child task with `ParentID = root.ID` and nil reply target.
3. Fires an event with `task_id = child.ID`.
4. Asserts the IM delivery was sent to `C1 / M1`.

- [ ] **Step 7: Implement rollup in the observer**

In the observer, replace:
```go
task, _ := taskRepo.GetByID(ctx, ev.TaskID)
if task.IMReplyTarget == nil { return }
deliver(task.IMReplyTarget, ev)
```
with:
```go
root, err := taskRepo.GetAncestorRoot(ctx, ev.TaskID)
if err != nil || root.IMReplyTarget == nil { return }
deliver(root.IMReplyTarget, ev)
```

- [ ] **Step 8: Run Go suite**

Run: `cd src-go && go test ./...`
Expected: all pass.

- [ ] **Step 9: Commit**

```bash
git add src-go/internal/repository/task_repo.go src-go/internal/repository/task_repo_test.go src-go/internal/service/im_*.go src-go/internal/service/im_*_test.go
git commit -m "feat(go): im_forward rollup routes sub-agent events to root task's reply target"
```

---

## Task 9: Delete legacy adapter files (after smoke green)

**Files:** (delete)
- `src-bridge/src/handlers/claude-runtime.ts`
- `src-bridge/src/handlers/codex-runtime.ts`
- `src-bridge/src/handlers/opencode-runtime.ts`
- `src-bridge/src/opencode/` (entire legacy transport tree)
- `AgentRuntime.claudeQuery` field and the `runtime === "claude_code"` gate (already done in T6b, verify and delete dead code)

**Files:** (modify)
- `src-bridge/src/handlers/command-runtime.ts` — remove cursor/gemini branches
- `src-bridge/src/runtime/registry.ts` — remove legacyFactories references for the 5 migrated adapters (keep the flag as no-op with a warning log)

Spec §11.2. Pre-requisite: T7 integration smoke passed in local env for all five adapters; flag defaults ON stayed stable for one release cycle.

- [ ] **Step 1: Verify smoke gate**

Confirm: integration tests pass (at minimum for adapters with credentials available) AND `pnpm dev:backend:verify` shows 5/5 echo OK.

- [ ] **Step 2: Delete legacy runtime files**

```bash
rm src-bridge/src/handlers/claude-runtime.ts
rm src-bridge/src/handlers/codex-runtime.ts
rm src-bridge/src/handlers/opencode-runtime.ts
rm -rf src-bridge/src/opencode
```

- [ ] **Step 3: Edit `command-runtime.ts` — remove cursor+gemini branches**

Run: `rg -n "cursor|gemini" src-bridge/src/handlers/command-runtime.ts`

Delete the two adapter factory switch-case entries and any helper code now unreferenced. Keep `qoder` and `iflow` intact.

- [ ] **Step 4: Remove legacyFactories wiring in `registry.ts`**

Replace:
```ts
legacyFactories: { claude_code: legacyClaudeFactory, ... },
```
with a stub that warns when flag=0 is used:
```ts
legacyFactories: {
  claude_code: async () => { deps.logger.warn("BRIDGE_ACP_CLAUDE_CODE=0 requested; legacy removed — ignoring flag"); throw new Error("legacy adapter removed"); },
  /* ... same for codex/opencode/cursor/gemini */
},
```

Remove the now-dead imports of the deleted handler files.

- [ ] **Step 5: Delete `AgentRuntime.claudeQuery` field**

Run: `rg -n "claudeQuery" src-bridge/src | head`

Remove the field from `AgentRuntime` class and any now-dead references.

- [ ] **Step 6: Typecheck + full test run**

Run:
```bash
cd src-bridge && bun run typecheck && bun test
```
Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add -A src-bridge/src
git commit -m "chore(bridge): delete legacy runtime files after ACP smoke-green (T9)"
```

---

## Task 10: Docs + env

**Files:**
- Modify: `docs/PRD.md` (ACP runtime status row)
- Modify: `src-bridge/.env.example` (document BRIDGE_ACP_* flags)
- Modify: `docs/superpowers/specs/2026-04-16-bridge-acp-client-integration.md` (mark as implemented)

- [ ] **Step 1: Document env flags**

Append to `src-bridge/.env.example`:

```
# ACP (Agent Client Protocol) rollout — spec 2026-04-16-bridge-acp-client-integration
# Default ON. Set =0 to fall back to legacy runtime (emergency only; legacy removed in T9).
BRIDGE_ACP_CLAUDE_CODE=1
BRIDGE_ACP_CODEX=1
BRIDGE_ACP_OPENCODE=1
BRIDGE_ACP_CURSOR=1
BRIDGE_ACP_GEMINI=1
```

- [ ] **Step 2: Update PRD.md ACP row**

Find (or add) the runtime table in `docs/PRD.md`; mark the 5 adapters as "ACP via @agentclientprotocol/sdk" and link to the spec.

- [ ] **Step 3: Bump spec status**

Edit `docs/superpowers/specs/2026-04-16-bridge-acp-client-integration.md` line 5 from:
```
- **Status**: draft-2
```
to:
```
- **Status**: implemented (2026-<when finished>)
```
Add a changelog entry:
```
- **implemented** (YYYY-MM-DD) — T0–T9 completed; legacy runtime files deleted; 5 adapters live.
```

- [ ] **Step 4: Commit**

```bash
git add docs/PRD.md src-bridge/.env.example docs/superpowers/specs/2026-04-16-bridge-acp-client-integration.md
git commit -m "docs(acp): mark spec implemented; document BRIDGE_ACP_* flags"
```

---

## Verification checklist (run before merging)

- [ ] `cd src-bridge && bun run typecheck` — clean
- [ ] `cd src-bridge && bun test` — all unit + component pass
- [ ] `cd src-bridge && SKIP_ACP_INTEGRATION=1 bun test tests/integration/` — all skipped cleanly
- [ ] `cd src-go && go test ./...` — all pass (covers T8 rollup)
- [ ] `pnpm dev:backend:verify` — 5 adapters echo OK (or recorded skip reasons)
- [ ] Root-task rollup verified: dispatch workflow with `llm_agent` child node from an IM-originated task; confirm progress appears in the IM thread of the root task, not separately
- [ ] Legacy files absent: `ls src-bridge/src/handlers/` shows no claude/codex/opencode-runtime files; `ls src-bridge/src/opencode` fails
- [ ] Flag rollback verified: set `BRIDGE_ACP_CLAUDE_CODE=0` → bridge logs warning and rejects (legacy removed)

## Open items carried forward (not this plan)

Tracked in spec §12:
- allow_always / reject_always persistence (new spec: permissions UX)
- Cursor `cursor/*` extension UI mapping
- Unified cost_update emission for codex/opencode/cursor/gemini (requires T7 `_meta.usage` sample data)
- Linux Tauri resolution for `@zed-industries/codex-acp` native binary
- Sub-agent folding per IM platform (if飞书/Slack/QQ差异超出 existing renderer capability)
