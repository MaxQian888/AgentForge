import rawProfiles from "../../../src-go/internal/service/coding_agent_backend_profiles.json";
import type { AgentRuntimeKey } from "../types.js";

export interface RuntimeProfileCommand {
  default_command?: string;
  env_var?: string;
  install_hint?: string;
}

export interface RuntimeProfileAuth {
  mode?: "none" | "env_any";
  env_vars?: string[];
  message?: string;
}

export interface RuntimeProfile {
  key: AgentRuntimeKey;
  label: string;
  adapter_family: "dedicated" | "cli";
  default_provider: string;
  compatible_providers: string[];
  default_model?: string;
  model_options?: string[];
  strict_model_options?: boolean;
  command?: RuntimeProfileCommand;
  auth?: RuntimeProfileAuth;
  supported_features: string[];
}

const runtimeProfiles = (rawProfiles as RuntimeProfile[]).map((profile) => ({
  ...profile,
  compatible_providers: [...profile.compatible_providers],
  model_options: profile.model_options ? [...profile.model_options] : undefined,
  auth: profile.auth
    ? {
        ...profile.auth,
        env_vars: profile.auth.env_vars ? [...profile.auth.env_vars] : undefined,
      }
    : undefined,
}));

export const RUNTIME_PROFILES: RuntimeProfile[] = runtimeProfiles;

export function getRuntimeProfile(key: AgentRuntimeKey): RuntimeProfile {
  const profile = runtimeProfiles.find((item) => item.key === key);
  if (!profile) {
    throw new Error(`Unknown runtime profile: ${key}`);
  }
  return profile;
}

export function getRuntimeProfiles(): RuntimeProfile[] {
  return runtimeProfiles.map((profile) => ({
    ...profile,
    compatible_providers: [...profile.compatible_providers],
    model_options: profile.model_options ? [...profile.model_options] : undefined,
    auth: profile.auth
      ? {
          ...profile.auth,
          env_vars: profile.auth.env_vars ? [...profile.auth.env_vars] : undefined,
        }
      : undefined,
  }));
}
