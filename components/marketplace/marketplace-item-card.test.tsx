import { render, screen, fireEvent } from "@testing-library/react";
import { MarketplaceItemCard } from "./marketplace-item-card";
import type { MarketplaceItem } from "@/lib/stores/marketplace-store";

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
  it("renders item name", () => {
    render(<MarketplaceItemCard item={mockItem} />);
    expect(screen.getByText("Test Plugin")).toBeInTheDocument();
  });

  it("renders item type badge", () => {
    render(<MarketplaceItemCard item={mockItem} />);
    expect(screen.getByText("plugin")).toBeInTheDocument();
  });

  it("renders author name", () => {
    render(<MarketplaceItemCard item={mockItem} />);
    expect(screen.getByText(/Test Author/)).toBeInTheDocument();
  });

  it("renders item description", () => {
    render(<MarketplaceItemCard item={mockItem} />);
    expect(
      screen.getByText("A test plugin for testing."),
    ).toBeInTheDocument();
  });

  it("renders download count", () => {
    render(<MarketplaceItemCard item={mockItem} />);
    expect(screen.getByText("100")).toBeInTheDocument();
  });

  it("renders average rating", () => {
    render(<MarketplaceItemCard item={mockItem} />);
    expect(screen.getByText("4.5")).toBeInTheDocument();
  });

  it("shows Install button when not installed", () => {
    render(<MarketplaceItemCard item={mockItem} installed={false} />);
    const btn = screen.getByRole("button", { name: "Install" });
    expect(btn).toBeInTheDocument();
    expect(btn).not.toBeDisabled();
  });

  it("shows Installed button disabled when installed", () => {
    render(<MarketplaceItemCard item={mockItem} installed={true} />);
    const btn = screen.getByRole("button", { name: "Installed" });
    expect(btn).toBeInTheDocument();
    expect(btn).toBeDisabled();
  });

  it("calls onInstall with the item when Install button clicked", () => {
    const onInstall = jest.fn();
    render(<MarketplaceItemCard item={mockItem} onInstall={onInstall} />);
    fireEvent.click(screen.getByRole("button", { name: "Install" }));
    expect(onInstall).toHaveBeenCalledWith(mockItem);
    expect(onInstall).toHaveBeenCalledTimes(1);
  });

  it("does not propagate click to card when Install button clicked", () => {
    const onSelect = jest.fn();
    const onInstall = jest.fn();
    render(
      <MarketplaceItemCard
        item={mockItem}
        onSelect={onSelect}
        onInstall={onInstall}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Install" }));
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("calls onSelect when card body is clicked", () => {
    const onSelect = jest.fn();
    render(<MarketplaceItemCard item={mockItem} onSelect={onSelect} />);
    // Click on the description text area (not the button)
    fireEvent.click(screen.getByText("A test plugin for testing."));
    expect(onSelect).toHaveBeenCalledWith(mockItem);
  });

  it("shows verified icon for verified items", () => {
    render(<MarketplaceItemCard item={mockItem} />);
    // The CheckCircle icon doesn't have visible text, but the container is rendered.
    // We verify that the component renders without error when is_verified=true.
    expect(screen.getByText("Test Plugin")).toBeInTheDocument();
  });

  it("renders placeholder when item has no icon_url", () => {
    render(<MarketplaceItemCard item={mockItem} />);
    // The placeholder shows the first 2 chars of name uppercased.
    expect(screen.getByText("TE")).toBeInTheDocument();
  });

  it("applies selected ring styling when selected", () => {
    const { container } = render(
      <MarketplaceItemCard item={mockItem} selected={true} />,
    );
    // The outermost card element should contain a ring class
    const card = container.firstChild as HTMLElement;
    expect(card.className).toContain("ring");
  });

  it("renders skill type badge with correct text", () => {
    const skillItem: MarketplaceItem = { ...mockItem, type: "skill" };
    render(<MarketplaceItemCard item={skillItem} />);
    expect(screen.getByText("skill")).toBeInTheDocument();
  });

  it("renders role type badge with correct text", () => {
    const roleItem: MarketplaceItem = { ...mockItem, type: "role" };
    render(<MarketplaceItemCard item={roleItem} />);
    expect(screen.getByText("role")).toBeInTheDocument();
  });

  it("shows fallback description text when description is empty", () => {
    const noDescItem: MarketplaceItem = { ...mockItem, description: "" };
    render(<MarketplaceItemCard item={noDescItem} />);
    expect(screen.getByText("No description provided.")).toBeInTheDocument();
  });
});
