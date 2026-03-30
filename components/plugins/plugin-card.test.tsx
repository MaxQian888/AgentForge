import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginCard } from "./plugin-card";
import type { PluginRecord } from "@/lib/stores/plugin-store";

const enablePlugin = jest.fn();
const disablePlugin = jest.fn();
const activatePlugin = jest.fn();
const deactivatePlugin = jest.fn();
const updatePlugin = jest.fn();
const uninstallPlugin = jest.fn();
const updateConfig = jest.fn();
const checkHealth = jest.fn();
const restartPlugin = jest.fn();

jest.mock("@/lib/stores/plugin-store", () => ({
  usePluginStore: (
    selector: (state: {
      enablePlugin: typeof enablePlugin;
      disablePlugin: typeof disablePlugin;
      activatePlugin: typeof activatePlugin;
      deactivatePlugin: typeof deactivatePlugin;
      updatePlugin: typeof updatePlugin;
      uninstallPlugin: typeof uninstallPlugin;
      updateConfig: typeof updateConfig;
      checkHealth: typeof checkHealth;
      restartPlugin: typeof restartPlugin;
    }) => unknown,
  ) =>
    selector({
      enablePlugin,
      disablePlugin,
      activatePlugin,
      deactivatePlugin,
      updatePlugin,
      uninstallPlugin,
      updateConfig,
      checkHealth,
      restartPlugin,
    }),
}));

const rolePlugin: PluginRecord = {
  apiVersion: "plugin.agentforge.dev/v1",
  kind: "RolePlugin",
  metadata: {
    id: "frontend-role",
    name: "Frontend Role",
    version: "1.0.0",
    description: "Declarative role plugin",
  },
  spec: {
    runtime: "declarative",
  },
  permissions: {},
  source: { type: "local" },
  lifecycle_state: "disabled",
  restart_count: 0,
};

const activeToolPlugin: PluginRecord = {
  apiVersion: "plugin.agentforge.dev/v1",
  kind: "ToolPlugin",
  metadata: {
    id: "github-tool",
    name: "GitHub Tool",
    version: "1.0.0",
    description: "GitHub integration",
  },
  spec: {
    runtime: "mcp",
  },
  permissions: {
    network: {
      required: true,
      domains: ["api.github.com"],
    },
  },
  source: { type: "builtin" },
  lifecycle_state: "active",
  runtime_host: "ts-bridge",
  restart_count: 2,
};

const enabledToolPlugin: PluginRecord = {
  ...activeToolPlugin,
  lifecycle_state: "enabled",
};

const installedToolPlugin: PluginRecord = {
  ...activeToolPlugin,
  lifecycle_state: "installed",
};

const updatableActiveToolPlugin: PluginRecord = {
  ...activeToolPlugin,
  source: {
    type: "local",
    path: "/plugins/github-tool/manifest.yaml",
    release: {
      version: "1.0.0",
      availableVersion: "1.1.0",
    },
  },
};

describe("PluginCard", () => {
  beforeEach(() => {
    enablePlugin.mockReset();
    disablePlugin.mockReset();
    activatePlugin.mockReset();
    deactivatePlugin.mockReset();
    updatePlugin.mockReset();
    uninstallPlugin.mockReset();
    updateConfig.mockReset();
    checkHealth.mockReset();
    restartPlugin.mockReset();
  });

  it("shows enable button for non-executable disabled plugins", () => {
    render(<PluginCard plugin={rolePlugin} />);

    expect(screen.getByRole("button", { name: "Enable" })).toBeInTheDocument();
    expect(screen.queryByText("ts-bridge")).not.toBeInTheDocument();
    expect(screen.getByText("Not executable")).toBeInTheDocument();
  });

  it("exposes runtime actions for active executable plugins via dropdown menu", async () => {
    const user = userEvent.setup();

    render(<PluginCard plugin={activeToolPlugin} />);

    // Open dropdown menu
    await user.click(screen.getByRole("button", { name: "More actions" }));

    await user.click(screen.getByRole("menuitem", { name: /Restart/i }));
    expect(restartPlugin).toHaveBeenCalledWith("github-tool");

    // Reopen dropdown for next action
    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(screen.getByRole("menuitem", { name: /Health/i }));
    expect(checkHealth).toHaveBeenCalledWith("github-tool");

    expect(screen.getByText("ts-bridge")).toBeInTheDocument();
  });

  it("supports activation and disable as primary actions for enabled plugins", async () => {
    const user = userEvent.setup();

    render(<PluginCard plugin={enabledToolPlugin} selected />);

    // Activate is a primary button (visible outside dropdown)
    await user.click(screen.getByRole("button", { name: "Activate" }));
    expect(activatePlugin).toHaveBeenCalledWith("github-tool");
  });

  it("supports configure, uninstall via dropdown menu", async () => {
    const user = userEvent.setup();
    const onConfigure = jest.fn();

    render(<PluginCard plugin={enabledToolPlugin} onConfigure={onConfigure} />);

    // Open dropdown to configure
    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(screen.getByRole("menuitem", { name: /Configure/i }));
    expect(onConfigure).toHaveBeenCalledWith(enabledToolPlugin);

    // Uninstall via dropdown
    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(screen.getByRole("menuitem", { name: /Uninstall/i }));
    expect(uninstallPlugin).toHaveBeenCalledWith("github-tool");
  });

  it("selects the plugin when the card is clicked", async () => {
    const user = userEvent.setup();
    const onSelect = jest.fn();

    render(<PluginCard plugin={enabledToolPlugin} onSelect={onSelect} />);

    // Click the card itself (the outer button)
    await user.click(screen.getByText("GitHub Tool"));
    expect(onSelect).toHaveBeenCalledWith(enabledToolPlugin);
  });

  it("shows enable button for installed executable plugins", () => {
    render(<PluginCard plugin={installedToolPlugin} />);

    expect(screen.getByRole("button", { name: "Enable" })).toBeInTheDocument();
  });

  it("supports deactivate, update, and invoke flows for active plugins via dropdown", async () => {
    const user = userEvent.setup();
    const onInvoke = jest.fn();

    render(<PluginCard plugin={updatableActiveToolPlugin} onInvoke={onInvoke} />);

    // Update is a visible button (not in dropdown)
    await user.click(screen.getByRole("button", { name: "Update" }));
    expect(updatePlugin).toHaveBeenCalledWith(updatableActiveToolPlugin);

    // Deactivate via dropdown
    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(screen.getByRole("menuitem", { name: /Deactivate/i }));
    expect(deactivatePlugin).toHaveBeenCalledWith("github-tool");

    // Invoke via dropdown
    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(screen.getByRole("menuitem", { name: /Invoke/i }));
    expect(onInvoke).toHaveBeenCalledWith(updatableActiveToolPlugin);
  });
});
