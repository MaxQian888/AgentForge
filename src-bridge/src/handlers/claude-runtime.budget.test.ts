import { describe, expect, test } from "bun:test";
import { buildMcpServersOption } from "./claude-runtime.js";
import type { PluginRecord } from "../plugins/types.js";

describe("buildMcpServersOption", () => {
  test("returns undefined for empty plugin list", () => {
    expect(buildMcpServersOption([])).toBeUndefined();
  });

  test("builds stdio server config from plugin record", () => {
    const plugins: PluginRecord[] = [
      {
        apiVersion: "v1",
        kind: "ToolPlugin",
        metadata: { id: "github-tool", name: "GitHub", version: "1.0.0" },
        spec: {
          runtime: "mcp",
          transport: "stdio",
          command: "node",
          args: ["server.js"],
          env: { TOKEN: "abc" },
        },
        permissions: {},
        source: { type: "local" },
        lifecycle_state: "active",
        runtime_host: "ts-bridge",
        restart_count: 0,
      },
    ];

    const result = buildMcpServersOption(plugins);
    expect(result).toEqual({
      "github-tool": {
        command: "node",
        args: ["server.js"],
        env: { TOKEN: "abc" },
      },
    });
  });

  test("builds http server config from plugin record", () => {
    const plugins: PluginRecord[] = [
      {
        apiVersion: "v1",
        kind: "ToolPlugin",
        metadata: { id: "web-search", name: "Web Search", version: "1.0.0" },
        spec: {
          runtime: "mcp",
          transport: "http",
          url: "http://localhost:3000/mcp",
        },
        permissions: {},
        source: { type: "local" },
        lifecycle_state: "active",
        runtime_host: "ts-bridge",
        restart_count: 0,
      },
    ];

    const result = buildMcpServersOption(plugins);
    expect(result).toEqual({
      "web-search": { type: "http", url: "http://localhost:3000/mcp" },
    });
  });

  test("skips plugins without command or url", () => {
    const plugins: PluginRecord[] = [
      {
        apiVersion: "v1",
        kind: "ToolPlugin",
        metadata: { id: "broken", name: "Broken", version: "1.0.0" },
        spec: { runtime: "mcp" },
        permissions: {},
        source: { type: "local" },
        lifecycle_state: "active",
        runtime_host: "ts-bridge",
        restart_count: 0,
      },
    ];

    expect(buildMcpServersOption(plugins)).toBeUndefined();
  });
});
