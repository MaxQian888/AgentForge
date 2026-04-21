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
      executableLookup() {
        return null;
      },
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
          interaction_capabilities: expect.objectContaining({
            inputs: expect.objectContaining({
              structured_output: expect.objectContaining({
                state: "degraded",
                reason_code: "missing_credentials",
              }),
            }),
          }),
          available: false,
        }),
        expect.objectContaining({
          key: "codex",
          default_provider: "openai",
          compatible_providers: ["openai", "codex"],
          model_options: expect.arrayContaining(["gpt-5-codex", "o3"]),
          supported_features: expect.arrayContaining(["reasoning", "output_schema"]),
          interaction_capabilities: expect.objectContaining({
            mcp: expect.objectContaining({
              config_overlay: expect.objectContaining({
                state: "supported",
              }),
            }),
          }),
          available: true,
        }),
        expect.objectContaining({
          key: "cursor",
          default_provider: "cursor",
          compatible_providers: ["cursor"],
          model_options: expect.arrayContaining(["claude-sonnet-4-20250514", "gpt-4o"]),
        }),
        expect.objectContaining({
          key: "opencode",
          interaction_capabilities: expect.objectContaining({
            lifecycle: expect.objectContaining({
              shell: expect.objectContaining({
                state: "degraded",
                reason_code: "missing_server_url",
              }),
            }),
          }),
        }),
      ]),
    });
  });

  test("rejects execute requests for providers without agent execution support", async () => {
    const pool = new RuntimePoolManager(1);
    const app = createApp({
      pool,
      awaitExecution: true,
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

  test("rejects resume when the provided runtime context drifts from the persisted snapshot", async () => {
    const sessionManager = new SessionManager();
    sessionManager.save("task-context-drift", {
      task_id: "task-context-drift",
      session_id: "session-context-drift",
      status: "paused",
      turn_number: 2,
      spent_usd: 0.11,
      created_at: 100,
      updated_at: 200,
      request: {
        task_id: "task-context-drift",
        session_id: "session-context-drift",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        prompt: "Resume codex task",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-context-drift",
        system_prompt: "",
        max_turns: 8,
        budget_usd: 2,
        allowed_tools: ["Read"],
        permission_mode: "default",
        team_id: "team-123",
        team_role: "reviewer",
      },
      continuity: {
        runtime: "codex",
        resume_ready: true,
        captured_at: 200,
        thread_id: "thread-123",
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
        task_id: "task-context-drift",
        runtime: "opencode",
        provider: "openai",
        model: "gpt-5-codex",
        team_id: "team-123",
        team_role: "reviewer",
      }),
    });

    expect(response.status).toBe(409);
    expect(await response.json()).toEqual({
      error: "Resume request context mismatch for runtime: expected codex, got opencode",
      code: "resume_context_mismatch",
      field: "runtime",
    });
  });

});
