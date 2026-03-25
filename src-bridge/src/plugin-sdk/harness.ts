import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import { listPluginToolDefinitions, type SDKPluginDefinition } from "./server.js";

export interface LocalPluginHarness {
  manifest: SDKPluginDefinition["manifest"];
  listTools(): Array<{ name: string; title?: string; description?: string }>;
  callTool(name: string, args: Record<string, unknown>): Promise<CallToolResult>;
}

export function createLocalPluginHarness(definition: SDKPluginDefinition): LocalPluginHarness {
  const tools = listPluginToolDefinitions(definition);

  return {
    manifest: definition.manifest,
    listTools: () =>
      tools.map((tool) => ({
        name: tool.name,
        title: tool.title,
        description: tool.description,
      })),
    async callTool(name, args) {
      const tool = tools.find((item) => item.name === name);
      if (!tool) {
        throw new Error(`Unknown tool ${name}`);
      }

      const parsedArgs = tool.inputSchema ? tool.inputSchema.parse(args) : args;
      return await tool.execute(parsedArgs as Record<string, unknown>);
    },
  };
}
