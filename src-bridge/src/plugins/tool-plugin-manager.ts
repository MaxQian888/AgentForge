import { spawn, type ChildProcess } from "node:child_process";
import { ToolPluginManifestSchema } from "./schema.js";
import { createDefaultPluginRuntimeReporter } from "./reporter.js";
import type {
  PluginLifecycleState,
  PluginManifest,
  PluginRecord,
  PluginRuntimeReporter,
  ToolPluginManagerOptions,
} from "./types.js";

type ProcessEntry = {
  child: ChildProcess;
  intentionalStop: boolean;
};

export class ToolPluginManager {
  private readonly records = new Map<string, PluginRecord>();
  private readonly processes = new Map<string, ProcessEntry>();
  private readonly reporter: PluginRuntimeReporter;

  constructor(options: ToolPluginManagerOptions = {}) {
    this.reporter = options.reporter ?? createDefaultPluginRuntimeReporter();
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
    record.lifecycle_state = "enabled";
    record.last_error = undefined;
    await this.report(record);
    return { ...record };
  }

  async disable(pluginId: string): Promise<PluginRecord> {
    const record = this.requireRecord(pluginId);
    await this.stopProcess(pluginId, true);
    record.lifecycle_state = "disabled";
    await this.report(record);
    return { ...record };
  }

  async activate(pluginId: string): Promise<PluginRecord> {
    const record = this.requireRecord(pluginId);
    if (record.lifecycle_state === "disabled") {
      throw new Error(`Plugin ${pluginId} is disabled and cannot be activated`);
    }

    record.lifecycle_state = "activating";
    await this.report(record);

    if (record.spec.transport === "stdio") {
      const command = record.spec.command;
      if (!command) {
        throw new Error(`Plugin ${pluginId} is missing spec.command`);
      }
      await this.stopProcess(pluginId, true);
      const child = spawn(command, record.spec.args ?? [], {
        stdio: "ignore",
      });
      this.processes.set(pluginId, { child, intentionalStop: false });
      child.once("exit", (code, signal) => {
        void this.handleProcessExit(pluginId, code, signal);
      });
    }

    this.markHealthy(record, "active");
    await this.report(record);
    return { ...record };
  }

  async checkHealth(pluginId: string): Promise<PluginRecord> {
    const record = this.requireRecord(pluginId);
    const processEntry = this.processes.get(pluginId);

    if (record.spec.transport === "stdio") {
      if (!processEntry || processEntry.child.exitCode !== null || processEntry.child.killed) {
        record.lifecycle_state = "degraded";
        record.last_error = record.last_error ?? "tool runtime process is not running";
      } else {
        this.markHealthy(record, "active");
      }
    } else {
      this.markHealthy(record, record.lifecycle_state === "disabled" ? "disabled" : "active");
    }

    await this.report(record);
    return { ...record };
  }

  async restart(pluginId: string): Promise<PluginRecord> {
    const record = this.requireRecord(pluginId);
    record.restart_count += 1;
    await this.stopProcess(pluginId, true);
    record.lifecycle_state = "enabled";
    await this.report(record);
    return this.activate(pluginId);
  }

  async dispose(): Promise<void> {
    const ids = Array.from(this.processes.keys());
    await Promise.all(ids.map((pluginId) => this.stopProcess(pluginId, true)));
  }

  private async stopProcess(pluginId: string, intentionalStop: boolean): Promise<void> {
    const processEntry = this.processes.get(pluginId);
    if (!processEntry) {
      return;
    }

    processEntry.intentionalStop = intentionalStop;
    processEntry.child.kill();
    this.processes.delete(pluginId);
  }

  private async handleProcessExit(
    pluginId: string,
    code: number | null,
    signal: NodeJS.Signals | null,
  ): Promise<void> {
    const processEntry = this.processes.get(pluginId);
    this.processes.delete(pluginId);

    if (processEntry?.intentionalStop) {
      return;
    }

    const record = this.records.get(pluginId);
    if (!record) {
      return;
    }

    record.lifecycle_state = "degraded";
    record.last_error = `process exited unexpectedly (code=${code ?? "null"}, signal=${signal ?? "null"})`;
    await this.report(record);
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

  private requireRecord(pluginId: string): PluginRecord {
    const record = this.records.get(pluginId);
    if (!record) {
      throw new Error(`Unknown plugin ${pluginId}`);
    }
    return record;
  }
}
