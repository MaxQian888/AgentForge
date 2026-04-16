import { describe, expect, test } from "bun:test";
import { join } from "node:path";
import { AcpConnectionPool } from "../../../src/runtime/acp/connection-pool.js";
import { AcpSession } from "../../../src/runtime/acp/session.js";
import { createPooledEntryFactory } from "../../../src/runtime/acp/connection-pool-factory.js";
import { MultiplexedClient } from "../../../src/runtime/acp/multiplexed-client.js";
import { AcpConcurrentPrompt } from "../../../src/runtime/acp/errors.js";

const logger = { debug: () => {}, warn: () => {}, error: () => {} };
const MOCK_AGENT = join(import.meta.dir, "mock-acp-agent.ts");

// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function makeSession(events: any[]) {
  const mc = new MultiplexedClient({ logger });
  const pool = new AcpConnectionPool({
    logger,
    factory: createPooledEntryFactory({
      logger,
      clientDispatcher: mc,
      resolveSpawn: () => ({ command: "bun", args: [MOCK_AGENT], env: {} }),
    }),
  });
  const session = await AcpSession.open(pool, {
    taskId: "t1",
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
  return { session, pool };
}

describe("ACP cancel semantics", () => {
  test("cancel mid-prompt → stopReason=cancelled", async () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const events: any[] = [];
    const { session, pool } = await makeSession(events);

    const promptP = session.prompt([
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      { type: "text", text: "cancel-me" } as any,
    ]);
    // Give mock agent time to emit at least one chunk
    await new Promise((r) => setTimeout(r, 80));
    await session.cancel();
    const stop = await promptP;

    expect(stop).toBe("cancelled");
    await session.dispose();
    await pool.shutdownAll();
  }, 15_000);

  test("cancel before prompt is a no-op (next prompt still runs)", async () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const events: any[] = [];
    const { session, pool } = await makeSession(events);

    // Cancel with no prompt in flight — the ACP spec allows this as a
    // notification; there is nothing to cancel, so the next prompt
    // still runs to completion.
    await session.cancel();
    const stop = await session.prompt([
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      { type: "text", text: "hi" } as any,
    ]);
    expect(stop).toBe("end_turn");

    await session.dispose();
    await pool.shutdownAll();
  }, 15_000);

  test("concurrent prompt on same session throws AcpConcurrentPrompt", async () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const events: any[] = [];
    const { session, pool } = await makeSession(events);

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const p1 = session.prompt([{ type: "text", text: "first" } as any]);
    // Second call while first is in flight should reject immediately, not queue
    await expect(
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      session.prompt([{ type: "text", text: "second" } as any]),
    ).rejects.toBeInstanceOf(AcpConcurrentPrompt);
    await p1; // let first finish cleanly

    await session.dispose();
    await pool.shutdownAll();
  }, 15_000);
});
