jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const messages: Record<string, string> = {
      patchTitle: "Suggested Patch",
      patchUnavailable: "No patch available.",
    };
    return messages[key] ?? key;
  },
}));

jest.mock("react-diff-viewer-continued", () => {
  return {
    __esModule: true,
    default: ({ oldValue, newValue }: { oldValue: string; newValue: string }) => (
      <div data-testid="diff-viewer">
        <pre>{oldValue}</pre>
        <pre>{newValue}</pre>
      </div>
    ),
    DiffMethod: { LINES: "lines" },
  };
});

import { render, screen } from "@testing-library/react";
import { FindingPatchModal } from "./finding-patch-modal";

describe("FindingPatchModal", () => {
  it("renders patch text inside diff viewer", () => {
    const patch = "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b";
    render(<FindingPatchModal patch={patch} open onClose={() => {}} />);

    const viewer = screen.getByTestId("diff-viewer");
    expect(viewer).toBeInTheDocument();
    expect(viewer.textContent).toContain("a");
    expect(viewer.textContent).toContain("b");
  });

  it("shows empty state when patch is null", () => {
    render(<FindingPatchModal patch={null} open onClose={() => {}} />);

    expect(screen.getByText("No patch available.")).toBeInTheDocument();
  });
});
