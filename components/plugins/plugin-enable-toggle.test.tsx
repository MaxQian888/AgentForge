jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "pluginCard.enable": "Enable",
      "pluginCard.disable": "Disable",
      "pluginCard.hintActivating": "Plugin is activating",
    };
    return map[key] ?? key;
  },
}));

const enablePlugin = jest.fn();
const disablePlugin = jest.fn();

jest.mock("@/lib/stores/plugin-store", () => {
  const actual = jest.requireActual("@/lib/stores/plugin-store");
  return {
    ...actual,
    usePluginStore: (
      selector: (state: {
        enablePlugin: typeof enablePlugin;
        disablePlugin: typeof disablePlugin;
      }) => unknown,
    ) => selector({ enablePlugin, disablePlugin }),
  };
});

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginEnableToggle } from "./plugin-enable-toggle";
import type { PluginRecord } from "@/lib/stores/plugin-store";

function makePlugin(overrides: Partial<PluginRecord> = {}): PluginRecord {
  return {
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
    },
    lifecycle_state: "active",
    runtime_host: "ts-bridge",
    restart_count: 0,
    ...overrides,
  };
}

describe("PluginEnableToggle", () => {
  beforeEach(() => {
    enablePlugin.mockReset();
    disablePlugin.mockReset();
  });

  it("renders checked switch for active plugins and calls disable on toggle off", async () => {
    const user = userEvent.setup();
    render(<PluginEnableToggle plugin={makePlugin()} />);

    const toggle = screen.getByRole("switch");
    expect(toggle).toHaveAttribute("data-state", "checked");

    await user.click(toggle);
    expect(disablePlugin).toHaveBeenCalledWith("repo-search");
    expect(enablePlugin).not.toHaveBeenCalled();
  });

  it("renders unchecked switch for disabled plugins and calls enable on toggle on", async () => {
    const user = userEvent.setup();
    render(
      <PluginEnableToggle
        plugin={makePlugin({ lifecycle_state: "disabled" })}
      />,
    );

    const toggle = screen.getByRole("switch");
    expect(toggle).toHaveAttribute("data-state", "unchecked");

    await user.click(toggle);
    expect(enablePlugin).toHaveBeenCalledWith("repo-search");
    expect(disablePlugin).not.toHaveBeenCalled();
  });

  it("locks the toggle while the plugin is activating", () => {
    render(
      <PluginEnableToggle
        plugin={makePlugin({ lifecycle_state: "activating" })}
      />,
    );

    const toggle = screen.getByRole("switch");
    expect(toggle).toBeDisabled();
  });
});
