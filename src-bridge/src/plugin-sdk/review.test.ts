import { describe, expect, test } from "bun:test";
import { createReviewFinding, createReviewResult } from "./index.js";
import type { ReviewFinding } from "../review/types.js";

interface FindingsPayload {
  format: string;
  summary: string;
  findings: ReviewFinding[];
}

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

  test("createReviewResult with suggestedPatch emits findings/v2", () => {
    const patch = "--- a/foo\n+++ b/foo\n@@ -1 +1 @@\n-x\n+y\n";
    const result = createReviewResult({
      pluginId: "review.fixer",
      summary: "One fixable issue.",
      findings: [
        {
          category: "logic",
          severity: "high",
          file: "foo.ts",
          line: 1,
          message: "Bad logic.",
          suggestedPatch: patch,
        },
      ],
    });

    const sc = result.structuredContent as unknown as FindingsPayload;
    expect(sc.format).toBe("findings/v2");
    expect(sc.findings[0].suggested_patch).toBe(patch);
  });

  test("createReviewResult without patches stays on findings/v1", () => {
    const result = createReviewResult({
      pluginId: "review.lint",
      summary: "Lint issues.",
      findings: [
        {
          category: "style",
          severity: "low",
          message: "Indentation off.",
        },
      ],
    });

    const sc = result.structuredContent as unknown as FindingsPayload;
    expect(sc.format).toBe("findings/v1");
  });

  test("mixed findings — at least one patch upgrades to v2 and normalizes nulls", () => {
    const result = createReviewResult({
      pluginId: "review.mix",
      summary: "Mixed.",
      findings: [
        {
          category: "logic",
          severity: "high",
          message: "Fix this.",
          suggestedPatch: "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n",
        },
        {
          category: "style",
          severity: "low",
          message: "Format.",
        },
      ],
    });

    const sc = result.structuredContent as unknown as FindingsPayload;
    expect(sc.format).toBe("findings/v2");
    expect(sc.findings[0].suggested_patch).toBeTruthy();
    expect(sc.findings[1].suggested_patch).toBeNull();
  });
});
