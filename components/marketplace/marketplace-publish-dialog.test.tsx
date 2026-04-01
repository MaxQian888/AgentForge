import { fireEvent, render, screen, waitFor } from "@testing-library/react";

const publishItem = jest.fn().mockResolvedValue({ id: "new-item" });

const mockState = {
  publishItem,
  fetchItems: jest.fn(),
  fetchFeatured: jest.fn(),
};

jest.mock("@/lib/stores/marketplace-store", () => ({
  useMarketplaceStore: (selector: (state: Record<string, unknown>) => unknown) =>
    typeof selector === "function" ? selector(mockState) : mockState,
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

import { MarketplacePublishDialog } from "./marketplace-publish-dialog";

describe("MarketplacePublishDialog", () => {
  beforeEach(() => {
    publishItem.mockClear();
  });

  it("renders the dialog when open", () => {
    render(<MarketplacePublishDialog open={true} onClose={jest.fn()} />);
    expect(screen.getByText("Publish Item")).toBeInTheDocument();
  });

  it("auto-generates slug from name", () => {
    render(<MarketplacePublishDialog open={true} onClose={jest.fn()} />);
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "My Plugin" },
    });
    expect(screen.getByLabelText("Slug")).toHaveValue("my-plugin");
  });

  it("calls publishItem on form submit", async () => {
    const onClose = jest.fn();
    render(<MarketplacePublishDialog open={true} onClose={onClose} />);

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Test Plugin" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Publish" }));

    await waitFor(() => {
      expect(publishItem).toHaveBeenCalledTimes(1);
    });
  });

  it("calls onClose when Cancel is clicked", () => {
    const onClose = jest.fn();
    render(<MarketplacePublishDialog open={true} onClose={onClose} />);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalled();
  });
});
