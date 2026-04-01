const post = jest.fn();
const get = jest.fn();
const patch = jest.fn();
const del = jest.fn();

jest.mock("@/lib/api-client", () => {
  const actual = jest.requireActual("@/lib/api-client");
  return {
    ...actual,
    createApiClient: jest.fn(() => ({
      post,
      get,
      patch,
      delete: del,
    })),
  };
});

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({
      accessToken: "test-token",
      user: { id: "user-1" },
    })),
  },
}));

import {
  resolveMarketplaceConsumptionRecord,
  useMarketplaceStore,
  type MarketplaceConsumptionRecord,
  type MarketplaceItem,
  type SkillPackagePreview,
  type MarketplaceFilters,
} from "./marketplace-store";

function createMarketplaceItem(
  id: string,
  overrides: Partial<MarketplaceItem> = {},
): MarketplaceItem {
  return {
    id,
    type: "plugin",
    slug: `item-${id}`,
    name: `Item ${id}`,
    author_id: "user-1",
    author_name: "Author",
    description: "Marketplace item",
    category: "testing",
    tags: [],
    license: "MIT",
    extra_metadata: {},
    download_count: 10,
    avg_rating: 4.2,
    rating_count: 2,
    is_verified: false,
    is_featured: false,
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
    ...overrides,
  };
}

const defaultFilters: MarketplaceFilters = {
  type: "all",
  category: "",
  tags: [],
  sort: "downloads",
  page: 1,
  query: "",
};

describe("marketplace-store", () => {
  beforeEach(() => {
    post.mockReset();
    get.mockReset();
    patch.mockReset();
    del.mockReset();

    useMarketplaceStore.setState({
      items: [],
      builtInItems: [],
      featuredItems: [],
      selectedItem: null,
      selectedItemVersions: [],
      selectedItemReviews: [],
      consumptionItems: [],
      filters: { ...defaultFilters },
      total: 0,
      loading: false,
      builtInLoading: false,
      consumptionLoading: false,
      installLoading: false,
      serviceStatus: "idle",
      serviceMessage: null,
      builtInMessage: null,
      error: null,
      publishDialogOpen: false,
      installConfirmItem: null,
    });
  });

  it("fetchItems populates items and marks the marketplace as ready", async () => {
    get.mockResolvedValueOnce({
      data: {
        items: [createMarketplaceItem("1"), createMarketplaceItem("2")],
        total: 2,
        page: 1,
        page_size: 20,
      },
      status: 200,
    });

    await useMarketplaceStore.getState().fetchItems();

    expect(useMarketplaceStore.getState().items).toHaveLength(2);
    expect(useMarketplaceStore.getState().total).toBe(2);
    expect(useMarketplaceStore.getState().serviceStatus).toBe("ready");
    expect(useMarketplaceStore.getState().serviceMessage).toBeNull();
  });

  it("fetchItems surfaces marketplace unavailability instead of silently returning empty state", async () => {
    get.mockRejectedValueOnce(new Error("service unavailable"));

    await useMarketplaceStore.getState().fetchItems();

    expect(useMarketplaceStore.getState().items).toHaveLength(0);
    expect(useMarketplaceStore.getState().serviceStatus).toBe("unavailable");
    expect(useMarketplaceStore.getState().serviceMessage).toContain("service unavailable");
  });

  it("fetchBuiltInItems loads repo-owned built-in skills from the app backend", async () => {
    const preview: SkillPackagePreview = {
      canonicalPath: "skills/react",
      label: "React",
      markdownBody: "# React",
      frontmatterYaml: "name: React",
      requires: ["skills/typescript"],
      tools: ["code_editor", "browser_preview"],
      availableParts: ["agents", "references"],
      agentConfigs: [
        {
          path: "agents/openai.yaml",
          yaml: "interface:\n  display_name: AgentForge React",
        },
      ],
    };
    get.mockResolvedValueOnce({
      data: [
        createMarketplaceItem("react", {
          type: "skill",
          slug: "react",
          sourceType: "builtin",
          localPath: "D:/Project/AgentForge/skills/react",
          skillPreview: preview,
        }),
      ],
      status: 200,
    });

    await useMarketplaceStore.getState().fetchBuiltInItems();

    expect(get).toHaveBeenCalledWith("/api/v1/marketplace/built-in-skills", {
      token: "test-token",
    });
    expect(useMarketplaceStore.getState().builtInItems).toHaveLength(1);
    expect(useMarketplaceStore.getState().builtInItems[0]?.sourceType).toBe("builtin");
    expect(useMarketplaceStore.getState().builtInItems[0]?.skillPreview?.canonicalPath).toBe(
      "skills/react",
    );
  });

  it("fetchConsumption loads typed install state", async () => {
    const consumption: MarketplaceConsumptionRecord = {
      itemId: "item-1",
      itemType: "plugin",
      version: "1.0.0",
      status: "installed",
      consumerSurface: "plugin-management-panel",
      installed: true,
      used: true,
      recordId: "repo-search",
      provenance: {
        marketplaceItemId: "item-1",
        selectedVersion: "1.0.0",
      },
      updatedAt: "2024-01-01T00:00:00Z",
    };
    get.mockResolvedValueOnce({
      data: { items: [consumption] },
      status: 200,
    });

    await useMarketplaceStore.getState().fetchConsumption();

    expect(useMarketplaceStore.getState().consumptionItems).toEqual([consumption]);
  });

  it("installItem posts the install request and refreshes typed consumption", async () => {
    post.mockResolvedValueOnce({
      data: {
        ok: true,
        item: {
          itemId: "item-1",
          itemType: "plugin",
          version: "1.0.0",
          status: "installed",
          consumerSurface: "plugin-management-panel",
          installed: true,
          used: true,
        },
      },
      status: 200,
    });
    get.mockResolvedValueOnce({
      data: {
        items: [
          {
            itemId: "item-1",
            itemType: "plugin",
            version: "1.0.0",
            status: "installed",
            consumerSurface: "plugin-management-panel",
            installed: true,
            used: true,
          },
        ],
      },
      status: 200,
    });

    const result = await useMarketplaceStore.getState().installItem("item-1", "1.0.0");

    expect(post).toHaveBeenCalledWith(
      "/api/v1/marketplace/install",
      { item_id: "item-1", version: "1.0.0" },
      { token: "test-token" },
    );
    expect(result.ok).toBe(true);
    expect(useMarketplaceStore.getState().consumptionItems[0]?.itemId).toBe("item-1");
  });

  it("uninstallItem posts uninstall request and refreshes consumption", async () => {
    post.mockResolvedValueOnce({
      data: { ok: true, itemId: "item-1", message: "item uninstalled" },
      status: 200,
    });
    get.mockResolvedValueOnce({
      data: { items: [] },
      status: 200,
    });

    await useMarketplaceStore.getState().uninstallItem("item-1", "plugin");

    expect(post).toHaveBeenCalledWith(
      "/api/v1/marketplace/uninstall",
      { item_id: "item-1", item_type: "plugin" },
      { token: "test-token" },
    );
    expect(useMarketplaceStore.getState().consumptionItems).toEqual([]);
    expect(useMarketplaceStore.getState().uninstallLoading).toBe(false);
  });

  it("uninstallItem sets error on failure", async () => {
    post.mockRejectedValueOnce(new Error("uninstall failed"));

    await expect(
      useMarketplaceStore.getState().uninstallItem("item-1", "plugin"),
    ).rejects.toThrow("uninstall failed");

    expect(useMarketplaceStore.getState().error).toBe("uninstall failed");
    expect(useMarketplaceStore.getState().uninstallLoading).toBe(false);
  });

  it("checkUpdates populates updates list from backend", async () => {
    get.mockResolvedValueOnce({
      data: {
        items: [
          {
            itemId: "item-1",
            itemType: "plugin",
            installedVersion: "1.0.0",
            latestVersion: "2.0.0",
            hasUpdate: true,
          },
        ],
      },
      status: 200,
    });

    await useMarketplaceStore.getState().checkUpdates();

    expect(get).toHaveBeenCalledWith(
      "/api/v1/marketplace/updates",
      { token: "test-token" },
    );
    expect(useMarketplaceStore.getState().updates).toHaveLength(1);
    expect(useMarketplaceStore.getState().updates[0]?.hasUpdate).toBe(true);
  });

  it("checkUpdates sets empty array on failure", async () => {
    get.mockRejectedValueOnce(new Error("service down"));

    await useMarketplaceStore.getState().checkUpdates();

    expect(useMarketplaceStore.getState().updates).toEqual([]);
  });

  it("setFilters includes tags in query params when fetching", async () => {
    get.mockResolvedValueOnce({
      data: { items: [], total: 0, page: 1, page_size: 20 },
      status: 200,
    });

    useMarketplaceStore.getState().setFilters({ tags: ["react", "nextjs"], page: 1 });

    await new Promise((r) => setTimeout(r, 50));

    const lastCall = get.mock.calls.find(
      (call: string[]) => typeof call[0] === "string" && call[0].includes("/api/v1/items?"),
    );
    expect(lastCall).toBeDefined();
    expect(lastCall![0]).toContain("tags=react%2Cnextjs");
  });
});

describe("resolveMarketplaceConsumptionRecord", () => {
  it("prefers installed records over blocked or warning records for the same item", () => {
    const result = resolveMarketplaceConsumptionRecord(
      [
        {
          itemId: "item-1",
          itemType: "plugin",
          status: "blocked",
          consumerSurface: "plugin-management-panel",
          installed: false,
          used: false,
          updatedAt: "2024-01-01T00:00:00Z",
        },
        {
          itemId: "item-1",
          itemType: "plugin",
          status: "installed",
          consumerSurface: "plugin-management-panel",
          installed: true,
          used: true,
          updatedAt: "2024-01-02T00:00:00Z",
        },
      ],
      "item-1",
    );

    expect(result?.status).toBe("installed");
    expect(result?.used).toBe(true);
  });
});
