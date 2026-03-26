import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { DocsVersion } from "@/lib/stores/docs-store";
import { VersionHistoryPanel } from "./version-history-panel";

function makeVersion(overrides: Partial<DocsVersion> = {}): DocsVersion {
  return {
    id: "version-1",
    pageId: "page-1",
    versionNumber: 3,
    name: "Draft review",
    content: "[]",
    createdBy: "user-1",
    createdAt: "2026-03-26T12:00:00.000Z",
    ...overrides,
  };
}

describe("VersionHistoryPanel", () => {
  const confirmSpy = jest.spyOn(window, "confirm");

  afterEach(() => {
    confirmSpy.mockReset();
  });

  afterAll(() => {
    confirmSpy.mockRestore();
  });

  it("selects, restores, and shares versions", async () => {
    const user = userEvent.setup();
    const onSelect = jest.fn();
    const onRestore = jest.fn();
    const onShare = jest.fn();

    confirmSpy.mockReturnValue(true);

    render(
      <VersionHistoryPanel
        versions={[makeVersion(), makeVersion({ id: "version-2", versionNumber: 4 })]}
        selectedVersionId="version-2"
        onSelect={onSelect}
        onRestore={onRestore}
        onShare={onShare}
      />,
    );

    expect(screen.getByText("Version History")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /v4 · Draft review/i }).parentElement).toHaveClass(
      "border-primary",
    );

    await user.click(screen.getByRole("button", { name: /v3 · Draft review/i }));
    await user.click(screen.getAllByRole("button", { name: "Restore" })[0]);
    await user.click(screen.getAllByRole("button", { name: "Share" })[0]);

    expect(onSelect).toHaveBeenCalledWith("version-1");
    expect(onRestore).toHaveBeenCalledWith("version-1");
    expect(onShare).toHaveBeenCalledWith("version-1");
  });

  it("does not restore without confirmation and shows an empty state", async () => {
    const user = userEvent.setup();
    const onRestore = jest.fn();

    confirmSpy.mockReturnValue(false);

    const { rerender } = render(
      <VersionHistoryPanel versions={[makeVersion()]} onRestore={onRestore} />,
    );

    await user.click(screen.getByRole("button", { name: "Restore" }));
    expect(onRestore).not.toHaveBeenCalled();

    rerender(<VersionHistoryPanel versions={[]} />);
    expect(screen.getByText("No saved versions yet.")).toBeInTheDocument();
  });

  it("disables restore actions in readonly mode", () => {
    render(<VersionHistoryPanel versions={[makeVersion()]} readonly />);

    expect(screen.getByRole("button", { name: "Restore" })).toBeDisabled();
  });
});
