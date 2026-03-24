import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import type { MCPServerConfig } from "./types.js";

/**
 * Replace `${VAR_NAME}` patterns in values with corresponding `process.env` entries.
 * Missing variables resolve to empty string.
 */
export function interpolateEnv(env: Record<string, string>): Record<string, string> {
  return Object.fromEntries(
    Object.entries(env).map(([key, value]) => [
      key,
      value.replace(/\$\{([^}]+)\}/g, (_, name: string) => process.env[name] ?? ""),
    ]),
  );
}

/**
 * Create a StdioClientTransport from an MCP server config.
 * Spawns the configured command with piped stdio and interpolated environment variables.
 */
export function createStdioTransport(config: MCPServerConfig): StdioClientTransport {
  if (!config.command) {
    throw new Error(`MCP server ${config.pluginId} is missing spec.command for stdio transport`);
  }

  const processEnv = config.env ? interpolateEnv(config.env) : undefined;

  // Merge process.env with plugin-specific env vars, filtering out undefined values
  let mergedEnv: Record<string, string> | undefined;
  if (processEnv) {
    mergedEnv = {};
    for (const [k, v] of Object.entries(process.env)) {
      if (v !== undefined) mergedEnv[k] = v;
    }
    Object.assign(mergedEnv, processEnv);
  }

  return new StdioClientTransport({
    command: config.command,
    args: config.args ?? [],
    env: mergedEnv,
  });
}
