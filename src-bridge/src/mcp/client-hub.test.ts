import { afterEach, describe, expect, mock, test } from "bun:test";
import { MCPClientHub } from "./client-hub.js";
import type { MCPClientEntry } from "./types.js";
import type { Client } from "@modelcontextprotocol/sdk/client/index.js";
import type { Transport } from "@modelcontextprotocol/sdk/shared/transport.js";
import type { Tool, Resource, Prompt } from "@modelcontextprotocol/sdk/types.js";

let hub: MCPClientHub | undefined;
let dynamicImportCounter = 0;

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

async function importClientHubModule(tag: string) {
  dynamicImportCounter += 1;
  return import(`./client-hub.ts?${tag}-${dynamicImportCounter}`);
}

async function restoreClientHubModuleMocks() {
  dynamicImportCounter += 1;
  const actualStdioTransport = await import(`./stdio-transport.ts?restore-stdio-${dynamicImportCounter}`);
  dynamicImportCounter += 1;
  const actualHttpTransport = await import(`./http-transport.ts?restore-http-${dynamicImportCounter}`);
  mock.module("./stdio-transport.js", () => actualStdioTransport);
  mock.module("./http-transport.js", () => actualHttpTransport);
}

afterEach(async () => {
  if (hub) {
    await hub.dispose();
    hub = undefined;
  }
  mock.restore();
  mock.clearAllMocks();
  await restoreClientHubModuleMocks();
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

  test("callTool enforces the allowed tool list before dispatching to the MCP client", async () => {
    hub = new MCPClientHub();

    await expect(
      hub.callTool("unknown", "forbidden-tool", {}, { allowedTools: new Set(["allowed-tool"]) }),
    ).rejects.toThrow('Tool "forbidden-tool" is not in the allowed tools list');
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

  test("connectServer uses stdio transport, discovers tools, and wires process crash tracking", async () => {
    const constructorArgs: Array<{ name: string; version: string }> = [];
    const connectCalls: unknown[] = [];
    const stdioConfigs: unknown[] = [];
    const crashEvents: Array<{ pluginId: string; code: number | null }> = [];
    let exitListener: ((code: number | null) => void) | undefined;

    mock.module("@modelcontextprotocol/sdk/client/index.js", () => ({
      Client: class FakeClient {
        constructor(meta: { name: string; version: string }) {
          constructorArgs.push(meta);
        }
        async connect(transport: unknown) {
          connectCalls.push(transport);
        }
        async listTools() {
          return { tools: [createTool("tool-stdio", "Stdio tool")] };
        }
        async listResources() {
          return { resources: [createResource("file://stdio-guide.md", "stdio-guide.md")] };
        }
        async listPrompts() {
          return { prompts: [createPrompt("stdio-plan", "Plan stdio work")] };
        }
        async close() {}
      },
    }));
    mock.module("./stdio-transport.js", () => ({
      createStdioTransport(config: unknown) {
        stdioConfigs.push(config);
        return {
          _process: {
            pid: 4321,
            on(event: "exit", listener: (code: number | null) => void) {
              if (event === "exit") {
                exitListener = listener;
              }
            },
          },
        };
      },
    }));
    mock.module("./http-transport.js", () => ({
      createHttpTransport() {
        throw new Error("unexpected http transport");
      },
    }));

    const { MCPClientHub: DynamicHub } = await importClientHubModule("connect-stdio");
    const dynamicHub = new DynamicHub({
      onProcessCrash(pluginId: string, code: number | null) {
        crashEvents.push({ pluginId, code });
      },
    });

    try {
      const tools = await dynamicHub.connectServer("plugin-stdio", {
        pluginId: "plugin-stdio",
        transport: "stdio",
        command: "node",
        args: ["dist/index.js"],
      });

      expect(constructorArgs).toEqual([{ name: "agentforge-bridge", version: "0.1.0" }]);
      expect(stdioConfigs).toEqual([
        {
          pluginId: "plugin-stdio",
          transport: "stdio",
          command: "node",
          args: ["dist/index.js"],
        },
      ]);
      expect(connectCalls).toHaveLength(1);
      expect(tools).toEqual([createTool("tool-stdio", "Stdio tool")]);
      expect(dynamicHub.getServerStatus("plugin-stdio")).toEqual(
        expect.objectContaining({
          id: "plugin-stdio",
          status: "active",
          pid: 4321,
          tool_count: 1,
          resource_count: 1,
          prompt_count: 1,
        }),
      );

      exitListener?.(137);

      expect(dynamicHub.getServerStatus("plugin-stdio")).toEqual(
        expect.objectContaining({
          status: "degraded",
          last_error: "Process exited unexpectedly with code 137",
        }),
      );
      expect(crashEvents).toEqual([{ pluginId: "plugin-stdio", code: 137 }]);
    } finally {
      await dynamicHub.dispose();
    }
  });

  test("connectServer replaces an existing connection and uses the http transport path", async () => {
    let closeCalls = 0;
    const httpConfigs: unknown[] = [];

    mock.module("@modelcontextprotocol/sdk/client/index.js", () => ({
      Client: class FakeClient {
        async connect() {}
        async listTools() {
          return { tools: [createTool("tool-http", "Http tool")] };
        }
        async listResources() {
          return { resources: [] };
        }
        async listPrompts() {
          return { prompts: [] };
        }
        async close() {}
      },
    }));
    mock.module("./stdio-transport.js", () => ({
      createStdioTransport() {
        throw new Error("unexpected stdio transport");
      },
    }));
    mock.module("./http-transport.js", () => ({
      createHttpTransport(config: unknown) {
        httpConfigs.push(config);
        return { kind: "http-transport" };
      },
    }));

    const { MCPClientHub: DynamicHub } = await importClientHubModule("connect-http");
    const dynamicHub = new DynamicHub();
    const clients = (dynamicHub as unknown as { clients: Map<string, MCPClientEntry> }).clients;
    clients.set("plugin-http", {
      client: createMockClient({
        close: async () => {
          closeCalls += 1;
        },
      }),
      transport: createMockTransport(),
      config: { pluginId: "plugin-http", transport: "stdio" },
      state: "active",
      connectedAt: Date.now(),
      tools: [],
      resources: [],
      prompts: [],
    });

    try {
      const tools = await dynamicHub.connectServer("plugin-http", {
        pluginId: "plugin-http",
        transport: "http",
        url: "http://127.0.0.1:4000/mcp",
      });

      expect(closeCalls).toBe(1);
      expect(httpConfigs).toEqual([
        {
          pluginId: "plugin-http",
          transport: "http",
          url: "http://127.0.0.1:4000/mcp",
        },
      ]);
      expect(tools).toEqual([createTool("tool-http", "Http tool")]);
      expect(dynamicHub.getServerStatus("plugin-http")).toEqual(
        expect.objectContaining({
          status: "active",
          pid: undefined,
          tool_count: 1,
        }),
      );
    } finally {
      await dynamicHub.dispose();
    }
  });

  test("connectServer degrades the entry and rethrows when handshake or discovery fails", async () => {
    mock.module("@modelcontextprotocol/sdk/client/index.js", () => ({
      Client: class FakeClient {
        async connect() {}
        async listTools() {
          throw new Error("tool discovery failed");
        }
        async listResources() {
          return { resources: [] };
        }
        async listPrompts() {
          return { prompts: [] };
        }
        async close() {}
      },
    }));
    mock.module("./stdio-transport.js", () => ({
      createStdioTransport(config: unknown) {
        return { config };
      },
    }));
    mock.module("./http-transport.js", () => ({
      createHttpTransport(config: unknown) {
        return { config };
      },
    }));

    const { MCPClientHub: DynamicHub } = await importClientHubModule("connect-failure");
    const dynamicHub = new DynamicHub();

    try {
      await expect(
        dynamicHub.connectServer("plugin-failure", {
          pluginId: "plugin-failure",
          transport: "http",
          url: "http://127.0.0.1:4100/mcp",
        }),
      ).rejects.toThrow("tool discovery failed");
      expect(dynamicHub.getServerStatus("plugin-failure")).toEqual(
        expect.objectContaining({
          status: "degraded",
          last_error: "tool discovery failed",
          tool_count: 0,
        }),
      );
    } finally {
      await dynamicHub.dispose();
    }
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

  test("refreshes tools, resources, prompts, and prompt/resource reads through an active server", async () => {
    hub = new MCPClientHub();
    const clients = (hub as unknown as { clients: Map<string, MCPClientEntry> }).clients;

    clients.set("plugin-b", {
      client: createMockClient({
        listTools: async () => ({
          tools: [createTool("tool-refresh", "Refresh tool")],
        }),
        listResources: async () => ({
          resources: [createResource("memory://guide", "guide")],
        }),
        listPrompts: async () => ({
          prompts: [createPrompt("planner-template", "Plan work")],
        }),
        readResource: async () => ({
          contents: [{ uri: "memory://guide", mimeType: "text/plain", text: "guide body" }],
        }),
        getPrompt: async () => ({
          description: "Planner prompt",
          messages: [
            {
              role: "user",
              content: { type: "text", text: "Plan the bridge rollout" },
            },
          ],
        }),
      }),
      transport: createMockTransport(),
      config: { pluginId: "plugin-b", transport: "http" },
      state: "active",
      connectedAt: Date.now(),
      tools: [],
      resources: [],
      prompts: [],
    });

    await expect(hub.refreshTools("plugin-b")).resolves.toEqual([createTool("tool-refresh", "Refresh tool")]);
    await expect(hub.listResources("plugin-b")).resolves.toEqual([
      createResource("memory://guide", "guide"),
    ]);
    await expect(hub.listPromptsFromServer("plugin-b")).resolves.toEqual([
      createPrompt("planner-template", "Plan work"),
    ]);
    await expect(hub.readResource("plugin-b", "memory://guide")).resolves.toEqual({
      contents: [{ uri: "memory://guide", mimeType: "text/plain", text: "guide body" }],
    });
    await expect(hub.getPrompt("plugin-b", "planner-template", { team: "bridge" })).resolves.toEqual({
      description: "Planner prompt",
      messages: [
        {
          role: "user",
          content: { type: "text", text: "Plan the bridge rollout" },
        },
      ],
    });
    expect(hub.listTools("plugin-b")).toEqual([createTool("tool-refresh", "Refresh tool")]);
    expect(hub.listPrompts("plugin-b")).toEqual([createPrompt("planner-template", "Plan work")]);
  });

  test("callTool returns MCP results and emits call logs for success, MCP errors, and thrown failures", async () => {
    const logs: Array<{ plugin_id: string; tool_name: string; duration_ms: number; is_error: boolean }> = [];
    hub = new MCPClientHub({
      onToolCallLog(log) {
        logs.push(log);
      },
    });
    const clients = (hub as unknown as { clients: Map<string, MCPClientEntry> }).clients;
    let callIndex = 0;

    clients.set("plugin-c", {
      client: createMockClient({
        callTool: async () => {
          callIndex += 1;
          if (callIndex === 1) {
            return {
              content: [{ type: "text", text: "ok" }],
              structuredContent: { ok: true },
            };
          }
          if (callIndex === 2) {
            return {
              content: [{ type: "text", text: "failed" }],
              isError: true,
            };
          }
          throw new Error("tool crashed");
        },
      }),
      transport: createMockTransport(),
      config: { pluginId: "plugin-c", transport: "stdio" },
      state: "active",
      connectedAt: Date.now(),
      tools: [],
      resources: [],
      prompts: [],
    });

    await expect(hub.callTool("plugin-c", "echo", { text: "hello" })).resolves.toEqual({
      content: [{ type: "text", text: "ok" }],
      isError: false,
      structuredContent: { ok: true },
    });
    await expect(hub.callTool("plugin-c", "echo", { text: "warn" })).resolves.toEqual({
      content: [{ type: "text", text: "failed" }],
      isError: true,
      structuredContent: undefined,
    });
    await expect(hub.callTool("plugin-c", "echo", { text: "boom" })).rejects.toThrow("tool crashed");
    expect(logs).toHaveLength(3);
    expect(logs.map((entry) => entry.is_error)).toEqual([false, true, true]);
    expect(logs.every((entry) => entry.plugin_id === "plugin-c" && entry.tool_name === "echo")).toBe(true);
  });

  test("operations that require an active server reject degraded entries", async () => {
    hub = new MCPClientHub();
    const clients = (hub as unknown as { clients: Map<string, MCPClientEntry> }).clients;

    clients.set("plugin-degraded", {
      client: createMockClient(),
      transport: createMockTransport(),
      config: { pluginId: "plugin-degraded", transport: "stdio" },
      state: "degraded",
      connectedAt: Date.now(),
      tools: [],
      resources: [],
      prompts: [],
    });

    await expect(hub.refreshTools("plugin-degraded")).rejects.toThrow(
      "MCP server plugin-degraded is not active (state: degraded)",
    );
  });

  test("disconnectServer swallows close failures and still removes the client entry", async () => {
    hub = new MCPClientHub();
    const clients = (hub as unknown as { clients: Map<string, MCPClientEntry> }).clients;

    clients.set("plugin-close-failure", {
      client: createMockClient({
        close: async () => {
          throw new Error("close failed");
        },
      }),
      transport: createMockTransport(),
      config: { pluginId: "plugin-close-failure", transport: "stdio" },
      state: "active",
      connectedAt: Date.now(),
      tools: [],
      resources: [],
      prompts: [],
    });

    await hub.disconnectServer("plugin-close-failure");
    expect(hub.connectedIds()).toEqual([]);
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
