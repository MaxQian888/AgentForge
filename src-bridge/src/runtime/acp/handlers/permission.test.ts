import { describe, expect, test } from "bun:test";
import { handle } from "./permission.js";
import type { PerSessionContext } from "../multiplexed-client.js";

function mkCtx(
   
  routerRequest: PerSessionContext["permissionRouter"]["request"],
): PerSessionContext {
  return {
    taskId: "t1",
    cwd: "/tmp",
    fsSandbox: { resolve: (_s, p) => p },
    terminalManager: {},
    permissionRouter: { request: routerRequest },
    streamer: { emit: () => {} },
    logger: { debug: () => {}, warn: () => {}, error: () => {} },
  };
}

describe("permission handler", () => {
  test("returns selected outcome when router resolves selected", async () => {
    const ctx = mkCtx(async () => ({
      outcome: "selected",
      optionId: "allow",
    }));
    const res = await handle(ctx, {
      sessionId: "s1",
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      toolCall: { name: "Write" } as any,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      options: [{ optionId: "allow", name: "Allow" }] as any,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(res.outcome).toEqual({ outcome: "selected", optionId: "allow" });
  });

  test("returns cancelled outcome when router resolves cancelled", async () => {
    const ctx = mkCtx(async () => ({ outcome: "cancelled" }));
    const res = await handle(ctx, {
      sessionId: "s1",
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      toolCall: {} as any,
      options: [],
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(res.outcome).toEqual({ outcome: "cancelled" });
  });

  test("forwards taskId + toolCall + options to router", async () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const calls: any[] = [];
    const ctx = mkCtx(async (taskId, tc, opts) => {
      calls.push({ taskId, tc, opts });
      return { outcome: "cancelled" };
    });
    await handle(ctx, {
      sessionId: "s1",
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      toolCall: { name: "Bash" } as any,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      options: [{ optionId: "a" }] as any,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(calls[0].taskId).toBe("t1");
    expect(calls[0].tc).toEqual({ name: "Bash" });
    expect(calls[0].opts).toEqual([{ optionId: "a" }]);
  });

  test("router rejection propagates", async () => {
    const ctx = mkCtx(async () => {
      throw new Error("router failure");
    });
    await expect(
      handle(ctx, {
        sessionId: "s1",
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        toolCall: {} as any,
        options: [],
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      } as any),
    ).rejects.toThrow("router failure");
  });
});
