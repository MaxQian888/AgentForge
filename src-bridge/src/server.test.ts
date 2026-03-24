import { describe, expect, test } from "bun:test";
import { createApp } from "./server.js";
import { RuntimePoolManager } from "./runtime/pool-manager.js";
import { SessionManager } from "./session/manager.js";

const validRequest = {
  task_id: "task-123",
  title: "Build task decomposition",
  description: "Implement an AI-powered task decomposition endpoint across bridge, Go API, and IM bridge.",
  priority: "high",
};

describe("bridge decompose route", () => {
  test("rejects invalid payloads", async () => {
    const app = createApp({
      decomposeTask: async () => {
        throw new Error("should not be called");
      },
    });

    const response = await app.request("/bridge/decompose", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ task_id: "", title: "" }),
    });

    expect(response.status).toBe(400);
    expect(await response.json()).toMatchObject({
      error: "Validation failed",
    });
  });

  test("returns structured decomposition output on success", async () => {
    const app = createApp({
      decomposeTask: async () => ({
        summary: "Split the bridge, backend, and IM work into separate tasks.",
        subtasks: [
          {
            title: "Add bridge decompose route",
            description: "Create request and response schemas for bridge decomposition.",
            priority: "high",
          },
          {
            title: "Wire Go task API",
            description: "Call the bridge and persist the generated child tasks.",
            priority: "medium",
          },
        ],
      }),
    });

    const response = await app.request("/bridge/decompose", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(validRequest),
    });

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({
      summary: "Split the bridge, backend, and IM work into separate tasks.",
      subtasks: [
        {
          title: "Add bridge decompose route",
          description: "Create request and response schemas for bridge decomposition.",
          priority: "high",
        },
        {
          title: "Wire Go task API",
          description: "Call the bridge and persist the generated child tasks.",
          priority: "medium",
        },
      ],
    });
  });

  test("reports upstream decomposition failures", async () => {
    const app = createApp({
      decomposeTask: async () => {
        throw new Error("LLM unavailable");
      },
    });

    const response = await app.request("/bridge/decompose", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(validRequest),
    });

    expect(response.status).toBe(500);
    expect(await response.json()).toEqual({
      error: "LLM unavailable",
    });
  });

  test("rejects unknown decomposition providers before invoking the executor", async () => {
    let called = false;
    const app = createApp({
      decomposeTask: async () => {
        called = true;
        return {
          summary: "should not be returned",
          subtasks: [],
        };
      },
    });

    const response = await app.request("/bridge/decompose", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        ...validRequest,
        provider: "does-not-exist",
        model: "missing-model",
      }),
    });

    expect(response.status).toBe(400);
    expect(await response.json()).toMatchObject({
      error: "Unknown provider: does-not-exist",
    });
    expect(called).toBe(false);
  });

  test("rejects invalid decomposition output", async () => {
    const app = createApp({
      decomposeTask: async () =>
        ({
          summary: "",
          subtasks: [
            {
              title: "",
              description: "Missing title should fail output validation.",
              priority: "urgent",
            },
          ],
        }) as never,
    });

    const response = await app.request("/bridge/decompose", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(validRequest),
    });

    expect(response.status).toBe(500);
    expect(await response.json()).toMatchObject({
      error: "Invalid decomposition output",
    });
  });
});

describe("bridge execute route", () => {
  test("persists continuity state when the execute route runs to completion", async () => {
    const sessionManager = new SessionManager();
    const app = createApp({
      awaitExecution: true,
      queryRunner: async function* () {
        yield {
          type: "assistant",
          session_id: "session-123",
          message: {
            content: [{ type: "text", text: "Running real work." }],
          },
        };
        yield {
          type: "result",
          session_id: "session-123",
          subtype: "success",
          result: "Done",
          stop_reason: "end_turn",
          total_cost_usd: 0.01,
          usage: {
            input_tokens: 1_000,
            output_tokens: 500,
            cache_read_input_tokens: 0,
          },
        };
      },
      sessionManager,
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const response = await app.request("/bridge/execute", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-123",
        session_id: "session-123",
        prompt: "Implement the bridge runtime.",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-123",
        system_prompt: "",
        max_turns: 8,
        budget_usd: 2,
        allowed_tools: ["Read"],
        permission_mode: "default",
      }),
    });

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ session_id: "session-123" });
    expect(sessionManager.restore("task-123")).toMatchObject({
      task_id: "task-123",
      session_id: "session-123",
      status: "completed",
    });
  });

  test("reports running status, honors cancel, and cleans up runtime truthfully", async () => {
    const pool = new RuntimePoolManager(1);
    const sessionManager = new SessionManager();
    const events: string[] = [];
    const app = createApp({
      pool,
      queryRunner: async function* ({ options }) {
        const abortController = options?.abortController as AbortController | undefined;

        yield {
          type: "assistant",
          session_id: "session-456",
          message: {
            content: [{ type: "text", text: "Working until cancelled." }],
          },
        };

        while (!abortController?.signal.aborted) {
          await Bun.sleep(5);
        }

        throw new Error("aborted by user");
      },
      sessionManager,
      streamer: {
        close() {},
        connect() {},
        send(event: { type: string }) {
          events.push(event.type);
        },
      } as never,
    });

    const executeResponse = await app.request("/bridge/execute", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-456",
        session_id: "session-456",
        prompt: "Run until cancelled.",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-456",
        system_prompt: "",
        max_turns: 8,
        budget_usd: 2,
        allowed_tools: ["Read"],
        permission_mode: "default",
      }),
    });

    expect(executeResponse.status).toBe(200);

    const runningStatusResponse = await app.request("/bridge/status/task-456");
    expect(runningStatusResponse.status).toBe(200);
    expect(await runningStatusResponse.json()).toMatchObject({
      task_id: "task-456",
      state: "running",
    });

    const healthWhileRunning = await app.request("/bridge/health");
    expect(healthWhileRunning.status).toBe(200);
    expect(await healthWhileRunning.json()).toMatchObject({
      status: "SERVING",
      active_agents: 1,
      max_agents: 1,
    });

    const cancelResponse = await app.request("/bridge/cancel", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-456",
        reason: "user requested stop",
      }),
    });

    expect(cancelResponse.status).toBe(200);
    expect(await cancelResponse.json()).toEqual({ success: true });

    await waitFor(() => sessionManager.restore("task-456") !== null);

    expect(sessionManager.restore("task-456")).toMatchObject({
      task_id: "task-456",
      session_id: "session-456",
      status: "failed",
    });
    expect(events).toContain("error");
    expect(events).toContain("snapshot");
    expect(pool.get("task-456")).toBeUndefined();

    const healthAfterCancel = await app.request("/bridge/health");
    expect(healthAfterCancel.status).toBe(200);
    expect(await healthAfterCancel.json()).toMatchObject({
      status: "SERVING",
      active_agents: 0,
      max_agents: 1,
    });
  });

  test("rejects execute requests for providers without agent execution support", async () => {
    const pool = new RuntimePoolManager(1);
    const app = createApp({
      pool,
      awaitExecution: true,
      queryRunner: async function* () {
        yield {
          type: "result",
          session_id: "session-unsupported",
          subtype: "success",
          result: "should not run",
          stop_reason: "end_turn",
          total_cost_usd: 0,
          usage: {
            input_tokens: 0,
            output_tokens: 0,
            cache_read_input_tokens: 0,
          },
        };
      },
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const response = await app.request("/bridge/execute", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-unsupported",
        session_id: "session-unsupported",
        prompt: "Run with an unsupported runtime provider.",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-unsupported",
        system_prompt: "",
        max_turns: 8,
        budget_usd: 2,
        allowed_tools: ["Read"],
        permission_mode: "default",
        provider: "openai",
        model: "gpt-5",
      }),
    });

    expect(response.status).toBe(400);
    expect(await response.json()).toMatchObject({
      error: "Provider openai does not support agent_execution",
    });
    expect(pool.get("task-unsupported")).toBeUndefined();
  });

  test("returns a configuration error when claude_code credentials are missing", async () => {
    const app = createApp({
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
      envLookup() {
        return undefined;
      },
    });

    const response = await app.request("/bridge/execute", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-missing-claude-creds",
        session_id: "session-missing-claude-creds",
        runtime: "claude_code",
        prompt: "Run with missing credentials.",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-missing-claude-creds",
        system_prompt: "",
        max_turns: 8,
        budget_usd: 2,
        allowed_tools: ["Read"],
        permission_mode: "default",
      }),
    });

    expect(response.status).toBe(503);
    expect(await response.json()).toMatchObject({
      error: "Missing required environment variable for runtime claude_code: ANTHROPIC_API_KEY",
    });
  });

  test("honors injected envLookup before pool acquisition for claude runtime validation", async () => {
    const previousApiKey = process.env.ANTHROPIC_API_KEY;
    process.env.ANTHROPIC_API_KEY = "available-in-process-env";

    try {
      const app = createApp({
        pool: new RuntimePoolManager(0),
        streamer: {
          close() {},
          connect() {},
          send() {},
        } as never,
        envLookup() {
          return undefined;
        },
      });

      const response = await app.request("/bridge/execute", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          task_id: "task-env-lookup-priority",
          session_id: "session-env-lookup-priority",
          runtime: "claude_code",
          prompt: "Run with injected missing credentials.",
          worktree_path: "D:/Project/AgentForge",
          branch_name: "agent/task-env-lookup-priority",
          system_prompt: "",
          max_turns: 8,
          budget_usd: 2,
          allowed_tools: ["Read"],
          permission_mode: "default",
        }),
      });

      expect(response.status).toBe(503);
      expect(await response.json()).toMatchObject({
        error:
          "Missing required environment variable for runtime claude_code: ANTHROPIC_API_KEY",
      });
    } finally {
      if (previousApiKey === undefined) {
        delete process.env.ANTHROPIC_API_KEY;
      } else {
        process.env.ANTHROPIC_API_KEY = previousApiKey;
      }
    }
  });
});

async function waitFor(predicate: () => boolean, timeoutMs = 500): Promise<void> {
  const deadline = Date.now() + timeoutMs;

  while (Date.now() < deadline) {
    if (predicate()) {
      return;
    }
    await Bun.sleep(10);
  }

  throw new Error("Timed out waiting for async bridge work to settle.");
}
