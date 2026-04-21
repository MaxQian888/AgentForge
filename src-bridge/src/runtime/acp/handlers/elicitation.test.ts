import { describe, expect, test } from "bun:test";
import { createElicitation } from "./elicitation.js";
import type { PerSessionContext } from "../multiplexed-client.js";

function mkCtx(
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  emit: (e: any) => void,
): PerSessionContext {
  return {
    taskId: "t1",
    cwd: "/tmp",
    fsSandbox: { resolve: (_s, p) => p },
    terminalManager: {},
    permissionRouter: {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      request: async () => ({ outcome: "cancelled" } as any),
    },
    streamer: { emit },
    logger: { debug: () => {}, warn: () => {}, error: () => {} },
  };
}

describe("elicitation handler", () => {
  test("emits elicitation_request and returns action:cancel", async () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const emitted: any[] = [];
    const ctx = mkCtx((e) => emitted.push(e));
    const res = await createElicitation(ctx, {
      sessionId: "s42",
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      schema: {} as any,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(res.action).toBe("cancel");
    expect(emitted.length).toBe(1);
    expect(emitted[0].type).toBe("elicitation_request");
    expect(emitted[0].session_id).toBe("s42");
  });

  test("payload is preserved verbatim for downstream observers", async () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const emitted: any[] = [];
    const ctx = mkCtx((e) => emitted.push(e));
    const params = {
      sessionId: "s1",
      prompt: "What's your name?",
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any;
    await createElicitation(ctx, params);
    expect(emitted[0].payload).toBe(params);
  });
});
