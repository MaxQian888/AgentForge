import { describe, expect, test } from "bun:test";
import { join } from "node:path";
import { AcpConnectionPool } from "../../../src/runtime/acp/connection-pool.js";
import { AcpSession } from "../../../src/runtime/acp/session.js";
import { createPooledEntryFactory } from "../../../src/runtime/acp/connection-pool-factory.js";
import { MultiplexedClient } from "../../../src/runtime/acp/multiplexed-client.js";

const logger = { debug: () => {}, warn: () => {}, error: () => {} };
const MOCK_AGENT = join(import.meta.dir, "mock-acp-agent.ts");

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function mkSession(pool: AcpConnectionPool, mc: MultiplexedClient, taskId: string, events: any[]) {
  return AcpSession.open(pool, {
    taskId,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    adapterId: "claude_code" as any,
    cwd: process.cwd(),
    streamer: { emit: (e) => events.push(e) },
    permissionRouter: {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      request: async () => ({ outcome: "selected", optionId: "allow" } as any),
    },
    fsSandbox: { resolve: (_s, p) => p },
    terminalManager: {},
    mcpServers: [],
    logger,
    multiplexedClient: mc,
  });
}

describe("ACP pooling", () => {
  test("two concurrent tasks share one host, distinct sessionIds", async () => {
    const mc = new MultiplexedClient({ logger });
    const factory = createPooledEntryFactory({
      logger,
      clientDispatcher: mc,
      resolveSpawn: () => ({ command: "bun", args: [MOCK_AGENT], env: {} }),
    });
    const pool = new AcpConnectionPool({ logger, factory });

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const eventsA: any[] = [];
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const eventsB: any[] = [];
    const [sessA, sessB] = await Promise.all([
      mkSession(pool, mc, "taskA", eventsA),
      mkSession(pool, mc, "taskB", eventsB),
    ]);

    expect(sessA.sessionId).not.toBe(sessB.sessionId);
    // Both sessions drive the same pooled host (one bun child).
    // Verify by prompting both and observing events route to the right streamer.
    const [stopA, stopB] = await Promise.all([
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      sessA.prompt([{ type: "text", text: "hi-a" } as any]),
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      sessB.prompt([{ type: "text", text: "hi-b" } as any]),
    ]);
    expect(stopA).toBe("end_turn");
    expect(stopB).toBe("end_turn");
    expect(eventsA.every((e) => e.session_id === sessA.sessionId)).toBe(true);
    expect(eventsB.every((e) => e.session_id === sessB.sessionId)).toBe(true);
    expect(eventsA.length).toBeGreaterThan(0);
    expect(eventsB.length).toBeGreaterThan(0);

    await sessA.dispose();
    await sessB.dispose();
    await pool.shutdownAll();
  }, 20_000);
});
