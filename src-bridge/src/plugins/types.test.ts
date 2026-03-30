import { describe, expect, test } from "bun:test";
import type {
  MCPCapabilitySnapshot,
  PluginManifest,
  PluginRecord,
  PluginRuntimeReporter,
  PluginRuntimeUpdate,
  ToolPluginManagerOptions,
} from "./types.js";

describe("plugin contract types", () => {
  test("accepts plugin manifest, capability snapshot, and lifecycle record shapes", async () => {
    const snapshot: MCPCapabilitySnapshot = {
      transport: "stdio",
      last_discovery_at: "2026-03-30T00:00:00Z",
      tool_count: 2,
      resource_count: 1,
      prompt_count: 1,
      tools: [{ name: "echo", description: "Echoes input" }],
      resources: [{ uri: "memory://tool.echo/report", name: "report" }],
      prompts: [{ name: "review", description: "Review prompt" }],
      latest_interaction: {
        operation: "call_tool",
        status: "succeeded",
        at: "2026-03-30T00:00:00Z",
        target: "echo",
        summary: "Tool call completed",
      },
    };
    const manifest: PluginManifest = {
      apiVersion: "agentforge.dev/v1",
      kind: "ToolPlugin",
      metadata: {
        id: "tool.echo",
        name: "Echo Tool",
        version: "1.0.0",
        tags: ["utility"],
      },
      spec: {
        runtime: "mcp",
        transport: "stdio",
        command: "node",
        args: ["dist/index.js"],
        capabilities: ["echo"],
      },
      permissions: {},
      source: {
        type: "local",
        path: "plugins/tool.echo",
        trust: {
          status: "verified",
          approvalState: "approved",
        },
      },
    };
    const record: PluginRecord = {
      apiVersion: manifest.apiVersion,
      kind: manifest.kind,
      metadata: manifest.metadata,
      spec: manifest.spec,
      permissions: manifest.permissions ?? {},
      source: manifest.source,
      lifecycle_state: "active",
      runtime_host: "ts-bridge",
      restart_count: 1,
      discovered_tools: ["echo"],
      mcp_capability_snapshot: snapshot,
    };
    const updates: PluginRuntimeUpdate[] = [];
    const reporter: PluginRuntimeReporter = {
      async report(update) {
        updates.push(update);
      },
    };
    const streamedEvents: Array<{ type: string; data: unknown }> = [];
    const options: ToolPluginManagerOptions = {
      reporter,
      streamer: {
        send(event) {
          streamedEvents.push({ type: event.type, data: event.data });
        },
      },
    };
    const update: PluginRuntimeUpdate = {
      plugin_id: manifest.metadata.id,
      host: "ts-bridge",
      lifecycle_state: "active",
      restart_count: 1,
      runtime_metadata: {
        mcp: {
          transport: "stdio",
          tool_count: snapshot.tool_count,
          resource_count: snapshot.resource_count,
          prompt_count: snapshot.prompt_count,
          latest_interaction: snapshot.latest_interaction,
        },
      },
    };

    await options.reporter?.report(update);
    options.streamer?.send({
      task_id: "task-1",
      session_id: "session-1",
      timestamp_ms: 1,
      type: "tool.status_change",
      data: {
        plugin_id: manifest.metadata.id,
        lifecycle_state: update.lifecycle_state,
      },
    });

    expect(record.mcp_capability_snapshot?.tools[0]?.name).toBe("echo");
    expect(updates).toEqual([update]);
    expect(streamedEvents[0]?.type).toBe("tool.status_change");
  });
});
