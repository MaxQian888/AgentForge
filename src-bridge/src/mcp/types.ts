import type { Client } from "@modelcontextprotocol/sdk/client/index.js";
import type { Transport } from "@modelcontextprotocol/sdk/shared/transport.js";
import type { Prompt, Tool, Resource } from "@modelcontextprotocol/sdk/types.js";

/** Configuration for connecting to an MCP server. */
export interface MCPServerConfig {
  /** Unique plugin identifier. */
  pluginId: string;
  /** Transport type: "stdio" spawns a local process, "http" connects to a remote endpoint. */
  transport: "stdio" | "http";
  /** Command to spawn (stdio only). */
  command?: string;
  /** Arguments for the command (stdio only). */
  args?: string[];
  /** URL endpoint (http only). */
  url?: string;
  /** Environment variables to pass to the spawned process. Supports ${VAR} interpolation. */
  env?: Record<string, string>;
}

/** Runtime state of a single MCP server connection. */
export type MCPServerState = "connecting" | "active" | "degraded" | "disconnected";

/** Status snapshot of an MCP server for heartbeat reporting. */
export interface MCPServerStatus {
  id: string;
  status: MCPServerState;
  pid?: number;
  uptime_ms: number;
  tool_count: number;
  resource_count?: number;
  prompt_count?: number;
  last_error?: string;
}

/** Internal entry tracking one connected MCP server. */
export interface MCPClientEntry {
  client: Client;
  transport: Transport;
  config: MCPServerConfig;
  state: MCPServerState;
  pid?: number;
  connectedAt: number;
  tools: Tool[];
  resources: Resource[];
  prompts: Prompt[];
  lastDiscoveryAt?: number;
  lastError?: string;
}

/** Result of an MCP tool call. */
export interface MCPToolCallResult {
  content: Array<{ type: string; text?: string; [key: string]: unknown }>;
  isError?: boolean;
  structuredContent?: unknown;
}

/** Result of reading an MCP resource. */
export interface MCPResourceResult {
  contents: Array<{ uri: string; mimeType?: string; text?: string; blob?: string }>;
}

/** Result of retrieving an MCP prompt. */
export interface MCPPromptResult {
  description?: string;
  messages: Array<{
    role: "user" | "assistant";
    content: { type: string; text?: string; [key: string]: unknown };
  }>;
}

/** Complete discovered MCP capability surface for a plugin. */
export interface MCPCapabilitySurface {
  plugin_id: string;
  transport: MCPServerConfig["transport"];
  tools: Tool[];
  resources: Resource[];
  prompts: Prompt[];
  tool_count: number;
  resource_count: number;
  prompt_count: number;
  refreshed_at: string;
}

/** Log entry for a tool call (emitted as tool.call_log event). */
export interface ToolCallLog {
  plugin_id: string;
  tool_name: string;
  duration_ms: number;
  is_error: boolean;
}

/** Options for MCPClientHub construction. */
export interface MCPClientHubOptions {
  /** Callback invoked after each tool call with timing and status info. */
  onToolCallLog?: (log: ToolCallLog) => void;
  /** Callback invoked when a stdio MCP server process crashes. */
  onProcessCrash?: (pluginId: string, exitCode: number | null) => void;
}
