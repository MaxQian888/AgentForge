export type CostAccountingMode =
  | "authoritative_total"
  | "estimated_api_pricing"
  | "plan_included"
  | "unpriced";

export type CostAccountingCoverage = "full" | "partial" | "none";

export interface PricingModel {
  provider: "anthropic" | "openai";
  canonicalModel: string;
  inputPerMillion: number;
  outputPerMillion: number;
  cacheReadPerMillion?: number;
  cacheWritePerMillion?: number;
  cachedInputPerMillion?: number;
}

export interface CostUsageTotals {
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheCreationTokens: number;
}

export interface CostAccountingComponent {
  model: string;
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheCreationTokens: number;
  costUsd: number;
  source: string;
}

export interface CostAccountingSnapshot extends CostUsageTotals {
  totalCostUsd: number;
  mode: CostAccountingMode;
  coverage: CostAccountingCoverage;
  source: string;
  components: CostAccountingComponent[];
}

export interface CostAccountingComponentData {
  model: string;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_creation_tokens: number;
  cost_usd: number;
  source: string;
}

export interface CostAccountingData {
  total_cost_usd: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_creation_tokens: number;
  mode: CostAccountingMode;
  coverage: CostAccountingCoverage;
  source: string;
  components: CostAccountingComponentData[];
}

export interface CostAccountingComponentInput {
  model: string;
  inputTokens?: number;
  outputTokens?: number;
  cacheReadTokens?: number;
  cacheCreationTokens?: number;
  costUsd?: number;
  source: string;
}

export interface AccumulateCostAccountingParams {
  previous?: CostAccountingSnapshot | null;
  runtime: string;
  provider: string;
  requestedModel?: string;
  usageDelta?: Partial<CostUsageTotals>;
  authoritativeTotalCostUsd?: number;
  source: string;
  components?: CostAccountingComponentInput[];
}

const PRICING_MODELS: Record<string, PricingModel> = {
  "claude-opus-4-6": {
    provider: "anthropic",
    canonicalModel: "claude-opus-4-6",
    inputPerMillion: 5,
    outputPerMillion: 25,
    cacheReadPerMillion: 0.5,
    cacheWritePerMillion: 6.25,
  },
  "claude-sonnet-4-6": {
    provider: "anthropic",
    canonicalModel: "claude-sonnet-4-6",
    inputPerMillion: 3,
    outputPerMillion: 15,
    cacheReadPerMillion: 0.3,
    cacheWritePerMillion: 3.75,
  },
  "claude-sonnet-4-5": {
    provider: "anthropic",
    canonicalModel: "claude-sonnet-4-5",
    inputPerMillion: 3,
    outputPerMillion: 15,
    cacheReadPerMillion: 0.3,
    cacheWritePerMillion: 3.75,
  },
  "claude-opus-4-1": {
    provider: "anthropic",
    canonicalModel: "claude-opus-4-1",
    inputPerMillion: 15,
    outputPerMillion: 75,
    cacheReadPerMillion: 1.5,
    cacheWritePerMillion: 18.75,
  },
  "claude-haiku-4-5": {
    provider: "anthropic",
    canonicalModel: "claude-haiku-4-5",
    inputPerMillion: 1,
    outputPerMillion: 5,
    cacheReadPerMillion: 0.1,
    cacheWritePerMillion: 1.25,
  },
  "gpt-5-codex": {
    provider: "openai",
    canonicalModel: "gpt-5-codex",
    inputPerMillion: 1.25,
    outputPerMillion: 10,
    cachedInputPerMillion: 0.125,
  },
  "gpt-5.4": {
    provider: "openai",
    canonicalModel: "gpt-5.4",
    inputPerMillion: 2.5,
    outputPerMillion: 15,
    cachedInputPerMillion: 0.25,
  },
  "gpt-4.1": {
    provider: "openai",
    canonicalModel: "gpt-4.1",
    inputPerMillion: 2,
    outputPerMillion: 8,
    cachedInputPerMillion: 0.5,
  },
  o3: {
    provider: "openai",
    canonicalModel: "o3",
    inputPerMillion: 10,
    outputPerMillion: 40,
    cachedInputPerMillion: 2.5,
  },
};

const MODEL_ALIASES: Record<string, string> = {
  "claude-opus-4": "claude-opus-4-1",
  "claude-sonnet-4": "claude-sonnet-4-5",
  "claude-haiku-4": "claude-haiku-4-5",
  "gpt-5": "gpt-5.4",
  "gpt-5-codex-mini": "gpt-5-codex",
  "gpt-5.2": "gpt-5.4",
  "gpt-5.2-codex": "gpt-5-codex",
  "gpt-5.1": "gpt-5.4",
  "gpt-5.1-codex-max": "gpt-5-codex",
  "gpt-5.4-mini": "gpt-5.4",
};

export function resolvePricingModel(model?: string | null): PricingModel | null {
  if (!model) {
    return null;
  }

  const normalized = MODEL_ALIASES[model] ?? model;
  return PRICING_MODELS[normalized] ?? null;
}

export function accumulateCostAccounting(
  params: AccumulateCostAccountingParams,
): CostAccountingSnapshot {
  const previous = params.previous ?? null;
  const usageDelta = normalizeUsageTotals(params.usageDelta);
  const totals = {
    inputTokens: (previous?.inputTokens ?? 0) + usageDelta.inputTokens,
    outputTokens: (previous?.outputTokens ?? 0) + usageDelta.outputTokens,
    cacheReadTokens: (previous?.cacheReadTokens ?? 0) + usageDelta.cacheReadTokens,
    cacheCreationTokens:
      (previous?.cacheCreationTokens ?? 0) + usageDelta.cacheCreationTokens,
  };

  const components = mergeComponents(previous?.components ?? [], params.components ?? []);

  if (
    typeof params.authoritativeTotalCostUsd === "number" &&
    Number.isFinite(params.authoritativeTotalCostUsd)
  ) {
    return {
      ...totals,
      totalCostUsd: Math.max(params.authoritativeTotalCostUsd, 0),
      mode: "authoritative_total",
      coverage: "full",
      source: params.source,
      components,
    };
  }

  const billingMode = resolveBillingMode(params.provider, params.runtime);
  if (billingMode === "plan_included") {
    return {
      ...totals,
      totalCostUsd: previous?.totalCostUsd ?? 0,
      mode: "plan_included",
      coverage: "none",
      source: params.source,
      components,
    };
  }

  if (billingMode === "api_pricing") {
    const estimatedDelta = estimateCostUsd(params.requestedModel, usageDelta);
    if (estimatedDelta !== null) {
      return {
        ...totals,
        totalCostUsd: (previous?.totalCostUsd ?? 0) + estimatedDelta,
        mode: "estimated_api_pricing",
        coverage: "full",
        source: params.source,
        components,
      };
    }
  }

  return {
    ...totals,
    totalCostUsd: previous?.totalCostUsd ?? 0,
    mode: "unpriced",
    coverage: "none",
    source: params.source,
    components,
  };
}

export function estimateCostUsd(
  model: string | undefined,
  usage: Partial<CostUsageTotals>,
): number | null {
  const pricing = resolvePricingModel(model);
  if (!pricing) {
    return null;
  }

  const normalized = normalizeUsageTotals(usage);
  const input =
    (normalized.inputTokens / 1_000_000) * pricing.inputPerMillion;
  const output =
    (normalized.outputTokens / 1_000_000) * pricing.outputPerMillion;
  const cacheReadRate =
    pricing.provider === "openai"
      ? pricing.cachedInputPerMillion ?? 0
      : pricing.cacheReadPerMillion ?? 0;
  const cacheRead =
    (normalized.cacheReadTokens / 1_000_000) * cacheReadRate;
  const cacheWrite =
    (normalized.cacheCreationTokens / 1_000_000) *
    (pricing.cacheWritePerMillion ?? 0);

  return input + output + cacheRead + cacheWrite;
}

export function serializeCostAccounting(
  snapshot: CostAccountingSnapshot | null | undefined,
): CostAccountingData | undefined {
  if (!snapshot) {
    return undefined;
  }

  return {
    total_cost_usd: snapshot.totalCostUsd,
    input_tokens: snapshot.inputTokens,
    output_tokens: snapshot.outputTokens,
    cache_read_tokens: snapshot.cacheReadTokens,
    cache_creation_tokens: snapshot.cacheCreationTokens,
    mode: snapshot.mode,
    coverage: snapshot.coverage,
    source: snapshot.source,
    components: snapshot.components.map((component) => ({
      model: component.model,
      input_tokens: component.inputTokens,
      output_tokens: component.outputTokens,
      cache_read_tokens: component.cacheReadTokens,
      cache_creation_tokens: component.cacheCreationTokens,
      cost_usd: component.costUsd,
      source: component.source,
    })),
  };
}

function normalizeUsageTotals(
  usage: Partial<CostUsageTotals> | undefined,
): CostUsageTotals {
  return {
    inputTokens:
      typeof usage?.inputTokens === "number" && Number.isFinite(usage.inputTokens)
        ? Math.max(usage.inputTokens, 0)
        : 0,
    outputTokens:
      typeof usage?.outputTokens === "number" && Number.isFinite(usage.outputTokens)
        ? Math.max(usage.outputTokens, 0)
        : 0,
    cacheReadTokens:
      typeof usage?.cacheReadTokens === "number" &&
      Number.isFinite(usage.cacheReadTokens)
        ? Math.max(usage.cacheReadTokens, 0)
        : 0,
    cacheCreationTokens:
      typeof usage?.cacheCreationTokens === "number" &&
      Number.isFinite(usage.cacheCreationTokens)
        ? Math.max(usage.cacheCreationTokens, 0)
        : 0,
  };
}

function mergeComponents(
  previous: CostAccountingComponent[],
  next: CostAccountingComponentInput[],
): CostAccountingComponent[] {
  const merged = new Map<string, CostAccountingComponent>();

  for (const component of previous) {
    merged.set(componentKey(component.model, component.source), {
      ...component,
    });
  }

  for (const component of next) {
    const key = componentKey(component.model, component.source);
    const current = merged.get(key);
    merged.set(key, {
      model: component.model,
      source: component.source,
      inputTokens:
        (current?.inputTokens ?? 0) + normalizeCount(component.inputTokens),
      outputTokens:
        (current?.outputTokens ?? 0) + normalizeCount(component.outputTokens),
      cacheReadTokens:
        (current?.cacheReadTokens ?? 0) +
        normalizeCount(component.cacheReadTokens),
      cacheCreationTokens:
        (current?.cacheCreationTokens ?? 0) +
        normalizeCount(component.cacheCreationTokens),
      costUsd: (current?.costUsd ?? 0) + normalizeFloat(component.costUsd),
    });
  }

  return [...merged.values()];
}

function componentKey(model: string, source: string): string {
  return `${model}::${source}`;
}

function normalizeCount(value: number | undefined): number {
  return typeof value === "number" && Number.isFinite(value)
    ? Math.max(value, 0)
    : 0;
}

function normalizeFloat(value: number | undefined): number {
  return typeof value === "number" && Number.isFinite(value)
    ? Math.max(value, 0)
    : 0;
}

function resolveBillingMode(
  provider: string,
  runtime: string,
): "api_pricing" | "plan_included" | "unpriced" {
  if (provider === "anthropic" || provider === "openai") {
    return "api_pricing";
  }
  if (provider === "codex" || runtime === "codex") {
    return "plan_included";
  }
  return "unpriced";
}
