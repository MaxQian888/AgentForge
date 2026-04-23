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

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "publish.title": "Publish Item",
      "publish.success": "Item published successfully",
      "publish.cancel": "Cancel",
      "publish.failed": "Failed to publish",
      "publish.typeLabel": "Type",
      "publish.licenseLabel": "License",
      "publish.nameLabel": "Name",
      "publish.slugLabel": "Slug",
      "publish.descriptionLabel": "Description",
      "publish.categoryLabel": "Category",
      "publish.repoLabel": "Repository URL",
      "publish.tagsLabel": "Tags (comma-separated)",
      "publish.tagsPlaceholder": "e.g. testing, automation, ci",
      "publish.publishing": "Publishing...",
      "publish.submitLabel": "Publish",
      "publish.typePlugin": "Plugin",
      "publish.typeSkill": "Skill",
      "publish.typeRole": "Role",
      "publish.typeWorkflow": "Workflow Template",
      "publish.licenseMIT": "MIT",
      "publish.licenseApache": "Apache 2.0",
      "publish.licenseGPL": "GPL 3.0",
      "publish.licenseProprietary": "Proprietary",
    };
    return map[key] ?? key;
  },
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
