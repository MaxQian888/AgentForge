import { describe, expect, test } from "bun:test";
import { BRIDGE_HTTP_ROUTE_GROUPS, createApp } from "./server.js";
import { RuntimePoolManager } from "./runtime/pool-manager.js";
import { SessionManager } from "./session/manager.js";

const validRequest = {
  task_id: "task-123",
  title: "Build task decomposition",
  description: "Implement an AI-powered task decomposition endpoint across bridge, Go API, and IM bridge.",
  priority: "high",
};

describe("bridge HTTP contract", () => {
  test("declares canonical /bridge routes and compatibility-only aliases", () => {
    expect(BRIDGE_HTTP_ROUTE_GROUPS.execute).toEqual({
      method: "post",
      canonicalPath: "/bridge/execute",
      compatibilityAliases: ["/execute"],
    });
    expect(BRIDGE_HTTP_ROUTE_GROUPS.decompose).toEqual({
      method: "post",
      canonicalPath: "/bridge/decompose",
      compatibilityAliases: ["/ai/decompose"],
    });
    expect(BRIDGE_HTTP_ROUTE_GROUPS.classifyIntent).toEqual({
      method: "post",
      canonicalPath: "/bridge/classify-intent",
      compatibilityAliases: ["/ai/classify"],
    });
    expect(BRIDGE_HTTP_ROUTE_GROUPS.generate).toEqual({
      method: "post",
      canonicalPath: "/bridge/generate",
      compatibilityAliases: ["/ai/generate"],
    });
    expect(BRIDGE_HTTP_ROUTE_GROUPS.cancel).toEqual({
      method: "post",
      canonicalPath: "/bridge/cancel",
      compatibilityAliases: ["/abort"],
    });
    expect(BRIDGE_HTTP_ROUTE_GROUPS.resume).toEqual({
      method: "post",
      canonicalPath: "/bridge/resume",
      compatibilityAliases: ["/resume"],
    });
    expect(BRIDGE_HTTP_ROUTE_GROUPS.health).toEqual({
      method: "get",
      canonicalPath: "/bridge/health",
      compatibilityAliases: ["/health"],
    });
  });

  test("compatibility aliases share canonical validation and response semantics", async () => {
    const app = createApp({
      decomposeTask: async () => ({
        summary: "Alias route uses the same handler.",
        subtasks: [],
      }),
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const invalidDecomposeCanonical = await app.request("/bridge/decompose", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ task_id: "", title: "" }),
    });
    const invalidDecomposeAlias = await app.request("/ai/decompose", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ task_id: "", title: "" }),
    });
    expect(invalidDecomposeAlias.status).toBe(invalidDecomposeCanonical.status);
    expect(await invalidDecomposeAlias.json()).toEqual(await invalidDecomposeCanonical.json());

    const invalidCancelCanonical = await app.request("/bridge/cancel", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ task_id: "" }),
    });
    const invalidCancelAlias = await app.request("/abort", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ task_id: "" }),
    });
    expect(invalidCancelAlias.status).toBe(invalidCancelCanonical.status);
    expect(await invalidCancelAlias.json()).toEqual(await invalidCancelCanonical.json());

    const runtimesCanonical = await app.request("/bridge/runtimes");
    const runtimesAlias = await app.request("/runtimes");
    expect(runtimesAlias.status).toBe(runtimesCanonical.status);
    expect(await runtimesAlias.json()).toEqual(await runtimesCanonical.json());

    const healthCanonical = await app.request("/bridge/health");
    const healthAlias = await app.request("/health");
    expect(healthAlias.status).toBe(healthCanonical.status);
    const canonicalHealth = await healthCanonical.json();
    const aliasHealth = await healthAlias.json();
    expect(aliasHealth).toMatchObject({
      status: canonicalHealth.status,
      active_agents: canonicalHealth.active_agents,
      max_agents: canonicalHealth.max_agents,
    });
    expect(typeof aliasHealth.uptime_ms).toBe("number");
    expect(typeof canonicalHealth.uptime_ms).toBe("number");
  });
});

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
            executionMode: "agent",
          },
          {
            title: "Wire Go task API",
            description: "Call the bridge and persist the generated child tasks.",
            priority: "medium",
            executionMode: "human",
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
          executionMode: "agent",
        },
        {
          title: "Wire Go task API",
          description: "Call the bridge and persist the generated child tasks.",
          priority: "medium",
          executionMode: "human",
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
              executionMode: "agent",
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
  test("reports active runtimes when agents are running", async () => {
    const pool = new RuntimePoolManager(3);
    const runtimeA = pool.acquire("task-active-1", "session-active-1", "codex");
    runtimeA.status = "running";
    runtimeA.spentUsd = 0.25;
    runtimeA.bindRequest({
      task_id: "task-active-1",
      session_id: "session-active-1",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
      prompt: "run",
      worktree_path: "D:/Project/AgentForge",
      branch_name: "agent/task-active-1",
      system_prompt: "",
      max_turns: 8,
      budget_usd: 2,
      allowed_tools: ["Read"],
      permission_mode: "default",
    });
    const runtimeB = pool.acquire("task-active-2", "session-active-2", "claude_code");
    runtimeB.status = "running";
    runtimeB.spentUsd = 0.5;
    runtimeB.bindRequest({
      task_id: "task-active-2",
      session_id: "session-active-2",
      runtime: "claude_code",
      provider: "anthropic",
      model: "claude-sonnet-4-5",
      prompt: "run",
      worktree_path: "D:/Project/AgentForge",
      branch_name: "agent/task-active-2",
      system_prompt: "",
      max_turns: 8,
      budget_usd: 2,
      allowed_tools: ["Read"],
      permission_mode: "default",
    });
    const app = createApp({ pool });

    const response = await app.request("/bridge/active");

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual([
      expect.objectContaining({
        task_id: "task-active-1",
        runtime: "codex",
        state: "running",
        spent_usd: 0.25,
      }),
      expect.objectContaining({
        task_id: "task-active-2",
        runtime: "claude_code",
        state: "running",
        spent_usd: 0.5,
      }),
    ]);
  });

  test("returns an empty active runtime list when no agents are running", async () => {
    const app = createApp({ pool: new RuntimePoolManager(3) });

    const response = await app.request("/bridge/active");

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual([]);
  });

  test("returns an empty pool summary before any runtime is acquired", async () => {
    const app = createApp({ pool: new RuntimePoolManager(3) });

    const response = await app.request("/bridge/pool");

    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({
      active: 0,
      max: 3,
      warm_total: 0,
      warm_available: 0,
      warm_reuse_hits: 0,
      cold_starts: 0,
    });
  });

  test("exposes runtime catalog metadata and readiness diagnostics", async () => {
    const app = createApp({
      executableLookup(command) {
        switch (command) {
          case "codex":
          case "cursor-agent":
          case "gemini":
          case "qodercli":
          case "iflow":
            return `C:/mock/${command}.exe`;
          default:
            return null;
        }
      },
      codexAuthStatusProvider() {
        return {
          authenticated: true,
          message: "Logged in using an API key",
        };
      },
      envLookup(name) {
        switch (name) {
          case "ANTHROPIC_API_KEY":
            return "";
          case "CLAUDE_CODE_RUNTIME_MODEL":
            return "claude-sonnet-4-5";
          case "CODEX_RUNTIME_MODEL":
            return "gpt-5-codex";
          case "CURSOR_API_KEY":
            return "cursor-token";
          case "GEMINI_API_KEY":
            return "gemini-token";
          case "IFLOW_API_KEY":
            return "iflow-token";
          default:
            return undefined;
        }
      },
    });

    const response = await app.request("/bridge/runtimes");

    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({
      default_runtime: "claude_code",
      runtimes: expect.arrayContaining([
        expect.objectContaining({
          key: "claude_code",
          default_provider: "anthropic",
          compatible_providers: ["anthropic"],
          supported_features: expect.arrayContaining(["structured_output", "interrupt"]),
          available: false,
        }),
        expect.objectContaining({
          key: "codex",
          default_provider: "openai",
          compatible_providers: ["openai", "codex"],
          model_options: expect.arrayContaining(["gpt-5-codex", "o3"]),
          supported_features: expect.arrayContaining(["reasoning", "output_schema"]),
          available: true,
        }),
        expect.objectContaining({
          key: "cursor",
          default_provider: "cursor",
          compatible_providers: ["cursor"],
          model_options: expect.arrayContaining(["claude-sonnet-4-20250514", "gpt-4o"]),
        }),
      ]),
    });
  });

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
      runtime: "claude_code",
      provider: "anthropic",
    });

    const healthWhileRunning = await app.request("/bridge/health");
    expect(healthWhileRunning.status).toBe(200);
    expect(await healthWhileRunning.json()).toMatchObject({
      status: "SERVING",
      active_agents: 1,
      max_agents: 1,
    });

    const poolWhileRunning = await app.request("/bridge/pool");
    expect(poolWhileRunning.status).toBe(200);
    expect(await poolWhileRunning.json()).toMatchObject({
      active: 1,
      max: 1,
      cold_starts: 1,
      warm_reuse_hits: 0,
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
      status: "cancelled",
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

    const poolAfterCancel = await app.request("/bridge/pool");
    expect(poolAfterCancel.status).toBe(200);
    expect(await poolAfterCancel.json()).toMatchObject({
      active: 0,
      max: 1,
      warm_available: 1,
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

  test("rejects explicit runtime and provider combinations before runtime acquisition", async () => {
    const pool = new RuntimePoolManager(1);
    const app = createApp({
      pool,
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      codexAuthStatusProvider() {
        return {
          authenticated: true,
          message: "Logged in using an API key",
        };
      },
      envLookup() {
        return "test-token";
      },
    });

    const response = await app.request("/bridge/execute", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-incompatible-runtime-provider",
        session_id: "session-incompatible-runtime-provider",
        runtime: "codex",
        provider: "anthropic",
        model: "gpt-5-codex",
        prompt: "Run with an incompatible runtime/provider pair.",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-incompatible-runtime-provider",
        system_prompt: "",
        max_turns: 8,
        budget_usd: 2,
        allowed_tools: ["Read"],
        permission_mode: "default",
      }),
    });

    expect(response.status).toBe(400);
    expect(await response.json()).toMatchObject({
      error: "Runtime codex is incompatible with provider anthropic",
    });
    expect(pool.get("task-incompatible-runtime-provider")).toBeUndefined();
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

  test("pauses a runtime into a resumable snapshot and resumes it with persisted continuity", async () => {
    const pool = new RuntimePoolManager(1);
    const sessionManager = new SessionManager();
    const executedPrompts: string[] = [];

    const app = createApp({
      pool,
      sessionManager,
      queryRunner: async function* ({ prompt, options }) {
        executedPrompts.push(prompt);
        const abortController = options?.abortController as AbortController | undefined;

        yield {
          type: "assistant",
          session_id: "session-pause",
          message: {
            content: [{ type: "text", text: `Working on ${prompt}` }],
          },
        };

        while (!abortController?.signal.aborted) {
          await Bun.sleep(5);
        }

        if (abortController.signal.reason === "paused_by_user") {
          throw new Error("paused by user");
        }

        yield {
          type: "result",
          session_id: "session-pause",
          subtype: "success",
          result: "Resumed successfully",
          stop_reason: "end_turn",
          total_cost_usd: 0.02,
          usage: {
            input_tokens: 50,
            output_tokens: 25,
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

    const executeResponse = await app.request("/bridge/execute", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-pause",
        session_id: "session-pause",
        prompt: "Pause and resume the runtime",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-pause",
        system_prompt: "",
        max_turns: 8,
        budget_usd: 2,
        allowed_tools: ["Read"],
        permission_mode: "default",
      }),
    });

    expect(executeResponse.status).toBe(200);

    const pauseResponse = await app.request("/bridge/pause", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-pause",
        reason: "user requested pause",
      }),
    });

    expect(pauseResponse.status).toBe(200);
    expect(await pauseResponse.json()).toEqual({
      success: true,
      session_id: "session-pause",
      status: "paused",
    });

    const poolAfterPause = await app.request("/bridge/pool");
    expect(poolAfterPause.status).toBe(200);
    expect(await poolAfterPause.json()).toMatchObject({
      active: 0,
      warm_available: 1,
    });

    await waitFor(() => sessionManager.restore("task-pause")?.status === "paused");
    expect(pool.get("task-pause")).toBeUndefined();

    const resumeResponse = await app.request("/bridge/resume", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-pause",
      }),
    });

    expect(resumeResponse.status).toBe(200);
    expect(await resumeResponse.json()).toEqual({
      session_id: "session-pause",
      resumed: true,
    });
    expect(executedPrompts).toEqual([
      "Pause and resume the runtime",
      "Pause and resume the runtime",
    ]);

    const poolAfterResume = await app.request("/bridge/pool");
    expect(poolAfterResume.status).toBe(200);
    expect(await poolAfterResume.json()).toMatchObject({
      active: 1,
      warm_available: 0,
      warm_reuse_hits: 1,
    });
  });

  test("rejects Claude resume when only a legacy request snapshot is available", async () => {
    const sessionManager = new SessionManager();
    sessionManager.save("task-legacy-claude", {
      task_id: "task-legacy-claude",
      session_id: "session-legacy-claude",
      status: "paused",
      turn_number: 2,
      spent_usd: 0.11,
      created_at: 100,
      updated_at: 200,
      request: {
        task_id: "task-legacy-claude",
        session_id: "session-legacy-claude",
        runtime: "claude_code",
        provider: "anthropic",
        model: "claude-sonnet-4-5",
        prompt: "Resume a legacy Claude snapshot",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-legacy-claude",
        system_prompt: "",
        max_turns: 8,
        budget_usd: 2,
        allowed_tools: ["Read"],
        permission_mode: "default",
      },
      continuity: {
        runtime: "claude_code",
        resume_ready: false,
        captured_at: 200,
        blocking_reason: "missing_continuity_state",
      },
    });

    const app = createApp({
      sessionManager,
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const response = await app.request("/bridge/resume", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-legacy-claude",
      }),
    });

    expect(response.status).toBe(409);
    expect(await response.json()).toEqual({
      error: "Claude Code continuity state is not resumable for task task-legacy-claude",
      code: "missing_continuity_state",
    });
  });

  test("rejects Codex resume when continuity metadata is missing or not resumable", async () => {
    const sessionManager = new SessionManager();
    sessionManager.save("task-legacy-codex", {
      task_id: "task-legacy-codex",
      session_id: "session-legacy-codex",
      status: "paused",
      turn_number: 2,
      spent_usd: 0.11,
      created_at: 100,
      updated_at: 200,
      request: {
        task_id: "task-legacy-codex",
        session_id: "session-legacy-codex",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        prompt: "Resume a legacy Codex snapshot",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-legacy-codex",
        system_prompt: "",
        max_turns: 8,
        budget_usd: 2,
        allowed_tools: ["Read"],
        permission_mode: "default",
      },
    });

    const app = createApp({
      sessionManager,
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const response = await app.request("/bridge/resume", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-legacy-codex",
      }),
    });

    expect(response.status).toBe(409);
    expect(await response.json()).toEqual({
      error: "Codex continuity state is not resumable for task task-legacy-codex",
      code: "missing_continuity_state",
    });
  });

  test("rejects OpenCode resume when continuity metadata is missing", async () => {
    const sessionManager = new SessionManager();
    sessionManager.save("task-legacy-opencode", {
      task_id: "task-legacy-opencode",
      session_id: "session-legacy-opencode",
      status: "paused",
      turn_number: 2,
      spent_usd: 0.11,
      created_at: 100,
      updated_at: 200,
      request: {
        task_id: "task-legacy-opencode",
        session_id: "session-legacy-opencode",
        runtime: "opencode",
        provider: "opencode",
        model: "opencode-default",
        prompt: "Resume a legacy OpenCode snapshot",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-legacy-opencode",
        system_prompt: "",
        max_turns: 8,
        budget_usd: 2,
        allowed_tools: ["Read"],
        permission_mode: "default",
      },
    });

    const app = createApp({
      sessionManager,
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const response = await app.request("/bridge/resume", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-legacy-opencode",
      }),
    });

    expect(response.status).toBe(409);
    expect(await response.json()).toEqual({
      error: "OpenCode continuity state is not resumable for task task-legacy-opencode",
      code: "missing_continuity_state",
    });
  });

  test("pauses and resumes OpenCode through the same upstream session", async () => {
    const pool = new RuntimePoolManager(1);
    const sessionManager = new SessionManager();
    const calls: Array<{ kind: string; payload: unknown }> = [];

    const app = createApp({
      pool,
      sessionManager,
      opencodeTransport: {
        async createSession(input: { title?: string }) {
          calls.push({ kind: "createSession", payload: input });
          return { id: "opencode-session-123" };
        },
        async sendPromptAsync(input: { sessionId: string; prompt: string; provider: string; model?: string }) {
          calls.push({ kind: "sendPromptAsync", payload: input });
        },
        async abortSession(sessionId: string) {
          calls.push({ kind: "abortSession", payload: { sessionId } });
          return true;
        },
        checkReadiness() {
          return Promise.resolve({ ok: true, diagnostics: [] });
        },
      } as never,
      opencodeEventRunner: async function* (params) {
        calls.push({
          kind: "eventRunner",
          payload: { mode: params.mode, sessionId: params.sessionId, prompt: params.prompt },
        });
        yield {
          event: "message.part.delta",
          data: {
            sessionID: params.sessionId,
            part: { type: "text", text: "OpenCode is working." },
          },
        };

        if (params.mode === "start") {
          while (!params.abortSignal.aborted) {
            await Bun.sleep(5);
          }
          return;
        }

        yield {
          event: "session.idle",
          data: {
            sessionID: params.sessionId,
          },
        };
      },
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const executeResponse = await app.request("/bridge/execute", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-opencode-pause",
        session_id: "session-opencode-pause",
        runtime: "opencode",
        provider: "opencode",
        model: "opencode-default",
        prompt: "Pause and resume OpenCode",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-opencode-pause",
        system_prompt: "",
        max_turns: 8,
        budget_usd: 2,
        allowed_tools: ["Read"],
        permission_mode: "default",
      }),
    });

    expect(executeResponse.status).toBe(200);

    const pauseResponse = await app.request("/bridge/pause", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-opencode-pause",
        reason: "pause openCode",
      }),
    });

    expect(pauseResponse.status).toBe(200);
    await waitFor(
      () => sessionManager.restore("task-opencode-pause")?.status === "paused",
      1000,
    );
    expect(sessionManager.restore("task-opencode-pause")).toMatchObject({
      continuity: {
        runtime: "opencode",
        resume_ready: true,
        upstream_session_id: "opencode-session-123",
      },
    });

    const resumeResponse = await app.request("/bridge/resume", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-opencode-pause",
      }),
    });

    expect(resumeResponse.status).toBe(200);
    expect(await resumeResponse.json()).toEqual({
      session_id: "session-opencode-pause",
      resumed: true,
    });

    expect(calls).toMatchObject([
      { kind: "createSession", payload: { title: "task-opencode-pause" } },
      {
        kind: "sendPromptAsync",
        payload: {
          sessionId: "opencode-session-123",
          prompt: "Pause and resume OpenCode",
          provider: "opencode",
          model: "opencode-default",
        },
      },
      {
        kind: "eventRunner",
        payload: {
          mode: "start",
          sessionId: "opencode-session-123",
          prompt: "Pause and resume OpenCode",
        },
      },
      { kind: "abortSession", payload: { sessionId: "opencode-session-123" } },
      {
        kind: "sendPromptAsync",
        payload: {
          sessionId: "opencode-session-123",
          provider: "opencode",
          model: "opencode-default",
        },
      },
      {
        kind: "eventRunner",
        payload: {
          mode: "resume",
          sessionId: "opencode-session-123",
        },
      },
    ]);
    expect(String((calls[4] as { payload: { prompt: string } }).payload.prompt)).toContain(
      "Continue",
    );
  });

  test("cancel drops resumable OpenCode continuity after aborting the upstream session", async () => {
    const pool = new RuntimePoolManager(1);
    const sessionManager = new SessionManager();
    const calls: Array<{ kind: string; payload: unknown }> = [];

    const app = createApp({
      pool,
      sessionManager,
      opencodeTransport: {
        async createSession() {
          return { id: "opencode-session-cancel" };
        },
        async sendPromptAsync() {},
        async abortSession(sessionId: string) {
          calls.push({ kind: "abortSession", payload: { sessionId } });
          return true;
        },
        checkReadiness() {
          return Promise.resolve({ ok: true, diagnostics: [] });
        },
      } as never,
      opencodeEventRunner: async function* (params) {
        yield {
          event: "message.part.delta",
          data: {
            sessionID: params.sessionId,
            part: { type: "text", text: "OpenCode is running." },
          },
        };
        while (!params.abortSignal.aborted) {
          await Bun.sleep(5);
        }
      },
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    await app.request("/bridge/execute", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-opencode-cancel",
        session_id: "session-opencode-cancel",
        runtime: "opencode",
        provider: "opencode",
        model: "opencode-default",
        prompt: "Cancel OpenCode",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-opencode-cancel",
        system_prompt: "",
        max_turns: 8,
        budget_usd: 2,
        allowed_tools: ["Read"],
        permission_mode: "default",
      }),
    });

    const cancelResponse = await app.request("/bridge/cancel", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-opencode-cancel",
        reason: "cancel openCode",
      }),
    });

    expect(cancelResponse.status).toBe(200);
    await waitFor(
      () => sessionManager.restore("task-opencode-cancel")?.status === "cancelled",
      1000,
    );
    expect(calls).toEqual([{ kind: "abortSession", payload: { sessionId: "opencode-session-cancel" } }]);
    expect(sessionManager.restore("task-opencode-cancel")).toMatchObject({
      continuity: {
        runtime: "opencode",
        resume_ready: false,
        blocking_reason: "continuity_not_supported",
      },
    });
  });

  test("rejects resume for paused CLI-backed runtimes that do not support truthful continuity", async () => {
    const pool = new RuntimePoolManager(1);
    const sessionManager = new SessionManager();

    const app = createApp({
      pool,
      sessionManager,
      commandRuntimeRunner: async function* () {
        yield {
          type: "assistant_text",
          content: "Cursor is working.",
        };
        while (true) {
          await Bun.sleep(5);
        }
      },
      executableLookup(command) {
        return command === "cursor-agent" ? "C:/mock/cursor-agent.exe" : null;
      },
      envLookup(name) {
        return name === "CURSOR_API_KEY" ? "cursor-token" : undefined;
      },
      streamer: {
        close() {},
        connect() {},
        send() {},
      } as never,
    });

    const executeResponse = await app.request("/bridge/execute", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-cursor-pause",
        session_id: "session-cursor-pause",
        runtime: "cursor",
        provider: "cursor",
        model: "claude-sonnet-4-20250514",
        prompt: "Pause and resume Cursor",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-cursor-pause",
        system_prompt: "",
        max_turns: 8,
        budget_usd: 2,
        allowed_tools: ["Read"],
        permission_mode: "default",
      }),
    });

    expect(executeResponse.status).toBe(200);

    const pauseResponse = await app.request("/bridge/pause", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-cursor-pause",
        reason: "pause cursor",
      }),
    });
    expect(pauseResponse.status).toBe(200);

    await waitFor(() => sessionManager.restore("task-cursor-pause")?.status === "paused");
    expect(sessionManager.restore("task-cursor-pause")).toMatchObject({
      continuity: {
        runtime: "cursor",
        resume_ready: false,
        blocking_reason: "continuity_not_supported",
      },
    });

    const resumeResponse = await app.request("/bridge/resume", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        task_id: "task-cursor-pause",
      }),
    });

    expect(resumeResponse.status).toBe(409);
    expect(await resumeResponse.json()).toEqual({
      error: "Cursor Agent continuity state is not resumable for task task-cursor-pause",
      code: "continuity_not_supported",
    });
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
