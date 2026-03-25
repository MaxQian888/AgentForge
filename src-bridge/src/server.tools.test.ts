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
    const fakeResource = { uri: "file://guide.md", name: "guide.md" };
    const fakePrompt = { name: "planner-template", description: "Plan work" };
    (hub as any).clients.set(pluginId, {
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
      },
      transport: {},
      config: _config,
      state: "active",
      connectedAt: Date.now(),
      tools: [fakeTool],
      resources: [fakeResource],
      prompts: [fakePrompt],
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

  test("POST /bridge/plugins/:id/mcp/refresh returns refreshed capability metadata", async () => {
    const mcpHub = createMockHub();
    const pluginManager = new ToolPluginManager({ mcpHub });
    const app = createApp({ pluginManager });

    await app.request("/bridge/tools/install", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ manifest }),
    });

    const res = await app.request("/bridge/plugins/web-search/mcp/refresh", {
      method: "POST",
    });
    expect(res.status).toBe(200);
    const body = await res.json();
    expect(body.mcp_capability_snapshot.tool_count).toBe(1);
    expect(body.mcp_capability_snapshot.resource_count).toBe(1);
    expect(body.mcp_capability_snapshot.prompt_count).toBe(1);

    await pluginManager.dispose();
  });

  test("POST /bridge/plugins/:id/mcp/tools/call proxies tool invocation", async () => {
    const mcpHub = createMockHub();
    const pluginManager = new ToolPluginManager({ mcpHub });
    const app = createApp({ pluginManager });

    await app.request("/bridge/tools/install", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ manifest }),
    });

    const res = await app.request("/bridge/plugins/web-search/mcp/tools/call", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ tool_name: "search", arguments: { query: "bridge" } }),
    });
    expect(res.status).toBe(200);
    const body = await res.json();
    expect(body.result.structuredContent).toEqual({ ok: true });

    await pluginManager.dispose();
  });

  test("POST /bridge/plugins/:id/mcp/resources/read proxies resource access", async () => {
    const mcpHub = createMockHub();
    const pluginManager = new ToolPluginManager({ mcpHub });
    const app = createApp({ pluginManager });

    await app.request("/bridge/tools/install", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ manifest }),
    });

    const res = await app.request("/bridge/plugins/web-search/mcp/resources/read", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ uri: "file://guide.md" }),
    });
    expect(res.status).toBe(200);
    const body = await res.json();
    expect(body.result.contents[0].text).toBe("guide body");

    await pluginManager.dispose();
  });

  test("POST /bridge/plugins/:id/mcp/prompts/get proxies prompt retrieval", async () => {
    const mcpHub = createMockHub();
    const pluginManager = new ToolPluginManager({ mcpHub });
    const app = createApp({ pluginManager });

    await app.request("/bridge/tools/install", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ manifest }),
    });

    const res = await app.request("/bridge/plugins/web-search/mcp/prompts/get", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ name: "planner-template" }),
    });
    expect(res.status).toBe(200);
    const body = await res.json();
    expect(body.result.description).toBe("Plan work");

    await pluginManager.dispose();
  });
});
