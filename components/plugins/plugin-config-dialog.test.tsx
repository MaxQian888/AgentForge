const updateConfig = jest.fn().mockResolvedValue(undefined);

jest.mock("@/lib/stores/plugin-store", () => ({
  usePluginStore: (selector: (state: { updateConfig: typeof updateConfig }) => unknown) =>
    selector({ updateConfig }),
}));

import userEvent from "@testing-library/user-event";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { PluginConfigDialog } from "./plugin-config-dialog";
import type { PluginRecord } from "@/lib/stores/plugin-store";

const plugin: PluginRecord = {
  apiVersion: "agentforge/v1",
  kind: "ToolPlugin",
  metadata: {
    id: "plugin-1",
    name: "Review Tools",
    version: "1.0.0",
    description: "Adds review utilities.",
  },
  spec: {
    runtime: "mcp",
    config: { enabled: true },
  },
  permissions: {},
  source: { type: "local" },
  lifecycle_state: "enabled",
  restart_count: 0,
};

describe("PluginConfigDialog", () => {
  beforeEach(() => {
    updateConfig.mockClear();
  });

  it("validates JSON before saving and closes after a successful update", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();

    render(
      <PluginConfigDialog plugin={plugin} open={true} onOpenChange={onOpenChange} />,
    );

    // The form should render in JSON fallback mode (no configSchema)
    const textarea = screen.getByLabelText("Configuration (JSON)");
    expect(textarea).toHaveValue('{\n  "enabled": true\n}');

    await user.clear(textarea);
    fireEvent.change(textarea, { target: { value: "{bad" } });
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByText("Invalid JSON")).toBeInTheDocument();
    expect(updateConfig).not.toHaveBeenCalled();

    await user.clear(textarea);
    fireEvent.change(textarea, { target: { value: '{"mode":"strict"}' } });
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(updateConfig).toHaveBeenCalledWith("plugin-1", { mode: "strict" }),
    );
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("renders a generic title when no plugin is selected", () => {
    render(<PluginConfigDialog plugin={null} open={true} onOpenChange={jest.fn()} />);

    expect(screen.getByText("Configure Plugin")).toBeInTheDocument();
    expect(screen.queryByLabelText("Configuration (JSON)")).not.toBeInTheDocument();
  });
});
