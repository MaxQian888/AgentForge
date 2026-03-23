const PRICING: Record<string, { input: number; output: number; cacheRead: number }> = {
  "claude-sonnet-4": { input: 3.0, output: 15.0, cacheRead: 0.3 },
  "claude-haiku-4": { input: 0.8, output: 4.0, cacheRead: 0.08 },
  "claude-opus-4": { input: 15.0, output: 75.0, cacheRead: 1.5 },
} as const;

export interface UsageInfo {
  input_tokens?: number;
  output_tokens?: number;
  cache_read_input_tokens?: number;
}

export function calculateCost(usage: UsageInfo, model = "claude-sonnet-4"): number {
  const pricing = PRICING[model] ?? PRICING["claude-sonnet-4"];
  const input = ((usage.input_tokens ?? 0) / 1_000_000) * pricing.input;
  const output = ((usage.output_tokens ?? 0) / 1_000_000) * pricing.output;
  const cache = ((usage.cache_read_input_tokens ?? 0) / 1_000_000) * pricing.cacheRead;
  return input + output + cache;
}
