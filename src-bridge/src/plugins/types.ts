export type PluginKind =
  | "RolePlugin"
  | "ToolPlugin"
  | "WorkflowPlugin"
  | "IntegrationPlugin"
  | "ReviewPlugin";

export type PluginRuntime = "declarative" | "mcp" | "go-plugin" | "wasm";
export type PluginSourceType = "builtin" | "local" | "git" | "npm" | "catalog";
export type PluginTrustState = "unknown" | "verified" | "untrusted";
export type PluginApprovalState = "not-required" | "pending" | "approved" | "rejected";
export type PluginLifecycleOperation =
  | "install"
  | "enable"
  | "activate"
  | "deactivate"
  | "disable"
  | "uninstall"
  | "update";
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
    module?: string;
    abiVersion?: string;
    capabilities?: string[];
    config?: Record<string, unknown>;
    env?: Record<string, string>;
    workflow?: {
      process: "sequential" | "hierarchical" | "event-driven";
      roles?: Array<{ id: string }>;
      steps: Array<{
        id: string;
        role: string;
        action: "agent" | "review" | "task";
        next?: string[];
      }>;
      triggers?: Array<{ event?: string }>;
      limits?: { maxRetries?: number };
    };
    review?: {
      entrypoint?: string;
      triggers: {
        events: string[];
        filePatterns?: string[];
      };
      output: {
        format: string;
      };
    };
  };
  permissions?: Record<string, unknown>;
  source?: {
    type: PluginSourceType;
    path?: string;
    repository?: string;
    ref?: string;
    package?: string;
    version?: string;
    registry?: string;
    catalog?: string;
    entry?: string;
    digest?: string;
    signature?: string;
    trust?: {
      status: PluginTrustState;
      approvalState?: PluginApprovalState;
      source?: string;
      verifiedAt?: string;
      approvedBy?: string;
      approvedAt?: string;
      reason?: string;
    };
    release?: {
      version?: string;
      channel?: string;
      artifact?: string;
      notesUrl?: string;
      publishedAt?: string;
      availableVersion?: string;
    };
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
  discovered_tools?: string[];
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

export type EventSink = {
  send(event: { task_id: string; session_id: string; timestamp_ms: number; type: string; data: unknown }): void;
};

export interface ToolPluginManagerOptions {
  reporter?: PluginRuntimeReporter;
  mcpHub?: import("../mcp/client-hub.js").MCPClientHub;
  streamer?: EventSink;
}
