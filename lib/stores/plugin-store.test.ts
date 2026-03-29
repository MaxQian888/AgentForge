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
  type RemoteMarketplaceState,
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
      remoteMarketplace: {
        available: false,
        registry: "",
        entries: [],
      } satisfies RemoteMarketplaceState,
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

  it("loads the remote marketplace envelope into store state", async () => {
    get.mockResolvedValue({
      data: {
        available: true,
        registry: "https://registry.agentforge.dev",
        entries: [
          {
            id: "release-train",
            name: "Release Train",
            description: "Workflow release automation",
            version: "1.2.0",
            author: "AgentForge",
            kind: "remote",
            registry: "https://registry.agentforge.dev",
            installable: true,
            sourceType: "registry",
          },
        ],
      },
      status: 200,
    });

    await usePluginStore.getState().fetchRemoteMarketplace();

    expect(get).toHaveBeenCalledWith("/api/v1/plugins/marketplace/remote", {
      token: "test-token",
    });
    expect(usePluginStore.getState().remoteMarketplace).toEqual({
      available: true,
      registry: "https://registry.agentforge.dev",
      entries: [
        expect.objectContaining({
          id: "release-train",
          installable: true,
          registry: "https://registry.agentforge.dev",
        }),
      ],
    });
  });

  it("posts explicit remote install requests with the selected version", async () => {
    post.mockResolvedValue({
      data: {
        ok: true,
        pluginId: "release-train",
        version: "1.2.0",
      },
      status: 200,
    });
    get.mockResolvedValue({
      data: [],
      status: 200,
    });

    await usePluginStore.getState().installFromRemote("release-train", "1.2.0");

    expect(post).toHaveBeenCalledWith(
      "/api/v1/plugins/marketplace/release-train/install-remote",
      { version: "1.2.0" },
      { token: "test-token" },
    );
  });
});
