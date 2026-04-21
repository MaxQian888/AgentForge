import { describe, expect, test } from "bun:test";
import { handleExecute } from "./execute.js";
import type { AgentRuntime } from "../runtime/agent-runtime.js";
import { RuntimePoolManager } from "../runtime/pool-manager.js";
import { SessionManager } from "../session/manager.js";
import type { ExecuteRequest } from "../types.js";

/**
 * Returns "0" for all BRIDGE_ACP_* env flags, forcing every adapter to take
 * the legacy path. Used in tests that mock legacy runners and should not
 * attempt to spawn real ACP processes.
 */
function noAcpEnvLookup(name: string): string | undefined {
  if (name.startsWith("BRIDGE_ACP_")) return "0";
  return undefined;
}

function createRequest(overrides: Partial<ExecuteRequest> = {}): ExecuteRequest {
  return {
    task_id: "task-123",
    session_id: "session-123",
    prompt: "Inspect the repository and implement the requested change.",
    worktree_path: "D:/Project/AgentForge",
    branch_name: "agent/task-123",
    system_prompt: "",
    max_turns: 12,
    budget_usd: 5,
    allowed_tools: ["Read", "Edit"],
    permission_mode: "default",
    runtime: "claude_code",
    ...overrides,
  };
}

describe("handleExecute", () => {
  test("runs a real query runner and normalizes bridge events", async () => {
    const pool = new RuntimePoolManager(2);
    const sessionManager = new SessionManager();
    const events: Array<{ type: string; data: unknown }> = [];
    const request = createRequest();
    let runnerInvocation:
      | {
          prompt: string;
          options?: Record<string, unknown>;
        }
      | undefined;

    async function* queryRunner(params: {
      prompt: string;
      options?: Record<string, unknown>;
    }): AsyncGenerator<Record<string, unknown>, void> {
      runnerInvocation = params;

      yield {
        type: "assistant",
        session_id: request.session_id,
        message: {
          content: [
            { type: "text", text: "Analyzing the codebase." },
            { type: "tool_use", id: "call-1", name: "Read", input: { file_path: "README.md" } },
          ],
        },
      };

      yield {
        type: "user",
        session_id: request.session_id,
        parent_tool_use_id: "call-1",
        tool_use_result: {
          tool_use_id: "call-1",
          output: "# README",
          is_error: false,
        },
      };

      yield {
        type: "result",
        session_id: request.session_id,
        subtype: "success",
        result: "Done",
        stop_reason: "end_turn",
        total_cost_usd: 0.02,
        usage: {
          input_tokens: 5_000,
          output_tokens: 1_000,
          cache_read_input_tokens: 0,
        },
      };
    }

    const streamer = {
      send(event: { type: string; data: unknown }) {
        events.push(event);
      },
    };

    const response = await handleExecute(pool, streamer as never, request, {
      queryRunner,
      sessionManager,
      awaitCompletion: true,
      envLookup: noAcpEnvLookup,
    });

    expect(response).toEqual({ session_id: request.session_id });
    expect(runnerInvocation).toMatchObject({
      prompt: request.prompt,
      options: {
        cwd: request.worktree_path,
        maxTurns: request.max_turns,
        allowedTools: request.allowed_tools,
        permissionMode: request.permission_mode,
      },
    });

    expect(events.map((event) => event.type)).toEqual([
      "status_change",
      "status_change",
      "output",
      "tool_call",
      "tool_result",
      "cost_update",
      "snapshot",
      "status_change",
    ]);

    const snapshotEvent = events.find((event) => event.type === "snapshot");
    expect(snapshotEvent).toBeDefined();
    expect(sessionManager.restore(request.task_id)).toMatchObject({
      task_id: request.task_id,
      session_id: request.session_id,
      status: "completed",
    });
    expect(pool.get(request.task_id)).toBeUndefined();
  });

  test("includes structured output on the completed status change event", async () => {
    const pool = new RuntimePoolManager(1);
    const events: Array<{ type: string; data: unknown }> = [];

    await handleExecute(
      pool,
      {
        send(event: { type: string; data: unknown }) {
          events.push(event);
        },
      } as never,
      createRequest({
        task_id: "task-structured",
        session_id: "session-structured",
        output_schema: {
          type: "json_schema",
          schema: {
            type: "object",
            properties: {
              summary: { type: "string" },
            },
          },
        },
      }),
      {
        awaitCompletion: true,
        envLookup: noAcpEnvLookup,
        queryRunner: async function* () {
          yield {
            type: "result",
            session_id: "session-structured",
            subtype: "success",
            result: "Done",
            structured_output: {
              summary: "Structured done",
            },
            stop_reason: "end_turn",
            total_cost_usd: 0.01,
            usage: {
              input_tokens: 100,
              output_tokens: 50,
              cache_read_input_tokens: 0,
            },
          };
        },
      },
    );

    expect(events.at(-1)).toMatchObject({
      type: "status_change",
      data: {
        new_status: "completed",
        structured_output: {
          summary: "Structured done",
        },
      },
    });
  });

  test("aborts execution when the local budget is exceeded and persists continuity state", async () => {
    const pool = new RuntimePoolManager(2);
    const sessionManager = new SessionManager();
    const events: Array<{ type: string; data: unknown }> = [];
    const request = createRequest({ budget_usd: 0.000001 });

    async function* queryRunner(): AsyncGenerator<Record<string, unknown>, void> {
      yield {
        type: "assistant",
        session_id: request.session_id,
        message: {
          content: [{ type: "text", text: "Starting expensive work." }],
        },
        usage: {
          input_tokens: 2_000,
          output_tokens: 2_000,
          cache_read_input_tokens: 0,
        },
      };
    }

    const streamer = {
      send(event: { type: string; data: unknown }) {
        events.push(event);
      },
    };

    await handleExecute(pool, streamer as never, request, {
      queryRunner,
      sessionManager,
      awaitCompletion: true,
      envLookup: noAcpEnvLookup,
    });

    expect(events.map((event) => event.type)).toEqual([
      "status_change",
      "status_change",
      "output",
      "cost_update",
      "budget_alert",
      "error",
      "snapshot",
      "status_change",
    ]);

    expect(sessionManager.restore(request.task_id)).toMatchObject({
      task_id: request.task_id,
      session_id: request.session_id,
      status: "budget_exceeded",
    });
    expect(pool.get(request.task_id)).toBeUndefined();
  });

  test("rejects unknown runtimes before acquiring pool state", async () => {
    const pool = new RuntimePoolManager(1);
    const streamer = {
      send() {},
    };

    await expect(
      handleExecute(pool, streamer as never, createRequest({ runtime: "unknown_runtime" as never }), {
        awaitCompletion: true,
      }),
    ).rejects.toThrow("Unknown runtime: unknown_runtime");

    expect(pool.get("task-123")).toBeUndefined();
  });

  test("routes codex requests through the dedicated codex runtime adapter", async () => {
    const pool = new RuntimePoolManager(2);
    const sessionManager = new SessionManager();
    const events: Array<{ type: string; data: unknown }> = [];
    const request = createRequest({
      task_id: "task-codex",
      session_id: "session-codex",
      runtime: "codex",
      provider: "codex",
      model: "gpt-5-codex",
    });
    let invocation:
      | {
          mode: "start" | "resume";
          threadId?: string;
          systemPrompt: string;
          req: ExecuteRequest;
        }
      | undefined;

    async function* codexRuntimeRunner(params: {
      mode: "start" | "resume";
      threadId?: string;
      systemPrompt: string;
      req: ExecuteRequest;
    }): AsyncGenerator<Record<string, unknown>, void> {
      invocation = params;

      yield { type: "thread.started", thread_id: "thread-codex-1" };
      yield {
        type: "item.completed",
        item: {
          id: "item-codex-0",
          type: "agent_message",
          text: "Planning Codex work.",
        },
      };

      yield {
        type: "item.started",
        item: {
          id: "codex-call-1",
          type: "command_execution",
          command: "Get-Content README.md",
          aggregated_output: "",
          exit_code: null,
          status: "in_progress",
        },
      };

      yield {
        type: "item.completed",
        item: {
          id: "codex-call-1",
          type: "command_execution",
          command: "Get-Content README.md",
          aggregated_output: "# README",
          exit_code: 0,
          status: "completed",
        },
      };

      yield {
        type: "turn.completed",
        usage: {
          input_tokens: 120,
          cached_input_tokens: 0,
          output_tokens: 45,
        },
        total_cost_usd: 0.03,
      };
    }

    const streamer = {
      send(event: { type: string; data: unknown }) {
        events.push(event);
      },
    };

    const response = await handleExecute(pool, streamer as never, request, {
      codexRuntimeRunner,
      awaitCompletion: true,
      envLookup: noAcpEnvLookup,
      executableLookup(command) {
        return `C:/mock/${command}.exe`;
      },
      codexAuthStatusProvider() {
        return {
          authenticated: true,
          message: "Logged in using an API key",
        };
      },
      sessionManager,
    });

    expect(response).toEqual({ session_id: request.session_id });
    expect(invocation).toMatchObject({
      mode: "start",
      req: {
        task_id: request.task_id,
        runtime: "codex",
        provider: "codex",
        model: "gpt-5-codex",
      },
    });
    expect(events.map((event) => event.type)).toEqual([
      "status_change",
      "status_change",
      "output",
      "tool_call",
      "tool_result",
      "cost_update",
      "snapshot",
      "status_change",
    ]);
    expect(sessionManager.restore(request.task_id)).toMatchObject({
      task_id: request.task_id,
      session_id: request.session_id,
      status: "completed",
      continuity: expect.objectContaining({
        runtime: "codex",
        resume_ready: true,
        thread_id: "thread-codex-1",
      }),
    });
  });

  test("passes advanced execute request fields through to the Codex runtime adapter", async () => {
    const pool = new RuntimePoolManager(1);
    let invocation:
      | {
          req: ExecuteRequest;
        }
      | undefined;

    const request = createRequest({
      task_id: "task-codex-advanced",
      session_id: "session-codex-advanced",
      runtime: "codex",
      provider: "codex",
      model: "gpt-5-codex",
      output_schema: {
        type: "json_schema",
        schema: {
          type: "object",
          properties: {
            summary: { type: "string" },
          },
        },
      },
      attachments: [{ type: "image", path: "D:/tmp/screenshot.png" }],
      additional_directories: ["D:/Shared"],
      web_search: true,
      env: { FEATURE_FLAG: "enabled" },
    });

    await handleExecute(
      pool,
      { send() {} } as never,
      request,
      {
        awaitCompletion: true,
        envLookup: noAcpEnvLookup,
        executableLookup(command) {
          return `C:/mock/${command}.exe`;
        },
        codexAuthStatusProvider() {
          return {
            authenticated: true,
            message: "Logged in using an API key",
          };
        },
        codexRuntimeRunner: async function* (params) {
          invocation = params;
          yield { type: "thread.started", thread_id: "thread-codex-advanced" };
          yield { type: "turn.completed", total_cost_usd: 0 };
        },
      },
    );

    expect(invocation?.req.output_schema).toEqual(request.output_schema);
    expect(invocation?.req.attachments).toEqual(request.attachments);
    expect(invocation?.req.additional_directories).toEqual(["D:/Shared"]);
    expect(invocation?.req.web_search).toBe(true);
    expect(invocation?.req.env).toEqual({ FEATURE_FLAG: "enabled" });
  });

  test("routes opencode requests through the dedicated opencode runtime adapter with the same canonical events", async () => {
    const pool = new RuntimePoolManager(2);
    const sessionManager = new SessionManager();
    const events: Array<{ type: string; data: unknown }> = [];
    const request = createRequest({
      task_id: "task-opencode",
      session_id: "session-opencode",
      runtime: "opencode",
      provider: "opencode",
      model: "opencode-default",
    });
    const calls: Array<{ kind: string; payload: unknown }> = [];

    const streamer = {
      send(event: { type: string; data: unknown }) {
        events.push(event);
      },
    };

    const response = await handleExecute(pool, streamer as never, request, {
      awaitCompletion: true,
      envLookup: noAcpEnvLookup,
      opencodeTransport: {
        async createSession(input: { title?: string }) {
          calls.push({ kind: "createSession", payload: input });
          return { id: "opencode-session-123" };
        },
        async sendPromptAsync(input: { sessionId: string; prompt: string; provider: string; model?: string }) {
          calls.push({ kind: "sendPromptAsync", payload: input });
        },
        async abortSession() {
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
            part: {
              type: "text",
              text: "Planning OpenCode work.",
            },
          },
        };
        yield {
          event: "message.part.updated",
          data: {
            sessionID: params.sessionId,
            part: {
              type: "tool",
              id: "tool-1",
              toolName: "Edit",
              state: "running",
              input: { file_path: "README.md" },
            },
          },
        };
        yield {
          event: "message.part.updated",
          data: {
            sessionID: params.sessionId,
            part: {
              type: "tool",
              id: "tool-1",
              toolName: "Edit",
              state: "completed",
              output: "patched",
              isError: false,
            },
          },
        };
        yield {
          event: "session.updated",
          data: {
            sessionID: params.sessionId,
            usage: {
              input_tokens: 140,
              cached_input_tokens: 5,
              output_tokens: 65,
            },
            total_cost_usd: 0.04,
          },
        };
        yield {
          event: "session.idle",
          data: {
            sessionID: params.sessionId,
          },
        };
      },
      sessionManager,
    });

    expect(response).toEqual({ session_id: request.session_id });
    expect(calls).toMatchObject([
      { kind: "createSession" },
      {
        kind: "sendPromptAsync",
        payload: { sessionId: "opencode-session-123", prompt: request.prompt },
      },
      {
        kind: "eventRunner",
        payload: { mode: "start", sessionId: "opencode-session-123" },
      },
    ]);
    expect(events.map((event) => event.type)).toEqual([
      "status_change",
      "status_change",
      "output",
      "tool_call",
      "tool_result",
      "cost_update",
      "snapshot",
      "status_change",
    ]);
    expect(sessionManager.restore(request.task_id)).toMatchObject({
      task_id: request.task_id,
      session_id: request.session_id,
      status: "completed",
      continuity: expect.objectContaining({
        runtime: "opencode",
        resume_ready: true,
        upstream_session_id: "opencode-session-123",
      }),
    });
  });

  test("filters active role-scoped MCP plugins before constructing the runtime adapter", async () => {
    const pool = new RuntimePoolManager(1);
    const request = createRequest({
      task_id: "task-plugin-filter",
      session_id: "session-plugin-filter",
      runtime: "codex",
      provider: "codex",
      model: "gpt-5-codex",
      role_config: {
        role_id: "reviewer",
        name: "Reviewer",
        role: "Senior Reviewer",
        goal: "Review risky changes",
        backstory: "Finds subtle regressions.",
        system_prompt: "Review carefully.",
        allowed_tools: ["Read"],
        max_budget_usd: 5,
        max_turns: 8,
        permission_mode: "default",
        tools: ["plugin-allowed", "plugin-inactive"],
      },
    });
    let activePluginIds: string[] | undefined;

    await handleExecute(
      pool,
      { send() {} } as never,
      request,
      {
        awaitCompletion: true,
        envLookup: noAcpEnvLookup,
        pluginManager: {
          list() {
            return [
              {
                metadata: { id: "plugin-allowed" },
                lifecycle_state: "active",
              },
              {
                metadata: { id: "plugin-inactive" },
                lifecycle_state: "disabled",
              },
              {
                metadata: { id: "plugin-extra" },
                lifecycle_state: "active",
              },
            ];
          },
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
        codexRuntimeRunner: async function* (params) {
          activePluginIds = params.activePlugins?.map((plugin) => plugin.metadata.id);
          yield { type: "thread.started", thread_id: "thread-plugin-filter" };
          yield { type: "turn.completed", total_cost_usd: 0 };
        },
      },
    );

    expect(activePluginIds).toEqual(["plugin-allowed"]);
  });

  test("reports background execution failures when awaitCompletion is disabled", async () => {
    const pool = new RuntimePoolManager(1);
    const request = createRequest({
      task_id: "task-background-error",
      session_id: "session-background-error",
    });
    const events: Array<{ type: string; data: unknown }> = [];

    const response = await handleExecute(
      pool,
      {
        send(event: { type: string; data: unknown }) {
          if (
            event.type === "status_change" &&
            (event.data as { new_status?: string }).new_status === "running"
          ) {
            throw new Error("streamer failed during running transition");
          }
          events.push(event);
        },
      } as never,
      request,
      {
        awaitCompletion: false,
        runtimeRegistry: {
          async resolveExecute(req: ExecuteRequest) {
            return {
              request: req,
              adapter: {
                async execute() {},
              },
            };
          },
        } as never,
        now: () => 99,
      },
    );

    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(response).toEqual({ session_id: request.session_id });
    expect(events).toEqual([
      expect.objectContaining({
        type: "status_change",
        data: { old_status: "idle", new_status: "starting" },
      }),
      expect.objectContaining({
        type: "error",
        data: {
          code: "INTERNAL",
          message: "Error: streamer failed during running transition",
          retryable: false,
        },
      }),
    ]);
    expect(pool.get(request.task_id)).toBeUndefined();
  });

  test("snapshots only while running and classifies paused executions without emitting an error event", async () => {
    const pool = new RuntimePoolManager(1);
    const sessionManager = new SessionManager();
    const request = createRequest({
      task_id: "task-paused",
      session_id: "session-paused",
    });
    const events: Array<{ type: string; data: unknown }> = [];
    const originalSetInterval = globalThis.setInterval;
    const originalClearInterval = globalThis.clearInterval;
    let intervalCallback: (() => void) | undefined;
    const cleared: unknown[] = [];

    globalThis.setInterval = (((callback: TimerHandler) => {
      intervalCallback = callback as () => void;
      return "interval-token" as never;
    }) as unknown as typeof globalThis.setInterval);
    globalThis.clearInterval = (((token?: unknown) => {
      cleared.push(token);
    }) as typeof globalThis.clearInterval);

    try {
      await handleExecute(
        pool,
        {
          send(event: { type: string; data: unknown }) {
            events.push(event);
          },
        } as never,
        request,
        {
          awaitCompletion: true,
          sessionManager,
          now: () => 1234,
          runtimeRegistry: {
            async resolveExecute(req: ExecuteRequest) {
              return {
                request: req,
                adapter: {
                  async execute(runtime: AgentRuntime) {
                    intervalCallback?.();
                    runtime.status = "paused";
                    intervalCallback?.();
                    throw new Error("pause requested");
                  },
                },
              };
            },
          } as never,
        },
      );
    } finally {
      globalThis.setInterval = originalSetInterval;
      globalThis.clearInterval = originalClearInterval;
    }

    expect(events.map((event) => event.type)).toEqual([
      "status_change",
      "status_change",
      "snapshot",
      "snapshot",
      "status_change",
    ]);
    expect(events.some((event) => event.type === "error")).toBe(false);
    expect(cleared).toEqual(["interval-token"]);
    expect(sessionManager.restore(request.task_id)).toMatchObject({
      task_id: request.task_id,
      status: "paused",
    });
  });

  test("classifies cancelled and failed executions truthfully", async () => {
    const cancelledPool = new RuntimePoolManager(1);
    const failedPool = new RuntimePoolManager(1);
    const cancelledSessionManager = new SessionManager();
    const failedSessionManager = new SessionManager();
    const cancelledEvents: Array<{ type: string; data: unknown }> = [];
    const failedEvents: Array<{ type: string; data: unknown }> = [];

    await handleExecute(
      cancelledPool,
      {
        send(event: { type: string; data: unknown }) {
          cancelledEvents.push(event);
        },
      } as never,
      createRequest({
        task_id: "task-cancelled",
        session_id: "session-cancelled",
      }),
      {
        awaitCompletion: true,
        sessionManager: cancelledSessionManager,
        runtimeRegistry: {
          async resolveExecute(req: ExecuteRequest) {
            return {
              request: req,
              adapter: {
                async execute(runtime: AgentRuntime) {
                  runtime.abortController.abort("cancelled_by_user");
                  throw new Error("cancelled");
                },
              },
            };
          },
        } as never,
      },
    );

    await handleExecute(
      failedPool,
      {
        send(event: { type: string; data: unknown }) {
          failedEvents.push(event);
        },
      } as never,
      createRequest({
        task_id: "task-failed",
        session_id: "session-failed",
      }),
      {
        awaitCompletion: true,
        sessionManager: failedSessionManager,
        runtimeRegistry: {
          async resolveExecute(req: ExecuteRequest) {
            return {
              request: req,
              adapter: {
                async execute() {
                  throw new Error("runtime exploded");
                },
              },
            };
          },
        } as never,
      },
    );

    expect(cancelledSessionManager.restore("task-cancelled")).toMatchObject({
      task_id: "task-cancelled",
      status: "cancelled",
    });
    expect(failedSessionManager.restore("task-failed")).toMatchObject({
      task_id: "task-failed",
      status: "failed",
    });
    expect(cancelledEvents.at(-1)).toMatchObject({
      type: "status_change",
      data: { new_status: "cancelled", reason: "cancelled_by_user" },
    });
    expect(failedEvents.at(-1)).toMatchObject({
      type: "status_change",
      data: { new_status: "failed", reason: "runtime_error" },
    });
  });
});
