import { describe, expect, test } from "bun:test";
import { createApp } from "./server.js";
import { RuntimePoolManager } from "./runtime/pool-manager.js";
import { createRuntimeRegistry } from "./runtime/registry.js";
import { HookCallbackManager } from "./runtime/hook-callback-manager.js";

function createAdvancedRequest() {
  return {
    task_id: "task-advanced",
    session_id: "session-advanced",
    runtime: "opencode" as const,
    provider: "opencode",
    model: "opencode-default",
    prompt: "Use the advanced runtime controls",
    worktree_path: "D:/Project/AgentForge",
    branch_name: "agent/task-advanced",
    system_prompt: "",
    max_turns: 8,
    budget_usd: 2,
    allowed_tools: ["Read"],
    permission_mode: "default",
  };
}

describe("bridge advanced runtime routes", () => {
  test("exposes advanced runtime operation routes and resolves permission callbacks", async () => {
    const pool = new RuntimePoolManager(2);
    const runtime = pool.acquire("task-advanced", "session-advanced", "opencode");
    runtime.bindRequest(createAdvancedRequest());
    runtime.continuity = {
      runtime: "opencode",
      resume_ready: true,
      captured_at: 100,
      upstream_session_id: "opencode-session-123",
      fork_available: true,
      revert_message_ids: ["message-1"],
    };

    const pendingManager = new HookCallbackManager({
      fetchImpl: async () => new Response(null, { status: 202 }),
    });
    const pending = await pendingManager.register({
      callbackUrl: "http://127.0.0.1:7777/hooks",
      payload: {
        callback_type: "tool_permission",
        tool_name: "Read",
      },
      timeoutMs: 200,
    });

    const calls: Array<{ kind: string; payload: unknown }> = [];
    const registry = createRuntimeRegistry({
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "test-token";
          case "OPENCODE_SERVER_URL":
            return "http://127.0.0.1:4096";
          default:
            return undefined;
        }
      },
      opencodeTransport: {
        checkReadiness() {
          return Promise.resolve({ ok: true, diagnostics: [] });
        },
      } as never,
      advancedOperations: {
        opencode: {
          async fork(_runtime, params) {
            calls.push({ kind: "fork", payload: params });
            return {
              continuity: {
                runtime: "opencode" as const,
                resume_ready: true,
                captured_at: 200,
                upstream_session_id: params.message_id
                  ? `fork:${params.message_id}`
                  : "forked-session",
                fork_available: true,
                revert_message_ids: [],
              },
            };
          },
          async rollback(_runtime, params) {
            calls.push({ kind: "rollback", payload: params });
          },
          async revert(_runtime, params) {
            calls.push({ kind: "revert", payload: params });
          },
          async getDiff() {
            return [{ path: "src/index.ts", diff: "@@ -1 +1 @@" }];
          },
          async getMessages() {
            return [{ id: "message-1", role: "assistant", content: "Advanced output" }];
          },
          async executeCommand(_runtime, params) {
            calls.push({ kind: "command", payload: params });
            return { success: true, output: "compacted" };
          },
          async interrupt() {
            calls.push({ kind: "interrupt", payload: null });
          },
          async setModel(_runtime, params) {
            calls.push({ kind: "setModel", payload: params });
          },
        },
      },
    });

    const app = createApp({
      pool,
      runtimeRegistry: registry,
      hookCallbackManager: pendingManager,
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const forkResponse = await app.request("/bridge/fork", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-advanced",
        message_id: "message-1",
      }),
    });
    expect(forkResponse.status).toBe(200);
    expect(await forkResponse.json()).toMatchObject({
      continuity: {
        runtime: "opencode",
        upstream_session_id: "fork:message-1",
      },
    });

    const rollbackResponse = await app.request("/bridge/rollback", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-advanced",
        checkpoint_id: "checkpoint-1",
        turns: 1,
      }),
    });
    expect(rollbackResponse.status).toBe(200);
    expect(await rollbackResponse.json()).toEqual({ success: true });

    const revertResponse = await app.request("/bridge/revert", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-advanced",
        message_id: "message-1",
      }),
    });
    expect(revertResponse.status).toBe(200);
    expect(await revertResponse.json()).toEqual({ success: true });

    const unrevertResponse = await app.request("/bridge/unrevert", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-advanced",
      }),
    });
    expect(unrevertResponse.status).toBe(200);
    expect(await unrevertResponse.json()).toEqual({ success: true });

    const diffResponse = await app.request("/bridge/diff/task-advanced");
    expect(diffResponse.status).toBe(200);
    expect(await diffResponse.json()).toEqual([{ path: "src/index.ts", diff: "@@ -1 +1 @@" }]);

    const messagesResponse = await app.request("/bridge/messages/task-advanced");
    expect(messagesResponse.status).toBe(200);
    expect(await messagesResponse.json()).toEqual([
      { id: "message-1", role: "assistant", content: "Advanced output" },
    ]);

    const commandResponse = await app.request("/bridge/command", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-advanced",
        command: "/compact",
        arguments: "--full",
      }),
    });
    expect(commandResponse.status).toBe(200);
    expect(await commandResponse.json()).toEqual({ success: true, output: "compacted" });

    const interruptResponse = await app.request("/bridge/interrupt", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-advanced",
      }),
    });
    expect(interruptResponse.status).toBe(200);
    expect(await interruptResponse.json()).toEqual({ success: true });

    const modelResponse = await app.request("/bridge/model", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-advanced",
        model: "opencode-fast",
      }),
    });
    expect(modelResponse.status).toBe(200);
    expect(await modelResponse.json()).toEqual({ success: true });

    const permissionResponse = await app.request(
      `/bridge/permission-response/${pending.requestId}`,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          decision: "allow",
          reason: "approved",
        }),
      },
    );
    expect(permissionResponse.status).toBe(200);
    expect(await permissionResponse.json()).toEqual({ success: true });
    await expect(pending.response).resolves.toEqual({
      decision: "allow",
      reason: "approved",
    });

    expect(calls).toEqual([
      { kind: "fork", payload: { message_id: "message-1" } },
      { kind: "rollback", payload: { checkpoint_id: "checkpoint-1", turns: 1 } },
      { kind: "revert", payload: { action: "revert", message_id: "message-1" } },
      { kind: "revert", payload: { action: "unrevert" } },
      { kind: "command", payload: { command: "/compact", arguments: "--full" } },
      { kind: "interrupt", payload: null },
      { kind: "setModel", payload: { model: "opencode-fast" } },
    ]);
  });

  test("returns validation and missing-task errors for advanced routes", async () => {
    const app = createApp({
      pool: new RuntimePoolManager(1),
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const invalidFork = await app.request("/bridge/fork", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ task_id: "" }),
    });
    expect(invalidFork.status).toBe(400);

    const invalidRevert = await app.request("/bridge/revert", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ task_id: "task-1" }),
    });
    expect(invalidRevert.status).toBe(400);

    const invalidCommand = await app.request("/bridge/command", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ task_id: "task-1", command: "" }),
    });
    expect(invalidCommand.status).toBe(400);

    const invalidModel = await app.request("/bridge/model", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ task_id: "task-1", model: "" }),
    });
    expect(invalidModel.status).toBe(400);

    const invalidPermission = await app.request("/bridge/permission-response/req-missing", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ decision: "maybe" }),
    });
    expect(invalidPermission.status).toBe(400);

    for (const route of [
      ["/bridge/fork", { task_id: "missing-task" }],
      ["/bridge/rollback", { task_id: "missing-task" }],
      ["/bridge/revert", { task_id: "missing-task", message_id: "message-1" }],
      ["/bridge/unrevert", { task_id: "missing-task" }],
      ["/bridge/command", { task_id: "missing-task", command: "/compact" }],
      ["/bridge/interrupt", { task_id: "missing-task" }],
      ["/bridge/model", { task_id: "missing-task", model: "opencode-fast" }],
    ] as const) {
      const response = await app.request(route[0], {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(route[1]),
      });
      expect(response.status).toBe(404);
      expect(await response.json()).toEqual({ error: "task not found" });
    }

    const diffResponse = await app.request("/bridge/diff/missing-task");
    expect(diffResponse.status).toBe(404);
    expect(await diffResponse.json()).toEqual({ error: "task not found" });

    const messagesResponse = await app.request("/bridge/messages/missing-task");
    expect(messagesResponse.status).toBe(404);
    expect(await messagesResponse.json()).toEqual({ error: "task not found" });

    const missingPermission = await app.request("/bridge/permission-response/req-missing", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ decision: "allow" }),
    });
    expect(missingPermission.status).toBe(404);
    expect(await missingPermission.json()).toEqual({ error: "pending permission request not found" });
  });
});
