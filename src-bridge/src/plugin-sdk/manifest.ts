import { PluginManifestSchema } from "../plugins/schema.js";
import type { PluginManifest } from "../plugins/types.js";

type PluginSource = PluginManifest["source"];
type SharedManifestInput = {
  apiVersion?: string;
  id: string;
  name: string;
  version: string;
  description?: string;
  tags?: string[];
  permissions?: Record<string, unknown>;
  config?: Record<string, unknown>;
  env?: Record<string, string>;
  source?: PluginSource;
};

export type ToolPluginManifestInput = SharedManifestInput & {
  transport: "stdio" | "http";
  command?: string;
  args?: string[];
  url?: string;
  capabilities?: string[];
};

export type ReviewPluginManifestInput = SharedManifestInput & {
  transport: "stdio" | "http";
  command?: string;
  args?: string[];
  url?: string;
  entrypoint?: string;
  triggers: {
    events: string[];
    filePatterns?: string[];
  };
};

const DEFAULT_API_VERSION = "agentforge.dev/v1";
const DEFAULT_REVIEW_ENTRYPOINT = "review:run";

export function createToolPluginManifest(input: ToolPluginManifestInput): PluginManifest {
  return PluginManifestSchema.parse({
    apiVersion: input.apiVersion ?? DEFAULT_API_VERSION,
    kind: "ToolPlugin",
    metadata: {
      id: input.id,
      name: input.name,
      version: input.version,
      description: input.description,
      tags: input.tags,
    },
    spec: {
      runtime: "mcp",
      transport: input.transport,
      command: input.command,
      args: input.args,
      url: input.url,
      capabilities: input.capabilities,
      config: input.config,
      env: input.env,
    },
    permissions: input.permissions ?? {},
    source: input.source,
  }) as PluginManifest;
}

export function createReviewPluginManifest(input: ReviewPluginManifestInput): PluginManifest {
  return PluginManifestSchema.parse({
    apiVersion: input.apiVersion ?? DEFAULT_API_VERSION,
    kind: "ReviewPlugin",
    metadata: {
      id: input.id,
      name: input.name,
      version: input.version,
      description: input.description,
      tags: input.tags,
    },
    spec: {
      runtime: "mcp",
      transport: input.transport,
      command: input.command,
      args: input.args,
      url: input.url,
      config: input.config,
      env: input.env,
      review: {
        entrypoint: input.entrypoint ?? DEFAULT_REVIEW_ENTRYPOINT,
        triggers: input.triggers,
        output: {
          format: "findings/v1",
        },
      },
    },
    permissions: input.permissions ?? {},
    source: input.source,
  }) as PluginManifest;
}

export { DEFAULT_API_VERSION, DEFAULT_REVIEW_ENTRYPOINT };
