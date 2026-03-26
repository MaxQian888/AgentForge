import { afterEach, describe, expect, test } from "bun:test";
import { ToolPluginManager } from "./tool-plugin-manager.js";
import { MCPClientHub } from "../mcp/client-hub.js";
import type { MCPClientEntry } from "../mcp/types.js";
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
  const clients = (hub as unknown as { clients: Map<string, MCPClientEntry> }).clients;
  // Override connectServer to skip real MCP handshake
  hub.connectServer = async (pluginId, _config) => {
    // Simulate discovered tools
    const fakeTool = { name: "search", description: "Search repos", inputSchema: { type: "object" as const, properties: {} } };
    const fakeResource = { uri: "file://guide.md", name: "guide.md" };
    const fakePrompt = { name: "planner-template", description: "Plan work" };
    // Access internal state via type cast
    const entry = {
      client: {
        listTools: async () => ({ tools: [fakeTool] }),
        listResources: async () => ({ resources: [fakeResource] }),
        listPrompts: async () => ({ prompts: [fakePrompt] }),
        callTool: async () => ({
          content: [{ type: "text", text: "search complete" }],
          structuredContent: { ok: true },
        }),
        readResource: async () => ({
          contents: [{ uri: "file://guide.md", text: "guide body" }],
        }),
        getPrompt: async () => ({
          description: "Plan work",
          messages: [{ role: "user", content: { type: "text", text: "Plan the task" } }],
        }),
        close: async () => {},
      } as unknown as MCPClientEntry["client"],
      transport: {
        start: async () => {},
        send: async () => {},
        close: async () => {},
      } as unknown as MCPClientEntry["transport"],
      config: _config,
      state: "active" as const,
      connectedAt: Date.now(),
      tools: [fakeTool],
      resources: [fakeResource],
      prompts: [fakePrompt],
    };
    clients.set(pluginId, entry);
    return [fakeTool];
  };
  hub.disconnectServer = async (pluginId) => {
    clients.delete(pluginId);
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

  test("captures an MCP capability snapshot after activation and refresh", async () => {
    manager = new ToolPluginManager({ reporter, mcpHub: createMockHub() });
    await manager.register(validManifest);
    await manager.enable("repo-search");
    const activated = await manager.activate("repo-search");

    expect(activated.mcp_capability_snapshot?.transport).toBe("stdio");
    expect(activated.mcp_capability_snapshot?.tool_count).toBe(1);
    expect(activated.mcp_capability_snapshot?.resource_count).toBe(1);
    expect(activated.mcp_capability_snapshot?.prompt_count).toBe(1);

    const refreshed = await manager.refreshCapabilitySurface("repo-search");
    expect(refreshed.mcp_capability_snapshot?.last_discovery_at).toBeTruthy();
    expect(refreshed.mcp_capability_snapshot?.latest_interaction?.operation).toBe("refresh");
  });

  test("updates latest interaction summaries for tool, resource, and prompt operations", async () => {
    manager = new ToolPluginManager({ reporter, mcpHub: createMockHub() });
    await manager.register(validManifest);
    await manager.enable("repo-search");
    await manager.activate("repo-search");

    const toolResult = await manager.invokeTool("repo-search", "search", { query: "bridge" });
    expect(toolResult.structuredContent).toEqual({ ok: true });

    const resourceResult = await manager.readResource("repo-search", "file://guide.md");
    expect(resourceResult.contents[0]?.text).toBe("guide body");

    const promptResult = await manager.getPrompt("repo-search", "planner-template");
    expect(promptResult.description).toBe("Plan work");

    const record = manager.list()[0];
    expect(record?.mcp_capability_snapshot?.latest_interaction?.operation).toBe("get_prompt");
    expect(record?.mcp_capability_snapshot?.latest_interaction?.status).toBe("succeeded");
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
    const crashEvent = crashEvents.find(
      (event) =>
        typeof event === "object" &&
        event !== null &&
        "data" in event &&
        (event as { data?: { code?: string } }).data?.code === "MCP_SERVER_CRASHED"
    );
    expect(crashEvent).toBeTruthy();
    expect((crashEvent as { data: { code: string } }).data.code).toBe("MCP_SERVER_CRASHED");
    // Verify tool.status_change events were emitted
    const statusEvents = crashEvents.filter(
      (event) =>
        typeof event === "object" &&
        event !== null &&
        "type" in event &&
        (event as { type?: string }).type === "tool.status_change"
    );
    expect(statusEvents.length).toBe(3);
  });
});
