import { describe, expect, test } from "bun:test";
import { MCPClientHub } from "../mcp/client-hub.js";
import { createDeepReviewOrchestrator, orchestrateDeepReview } from "./orchestrator.js";

describe("orchestrateDeepReview", () => {
  test("runs the default dimensions and aggregates their findings", async () => {
    const response = await orchestrateDeepReview({
      review_id: "review-123",
      task_id: "task-123",
      pr_url: "https://example.com/pr/123",
      title: "TODO: remove API_TOKEN fallback",
      description: "console.log should not ship",
      diff: [
        "eval('dangerous')",
        "for await (const item of items) { }",
        "SELECT * FROM users",
      ].join("\n"),
    });

    expect(response.dimension_results.map((result) => result.dimension)).toEqual([
      "logic",
      "security",
      "performance",
      "compliance",
    ]);
    expect(response.findings.map((finding) => finding.category)).toEqual(
      expect.arrayContaining(["logic", "security", "performance", "compliance"]),
    );
    expect(response.risk_level).toBe("high");
    expect(response.recommendation).toBe("request_changes");
    expect(response.cost_usd).toBe(0.2);
    expect(response.summary).toContain("logic:");
    expect(response.summary).toContain("security:");
  });

  test("respects explicit dimension selection", async () => {
    const response = await orchestrateDeepReview({
      review_id: "review-456",
      task_id: "task-456",
      pr_url: "https://example.com/pr/456",
      diff: "console.log('leftover debug');",
      dimensions: ["compliance"],
    });

    expect(response.dimension_results).toHaveLength(1);
    expect(response.dimension_results[0]?.dimension).toBe("compliance");
    expect(response.findings).toHaveLength(1);
    expect(response.findings[0]?.category).toBe("compliance");
  });

  test("aggregates custom review plugin findings with built-in dimensions", async () => {
    const runReview = createDeepReviewOrchestrator({
      executeReviewPlugin: async (plugin) => ({
        dimension: plugin.plugin_id,
        source_type: "plugin",
        plugin_id: plugin.plugin_id,
        display_name: plugin.name,
        status: "completed",
        findings: [
          {
            category: "architecture",
            severity: "high",
            file: "src/server/routes.ts",
            line: 18,
            message: "Route registration bypasses the approved architecture boundary.",
          },
        ],
        summary: "Architecture plugin found one issue.",
      }),
    });

    const response = await runReview({
      review_id: "review-789",
      task_id: "task-789",
      pr_url: "https://example.com/pr/789",
      diff: "console.log('leftover debug');",
      dimensions: ["compliance"],
      review_plugins: [
        {
          plugin_id: "review.architecture",
          name: "Architecture Review",
          entrypoint: "review:run",
          output_format: "findings/v1",
        },
      ],
    });

    expect(response.dimension_results).toHaveLength(2);
    expect(response.dimension_results[1]?.plugin_id).toBe("review.architecture");
    expect(response.findings).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ category: "compliance", sources: ["compliance"] }),
        expect.objectContaining({ category: "architecture", sources: ["review.architecture"] }),
      ]),
    );
  });

  test("preserves plugin failures without discarding successful dimensions", async () => {
    const runReview = createDeepReviewOrchestrator({
      executeReviewPlugin: async () => {
        throw new Error("review plugin timed out");
      },
    });

    const response = await runReview({
      review_id: "review-999",
      task_id: "task-999",
      pr_url: "https://example.com/pr/999",
      diff: "console.log('leftover debug');",
      dimensions: ["compliance"],
      review_plugins: [
        {
          plugin_id: "review.architecture",
          name: "Architecture Review",
          entrypoint: "review:run",
          output_format: "findings/v1",
        },
      ],
    });

    expect(response.dimension_results).toHaveLength(2);
    expect(response.dimension_results[1]).toEqual(
      expect.objectContaining({
        plugin_id: "review.architecture",
        source_type: "plugin",
        status: "failed",
      }),
    );
    expect(response.findings).toHaveLength(1);
    expect(response.summary).toContain("review.architecture failed");
  });

  test("marks review plugins as failed when entrypoint or transport metadata is missing", async () => {
    const response = await orchestrateDeepReview({
      review_id: "review-missing-plugin-metadata",
      task_id: "task-missing-plugin-metadata",
      pr_url: "https://example.com/pr/missing-plugin-metadata",
      dimensions: ["logic"],
      review_plugins: [
        {
          plugin_id: "review.no-entrypoint",
          name: "Missing Entrypoint",
          transport: "stdio",
        },
        {
          plugin_id: "review.no-transport",
          name: "Missing Transport",
          entrypoint: "review:run",
        },
      ],
    });

    expect(response.dimension_results).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          plugin_id: "review.no-entrypoint",
          source_type: "plugin",
          status: "failed",
          error: "review plugin review.no-entrypoint is missing an entrypoint",
        }),
        expect.objectContaining({
          plugin_id: "review.no-transport",
          source_type: "plugin",
          status: "failed",
          error: "review plugin review.no-transport is missing a transport",
        }),
      ]),
    );
  });

  test("uses the default MCP plugin path and normalizes structured findings", async () => {
    const originalConnectServer = MCPClientHub.prototype.connectServer;
    const originalCallTool = MCPClientHub.prototype.callTool;
    const originalDispose = MCPClientHub.prototype.dispose;
    const calls: Array<{ pluginId: string; entrypoint: string; input: unknown }> = [];
    let disposeCalls = 0;

    MCPClientHub.prototype.connectServer = (async () => {}) as unknown as typeof MCPClientHub.prototype.connectServer;
    MCPClientHub.prototype.callTool = (async (pluginId, entrypoint, input) => {
      calls.push({ pluginId, entrypoint, input });
      return {
        content: [],
        structuredContent: {
          summary: "Architecture plugin found one issue.",
          findings: [
            {
              category: "architecture",
              severity: "high",
              file: "src/server/routes.ts",
              line: 18,
              message: "Route registration bypasses the approved architecture boundary.",
            },
          ],
        },
      };
    }) as typeof MCPClientHub.prototype.callTool;
    MCPClientHub.prototype.dispose = (async () => {
      disposeCalls += 1;
    }) as typeof MCPClientHub.prototype.dispose;

    try {
      const runReview = createDeepReviewOrchestrator();
      const response = await runReview({
        review_id: "review-mcp-success",
        task_id: "task-mcp-success",
        pr_url: "https://example.com/pr/mcp-success",
        changed_files: ["src/server/routes.ts"],
        dimensions: ["logic"],
        review_plugins: [
          {
            plugin_id: "review.architecture",
            name: "Architecture Review",
            entrypoint: "review:run",
            transport: "stdio",
            command: "node",
            args: ["dist/review.js"],
          },
        ],
      });

      expect(calls).toEqual([
        {
          pluginId: "review.architecture",
          entrypoint: "review:run",
          input: expect.objectContaining({
            review_id: "review-mcp-success",
            task_id: "task-mcp-success",
            pr_url: "https://example.com/pr/mcp-success",
            changed_files: ["src/server/routes.ts"],
            dimensions: ["logic"],
            plugin_id: "review.architecture",
          }),
        },
      ]);
      expect(response.dimension_results).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            plugin_id: "review.architecture",
            source_type: "plugin",
            status: "completed",
            summary: "Architecture plugin found one issue.",
            findings: expect.arrayContaining([
              expect.objectContaining({
                category: "architecture",
                severity: "high",
              }),
            ]),
          }),
        ]),
      );
      expect(response.findings).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            category: "architecture",
            severity: "high",
            sources: ["review.architecture"],
          }),
        ]),
      );
      expect(disposeCalls).toBe(1);
    } finally {
      MCPClientHub.prototype.connectServer = originalConnectServer;
      MCPClientHub.prototype.callTool = originalCallTool;
      MCPClientHub.prototype.dispose = originalDispose;
    }
  });

  test("falls back to text payload parsing, filters invalid findings, and preserves MCP errors as plugin failures", async () => {
    const originalConnectServer = MCPClientHub.prototype.connectServer;
    const originalCallTool = MCPClientHub.prototype.callTool;
    const originalDispose = MCPClientHub.prototype.dispose;
    const responses = [
      {
        content: [
          {
            type: "text",
            text: JSON.stringify({
              summary: "",
              findings: [
                {
                  severity: "unknown",
                  message: "Missing cache header on SSR response.",
                },
                {
                  category: "architecture",
                  severity: "low",
                  message: "   ",
                },
                "skip-me",
              ],
            }),
          },
        ],
      },
      {
        content: [
          {
            type: "text",
            text: "Plugin completed without structured JSON output.",
          },
        ],
      },
      {
        content: [],
        isError: true,
      },
    ];
    let callIndex = 0;
    let disposeCalls = 0;

    MCPClientHub.prototype.connectServer = (async () => {}) as unknown as typeof MCPClientHub.prototype.connectServer;
    MCPClientHub.prototype.callTool = (async () => {
      const response = responses[callIndex];
      callIndex += 1;
      return response as Awaited<ReturnType<typeof MCPClientHub.prototype.callTool>>;
    }) as typeof MCPClientHub.prototype.callTool;
    MCPClientHub.prototype.dispose = (async () => {
      disposeCalls += 1;
    }) as typeof MCPClientHub.prototype.dispose;

    try {
      const runReview = createDeepReviewOrchestrator();
      const response = await runReview({
        review_id: "review-mcp-fallbacks",
        task_id: "task-mcp-fallbacks",
        pr_url: "https://example.com/pr/mcp-fallbacks",
        dimensions: ["logic"],
        review_plugins: [
          {
            plugin_id: "review.security",
            name: "Security Review",
            entrypoint: "review:run",
            transport: "stdio",
          },
          {
            plugin_id: "review.notes",
            name: "Notes Review",
            entrypoint: "review:run",
            transport: "stdio",
          },
          {
            plugin_id: "review.error",
            name: "Error Review",
            entrypoint: "review:run",
            transport: "stdio",
          },
        ],
      });

      expect(response.dimension_results).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            plugin_id: "review.security",
            status: "completed",
            summary: "Security Review reported 1 finding(s).",
            findings: expect.arrayContaining([
              expect.objectContaining({
                category: "review.security",
                severity: "medium",
                message: "Missing cache header on SSR response.",
              }),
            ]),
          }),
          expect.objectContaining({
            plugin_id: "review.notes",
            status: "completed",
            summary: "Plugin completed without structured JSON output.",
            findings: [],
          }),
          expect.objectContaining({
            plugin_id: "review.error",
            status: "failed",
            error: "review plugin review.error returned an MCP error",
          }),
        ]),
      );
      expect(response.findings).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            category: "review.security",
            severity: "medium",
            message: "Missing cache header on SSR response.",
            sources: ["review.security"],
          }),
        ]),
      );
      expect(disposeCalls).toBe(3);
    } finally {
      MCPClientHub.prototype.connectServer = originalConnectServer;
      MCPClientHub.prototype.callTool = originalCallTool;
      MCPClientHub.prototype.dispose = originalDispose;
    }
  });
});
