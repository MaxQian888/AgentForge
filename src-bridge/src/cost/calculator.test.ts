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

  test("uses normalized aliases and current OpenAI pricing where available", () => {
    expect(calculateCost({}, "unknown-model")).toBe(0);
    expect(
      calculateCost(
        {
          input_tokens: 500_000,
          output_tokens: 500_000,
        },
        "claude-haiku-4",
      ),
    ).toBeCloseTo(3, 5);
    expect(
      calculateCost(
        {
          input_tokens: 1_000_000,
          cache_read_input_tokens: 1_000_000,
        },
        "gpt-5-codex",
      ),
    ).toBeCloseTo(1.375, 5);
  });
});
