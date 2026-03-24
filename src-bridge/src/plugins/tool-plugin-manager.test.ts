import { afterEach, describe, expect, test } from "bun:test";
import { ToolPluginManager } from "./tool-plugin-manager.js";
import { MCPClientHub } from "../mcp/client-hub.js";
import type { PluginManifest, PluginRuntimeReporter } from "./types.js";

const reporterCalls: unknown[] = [];

const reporter: PluginRuntimeReporter = {
  async report(update) {
    reporterCalls.push(update);
  },
};

const validManifest: PluginManifest = {
  apiVersion: "agentforge/v1",
  kind: "ToolPlugin",
  metadata: {
    id: "repo-search",
    name: "Repo Search",
    version: "1.0.0",
  },
  spec: {
    runtime: "mcp",
    transport: "stdio",
    command: "node",
    args: ["--version"],
  },
  permissions: {},
  source: {
    type: "local",
    path: "./plugins/repo-search/manifest.yaml",
  },
};

/** Minimal mock MCPClientHub that doesn't spawn real processes. */
function createMockHub(): MCPClientHub {
  const hub = new MCPClientHub();
  // Override connectServer to skip real MCP handshake
  hub.connectServer = async (pluginId, _config) => {
    // Simulate discovered tools
    const fakeTool = { name: "search", description: "Search repos", inputSchema: { type: "object" as const, properties: {} } };
    // Access internal state via type cast
    const entry = {
      client: {} as any,
      transport: {} as any,
      config: _config,
      state: "active" as const,
      connectedAt: Date.now(),
      tools: [fakeTool],
    };
    (hub as any).clients.set(pluginId, entry);
    return [fakeTool];
  };
  hub.disconnectServer = async (pluginId) => {
    (hub as any).clients.delete(pluginId);
  };
  return hub;
}

let manager: ToolPluginManager | undefined;

afterEach(async () => {
  reporterCalls.length = 0;
  if (manager) {
    await manager.dispose();
    manager = undefined;
  }
});

describe("tool plugin manager", () => {
  test("registers a valid tool plugin manifest", async () => {
    manager = new ToolPluginManager({ reporter, mcpHub: createMockHub() });

    const record = await manager.register(validManifest);

    expect(record.metadata.id).toBe("repo-search");
    expect(record.lifecycle_state).toBe("installed");
  });

  test("rejects unsupported kind and runtime combinations", async () => {
    manager = new ToolPluginManager({ reporter, mcpHub: createMockHub() });

    await expect(
      manager.register({
        ...validManifest,
        kind: "IntegrationPlugin",
      } as PluginManifest),
    ).rejects.toThrow("kind");
  });

  test("refuses to activate a disabled plugin until re-enabled", async () => {
    manager = new ToolPluginManager({ reporter, mcpHub: createMockHub() });
    await manager.register(validManifest);
    await manager.disable("repo-search");

    await expect(manager.activate("repo-search")).rejects.toThrow("disabled");

    await manager.enable("repo-search");
    const status = await manager.activate("repo-search");
    expect(status.lifecycle_state).toBe("active");
  });

  test("reports activation and health transitions", async () => {
    manager = new ToolPluginManager({ reporter, mcpHub: createMockHub() });
    await manager.register(validManifest);

    await manager.enable("repo-search");
    await manager.activate("repo-search");
    const health = await manager.checkHealth("repo-search");

    expect(health.lifecycle_state).toBe("active");
    expect(reporterCalls.length).toBeGreaterThanOrEqual(2);
  });

  test("populates discovered_tools after activation", async () => {
    manager = new ToolPluginManager({ reporter, mcpHub: createMockHub() });
    await manager.register(validManifest);
    await manager.enable("repo-search");
    const activated = await manager.activate("repo-search");

    expect(activated.discovered_tools).toEqual(["search"]);
  });

  test("emits crash event via streamer on activation failure", async () => {
    const crashEvents: unknown[] = [];
    const streamer = {
      send(event: unknown) { crashEvents.push(event); },
    };

    const failHub = createMockHub();
    failHub.connectServer = async () => { throw new Error("MCP handshake failed"); };

    manager = new ToolPluginManager({ reporter, mcpHub: failHub, streamer });
    await manager.register(validManifest);
    await manager.enable("repo-search");

    await expect(manager.activate("repo-search")).rejects.toThrow("MCP handshake failed");
    // Events: enable→tool.status_change, activate→tool.status_change(activating),
    //         activate fail→tool.status_change(degraded), crash→error
    expect(crashEvents.length).toBe(4);
    const crashEvent = crashEvents.find((e: any) => e.data?.code === "MCP_SERVER_CRASHED");
    expect(crashEvent).toBeTruthy();
    expect((crashEvent as any).data.code).toBe("MCP_SERVER_CRASHED");
    // Verify tool.status_change events were emitted
    const statusEvents = crashEvents.filter((e: any) => e.type === "tool.status_change");
    expect(statusEvents.length).toBe(3);
  });
});
