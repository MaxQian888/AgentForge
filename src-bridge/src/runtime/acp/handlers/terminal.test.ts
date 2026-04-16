import { describe, expect, test } from "bun:test";
import {
  createTerminal,
  terminalOutput,
  waitForExit,
  kill,
  release,
} from "./terminal.js";
import { TerminalManager } from "../terminal-manager.js";
import type { PerSessionContext } from "../multiplexed-client.js";

function mkCtx(opts?: { perTaskByteLimit?: number; maxConcurrent?: number }): PerSessionContext {
  return {
    taskId: "t1",
    cwd: process.cwd(),
    fsSandbox: { resolve: (_s, p) => p },
    terminalManager: new TerminalManager(opts),
    permissionRouter: {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      request: async () => ({ outcome: "selected", optionId: "allow" } as any),
    },
    streamer: { emit: () => {} },
    logger: { debug: () => {}, warn: () => {}, error: () => {} },
  };
}

describe("terminal handler", () => {
  test("create → waitForExit → terminalOutput round trip", async () => {
    const ctx = mkCtx();
    const cr = await createTerminal(ctx, {
      sessionId: "s1",
      command: "node",
      args: ["-e", "console.log('pong')"],
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(cr.terminalId).toMatch(/.+/);

    const exit = await waitForExit(ctx, {
      sessionId: "s1",
      terminalId: cr.terminalId,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(exit.exitCode).toBe(0);

    const out = await terminalOutput(ctx, {
      sessionId: "s1",
      terminalId: cr.terminalId,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(out.output).toContain("pong");
    expect(out.exitStatus?.exitCode).toBe(0);

    await release(ctx, {
      sessionId: "s1",
      terminalId: cr.terminalId,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
  }, 10_000);

  test("rejects when maxConcurrent exceeded with -32000", async () => {
    const ctx = mkCtx({ maxConcurrent: 2 });
    const c1 = await createTerminal(ctx, {
      sessionId: "s1",
      command: "node",
      args: ["-e", "setInterval(()=>{},1000)"],
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    const c2 = await createTerminal(ctx, {
      sessionId: "s1",
      command: "node",
      args: ["-e", "setInterval(()=>{},1000)"],
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    await expect(
      createTerminal(ctx, {
        sessionId: "s1",
        command: "node",
        args: ["-e", "1"],
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      } as any),
    ).rejects.toMatchObject({ code: -32000 });

    // cleanup
    await kill(ctx, {
      sessionId: "s1",
      terminalId: c1.terminalId,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    await kill(ctx, {
      sessionId: "s1",
      terminalId: c2.terminalId,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    await release(ctx, {
      sessionId: "s1",
      terminalId: c1.terminalId,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    await release(ctx, {
      sessionId: "s1",
      terminalId: c2.terminalId,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
  }, 10_000);

  test("kill keeps id valid; release frees the slot", async () => {
    const ctx = mkCtx({ maxConcurrent: 2 });
    const c = await createTerminal(ctx, {
      sessionId: "s1",
      command: "node",
      args: ["-e", "setInterval(()=>{},1000)"],
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    await kill(ctx, {
      sessionId: "s1",
      terminalId: c.terminalId,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    // After kill, id is still valid for output query
    const out = await terminalOutput(ctx, {
      sessionId: "s1",
      terminalId: c.terminalId,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(out).toBeDefined();

    await release(ctx, {
      sessionId: "s1",
      terminalId: c.terminalId,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    // After release, output query rejects
    await expect(
      terminalOutput(ctx, {
        sessionId: "s1",
        terminalId: c.terminalId,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      } as any),
    ).rejects.toMatchObject({ code: -32602 });
  }, 10_000);
});
