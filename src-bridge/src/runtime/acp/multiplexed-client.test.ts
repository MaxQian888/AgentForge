import { describe, expect, test } from "bun:test";
import { MultiplexedClient } from "./multiplexed-client.js";
import type { PerSessionContext } from "./multiplexed-client.js";
import type { Logger } from "./process-host.js";

const logger: Logger = {
  debug: () => {},
  warn: () => {},
  error: () => {},
};

function mkCtx(overrides: Partial<PerSessionContext> = {}): PerSessionContext {
  return {
    taskId: "t1",
    cwd: "/tmp/wt",
    fsSandbox: {
      resolve: (_sid: string, p: string) => "/tmp/wt/" + p,
    },
    terminalManager: {},
    permissionRouter: {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      request: async () => ({ outcome: "selected", optionId: "allow" } as any),
    },
    streamer: { emit: () => {} },
    logger,
    ...overrides,
  };
}

describe("MultiplexedClient", () => {
  test("routes readTextFile by sessionId", async () => {
    const mc = new MultiplexedClient({ logger });
    let resolvedPath: string | undefined;
    const ctx = mkCtx({
      fsSandbox: {
        resolve: (_s, p) => {
          resolvedPath = p;
          // Return a non-existent path so fs.readFile rejects; we only
          // care that routing reached the real fs handler with the
          // correct per-session sandbox.
          return "/non-existent-for-routing-check/" + p;
        },
      },
    });
    mc.register("s1", ctx);
    await expect(
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      mc.readTextFile!({ sessionId: "s1", path: "foo.ts" } as any),
    ).rejects.toThrow(/ENOENT|no such file/i);
    expect(resolvedPath).toBe("foo.ts");
  });

  test("rejects unknown sessionId with -32602", async () => {
    const mc = new MultiplexedClient({ logger });
    await expect(
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      mc.readTextFile!({ sessionId: "unknown", path: "x" } as any),
    ).rejects.toMatchObject({ code: -32602 });
  });

  test("sessionUpdate errors are swallowed (notification contract)", async () => {
    const mc = new MultiplexedClient({ logger });
    // No session registered — sessionUpdate MUST NOT throw
    await expect(
      mc.sessionUpdate({
        sessionId: "unknown",
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        update: { sessionUpdate: "agent_message_chunk", content: { type: "text", text: "hi" } } as any,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      } as any),
    ).resolves.toBeUndefined();
  });

  test("sessionUpdate emits via per-session streamer", async () => {
    const emitted: unknown[] = [];
    const mc = new MultiplexedClient({ logger });
    mc.register("s42", mkCtx({ streamer: { emit: (e) => emitted.push(e) } }));
    await mc.sessionUpdate({
      sessionId: "s42",
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      update: { sessionUpdate: "agent_message_chunk", content: { type: "text", text: "hi" } } as any,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(emitted.length).toBe(1);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    expect((emitted[0] as any).session_id).toBe("s42");
  });

  test("unregister removes session", async () => {
    const mc = new MultiplexedClient({ logger });
    const ctx = mkCtx();
    mc.register("s2", ctx);
    mc.unregister("s2");
    await expect(
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      mc.readTextFile!({ sessionId: "s2", path: "x" } as any),
    ).rejects.toMatchObject({ code: -32602 });
  });

  test("requestPermission delegates to per-session router", async () => {
    const mc = new MultiplexedClient({ logger });
    let seenTaskId: string | undefined;
    mc.register(
      "s3",
      mkCtx({
        permissionRouter: {
          request: async (taskId) => {
            seenTaskId = taskId;
            return { outcome: "selected", optionId: "allow" };
          },
        },
      }),
    );
    const res = await mc.requestPermission({
      sessionId: "s3",
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      toolCall: { name: "Write" } as any,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      options: [] as any,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    expect(res.outcome).toEqual({ outcome: "selected", optionId: "allow" });
    expect(seenTaskId).toBe("t1");
  });
});
