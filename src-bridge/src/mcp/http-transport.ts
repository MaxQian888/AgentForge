import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import type { MCPServerConfig } from "./types.js";

/**
 * Create a StreamableHTTPClientTransport for a remote MCP server.
 */
export function createHttpTransport(config: MCPServerConfig): StreamableHTTPClientTransport {
  if (!config.url) {
    throw new Error(`MCP server ${config.pluginId} is missing spec.url for http transport`);
  }

  return new StreamableHTTPClientTransport(new URL(config.url));
}
