import { describe, expect, test } from "bun:test";
import type {
  MCPCapabilitySurface,
  MCPClientHubOptions,
  MCPPromptResult,
  MCPResourceResult,
  MCPServerConfig,
  MCPServerStatus,
  MCPToolCallResult,
  ToolCallLog,
} from "./types.js";

describe("mcp contract types", () => {
  test("accepts config, status, and discovery payloads for a plugin-managed MCP server", () => {
    const config: MCPServerConfig = {
      pluginId: "tool.echo",
      transport: "stdio",
      command: "node",
      args: ["dist/index.js"],
      env: {
        NODE_ENV: "test",
      },
    };
    const status: MCPServerStatus = {
      id: "tool.echo",
      status: "active",
      pid: 4120,
      uptime_ms: 30_000,
      tool_count: 2,
      resource_count: 1,
      prompt_count: 1,
    };
    const toolResult: MCPToolCallResult = {
      content: [{ type: "text", text: "ok" }],
      structuredContent: {
        ok: true,
      },
    };
    const resourceResult: MCPResourceResult = {
      contents: [
        {
          uri: "memory://tool.echo/report",
          mimeType: "text/plain",
          text: "latest report",
        },
      ],
    };
    const promptResult: MCPPromptResult = {
      description: "Review prompt",
      messages: [
        {
          role: "user",
          content: {
            type: "text",
            text: "Review this bridge change",
          },
        },
      ],
    };
    const surface: MCPCapabilitySurface = {
      plugin_id: config.pluginId,
      transport: config.transport,
      tools: [],
      resources: [],
      prompts: [],
      tool_count: status.tool_count,
      resource_count: status.resource_count ?? 0,
      prompt_count: status.prompt_count ?? 0,
      refreshed_at: "2026-03-30T00:00:00Z",
    };

    expect(config.transport).toBe("stdio");
    expect(status.status).toBe("active");
    expect(toolResult.content[0]?.text).toBe("ok");
    expect(resourceResult.contents[0]?.uri).toContain("tool.echo");
    expect(promptResult.messages[0]?.role).toBe("user");
    expect(surface.tool_count).toBe(2);
  });

  test("accepts logging and crash callbacks through MCP client hub options", () => {
    const logs: ToolCallLog[] = [];
    const crashes: Array<{ pluginId: string; exitCode: number | null }> = [];
    const options: MCPClientHubOptions = {
      onToolCallLog(log) {
        logs.push(log);
      },
      onProcessCrash(pluginId, exitCode) {
        crashes.push({ pluginId, exitCode });
      },
    };
    const log: ToolCallLog = {
      plugin_id: "tool.echo",
      tool_name: "echo",
      duration_ms: 42,
      is_error: false,
    };

    options.onToolCallLog?.(log);
    options.onProcessCrash?.("tool.echo", 137);

    expect(logs).toEqual([log]);
    expect(crashes).toEqual([{ pluginId: "tool.echo", exitCode: 137 }]);
  });
});
