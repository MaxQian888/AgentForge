import { describe, expect, test } from "bun:test";
import { HookCallbackManager } from "./hook-callback-manager.js";

describe("HookCallbackManager", () => {
  test("registers a pending callback, POSTs the payload, and resolves it later", async () => {
    const fetchCalls: Array<{ url: string; body: Record<string, unknown> }> = [];
    const manager = new HookCallbackManager({
      fetchImpl: async (input, init) => {
        fetchCalls.push({
          url: String(input),
          body: JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>,
        });
        return new Response(null, { status: 202 });
      },
    });

    const pending = await manager.register({
      callbackUrl: "http://127.0.0.1:7777/hooks",
      payload: {
        callback_type: "tool_permission",
        tool_name: "Read",
      },
      timeoutMs: 200,
    });

    expect(fetchCalls).toHaveLength(1);
    expect(fetchCalls[0]?.url).toBe("http://127.0.0.1:7777/hooks");
    expect(fetchCalls[0]?.body).toMatchObject({
      callback_type: "tool_permission",
      tool_name: "Read",
      request_id: pending.requestId,
    });

    expect(
      manager.resolve(pending.requestId, {
        decision: "allow",
        reason: "approved",
      }),
    ).toBe(true);

    await expect(pending.response).resolves.toEqual({
      decision: "allow",
      reason: "approved",
    });
  });

  test("times out unresolved callbacks", async () => {
    const manager = new HookCallbackManager({
      fetchImpl: async () => new Response(null, { status: 202 }),
    });

    const pending = await manager.register({
      callbackUrl: "http://127.0.0.1:7777/hooks",
      payload: {
        callback_type: "tool_permission",
      },
      timeoutMs: 10,
    });

    await expect(pending.response).rejects.toThrow("timed out");
  });

  test("rejects pending callbacks explicitly", async () => {
    const manager = new HookCallbackManager({
      fetchImpl: async () => new Response(null, { status: 202 }),
    });

    const pending = await manager.register({
      callbackUrl: "http://127.0.0.1:7777/hooks",
      payload: {
        callback_type: "tool_permission",
      },
      timeoutMs: 200,
    });

    expect(manager.reject(pending.requestId, new Error("permission denied"))).toBe(true);
    await expect(pending.response).rejects.toThrow("permission denied");
  });

  test("rejects registration when the orchestrator callback cannot be notified", async () => {
    const manager = new HookCallbackManager({
      fetchImpl: async () => new Response("boom", { status: 500 }),
    });

    await expect(
      manager.register({
        callbackUrl: "http://127.0.0.1:7777/hooks",
        payload: {
          callback_type: "tool_permission",
        },
        timeoutMs: 200,
      }),
    ).rejects.toThrow("Hook callback registration failed");
  });
});
