import { describe, expect, test } from "bun:test";
import {
  accumulateCostAccounting,
  resolvePricingModel,
} from "./accounting.js";

describe("resolvePricingModel", () => {
  test("normalizes repository-supported Anthropic and OpenAI model aliases", () => {
    expect(resolvePricingModel("claude-haiku-4-5")).toMatchObject({
      provider: "anthropic",
      canonicalModel: "claude-haiku-4-5",
      inputPerMillion: 1,
      outputPerMillion: 5,
      cacheReadPerMillion: 0.1,
      cacheWritePerMillion: 1.25,
    });

    expect(resolvePricingModel("gpt-5-codex")).toMatchObject({
      provider: "openai",
      canonicalModel: "gpt-5-codex",
      inputPerMillion: 1.25,
      outputPerMillion: 10,
      cachedInputPerMillion: 0.125,
    });
  });
});

describe("accumulateCostAccounting", () => {
  test("prefers authoritative totals and preserves native component breakdowns", () => {
    const snapshot = accumulateCostAccounting({
      runtime: "claude_code",
      provider: "anthropic",
      requestedModel: "claude-sonnet-4-5",
      usageDelta: {
        inputTokens: 2_000,
        outputTokens: 750,
        cacheReadTokens: 120,
        cacheCreationTokens: 480,
      },
      authoritativeTotalCostUsd: 0.04,
      source: "anthropic_result_total",
      components: [
        {
          model: "claude-sonnet-4-5",
          inputTokens: 2_000,
          outputTokens: 750,
          cacheReadTokens: 120,
          cacheCreationTokens: 480,
          costUsd: 0.04,
          source: "anthropic_model_usage",
        },
      ],
    });

    expect(snapshot).toMatchObject({
      totalCostUsd: 0.04,
      inputTokens: 2_000,
      outputTokens: 750,
      cacheReadTokens: 120,
      cacheCreationTokens: 480,
      mode: "authoritative_total",
      coverage: "full",
      source: "anthropic_result_total",
      components: [
        {
          model: "claude-sonnet-4-5",
          costUsd: 0.04,
          source: "anthropic_model_usage",
        },
      ],
    });
  });

  test("accumulates official API-priced usage when no authoritative total is available", () => {
    const first = accumulateCostAccounting({
      runtime: "codex",
      provider: "openai",
      requestedModel: "gpt-5-codex",
      usageDelta: {
        inputTokens: 120,
        outputTokens: 45,
        cacheReadTokens: 30,
      },
      source: "openai_api_pricing",
    });

    const second = accumulateCostAccounting({
      previous: first,
      runtime: "codex",
      provider: "openai",
      requestedModel: "gpt-5-codex",
      usageDelta: {
        inputTokens: 80,
        outputTokens: 20,
        cacheReadTokens: 0,
      },
      source: "openai_api_pricing",
    });

    expect(first).toMatchObject({
      mode: "estimated_api_pricing",
      coverage: "full",
      inputTokens: 120,
      outputTokens: 45,
      cacheReadTokens: 30,
    });
    expect(first.totalCostUsd).toBeCloseTo(0.00060375, 10);

    expect(second).toMatchObject({
      mode: "estimated_api_pricing",
      coverage: "full",
      inputTokens: 200,
      outputTokens: 65,
      cacheReadTokens: 30,
    });
    expect(second.totalCostUsd).toBeCloseTo(0.00090375, 10);
  });

  test("marks plan-backed Codex usage as plan_included instead of fabricating billable usd", () => {
    const snapshot = accumulateCostAccounting({
      runtime: "codex",
      provider: "codex",
      requestedModel: "gpt-5-codex",
      usageDelta: {
        inputTokens: 500,
        outputTokens: 100,
        cacheReadTokens: 25,
      },
      source: "codex_plan_usage",
    });

    expect(snapshot).toMatchObject({
      totalCostUsd: 0,
      mode: "plan_included",
      coverage: "none",
      inputTokens: 500,
      outputTokens: 100,
      cacheReadTokens: 25,
      source: "codex_plan_usage",
    });
  });
});
