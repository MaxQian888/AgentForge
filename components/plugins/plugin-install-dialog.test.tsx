import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginInstallDialog } from "./plugin-install-dialog";

const installLocal = jest.fn();
const selectFiles = jest.fn();

jest.mock("@/lib/stores/plugin-store", () => ({
  usePluginStore: (
    selector: (state: {
      installLocal: typeof installLocal;
      loading: boolean;
    }) => unknown,
  ) =>
    selector({
      installLocal,
      loading: false,
    }),
}));

jest.mock("@/hooks/use-platform-capability", () => ({
  usePlatformCapability: () => ({
    isDesktop: true,
    selectFiles,
  }),
}));

describe("PluginInstallDialog", () => {
  beforeEach(() => {
    installLocal.mockReset();
    installLocal.mockResolvedValue(undefined);
    selectFiles.mockReset();
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
});
