import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginInstallConfirmation } from "./plugin-install-confirmation";

describe("PluginInstallConfirmation", () => {
  it("renders requested permissions and unsigned install warnings", async () => {
    const user = userEvent.setup();
    const onConfirm = jest.fn();
    const onBack = jest.fn();

    render(
      <PluginInstallConfirmation
        sourceType="catalog"
        sourceLabel="catalog://repo-search"
        permissions={{
          network: {
            required: true,
            domains: ["api.example.com"],
          },
          filesystem: {
            required: true,
            allowed_paths: ["/tmp/plugins"],
          },
        }}
        unsigned
        onConfirm={onConfirm}
        onBack={onBack}
      />,
    );

    expect(screen.getByText("catalog://repo-search")).toBeInTheDocument();
    expect(screen.getByText("Requested Permissions")).toBeInTheDocument();
    expect(screen.getByText("Network access")).toBeInTheDocument();
    expect(screen.getByText("api.example.com")).toBeInTheDocument();
    expect(screen.getByText("Filesystem access")).toBeInTheDocument();
    expect(screen.getByText("/tmp/plugins")).toBeInTheDocument();
    expect(
      screen.getByText(/This plugin has no cryptographic signature/i),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Back" }));
    await user.click(screen.getByRole("button", { name: "Install Anyway" }));

    expect(onBack).toHaveBeenCalled();
    expect(onConfirm).toHaveBeenCalled();
  });

  it("uses the default confirmation copy for trusted installs", () => {
    render(
      <PluginInstallConfirmation
        sourceType="local"
        sourceLabel="C:\\plugins\\repo-search"
        onConfirm={jest.fn()}
        onBack={jest.fn()}
      />,
    );

    expect(screen.queryByText("Requested Permissions")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Confirm Install" }),
    ).toBeInTheDocument();
  });
});
