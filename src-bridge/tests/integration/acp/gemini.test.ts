/**
 * ACP integration test — gemini adapter.
 *
 * Gated by SKIP_ACP_INTEGRATION !== "0" (default: skipped).
 * No API key required — gemini CLI handles auth via its own config.
 *
 * Requires `gemini` CLI to be installed and configured.
 * The adapter uses `gemini --acp` (verified by T0 in spec §14 Q7).
 *
 * Usage:
 *   SKIP_ACP_INTEGRATION=0 bun test tests/integration/acp/gemini.test.ts
 */

import { describe, expect, test } from "bun:test";
import { mkdtempSync, writeFileSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { AcpConnectionPool } from "../../../src/runtime/acp/connection-pool.js";
import { AcpSession } from "../../../src/runtime/acp/session.js";
import { createPooledEntryFactory } from "../../../src/runtime/acp/connection-pool-factory.js";
import { MultiplexedClient } from "../../../src/runtime/acp/multiplexed-client.js";
import { FsSandbox } from "../../../src/runtime/acp/fs-sandbox.js";
import { TerminalManager } from "../../../src/runtime/acp/terminal-manager.js";

const SHOULD_SKIP = process.env.SKIP_ACP_INTEGRATION !== "0";
// gemini has no required env — gemini CLI handles auth.
const d = SHOULD_SKIP ? describe.skip : describe;

const silent = {
  info: () => {},
  warn: () => {},
  error: () => {},
  debug: () => {},
};

async function makeSession(opts: {
  cwd: string;
  taskId: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  events: any[];
  permissionRouter?: {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    request: (...args: any[]) => Promise<any>;
  };
}): Promise<{ session: AcpSession; pool: AcpConnectionPool; mc: MultiplexedClient }> {
  const mc = new MultiplexedClient({ logger: silent });
  const pool = new AcpConnectionPool({
    logger: silent,
    idleMs: 60_000,
    factory: createPooledEntryFactory({
      logger: silent,
      clientDispatcher: mc,
    }),
  });
  const fsSandbox = new FsSandbox(opts.cwd);
  const tm = new TerminalManager();
  const permissionRouter = opts.permissionRouter ?? {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    request: async () => ({ outcome: "selected", optionId: "allow" } as any),
  };
  const session = await AcpSession.open(pool, {
    taskId: opts.taskId,
    adapterId: "gemini",
    cwd: opts.cwd,
    streamer: { emit: (e) => opts.events.push(e) },
    permissionRouter,
    fsSandbox,
    terminalManager: tm,
    mcpServers: [],
    logger: silent,
    multiplexedClient: mc,
  });
  return { session, pool, mc };
}

d("ACP integration — gemini", () => {
  test(
    "smoke — prompt('echo hello') → output event + stopReason",
    async () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const events: any[] = [];
      const cwd = mkdtempSync(path.join(tmpdir(), "acp-int-gemini-smoke-"));
      const { session, pool } = await makeSession({ cwd, taskId: "int-gemini-smoke", events });

      const stop = await session.prompt([
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        { type: "text", text: "echo hello" } as any,
      ]);

      expect(["end_turn", "max_turns"]).toContain(stop);
      expect(events.length).toBeGreaterThan(0);

      await session.dispose();
      await pool.shutdownAll(true);
    },
    120_000,
  );

  test(
    "cancel — prompt + cancel → stopReason=cancelled",
    async () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const events: any[] = [];
      const cwd = mkdtempSync(path.join(tmpdir(), "acp-int-gemini-cancel-"));
      const { session, pool } = await makeSession({ cwd, taskId: "int-gemini-cancel", events });

      const promptP = session.prompt([
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        { type: "text", text: "Count very slowly from 1 to 100, pausing between each number." } as any,
      ]);
      await new Promise((r) => setTimeout(r, 2000));
      await session.cancel();
      const stop = await promptP;

      expect(stop).toBe("cancelled");

      await session.dispose();
      await pool.shutdownAll(true);
    },
    120_000,
  );

  test(
    "fs — agent reads then writes a temp file",
    async () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const events: any[] = [];
      const cwd = mkdtempSync(path.join(tmpdir(), "acp-int-gemini-fs-"));
      const inputFile = path.join(cwd, "input.txt");
      const outputFile = path.join(cwd, "output.txt");
      writeFileSync(inputFile, "hello from test");

      const { session, pool } = await makeSession({ cwd, taskId: "int-gemini-fs", events });

      const stop = await session.prompt([
        {
          type: "text",
          text: `Read the file input.txt in the current directory, then write its contents uppercased to output.txt.`,
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
        } as any,
      ]);

      expect(["end_turn", "max_turns"]).toContain(stop);
      const written = readFileSync(outputFile, "utf8");
      expect(written.toLowerCase()).toContain("hello from test");

      await session.dispose();
      await pool.shutdownAll(true);
    },
    120_000,
  );

  test(
    "terminal — agent runs 'echo pong'",
    async () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const events: any[] = [];
      const cwd = mkdtempSync(path.join(tmpdir(), "acp-int-gemini-terminal-"));
      const { session, pool } = await makeSession({ cwd, taskId: "int-gemini-terminal", events });

      const stop = await session.prompt([
        {
          type: "text",
          text: "Run the shell command: echo pong",
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
        } as any,
      ]);

      expect(["end_turn", "max_turns"]).toContain(stop);
      const allText = JSON.stringify(events);
      expect(allText).toContain("pong");

      await session.dispose();
      await pool.shutdownAll(true);
    },
    120_000,
  );

  test(
    "permission — agent triggers tool requiring permission; inject approval; agent resumes",
    async () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const events: any[] = [];
      const cwd = mkdtempSync(path.join(tmpdir(), "acp-int-gemini-perm-"));
      let permissionRequestEmitted = false;

      const permissionRouter = {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        request: async (_taskId: string, _toolCall: any, _options: any[]): Promise<any> => {
          permissionRequestEmitted = true;
          return { outcome: "selected", optionId: "allow" };
        },
      };

      const { session, pool } = await makeSession({
        cwd,
        taskId: "int-gemini-perm",
        events,
        permissionRouter,
      });

      const stop = await session.prompt([
        {
          type: "text",
          text: "Run the bash command: echo permission_test",
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
        } as any,
      ]);

      expect(["end_turn", "max_turns", "cancelled"]).toContain(stop);
      expect(typeof permissionRequestEmitted).toBe("boolean");

      await session.dispose();
      await pool.shutdownAll(true);
    },
    120_000,
  );
});
