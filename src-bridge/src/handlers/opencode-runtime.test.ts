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
    expect(events[3]).toMatchObject({
      data: {
        cost_usd: 0.04,
        cost_accounting: {
          total_cost_usd: 0.04,
          mode: "authoritative_total",
          coverage: "full",
          source: "opencode_native_total",
        },
      },
    });
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

  test("preserves parity-sensitive execute inputs through session bootstrap and prompt payloads", async () => {
    const req = createRequest({
      provider: "anthropic",
      model: "claude-sonnet-4-5",
      env: {
        FEATURE_FLAG: "enabled",
      },
      web_search: true,
      attachments: [
        {
          type: "image",
          path: "D:/tmp/screen.png",
          mime_type: "image/png",
        },
        {
          type: "file",
          path: "D:/tmp/spec.md",
          mime_type: "text/markdown",
        },
      ],
    });
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);
    const calls: Array<{ kind: string; payload: unknown }> = [];

    await streamOpenCodeRuntime(
      runtime,
      { send() {} },
      req,
      "System prompt",
      {
        transport: {
          async createSession(input: unknown) {
            calls.push({ kind: "createSession", payload: input });
            return { id: "opencode-session-parity" };
          },
          async sendPromptAsync(input: unknown) {
            calls.push({ kind: "sendPromptAsync", payload: input });
          },
          async abortSession() {
            return true;
          },
        } as never,
        async *eventRunner(params: { sessionId: string }) {
          yield {
            event: "session.idle",
            data: {
              sessionID: params.sessionId,
            },
          };
        },
      } as never,
    );

    expect(calls).toEqual([
      {
        kind: "createSession",
        payload: {
          title: "task-opencode",
          provider: "anthropic",
          model: "claude-sonnet-4-5",
          env: {
            FEATURE_FLAG: "enabled",
          },
          web_search: true,
        },
      },
      {
        kind: "sendPromptAsync",
        payload: {
          sessionId: "opencode-session-parity",
          provider: "anthropic",
          model: "claude-sonnet-4-5",
          prompt: "Inspect the bridge task.",
          parts: [
            {
              type: "image",
              path: "D:/tmp/screen.png",
              mime_type: "image/png",
            },
            {
              type: "file",
              path: "D:/tmp/spec.md",
              mime_type: "text/markdown",
            },
            {
              type: "text",
              text: "Inspect the bridge task.",
            },
          ],
        },
      },
    ]);
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

  test("handles advanced OpenCode events, parts, and continuity metadata", async () => {
    const req = createRequest();
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);
    const events: Array<{ type: string; data: unknown }> = [];

    await streamOpenCodeRuntime(
      runtime,
      {
        send(event) {
          events.push(event);
        },
      },
      req,
      "System prompt",
      {
        transport: {
          async createSession() {
            return { id: "opencode-session-advanced" };
          },
          async sendPromptAsync() {},
          async abortSession() {
            return true;
          },
        } as never,
        async *eventRunner(params) {
          yield {
            event: "session.status",
            data: {
              sessionID: params.sessionId,
              status: "busy",
            },
          };
          yield {
            event: "todo.updated",
            data: {
              sessionID: params.sessionId,
              todos: [{ id: "todo-1", content: "Ship OpenCode" }],
            },
          };
          yield {
            event: "message.updated",
            data: {
              sessionID: params.sessionId,
              message: { id: "msg-2", content: "updated content" },
            },
          };
          yield {
            event: "command.executed",
            data: {
              sessionID: params.sessionId,
              name: "compact",
            },
          };
          yield {
            event: "vcs.branch.updated",
            data: {
              sessionID: params.sessionId,
              branch: "agent/task-opencode",
            },
          };
          yield {
            event: "message.part.delta",
            data: {
              sessionID: params.sessionId,
              messageID: "msg-3",
              part: { type: "reasoning", reasoning: "thinking" },
            },
          };
          yield {
            event: "message.part.updated",
            data: {
              sessionID: params.sessionId,
              messageID: "msg-4",
              part: { type: "file", files: [{ path: "README.md", change_type: "modified" }] },
            },
          };
          yield {
            event: "message.part.updated",
            data: {
              sessionID: params.sessionId,
              messageID: "msg-5",
              part: { type: "agent", agentName: "reviewer", output: "reviewed" },
            },
          };
          yield {
            event: "message.part.updated",
            data: {
              sessionID: params.sessionId,
              messageID: "msg-6",
              part: { type: "compaction", summary: "compacted" },
            },
          };
          yield {
            event: "message.part.updated",
            data: {
              sessionID: params.sessionId,
              messageID: "msg-7",
              part: { type: "subtask", title: "task A", content: "finish A" },
            },
          };
          yield {
            event: "session.idle",
            data: {
              sessionID: params.sessionId,
              messageID: "msg-7",
            },
          };
        },
        now: () => 1_700_000_000_111,
      },
    );

    expect(events.map((event) => event.type)).toEqual([
      "status_change",
      "todo_update",
      "output",
      "output",
      "status_change",
      "reasoning",
      "file_change",
      "output",
      "status_change",
      "output",
    ]);
    expect(events[0]).toMatchObject({ data: { state: "running" } });
    expect(events[1]).toMatchObject({ data: { todos: [{ id: "todo-1", content: "Ship OpenCode" }] } });
    expect(events[2]).toMatchObject({ data: { content: "updated content" } });
    expect(events[3]).toMatchObject({ data: { content: "Command /compact executed" } });
    expect(events[4]).toMatchObject({ data: { reason: "branch_updated", branch: "agent/task-opencode" } });
    expect(events[5]).toMatchObject({ data: { content: "thinking" } });
    expect(events[6]).toMatchObject({ data: { files: [{ path: "README.md", change_type: "modified" }] } });
    expect(events[7]).toMatchObject({ data: { content: "reviewed", agent_name: "reviewer" } });
    expect(events[8]).toMatchObject({ data: { reason: "compaction", summary: "compacted" } });
    expect(events[9]).toMatchObject({ data: { content: "Subtask task A: finish A" } });
    expect(runtime.continuity).toMatchObject({
      runtime: "opencode",
      upstream_session_id: "opencode-session-advanced",
      latest_message_id: "msg-7",
      revert_message_ids: expect.arrayContaining(["msg-3", "msg-4", "msg-5", "msg-6", "msg-7"]),
      fork_available: true,
    });
  });

  test("emits permission_request events for OpenCode permission prompts", async () => {
    const req = createRequest();
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);
    const events: Array<{ type: string; data: unknown }> = [];
    const pendingCalls: Array<unknown> = [];

    await streamOpenCodeRuntime(
      runtime,
      {
        send(event) {
          events.push(event);
        },
      },
      req,
      "System prompt",
      {
        transport: {
          async createSession() {
            return { id: "opencode-session-permission" };
          },
          async sendPromptAsync() {},
          async abortSession() {
            return true;
          },
        } as never,
        opencodePendingInteractions: {
          createPermissionRequest(input: unknown) {
            pendingCalls.push(input);
            return { requestId: "opencode-permission-1" };
          },
        } as never,
        async *eventRunner(params: { sessionId: string }) {
          yield {
            event: "permission.asked",
            data: {
              sessionID: params.sessionId,
              permissionID: "perm-1",
              toolName: "Read",
              context: {
                file_path: "README.md",
              },
            },
          };
          yield {
            event: "session.idle",
            data: {
              sessionID: params.sessionId,
            },
          };
        },
      } as never,
    );

    expect(events).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          type: "permission_request",
          data: expect.objectContaining({
            request_id: "opencode-permission-1",
            tool_name: "Read",
          }),
        }),
      ]),
    );
    expect(pendingCalls).toEqual([
      expect.objectContaining({
        sessionId: "opencode-session-permission",
        permissionId: "perm-1",
      }),
    ]);
  });
});
