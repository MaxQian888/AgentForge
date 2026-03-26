const post = jest.fn();
const get = jest.fn();
const put = jest.fn();
const del = jest.fn();

jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(() => ({
    post,
    get,
    put,
    delete: del,
  })),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: () => ({
      accessToken: "test-token",
    }),
  },
}));

import {
  DEFAULT_PLUGIN_PANEL_FILTERS,
  usePluginStore,
  type PluginRecord,
} from "./plugin-store";

describe("plugin store control-plane actions", () => {
  beforeEach(() => {
    post.mockReset();
    get.mockReset();
    put.mockReset();
    del.mockReset();

    usePluginStore.setState({
      plugins: [],
      builtins: [],
      marketplace: [],
      catalogResults: [],
      catalogQuery: "",
      events: {},
      mcpSnapshots: {},
      workflowRuns: {},
      selectedWorkflowRunId: null,
      filters: DEFAULT_PLUGIN_PANEL_FILTERS,
      selectedPluginId: null,
      loading: false,
      error: null,
    });
  });

  it("uses tool_name when proxying MCP tool calls", async () => {
    post.mockResolvedValue({
      data: {
        plugin_id: "repo-search",
        operation: "call_tool",
        result: {
          content: [{ type: "text", text: "found 3 files" }],
          isError: false,
        },
      },
      status: 200,
    });

    const result = await usePluginStore
      .getState()
      .callMCPTool("repo-search", "search", { query: "bridge" });

    expect(post).toHaveBeenCalledWith(
      "/api/v1/plugins/repo-search/mcp/tools/call",
      { tool_name: "search", arguments: { query: "bridge" } },
      { token: "test-token" },
    );
    expect(result).toEqual({
      content: [{ type: "text", text: "found 3 files" }],
      isError: false,
    });
  });

  it("uses the plugin source path and metadata when updating a plugin", async () => {
    post.mockResolvedValue({
      data: {},
      status: 200,
    });

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
        release: {
          version: "1.0.0",
          availableVersion: "1.1.0",
        },
      },
      lifecycle_state: "active",
      runtime_host: "ts-bridge",
      restart_count: 0,
    };

    await usePluginStore.getState().updatePlugin(plugin);

    expect(post).toHaveBeenCalledWith(
      "/api/v1/plugins/repo-search/update",
      {
        path: "/plugins/repo-search/manifest.yaml",
        source: plugin.source,
      },
      { token: "test-token" },
    );
  });
});
