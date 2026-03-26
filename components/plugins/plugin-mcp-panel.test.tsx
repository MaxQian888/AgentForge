import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginMCPPanel } from "./plugin-mcp-panel";
import type { PluginRecord } from "@/lib/stores/plugin-store";

const refreshMCP = jest.fn();
const callMCPTool = jest.fn();
const readMCPResource = jest.fn();
const getMCPPrompt = jest.fn();

jest.mock("@/lib/stores/plugin-store", () => ({
  usePluginStore: (
    selector: (state: {
      mcpSnapshots: Record<string, unknown>;
      refreshMCP: typeof refreshMCP;
      callMCPTool: typeof callMCPTool;
      readMCPResource: typeof readMCPResource;
      getMCPPrompt: typeof getMCPPrompt;
    }) => unknown,
  ) =>
    selector({
      mcpSnapshots: {
        "repo-search": {
          transport: "stdio",
          tool_count: 1,
          resource_count: 1,
          prompt_count: 1,
          tools: [{ name: "search", description: "Search repository" }],
          resources: [{ uri: "file://README.md", name: "README" }],
          prompts: [{ name: "summarize", description: "Summarize a file" }],
        },
      },
      refreshMCP,
      callMCPTool,
      readMCPResource,
      getMCPPrompt,
    }),
}));

const plugin: PluginRecord = {
  apiVersion: "plugin.agentforge.dev/v1",
  kind: "ToolPlugin",
  metadata: {
    id: "repo-search",
    name: "Repo Search",
    version: "1.0.0",
  },
  spec: {
    runtime: "mcp",
  },
  permissions: {},
  source: {
    type: "local",
    path: "/plugins/repo-search/manifest.yaml",
  },
  lifecycle_state: "active",
  runtime_host: "ts-bridge",
  restart_count: 0,
  runtime_metadata: {
    compatible: true,
    mcp: {
      transport: "stdio",
      tool_count: 1,
      resource_count: 1,
      prompt_count: 1,
      latest_interaction: {
        operation: "call_tool",
        status: "succeeded",
        target: "search",
        summary: "found 3 files",
        at: "2026-03-26T00:00:00.000Z",
      },
    },
  },
};

describe("PluginMCPPanel", () => {
  beforeEach(() => {
    refreshMCP.mockReset();
    callMCPTool.mockReset();
    readMCPResource.mockReset();
    getMCPPrompt.mockReset();
  });

  it("renders capability counts and latest interaction details", () => {
    render(<PluginMCPPanel plugin={plugin} />);

    expect(screen.getByText("MCP Capabilities")).toBeInTheDocument();
    expect(screen.getByText("Transport: stdio")).toBeInTheDocument();
    expect(screen.getByText("Latest Interaction")).toBeInTheDocument();
    expect(screen.getByText("found 3 files")).toBeInTheDocument();
    expect(screen.getByText("search")).toBeInTheDocument();
    expect(screen.getByText("README")).toBeInTheDocument();
    expect(screen.getByText("summarize")).toBeInTheDocument();
  });

  it("refreshes MCP capabilities through the store action", async () => {
    const user = userEvent.setup();
    refreshMCP.mockResolvedValue(null);

    render(<PluginMCPPanel plugin={plugin} />);
    await user.click(screen.getByRole("button", { name: "Refresh" }));

    expect(refreshMCP).toHaveBeenCalledWith("repo-search");
  });
});
