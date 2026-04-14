import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { createLocalPluginHarness } from "../../../../src-bridge/src/plugin-sdk/index.js";
import { plugin } from "./index.js";

const originalFetch = globalThis.fetch;

describe("review-control tool plugin", () => {
  beforeEach(() => {
    process.env.AGENTFORGE_API_BASE_URL = "http://127.0.0.1:7777";
    process.env.AGENTFORGE_API_TOKEN = "test-token";
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    delete process.env.AGENTFORGE_API_BASE_URL;
    delete process.env.AGENTFORGE_API_TOKEN;
  });

  test("exposes the maintained review control tools", () => {
    const harness = createLocalPluginHarness(plugin);

    expect(harness.listTools().map((tool) => tool.name)).toEqual([
      "review:trigger",
      "review:get",
      "review:list-by-task",
    ]);
    expect(harness.manifest.metadata.id).toBe("review-control");
  });

  test("triggers a review through the current control-plane entrypoint", async () => {
    globalThis.fetch = (async (input, init) => {
      expect(input).toBe("http://127.0.0.1:7777/api/v1/reviews/trigger");
      expect(init?.method).toBe("POST");
      expect(JSON.parse(String(init?.body))).toEqual(
        expect.objectContaining({
          taskId: "task-123",
          prUrl: "https://github.com/agentforge/agentforge/pull/42",
          trigger: "manual",
        }),
      );
      return new Response(
        JSON.stringify({
          id: "review-123",
          status: "pending",
          prUrl: "https://github.com/agentforge/agentforge/pull/42",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    const harness = createLocalPluginHarness(plugin);
    const result = await harness.callTool("review:trigger", {
      taskId: "task-123",
      prUrl: "https://github.com/agentforge/agentforge/pull/42",
    });

    expect(result.isError).toBe(false);
    expect(result.structuredContent).toEqual(
      expect.objectContaining({
        review: expect.objectContaining({
          id: "review-123",
          status: "pending",
        }),
      }),
    );
  });

  test("reads review detail through the review detail endpoint", async () => {
    globalThis.fetch = (async (input, init) => {
      expect(input).toBe("http://127.0.0.1:7777/api/v1/reviews/review-123");
      expect(init?.method).toBe("GET");
      return new Response(
        JSON.stringify({
          id: "review-123",
          status: "completed",
          riskLevel: "medium",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    const harness = createLocalPluginHarness(plugin);
    const result = await harness.callTool("review:get", { reviewId: "review-123" });

    expect(result.isError).toBe(false);
    expect(result.structuredContent).toEqual(
      expect.objectContaining({
        review: expect.objectContaining({
          id: "review-123",
          riskLevel: "medium",
        }),
      }),
    );
  });

  test("lists reviews for a task through the task review surface", async () => {
    globalThis.fetch = (async (input, init) => {
      expect(input).toBe("http://127.0.0.1:7777/api/v1/tasks/task-123/reviews");
      expect(init?.method).toBe("GET");
      return new Response(
        JSON.stringify([
          {
            id: "review-123",
            status: "completed",
          },
        ]),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    const harness = createLocalPluginHarness(plugin);
    const result = await harness.callTool("review:list-by-task", { taskId: "task-123" });

    expect(result.isError).toBe(false);
    expect(result.structuredContent).toEqual(
      expect.objectContaining({
        reviews: [
          expect.objectContaining({
            id: "review-123",
            status: "completed",
          }),
        ],
      }),
    );
  });

  test("rejects review triggers without a pull request url", async () => {
    const harness = createLocalPluginHarness(plugin);

    await expect(harness.callTool("review:trigger", { taskId: "task-123" })).rejects.toThrow(
      "prUrl is required",
    );
  });
});
