import {
  filterMarketplaceEntries,
  filterPluginRecords,
  usePluginStore,
  type MarketplacePluginEntry,
  type PluginPanelFilters,
  type PluginRecord,
} from "./plugin-store";

const mockGet = jest.fn();
const mockPost = jest.fn();
const mockPut = jest.fn();
const mockDelete = jest.fn();

jest.mock("@/lib/api-client", () => ({
  createApiClient: () => ({
    get: mockGet,
    post: mockPost,
    put: mockPut,
    delete: mockDelete,
  }),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: { getState: () => ({ accessToken: "test-token" }) },
}));

const samplePlugin: PluginRecord = {
  apiVersion: "plugin.agentforge.dev/v1",
  kind: "ToolPlugin",
  metadata: {
    id: "github-tool",
    name: "GitHub Tool",
    version: "1.0.0",
    description: "GitHub integration",
    tags: ["github", "tool"],
  },
  spec: {
    runtime: "mcp",
    command: "npx",
    args: ["github-tool"],
  },
  permissions: {
    network: { required: true, domains: ["api.github.com"] },
  },
  source: { type: "builtin" },
  lifecycle_state: "active",
  runtime_host: "ts-bridge",
  restart_count: 2,
  resolved_source_path: "/plugins/github-tool",
};

const sampleFilters: PluginPanelFilters = {
  query: "",
  kind: "all",
  lifecycleState: "all",
  runtimeHost: "all",
  sourceType: "all",
};

beforeEach(() => {
  usePluginStore.setState({
    plugins: [],
    builtins: [],
    marketplace: [],
    filters: sampleFilters,
    selectedPluginId: null,
    loading: false,
    error: null,
  });
  mockGet.mockReset();
  mockPost.mockReset();
  mockPut.mockReset();
  mockDelete.mockReset();
});

describe("usePluginStore", () => {
  it("loads marketplace plugin entries", async () => {
    const marketplace: MarketplacePluginEntry[] = [
      {
        id: "role-coder",
        name: "Coder Role",
        description: "Default coding role",
        version: "1.0.0",
        author: "AgentForge",
        kind: "role",
        installUrl: "",
      },
    ];

    mockGet.mockResolvedValueOnce({ data: marketplace });

    await usePluginStore.getState().fetchMarketplace();

    expect(mockGet).toHaveBeenCalledWith("/api/v1/plugins/marketplace", {
      token: "test-token",
    });
    expect(usePluginStore.getState().marketplace).toEqual(marketplace);
  });

  it("tracks panel filters and selected plugin", () => {
    usePluginStore.getState().setFilters({
      kind: "ToolPlugin",
      runtimeHost: "ts-bridge",
      query: "github",
    });
    usePluginStore.getState().selectPlugin("github-tool");

    expect(usePluginStore.getState().filters).toEqual({
      ...sampleFilters,
      kind: "ToolPlugin",
      runtimeHost: "ts-bridge",
      query: "github",
    });
    expect(usePluginStore.getState().selectedPluginId).toBe("github-tool");

    usePluginStore.getState().resetFilters();
    expect(usePluginStore.getState().filters).toEqual(sampleFilters);
  });
});

describe("plugin panel filter helpers", () => {
  it("filters installed plugin records by kind, state, host, source, and query", () => {
    const results = filterPluginRecords(
      [
        samplePlugin,
        {
          ...samplePlugin,
          metadata: {
            ...samplePlugin.metadata,
            id: "review-plugin",
            name: "Review Plugin",
          },
          kind: "ReviewPlugin",
          lifecycle_state: "disabled",
          runtime_host: "ts-bridge",
          source: { type: "local" },
        },
      ],
      {
        query: "git",
        kind: "ToolPlugin",
        lifecycleState: "active",
        runtimeHost: "ts-bridge",
        sourceType: "builtin",
      },
    );

    expect(results).toEqual([samplePlugin]);
  });

  it("filters marketplace entries by kind and query without requiring install support", () => {
    const entries: MarketplacePluginEntry[] = [
      {
        id: "role-coder",
        name: "Coder Role",
        description: "Coding role",
        version: "1.0.0",
        author: "AgentForge",
        kind: "role",
        installUrl: "",
      },
      {
        id: "tool-github",
        name: "GitHub Tool",
        description: "GitHub MCP",
        version: "1.0.0",
        author: "AgentForge",
        kind: "tool",
        installUrl: "https://example.com/tool-github",
      },
    ];

    const results = filterMarketplaceEntries(entries, {
      ...sampleFilters,
      query: "git",
      kind: "all",
    });

    expect(results).toEqual([entries[1]]);
  });
});
