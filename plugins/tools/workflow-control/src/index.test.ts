import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { createLocalPluginHarness } from "../../../../src-bridge/src/plugin-sdk/index.js";
import { plugin } from "./index.js";

const originalFetch = globalThis.fetch;

describe("workflow-control tool plugin", () => {
  beforeEach(() => {
    process.env.AGENTFORGE_API_BASE_URL = "http://127.0.0.1:7777";
    process.env.AGENTFORGE_API_TOKEN = "test-token";
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    delete process.env.AGENTFORGE_API_BASE_URL;
    delete process.env.AGENTFORGE_API_TOKEN;
  });

  test("exposes the maintained workflow control tools", () => {
    const harness = createLocalPluginHarness(plugin);

    expect(harness.listTools().map((tool) => tool.name)).toEqual([
      "workflow:start",
      "workflow:get-run",
      "workflow:list-runs",
    ]);
    expect(harness.manifest.metadata.id).toBe("workflow-control");
  });

  test("starts a workflow run through the plugin workflow run endpoint", async () => {
    globalThis.fetch = (async (input, init) => {
      expect(input).toBe("http://127.0.0.1:7777/api/v1/plugins/standard-dev-flow/workflow-runs");
      expect(init?.method).toBe("POST");
      expect(JSON.parse(String(init?.body))).toEqual({
        trigger: {
          taskId: "task-123",
          memberId: "member-456",
        },
      });
      return new Response(
        JSON.stringify({
          id: "run-123",
          pluginId: "standard-dev-flow",
          status: "running",
        }),
        { status: 201, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    const harness = createLocalPluginHarness(plugin);
    const result = await harness.callTool("workflow:start", {
      pluginId: "standard-dev-flow",
      trigger: {
        taskId: "task-123",
        memberId: "member-456",
      },
    });

    expect(result.isError).toBe(false);
    expect(result.structuredContent).toEqual(
      expect.objectContaining({
        run: expect.objectContaining({
          id: "run-123",
          status: "running",
        }),
      }),
    );
  });

  test("reads a workflow run by id through the current detail endpoint", async () => {
    globalThis.fetch = (async (input, init) => {
      expect(input).toBe("http://127.0.0.1:7777/api/v1/plugins/workflow-runs/run-123");
      expect(init?.method).toBe("GET");
      return new Response(
        JSON.stringify({
          id: "run-123",
          status: "completed",
          currentStepId: "",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    const harness = createLocalPluginHarness(plugin);
    const result = await harness.callTool("workflow:get-run", { runId: "run-123" });

    expect(result.isError).toBe(false);
    expect(result.structuredContent).toEqual(
      expect.objectContaining({
        run: expect.objectContaining({
          id: "run-123",
          status: "completed",
        }),
      }),
    );
  });

  test("lists workflow runs for a plugin through the run history endpoint", async () => {
    globalThis.fetch = (async (input, init) => {
      expect(input).toBe("http://127.0.0.1:7777/api/v1/plugins/standard-dev-flow/workflow-runs");
      expect(init?.method).toBe("GET");
      return new Response(
        JSON.stringify([
          {
            id: "run-123",
            status: "completed",
          },
        ]),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    const harness = createLocalPluginHarness(plugin);
    const result = await harness.callTool("workflow:list-runs", { pluginId: "standard-dev-flow" });

    expect(result.isError).toBe(false);
    expect(result.structuredContent).toEqual(
      expect.objectContaining({
        runs: [
          expect.objectContaining({
            id: "run-123",
            status: "completed",
          }),
        ],
      }),
    );
  });

  test("rejects workflow start without a plugin id", async () => {
    const harness = createLocalPluginHarness(plugin);

    await expect(harness.callTool("workflow:start", { trigger: {} })).rejects.toThrow(
      "pluginId is required",
    );
  });
});
