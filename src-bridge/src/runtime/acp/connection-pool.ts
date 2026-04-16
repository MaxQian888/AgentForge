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
