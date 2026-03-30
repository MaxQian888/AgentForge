const post = jest.fn();
const get = jest.fn();
const put = jest.fn();
const del = jest.fn();

jest.mock("@/lib/api-client", () => {
  const actual = jest.requireActual("@/lib/api-client");
  return {
    ...actual,
    createApiClient: jest.fn(() => ({
      post,
      get,
      put,
      delete: del,
    })),
  };
});

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({
      accessToken: "test-token",
    })),
  },
}));

import {
  type RemoteMarketplaceState,
  DEFAULT_PLUGIN_PANEL_FILTERS,
  filterMarketplaceEntries,
  filterPluginRecords,
  usePluginStore,
  type MarketplacePluginEntry,
  type PluginRecord,
  type WorkflowPluginRun,
} from "./plugin-store";

const authStoreModule = jest.requireMock("./auth-store") as {
  useAuthStore: {
    getState: jest.Mock<{ accessToken: string | null }, []>;
  };
};

function createPluginRecord(
  id: string,
  overrides: Partial<PluginRecord> = {},
): PluginRecord {
  return {
    apiVersion: "plugin.agentforge.dev/v1",
    kind: "ToolPlugin",
    metadata: {
      id,
      name: `Plugin ${id}`,
      version: "1.0.0",
      description: "Searches the repository",
      tags: ["search", "repo"],
      ...(overrides.metadata ?? {}),
    },
    spec: {
      runtime: "mcp",
      ...(overrides.spec ?? {}),
    },
    permissions: overrides.permissions ?? {},
    source: {
      type: "local",
      path: `/plugins/${id}/manifest.yaml`,
      ...(overrides.source ?? {}),
    },
    lifecycle_state: "active",
    runtime_host: "ts-bridge",
    restart_count: 0,
    ...(overrides ?? {}),
  };
}

function createMarketplaceEntry(
  id: string,
  overrides: Partial<MarketplacePluginEntry> = {},
): MarketplacePluginEntry {
  return {
    id,
    name: `Marketplace ${id}`,
    description: "Release automation",
    version: "1.0.0",
    author: "AgentForge",
    kind: "workflow",
    sourceType: "registry",
    ...overrides,
  };
}

function createWorkflowRun(
  id: string,
  overrides: Partial<WorkflowPluginRun> = {},
): WorkflowPluginRun {
  return {
    id,
    plugin_id: "release-train",
    process: "sequential",
    status: "running",
    started_at: "2026-03-30T10:00:00.000Z",
    ...overrides,
  };
}

describe("plugin store control-plane actions", () => {
  beforeEach(() => {
    post.mockReset();
    get.mockReset();
    put.mockReset();
    del.mockReset();
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: "test-token",
    });

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

  it("filters plugin records across kind, runtime host, source type, and search text", () => {
    const plugins = [
      createPluginRecord("repo-search", {
        metadata: {
          id: "repo-search",
          name: "Repo Search",
          version: "1.0.0",
          description: "Searches the repository",
          tags: ["search", "repo"],
        },
      }),
      createPluginRecord("review-bot", {
        kind: "ReviewPlugin",
        runtime_host: "go-orchestrator",
        source: {
          type: "builtin",
        },
        metadata: {
          id: "review-bot",
          name: "Review Bot",
          version: "1.0.0",
          description: "Reviews pull requests",
          tags: ["review"],
        },
      }),
    ];

    expect(
      filterPluginRecords(plugins, {
        ...DEFAULT_PLUGIN_PANEL_FILTERS,
        kind: "ReviewPlugin",
      }),
    ).toEqual([plugins[1]]);

    expect(
      filterPluginRecords(plugins, {
        ...DEFAULT_PLUGIN_PANEL_FILTERS,
        runtimeHost: "ts-bridge",
        sourceType: "local",
        query: "repo",
      }),
    ).toEqual([plugins[0]]);
  });

  it("filters marketplace entries for marketplace-only sources and fuzzy queries", () => {
    const entries = [
      createMarketplaceEntry("release-train", {
        kind: "workflow",
        sourceType: "registry",
      }),
      createMarketplaceEntry("local-tool", {
        kind: "tool",
        sourceType: "local",
        description: "Local helper",
      }),
    ];

    expect(
      filterMarketplaceEntries(entries, {
        ...DEFAULT_PLUGIN_PANEL_FILTERS,
        sourceType: "marketplace",
        query: "release",
      }),
    ).toEqual([entries[0]]);

    expect(
      filterMarketplaceEntries(entries, {
        ...DEFAULT_PLUGIN_PANEL_FILTERS,
        kind: "ToolPlugin",
        sourceType: "local",
      }),
    ).toEqual([entries[1]]);
  });

  it("fetches plugin, builtin, and marketplace lists", async () => {
    get.mockResolvedValueOnce({
      data: [createPluginRecord("repo-search")],
      status: 200,
    });
    get.mockResolvedValueOnce({
      data: [createPluginRecord("builtin-review", { source: { type: "builtin" } })],
      status: 200,
    });
    get.mockResolvedValueOnce({
      data: [createMarketplaceEntry("release-train")],
      status: 200,
    });

    await usePluginStore.getState().fetchPlugins();
    await usePluginStore.getState().discoverBuiltins();
    await usePluginStore.getState().fetchMarketplace();

    expect(usePluginStore.getState()).toMatchObject({
      plugins: [expect.objectContaining({ metadata: expect.objectContaining({ id: "repo-search" }) })],
      builtins: [expect.objectContaining({ source: expect.objectContaining({ type: "builtin" }) })],
      marketplace: [expect.objectContaining({ id: "release-train" })],
      loading: false,
    });
  });

  it("stores list-loading failures with descriptive errors", async () => {
    get.mockRejectedValueOnce(new Error("boom"));
    await usePluginStore.getState().fetchPlugins();
    expect(usePluginStore.getState().error).toBe("Unable to load plugins");

    get.mockRejectedValueOnce(new Error("boom"));
    await usePluginStore.getState().discoverBuiltins();
    expect(usePluginStore.getState().error).toBe(
      "Unable to discover built-in plugins",
    );

    get.mockRejectedValueOnce(new Error("boom"));
    await usePluginStore.getState().fetchMarketplace();
    expect(usePluginStore.getState().error).toBe(
      "Unable to load plugin marketplace",
    );
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

  it("runs lifecycle mutations and refreshes the plugin list", async () => {
    post.mockResolvedValue({ data: {}, status: 200 });
    put.mockResolvedValue({ data: {}, status: 200 });
    del.mockResolvedValue({ data: {}, status: 200 });
    get.mockResolvedValue({ data: [], status: 200 });

    const store = usePluginStore.getState();
    await store.installLocal("/plugins/repo-search/manifest.yaml");
    await store.enablePlugin("repo-search");
    await store.disablePlugin("repo-search");
    await store.activatePlugin("repo-search");
    await store.deactivatePlugin("repo-search");
    await store.updateConfig("repo-search", { enabled: true });
    await store.checkHealth("repo-search");
    await store.restartPlugin("repo-search");
    await store.uninstallPlugin("repo-search");

    expect(post).toHaveBeenCalledWith(
      "/api/v1/plugins/install",
      { path: "/plugins/repo-search/manifest.yaml" },
      { token: "test-token" },
    );
    expect(put).toHaveBeenCalledWith(
      "/api/v1/plugins/repo-search/enable",
      {},
      { token: "test-token" },
    );
    expect(put).toHaveBeenCalledWith(
      "/api/v1/plugins/repo-search/disable",
      {},
      { token: "test-token" },
    );
    expect(post).toHaveBeenCalledWith(
      "/api/v1/plugins/repo-search/activate",
      {},
      { token: "test-token" },
    );
    expect(post).toHaveBeenCalledWith(
      "/api/v1/plugins/repo-search/deactivate",
      {},
      { token: "test-token" },
    );
    expect(put).toHaveBeenCalledWith(
      "/api/v1/plugins/repo-search/config",
      { config: { enabled: true } },
      { token: "test-token" },
    );
    expect(get).toHaveBeenCalledWith(
      "/api/v1/plugins/repo-search/health",
      { token: "test-token" },
    );
    expect(post).toHaveBeenCalledWith(
      "/api/v1/plugins/repo-search/restart",
      {},
      { token: "test-token" },
    );
    expect(del).toHaveBeenCalledWith("/api/v1/plugins/repo-search", {
      token: "test-token",
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

  it("surfaces an unsupported update source when no path is available", async () => {
    await usePluginStore.getState().updatePlugin(
      createPluginRecord("registry-only", {
        source: {
          type: "registry",
          path: undefined,
        },
        resolved_source_path: undefined,
      }),
    );

    expect(usePluginStore.getState().error).toBe(
      "No supported update source is available for this plugin",
    );
    expect(post).not.toHaveBeenCalled();
  });

  it("invokes plugins with a default empty payload and captures failures", async () => {
    post.mockResolvedValueOnce({
      data: { ok: true },
      status: 200,
    });
    post.mockRejectedValueOnce(new Error("invoke failed"));

    await expect(
      usePluginStore.getState().invokePlugin("repo-search", "run"),
    ).resolves.toEqual({ ok: true });
    await expect(
      usePluginStore.getState().invokePlugin("repo-search", "run"),
    ).resolves.toBeNull();

    expect(post).toHaveBeenNthCalledWith(
      1,
      "/api/v1/plugins/repo-search/invoke",
      { operation: "run", payload: {} },
      { token: "test-token" },
    );
    expect(usePluginStore.getState().error).toBe("Failed to invoke plugin");
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

  it("propagates remote registry install errors from thrown messages", async () => {
    post.mockRejectedValueOnce(new Error("Approval required"));

    await usePluginStore.getState().installFromRemote("release-train");

    expect(usePluginStore.getState().error).toBe("Approval required");
  });

  it("searches the catalog and installs entries from the catalog", async () => {
    get.mockResolvedValueOnce({
      data: [createMarketplaceEntry("repo-search")],
      status: 200,
    });
    post.mockResolvedValueOnce({ data: {}, status: 200 });
    get.mockResolvedValueOnce({ data: [], status: 200 });

    await usePluginStore.getState().searchCatalog(" repo search ");
    await usePluginStore.getState().installFromCatalog("repo-search");

    expect(usePluginStore.getState()).toMatchObject({
      catalogQuery: " repo search ",
      catalogResults: [expect.objectContaining({ id: "repo-search" })],
    });
    expect(get).toHaveBeenCalledWith(
      "/api/v1/plugins/catalog?q=%20repo%20search%20",
      { token: "test-token" },
    );
    expect(post).toHaveBeenCalledWith(
      "/api/v1/plugins/catalog/install",
      { entry_id: "repo-search" },
      { token: "test-token" },
    );
  });

  it("refreshes MCP metadata and proxies resource and prompt reads", async () => {
    post
      .mockResolvedValueOnce({
        data: {
          plugin_id: "repo-search",
          snapshot: {
            transport: "stdio",
            tool_count: 1,
            resource_count: 1,
            prompt_count: 1,
          },
        },
        status: 200,
      })
      .mockResolvedValueOnce({
        data: {
          result: {
            contents: [{ uri: "memory://doc", text: "hello" }],
          },
        },
        status: 200,
      })
      .mockResolvedValueOnce({
        data: {
          result: {
            description: "Prompt",
            messages: [
              {
                role: "user",
                content: { type: "text", text: "hello" },
              },
            ],
          },
        },
        status: 200,
      });
    get.mockResolvedValueOnce({ data: [], status: 200 });

    await expect(
      usePluginStore.getState().refreshMCP("repo-search"),
    ).resolves.toEqual(
      expect.objectContaining({
        plugin_id: "repo-search",
      }),
    );
    await expect(
      usePluginStore.getState().readMCPResource("repo-search", "memory://doc"),
    ).resolves.toEqual({
      contents: [{ uri: "memory://doc", text: "hello" }],
    });
    await expect(
      usePluginStore.getState().getMCPPrompt("repo-search", "summarize", {
        topic: "bridge",
      }),
    ).resolves.toEqual(
      expect.objectContaining({
        description: "Prompt",
      }),
    );

    expect(usePluginStore.getState().mcpSnapshots["repo-search"]).toEqual(
      expect.objectContaining({
        transport: "stdio",
      }),
    );
  });

  it("stores plugin events and workflow runs and can append detailed workflow data", async () => {
    get
      .mockResolvedValueOnce({
        data: [
          {
            id: "evt-1",
            plugin_id: "release-train",
            event_type: "installed",
            event_source: "control-plane",
          },
        ],
        status: 200,
      })
      .mockResolvedValueOnce({
        data: [createWorkflowRun("run-1")],
        status: 200,
      })
      .mockResolvedValueOnce({
        data: [createWorkflowRun("run-1")],
        status: 200,
      })
      .mockResolvedValueOnce({
        data: createWorkflowRun("run-2", { status: "completed" }),
        status: 200,
      });
    post.mockResolvedValueOnce({ data: {}, status: 200 });

    await usePluginStore.getState().fetchEvents("release-train", 10);
    await usePluginStore.getState().startWorkflowRun("release-train", {
      source: "manual",
    });
    await usePluginStore.getState().fetchWorkflowRuns("release-train");
    await usePluginStore.getState().fetchWorkflowRun("run-2");

    expect(usePluginStore.getState().events["release-train"]).toEqual([
      expect.objectContaining({ id: "evt-1" }),
    ]);
    expect(usePluginStore.getState().workflowRuns["release-train"]).toEqual([
      expect.objectContaining({ id: "run-1" }),
      expect.objectContaining({ id: "run-2", status: "completed" }),
    ]);
    expect(post).toHaveBeenCalledWith(
      "/api/v1/plugins/release-train/workflow-runs",
      { trigger: { source: "manual" } },
      { token: "test-token" },
    );
  });

  it("updates panel state for catalog query, workflow selection, filters, and selected plugin", () => {
    const store = usePluginStore.getState();

    store.setCatalogQuery("repo");
    store.selectWorkflowRun("run-1");
    store.selectPlugin("repo-search");
    store.setFilters({
      query: "release",
      sourceType: "registry",
    });

    expect(usePluginStore.getState()).toMatchObject({
      catalogQuery: "repo",
      selectedWorkflowRunId: "run-1",
      selectedPluginId: "repo-search",
      filters: expect.objectContaining({
        query: "release",
        sourceType: "registry",
      }),
    });

    store.resetFilters();
    expect(usePluginStore.getState().filters).toEqual(
      DEFAULT_PLUGIN_PANEL_FILTERS,
    );
  });

  it("returns early when no auth token is available", async () => {
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: null,
    });

    await usePluginStore.getState().fetchPlugins();
    await usePluginStore.getState().discoverBuiltins();
    await usePluginStore.getState().fetchMarketplace();
    await usePluginStore.getState().fetchRemoteMarketplace();
    await usePluginStore.getState().installLocal("/plugins/repo-search/manifest.yaml");
    await usePluginStore.getState().enablePlugin("repo-search");
    await usePluginStore.getState().disablePlugin("repo-search");
    await usePluginStore.getState().activatePlugin("repo-search");
    await usePluginStore.getState().deactivatePlugin("repo-search");
    await usePluginStore.getState().uninstallPlugin("repo-search");
    await usePluginStore.getState().updatePlugin(createPluginRecord("repo-search"));
    await usePluginStore.getState().updateConfig("repo-search", { enabled: true });
    await usePluginStore.getState().checkHealth("repo-search");
    await usePluginStore.getState().restartPlugin("repo-search");
    await expect(
      usePluginStore.getState().invokePlugin("repo-search", "run"),
    ).resolves.toBeNull();
    await usePluginStore.getState().searchCatalog("repo");
    await usePluginStore.getState().installFromCatalog("repo-search");
    await usePluginStore.getState().installFromRemote("repo-search");
    await expect(usePluginStore.getState().refreshMCP("repo-search")).resolves.toBeNull();
    await expect(
      usePluginStore.getState().callMCPTool("repo-search", "search"),
    ).resolves.toBeNull();
    await expect(
      usePluginStore.getState().readMCPResource("repo-search", "memory://doc"),
    ).resolves.toBeNull();
    await expect(
      usePluginStore.getState().getMCPPrompt("repo-search", "summarize"),
    ).resolves.toBeNull();
    await usePluginStore.getState().fetchEvents("repo-search");
    await usePluginStore.getState().startWorkflowRun("repo-search");
    await usePluginStore.getState().fetchWorkflowRuns("repo-search");
    await usePluginStore.getState().fetchWorkflowRun("run-1");

    expect(post).not.toHaveBeenCalled();
    expect(get).not.toHaveBeenCalled();
    expect(put).not.toHaveBeenCalled();
    expect(del).not.toHaveBeenCalled();
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
