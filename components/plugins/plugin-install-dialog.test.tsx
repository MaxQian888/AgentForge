import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginInstallDialog } from "./plugin-install-dialog";

const installLocal = jest.fn();
const selectFiles = jest.fn();
const capabilityState = {
  isDesktop: true,
  selectFiles,
};
const pluginStoreState = {
  installLocal,
  loading: false,
};

jest.mock("@/lib/stores/plugin-store", () => ({
  usePluginStore: (
    selector: (state: {
      installLocal: typeof installLocal;
      loading: boolean;
    }) => unknown,
  ) =>
    selector(pluginStoreState),
}));

jest.mock("@/hooks/use-platform-capability", () => ({
  usePlatformCapability: () => capabilityState,
}));

describe("PluginInstallDialog", () => {
  beforeEach(() => {
    installLocal.mockReset();
    installLocal.mockResolvedValue(undefined);
    selectFiles.mockReset();
    capabilityState.isDesktop = true;
    pluginStoreState.loading = false;
  });

  it("fills the path field from the desktop file picker", async () => {
    const user = userEvent.setup();
    selectFiles.mockResolvedValue({
      ok: true,
      mode: "desktop",
      paths: ["C:\\plugins\\github-tool"],
    });

    render(<PluginInstallDialog open onOpenChange={jest.fn()} />);

    await user.click(screen.getByRole("button", { name: "Browse" }));

    expect(selectFiles).toHaveBeenCalledWith({
      directory: true,
      multiple: false,
      title: "Select a local plugin directory",
    });
    expect(screen.getByLabelText("Plugin Path")).toHaveValue(
      "C:\\plugins\\github-tool",
    );
  });

  it("submits a trimmed local path and closes the dialog", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();

    render(<PluginInstallDialog open onOpenChange={onOpenChange} />);

    await user.type(screen.getByLabelText("Plugin Path"), "  C:\\plugins\\local  ");
    await user.click(screen.getByRole("button", { name: "Install" }));

    expect(installLocal).toHaveBeenCalledWith("C:\\plugins\\local");
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("shows the non-desktop hint and ignores failed browse results", async () => {
    const user = userEvent.setup();
    capabilityState.isDesktop = false;
    selectFiles.mockResolvedValue({ ok: false, mode: "desktop", paths: [] });

    render(<PluginInstallDialog open onOpenChange={jest.fn()} />);

    expect(
      screen.getByText(/Native path browsing is only available in the desktop shell/i),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Browse" }));
    expect(screen.getByLabelText("Plugin Path")).toHaveValue("");
  });
});
