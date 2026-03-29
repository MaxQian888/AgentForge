import { describe, expect, test } from "bun:test";
import { createApp } from "./server.js";
import { ToolPluginManager } from "./plugins/tool-plugin-manager.js";
import { MCPClientHub } from "./mcp/client-hub.js";
import type { MCPClientEntry } from "./mcp/types.js";
import type { PluginManifest } from "./plugins/types.js";

function createMockHub(): MCPClientHub {
  const hub = new MCPClientHub();
  const clients = (hub as unknown as { clients: Map<string, MCPClientEntry> }).clients;
  hub.connectServer = async (pluginId, _config) => {
    const fakeTool = {
      name: "search",
      description: "Search repos",
      inputSchema: { type: "object" as const, properties: {} },
    };
    clients.set(pluginId, {
      client: {
        listTools: async () => ({ tools: [fakeTool] }),
        listResources: async () => ({ resources: [] }),
        listPrompts: async () => ({ prompts: [] }),
        callTool: async () => ({
          content: [{ type: "text", text: "search complete" }],
          structuredContent: { ok: true },
        }),
        readResource: async () => ({ contents: [] }),
        getPrompt: async () => ({ description: "noop", messages: [] }),
        close: async () => {},
      } as unknown as MCPClientEntry["client"],
      transport: {
        start: async () => {},
        send: async () => {},
        close: async () => {},
      } as unknown as MCPClientEntry["transport"],
      config: _config,
      state: "active",
      connectedAt: Date.now(),
      tools: [fakeTool],
      resources: [],
      prompts: [],
    });
    return [fakeTool];
  };
  hub.disconnectServer = async (pluginId) => {
    clients.delete(pluginId);
  };
  return hub;
}

const manifest: PluginManifest = {
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
    command: process.execPath,
    args: ["-e", "setInterval(() => {}, 1000)"],
  },
  permissions: {},
  source: {
    type: "local",
    path: "./plugins/repo-search/manifest.yaml",
  },
};

describe("bridge plugin routes", () => {
  test("registers and lists tool plugins", async () => {
    const pluginManager = new ToolPluginManager({ mcpHub: createMockHub() });
    const app = createApp({ pluginManager });

    const registerResponse = await app.request("/bridge/plugins/register", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ manifest }),
    });
    expect(registerResponse.status).toBe(200);

    const listResponse = await app.request("/bridge/plugins");
    expect(listResponse.status).toBe(200);
    const body = await listResponse.json();
    expect(body.plugins).toHaveLength(1);

    await pluginManager.dispose();
  });

  test("supports enable, activate, health, tool call, and disable lifecycle routes", async () => {
    const pluginManager = new ToolPluginManager({ mcpHub: createMockHub() });
    const app = createApp({ pluginManager });

    const registerResponse = await app.request("/bridge/plugins/register", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ manifest }),
    });
    expect(registerResponse.status).toBe(200);

    const enableResponse = await app.request("/bridge/plugins/repo-search/enable", {
      method: "POST",
    });
    expect(enableResponse.status).toBe(200);
    expect(await enableResponse.json()).toMatchObject({
      lifecycle_state: "enabled",
    });

    const activateResponse = await app.request("/bridge/plugins/repo-search/activate", {
      method: "POST",
    });
    expect(activateResponse.status).toBe(200);
    expect(await activateResponse.json()).toMatchObject({
      lifecycle_state: "active",
      discovered_tools: ["search"],
    });

    const healthResponse = await app.request("/bridge/plugins/repo-search/health");
    expect(healthResponse.status).toBe(200);
    expect(await healthResponse.json()).toMatchObject({
      lifecycle_state: "active",
    });

    const refreshResponse = await app.request("/bridge/plugins/repo-search/mcp/refresh", {
      method: "POST",
    });
    expect(refreshResponse.status).toBe(200);
    expect(await refreshResponse.json()).toMatchObject({
      mcp_capability_snapshot: expect.objectContaining({
        tool_count: 1,
      }),
    });

    const toolCallResponse = await app.request("/bridge/plugins/repo-search/mcp/tools/call", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ tool_name: "search", arguments: { query: "bridge" } }),
    });
    expect(toolCallResponse.status).toBe(200);
    expect(await toolCallResponse.json()).toMatchObject({
      result: expect.objectContaining({
        structuredContent: { ok: true },
      }),
    });

    const disableResponse = await app.request("/bridge/plugins/repo-search/disable", {
      method: "POST",
    });
    expect(disableResponse.status).toBe(200);
    expect(await disableResponse.json()).toMatchObject({
      lifecycle_state: "disabled",
    });

    await pluginManager.dispose();
  });
});
