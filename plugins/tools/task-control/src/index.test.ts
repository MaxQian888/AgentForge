import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { createLocalPluginHarness } from "../../../../src-bridge/src/plugin-sdk/index.js";
import { plugin } from "./index.js";

const originalFetch = globalThis.fetch;

describe("task-control tool plugin", () => {
  beforeEach(() => {
    process.env.AGENTFORGE_API_BASE_URL = "http://127.0.0.1:7777";
    process.env.AGENTFORGE_API_TOKEN = "test-token";
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    delete process.env.AGENTFORGE_API_BASE_URL;
    delete process.env.AGENTFORGE_API_TOKEN;
  });

  test("exposes the maintained task control tools", () => {
    const harness = createLocalPluginHarness(plugin);

    expect(harness.listTools().map((tool) => tool.name)).toEqual([
      "task:get",
      "task:decompose",
      "task:dispatch-history",
    ]);
    expect(harness.manifest.metadata.id).toBe("task-control");
  });

  test("reads task detail through the existing task control-plane endpoint", async () => {
    globalThis.fetch = (async (input, init) => {
      expect(input).toBe("http://127.0.0.1:7777/api/v1/tasks/task-123");
      expect(init?.method).toBe("GET");
      expect(init?.headers).toEqual(
        expect.objectContaining({
          Authorization: "Bearer test-token",
        }),
      );
      return new Response(
        JSON.stringify({
          id: "task-123",
          title: "Fix dispatch drift",
          status: "in_progress",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    const harness = createLocalPluginHarness(plugin);
    const result = await harness.callTool("task:get", { taskId: "task-123" });

    expect(result.isError).toBe(false);
    expect(result.structuredContent).toEqual(
      expect.objectContaining({
        task: expect.objectContaining({
          id: "task-123",
          title: "Fix dispatch drift",
        }),
      }),
    );
  });

  test("decomposes a task through the existing bridge-backed endpoint", async () => {
    globalThis.fetch = (async (input, init) => {
      expect(input).toBe("http://127.0.0.1:7777/api/v1/tasks/task-123/decompose");
      expect(init?.method).toBe("POST");
      return new Response(
        JSON.stringify({
          parentTask: { id: "task-123", title: "Ship starter catalog" },
          summary: "Split into catalog, tools, and workflow workstreams.",
          subtasks: [{ id: "sub-1", title: "Update bundle metadata" }],
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    const harness = createLocalPluginHarness(plugin);
    const result = await harness.callTool("task:decompose", { taskId: "task-123" });

    expect(result.isError).toBe(false);
    expect(result.structuredContent).toEqual(
      expect.objectContaining({
        decomposition: expect.objectContaining({
          summary: "Split into catalog, tools, and workflow workstreams.",
        }),
      }),
    );
  });

  test("returns dispatch history through the observability endpoint", async () => {
    globalThis.fetch = (async (input, init) => {
      expect(input).toBe("http://127.0.0.1:7777/api/v1/tasks/task-123/dispatch/history");
      expect(init?.method).toBe("GET");
      return new Response(
        JSON.stringify([
          {
            id: "attempt-1",
            outcome: "blocked",
            guardrailType: "budget",
          },
        ]),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    const harness = createLocalPluginHarness(plugin);
    const result = await harness.callTool("task:dispatch-history", { taskId: "task-123" });

    expect(result.isError).toBe(false);
    expect(result.structuredContent).toEqual(
      expect.objectContaining({
        history: [
          expect.objectContaining({
            id: "attempt-1",
            outcome: "blocked",
          }),
        ],
      }),
    );
  });

  test("rejects calls without a task id", async () => {
    const harness = createLocalPluginHarness(plugin);

    await expect(harness.callTool("task:get", {})).rejects.toThrow("taskId is required");
  });
});
