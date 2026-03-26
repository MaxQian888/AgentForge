import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginInvokeDialog } from "./plugin-invoke-dialog";
import type { PluginRecord } from "@/lib/stores/plugin-store";

const invokePlugin = jest.fn();

const storeState = {
  invokePlugin,
};

jest.mock("@/lib/stores/plugin-store", () => ({
  usePluginStore: (
    selector: (state: typeof storeState) => unknown,
  ) => selector(storeState),
}));

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
  },
  lifecycle_state: "active",
  restart_count: 0,
};

describe("PluginInvokeDialog", () => {
  beforeEach(() => {
    invokePlugin.mockReset().mockResolvedValue({ ok: true });
  });

  it("renders a generic title when no plugin is selected", () => {
    render(<PluginInvokeDialog plugin={null} open={true} onOpenChange={jest.fn()} />);

    expect(screen.getByText("Invoke Plugin")).toBeInTheDocument();
    expect(screen.queryByLabelText("Operation")).not.toBeInTheDocument();
  });

  it("validates the payload and shows invocation results", async () => {
    const user = userEvent.setup();

    render(<PluginInvokeDialog plugin={plugin} open={true} onOpenChange={jest.fn()} />);

    await user.click(screen.getByRole("button", { name: "Submit" }));
    expect(screen.getByText("Operation name is required")).toBeInTheDocument();

    await user.type(screen.getByLabelText("Operation"), "run");
    fireEvent.change(screen.getByLabelText("Payload (JSON)"), {
      target: { value: "{bad" },
    });
    await user.click(screen.getByRole("button", { name: "Submit" }));
    expect(screen.getByText("Invalid JSON payload")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Payload (JSON)"), {
      target: { value: '{"mode":"fast"}' },
    });
    await user.click(screen.getByRole("button", { name: "Submit" }));

    await waitFor(() => {
      expect(invokePlugin).toHaveBeenCalledWith("repo-search", "run", {
        mode: "fast",
      });
    });
    expect(screen.getByText(/"ok": true/)).toBeInTheDocument();
  });
});
