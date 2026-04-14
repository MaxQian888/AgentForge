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

export type CliPromptTransport = "stdin" | "positional" | "prompt_flag";
export type CliOutputMode = "text" | "json" | "stream-json";
export type RuntimeLifecycleStage = "active" | "sunsetting" | "sunset";

export interface RuntimeLifecycleMetadata {
  stage: RuntimeLifecycleStage;
  sunset_at?: string;
  replacement_runtime?: AgentRuntimeKey;
  message?: string;
}

export interface CliRuntimeLaunchContract {
  prompt_transport: CliPromptTransport;
  output_mode: CliOutputMode;
  supported_output_modes: CliOutputMode[];
  supported_approval_modes: string[];
  additional_directories: boolean;
  env_overrides: boolean;
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
  cli_launch?: CliRuntimeLaunchContract;
  lifecycle?: RuntimeLifecycleMetadata;
}

const CLI_RUNTIME_METADATA: Partial<
  Record<
    AgentRuntimeKey,
    {
      cli_launch: CliRuntimeLaunchContract;
      lifecycle?: RuntimeLifecycleMetadata;
    }
  >
> = {
  cursor: {
    cli_launch: {
      prompt_transport: "positional",
      output_mode: "stream-json",
      supported_output_modes: ["text", "json", "stream-json"],
      supported_approval_modes: ["default", "ask", "plan", "yolo"],
      additional_directories: false,
      env_overrides: false,
    },
  },
  gemini: {
    cli_launch: {
      prompt_transport: "prompt_flag",
      output_mode: "stream-json",
      supported_output_modes: ["text", "json", "stream-json"],
      supported_approval_modes: ["default", "auto_edit", "yolo", "plan"],
      additional_directories: true,
      env_overrides: false,
    },
  },
  qoder: {
    cli_launch: {
      prompt_transport: "prompt_flag",
      output_mode: "stream-json",
      supported_output_modes: ["text", "json", "stream-json"],
      supported_approval_modes: ["default", "yolo"],
      additional_directories: false,
      env_overrides: false,
    },
  },
  iflow: {
    cli_launch: {
      prompt_transport: "prompt_flag",
      output_mode: "text",
      supported_output_modes: ["text"],
      supported_approval_modes: ["default", "yolo"],
      additional_directories: true,
      env_overrides: false,
    },
    lifecycle: {
      stage: "sunsetting",
      sunset_at: "2026-04-17T00:00:00+08:00",
      replacement_runtime: "qoder",
      message: "iFlow CLI is scheduled to shut down on April 17, 2026 (Beijing Time). Migrate to Qoder.",
    },
  },
};

const runtimeProfiles = (rawProfiles as RuntimeProfile[]).map((profile) => {
  const runtimeMetadata = CLI_RUNTIME_METADATA[profile.key];
  return {
    ...profile,
    compatible_providers: [...profile.compatible_providers],
    model_options: profile.model_options ? [...profile.model_options] : undefined,
    auth: profile.auth
      ? {
          ...profile.auth,
          env_vars: profile.auth.env_vars ? [...profile.auth.env_vars] : undefined,
        }
      : undefined,
    cli_launch: runtimeMetadata?.cli_launch
      ? {
          ...runtimeMetadata.cli_launch,
          supported_output_modes: [...runtimeMetadata.cli_launch.supported_output_modes],
          supported_approval_modes: [...runtimeMetadata.cli_launch.supported_approval_modes],
        }
      : undefined,
    lifecycle: runtimeMetadata?.lifecycle
      ? { ...runtimeMetadata.lifecycle }
      : undefined,
  };
});

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
    cli_launch: profile.cli_launch
      ? {
          ...profile.cli_launch,
          supported_output_modes: [...profile.cli_launch.supported_output_modes],
          supported_approval_modes: [...profile.cli_launch.supported_approval_modes],
        }
      : undefined,
    lifecycle: profile.lifecycle ? { ...profile.lifecycle } : undefined,
  }));
}
