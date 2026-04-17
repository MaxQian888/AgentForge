import { render, screen } from "@testing-library/react";
import type { AssetVersion } from "@/lib/stores/knowledge-store";
import { VersionViewer } from "./version-viewer";

function makeVersion(overrides: Partial<AssetVersion> = {}): AssetVersion {
  return {
    id: "version-1",
    assetId: "page-1",
    versionNumber: 3,
    name: "Draft review",
    kindSnapshot: "wiki_page",
    contentJson: '[{"type":"paragraph","content":"Hello"}]',
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
