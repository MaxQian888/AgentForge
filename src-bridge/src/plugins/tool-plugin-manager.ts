import { ToolPluginManifestSchema } from "./schema.js";
import { createDefaultPluginRuntimeReporter } from "./reporter.js";
import { MCPClientHub } from "../mcp/client-hub.js";
import type { MCPCapabilitySurface, MCPPromptResult, MCPResourceResult, MCPToolCallResult } from "../mcp/types.js";
import type {
  EventSink,
  MCPCapabilitySnapshot,
  MCPInteractionOperation,
  PluginLifecycleState,
  PluginManifest,
  PluginRecord,
  PluginRuntimeReporter,
  ToolPluginManagerOptions,
} from "./types.js";

export class ToolPluginManager {
  private readonly records = new Map<string, PluginRecord>();
  private readonly reporter: PluginRuntimeReporter;
  private readonly mcpHub: MCPClientHub;
  private readonly streamer: EventSink | undefined;

  constructor(options: ToolPluginManagerOptions = {}) {
    this.reporter = options.reporter ?? createDefaultPluginRuntimeReporter();
    this.mcpHub = options.mcpHub ?? new MCPClientHub();
    this.streamer = options.streamer;
  }

  /** Access the underlying MCP Client Hub. */
  get hub(): MCPClientHub {
    return this.mcpHub;
  }

  async register(input: PluginManifest): Promise<PluginRecord> {
    const manifest = ToolPluginManifestSchema.parse(input);
    const record: PluginRecord = {
      apiVersion: manifest.apiVersion,
      kind: manifest.kind,
      metadata: manifest.metadata,
      spec: manifest.spec,
      permissions: manifest.permissions ?? {},
      source: manifest.source,
      lifecycle_state: "installed",
      runtime_host: "ts-bridge",
      restart_count: 0,
    };
    this.records.set(record.metadata.id, record);
    return record;
  }

  list(): PluginRecord[] {
    return Array.from(this.records.values()).map((record) => ({ ...record }));
  }

  async enable(pluginId: string): Promise<PluginRecord> {
    const record = this.requireRecord(pluginId);
    const previousState = record.lifecycle_state;
    record.lifecycle_state = "enabled";
    record.last_error = undefined;
    await this.report(record);
    this.emitStatusChange(pluginId, previousState, "enabled");
    return { ...record };
  }

  async disable(pluginId: string): Promise<PluginRecord> {
    const record = this.requireRecord(pluginId);
    const previousState = record.lifecycle_state;
    await this.mcpHub.disconnectServer(pluginId);
    record.lifecycle_state = "disabled";
    record.discovered_tools = undefined;
    await this.report(record);
    this.emitStatusChange(pluginId, previousState, "disabled");
    return { ...record };
  }

  async activate(pluginId: string): Promise<PluginRecord> {
    const record = this.requireRecord(pluginId);
    if (record.lifecycle_state === "disabled") {
      throw new Error(`Plugin ${pluginId} is disabled and cannot be activated`);
    }

    const previousState = record.lifecycle_state;
    record.lifecycle_state = "activating";
    await this.report(record);
    this.emitStatusChange(pluginId, previousState, "activating");

    try {
      const tools = await this.mcpHub.connectServer(pluginId, {
        pluginId,
        transport: (record.spec.transport ?? "stdio") as "stdio" | "http",
        command: record.spec.command,
        args: record.spec.args,
        url: record.spec.url,
        env: record.spec.env,
      });

      record.discovered_tools = tools.map((t) => t.name);
      this.applyCapabilitySurface(record, await this.mcpHub.refreshCapabilitySurface(pluginId));
      this.markHealthy(record, "active");
      await this.report(record);
      this.emitStatusChange(pluginId, "activating", "active");
      return { ...record };
    } catch (err) {
      record.lifecycle_state = "degraded";
      record.last_error = err instanceof Error ? err.message : String(err);
      await this.report(record);
      this.emitStatusChange(pluginId, "activating", "degraded", record.last_error);
      this.emitCrashEvent(pluginId, record.last_error);
      throw err;
    }
  }

  async checkHealth(pluginId: string): Promise<PluginRecord> {
    const record = this.requireRecord(pluginId);
    const status = this.mcpHub.getServerStatus(pluginId);

    if (status) {
      if (status.status === "active") {
        this.markHealthy(record, "active");
      } else {
        record.lifecycle_state = status.status === "disconnected" ? "degraded" : status.status as PluginLifecycleState;
        record.last_error = status.last_error ?? "MCP server not active";
      }
    } else if (record.lifecycle_state !== "disabled" && record.lifecycle_state !== "installed" && record.lifecycle_state !== "enabled") {
      record.lifecycle_state = "degraded";
      record.last_error = record.last_error ?? "MCP server not connected";
    }

    await this.report(record);
    return { ...record };
  }

  async restart(pluginId: string): Promise<PluginRecord> {
    const record = this.requireRecord(pluginId);
    const previousState = record.lifecycle_state;
    record.restart_count += 1;
    await this.mcpHub.disconnectServer(pluginId);
    record.lifecycle_state = "enabled";
    await this.report(record);
    this.emitStatusChange(pluginId, previousState, "enabled", undefined, true);
    return this.activate(pluginId);
  }

  async refreshCapabilitySurface(pluginId: string): Promise<PluginRecord> {
    const record = this.requireRecord(pluginId);
    const surface = await this.mcpHub.refreshCapabilitySurface(pluginId);
    this.applyCapabilitySurface(record, surface);
    this.updateLatestInteraction(record, "refresh", "succeeded", pluginId, this.summarizeRefresh(surface));
    await this.report(record);
    return { ...record };
  }

  async invokeTool(
    pluginId: string,
    toolName: string,
    args: Record<string, unknown>,
  ): Promise<MCPToolCallResult> {
    const record = this.requireRecord(pluginId);
    try {
      const result = await this.mcpHub.callTool(pluginId, toolName, args);
      this.updateLatestInteraction(record, "call_tool", "succeeded", toolName, this.summarizeToolResult(result));
      await this.report(record);
      return result;
    } catch (err) {
      this.updateInteractionFailure(record, "call_tool", toolName, err);
      await this.report(record);
      throw err;
    }
  }

  async readResource(pluginId: string, uri: string): Promise<MCPResourceResult> {
    const record = this.requireRecord(pluginId);
    try {
      const result = await this.mcpHub.readResource(pluginId, uri);
      this.updateLatestInteraction(record, "read_resource", "succeeded", uri, this.summarizeResourceResult(result));
      await this.report(record);
      return result;
    } catch (err) {
      this.updateInteractionFailure(record, "read_resource", uri, err);
      await this.report(record);
      throw err;
    }
  }

  async getPrompt(
    pluginId: string,
    promptName: string,
    args?: Record<string, string>,
  ): Promise<MCPPromptResult> {
    const record = this.requireRecord(pluginId);
    try {
      const result = await this.mcpHub.getPrompt(pluginId, promptName, args);
      this.updateLatestInteraction(record, "get_prompt", "succeeded", promptName, this.summarizePromptResult(result));
      await this.report(record);
      return result;
    } catch (err) {
      this.updateInteractionFailure(record, "get_prompt", promptName, err);
      await this.report(record);
      throw err;
    }
  }

  async dispose(): Promise<void> {
    await this.mcpHub.dispose();
  }

  private markHealthy(record: PluginRecord, state: PluginLifecycleState): void {
    record.lifecycle_state = state;
    record.last_health_at = new Date().toISOString();
    record.last_error = undefined;
  }

  private async report(record: PluginRecord): Promise<void> {
    await this.reporter.report({
      plugin_id: record.metadata.id,
      host: "ts-bridge",
      lifecycle_state: record.lifecycle_state,
      last_health_at: record.last_health_at,
      last_error: record.last_error,
      restart_count: record.restart_count,
      runtime_metadata: record.mcp_capability_snapshot
        ? {
            mcp: {
              transport: record.mcp_capability_snapshot.transport,
              last_discovery_at: record.mcp_capability_snapshot.last_discovery_at,
              tool_count: record.mcp_capability_snapshot.tool_count,
              resource_count: record.mcp_capability_snapshot.resource_count,
              prompt_count: record.mcp_capability_snapshot.prompt_count,
              latest_interaction: record.mcp_capability_snapshot.latest_interaction,
            },
          }
        : undefined,
    });
  }

  private applyCapabilitySurface(record: PluginRecord, surface: MCPCapabilitySurface): void {
    record.discovered_tools = surface.tools.map((tool) => tool.name);
    record.mcp_capability_snapshot = {
      transport: surface.transport,
      last_discovery_at: surface.refreshed_at,
      tool_count: surface.tool_count,
      resource_count: surface.resource_count,
      prompt_count: surface.prompt_count,
      tools: surface.tools.map((tool) => ({
        name: tool.name,
        description: tool.description,
      })),
      resources: surface.resources.map((resource) => ({
        uri: resource.uri,
        name: resource.name,
      })),
      prompts: surface.prompts.map((prompt) => ({
        name: prompt.name,
        description: prompt.description,
      })),
      latest_interaction: record.mcp_capability_snapshot?.latest_interaction,
    };
  }

  private updateLatestInteraction(
    record: PluginRecord,
    operation: MCPInteractionOperation,
    status: "succeeded" | "failed",
    target: string,
    summary?: string,
  ): void {
    const snapshot = this.ensureCapabilitySnapshot(record);
    snapshot.latest_interaction = {
      operation,
      status,
      at: new Date().toISOString(),
      target,
      summary,
    };
  }

  private updateInteractionFailure(
    record: PluginRecord,
    operation: MCPInteractionOperation,
    target: string,
    err: unknown,
  ): void {
    const message = err instanceof Error ? err.message : String(err);
    const snapshot = this.ensureCapabilitySnapshot(record);
    snapshot.latest_interaction = {
      operation,
      status: "failed",
      at: new Date().toISOString(),
      target,
      error_code: "mcp_interaction_failed",
      error_message: message,
      summary: message,
    };
  }

  private ensureCapabilitySnapshot(record: PluginRecord): MCPCapabilitySnapshot {
    if (!record.mcp_capability_snapshot) {
      record.mcp_capability_snapshot = {
        transport: (record.spec.transport ?? "stdio") as "stdio" | "http",
        tool_count: 0,
        resource_count: 0,
        prompt_count: 0,
        tools: [],
        resources: [],
        prompts: [],
      };
    }
    return record.mcp_capability_snapshot;
  }

  private summarizeRefresh(surface: MCPCapabilitySurface): string {
    return `tools=${surface.tool_count}, resources=${surface.resource_count}, prompts=${surface.prompt_count}`;
  }

  private summarizeToolResult(result: MCPToolCallResult): string | undefined {
    const text = result.content.find((item) => typeof item.text === "string")?.text;
    return text?.slice(0, 120);
  }

  private summarizeResourceResult(result: MCPResourceResult): string | undefined {
    return result.contents[0]?.uri;
  }

  private summarizePromptResult(result: MCPPromptResult): string | undefined {
    return result.description ?? result.messages[0]?.content?.text?.slice(0, 120);
  }

  private emitStatusChange(
    pluginId: string,
    oldStatus: string,
    newStatus: string,
    error?: string,
    isRestart?: boolean,
  ): void {
    if (!this.streamer) return;
    this.streamer.send({
      task_id: "__plugin__",
      session_id: "",
      timestamp_ms: Date.now(),
      type: "tool.status_change",
      data: {
        plugin_id: pluginId,
        old_status: oldStatus,
        new_status: newStatus,
        error: error ?? undefined,
        is_restart: isRestart ?? false,
      },
    });
  }

  private emitCrashEvent(pluginId: string, error: string): void {
    if (!this.streamer) return;
    this.streamer.send({
      task_id: "__plugin__",
      session_id: "",
      timestamp_ms: Date.now(),
      type: "error",
      data: {
        code: "MCP_SERVER_CRASHED",
        message: error,
        plugin_id: pluginId,
        retryable: true,
      },
    });
  }

  private requireRecord(pluginId: string): PluginRecord {
    const record = this.records.get(pluginId);
    if (!record) {
      throw new Error(`Unknown plugin ${pluginId}`);
    }
    return record;
  }
}
