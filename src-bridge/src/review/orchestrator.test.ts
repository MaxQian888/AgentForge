import { describe, expect, test } from "bun:test";
import { orchestrateDeepReview } from "./orchestrator.js";

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
});
