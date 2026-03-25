import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import { z, type ZodTypeAny } from "zod";
import type { PluginManifest } from "../plugins/types.js";
import { DEFAULT_REVIEW_ENTRYPOINT } from "./manifest.js";

type ToolHandlerArgs = Record<string, unknown>;
type ToolExecuteResult = CallToolResult | Promise<CallToolResult>;
type ToolSchema = ZodTypeAny | undefined;

export interface SDKToolDefinition<InputSchema extends ToolSchema = ToolSchema> {
  name: string;
  title?: string;
  description?: string;
  inputSchema?: InputSchema;
  execute: (args: any) => ToolExecuteResult;
}

export interface ToolPluginDefinition {
  manifest: PluginManifest;
  tools: readonly SDKToolDefinition[];
}

export interface ReviewPluginDefinition<InputSchema extends ToolSchema = ToolSchema> {
  manifest: PluginManifest;
  inputSchema?: InputSchema;
  executeReview: (args: any) => ToolExecuteResult;
}

export type SDKPluginDefinition = ToolPluginDefinition | ReviewPluginDefinition;

export function defineToolPlugin<
  Tools extends readonly SDKToolDefinition[],
>(definition: { manifest: PluginManifest; tools: Tools }): { manifest: PluginManifest; tools: Tools } {
  return definition;
}

export function defineReviewPlugin<InputSchema extends ToolSchema>(
  definition: ReviewPluginDefinition<InputSchema>,
): ReviewPluginDefinition<InputSchema> {
  return definition;
}

export function createPluginMcpServer(definition: SDKPluginDefinition): McpServer {
  const server = new McpServer({
    name: definition.manifest.metadata.name,
    version: definition.manifest.metadata.version,
  });

  for (const tool of listPluginToolDefinitions(definition)) {
    server.registerTool(
      tool.name,
      {
        title: tool.title,
        description: tool.description,
        inputSchema: tool.inputSchema,
      },
      async (args) => tool.execute(args as ToolHandlerArgs),
    );
  }

  return server;
}

export async function connectPluginStdioServer(server: McpServer): Promise<McpServer> {
  await server.connect(new StdioServerTransport());
  return server;
}

type ListedToolDefinition = {
  name: string;
  title?: string;
  description?: string;
  inputSchema?: ZodTypeAny;
  execute: (args: ToolHandlerArgs) => ToolExecuteResult;
};

export function listPluginToolDefinitions(definition: SDKPluginDefinition): ListedToolDefinition[] {
  if ("tools" in definition) {
    return definition.tools.map((tool) => ({
      name: tool.name,
      title: tool.title,
      description: tool.description,
      inputSchema: tool.inputSchema,
      execute: tool.execute as (args: ToolHandlerArgs) => ToolExecuteResult,
    }));
  }

  return [
    {
      name: definition.manifest.spec.review?.entrypoint ?? DEFAULT_REVIEW_ENTRYPOINT,
      title: definition.manifest.metadata.name,
      description: definition.manifest.metadata.description ?? "Run review plugin analysis.",
      inputSchema: definition.inputSchema ?? z.record(z.string(), z.unknown()),
      execute: definition.executeReview as (args: ToolHandlerArgs) => ToolExecuteResult,
    },
  ];
}
