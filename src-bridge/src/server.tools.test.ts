import { describe, expect, test } from "bun:test";
import { createApp } from "./server.js";
import { ToolPluginManager } from "./plugins/tool-plugin-manager.js";
import { MCPClientHub } from "./mcp/client-hub.js";
import type { PluginManifest } from "./plugins/types.js";

function createMockHub(): MCPClientHub {
  const hub = new MCPClientHub();
  hub.connectServer = async (pluginId, _config) => {
    const fakeTool = {
      name: "search",
      description: "Search repos",
      inputSchema: { type: "object" as const, properties: {} },
    };
    (hub as any).clients.set(pluginId, {
      client: { close: async () => {} },
      transport: {},
      config: _config,
      state: "active",
      connectedAt: Date.now(),
      tools: [fakeTool],
    });
    return [fakeTool];
  };
  hub.disconnectServer = async (pluginId) => {
    (hub as any).clients.delete(pluginId);
  };
  return hub;
}

const manifest: PluginManifest = {
  apiVersion: "agentforge/v1",
  kind: "ToolPlugin",
  metadata: {
    id: "web-search",
    name: "Web Search",
    version: "1.0.0",
  },
  spec: {
    runtime: "mcp",
    transport: "stdio",
    command: "node",
    args: ["--version"],
    env: { API_KEY: "test-key" },
  },
  permissions: {},
  source: { type: "local" },
};

describe("bridge tool routes", () => {
  test("GET /bridge/tools returns empty list initially", async () => {
    const mcpHub = createMockHub();
    const pluginManager = new ToolPluginManager({ mcpHub });
    const app = createApp({ pluginManager });

    const res = await app.request("/bridge/tools");
    expect(res.status).toBe(200);
    const body = await res.json();
    expect(body.tools).toEqual([]);

    await pluginManager.dispose();
  });

  test("POST /bridge/tools/install registers, enables, and activates plugin", async () => {
    const mcpHub = createMockHub();
    const pluginManager = new ToolPluginManager({ mcpHub });
    const app = createApp({ pluginManager });

    const res = await app.request("/bridge/tools/install", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ manifest }),
    });
    expect(res.status).toBe(200);
    const body = await res.json();
    expect(body.lifecycle_state).toBe("active");
    expect(body.discovered_tools).toEqual(["search"]);

    // Now GET /bridge/tools should return the discovered tool
    const toolsRes = await app.request("/bridge/tools");
    const toolsBody = await toolsRes.json();
    expect(toolsBody.tools).toHaveLength(1);
    expect(toolsBody.tools[0].name).toBe("search");
    expect(toolsBody.tools[0].plugin_id).toBe("web-search");

    await pluginManager.dispose();
  });

  test("POST /bridge/tools/uninstall disables plugin", async () => {
    const mcpHub = createMockHub();
    const pluginManager = new ToolPluginManager({ mcpHub });
    const app = createApp({ pluginManager });

    // Install first
    await app.request("/bridge/tools/install", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ manifest }),
    });

    // Uninstall
    const res = await app.request("/bridge/tools/uninstall", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ plugin_id: "web-search" }),
    });
    expect(res.status).toBe(200);
    const body = await res.json();
    expect(body.lifecycle_state).toBe("disabled");

    // Tools list should be empty
    const toolsRes = await app.request("/bridge/tools");
    const toolsBody = await toolsRes.json();
    expect(toolsBody.tools).toEqual([]);

    await pluginManager.dispose();
  });

  test("POST /bridge/tools/uninstall requires plugin_id", async () => {
    const mcpHub = createMockHub();
    const pluginManager = new ToolPluginManager({ mcpHub });
    const app = createApp({ pluginManager });

    const res = await app.request("/bridge/tools/uninstall", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({}),
    });
    expect(res.status).toBe(400);

    await pluginManager.dispose();
  });

  test("POST /bridge/tools/:id/restart restarts a plugin", async () => {
    const mcpHub = createMockHub();
    const pluginManager = new ToolPluginManager({ mcpHub });
    const app = createApp({ pluginManager });

    // Install first
    await app.request("/bridge/tools/install", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ manifest }),
    });

    const res = await app.request("/bridge/tools/web-search/restart", {
      method: "POST",
    });
    expect(res.status).toBe(200);
    const body = await res.json();
    expect(body.lifecycle_state).toBe("active");
    expect(body.restart_count).toBe(1);

    await pluginManager.dispose();
  });
});
