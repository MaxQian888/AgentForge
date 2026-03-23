export type PluginKind =
  | "RolePlugin"
  | "ToolPlugin"
  | "WorkflowPlugin"
  | "IntegrationPlugin"
  | "ReviewPlugin";

export type PluginRuntime = "declarative" | "mcp" | "go-plugin";
export type PluginSourceType = "builtin" | "local";
export type PluginLifecycleState =
  | "installed"
  | "enabled"
  | "activating"
  | "active"
  | "degraded"
  | "disabled";
export type PluginRuntimeHost = "go-orchestrator" | "ts-bridge";

export interface PluginManifest {
  apiVersion: string;
  kind: PluginKind;
  metadata: {
    id: string;
    name: string;
    version: string;
    description?: string;
    tags?: string[];
  };
  spec: {
    runtime: PluginRuntime;
    transport?: string;
    command?: string;
    args?: string[];
    url?: string;
    binary?: string;
    config?: Record<string, unknown>;
  };
  permissions?: Record<string, unknown>;
  source?: {
    type: PluginSourceType;
    path?: string;
  };
}

export interface PluginRecord {
  apiVersion: string;
  kind: PluginKind;
  metadata: PluginManifest["metadata"];
  spec: PluginManifest["spec"];
  permissions: Record<string, unknown>;
  source: PluginManifest["source"];
  lifecycle_state: PluginLifecycleState;
  runtime_host: PluginRuntimeHost;
  last_health_at?: string;
  last_error?: string;
  restart_count: number;
}

export interface PluginRuntimeUpdate {
  plugin_id: string;
  host: PluginRuntimeHost;
  lifecycle_state: PluginLifecycleState;
  last_health_at?: string;
  last_error?: string;
  restart_count: number;
}

export interface PluginRuntimeReporter {
  report(update: PluginRuntimeUpdate): Promise<void>;
}

export interface ToolPluginManagerOptions {
  reporter?: PluginRuntimeReporter;
}
