import { fireEvent, render, screen } from "@testing-library/react";
import { MarketplaceItemCard } from "./marketplace-item-card";
import type {
  MarketplaceConsumptionRecord,
  MarketplaceItem,
} from "@/lib/stores/marketplace-store";

const mockItem: MarketplaceItem = {
  id: "test-id",
  type: "plugin",
  slug: "test-plugin",
  name: "Test Plugin",
  author_id: "author-1",
  author_name: "Test Author",
  description: "A test plugin for testing.",
  category: "testing",
  tags: [],
  license: "MIT",
  extra_metadata: {},
  download_count: 100,
  avg_rating: 4.5,
  rating_count: 10,
  is_verified: true,
  is_featured: false,
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
};

describe("MarketplaceItemCard", () => {
  it("renders item details and an install action by default", () => {
    render(<MarketplaceItemCard item={mockItem} />);

    expect(screen.getByText("Test Plugin")).toBeInTheDocument();
    expect(screen.getByText("plugin")).toBeInTheDocument();
    expect(screen.getByText("100")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Install" })).toBeInTheDocument();
  });

  it("renders manage state when consumption is installed and used", () => {
    const consumption: MarketplaceConsumptionRecord = {
      itemId: mockItem.id,
      itemType: "plugin",
      status: "installed",
      consumerSurface: "plugin-management-panel",
      installed: true,
      used: true,
    };

    render(<MarketplaceItemCard item={mockItem} consumption={consumption} />);

    expect(screen.getByRole("button", { name: "Manage" })).toBeInTheDocument();
  });

  it("renders blocked state and failure copy when install is unsupported", () => {
    const consumption: MarketplaceConsumptionRecord = {
      itemId: mockItem.id,
      itemType: "plugin",
      status: "blocked",
      consumerSurface: "plugin-management-panel",
      installed: false,
      used: false,
      failureReason: "Remote installation is unavailable",
    };

    render(<MarketplaceItemCard item={mockItem} consumption={consumption} />);

    expect(screen.getByRole("button", { name: "Blocked" })).toBeDisabled();
    expect(
      screen.getByText("Remote installation is unavailable"),
    ).toBeInTheDocument();
  });

  it("calls onInstall when the install button is pressed", () => {
    const onInstall = jest.fn();

    render(<MarketplaceItemCard item={mockItem} onInstall={onInstall} />);
    fireEvent.click(screen.getByRole("button", { name: "Install" }));

    expect(onInstall).toHaveBeenCalledWith(mockItem);
  });

  it("renders update badge and Update button when updateInfo.hasUpdate is true", () => {
    const consumption: MarketplaceConsumptionRecord = {
      itemId: mockItem.id,
      itemType: "plugin",
      status: "installed",
      consumerSurface: "plugin-management-panel",
      installed: true,
      used: true,
    };

    render(
      <MarketplaceItemCard
        item={mockItem}
        consumption={consumption}
        updateInfo={{
          itemId: mockItem.id,
          itemType: "plugin",
          installedVersion: "1.0.0",
          latestVersion: "2.0.0",
          hasUpdate: true,
        }}
      />,
    );

    expect(screen.getByText("Update: v2.0.0")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Update" })).toBeInTheDocument();
  });

  it("calls onInstall when Update button is clicked", () => {
    const onInstall = jest.fn();
    const consumption: MarketplaceConsumptionRecord = {
      itemId: mockItem.id,
      itemType: "plugin",
      status: "installed",
      consumerSurface: "plugin-management-panel",
      installed: true,
      used: true,
    };

    render(
      <MarketplaceItemCard
        item={mockItem}
        consumption={consumption}
        updateInfo={{
          itemId: mockItem.id,
          itemType: "plugin",
          installedVersion: "1.0.0",
          latestVersion: "2.0.0",
          hasUpdate: true,
        }}
        onInstall={onInstall}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Update" }));

    expect(onInstall).toHaveBeenCalledWith(mockItem);
  });

  it("renders clickable tags and calls onTagClick", () => {
    const itemWithTags = { ...mockItem, tags: ["react", "typescript"] };
    const onTagClick = jest.fn();

    render(<MarketplaceItemCard item={itemWithTags} onTagClick={onTagClick} />);

    const tagButton = screen.getByText("react");
    expect(tagButton).toBeInTheDocument();
    fireEvent.click(tagButton);

    expect(onTagClick).toHaveBeenCalledWith("react");
  });
});
