import { describe, expect, test } from "bun:test";
import { calculateCost } from "../cost/calculator.js";
import { streamCommandRuntime } from "./command-runtime.js";
import { AgentRuntime } from "../runtime/agent-runtime.js";
import type { ExecuteRequest } from "../types.js";

function createRequest(overrides: Partial<ExecuteRequest> = {}): ExecuteRequest {
  return {
    task_id: "task-123",
    session_id: "session-123",
    runtime: "codex",
    provider: "codex",
    model: "gpt-5-codex",
    prompt: "Inspect the bridge task.",
    worktree_path: "D:/Project/AgentForge",
    branch_name: "agent/task-123",
    system_prompt: "",
    max_turns: 8,
    budget_usd: 5,
    allowed_tools: ["Read"],
    permission_mode: "default",
    ...overrides,
  };
}

function createStream(chunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  return new ReadableStream<Uint8Array>({
    start(controller) {
      for (const chunk of chunks) {
        controller.enqueue(encoder.encode(chunk));
      }
      controller.close();
    },
  });
}

describe("streamCommandRuntime", () => {
  test("normalizes command runtime events and updates runtime state", async () => {
    const req = createRequest();
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);
    const events: Array<{ type: string; data: unknown }> = [];

    await streamCommandRuntime(
      runtime,
      {
        send(event) {
          events.push(event);
        },
      },
      req,
      "Follow the repo instructions closely.",
      {
        command: "codex",
        now: () => 1_700_000_000_000,
        async *commandRuntimeRunner() {
          yield { type: "assistant_text", content: "Planning bridge work." };
          yield {
            type: "tool_call",
            tool_name: "Read",
            tool_input: { file_path: "README.md" },
            call_id: "call-1",
          };
          yield {
            type: "tool_result",
            call_id: "call-1",
            output: { ok: true },
            is_error: false,
          };
          yield {
            type: "usage",
            input_tokens: 120,
            output_tokens: 45,
            cache_read_tokens: 5,
            cost_usd: 0.03,
          };
        },
      },
    );

    expect(events.map((event) => event.type)).toEqual([
      "output",
      "tool_call",
      "tool_result",
      "cost_update",
    ]);
    expect(events[0]).toMatchObject({
      data: {
        content: "Planning bridge work.",
        turn_number: 0,
      },
    });
    expect(events[1]).toMatchObject({
      data: {
        tool_name: "Read",
        tool_input: JSON.stringify({ file_path: "README.md" }),
        call_id: "call-1",
      },
    });
    expect(events[2]).toMatchObject({
      data: {
        call_id: "call-1",
        output: JSON.stringify({ ok: true }),
        is_error: false,
      },
    });
    expect(events[3]).toMatchObject({
      data: {
        input_tokens: 120,
        output_tokens: 45,
        cache_read_tokens: 5,
        cost_usd: 0.03,
        budget_remaining_usd: 4.97,
        turn_number: 1,
      },
    });
    expect(runtime.turnNumber).toBe(1);
    expect(runtime.lastTool).toBe("Read");
    expect(runtime.spentUsd).toBe(0.03);
    expect(runtime.lastActivity).toBe(1_700_000_000_000);
  });

  test("falls back to calculated cost and treats unknown content events as assistant output", async () => {
    const req = createRequest({ model: "gpt-5" });
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);
    const events: Array<{ type: string; data: unknown }> = [];

    await streamCommandRuntime(
      runtime,
      {
        send(event) {
          events.push(event);
        },
      },
      req,
      "No-op system prompt.",
      {
        command: "codex",
        now: () => 42,
        async *commandRuntimeRunner() {
          yield { type: "log", content: "Raw fallback output." };
          yield {
            type: "usage",
            input_tokens: 1_000,
            output_tokens: 500,
            cache_read_tokens: -10,
          };
        },
      },
    );

    expect(events.map((event) => event.type)).toEqual(["output", "cost_update"]);
    expect(events[0]).toMatchObject({
      data: {
        content: "Raw fallback output.",
      },
    });
    expect(events[1]).toMatchObject({
      data: {
        input_tokens: 1_000,
        output_tokens: 500,
        cache_read_tokens: 0,
        cost_usd: calculateCost(
          {
            input_tokens: 1_000,
            output_tokens: 500,
            cache_read_input_tokens: 0,
          },
          "gpt-5",
        ),
      },
    });
  });

  test("aborts the runtime when spending reaches the local budget", async () => {
    const req = createRequest({ budget_usd: 0.01 });
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);

    await expect(
      streamCommandRuntime(
        runtime,
        {
          send() {},
        },
        req,
        "Budget sensitive run.",
        {
          command: "codex",
          async *commandRuntimeRunner() {
            yield {
              type: "usage",
              input_tokens: 10,
              output_tokens: 10,
              cache_read_tokens: 0,
              cost_usd: 0.02,
            };
          },
        },
      ),
    ).rejects.toThrow("budget exceeded for task task-123");

    expect(runtime.abortController.signal.aborted).toBe(true);
    expect(runtime.abortController.signal.reason).toBe("budget_exceeded");
  });

  test("throws structured runtime errors immediately", async () => {
    const req = createRequest();
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);

    await expect(
      streamCommandRuntime(
        runtime,
        {
          send() {},
        },
        req,
        "Error path.",
        {
          command: "codex",
          async *commandRuntimeRunner() {
            yield {
              type: "error",
              message: "codex runtime crashed",
            };
          },
        },
      ),
    ).rejects.toThrow("codex runtime crashed");
  });

  test("ignores malformed events that do not map to bridge output", async () => {
    const req = createRequest();
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);
    const events: Array<{ type: string; data: unknown }> = [];

    await streamCommandRuntime(
      runtime,
      {
        send(event) {
          events.push(event);
        },
      },
      req,
      "Malformed event path.",
      {
        command: "codex",
        now: () => 111,
        async *commandRuntimeRunner() {
          yield { type: "assistant_text", content: "" };
          yield { type: "tool_call", tool_input: { file_path: "README.md" } };
          yield { type: "error", message: { detail: "not-a-string" } };
          yield { type: "unknown", content: "" };
        },
      },
    );

    expect(events).toEqual([]);
    expect(runtime.turnNumber).toBe(0);
    expect(runtime.lastTool).toBe("");
    expect(runtime.lastActivity).toBe(111);
  });

  test("uses the default Bun spawn runner, forwards stdin/env, and normalizes stdout lines", async () => {
    const req = createRequest({
      runtime: "opencode",
      provider: "opencode",
      model: "opencode-default",
      permission_mode: "bypassPermissions",
    });
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);
    const events: Array<{ type: string; data: unknown }> = [];
    const stdinWrites: string[] = [];
    let stdinEnded = false;
    let spawnParams:
      | {
          cmd?: string[];
          cwd?: string;
          env?: Record<string, string | undefined>;
        }
      | undefined;
    const originalSpawn = Bun.spawn;

    (Bun as unknown as { spawn: typeof Bun.spawn }).spawn = ((params: unknown) => {
      spawnParams = params as {
        cmd?: string[];
        cwd?: string;
        env?: Record<string, string | undefined>;
      };
      return {
        stdin: {
          write(chunk: string) {
            stdinWrites.push(chunk);
          },
          end() {
            stdinEnded = true;
          },
        },
        stdout: createStream([
          "{\"type\":\"assistant_text\",\"content\":\"Spawned output.\"}\nplain ",
          "fallback output\n",
          "{\"type\":\"tool_call\",\"tool_name\":\"Read\",\"tool_input\":{\"file_path\":\"README.md\"},\"call_id\":\"call-spawn\"}\n",
          "{\"type\":\"tool_result\",\"call_id\":\"call-spawn\",\"output\":\"done\",\"is_error\":false}\n",
          "{\"type\":\"usage\",\"input_tokens\":20,\"output_tokens\":10,\"cache_read_tokens\":2,\"cost_usd\":0.01}\n",
        ]),
        stderr: createStream([]),
        exited: Promise.resolve(0),
        kill() {},
      } as never;
    }) as typeof Bun.spawn;

    try {
      await streamCommandRuntime(
        runtime,
        {
          send(event) {
            events.push(event);
          },
        },
        req,
        "Use the default command runtime.",
        {
          command: "opencode",
          now: () => 2_000,
        },
      );
    } finally {
      (Bun as unknown as { spawn: typeof Bun.spawn }).spawn = originalSpawn;
    }

    expect(spawnParams).toMatchObject({
      cmd: ["opencode"],
      cwd: "D:/Project/AgentForge",
      env: expect.objectContaining({
        AGENTFORGE_RUNTIME: "opencode",
        AGENTFORGE_MODEL: "opencode-default",
        AGENTFORGE_PERMISSION_MODE: "bypassPermissions",
      }),
    });
    expect(stdinEnded).toBe(true);
    expect(stdinWrites).toHaveLength(1);
    expect(JSON.parse(stdinWrites[0] ?? "")).toMatchObject({
      task_id: "task-123",
      session_id: "session-123",
      prompt: "Inspect the bridge task.",
      system_prompt: "Use the default command runtime.",
      worktree_path: "D:/Project/AgentForge",
      branch_name: "agent/task-123",
      model: "opencode-default",
      permission_mode: "bypassPermissions",
      budget_usd: 5,
      max_turns: 8,
    });
    expect(events.map((event) => event.type)).toEqual([
      "output",
      "output",
      "tool_call",
      "tool_result",
      "cost_update",
    ]);
    expect(events[1]).toMatchObject({
      data: {
        content: "plain fallback output",
      },
    });
    expect(events[3]).toMatchObject({
      data: {
        call_id: "call-spawn",
        output: "done",
        is_error: false,
      },
    });
    expect(runtime.spentUsd).toBe(0.01);
    expect(runtime.lastTool).toBe("Read");
    expect(runtime.turnNumber).toBe(1);
    expect(runtime.lastActivity).toBe(2_000);
  });

  test("kills the spawned process when the default runner aborts on budget exceedance", async () => {
    const req = createRequest({ budget_usd: 0.01 });
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);
    let killCalled = false;
    const originalSpawn = Bun.spawn;

    (Bun as unknown as { spawn: typeof Bun.spawn }).spawn = (() => {
      return {
        stdin: {
          write() {},
          end() {},
        },
        stdout: createStream([
          "{\"type\":\"usage\",\"input_tokens\":1,\"output_tokens\":1,\"cache_read_tokens\":0,\"cost_usd\":0.02}\n",
        ]),
        stderr: createStream([]),
        exited: Promise.resolve(0),
        kill() {
          killCalled = true;
        },
      } as never;
    }) as typeof Bun.spawn;

    try {
      await expect(
        streamCommandRuntime(
          runtime,
          {
            send() {},
          },
          req,
          "Abort on budget.",
          {
            command: "codex",
          },
        ),
      ).rejects.toThrow("budget exceeded for task task-123");
    } finally {
      (Bun as unknown as { spawn: typeof Bun.spawn }).spawn = originalSpawn;
    }

    expect(killCalled).toBe(true);
    expect(runtime.abortController.signal.aborted).toBe(true);
  });

  test("surfaces stderr when the spawned command exits non-zero without an abort", async () => {
    const req = createRequest();
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);
    const originalSpawn = Bun.spawn;

    (Bun as unknown as { spawn: typeof Bun.spawn }).spawn = (() => {
      return {
        stdin: {
          write() {},
          end() {},
        },
        stdout: createStream([]),
        stderr: createStream(["runtime failed", "\nwith stderr details"]),
        exited: Promise.resolve(23),
        kill() {},
      } as never;
    }) as typeof Bun.spawn;

    try {
      await expect(
        streamCommandRuntime(
          runtime,
          {
            send() {},
          },
          req,
          "Exit error path.",
          {
            command: "codex",
          },
        ),
      ).rejects.toThrow("runtime failedwith stderr details");
    } finally {
      (Bun as unknown as { spawn: typeof Bun.spawn }).spawn = originalSpawn;
    }
  });
});
