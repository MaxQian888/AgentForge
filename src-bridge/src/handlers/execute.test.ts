import { describe, expect, test } from "bun:test";
import { handleExecute } from "./execute.js";
import { RuntimePoolManager } from "../runtime/pool-manager.js";
import { SessionManager } from "../session/manager.js";
import type { ExecuteRequest } from "../types.js";

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
    });

    expect(events.map((event) => event.type)).toEqual([
      "status_change",
      "status_change",
      "output",
      "cost_update",
      "cost_update",  // budget warning at 80% threshold
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
});
