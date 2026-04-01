import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginConfigForm } from "./plugin-config-form";
import type { PluginRecord } from "@/lib/stores/plugin-store";

const fallbackPlugin: PluginRecord = {
  apiVersion: "plugin.agentforge.dev/v1",
  kind: "ToolPlugin",
  metadata: {
    id: "review-tools",
    name: "Review Tools",
    version: "1.0.0",
  },
  spec: {
    runtime: "mcp",
    config: {
      enabled: true,
    },
  },
  permissions: {},
  source: {
    type: "local",
  },
  lifecycle_state: "enabled",
  restart_count: 0,
};

const schemaPlugin: PluginRecord = {
  ...fallbackPlugin,
  metadata: {
    ...fallbackPlugin.metadata,
    id: "workflow-tools",
    name: "Workflow Tools",
  },
  spec: {
    runtime: "mcp",
    config: {
      enabled: false,
      retries: 1,
      mode: "lenient",
      apiToken: "initial-token",
    },
    extra: {
      configSchema: {
        type: "object",
        properties: {
          enabled: {
            type: "boolean",
            description: "Enable this plugin",
          },
          retries: {
            type: "integer",
            description: "Retry count",
            default: 3,
          },
          mode: {
            type: "string",
            enum: ["strict", "lenient"],
            default: "strict",
          },
          apiToken: {
            type: "string",
            description: "API token used for outbound calls",
            default: "secret-default",
          },
        },
      },
    },
  },
};

describe("PluginConfigForm", () => {
  it("validates JSON fallback mode before saving", async () => {
    const user = userEvent.setup();
    const onSave = jest.fn();
    const onCancel = jest.fn();

    render(
      <PluginConfigForm plugin={fallbackPlugin} onSave={onSave} onCancel={onCancel} />,
    );

    const textarea = screen.getByLabelText("Configuration (JSON)");
    fireEvent.change(textarea, { target: { value: "{bad" } });
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(screen.getByText("Invalid JSON")).toBeInTheDocument();
    expect(onSave).not.toHaveBeenCalled();

    fireEvent.change(textarea, { target: { value: '{"mode":"strict"}' } });
    await user.click(screen.getByRole("button", { name: "Save" }));
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onSave).toHaveBeenCalledWith({ mode: "strict" });
    expect(onCancel).toHaveBeenCalled();
  });

  it("renders schema-driven fields, resets defaults, and saves typed values", async () => {
    const user = userEvent.setup();
    const onSave = jest.fn();

    render(
      <PluginConfigForm plugin={schemaPlugin} onSave={onSave} onCancel={jest.fn()} />,
    );

    const enabled = screen.getByLabelText("Enabled");
    const retries = screen.getByLabelText("Retries");
    const apiToken = screen.getByLabelText("Api Token");

    expect(apiToken).toHaveAttribute("type", "password");
    expect(screen.getByText("Default: 3")).toBeInTheDocument();
    expect(screen.getByText('Default: "secret-default"')).toBeInTheDocument();

    await user.click(enabled);
    await user.clear(retries);
    await user.type(retries, "5");
    await user.click(screen.getByRole("combobox", { name: "Mode" }));
    await user.click(screen.getByRole("option", { name: "strict" }));
    await user.clear(apiToken);
    await user.type(apiToken, "updated-token");
    await user.click(screen.getAllByRole("button", { name: "Reset to default" })[0]);
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(onSave).toHaveBeenCalledWith({
      enabled: true,
      retries: 3,
      mode: "strict",
      apiToken: "updated-token",
    });
  });
});
