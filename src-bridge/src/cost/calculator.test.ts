import { describe, expect, test } from "bun:test";
import { calculateCost } from "./calculator.js";

describe("calculateCost", () => {
  test("uses the default sonnet pricing table", () => {
    const cost = calculateCost({
      input_tokens: 1_000_000,
      output_tokens: 2_000_000,
      cache_read_input_tokens: 3_000_000,
    });

    expect(cost).toBeCloseTo(33.9, 5);
  });

  test("falls back to sonnet pricing for unknown models and missing usage fields", () => {
    expect(calculateCost({}, "unknown-model")).toBe(0);
    expect(
      calculateCost(
        {
          input_tokens: 500_000,
          output_tokens: 500_000,
        },
        "claude-haiku-4",
      ),
    ).toBeCloseTo(2.4, 5);
  });
});
