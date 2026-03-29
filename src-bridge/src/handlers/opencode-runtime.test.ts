import { describe, expect, test } from "bun:test";
import { AgentRuntime } from "../runtime/agent-runtime.js";
import type { ExecuteRequest } from "../types.js";
import { streamOpenCodeRuntime } from "./opencode-runtime.js";

function createRequest(overrides: Partial<ExecuteRequest> = {}): ExecuteRequest {
  return {
    task_id: "task-opencode",
    session_id: "session-opencode",
    runtime: "opencode",
    provider: "opencode",
    model: "opencode-default",
    prompt: "Inspect the bridge task.",
    worktree_path: "D:/Project/AgentForge",
    branch_name: "agent/task-opencode",
    system_prompt: "",
    max_turns: 8,
    budget_usd: 5,
    allowed_tools: ["Read"],
    permission_mode: "default",
    ...overrides,
  };
}

describe("streamOpenCodeRuntime", () => {
  test("creates an upstream session, normalizes canonical events, and captures resumable continuity", async () => {
    const req = createRequest();
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);
    const events: Array<{ type: string; data: unknown }> = [];
    const calls: Array<{ kind: string; payload: unknown }> = [];

    await streamOpenCodeRuntime(
      runtime,
      {
        send(event) {
          events.push(event);
        },
      },
      req,
      "Follow the repo instructions closely.",
      {
        transport: {
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
        } as never,
        async *eventRunner(params) {
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
        now: () => 1_700_000_000_000,
      },
    );

    expect(calls).toMatchObject([
      { kind: "createSession" },
      {
        kind: "sendPromptAsync",
        payload: { sessionId: "opencode-session-123", prompt: "Inspect the bridge task." },
      },
      {
        kind: "eventRunner",
        payload: { mode: "start", sessionId: "opencode-session-123" },
      },
    ]);
    expect(events.map((event) => event.type)).toEqual([
      "output",
      "tool_call",
      "tool_result",
      "cost_update",
    ]);
    expect(runtime.continuity).toMatchObject({
      runtime: "opencode",
      resume_ready: true,
      upstream_session_id: "opencode-session-123",
    });
  });

  test("resumes against the saved upstream session instead of creating a fresh session", async () => {
    const req = createRequest();
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);
    runtime.continuity = {
      runtime: "opencode",
      resume_ready: true,
      captured_at: 100,
      upstream_session_id: "opencode-session-continue",
      latest_message_id: "message-5",
      server_url: "http://127.0.0.1:4096",
    };

    const calls: Array<{ kind: string; payload: unknown }> = [];

    await streamOpenCodeRuntime(
      runtime,
      { send() {} },
      req,
      "System prompt",
      {
        transport: {
          async createSession() {
            throw new Error("should not create a fresh session");
          },
          async sendPromptAsync(input: { sessionId: string; prompt: string; provider: string; model?: string }) {
            calls.push({ kind: "sendPromptAsync", payload: input });
          },
          async abortSession() {
            return true;
          },
        } as never,
        async *eventRunner(params) {
          calls.push({
            kind: "eventRunner",
            payload: { mode: params.mode, sessionId: params.sessionId, prompt: params.prompt },
          });
          yield {
            event: "session.idle",
            data: {
              sessionID: params.sessionId,
            },
          };
        },
      },
    );

    expect(calls).toMatchObject([
      {
        kind: "sendPromptAsync",
        payload: { sessionId: "opencode-session-continue" },
      },
      {
        kind: "eventRunner",
        payload: { mode: "resume", sessionId: "opencode-session-continue" },
      },
    ]);
    expect(String((calls[0] as { payload: { prompt: string } }).payload.prompt)).toContain("Continue");
  });

  test("treats an event stream that ends before session idle as a runtime failure", async () => {
    const req = createRequest();
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);

    await expect(
      streamOpenCodeRuntime(
        runtime,
        { send() {} },
        req,
        "System prompt",
        {
          transport: {
            async createSession() {
              return { id: "opencode-session-interrupted" };
            },
            async sendPromptAsync() {},
            async abortSession() {
              return true;
            },
          } as never,
          async *eventRunner(params) {
            yield {
              event: "message.part.delta",
              data: {
                sessionID: params.sessionId,
                part: { type: "text", text: "Working..." },
              },
            };
          },
        },
      ),
    ).rejects.toThrow("OpenCode event stream ended before session became idle");
  });
});
