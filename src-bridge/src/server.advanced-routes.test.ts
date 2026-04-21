import { describe, expect, test } from "bun:test";
import { createApp } from "./server.js";
import { RuntimePoolManager } from "./runtime/pool-manager.js";
import { createRuntimeRegistry } from "./runtime/registry.js";
import { HookCallbackManager } from "./runtime/hook-callback-manager.js";
import { OpenCodePendingInteractionStore } from "./session/pending-interactions.js";
import { SessionManager } from "./session/manager.js";

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
          async executeShell(_runtime, params) {
            calls.push({ kind: "shell", payload: params });
            return { success: true, output: "lint ok" };
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

    const shellResponse = await app.request("/bridge/shell", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-advanced",
        command: "pnpm lint",
        agent: "reviewer",
      }),
    });
    expect(shellResponse.status).toBe(200);
    expect(await shellResponse.json()).toEqual({ success: true, output: "lint ok" });

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
      { kind: "shell", payload: { command: "pnpm lint", agent: "reviewer", model: undefined } },
      { kind: "command", payload: { command: "/compact", arguments: "--full" } },
      { kind: "interrupt", payload: null },
      { kind: "setModel", payload: { model: "opencode-fast" } },
    ]);
  });

  test("normalizes OpenCode shell responses to the Go bridge contract", async () => {
    const pool = new RuntimePoolManager(1);
    const runtime = pool.acquire("task-shell", "session-shell", "opencode");
    runtime.bindRequest({
      ...createAdvancedRequest(),
      task_id: "task-shell",
      session_id: "session-shell",
    });
    runtime.continuity = {
      runtime: "opencode",
      resume_ready: true,
      captured_at: 100,
      upstream_session_id: "opencode-session-shell",
      fork_available: true,
    };

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
          async executeShell() {
            return {
              ok: true,
              command: "pnpm lint",
            };
          },
        },
      },
    });

    const app = createApp({
      pool,
      runtimeRegistry: registry,
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const shellResponse = await app.request("/bridge/shell", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-shell",
        command: "pnpm lint",
      }),
    });

    expect(shellResponse.status).toBe(200);
    expect(await shellResponse.json()).toEqual({
      success: true,
      output: JSON.stringify({
        ok: true,
        command: "pnpm lint",
      }),
      task_id: "task-shell",
      session_id: "session-shell",
    });
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

    const invalidShell = await app.request("/bridge/shell", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ task_id: "task-1", command: "" }),
    });
    expect(invalidShell.status).toBe(400);

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
      ["/bridge/shell", { task_id: "missing-task", command: "pnpm lint" }],
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

  test("supports Claude live-control routes and returns structured unsupported errors", async () => {
    const pool = new RuntimePoolManager(2);

    const claudeRuntime = pool.acquire("task-claude", "session-claude", "claude_code");
    claudeRuntime.bindRequest({
      ...createAdvancedRequest(),
      task_id: "task-claude",
      session_id: "session-claude",
      runtime: "claude_code",
      provider: "anthropic",
      model: "claude-sonnet-4-5",
    });
    const controlCalls: Array<{ kind: string; payload: unknown }> = [];
    claudeRuntime.acpAdapter = {
      liveControls: { setThinkingBudget: true, mcpServerStatus: true, interrupt: true, setModel: false },
      setThinkingBudget: async (value: number | null) => {
        controlCalls.push({ kind: "thinking", payload: value });
      },
      session: {
        extMethod: async () => [{ name: "github", healthy: true }],
      },
    } as never;

    const codexRuntime = pool.acquire("task-codex", "session-codex", "codex");
    codexRuntime.bindRequest({
      ...createAdvancedRequest(),
      task_id: "task-codex",
      session_id: "session-codex",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
    });

    const app = createApp({
      pool,
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const thinkingResponse = await app.request("/bridge/thinking", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-claude",
        max_thinking_tokens: 1024,
      }),
    });
    expect(thinkingResponse.status).toBe(200);
    expect(await thinkingResponse.json()).toEqual({ success: true });

    const mcpStatusResponse = await app.request("/bridge/mcp-status/task-claude");
    expect(mcpStatusResponse.status).toBe(200);
    expect(await mcpStatusResponse.json()).toEqual([{ name: "github", healthy: true }]);

    const unsupportedShell = await app.request("/bridge/shell", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-codex",
        command: "echo hi",
      }),
    });
    expect(unsupportedShell.status).toBe(501);
    expect(await unsupportedShell.json()).toEqual({
      error: "Runtime codex does not support executeShell",
      operation: "executeShell",
      runtime: "codex",
      support_state: "unsupported",
      reason_code: "unsupported_operation",
    });

    expect(controlCalls).toEqual([{ kind: "thinking", payload: 1024 }]);
  });

  test("resolves paused OpenCode control routes from persisted session snapshots", async () => {
    const sessionManager = new SessionManager();
    sessionManager.save("task-paused", {
      task_id: "task-paused",
      session_id: "session-paused",
      status: "paused",
      turn_number: 3,
      spent_usd: 0.8,
      created_at: 100,
      updated_at: 200,
      request: {
        ...createAdvancedRequest(),
        task_id: "task-paused",
        session_id: "session-paused",
      },
      continuity: {
        runtime: "opencode",
        resume_ready: true,
        captured_at: 200,
        upstream_session_id: "opencode-session-paused",
        latest_message_id: "message-9",
        fork_available: true,
        revert_message_ids: ["message-9"],
      },
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
          async rollback(runtime, params) {
            calls.push({ kind: "rollback", payload: { continuity: runtime.continuity, params } });
          },
          async getMessages(runtime) {
            calls.push({ kind: "messages", payload: runtime.continuity });
            return [{ id: "paused-message", content: "from snapshot" }];
          },
          async getDiff(runtime) {
            calls.push({ kind: "diff", payload: runtime.continuity });
            return [{ path: "README.md", diff: "@@ -1 +1 @@" }];
          },
          async executeCommand(runtime, params) {
            calls.push({ kind: "command", payload: { continuity: runtime.continuity, params } });
            return { success: true, output: "command from snapshot" };
          },
          async executeShell(runtime, params) {
            calls.push({ kind: "shell", payload: { continuity: runtime.continuity, params } });
            return { success: true, output: "shell from snapshot" };
          },
          async revert(runtime, params) {
            calls.push({ kind: "revert", payload: { continuity: runtime.continuity, params } });
          },
        },
      },
    });

    const app = createApp({
      pool: new RuntimePoolManager(1),
      sessionManager,
      runtimeRegistry: registry,
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const messagesResponse = await app.request("/bridge/messages/task-paused");
    expect(messagesResponse.status).toBe(200);
    expect(await messagesResponse.json()).toEqual([{ id: "paused-message", content: "from snapshot" }]);

    const diffResponse = await app.request("/bridge/diff/task-paused");
    expect(diffResponse.status).toBe(200);
    expect(await diffResponse.json()).toEqual([{ path: "README.md", diff: "@@ -1 +1 @@" }]);

    const rollbackResponse = await app.request("/bridge/rollback", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-paused",
        checkpoint_id: "message-9",
      }),
    });
    expect(rollbackResponse.status).toBe(200);
    expect(await rollbackResponse.json()).toEqual({ success: true });

    const commandResponse = await app.request("/bridge/command", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-paused",
        command: "/compact",
      }),
    });
    expect(commandResponse.status).toBe(200);
    expect(await commandResponse.json()).toEqual({ success: true, output: "command from snapshot" });

    const shellResponse = await app.request("/bridge/shell", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-paused",
        command: "pnpm lint",
      }),
    });
    expect(shellResponse.status).toBe(200);
    expect(await shellResponse.json()).toEqual({ success: true, output: "shell from snapshot" });

    const revertResponse = await app.request("/bridge/revert", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-paused",
        message_id: "message-9",
      }),
    });
    expect(revertResponse.status).toBe(200);
    expect(await revertResponse.json()).toEqual({ success: true });

    expect(calls).toEqual([
      {
        kind: "messages",
        payload: expect.objectContaining({
          upstream_session_id: "opencode-session-paused",
        }),
      },
      {
        kind: "diff",
        payload: expect.objectContaining({
          upstream_session_id: "opencode-session-paused",
        }),
      },
      {
        kind: "rollback",
        payload: {
          continuity: expect.objectContaining({
            upstream_session_id: "opencode-session-paused",
          }),
          params: { checkpoint_id: "message-9", turns: undefined },
        },
      },
      {
        kind: "command",
        payload: {
          continuity: expect.objectContaining({
            upstream_session_id: "opencode-session-paused",
          }),
          params: { command: "/compact", arguments: undefined },
        },
      },
      {
        kind: "shell",
        payload: {
          continuity: expect.objectContaining({
            upstream_session_id: "opencode-session-paused",
          }),
          params: { command: "pnpm lint", agent: undefined, model: undefined },
        },
      },
      {
        kind: "revert",
        payload: {
          continuity: expect.objectContaining({
            upstream_session_id: "opencode-session-paused",
          }),
          params: { action: "revert", message_id: "message-9" },
        },
      },
    ]);
  });

  test("returns an explicit continuity error for paused OpenCode control routes without a bound session", async () => {
    const sessionManager = new SessionManager();
    sessionManager.save("task-paused-missing", {
      task_id: "task-paused-missing",
      session_id: "session-paused-missing",
      status: "paused",
      turn_number: 1,
      spent_usd: 0.2,
      created_at: 100,
      updated_at: 200,
      request: {
        ...createAdvancedRequest(),
        task_id: "task-paused-missing",
        session_id: "session-paused-missing",
      },
      continuity: {
        runtime: "opencode",
        resume_ready: false,
        captured_at: 200,
        blocking_reason: "missing_continuity_state",
      },
    });

    const app = createApp({
      pool: new RuntimePoolManager(1),
      sessionManager,
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const messagesResponse = await app.request("/bridge/messages/task-paused-missing");
    expect(messagesResponse.status).toBe(409);
    expect(await messagesResponse.json()).toEqual({
      error: "OpenCode continuity state is not available for task task-paused-missing",
      code: "missing_continuity_state",
      runtime: "opencode",
    });
  });

  test("forwards permission responses through OpenCode pending mappings", async () => {
    const calls: Array<{ requestId: string; payload: unknown }> = [];
    const app = createApp({
      pool: new RuntimePoolManager(1),
      opencodePendingInteractions: {
        resolvePermissionResponse(requestId: string, payload: unknown) {
          calls.push({ requestId, payload });
          return requestId === "opencode-permission-1";
        },
      } as never,
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const response = await app.request("/bridge/permission-response/opencode-permission-1", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        decision: "allow",
        reason: "approved",
      }),
    });

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ success: true });
    expect(calls).toEqual([
      {
        requestId: "opencode-permission-1",
        payload: { decision: "allow", reason: "approved" },
      },
    ]);
  });

  test("keeps an OpenCode permission request pending until the upstream response succeeds", async () => {
    const store = new OpenCodePendingInteractionStore({
      idGenerator: () => "opencode-permission-retry",
      now: () => 100,
      ttlMs: 5_000,
    });
    const pending = store.createPermissionRequest({
      sessionId: "opencode-session-123",
      permissionId: "perm-42",
    });

    let attempts = 0;
    const app = createApp({
      pool: new RuntimePoolManager(1),
      opencodePendingInteractions: store,
      opencodeTransport: {
        async respondToPermission(sessionId: string, permissionId: string, allow: boolean) {
          attempts += 1;
          expect({ sessionId, permissionId, allow }).toEqual({
            sessionId: "opencode-session-123",
            permissionId: "perm-42",
            allow: true,
          });
          if (attempts === 1) {
            throw new Error("temporary upstream failure");
          }
          return { ok: true };
        },
      } as never,
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const firstResponse = await app.request(`/bridge/permission-response/${pending.requestId}`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        decision: "allow",
      }),
    });

    expect(firstResponse.status).toBe(500);
    expect(await firstResponse.json()).toEqual({ error: "temporary upstream failure" });

    const secondResponse = await app.request(`/bridge/permission-response/${pending.requestId}`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        decision: "allow",
      }),
    });

    expect(secondResponse.status).toBe(200);
    expect(await secondResponse.json()).toEqual({ success: true });
    expect(attempts).toBe(2);

    const missingAfterSuccess = await app.request(`/bridge/permission-response/${pending.requestId}`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        decision: "allow",
      }),
    });

    expect(missingAfterSuccess.status).toBe(404);
    expect(await missingAfterSuccess.json()).toEqual({
      error: "pending permission request not found",
    });
  });

  test("starts and completes OpenCode provider auth through canonical bridge routes", async () => {
    const calls: Array<{ kind: string; provider: string; payload: unknown }> = [];
    const app = createApp({
      pool: new RuntimePoolManager(1),
      opencodeTransport: {
        startProviderOAuth(provider: string, payload: unknown) {
          calls.push({ kind: "start", provider, payload });
          return Promise.resolve({
            url: "https://auth.example.com/start",
            state: "oauth-state-1",
          });
        },
        completeProviderOAuth(provider: string, payload: unknown) {
          calls.push({ kind: "complete", provider, payload });
          return Promise.resolve({ connected: true, provider });
        },
      } as never,
      opencodePendingInteractions: {
        createProviderAuthRequest({ provider }: { provider: string }) {
          calls.push({ kind: "store:start", provider, payload: null });
          return { requestId: "provider-auth-1" };
        },
        consumeProviderAuthRequest(requestId: string) {
          if (requestId !== "provider-auth-1") {
            return null;
          }
          return { provider: "anthropic" };
        },
      } as never,
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const startResponse = await app.request("http://localhost/bridge/opencode/provider-auth/anthropic/start", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        redirect_uri: "http://127.0.0.1:7777/callback",
      }),
    });
    expect(startResponse.status).toBe(200);
    const started = await startResponse.json();
    expect(started).toMatchObject({
      request_id: "provider-auth-1",
      provider: "anthropic",
      auth: {
        url: "https://auth.example.com/start",
        state: "oauth-state-1",
      },
    });

    const completeResponse = await app.request(
      `http://localhost/bridge/opencode/provider-auth/${started.request_id}/complete`,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          code: "oauth-code-1",
          state: "oauth-state-1",
        }),
      },
    );
    expect(completeResponse.status).toBe(200);
    expect(await completeResponse.json()).toEqual({
      connected: true,
      provider: "anthropic",
    });

    expect(calls).toEqual([
      {
        kind: "start",
        provider: "anthropic",
        payload: {
          redirect_uri: "http://127.0.0.1:7777/callback",
        },
      },
      {
        kind: "store:start",
        provider: "anthropic",
        payload: null,
      },
      {
        kind: "complete",
        provider: "anthropic",
        payload: {
          code: "oauth-code-1",
          state: "oauth-state-1",
        },
      },
    ]);
  });
});
