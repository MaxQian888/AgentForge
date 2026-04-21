import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import pluginMessages from "../../../messages/en/plugins.json";

function translatePluginKey(key: string, values?: Record<string, string>) {
  const resolved = key
    .split(".")
    .reduce<unknown>((current, segment) => {
      if (!current || typeof current !== "object") return undefined;
      return (current as Record<string, unknown>)[segment];
    }, pluginMessages);
  if (typeof resolved !== "string") {
    return key;
  }
  return Object.entries(values ?? {}).reduce(
    (message, [token, value]) => message.replace(`{${token}}`, value),
    resolved,
  );
}

jest.mock("next-intl", () => ({
  useTranslations: () =>
    (key: string, values?: Record<string, string>) =>
      translatePluginKey(key, values),
}));
import PluginsPage from "./page";

const checkForUpdate = jest.fn();
const installUpdate = jest.fn();
const fetchPlugins = jest.fn();
const discoverBuiltins = jest.fn();
const fetchMarketplace = jest.fn();
const fetchRemoteMarketplace = jest.fn();
const installFromCatalog = jest.fn();
const installFromRemote = jest.fn();
const getDesktopRuntimeStatus = jest.fn();
const getPluginRuntimeSummary = jest.fn();
const installLocal = jest.fn();
let isDesktop = false;
const setFilters = jest.fn();
const setViewCategory = jest.fn();
const sendNotification = jest.fn();
const selectPlugin = jest.fn();
const selectMarketplaceEntry = jest.fn();
const subscribeDesktopEvents = jest.fn();
const updateTray = jest.fn();
const relaunchToUpdate = jest.fn();

const storeState = {
  plugins: [
    {
      apiVersion: "plugin.agentforge.dev/v1",
      kind: "ToolPlugin",
      metadata: {
        id: "github-tool",
        name: "GitHub Tool",
        version: "1.0.0",
        description: "GitHub integration",
        tags: ["github"],
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
      restart_count: 1,
      resolved_source_path: "/plugins/github-tool",
      runtime_metadata: { compatible: true, abi_version: "1" },
      last_error: "",
      last_health_at: "2026-03-24T12:00:00.000Z",
    },
  ],
  builtins: [
    {
      apiVersion: "plugin.agentforge.dev/v1",
      kind: "IntegrationPlugin",
      metadata: {
        id: "sample-integration-plugin",
        name: "Sample Integration Plugin",
        version: "1.0.0",
        description: "Built-in Feishu adapter",
      },
      spec: {
        runtime: "wasm",
        module: "./dist/feishu.wasm",
        abiVersion: "v1",
      },
      permissions: {},
      source: {
        type: "builtin",
        path: "/plugins/sample-integration-plugin/manifest.yaml",
      },
      lifecycle_state: "installed",
      runtime_host: "go-orchestrator",
      restart_count: 0,
      resolved_source_path: "./dist/feishu.wasm",
      runtime_metadata: { compatible: true, abi_version: "v1" },
      last_error: "",
      last_health_at: "",
      builtIn: {
        official: true,
        docsRef: "docs/GO_WASM_PLUGIN_RUNTIME.md",
        verificationProfile: "go-wasm",
        availabilityStatus: "requires_configuration",
        availabilityMessage:
          "Requires Feishu application credentials before live activation.",
        readinessStatus: "requires_configuration",
        readinessMessage:
          "Requires Feishu application credentials before live activation.",
        nextStep: "Set FEISHU_APP_ID and FEISHU_APP_SECRET on the bridge host.",
        blockingReasons: ["missing_configuration"],
        missingConfiguration: ["FEISHU_APP_ID", "FEISHU_APP_SECRET"],
        installable: true,
        installBlockedReason: "",
      },
    },
  ],
  marketplace: [
    {
      id: "role-coder",
      name: "Coder Role",
      description: "Coding role",
      version: "1.0.0",
      author: "AgentForge",
      kind: "role",
      installUrl: "",
      sourceType: "marketplace",
    },
    {
      id: "release-train",
      name: "Release Train",
      description: "Workflow automation",
      version: "1.1.0",
      author: "AgentForge",
      kind: "workflow",
      installUrl: "catalog://release-train",
      sourceType: "catalog",
    },
  ],
  remoteMarketplace: {
    available: true,
    registry: "https://registry.agentforge.dev",
    error: undefined as string | undefined,
    errorCode: undefined as string | undefined,
    entries: [
      {
        id: "remote-release-train",
        name: "Remote Release Train",
        description: "Hosted workflow automation",
        version: "2.0.0",
        author: "AgentForge Registry",
        kind: "remote",
        registry: "https://registry.agentforge.dev",
        installable: true,
        sourceType: "registry",
      },
    ],
  },
  filters: {
    query: "",
    kind: "all",
    lifecycleState: "all",
    runtimeHost: "all",
    sourceType: "all",
  },
  viewCategory: "installed",
  selectedPluginId: "github-tool",
  selectedMarketplaceId: null as string | null,
  loading: false,
  error: null,
  fetchPlugins,
  discoverBuiltins,
  fetchMarketplace,
  fetchRemoteMarketplace,
  installLocal,
  installFromCatalog,
  installFromRemote,
  setFilters,
  setViewCategory,
  resetFilters: jest.fn(),
  selectPlugin,
  selectMarketplaceEntry,
  enablePlugin: jest.fn(),
  disablePlugin: jest.fn(),
  activatePlugin: jest.fn(),
  uninstallPlugin: jest.fn(),
  updateConfig: jest.fn(),
  checkHealth: jest.fn(),
  restartPlugin: jest.fn(),
};

jest.mock("@/lib/stores/plugin-store", () => ({
  usePluginStore: (selector: (state: typeof storeState) => unknown) => selector(storeState),
  filterPluginRecords: (plugins: typeof storeState.plugins) => plugins,
  filterMarketplaceEntries: (entries: typeof storeState.marketplace) => entries,
}));

jest.mock("@/hooks/use-platform-capability", () => ({
  usePlatformCapability: () => ({
    checkForUpdate,
    installUpdate,
    getDesktopRuntimeStatus,
    getPluginRuntimeSummary,
    isDesktop,
    relaunchToUpdate,
    sendNotification,
    subscribeDesktopEvents,
    updateTray,
  }),
}));

describe("PluginsPage", () => {
  function getCategoryButton(label: string) {
    const textNode = screen.getByText(new RegExp(`^${label}$`, "i"));
    const button = textNode.closest("button");
    if (!button) {
      throw new Error(`Unable to find category button for ${label}`);
    }
    return button;
  }

  beforeEach(() => {
    checkForUpdate.mockReset();
    installUpdate.mockReset();
    isDesktop = false;
    fetchPlugins.mockReset();
    discoverBuiltins.mockReset();
    fetchMarketplace.mockReset();
    fetchRemoteMarketplace.mockReset();
    installFromCatalog.mockReset();
    installFromRemote.mockReset();
    setViewCategory.mockReset();
    setViewCategory.mockImplementation((next) => {
      storeState.viewCategory = next;
    });
    storeState.builtins = [
      {
        apiVersion: "plugin.agentforge.dev/v1",
        kind: "IntegrationPlugin",
        metadata: {
          id: "sample-integration-plugin",
          name: "Sample Integration Plugin",
          version: "1.0.0",
          description: "Built-in Feishu adapter",
        },
        spec: {
          runtime: "wasm",
          module: "./dist/feishu.wasm",
          abiVersion: "v1",
        },
        permissions: {},
        source: {
          type: "builtin",
          path: "/plugins/sample-integration-plugin/manifest.yaml",
        },
        lifecycle_state: "installed",
        runtime_host: "go-orchestrator",
        restart_count: 0,
        resolved_source_path: "./dist/feishu.wasm",
        runtime_metadata: { compatible: true, abi_version: "v1" },
        last_error: "",
        last_health_at: "",
        builtIn: {
          official: true,
          docsRef: "docs/GO_WASM_PLUGIN_RUNTIME.md",
          verificationProfile: "go-wasm",
          availabilityStatus: "requires_configuration",
          availabilityMessage:
            "Requires Feishu application credentials before live activation.",
          readinessStatus: "requires_configuration",
          readinessMessage:
            "Requires Feishu application credentials before live activation.",
          nextStep: "Set FEISHU_APP_ID and FEISHU_APP_SECRET on the bridge host.",
          blockingReasons: ["missing_configuration"],
          missingConfiguration: ["FEISHU_APP_ID", "FEISHU_APP_SECRET"],
          installable: true,
          installBlockedReason: "",
        },
      },
    ];
    storeState.remoteMarketplace = {
      available: true,
      registry: "https://registry.agentforge.dev",
      error: undefined,
      errorCode: undefined,
      entries: [
        {
          id: "remote-release-train",
          name: "Remote Release Train",
          description: "Hosted workflow automation",
          version: "2.0.0",
          author: "AgentForge Registry",
          kind: "remote",
          registry: "https://registry.agentforge.dev",
          installable: true,
          sourceType: "registry",
        },
      ],
    };
    storeState.viewCategory = "installed";
    storeState.selectedMarketplaceId = null;
    getDesktopRuntimeStatus.mockReset();
    getDesktopRuntimeStatus.mockResolvedValue({
      overall: "stopped",
      backend: {
        label: "backend",
        status: "stopped",
        url: null,
        pid: null,
        restartCount: 0,
        lastError: null,
        lastStartedAt: null,
      },
      bridge: {
        label: "bridge",
        status: "stopped",
        url: null,
        pid: null,
        restartCount: 0,
        lastError: null,
        lastStartedAt: null,
      },
      imBridge: {
        label: "im-bridge",
        status: "stopped",
        url: null,
        pid: null,
        restartCount: 0,
        lastError: null,
        lastStartedAt: null,
      },
    });
    getPluginRuntimeSummary.mockReset();
    getPluginRuntimeSummary.mockResolvedValue({
      activeRuntimeCount: 0,
      backendHealthy: false,
      bridgeHealthy: false,
      bridgePluginCount: 0,
      eventBridgeAvailable: false,
      lastUpdatedAt: null,
      warnings: [],
    });
    installLocal.mockReset();
    sendNotification.mockReset();
    setFilters.mockReset();
    setFilters.mockImplementation((next) => {
      storeState.filters = { ...storeState.filters, ...next };
    });
    selectPlugin.mockReset();
    selectPlugin.mockImplementation((pluginId) => {
      storeState.selectedPluginId = pluginId;
    });
    selectMarketplaceEntry.mockReset();
    selectMarketplaceEntry.mockImplementation((entryId) => {
      storeState.selectedMarketplaceId = entryId;
    });
    subscribeDesktopEvents.mockReset();
    subscribeDesktopEvents.mockResolvedValue(jest.fn());
    relaunchToUpdate.mockReset();
    updateTray.mockReset();
  });

  it("loads installed, builtin, and marketplace data on mount", () => {
    render(<PluginsPage />);

    expect(fetchPlugins).toHaveBeenCalled();
    expect(discoverBuiltins).toHaveBeenCalled();
    expect(fetchMarketplace).toHaveBeenCalled();
    expect(fetchRemoteMarketplace).toHaveBeenCalled();
  });

  it("renders tabs, search, and desktop runtime header", () => {
    render(<PluginsPage />);

    expect(screen.getByText("Desktop runtime")).toBeInTheDocument();
    expect(getCategoryButton("Installed")).toBeInTheDocument();
    expect(getCategoryButton("Built-in")).toBeInTheDocument();
    expect(getCategoryButton("Marketplace")).toBeInTheDocument();
    expect(getCategoryButton("Remote")).toBeInTheDocument();
    expect(screen.getByLabelText("Search plugins")).toBeInTheDocument();
    expect(screen.getByText("Desktop runtime")).toBeInTheDocument();
  });

  it("updates the query filter from the search input", () => {
    render(<PluginsPage />);

    fireEvent.change(screen.getByLabelText("Search plugins"), {
      target: { value: "git" },
    });

    expect(setFilters).toHaveBeenLastCalledWith({ query: "git" });
  });

  it("expands runtime panel and shows desktop update after check", async () => {
    const user = userEvent.setup();
    checkForUpdate.mockResolvedValue({
      mode: "desktop",
      ok: true,
      status: "available",
      update: {
        currentVersion: "0.1.0",
        notes: "Important fixes",
        publishedAt: "2026-03-25T04:00:00.000Z",
        version: "0.2.0",
      },
    });

    render(<PluginsPage />);

    await user.click(screen.getByRole("button", { name: "Show runtime" }));
    await user.click(screen.getByRole("button", { name: "Check update" }));

    expect(
      await screen.findByText("Update 0.2.0 is ready to install."),
    ).toBeInTheDocument();
    expect(screen.getByText("Current version: 0.1.0")).toBeInTheDocument();
    expect(screen.getByText("Important fixes")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Install update" }),
    ).toBeInTheDocument();
  });

  it("shows relaunch action after the desktop update installs", async () => {
    const user = userEvent.setup();
    checkForUpdate.mockResolvedValue({
      mode: "desktop",
      ok: true,
      status: "available",
      update: {
        currentVersion: "0.1.0",
        notes: "Important fixes",
        publishedAt: "2026-03-25T04:00:00.000Z",
        version: "0.2.0",
      },
    });
    installUpdate.mockResolvedValue({
      mode: "desktop",
      ok: true,
      status: "ready_to_relaunch",
      update: {
        currentVersion: "0.1.0",
        notes: "Important fixes",
        publishedAt: "2026-03-25T04:00:00.000Z",
        version: "0.2.0",
      },
    });

    render(<PluginsPage />);
    await user.click(screen.getByRole("button", { name: "Show runtime" }));
    await user.click(screen.getByRole("button", { name: "Check update" }));
    await user.click(await screen.findByRole("button", { name: "Install update" }));

    expect(installUpdate).toHaveBeenCalled();
    expect(
      await screen.findByRole("button", { name: "Restart to update" }),
    ).toBeInTheDocument();
  });

  it("sends a structured desktop notification payload from the runtime panel", async () => {
    const user = userEvent.setup();
    sendNotification.mockResolvedValue({
      mode: "web",
      notificationId: "plugins-desktop-runtime-stopped",
      ok: true,
      status: "delivered",
    });

    render(<PluginsPage />);
    await user.click(screen.getByRole("button", { name: "Show runtime" }));
    await user.click(screen.getByRole("button", { name: "Notify" }));

    expect(sendNotification).toHaveBeenCalledWith(
      expect.objectContaining({
        notificationId: "plugins-desktop-runtime-stopped",
        notificationType: "desktop.runtime.status",
        title: "AgentForge Desktop",
        body: "Desktop runtime is currently stopped.",
        deliveryPolicy: "always",
      }),
    );
    expect(sendNotification.mock.calls[0]?.[0]).toEqual(
      expect.objectContaining({
        createdAt: expect.any(String),
      }),
    );
  });

  it("refreshes plugin surfaces when a projected plugin lifecycle desktop event arrives", async () => {
    isDesktop = true;
    subscribeDesktopEvents.mockImplementationOnce(
      async (
        handler: (event: {
          type: string;
          source?: string;
          timestamp?: string;
          payload?: unknown;
        }) => void,
      ) => {
        handler({
          type: "plugin.lifecycle",
          source: "plugin",
          timestamp: "2026-03-28T10:05:00.000Z",
          payload: {
            plugin_id: "github-tool",
            event_type: "activated",
          },
        });
        return jest.fn();
      },
    );

    render(<PluginsPage />);

    await userEvent.click(screen.getByRole("button", { name: "Show runtime" }));

    expect(await screen.findByText(/Last desktop event.*plugin\.lifecycle/)).toBeInTheDocument();
    expect(await screen.findByText(/Event bridge.*available/)).toBeInTheDocument();
    expect(fetchPlugins).toHaveBeenCalledTimes(2);
  });

  it("shows built-in plugins in the built-in tab", async () => {
    storeState.viewCategory = "builtin";
    storeState.selectedMarketplaceId = "sample-integration-plugin";
    render(<PluginsPage />);

    expect(screen.getByText("Sample Integration Plugin")).toBeInTheDocument();
  });

  it("installs built-in availability entries through the explicit catalog flow", async () => {
    const user = userEvent.setup();
    storeState.viewCategory = "builtin";
    render(<PluginsPage />);
    await user.click(screen.getByRole("button", { name: "Install" }));

    expect(installFromCatalog).toHaveBeenCalledWith("sample-integration-plugin");
    expect(installLocal).not.toHaveBeenCalled();
  });

  it("renders unsupported-host built-ins as blocked instead of installable", async () => {
    storeState.builtins = [
      {
        ...storeState.builtins[0],
        metadata: {
          ...storeState.builtins[0].metadata,
          id: "desktop-only-tool",
          name: "Desktop Only Tool",
        },
        builtIn: {
          ...storeState.builtins[0].builtIn,
          availabilityStatus: "unsupported_host",
          availabilityMessage: "This built-in is not supported on the current host.",
          readinessStatus: "unsupported_host",
          readinessMessage: "This built-in is not supported on the current host.",
          nextStep: "Use a supported host family for this built-in.",
          blockingReasons: ["unsupported_host"],
          missingConfiguration: [],
          installable: false,
          installBlockedReason: "This built-in is not supported on the current host.",
        },
      },
    ];

    storeState.viewCategory = "builtin";
    storeState.selectedMarketplaceId = "desktop-only-tool";
    render(<PluginsPage />);

    expect(screen.getByText("Desktop Only Tool")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Install" })).not.toBeInTheDocument();
  });

  it("shows marketplace entries in the marketplace tab", async () => {
    storeState.viewCategory = "marketplace";
    render(<PluginsPage />);

    expect(screen.getByText("Coder Role")).toBeInTheDocument();
  });

  it("shows remote entries in the remote tab and supports install", async () => {
    const user = userEvent.setup();
    storeState.viewCategory = "remote";
    storeState.selectedMarketplaceId = "remote-release-train";
    render(<PluginsPage />);

    expect(screen.getByText("Remote Release Train")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Install" }));

    expect(installFromRemote).toHaveBeenCalledWith("remote-release-train", "2.0.0");
  });

  it("shows remote registry availability failures without breaking the rest of the page", async () => {
    storeState.remoteMarketplace = {
      available: false,
      registry: "https://registry.agentforge.dev",
      error: "Registry unavailable",
      errorCode: "remote_registry_unavailable",
      entries: [],
    };

    storeState.viewCategory = "remote";
    render(<PluginsPage />);

    expect(screen.getByText("Registry unavailable")).toBeInTheDocument();
    expect(getCategoryButton("Installed")).toBeInTheDocument();
  });
});
