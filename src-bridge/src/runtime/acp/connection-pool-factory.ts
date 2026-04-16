import {
  ClientSideConnection,
  ndJsonStream,
} from "@agentclientprotocol/sdk";
import { ChildProcessHost, type Logger } from "./process-host.js";
import type { PooledEntry, PooledEntryFactory } from "./connection-pool.js";
import type { MultiplexedClient } from "./multiplexed-client.js";
import type { AdapterId } from "./registry.js";
import { ACP_ADAPTERS } from "./registry.js";
import { AcpAuthMissing } from "./errors.js";

export interface PooledEntryFactoryOpts {
  logger: Logger;
  /** The client-side handler that inbound agent calls are dispatched to. */
  clientDispatcher: MultiplexedClient;
  /**
   * Optional override of the spawn descriptor (command/args/env).
   * Production code uses the default derived from ACP_ADAPTERS +
   * resolveEnv; tests supply a mock agent path via this hook.
   */
  resolveSpawn?: (adapterId: AdapterId) => {
    command: string;
    args: readonly string[];
    env: Record<string, string>;
  };
  /** Per-adapter env resolver (e.g., pulls ANTHROPIC_API_KEY / OPENAI_API_KEY). */
  resolveEnv?: (adapterId: AdapterId) => Record<string, string>;
}

/**
 * Builds a `PooledEntryFactory` that the `AcpConnectionPool` calls
 * on a pool miss. Responsibilities:
 *
 *   1. Resolve spawn descriptor for the adapter (command + args +
 *      env), honoring `envRequired` from ACP_ADAPTERS.
 *   2. Fail fast with `AcpAuthMissing` when required env vars are
 *      absent — NO child process is spawned in that case.
 *   3. Spawn via `ChildProcessHost`, wire its stdio through the
 *      SDK's `ndJsonStream()`, and construct a
 *      `ClientSideConnection`.
 *   4. Send `initialize` declaring the client's fs + terminal
 *      capabilities. Cache the agent's returned `AgentCapabilities`
 *      on the `PooledEntry.caps` field for subsequent
 *      `AcpSession.open` calls.
 *
 * Because `ClientSideConnection` takes a `toClient` callback that
 * returns the SAME `Client` instance for the whole connection,
 * `clientDispatcher` (a `MultiplexedClient`) is shared across all
 * sessions spawned on this host — dispatching inbound calls to the
 * correct per-session context by `params.sessionId`.
 */
export function createPooledEntryFactory(
  opts: PooledEntryFactoryOpts,
): PooledEntryFactory {
  return async (adapterId: AdapterId): Promise<PooledEntry> => {
    const spec = ACP_ADAPTERS[adapterId];
    const envFromResolver = opts.resolveEnv?.(adapterId) ?? {};
    const spawnOverridden = Boolean(opts.resolveSpawn);
    const spawnSpec = opts.resolveSpawn?.(adapterId) ?? {
      command: spec.command,
      args: spec.args,
      env: envFromResolver,
    };

    // Env check only applies to the default spawn (real adapter binary).
    // When `resolveSpawn` is overridden (e.g., component tests spawning a
    // mock agent), the caller owns spawn semantics — ANTHROPIC_API_KEY etc.
    // are meaningless for a local fixture. Skipping the check preserves
    // the AcpAuthMissing safeguard for production while unblocking tests.
    if (!spawnOverridden) {
      const missing = spec.envRequired.filter(
        (k) => !(k in spawnSpec.env) && !(k in process.env),
      );
      if (missing.length > 0) {
        throw new AcpAuthMissing(adapterId, missing);
      }
    }

    const host = new ChildProcessHost({
      adapterId,
      command: spawnSpec.command,
      args: spawnSpec.args,
      env: spawnSpec.env,
      logger: opts.logger,
    });
    const io = await host.start();
    const stream = ndJsonStream(io.stdin, io.stdout);
    const conn = new ClientSideConnection(
      () => opts.clientDispatcher,
      stream,
    );

    const initRes = await conn.initialize({
      protocolVersion: 1,
      clientCapabilities: {
        fs: { readTextFile: true, writeTextFile: true },
        terminal: true,
      },
    });

    return {
      host,
      conn,
      // `agentCapabilities` is optional in InitializeResponse; treat
      // omission as "agent advertises nothing" (all unstable gates fail closed).
      caps: initRes.agentCapabilities ?? {},
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      clientDispatcher: opts.clientDispatcher as any,
      sessions: new Set<string>(),
      restartPending: false,
    };
  };
}
