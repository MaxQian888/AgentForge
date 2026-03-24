import { describe, expect, test } from "bun:test";
import { classifyError } from "./errors.js";

describe("classifyError", () => {
  test("maps bridge failures to the expected error classes", () => {
    const cases = [
      {
        input: new Error("Cancelled by operator"),
        expected: { code: "CANCELLED", retryable: false },
      },
      {
        input: new Error("rate_limit reached with 429"),
        expected: { code: "RATE_LIMIT", retryable: true },
      },
      {
        input: new Error("cluster overloaded with 529"),
        expected: { code: "OVERLOADED", retryable: true },
      },
      {
        input: new Error("budget exceeded"),
        expected: { code: "BUDGET_EXCEEDED", retryable: false },
      },
      {
        input: new Error("ETIMEDOUT waiting for upstream"),
        expected: { code: "TIMEOUT", retryable: true },
      },
      {
        input: new Error("authentication failed with 401"),
        expected: { code: "AUTH_FAILED", retryable: false },
      },
      {
        input: "unexpected failure",
        expected: { code: "INTERNAL", retryable: false },
      },
    ];

    for (const testCase of cases) {
      const result = classifyError(testCase.input);
      expect(result).toMatchObject(testCase.expected);
      expect(result.message).toContain(
        testCase.input instanceof Error ? testCase.input.message : "unexpected failure",
      );
    }
  });
});
