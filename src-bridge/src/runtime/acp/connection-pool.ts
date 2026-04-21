import type { ClientSideConnection, Client, AgentCapabilities } from "@agentclientprotocol/sdk";
import type { ChildProcessHost, Logger } from "./process-host.js";
import type { AdapterId } from "./registry.js";

export interface PooledEntry {
  host: ChildProcessHost;
  conn: ClientSideConnection;
  caps: AgentCapabilities;
  clientDispatcher: Client & {
    register(sid: string, ctx: unknown): void;
    unregister(sid: string): void;
  };
  sessions: Set<string>;
  restartPending: boolean;
}

export type PooledEntryFactory = (adapterId: AdapterId) => Promise<PooledEntry>;

// AcquireContext is a forward-compatibility seam for T3b (AcpSession.open).
// The pool accepts but does not act on ctx in T2c; callers wire
// clientDispatcher.register(sessionId, perSessionContext) after session creation.
export interface AcquireContext {
  registerSession(sessionId: string, ctx: unknown): void;
  unregisterSession(sessionId: string): void;
}

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

  // ctx is a forward-compatibility seam per spec §4.3; forwarded by the caller
  // to clientDispatcher.register() after session creation. Not used in T2c.
  async acquire(adapterId: AdapterId, ctx?: AcquireContext): Promise<PooledEntry> {
    void ctx; // ctx forwarded by caller to clientDispatcher after session creation
    this.cancelIdle(adapterId);
    const existing = this.entries.get(adapterId);
    // C1 fix (spec §4.3): also respawn when host.exited is already settled.
    // Use a unique string sentinel — never in host.exited's resolution domain —
    // so Promise.race distinguishes "pending" from "resolved to number|null".
    const hasExited = existing
      ? (await Promise.race([existing.host.exited, Promise.resolve("__pending__" as const)])) !== "__pending__"
      : false;
    if (existing && !existing.restartPending && !hasExited) return existing;

    const pending = this.mutex.get(adapterId);
    if (pending) return pending;

    const p = (async () => {
      if (existing && (existing.restartPending || hasExited)) {
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
