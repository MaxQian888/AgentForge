import { afterEach, describe, expect, test } from "bun:test";
import { MCPClientHub } from "./client-hub.js";
import type { MCPClientEntry } from "./types.js";
import type { Client } from "@modelcontextprotocol/sdk/client/index.js";
import type { Transport } from "@modelcontextprotocol/sdk/shared/transport.js";
import type { Tool, Resource, Prompt } from "@modelcontextprotocol/sdk/types.js";

let hub: MCPClientHub | undefined;

function createMockTransport(): MCPClientEntry["transport"] {
  return {
    start: async () => {},
    send: async () => {},
    close: async () => {},
  } as unknown as Transport;
}

function createMockClient(overrides: Partial<MCPClientEntry["client"]> = {}): MCPClientEntry["client"] {
  return {
    close: async () => {},
    listTools: async () => ({ tools: [] as Tool[] }),
    listResources: async () => ({ resources: [] as Resource[] }),
    listPrompts: async () => ({ prompts: [] as Prompt[] }),
    callTool: async () => ({ content: [] }),
    readResource: async () => ({ contents: [] }),
    getPrompt: async () => ({ messages: [] }),
    ...overrides,
  } as unknown as Client;
}

function createTool(name: string, description: string): Tool {
  return {
    name,
    description,
    inputSchema: { type: "object", properties: {} },
  } as Tool;
}

function createResource(uri: string, name: string): Resource {
  return { uri, name } as Resource;
}

function createPrompt(name: string, description: string): Prompt {
  return { name, description } as Prompt;
}

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
    const clients = (hub as unknown as { clients: Map<string, MCPClientEntry> }).clients;
    clients.set("test-plugin", {
      client: createMockClient(),
      transport: createMockTransport(),
      config: { pluginId: "test-plugin", transport: "stdio" },
      state: "active",
      connectedAt: Date.now(),
      tools: [],
      resources: [],
      prompts: [],
    });

    hub.markDegraded("test-plugin", "process crashed");
    const status = hub.getServerStatus("test-plugin");
    expect(status?.status).toBe("degraded");
    expect(status?.last_error).toBe("process crashed");
  });

  test("discoverAllTools aggregates tools from active servers", () => {
    hub = new MCPClientHub();
    const clients = (hub as unknown as { clients: Map<string, MCPClientEntry> }).clients;

    clients.set("plugin-a", {
      client: createMockClient(),
      transport: createMockTransport(),
      config: { pluginId: "plugin-a", transport: "stdio" },
      state: "active",
      connectedAt: Date.now(),
      tools: [
        createTool("tool-1", "desc 1"),
        createTool("tool-2", "desc 2"),
      ],
      resources: [],
      prompts: [],
    });
    clients.set("plugin-b", {
      client: createMockClient(),
      transport: createMockTransport(),
      config: { pluginId: "plugin-b", transport: "http" },
      state: "degraded", // Not active — should be excluded
      connectedAt: Date.now(),
      tools: [createTool("tool-3", "desc 3")],
      resources: [],
      prompts: [],
    });

    const all = hub.discoverAllTools();
    expect(all).toHaveLength(2);
    expect(all[0].pluginId).toBe("plugin-a");
    expect(all[0].tool.name).toBe("tool-1");
    expect(all[1].tool.name).toBe("tool-2");
  });

  test("getAllServerStatuses returns statuses for all connected servers", () => {
    hub = new MCPClientHub();
    const clients = (hub as unknown as { clients: Map<string, MCPClientEntry> }).clients;

    clients.set("s1", {
      client: createMockClient(),
      transport: createMockTransport(),
      config: { pluginId: "s1", transport: "stdio" },
      state: "active",
      connectedAt: Date.now() - 5000,
      tools: [createTool("t1", "desc")],
      resources: [],
      prompts: [],
      pid: 1234,
    });
    clients.set("s2", {
      client: createMockClient(),
      transport: createMockTransport(),
      config: { pluginId: "s2", transport: "http" },
      state: "connecting",
      connectedAt: Date.now(),
      tools: [],
      resources: [],
      prompts: [],
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
    const clients = (hub as unknown as { clients: Map<string, MCPClientEntry> }).clients;

    clients.set("plugin-a", {
      client: createMockClient({
        listTools: async () => ({
          tools: [createTool("tool-1", "desc 1")],
        }),
        listResources: async () => ({
          resources: [createResource("file://guide.md", "guide.md")],
        }),
        listPrompts: async () => ({
          prompts: [createPrompt("planner-template", "Plan work")],
        }),
      }),
      transport: createMockTransport(),
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
    const clients = (hub as unknown as { clients: Map<string, MCPClientEntry> }).clients;
    let closeCalled = false;

    clients.set("s1", {
      client: createMockClient({ close: async () => { closeCalled = true; } }),
      transport: createMockTransport(),
      config: { pluginId: "s1", transport: "stdio" },
      state: "active",
      connectedAt: Date.now(),
      tools: [],
      resources: [],
      prompts: [],
    });

    await hub.dispose();
    expect(closeCalled).toBe(true);
    expect(hub.connectedIds()).toEqual([]);
  });
});
