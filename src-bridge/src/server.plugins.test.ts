import { describe, expect, test } from "bun:test";
import { createApp } from "./server.js";
import { ToolPluginManager } from "./plugins/tool-plugin-manager.js";
import type { PluginManifest } from "./plugins/types.js";

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
    const pluginManager = new ToolPluginManager();
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
});
