import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginInstallDialog } from "./plugin-install-dialog";

const installLocal = jest.fn();
const installFromCatalog = jest.fn();
const fetchPlugins = jest.fn();
const searchCatalog = jest.fn();
const selectFiles = jest.fn();
const capabilityState = {
  isDesktop: true,
  selectFiles,
};
const pluginStoreState: Record<string, unknown> = {
  installLocal,
  installFromCatalog,
  fetchPlugins,
  searchCatalog,
  catalogResults: [],
  catalogQuery: "",
  loading: false,
};

jest.mock("@/lib/stores/plugin-store", () => ({
  usePluginStore: (selector: (state: typeof pluginStoreState) => unknown) =>
    selector(pluginStoreState),
}));

jest.mock("@/hooks/use-platform-capability", () => ({
  usePlatformCapability: () => capabilityState,
}));

jest.mock("./plugin-catalog-search", () => ({
  PluginCatalogSearch: ({
    onSelect,
  }: {
    onSelect: (entry: { id: string; name: string; version: string }) => void;
  }) => (
    <div data-testid="catalog-search">
      <button
        onClick={() =>
          onSelect({
            id: "catalog-tool",
            name: "Catalog Tool",
            version: "1.0.0",
          })
        }
      >
        Pick Catalog Tool
      </button>
    </div>
  ),
}));

jest.mock("./plugin-install-confirmation", () => ({
  PluginInstallConfirmation: ({
    onConfirm,
    onBack,
    sourceLabel,
  }: {
    onConfirm: () => void;
    onBack: () => void;
    sourceLabel: string;
  }) => (
    <div data-testid="install-confirmation">
      <span>{sourceLabel}</span>
      <button onClick={onBack}>Back</button>
      <button onClick={onConfirm}>Confirm Install</button>
    </div>
  ),
}));

describe("PluginInstallDialog", () => {
  beforeEach(() => {
    installLocal.mockReset();
    installLocal.mockResolvedValue(undefined);
    installFromCatalog.mockReset();
    fetchPlugins.mockReset();
    fetchPlugins.mockResolvedValue(undefined);
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

  it("submits a trimmed local path through the multi-step flow", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();

    render(<PluginInstallDialog open onOpenChange={onOpenChange} />);

    // Step 1: Enter local path
    await user.type(screen.getByLabelText("Plugin Path"), "C:\\plugins\\local");
    await user.click(screen.getByRole("button", { name: "Next" }));

    // Step 2: Confirm
    expect(screen.getByTestId("install-confirmation")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Confirm Install" }));

    expect(installLocal).toHaveBeenCalledWith("C:\\plugins\\local");
  });

  it("shows the non-desktop hint on the local tab", () => {
    capabilityState.isDesktop = false;

    render(<PluginInstallDialog open onOpenChange={jest.fn()} />);

    expect(
      screen.getByText(
        /Native path browsing is only available in the desktop shell/i,
      ),
    ).toBeInTheDocument();
  });

  it("only exposes currently supported install source tabs", () => {
    render(<PluginInstallDialog open onOpenChange={jest.fn()} />);

    expect(screen.getByRole("tab", { name: "Local" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Catalog" })).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "Git" })).not.toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "npm" })).not.toBeInTheDocument();
  });

  it("installs catalog entries through the confirmation step", async () => {
    const user = userEvent.setup();

    render(<PluginInstallDialog open onOpenChange={jest.fn()} />);

    await user.click(screen.getByRole("tab", { name: "Catalog" }));
    await user.click(screen.getByRole("button", { name: "Pick Catalog Tool" }));
    await user.click(screen.getByRole("button", { name: "Next" }));
    await user.click(screen.getByRole("button", { name: "Confirm Install" }));

    expect(installFromCatalog).toHaveBeenCalledWith("catalog-tool");
  });
});
