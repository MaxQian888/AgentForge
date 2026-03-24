import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import PluginsPage from "./page";

const fetchPlugins = jest.fn();
const discoverBuiltins = jest.fn();
const fetchMarketplace = jest.fn();
const installLocal = jest.fn();
const setFilters = jest.fn();
const selectPlugin = jest.fn();

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
  builtins: [],
  marketplace: [
    {
      id: "role-coder",
      name: "Coder Role",
      description: "Coding role",
      version: "1.0.0",
      author: "AgentForge",
      kind: "role",
      installUrl: "",
    },
  ],
  filters: {
    query: "",
    kind: "all",
    lifecycleState: "all",
    runtimeHost: "all",
    sourceType: "all",
  },
  selectedPluginId: "github-tool",
  loading: false,
  error: null,
  fetchPlugins,
  discoverBuiltins,
  fetchMarketplace,
  installLocal,
  setFilters,
  resetFilters: jest.fn(),
  selectPlugin,
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

describe("PluginsPage", () => {
  beforeEach(() => {
    fetchPlugins.mockReset();
    discoverBuiltins.mockReset();
    fetchMarketplace.mockReset();
    installLocal.mockReset();
    setFilters.mockReset();
    selectPlugin.mockReset();
  });

  it("loads installed, builtin, and marketplace data on mount", () => {
    render(<PluginsPage />);

    expect(fetchPlugins).toHaveBeenCalled();
    expect(discoverBuiltins).toHaveBeenCalled();
    expect(fetchMarketplace).toHaveBeenCalled();
  });

  it("renders filter controls, marketplace, and selected plugin details", () => {
    render(<PluginsPage />);

    expect(screen.getByLabelText("Search plugins")).toBeInTheDocument();
    expect(screen.getByText("Marketplace")).toBeInTheDocument();
    expect(screen.getByText("Plugin details")).toBeInTheDocument();
    expect(screen.getByText("Runtime host")).toBeInTheDocument();
    expect(screen.getByText("Browse only")).toBeInTheDocument();
  });

  it("updates the query filter from the search input", async () => {
    const user = userEvent.setup();
    render(<PluginsPage />);

    await user.type(screen.getByLabelText("Search plugins"), "git");

    expect(setFilters).toHaveBeenCalledWith({ query: "git" });
  });
});
