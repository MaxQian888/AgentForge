import { describe, expect, test } from "bun:test";
import type {
  DeepReviewRequest,
  DeepReviewResponse,
  ReviewExecutionResult,
  ReviewFinding,
  ReviewPluginExecution,
} from "./types.js";

describe("review contract types", () => {
  test("accepts deep review requests and aggregated responses with plugin executions", () => {
    const finding: ReviewFinding = {
      category: "security",
      severity: "high",
      file: "src/auth.ts",
      line: 18,
      message: "Token fallback bypasses the expected secret source.",
      suggestion: "Read the token from the configured secret provider only.",
      sources: ["review.security"],
    };
    const plugin: ReviewPluginExecution = {
      plugin_id: "review.security",
      name: "Security Review",
      entrypoint: "review:run",
      source_type: "catalog",
      transport: "stdio",
      command: "node",
      args: ["dist/review.js"],
      events: ["pull_request.updated"],
      file_patterns: ["src/**/*.ts"],
      output_format: "findings/v1",
    };
    const request: DeepReviewRequest = {
      review_id: "review-1",
      task_id: "task-1",
      pr_url: "https://example.test/pr/1",
      pr_number: 1,
      title: "Harden auth fallback",
      description: "Ensure bridge auth paths use one canonical token source.",
      diff: "@@ -1 +1 @@",
      trigger_event: "pull_request.updated",
      changed_files: ["src/auth.ts"],
      dimensions: ["logic", "security"],
      review_plugins: [plugin],
    };
    const dimensionResult: ReviewExecutionResult = {
      dimension: "security",
      source_type: "plugin",
      plugin_id: plugin.plugin_id,
      display_name: plugin.name,
      status: "completed",
      findings: [finding],
      summary: "Security review found one blocking issue.",
    };
    const response: DeepReviewResponse = {
      risk_level: "high",
      findings: [finding],
      summary: "One blocking security issue requires remediation.",
      recommendation: "request_changes",
      cost_usd: 1.75,
      dimension_results: [dimensionResult],
    };

    expect(request.review_plugins?.[0]?.plugin_id).toBe("review.security");
    expect(response.dimension_results[0]?.status).toBe("completed");
    expect(response.findings[0]?.severity).toBe("high");
  });
});
