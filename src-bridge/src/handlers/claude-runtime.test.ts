import { describe, expect, test } from "bun:test";
import {
  buildClaudeQueryOptions,
  extractClaudeContinuity,
  persistRuntimeSnapshot,
  streamClaudeRuntime,
} from "./claude-runtime.js";
import { AgentRuntime } from "../runtime/agent-runtime.js";
import { SessionManager } from "../session/manager.js";
import type { ExecuteRequest } from "../types.js";

function createRequest(overrides: Partial<ExecuteRequest> = {}): ExecuteRequest {
  return {
    task_id: "task-123",
    session_id: "session-123",
    prompt: "Implement the requested bridge change.",
    worktree_path: "D:/Project/AgentForge",
    branch_name: "agent/task-123",
    system_prompt: "Base system prompt",
    max_turns: 12,
    budget_usd: 5,
    allowed_tools: ["Read", "Edit"],
    permission_mode: "default",
    ...overrides,
  };
}

function createNow(values: number[]): () => number {
  let index = 0;
  return () => values[index++] ?? values[values.length - 1] ?? 0;
}

describe("claude runtime", () => {
  test("builds query options and enables dangerous bypass only when requested", () => {
    const runtime = new AgentRuntime("task-1", "session-1");
    const defaultOptions = buildClaudeQueryOptions(
      createRequest(),
      "System prompt",
      runtime,
    );
    const bypassOptions = buildClaudeQueryOptions(
      createRequest({
        allowed_tools: [],
        permission_mode: "bypassPermissions",
      }),
      "System prompt",
      runtime,
    );

    expect(defaultOptions).toMatchObject({
      cwd: "D:/Project/AgentForge",
      maxTurns: 12,
      permissionMode: "default",
      systemPrompt: "System prompt",
      allowedTools: ["Read", "Edit"],
    });
    expect(defaultOptions.allowDangerouslySkipPermissions).toBeUndefined();
    expect(bypassOptions.allowedTools).toBeUndefined();
    expect(bypassOptions.allowDangerouslySkipPermissions).toBe(true);
  });

  test("builds a Claude launch tuple with resolved model, budget, and continuity resume inputs", () => {
    const runtime = new AgentRuntime("task-launch", "session-launch");
    const options = buildClaudeQueryOptions(
      createRequest({
        task_id: "task-launch",
        session_id: "session-launch",
        runtime: "claude_code",
        provider: "anthropic",
        model: "claude-sonnet-4-5",
        budget_usd: 7,
      }),
      "System prompt",
      runtime,
      undefined,
      {
        runtime: "claude_code",
        resume_ready: true,
        captured_at: 100,
        session_handle: "claude-session-launch",
        checkpoint_id: "assistant-uuid-1",
      },
    );

    expect(options).toMatchObject({
      model: "claude-sonnet-4-5",
      maxBudgetUsd: 7,
      resume: "claude-session-launch",
      resumeSessionAt: "assistant-uuid-1",
      permissionMode: "default",
    });
  });

  test("streams assistant output, tool events, and usage updates", async () => {
    const runtime = new AgentRuntime("task-2", "session-2");
    const request = createRequest({ task_id: "task-2", session_id: "session-2" });
    const events: Array<{ type: string; data: unknown; timestamp_ms: number }> = [];

    async function* queryRunner(): AsyncGenerator<Record<string, unknown>, void> {
      yield {
        type: "assistant",
        message: {
          content: [
            { type: "text", text: "Analyzing bridge state." },
            { type: "tool_use", id: "tool-1", name: "Read", input: { path: "README.md" } },
          ],
        },
      };

      yield {
        type: "user",
        parent_tool_use_id: "tool-1",
        tool_use_result: {
          content: "tool completed",
          is_error: false,
        },
      };

      yield {
        type: "user",
        tool_use_result: {
          tool_use_id: "tool-2",
          output: { ok: true },
          is_error: true,
        },
      };

      yield {
        type: "result",
        total_cost_usd: 0.25,
        usage: {
          input_tokens: 1_000,
          output_tokens: 500,
          cache_read_input_tokens: 250,
        },
      };
    }

    await streamClaudeRuntime(
      runtime,
      {
        send(event) {
          events.push(event as { type: string; data: unknown; timestamp_ms: number });
        },
      },
      request,
      "Injected system prompt",
      {
        queryRunner,
        now: createNow([101, 102, 103, 104, 105, 106]),
      },
    );

    expect(events.map((event) => event.type)).toEqual([
      "output",
      "tool_call",
      "tool_result",
      "tool_result",
      "cost_update",
    ]);
    expect(events[1]?.data).toMatchObject({
      tool_name: "Read",
      call_id: "tool-1",
    });
    expect(events[2]?.data).toMatchObject({
      call_id: "tool-1",
      output: "tool completed",
      is_error: false,
    });
    expect(events[3]?.data).toMatchObject({
      call_id: "tool-2",
      output: JSON.stringify({
        tool_use_id: "tool-2",
        output: { ok: true },
        is_error: true,
      }),
      is_error: true,
    });
    expect(events[4]?.data).toMatchObject({
      cost_usd: 0.25,
      budget_remaining_usd: 4.75,
    });
    expect(runtime.turnNumber).toBe(1);
    expect(runtime.lastTool).toBe("Read");
    expect(runtime.spentUsd).toBe(0.25);
    expect(runtime.lastActivity).toBe(106);
  });

  test("aborts when local budget is exceeded", async () => {
    const runtime = new AgentRuntime("task-3", "session-3");
    const request = createRequest({
      task_id: "task-3",
      session_id: "session-3",
      budget_usd: 0.000001,
    });

    async function* queryRunner(): AsyncGenerator<Record<string, unknown>, void> {
      yield {
        type: "assistant",
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

    await expect(
      streamClaudeRuntime(
        runtime,
        { send() {} },
        request,
        "Injected system prompt",
        {
          queryRunner,
          now: createNow([201, 202, 203]),
        },
      ),
    ).rejects.toThrow("budget exceeded for task task-3");
    expect(runtime.abortController.signal.aborted).toBe(true);
  });

  test("persists a runtime snapshot to the session manager and event sink", () => {
    const runtime = new AgentRuntime("task-4", "session-4");
    const sessionManager = new SessionManager();
    const events: Array<{ type: string; data: unknown }> = [];

    runtime.status = "completed";
    runtime.turnNumber = 3;
    runtime.spentUsd = 0.42;

    persistRuntimeSnapshot(
      runtime,
      createRequest({ task_id: "task-4", session_id: "session-4" }),
      {
        send(event) {
          events.push(event as { type: string; data: unknown });
        },
      },
      sessionManager,
      () => 999,
    );

    expect(sessionManager.restore("task-4")).toMatchObject({
      task_id: "task-4",
      session_id: "session-4",
      status: "completed",
      turn_number: 3,
      spent_usd: 0.42,
      updated_at: 999,
    });
    expect(events).toHaveLength(1);
    expect(events[0]).toMatchObject({
      type: "snapshot",
      data: {
        task_id: "task-4",
        session_id: "session-4",
      },
    });
  });

  test("persists role and team execution identity inside the saved snapshot request", () => {
    const runtime = new AgentRuntime("task-5", "session-5");
    const sessionManager = new SessionManager();

    persistRuntimeSnapshot(
      runtime,
      createRequest({
        task_id: "task-5",
        session_id: "session-5",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        team_id: "team-123",
        team_role: "reviewer",
        role_config: {
          role_id: "code-reviewer",
          name: "Code Reviewer",
          role: "Senior Reviewer",
          goal: "Find risky changes",
          backstory: "A skeptical reviewer.",
          system_prompt: "Review carefully.",
          allowed_tools: ["Read"],
          max_budget_usd: 3,
          max_turns: 10,
          permission_mode: "default",
          tools: ["github-tool"],
          knowledge_context: "docs/PRD.md",
          output_filters: ["no_pii"],
        },
      }),
      { send() {} },
      sessionManager,
      () => 1_234,
    );

    expect(sessionManager.restore("task-5")).toMatchObject({
      request: {
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        team_id: "team-123",
        team_role: "reviewer",
        role_config: {
          role_id: "code-reviewer",
          tools: ["github-tool"],
          knowledge_context: "docs/PRD.md",
          output_filters: ["no_pii"],
        },
      },
    });
  });

  test("persists Claude continuity metadata and resume readiness in saved snapshots", () => {
    const runtime = new AgentRuntime("task-6", "session-6");
    const sessionManager = new SessionManager();

    runtime.continuity = {
      runtime: "claude_code",
      resume_ready: true,
      captured_at: 1_111,
      session_handle: "claude-session-6",
      checkpoint_id: "checkpoint-6",
      resume_token: "resume-token-6",
    };

    persistRuntimeSnapshot(
      runtime,
      createRequest({
        task_id: "task-6",
        session_id: "session-6",
        runtime: "claude_code",
        provider: "anthropic",
        model: "claude-sonnet-4-5",
      }),
      { send() {} },
      sessionManager,
      () => 1_234,
    );

    expect(sessionManager.restore("task-6")).toMatchObject({
      continuity: {
        runtime: "claude_code",
        resume_ready: true,
        captured_at: 1_111,
        session_handle: "claude-session-6",
        checkpoint_id: "checkpoint-6",
        resume_token: "resume-token-6",
      },
    });
  });

  test("extracts resumable Claude continuity metadata from runtime events", () => {
    expect(
      extractClaudeContinuity(
        {
          type: "assistant",
          session_id: "claude-session-7",
          uuid: "assistant-uuid-7",
        },
        777,
      ),
    ).toEqual({
      runtime: "claude_code",
      resume_ready: true,
      captured_at: 777,
      session_handle: "claude-session-7",
      checkpoint_id: "assistant-uuid-7",
      resume_token: "claude-session-7",
    });
  });

  test("does not leak non-Claude continuity through Claude fallback branches", () => {
    expect(
      extractClaudeContinuity(
        {
          type: "assistant",
        },
        888,
        {
          runtime: "codex",
          resume_ready: true,
          captured_at: 777,
          thread_id: "thread-1",
        },
      ),
    ).toBeNull();
  });
});
