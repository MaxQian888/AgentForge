export type ProviderCapability = "agent_execution" | "text_generation";

export type BridgeProviderName = "anthropic" | "openai" | "google";

export interface ProviderRegistration {
  name: BridgeProviderName;
  capabilities: ProviderCapability[];
  default_models: Partial<Record<ProviderCapability, string>>;
  credential_env_groups: string[][];
}

export interface ProviderSelectionInput {
  provider?: string;
  model?: string;
}

export interface ResolvedProviderSelection {
  provider: BridgeProviderName;
  model: string;
  capability: ProviderCapability;
  credential_env_groups: string[][];
}

const PROVIDER_REGISTRY: Record<BridgeProviderName, ProviderRegistration> = {
  anthropic: {
    name: "anthropic",
    capabilities: ["agent_execution", "text_generation"],
    default_models: {
      agent_execution: "claude-sonnet-4-5",
      text_generation: "claude-haiku-4-5",
    },
    credential_env_groups: [["ANTHROPIC_API_KEY"], ["ANTHROPIC_AUTH_TOKEN"]],
  },
  openai: {
    name: "openai",
    capabilities: ["text_generation"],
    default_models: {
      text_generation: "gpt-5",
    },
    credential_env_groups: [["OPENAI_API_KEY"]],
  },
  google: {
    name: "google",
    capabilities: ["text_generation"],
    default_models: {
      text_generation: "gemini-2.5-flash",
    },
    credential_env_groups: [["GOOGLE_GENERATIVE_AI_API_KEY"]],
  },
};

const DEFAULT_PROVIDER_BY_CAPABILITY: Record<ProviderCapability, BridgeProviderName> = {
  agent_execution: "anthropic",
  text_generation: "anthropic",
};

export class UnknownProviderError extends Error {
  constructor(provider: string) {
    super(`Unknown provider: ${provider}`);
    this.name = "UnknownProviderError";
  }
}

export class UnsupportedProviderCapabilityError extends Error {
  constructor(provider: string, capability: ProviderCapability) {
    super(`Provider ${provider} does not support ${capability}`);
    this.name = "UnsupportedProviderCapabilityError";
  }
}

export class InvalidProviderModelError extends Error {
  constructor(provider: string, capability: ProviderCapability) {
    super(`Provider ${provider} is missing a default model for ${capability}`);
    this.name = "InvalidProviderModelError";
  }
}

export class MissingProviderCredentialsError extends Error {
  constructor(provider: string, credentialGroups: string[][]) {
    super(
      `Provider ${provider} is missing credentials: ${credentialGroups
        .map((group) => group.join(" + "))
        .join(" or ")}`,
    );
    this.name = "MissingProviderCredentialsError";
  }
}

export function listProviderRegistrations(): ProviderRegistration[] {
  return Object.values(PROVIDER_REGISTRY);
}

export function resolveProviderSelection(
  capability: ProviderCapability,
  input: ProviderSelectionInput = {},
): ResolvedProviderSelection {
  const explicitProvider = input.provider?.trim();
  const normalizedProvider = normalizeProviderName(explicitProvider);
  if (explicitProvider && !normalizedProvider) {
    throw new UnknownProviderError(explicitProvider);
  }

  const providerName = normalizedProvider ?? DEFAULT_PROVIDER_BY_CAPABILITY[capability];
  const registration = PROVIDER_REGISTRY[providerName];
  if (!registration) {
    throw new UnknownProviderError(explicitProvider ?? providerName);
  }
  if (!registration.capabilities.includes(capability)) {
    throw new UnsupportedProviderCapabilityError(providerName, capability);
  }

  const requestedModel = input.model?.trim();
  const defaultModel = registration.default_models[capability];
  const model = requestedModel && requestedModel.length > 0 ? requestedModel : defaultModel;
  if (!model) {
    throw new InvalidProviderModelError(providerName, capability);
  }

  return {
    provider: providerName,
    model,
    capability,
    credential_env_groups: registration.credential_env_groups,
  };
}

export function assertProviderCredentials(
  selection: ResolvedProviderSelection,
  env: Record<string, string | undefined> = process.env,
): void {
  const hasValidCredentialGroup = selection.credential_env_groups.some((group) =>
    group.every((name) => {
      const value = env[name];
      return typeof value === "string" && value.trim().length > 0;
    }),
  );

  if (!hasValidCredentialGroup) {
    throw new MissingProviderCredentialsError(selection.provider, selection.credential_env_groups);
  }
}

function normalizeProviderName(provider: string | undefined): BridgeProviderName | undefined {
  const normalized = provider?.trim().toLowerCase();
  if (!normalized) {
    return undefined;
  }

  if (normalized === "anthropic" || normalized === "openai" || normalized === "google") {
    return normalized;
  }

  return undefined;
}
