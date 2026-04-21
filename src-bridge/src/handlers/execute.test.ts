import { describe, expect, test } from "bun:test";
import { handleExecute } from "./execute.js";
import type { AgentRuntime } from "../runtime/agent-runtime.js";
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
