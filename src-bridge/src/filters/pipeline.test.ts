import { describe, expect, test } from "bun:test";
import { applyFilters, buildFilterPipeline } from "./pipeline.js";

describe("output filter pipeline", () => {
  test("builds only known filters in the requested order", () => {
    const pipeline = buildFilterPipeline(["no_pii", "missing", "no_credentials"]);

    expect(pipeline.map((filter) => filter.name)).toEqual(["no_pii", "no_credentials"]);
  });

  test("redacts credentials and pii across the configured pipeline", () => {
    const pipeline = buildFilterPipeline(["no_credentials", "no_pii"]);
    const input = [
      "Token: sk-abcdefghijklmnopqrstuvwxyz123456",
      "AWS=AKIAABCDEFGHIJKLMNOP",
      "Authorization: Bearer abcdefghijklmnopqrstuvwxyz123456",
      "Proxy=https://agent:super-secret@example.test",
      "API_KEY=bridge-secret",
      "Contact: engineer@example.com 555-123-4567",
    ].join("\n");

    const output = applyFilters(input, pipeline);

    expect(output).toContain("Token: [REDACTED]");
    expect(output).toContain("[REDACTED_AWS_KEY]");
    expect(output).toContain("Bearer [REDACTED]");
    expect(output).toContain("://agent:[REDACTED]@example.test");
    expect(output).toContain("API_KEY=[REDACTED]");
    expect(output).toContain("[REDACTED_EMAIL]");
    expect(output).toContain("[REDACTED_PHONE]");
  });

  test("returns the original text when no filters are configured", () => {
    const text = "No filtering should happen here.";

    expect(applyFilters(text, [])).toBe(text);
  });
});
