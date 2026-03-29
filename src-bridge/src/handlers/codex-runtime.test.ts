import { describe, expect, test } from "bun:test";
import { AgentRuntime } from "../runtime/agent-runtime.js";
import type { ExecuteRequest } from "../types.js";
import { streamCodexRuntime } from "./codex-runtime.js";

function createRequest(overrides: Partial<ExecuteRequest> = {}): ExecuteRequest {
  return {
    task_id: "task-codex",
    session_id: "session-codex",
    runtime: "codex",
    provider: "openai",
    model: "gpt-5-codex",
    prompt: "Inspect the bridge task.",
    worktree_path: "D:/Project/AgentForge",
    branch_name: "agent/task-codex",
    system_prompt: "",
    max_turns: 8,
    budget_usd: 5,
    allowed_tools: ["Read"],
    permission_mode: "default",
    ...overrides,
  };
}

describe("streamCodexRuntime", () => {
  test("normalizes official codex exec JSON events and captures resumable thread continuity", async () => {
    const req = createRequest();
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);
    const events: Array<{ type: string; data: unknown }> = [];

    await streamCodexRuntime(
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
        async *codexRuntimeRunner() {
          yield { type: "thread.started", thread_id: "thread-codex-123" };
          yield { type: "turn.started" };
          yield {
            type: "item.completed",
            item: {
              id: "item-0",
              type: "agent_message",
              text: "Planning Codex bridge work.",
            },
          };
          yield {
            type: "item.started",
            item: {
              id: "item-1",
              type: "command_execution",
              command: "git status",
              aggregated_output: "",
              exit_code: null,
              status: "in_progress",
            },
          };
          yield {
            type: "item.completed",
            item: {
              id: "item-1",
              type: "command_execution",
              command: "git status",
              aggregated_output: "M README.md",
              exit_code: 0,
              status: "completed",
            },
          };
          yield {
            type: "turn.completed",
            usage: {
              input_tokens: 120,
              cached_input_tokens: 30,
              output_tokens: 45,
            },
            total_cost_usd: 0.03,
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
        content: "Planning Codex bridge work.",
      },
    });
    expect(events[1]).toMatchObject({
      data: {
        tool_name: "shell",
        call_id: "item-1",
      },
    });
    expect(events[2]).toMatchObject({
      data: {
        call_id: "item-1",
        output: "M README.md",
        is_error: false,
      },
    });
    expect(events[3]).toMatchObject({
      data: {
        input_tokens: 120,
        cache_read_tokens: 30,
        output_tokens: 45,
        cost_usd: 0.03,
      },
    });
    expect(runtime.continuity).toMatchObject({
      runtime: "codex",
      resume_ready: true,
      thread_id: "thread-codex-123",
    });
  });

  test("resumes against the saved codex thread instead of starting a fresh exec session", async () => {
    const req = createRequest();
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);
    runtime.continuity = {
      runtime: "codex",
      resume_ready: true,
      captured_at: 100,
      thread_id: "thread-codex-continue",
    };

    let invocation:
      | {
          mode: "start" | "resume";
          threadId?: string;
          prompt: string;
        }
      | undefined;

    await streamCodexRuntime(
      runtime,
      {
        send() {},
      },
      req,
      "System prompt",
      {
        command: "codex",
        async *codexRuntimeRunner(params) {
          invocation = params;
          yield { type: "turn.completed", total_cost_usd: 0 };
        },
      },
    );

    expect(invocation).toMatchObject({
      mode: "resume",
      threadId: "thread-codex-continue",
    });
    expect(invocation?.prompt).not.toBe(req.prompt);
    expect(invocation?.prompt).toContain("Continue");
  });
});
