import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import type { Tool, Resource, Prompt } from "@modelcontextprotocol/sdk/types.js";
import { createStdioTransport } from "./stdio-transport.js";
import { createHttpTransport } from "./http-transport.js";
import type {
  MCPServerConfig,
  MCPClientEntry,
  MCPServerStatus,
  MCPToolCallResult,
  MCPResourceResult,
  MCPPromptResult,
  MCPCapabilitySurface,
  MCPClientHubOptions,
} from "./types.js";

const CLIENT_NAME = "agentforge-bridge";
const CLIENT_VERSION = "0.1.0";

/**
 * Central hub managing all MCP server connections.
 * Handles connect/disconnect, tool discovery, tool invocation,
 * resource access, crash detection, and call logging.
 */
export class MCPClientHub {
  private readonly clients = new Map<string, MCPClientEntry>();
  private readonly options: MCPClientHubOptions;

  constructor(options: MCPClientHubOptions = {}) {
    this.options = options;
  }

  /** Connect to an MCP server, perform handshake, and discover tools. */
  async connectServer(pluginId: string, config: MCPServerConfig): Promise<Tool[]> {
    // Disconnect existing connection if any
    if (this.clients.has(pluginId)) {
      await this.disconnectServer(pluginId);
    }

    const client = new Client({ name: CLIENT_NAME, version: CLIENT_VERSION });
    const transport =
      config.transport === "stdio"
        ? createStdioTransport(config)
        : createHttpTransport(config);

    const entry: MCPClientEntry = {
      client,
      transport,
      config,
      state: "connecting",
      connectedAt: Date.now(),
      tools: [],
      resources: [],
      prompts: [],
    };

    this.clients.set(pluginId, entry);

    try {
      await client.connect(transport);

      // Attempt to read PID from stdio transport's internal process
      if (config.transport === "stdio") {
        const stdioT = transport as unknown as { _process?: { pid?: number; on?: Function } };
        entry.pid = stdioT._process?.pid;

        // Attach crash detection for stdio processes
        if (stdioT._process?.on) {
          stdioT._process.on("exit", (code: number | null) => {
            if (entry.state === "active" || entry.state === "connecting") {
              entry.state = "degraded";
              entry.lastError = `Process exited unexpectedly with code ${code}`;
              this.options.onProcessCrash?.(pluginId, code);
            }
          });
        }
      }

      // Discover tools
      const surface = await this.refreshCapabilitySurface(pluginId, entry);
      entry.state = "active";

      return surface.tools;
    } catch (err) {
      entry.state = "degraded";
      entry.lastError = err instanceof Error ? err.message : String(err);
      throw err;
    }
  }

  /** Disconnect and clean up an MCP server connection. */
  async disconnectServer(pluginId: string): Promise<void> {
    const entry = this.clients.get(pluginId);
    if (!entry) return;

    try {
      await entry.client.close();
    } catch {
      // Ignore close errors
    }

    entry.state = "disconnected";
    this.clients.delete(pluginId);
  }

  /** Invoke a tool on a specific MCP server, with optional whitelist enforcement. */
  async callTool(
    pluginId: string,
    toolName: string,
    args: Record<string, unknown>,
    opts?: { allowedTools?: Set<string> },
  ): Promise<MCPToolCallResult> {
    // Defense-in-depth: enforce tool whitelist at MCP call level
    if (opts?.allowedTools && !opts.allowedTools.has(toolName)) {
      throw new Error(`Tool "${toolName}" is not in the allowed tools list`);
    }

    const entry = this.requireEntry(pluginId);

    const start = Date.now();
    let isError = false;
    try {
      const result = await entry.client.callTool({ name: toolName, arguments: args });
      isError = Boolean(result.isError);

      return {
        content: (result.content ?? []) as MCPToolCallResult["content"],
        isError,
        structuredContent: result.structuredContent,
      };
    } catch (err) {
      isError = true;
      throw err;
    } finally {
      this.options.onToolCallLog?.({
        plugin_id: pluginId,
        tool_name: toolName,
        duration_ms: Date.now() - start,
        is_error: isError,
      });
    }
  }

  /** List tools discovered on a specific MCP server. */
  listTools(pluginId: string): Tool[] {
    const entry = this.clients.get(pluginId);
    return entry?.tools ?? [];
  }

  /** List prompts discovered on a specific MCP server. */
  listPrompts(pluginId: string): Prompt[] {
    const entry = this.clients.get(pluginId);
    return entry?.prompts ?? [];
  }

  /** Refresh the tool list from a connected server. */
  async refreshTools(pluginId: string): Promise<Tool[]> {
    const entry = this.requireEntry(pluginId);
    const result = await entry.client.listTools();
    entry.tools = result.tools;
    return entry.tools;
  }

  /** Get a prompt definition or preview from a specific MCP server. */
  async getPrompt(
    pluginId: string,
    name: string,
    args?: Record<string, string>,
  ): Promise<MCPPromptResult> {
    const entry = this.requireEntry(pluginId);
    const result = await entry.client.getPrompt({ name, arguments: args });
    return {
      description: result.description,
      messages: (result.messages ?? []) as MCPPromptResult["messages"],
    };
  }

  /** Read a resource from a specific MCP server. */
  async readResource(pluginId: string, uri: string): Promise<MCPResourceResult> {
    const entry = this.requireEntry(pluginId);
    const result = await entry.client.readResource({ uri });
    return {
      contents: (result.contents ?? []) as MCPResourceResult["contents"],
    };
  }

  /** List available resources on a specific MCP server. */
  async listResources(pluginId: string): Promise<Resource[]> {
    const entry = this.requireEntry(pluginId);
    const result = await entry.client.listResources();
    entry.resources = result.resources;
    return entry.resources;
  }

  /** List available prompts on a specific MCP server. */
  async listPromptsFromServer(pluginId: string): Promise<Prompt[]> {
    const entry = this.requireEntry(pluginId);
    const result = await entry.client.listPrompts();
    entry.prompts = result.prompts;
    return entry.prompts;
  }

  /** Refresh the full capability surface from a connected server. */
  async refreshCapabilitySurface(
    pluginId: string,
    existingEntry?: MCPClientEntry,
  ): Promise<MCPCapabilitySurface> {
    const entry = existingEntry ?? this.requireEntry(pluginId);
    const [toolsResult, resourcesResult, promptsResult] = await Promise.all([
      entry.client.listTools(),
      entry.client.listResources(),
      entry.client.listPrompts(),
    ]);
    entry.tools = toolsResult.tools;
    entry.resources = resourcesResult.resources;
    entry.prompts = promptsResult.prompts;
    entry.lastDiscoveryAt = Date.now();

    return {
      plugin_id: pluginId,
      transport: entry.config.transport,
      tools: entry.tools,
      resources: entry.resources,
      prompts: entry.prompts,
      tool_count: entry.tools.length,
      resource_count: entry.resources.length,
      prompt_count: entry.prompts.length,
      refreshed_at: new Date(entry.lastDiscoveryAt).toISOString(),
    };
  }

  /** Aggregate all discovered tools across all active MCP servers. */
  discoverAllTools(): Array<{ pluginId: string; tool: Tool }> {
    const all: Array<{ pluginId: string; tool: Tool }> = [];
    for (const [pluginId, entry] of this.clients) {
      if (entry.state === "active") {
        for (const tool of entry.tools) {
          all.push({ pluginId, tool });
        }
      }
    }
    return all;
  }

  /** Get the status of a specific MCP server. */
  getServerStatus(pluginId: string): MCPServerStatus | null {
    const entry = this.clients.get(pluginId);
    if (!entry) return null;

    return {
      id: pluginId,
      status: entry.state,
      pid: entry.pid,
      uptime_ms: Date.now() - entry.connectedAt,
      tool_count: entry.tools.length,
      resource_count: entry.resources?.length ?? 0,
      prompt_count: entry.prompts?.length ?? 0,
      last_error: entry.lastError,
    };
  }

  /** Get status of all connected MCP servers. */
  getAllServerStatuses(): MCPServerStatus[] {
    const statuses: MCPServerStatus[] = [];
    for (const [pluginId] of this.clients) {
      const status = this.getServerStatus(pluginId);
      if (status) statuses.push(status);
    }
    return statuses;
  }

  /** Check if a specific server is connected and active. */
  isActive(pluginId: string): boolean {
    return this.clients.get(pluginId)?.state === "active";
  }

  /** Get all connected plugin IDs. */
  connectedIds(): string[] {
    return Array.from(this.clients.keys());
  }

  /** Mark a server as degraded (e.g. after process crash detected externally). */
  markDegraded(pluginId: string, error: string): void {
    const entry = this.clients.get(pluginId);
    if (entry) {
      entry.state = "degraded";
      entry.lastError = error;
    }
  }

  /** Close all connections and clean up. */
  async dispose(): Promise<void> {
    const ids = Array.from(this.clients.keys());
    await Promise.all(ids.map((id) => this.disconnectServer(id)));
  }

  private requireEntry(pluginId: string): MCPClientEntry {
    const entry = this.clients.get(pluginId);
    if (!entry) {
      throw new Error(`MCP server ${pluginId} is not connected`);
    }
    if (entry.state !== "active") {
      throw new Error(`MCP server ${pluginId} is not active (state: ${entry.state})`);
    }
    return entry;
  }
}
