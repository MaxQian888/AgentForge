import { estimateCostUsd } from "./accounting.js";

export interface UsageInfo {
  input_tokens?: number;
  output_tokens?: number;
  cache_read_input_tokens?: number;
  cache_creation_input_tokens?: number;
}

export function calculateCost(usage: UsageInfo, model = "claude-sonnet-4"): number {
  return (
    estimateCostUsd(model, {
      inputTokens: usage.input_tokens,
      outputTokens: usage.output_tokens,
      cacheReadTokens: usage.cache_read_input_tokens,
      cacheCreationTokens: usage.cache_creation_input_tokens,
    }) ?? 0
  );
}
