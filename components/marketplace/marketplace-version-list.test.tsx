import { render, screen } from "@testing-library/react";
import { MarketplaceVersionList } from "./marketplace-version-list";

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "versions.noVersions": "No versions published yet.",
      "versions.latest": "latest",
      "versions.yanked": "yanked",
      "versions.download": "Download",
      "versions.yank": "Yank",
    };
    return map[key] ?? key;
  },
}));

jest.mock("@/lib/stores/marketplace-store", () => ({
  useMarketplaceStore: jest.fn(() => ({
    fetchItemVersions: jest.fn(),
    selectedItemVersions: [
      {
        id: "version-1",
        item_id: "item-1",
        version: "1.0.0",
        changelog: "",
        artifact_size_bytes: 2048,
        artifact_digest: "sha256:test",
        is_latest: true,
        is_yanked: false,
        created_at: "2024-01-01T00:00:00Z",
      },
    ],
  })),
}));

describe("MarketplaceVersionList", () => {
  it("uses the standalone marketplace default URL for download links", () => {
    render(<MarketplaceVersionList itemId="item-1" />);

    expect(screen.getByRole("link", { name: /download/i })).toHaveAttribute(
      "href",
      "http://localhost:7781/api/v1/items/item-1/versions/1.0.0/download",
    );
  });
});
