import { describe, expect, test } from "bun:test";
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
});
