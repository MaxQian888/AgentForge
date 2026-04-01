import { afterEach, describe, expect, test } from "bun:test";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { AgentRuntime } from "../runtime/agent-runtime.js";
import type { PluginRecord } from "../plugins/types.js";
import type { ExecuteRequest } from "../types.js";
import { prepareCodexLaunch, streamCodexRuntime } from "./codex-runtime.js";

const tempDirs: string[] = [];

afterEach(() => {
  while (tempDirs.length > 0) {
    const dir = tempDirs.pop();
    if (dir) {
      rmSync(dir, { force: true, recursive: true });
    }
  }
});

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
      "status_change",
      "output",
      "tool_call",
      "tool_result",
      "cost_update",
    ]);
    expect(events[0]).toMatchObject({
      data: {
        old_status: "running",
        new_status: "running",
        reason: "turn_started",
      },
    });
    expect(events[1]).toMatchObject({
      data: {
        content: "Planning Codex bridge work.",
      },
    });
    expect(events[2]).toMatchObject({
      data: {
        tool_name: "shell",
        call_id: "item-1",
      },
    });
    expect(events[3]).toMatchObject({
      data: {
        call_id: "item-1",
        output: "M README.md",
        is_error: false,
      },
    });
    expect(events[4]).toMatchObject({
      data: {
        input_tokens: 120,
        cache_read_tokens: 30,
        output_tokens: 45,
        cost_usd: 0.03,
        cost_accounting: {
          total_cost_usd: 0.03,
          mode: "authoritative_total",
          coverage: "full",
          source: "codex_native_total",
        },
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

  test("handles advanced Codex event parsing and reports failed turns", async () => {
    const req = createRequest();
    const runtime = new AgentRuntime(req.task_id, req.session_id);
    runtime.bindRequest(req);
    const events: Array<{ type: string; data: unknown }> = [];

    await expect(
      streamCodexRuntime(
        runtime,
        {
          send(event) {
            events.push(event);
          },
        },
        req,
        "System prompt",
        {
          command: "codex",
          now: () => 1_700_000_000_111,
          async *codexRuntimeRunner() {
            yield { type: "thread.started", thread_id: "thread-codex-advanced" };
            yield { type: "turn.started", turn_id: "turn-1" };
            yield {
              type: "item.updated",
              item: {
                id: "cmd-1",
                type: "command_execution",
                command: "git diff",
                aggregated_output: "partial output",
                status: "in_progress",
              },
            };
            yield {
              type: "item.completed",
              item: {
                id: "reasoning-1",
                details: {
                  type: "Reasoning",
                  summary: "Thinking through the bridge changes.",
                },
              },
            };
            yield {
              type: "item.completed",
              item: {
                id: "files-1",
                details: {
                  type: "FileChange",
                  files: [{ path: "README.md", change_type: "modified" }],
                },
              },
            };
            yield {
              type: "item.completed",
              item: {
                id: "mcp-1",
                details: {
                  type: "McpToolCall",
                  toolName: "filesystem.read_file",
                  input: { path: "README.md" },
                  output: { content: "hello" },
                },
              },
            };
            yield {
              type: "item.completed",
              item: {
                id: "search-1",
                details: {
                  type: "WebSearch",
                  query: "AgentForge Codex bridge",
                  results: [{ title: "Result 1" }],
                },
              },
            };
            yield {
              type: "item.completed",
              item: {
                id: "todo-1",
                details: {
                  type: "TodoList",
                  todos: [
                    {
                      id: "todo-1",
                      content: "Finish Codex runtime parsing",
                      status: "in_progress",
                    },
                  ],
                },
              },
            };
            yield {
              type: "item.completed",
              item: {
                id: "collab-1",
                details: {
                  type: "CollabToolCall",
                  agentName: "reviewer",
                  output: "Reviewer completed analysis.",
                },
              },
            };
            yield {
              type: "item.completed",
              item: {
                id: "detail-error-1",
                details: {
                  type: "Error",
                  message: "detail parsing warning",
                },
              },
            };
            yield {
              type: "turn.failed",
              error: {
                message: "context overflow",
              },
            };
          },
        },
      ),
    ).rejects.toThrow("context overflow");

    expect(events.map((event) => event.type)).toEqual([
      "status_change",
      "progress",
      "reasoning",
      "file_change",
      "tool_call",
      "tool_result",
      "tool_call",
      "tool_result",
      "todo_update",
      "output",
      "error",
      "error",
    ]);
    expect(events[0]).toMatchObject({
      data: { old_status: "running", new_status: "running", reason: "turn_started" },
    });
    expect(events[1]).toMatchObject({
      data: { item_type: "command_execution", partial_output: "partial output" },
    });
    expect(events[2]).toMatchObject({
      data: { content: "Thinking through the bridge changes." },
    });
    expect(events[3]).toMatchObject({
      data: { files: [{ path: "README.md", change_type: "modified" }] },
    });
    expect(events[4]).toMatchObject({
      data: { tool_name: "filesystem.read_file" },
    });
    expect(events[5]).toMatchObject({
      data: { output: { content: "hello" } },
    });
    expect(events[6]).toMatchObject({
      data: { tool_name: "web_search" },
    });
    expect(events[7]).toMatchObject({
      data: { output: [{ title: "Result 1" }] },
    });
    expect(events[8]).toMatchObject({
      data: {
        todos: [{ id: "todo-1", content: "Finish Codex runtime parsing", status: "in_progress" }],
      },
    });
    expect(events[9]).toMatchObject({
      data: { content: "Reviewer completed analysis." },
    });
    expect(events[10]).toMatchObject({
      data: { message: "detail parsing warning", source: "codex" },
    });
    expect(events[11]).toMatchObject({
      data: { message: "context overflow", source: "codex" },
    });
    expect(runtime.status).toBe("failed");
    expect(runtime.continuity).toMatchObject({
      runtime: "codex",
      thread_id: "thread-codex-advanced",
      fork_available: true,
      rollback_turns: 1,
    });
  });

  test("builds advanced Codex CLI flags and config overrides from execute request fields", () => {
    const baseDir = mkdtempSync(join(tmpdir(), "agentforge-codex-launch-"));
    tempDirs.push(baseDir);
    const req = createRequest({
      task_id: "task-codex-flags",
      session_id: "session-codex-flags",
      worktree_path: baseDir,
      output_schema: {
        type: "json_schema",
        schema: {
          type: "object",
          properties: {
            summary: { type: "string" },
          },
        },
      },
      attachments: [
        { type: "image", path: "D:/tmp/one.png" },
        { type: "file", path: "D:/tmp/two.txt" },
      ],
      additional_directories: ["D:/Shared/A", "D:/Shared/B"],
      web_search: true,
      env: { FEATURE_FLAG: "enabled" },
    });
    const plugins: PluginRecord[] = [
      {
        apiVersion: "v1",
        kind: "ToolPlugin",
        metadata: { id: "stdio-plugin", name: "Stdio", version: "1.0.0" },
        spec: {
          runtime: "mcp",
          transport: "stdio",
          command: "node",
          args: ["server.js"],
          env: { TOKEN: "abc" },
        },
        permissions: {},
        source: { type: "local", path: "." },
        lifecycle_state: "active",
        runtime_host: "ts-bridge",
        restart_count: 0,
      },
      {
        apiVersion: "v1",
        kind: "ToolPlugin",
        metadata: { id: "http-plugin", name: "Http", version: "1.0.0" },
        spec: {
          runtime: "mcp",
          transport: "http",
          url: "http://127.0.0.1:8787/mcp",
        },
        permissions: {},
        source: { type: "local", path: "." },
        lifecycle_state: "active",
        runtime_host: "ts-bridge",
        restart_count: 0,
      },
    ];

    const launch = prepareCodexLaunch({
      mode: "start",
      command: "codex",
      req,
      prompt: "Run Codex with advanced options",
      activePlugins: plugins,
    });

    expect(launch.cmd).toEqual(
      expect.arrayContaining([
        "codex",
        "-C",
        baseDir,
        "exec",
        "--json",
        "--model",
        "gpt-5-codex",
        "--output-schema",
        expect.any(String),
        "--image",
        "D:/tmp/one.png",
        "--add-dir",
        "D:/Shared/A",
        "--add-dir",
        "D:/Shared/B",
        "--search",
      ]),
    );
    expect(launch.cmd).not.toContain("D:/tmp/two.txt");
    expect(launch.env).toMatchObject({
      FEATURE_FLAG: "enabled",
      AGENTFORGE_RUNTIME: "codex",
      AGENTFORGE_MODEL: "gpt-5-codex",
    });

    const schemaFlagIndex = launch.cmd.indexOf("--output-schema");
    const schemaPath = launch.cmd[schemaFlagIndex + 1];
    expect(schemaPath).toBeTruthy();
    expect(JSON.parse(readFileSync(String(schemaPath), "utf8"))).toEqual({
      type: "object",
      properties: {
        summary: { type: "string" },
      },
    });

    const configValues = launch.cmd.filter((entry) => entry.startsWith("mcp_servers."));
    expect(configValues).toEqual(
      expect.arrayContaining([
        'mcp_servers.stdio-plugin.command="node"',
        'mcp_servers.http-plugin.url="http://127.0.0.1:8787/mcp"',
      ]),
    );

    launch.cleanup();
    expect(() => readFileSync(String(schemaPath), "utf8")).toThrow();
  });
});
