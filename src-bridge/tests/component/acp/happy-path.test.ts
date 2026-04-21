import { describe, expect, test } from "bun:test";
import { join } from "node:path";
import { AcpConnectionPool } from "../../../src/runtime/acp/connection-pool.js";
import { AcpSession } from "../../../src/runtime/acp/session.js";
import { createPooledEntryFactory } from "../../../src/runtime/acp/connection-pool-factory.js";
import { MultiplexedClient } from "../../../src/runtime/acp/multiplexed-client.js";

const logger = { debug: () => {}, warn: () => {}, error: () => {} };
const MOCK_AGENT = join(import.meta.dir, "mock-acp-agent.ts");

describe("ACP happy path against mock agent", () => {
  test("initialize → newSession → prompt → agent_message_chunk → end_turn", async () => {
    const events: Array<Record<string, unknown>> = [];
    const mc = new MultiplexedClient({ logger });
    const factory = createPooledEntryFactory({
      logger,
      clientDispatcher: mc,
      resolveSpawn: () => ({
        command: "bun",
        args: [MOCK_AGENT],
        env: {},
      }),
    });
    const pool = new AcpConnectionPool({ logger, factory });

    const session = await AcpSession.open(pool, {
      taskId: "t1",
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      adapterId: "claude_code" as any,
      cwd: process.cwd(),
      streamer: { emit: (e) => events.push(e as Record<string, unknown>) },
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

    const stop = await session.prompt([
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      { type: "text", text: "hi" } as any,
    ]);
    expect(stop).toBe("end_turn");
    // session-update stub maps everything to status_change with kind=acp_passthrough.
    // Real mapping table lands in T5; what matters here is the stream reaches the per-task emitter.
    expect(events.length).toBeGreaterThan(0);
    expect(events.every((e) => e.session_id === session.sessionId)).toBe(true);

    await session.dispose();
    await pool.shutdownAll();
  }, 15_000);
});
