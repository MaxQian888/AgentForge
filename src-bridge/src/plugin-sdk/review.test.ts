import { describe, expect, test } from "bun:test";
import { createReviewFinding, createReviewResult } from "./index.js";

function firstTextContent(result: { content: Array<{ type: string; text?: string }> }): string | undefined {
  const item = result.content.find((entry) => entry.type === "text");
  return item?.text;
}

describe("plugin SDK review helpers", () => {
  test("normalizes findings and preserves plugin provenance", () => {
    const finding = createReviewFinding(
      {
        category: "security",
        severity: "high",
        file: "src/auth.ts",
        line: 18,
        message: "Token fallback bypasses the expected secret source.",
      },
      { pluginId: "review.security" },
    );

    expect(finding.sources).toEqual(["review.security"]);
    expect(finding.category).toBe("security");
  });

  test("creates a findings/v1 review result payload", () => {
    const result = createReviewResult({
      pluginId: "review.architecture",
      summary: "Architecture plugin found one issue.",
      findings: [
        {
          category: "architecture",
          severity: "medium",
          file: "src/server/routes.ts",
          line: 9,
          message: "Route crosses the approved boundary.",
        },
      ],
    });

    expect(result.structuredContent).toEqual({
      format: "findings/v1",
      summary: "Architecture plugin found one issue.",
      findings: [
        expect.objectContaining({
          category: "architecture",
          sources: ["review.architecture"],
        }),
      ],
    });
    expect(firstTextContent(result)).toContain("Architecture plugin found one issue.");
  });
});
