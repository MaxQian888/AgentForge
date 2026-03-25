import { describe, expect, test } from "bun:test";
import { aggregateReviewResults } from "./aggregator.js";
import type { ReviewExecutionResult } from "./types.js";

describe("aggregateReviewResults", () => {
  test("deduplicates findings reported by multiple dimensions", () => {
    const results: ReviewExecutionResult[] = [
      {
        dimension: "logic",
        status: "completed",
        findings: [
          {
            category: "logic",
            severity: "medium",
            file: "src/auth.ts",
            line: 14,
            message: "Missing guard clause",
          },
        ],
        summary: "Logic found one issue",
      },
      {
        dimension: "security",
        status: "completed",
        findings: [
          {
            category: "logic",
            severity: "medium",
            file: "src/auth.ts",
            line: 14,
            message: "Missing guard clause",
          },
        ],
        summary: "Security confirmed the same issue",
      },
    ];

    const aggregated = aggregateReviewResults(results);

    expect(aggregated.findings).toHaveLength(1);
    expect(aggregated.findings[0]?.sources).toEqual(["logic", "security"]);
    expect(aggregated.recommendation).toBe("request_changes");
  });

  test("keeps successful findings when one dimension fails", () => {
    const results: ReviewExecutionResult[] = [
      {
        dimension: "logic",
        status: "failed",
        findings: [],
        summary: "Logic review failed",
        error: "timeout",
      },
      {
        dimension: "performance",
        status: "completed",
        findings: [
          {
            category: "performance",
            severity: "high",
            file: "src/query.ts",
            line: 22,
            message: "Potential N+1 query",
          },
        ],
        summary: "Performance found one issue",
      },
    ];

    const aggregated = aggregateReviewResults(results);

    expect(aggregated.findings).toHaveLength(1);
    expect(aggregated.risk_level).toBe("high");
    expect(aggregated.summary).toContain("logic failed");
  });
});
