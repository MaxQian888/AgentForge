import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginCard } from "./plugin-card";
import type { PluginRecord } from "@/lib/stores/plugin-store";

const enablePlugin = jest.fn();
const disablePlugin = jest.fn();
const activatePlugin = jest.fn();
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

describe("PluginCard", () => {
  beforeEach(() => {
    enablePlugin.mockReset();
    disablePlugin.mockReset();
    activatePlugin.mockReset();
    uninstallPlugin.mockReset();
    updateConfig.mockReset();
    checkHealth.mockReset();
    restartPlugin.mockReset();
  });

  it("explains why runtime actions are unavailable for non-executable plugins", () => {
    render(<PluginCard plugin={rolePlugin} />);

    expect(
      screen.getByText(/does not use an executable runtime host/i),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Activate" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Restart" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Health" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Enable" })).toBeInTheDocument();
  });

  it("exposes runtime actions for active executable plugins", async () => {
    const user = userEvent.setup();

    render(<PluginCard plugin={activeToolPlugin} />);

    await user.click(screen.getByRole("button", { name: "Restart" }));
    await user.click(screen.getByRole("button", { name: "Health" }));

    expect(checkHealth).toHaveBeenCalledWith("github-tool");
    expect(restartPlugin).toHaveBeenCalledWith("github-tool");
    expect(screen.getByText("Host: ts-bridge")).toBeInTheDocument();
  });

  it("supports activation, configuration, selection, disable, and uninstall flows", async () => {
    const user = userEvent.setup();
    const onConfigure = jest.fn();
    const onSelect = jest.fn();

    render(
      <PluginCard
        plugin={enabledToolPlugin}
        onConfigure={onConfigure}
        onSelect={onSelect}
        selected
      />,
    );

    await user.click(screen.getByRole("button", { name: "Activate" }));
    await user.click(screen.getByRole("button", { name: "Disable" }));
    await user.click(screen.getByRole("button", { name: "Configure" }));
    await user.click(screen.getByRole("button", { name: "Details" }));
    await user.click(screen.getByRole("button", { name: "Uninstall" }));

    expect(activatePlugin).toHaveBeenCalledWith("github-tool");
    expect(disablePlugin).toHaveBeenCalledWith("github-tool");
    expect(uninstallPlugin).toHaveBeenCalledWith("github-tool");
    expect(onConfigure).toHaveBeenCalledWith(enabledToolPlugin);
    expect(onSelect).toHaveBeenCalledWith(enabledToolPlugin);
  });

  it("explains installed executable plugins before they are enabled", () => {
    render(<PluginCard plugin={installedToolPlugin} />);

    expect(screen.getByText("Installed but not enabled yet.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Enable" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Activate" })).not.toBeInTheDocument();
  });
});
