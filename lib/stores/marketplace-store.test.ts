const post = jest.fn();
const get = jest.fn();
const del = jest.fn();

jest.mock("@/lib/api-client", () => {
  const actual = jest.requireActual("@/lib/api-client");
  return {
    ...actual,
    createApiClient: jest.fn(() => ({
      post,
      get,
      delete: del,
    })),
  };
});

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({
      accessToken: "test-token",
    })),
  },
}));

import {
  useMarketplaceStore,
  type MarketplaceItem,
  type MarketplaceFilters,
} from "./marketplace-store";

function createMarketplaceItem(
  id: string,
  overrides: Partial<MarketplaceItem> = {},
): MarketplaceItem {
  return {
    id,
    type: "plugin",
    slug: `test-plugin-${id}`,
    name: `Test Plugin ${id}`,
    author_id: "author-1",
    author_name: "Test Author",
    description: "A test plugin",
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
    ...overrides,
  };
}

const defaultFilters: MarketplaceFilters = {
  type: "all",
  category: "",
  sort: "downloads",
  page: 1,
  query: "",
};

describe("marketplace-store", () => {
  beforeEach(() => {
    post.mockReset();
    get.mockReset();
    del.mockReset();

    useMarketplaceStore.setState({
      items: [],
      featuredItems: [],
      selectedItem: null,
      selectedItemVersions: [],
      selectedItemReviews: [],
      installedItemIds: new Set(),
      filters: { ...defaultFilters },
      total: 0,
      loading: false,
      error: null,
      publishDialogOpen: false,
      installConfirmItem: null,
    });
  });

  // ── fetchItems ─────────────────────────────────────────────────────────────

  it("fetchItems populates items and total from API response", async () => {
    const mockItems = [createMarketplaceItem("1"), createMarketplaceItem("2")];
    get.mockResolvedValueOnce({
      data: { items: mockItems, total: 2, page: 1, page_size: 20 },
      status: 200,
    });

    await useMarketplaceStore.getState().fetchItems();

    expect(useMarketplaceStore.getState().items).toHaveLength(2);
    expect(useMarketplaceStore.getState().total).toBe(2);
    expect(useMarketplaceStore.getState().loading).toBe(false);
    expect(useMarketplaceStore.getState().error).toBeNull();
  });

  it("fetchItems sets error when API call fails", async () => {
    get.mockRejectedValueOnce(new Error("Network error"));

    await useMarketplaceStore.getState().fetchItems();

    expect(useMarketplaceStore.getState().error).toBeTruthy();
    expect(useMarketplaceStore.getState().loading).toBe(false);
  });

  it("fetchItems sets loading to false after success", async () => {
    get.mockResolvedValueOnce({
      data: { items: [], total: 0, page: 1, page_size: 20 },
      status: 200,
    });

    await useMarketplaceStore.getState().fetchItems();

    expect(useMarketplaceStore.getState().loading).toBe(false);
  });

  it("fetchItems sends type filter when not 'all'", async () => {
    useMarketplaceStore.setState({
      filters: { ...defaultFilters, type: "plugin" },
    });

    get.mockResolvedValueOnce({
      data: { items: [], total: 0, page: 1, page_size: 20 },
      status: 200,
    });

    await useMarketplaceStore.getState().fetchItems();

    expect(get).toHaveBeenCalledWith(
      expect.stringContaining("type=plugin"),
      expect.objectContaining({ token: "test-token" }),
    );
  });

  it("fetchItems does not send type parameter when 'all'", async () => {
    get.mockResolvedValueOnce({
      data: { items: [], total: 0, page: 1, page_size: 20 },
      status: 200,
    });

    await useMarketplaceStore.getState().fetchItems();

    const [[url]] = get.mock.calls;
    expect(url).not.toContain("type=all");
  });

  // ── fetchFeatured ──────────────────────────────────────────────────────────

  it("fetchFeatured populates featuredItems", async () => {
    const featured = [
      createMarketplaceItem("featured-1", { is_featured: true }),
    ];
    get.mockResolvedValueOnce({ data: featured, status: 200 });

    await useMarketplaceStore.getState().fetchFeatured();

    expect(useMarketplaceStore.getState().featuredItems).toHaveLength(1);
    expect(useMarketplaceStore.getState().featuredItems[0].id).toBe(
      "featured-1",
    );
  });

  it("fetchFeatured silently ignores errors", async () => {
    get.mockRejectedValueOnce(new Error("boom"));

    await expect(
      useMarketplaceStore.getState().fetchFeatured(),
    ).resolves.toBeUndefined();

    expect(useMarketplaceStore.getState().featuredItems).toHaveLength(0);
  });

  // ── search ─────────────────────────────────────────────────────────────────

  it("search populates items from search endpoint", async () => {
    const results = [createMarketplaceItem("search-1")];
    get.mockResolvedValueOnce({ data: results, status: 200 });

    await useMarketplaceStore.getState().search("my-query");

    expect(get).toHaveBeenCalledWith(
      expect.stringContaining("my-query"),
      expect.objectContaining({ token: "test-token" }),
    );
    expect(useMarketplaceStore.getState().items).toHaveLength(1);
    expect(useMarketplaceStore.getState().total).toBe(1);
  });

  it("search sets error on failure", async () => {
    get.mockRejectedValueOnce(new Error("Search failed"));

    await useMarketplaceStore.getState().search("broken");

    expect(useMarketplaceStore.getState().error).toBeTruthy();
    expect(useMarketplaceStore.getState().loading).toBe(false);
  });

  // ── setFilters ─────────────────────────────────────────────────────────────

  it("setFilters merges partial updates without overwriting other fields", async () => {
    // setFilters also calls fetchItems, so mock a response
    get.mockResolvedValue({
      data: { items: [], total: 0, page: 1, page_size: 20 },
      status: 200,
    });

    useMarketplaceStore.getState().setFilters({ type: "plugin" });

    expect(useMarketplaceStore.getState().filters.type).toBe("plugin");
    expect(useMarketplaceStore.getState().filters.sort).toBe("downloads");
    expect(useMarketplaceStore.getState().filters.page).toBe(1);
  });

  it("setFilters triggers a new fetchItems call", async () => {
    get.mockResolvedValue({
      data: { items: [], total: 0, page: 1, page_size: 20 },
      status: 200,
    });

    useMarketplaceStore.getState().setFilters({ sort: "rating" });

    // Wait for the async fetchItems to be enqueued
    await new Promise((r) => setTimeout(r, 10));

    expect(get).toHaveBeenCalled();
  });

  // ── selectItem ─────────────────────────────────────────────────────────────

  it("selectItem sets selectedItem and clears versions/reviews", async () => {
    // fetchItemVersions and fetchItemReviews will be called silently
    get.mockResolvedValue({ data: [], status: 200 });

    const item = createMarketplaceItem("sel-1");
    useMarketplaceStore.getState().selectItem(item);

    expect(useMarketplaceStore.getState().selectedItem).toEqual(item);
    expect(useMarketplaceStore.getState().selectedItemVersions).toHaveLength(0);
    expect(useMarketplaceStore.getState().selectedItemReviews).toHaveLength(0);
  });

  it("selectItem with null clears selectedItem", () => {
    useMarketplaceStore.setState({
      selectedItem: createMarketplaceItem("old"),
    });

    useMarketplaceStore.getState().selectItem(null);

    expect(useMarketplaceStore.getState().selectedItem).toBeNull();
  });

  // ── publishDialogOpen ──────────────────────────────────────────────────────

  it("setPublishDialogOpen toggles publishDialogOpen", () => {
    useMarketplaceStore.getState().setPublishDialogOpen(true);
    expect(useMarketplaceStore.getState().publishDialogOpen).toBe(true);

    useMarketplaceStore.getState().setPublishDialogOpen(false);
    expect(useMarketplaceStore.getState().publishDialogOpen).toBe(false);
  });

  // ── installConfirmItem ─────────────────────────────────────────────────────

  it("setInstallConfirmItem stores and clears the item", () => {
    const item = createMarketplaceItem("confirm-1");

    useMarketplaceStore.getState().setInstallConfirmItem(item);
    expect(useMarketplaceStore.getState().installConfirmItem).toEqual(item);

    useMarketplaceStore.getState().setInstallConfirmItem(null);
    expect(useMarketplaceStore.getState().installConfirmItem).toBeNull();
  });

  // ── installItem ────────────────────────────────────────────────────────────

  it("installItem posts to install endpoint and refreshes installed list", async () => {
    post.mockResolvedValueOnce({ data: {}, status: 200 });
    get.mockResolvedValueOnce({ data: ["item-1"], status: 200 });

    await useMarketplaceStore.getState().installItem("item-1", "1.0.0");

    expect(post).toHaveBeenCalledWith(
      "/api/v1/marketplace/install",
      { item_id: "item-1", version: "1.0.0" },
      { token: "test-token" },
    );
    expect(useMarketplaceStore.getState().installedItemIds.has("item-1")).toBe(
      true,
    );
  });

  // ── publishItem ────────────────────────────────────────────────────────────

  it("publishItem posts to items endpoint and returns created item", async () => {
    const newItem = createMarketplaceItem("pub-1");
    post.mockResolvedValueOnce({ data: newItem, status: 201 });
    // fetchItems called after publish
    get.mockResolvedValueOnce({
      data: { items: [newItem], total: 1, page: 1, page_size: 20 },
      status: 200,
    });

    const result = await useMarketplaceStore.getState().publishItem({
      type: "plugin",
      slug: "test-plugin-pub-1",
      name: "Test Plugin pub-1",
      description: "desc",
      category: "testing",
      tags: [],
      license: "MIT",
    });

    expect(post).toHaveBeenCalledWith(
      "/api/v1/items",
      expect.objectContaining({ type: "plugin" }),
      { token: "test-token" },
    );
    expect(result.id).toBe("pub-1");
  });

  // ── deleteReview ───────────────────────────────────────────────────────────

  it("deleteReview calls DELETE on the review endpoint", async () => {
    del.mockResolvedValueOnce({ data: {}, status: 200 });
    // fetchItemReviews called after delete
    get.mockResolvedValueOnce({ data: [], status: 200 });

    await useMarketplaceStore.getState().deleteReview("item-1");

    expect(del).toHaveBeenCalledWith("/api/v1/items/item-1/reviews/me", {
      token: "test-token",
    });
  });
});
