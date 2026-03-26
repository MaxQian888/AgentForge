import { render, screen } from "@testing-library/react";
import type { DocsVersion } from "@/lib/stores/docs-store";
import { VersionViewer } from "./version-viewer";

function makeVersion(overrides: Partial<DocsVersion> = {}): DocsVersion {
  return {
    id: "version-1",
    pageId: "page-1",
    versionNumber: 3,
    name: "Draft review",
    content: '[{"type":"paragraph","content":"Hello"}]',
    createdBy: "user-1",
    createdAt: "2026-03-26T12:00:00.000Z",
    ...overrides,
  };
}

describe("VersionViewer", () => {
  it("renders the selected version preview", () => {
    render(<VersionViewer version={makeVersion()} />);

    expect(screen.getByText("Draft review")).toBeInTheDocument();
    expect(screen.getByText(/v3/)).toBeInTheDocument();
    expect(screen.getByText('[{"type":"paragraph","content":"Hello"}]')).toBeInTheDocument();
  });

  it("shows a placeholder when no version is selected", () => {
    render(<VersionViewer version={null} />);

    expect(screen.getByText("Select a version to preview it.")).toBeInTheDocument();
  });
});
