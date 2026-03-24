import { ToolPluginManifestSchema } from "./schema.js";
import { createDefaultPluginRuntimeReporter } from "./reporter.js";
import { MCPClientHub } from "../mcp/client-hub.js";
import type {
  EventSink,
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
    });
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
