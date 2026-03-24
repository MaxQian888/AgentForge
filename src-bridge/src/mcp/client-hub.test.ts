import { afterEach, describe, expect, test } from "bun:test";
import { MCPClientHub } from "./client-hub.js";

let hub: MCPClientHub | undefined;

afterEach(async () => {
  if (hub) {
    await hub.dispose();
    hub = undefined;
  }
});

describe("MCPClientHub", () => {
  test("starts with no connected servers", () => {
    hub = new MCPClientHub();
    expect(hub.connectedIds()).toEqual([]);
    expect(hub.getAllServerStatuses()).toEqual([]);
    expect(hub.discoverAllTools()).toEqual([]);
  });

  test("getServerStatus returns null for unknown plugin", () => {
    hub = new MCPClientHub();
    expect(hub.getServerStatus("nonexistent")).toBeNull();
  });

  test("isActive returns false for unconnected server", () => {
    hub = new MCPClientHub();
    expect(hub.isActive("unknown")).toBe(false);
  });

  test("listTools returns empty array for unconnected server", () => {
    hub = new MCPClientHub();
    expect(hub.listTools("unknown")).toEqual([]);
  });

  test("listPrompts returns empty array for unconnected server", () => {
    hub = new MCPClientHub();
    expect(hub.listPrompts("unknown")).toEqual([]);
  });

  test("callTool throws for unconnected server", async () => {
    hub = new MCPClientHub();
    await expect(hub.callTool("unknown", "tool", {})).rejects.toThrow("not connected");
  });

  test("getPrompt throws for unconnected server", async () => {
    hub = new MCPClientHub();
    await expect(hub.getPrompt("unknown", "planner-template")).rejects.toThrow("not connected");
  });

  test("disconnectServer is a no-op for unknown plugin", async () => {
    hub = new MCPClientHub();
    await hub.disconnectServer("nonexistent");
    // Should not throw
  });

  test("markDegraded updates the entry state", async () => {
    hub = new MCPClientHub();
    // Manually inject an entry for testing
    const clients = (hub as any).clients;
    clients.set("test-plugin", {
      client: {},
      transport: {},
      config: { pluginId: "test-plugin", transport: "stdio" },
      state: "active",
      connectedAt: Date.now(),
      tools: [],
    });

    hub.markDegraded("test-plugin", "process crashed");
    const status = hub.getServerStatus("test-plugin");
    expect(status?.status).toBe("degraded");
    expect(status?.last_error).toBe("process crashed");
  });

  test("discoverAllTools aggregates tools from active servers", () => {
    hub = new MCPClientHub();
    const clients = (hub as any).clients;

    clients.set("plugin-a", {
      client: {},
      transport: {},
      config: { pluginId: "plugin-a", transport: "stdio" },
      state: "active",
      connectedAt: Date.now(),
      tools: [
        { name: "tool-1", description: "desc 1", inputSchema: {} },
        { name: "tool-2", description: "desc 2", inputSchema: {} },
      ],
    });
    clients.set("plugin-b", {
      client: {},
      transport: {},
      config: { pluginId: "plugin-b", transport: "http" },
      state: "degraded", // Not active — should be excluded
      connectedAt: Date.now(),
      tools: [{ name: "tool-3", description: "desc 3", inputSchema: {} }],
    });

    const all = hub.discoverAllTools();
    expect(all).toHaveLength(2);
    expect(all[0].pluginId).toBe("plugin-a");
    expect(all[0].tool.name).toBe("tool-1");
    expect(all[1].tool.name).toBe("tool-2");
  });

  test("getAllServerStatuses returns statuses for all connected servers", () => {
    hub = new MCPClientHub();
    const clients = (hub as any).clients;

    clients.set("s1", {
      client: {},
      transport: {},
      config: { pluginId: "s1", transport: "stdio" },
      state: "active",
      connectedAt: Date.now() - 5000,
      tools: [{ name: "t1", inputSchema: {} }],
      pid: 1234,
    });
    clients.set("s2", {
      client: {},
      transport: {},
      config: { pluginId: "s2", transport: "http" },
      state: "connecting",
      connectedAt: Date.now(),
      tools: [],
    });

    const statuses = hub.getAllServerStatuses();
    expect(statuses).toHaveLength(2);
    expect(statuses[0].id).toBe("s1");
    expect(statuses[0].pid).toBe(1234);
    expect(statuses[0].tool_count).toBe(1);
    expect(statuses[1].id).toBe("s2");
    expect(statuses[1].tool_count).toBe(0);
  });

  test("refreshCapabilitySurface returns tools, resources, and prompts for an active plugin", async () => {
    hub = new MCPClientHub();
    const clients = (hub as any).clients;

    clients.set("plugin-a", {
      client: {
        listTools: async () => ({
          tools: [{ name: "tool-1", description: "desc 1", inputSchema: {} }],
        }),
        listResources: async () => ({
          resources: [{ uri: "file://guide.md", name: "guide.md" }],
        }),
        listPrompts: async () => ({
          prompts: [{ name: "planner-template", description: "Plan work" }],
        }),
      },
      transport: {},
      config: { pluginId: "plugin-a", transport: "stdio" },
      state: "active",
      connectedAt: Date.now(),
      tools: [],
      resources: [],
      prompts: [],
    });

    const surface = await hub.refreshCapabilitySurface("plugin-a");

    expect(surface.tools).toHaveLength(1);
    expect(surface.resources).toHaveLength(1);
    expect(surface.prompts).toHaveLength(1);
    expect(surface.prompt_count).toBe(1);
  });

  test("dispose clears all connections", async () => {
    hub = new MCPClientHub();
    const clients = (hub as any).clients;
    let closeCalled = false;

    clients.set("s1", {
      client: { close: async () => { closeCalled = true; } },
      transport: {},
      config: { pluginId: "s1", transport: "stdio" },
      state: "active",
      connectedAt: Date.now(),
      tools: [],
    });

    await hub.dispose();
    expect(closeCalled).toBe(true);
    expect(hub.connectedIds()).toEqual([]);
  });
});
