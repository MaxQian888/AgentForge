import { render, screen, waitFor } from "@testing-library/react";

jest.mock("react-markdown", () => ({
  __esModule: true,
  default: ({ children }: { children: unknown }) => children,
}));

jest.mock("remark-gfm", () => ({
  __esModule: true,
  default: () => null,
}));

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      serviceUnavailable: "Marketplace service unavailable",
      remoteUnavailable: "Remote marketplace unavailable",
      builtInItems: "Built-in items",
      "detail.sideload.pluginButton": "Side-load local plugin",
      "detail.moderation.verify": "Verify",
      "detail.moderation.feature": "Feature",
    };
    return map[key] ?? key;
  },
}));

import MarketplacePage from "./page";

const selectFiles = jest.fn();
const fetchItems = jest.fn().mockResolvedValue(undefined);
const fetchBuiltInItems = jest.fn().mockResolvedValue(undefined);
const fetchFeatured = jest.fn().mockResolvedValue(undefined);
const fetchConsumption = jest.fn().mockResolvedValue(undefined);
const search = jest.fn().mockResolvedValue(undefined);
const selectItem = jest.fn();
const setFilters = jest.fn();
const setPublishDialogOpen = jest.fn();
const setInstallConfirmItem = jest.fn();
const installLocalPlugin = jest.fn().mockResolvedValue(undefined);
const uploadVersion = jest.fn().mockResolvedValue(undefined);
const deleteItem = jest.fn().mockResolvedValue(undefined);
const verifyItem = jest.fn().mockResolvedValue(undefined);
const featureItem = jest.fn().mockResolvedValue(undefined);

function createMarketplaceItem(
  overrides: Record<string, unknown> = {},
) {
  return {
    id: "item-1",
    type: "skill",
    slug: "item-1",
    name: "Item 1",
    author_id: "author-1",
    author_name: "Author",
    description: "Marketplace item",
    category: "frontend",
    tags: [],
    license: "MIT",
    extra_metadata: {},
    latest_version: "1.0.0",
    download_count: 10,
    avg_rating: 4.8,
    rating_count: 6,
    is_verified: true,
    is_featured: false,
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
    ...overrides,
  };
}

const baseState = {
  items: [],
  builtInItems: [],
  featuredItems: [],
  selectedItem: null,
  selectedItemVersions: [],
  selectedItemReviews: [],
  consumptionItems: [],
  updates: [],
  filters: {
    type: "all",
    category: "",
    tags: [],
    sort: "downloads",
    page: 1,
    query: "",
  },
  total: 0,
  loading: false,
  builtInLoading: false,
  consumptionLoading: false,
  installLoading: false,
  uninstallLoading: false,
  sideloadLoading: false,
  serviceStatus: "ready",
  serviceMessage: null,
  builtInMessage: null,
  error: null,
  publishDialogOpen: false,
  installConfirmItem: null,
  fetchItems,
  fetchBuiltInItems,
  fetchFeatured,
  fetchConsumption,
  checkUpdates: jest.fn().mockResolvedValue(undefined),
  uninstallItem: jest.fn().mockResolvedValue(undefined),
  sideloadItem: jest.fn().mockResolvedValue({ ok: true }),
  search,
  selectItem,
  setFilters,
  setPublishDialogOpen,
  setInstallConfirmItem,
  installLocalPlugin,
  uploadVersion,
  deleteItem,
  verifyItem,
  featureItem,
  fetchItemVersions: jest.fn().mockResolvedValue(undefined),
  fetchItemReviews: jest.fn().mockResolvedValue(undefined),
  refreshSelectedItem: jest.fn().mockResolvedValue(undefined),
  installItem: jest.fn().mockResolvedValue({ ok: true }),
  updateItem: jest.fn().mockResolvedValue(undefined),
  yankVersion: jest.fn().mockResolvedValue(undefined),
  submitReview: jest.fn().mockResolvedValue(undefined),
  deleteReview: jest.fn().mockResolvedValue(undefined),
  publishItem: jest.fn().mockResolvedValue(undefined),
};

const useMarketplaceStore = jest.fn((selector?: (state: typeof baseState) => unknown) =>
  typeof selector === "function" ? selector(baseState) : baseState,
);

jest.mock("@/lib/stores/marketplace-store", () => {
  const actual = jest.requireActual("@/lib/stores/marketplace-store");
  return {
    ...actual,
    useMarketplaceStore: (...args: unknown[]) =>
      useMarketplaceStore(...(args as [])),
  };
});

jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: (selector: (state: { user: { id: string } | null }) => unknown) =>
    selector({ user: { id: "author-1" } }),
}));

jest.mock("@/hooks/use-platform-capability", () => ({
  usePlatformCapability: () => ({
    isDesktop: true,
    selectFiles,
  }),
}));

describe("MarketplacePage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    window.history.replaceState({}, "", "/marketplace");
    Object.assign(baseState, {
      items: [],
      builtInItems: [],
      featuredItems: [],
      selectedItem: null,
      selectedItemVersions: [],
      selectedItemReviews: [],
      consumptionItems: [],
      loading: false,
      builtInLoading: false,
      serviceStatus: "ready",
      serviceMessage: null,
      builtInMessage: null,
      publishDialogOpen: false,
      installConfirmItem: null,
    });
  });

  it("loads marketplace items, featured items, and typed consumption on mount", async () => {
    render(<MarketplacePage />);

    await waitFor(() => {
      expect(fetchItems).toHaveBeenCalled();
      expect(fetchBuiltInItems).toHaveBeenCalled();
      expect(fetchFeatured).toHaveBeenCalled();
      expect(fetchConsumption).toHaveBeenCalled();
    });
  });

  it("renders an explicit unavailable state when the marketplace service is down", () => {
    Object.assign(baseState, {
      serviceStatus: "unavailable",
      serviceMessage: "Marketplace is offline",
    });

    render(<MarketplacePage />);

    expect(screen.getByText("Marketplace service unavailable")).toBeInTheDocument();
    expect(screen.getByText("Marketplace is offline")).toBeInTheDocument();
  });

  it("keeps built-in skills visible when the remote marketplace is unavailable", () => {
    Object.assign(baseState, {
      serviceStatus: "unavailable",
      serviceMessage: "Remote marketplace is offline",
      builtInItems: [
        {
          id: "react",
          type: "skill",
          slug: "react",
          sourceType: "builtin",
          name: "React",
          author_id: "agentforge",
          author_name: "AgentForge",
          description: "Build React surfaces.",
          category: "frontend",
          tags: ["react"],
          license: "MIT",
          extra_metadata: {},
          download_count: 0,
          avg_rating: 0,
          rating_count: 0,
          is_verified: true,
          is_featured: true,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
          localPath: "D:/Project/AgentForge/skills/react",
          skillPreview: {
            canonicalPath: "skills/react",
            label: "React",
            markdownBody: "# React",
            frontmatterYaml: "name: React",
            requires: ["skills/typescript"],
            tools: ["code_editor"],
            availableParts: ["agents"],
            agentConfigs: [],
          },
        },
      ],
    });

    render(<MarketplacePage />);

    expect(screen.getByText("Remote marketplace unavailable")).toBeInTheDocument();
    expect(screen.getByText("Built-in items")).toBeInTheDocument();
    expect(screen.getByText("React")).toBeInTheDocument();
  });

  it("renders detail affordances for moderation, version upload, and local side-load", () => {
    Object.assign(baseState, {
      items: [
        {
          id: "plugin-item",
          type: "plugin",
          slug: "plugin-item",
          name: "Plugin Item",
          author_id: "author-1",
          author_name: "Author",
          description: "Plugin description",
          category: "tools",
          tags: [],
          license: "MIT",
          extra_metadata: {},
          latest_version: "1.0.0",
          download_count: 10,
          avg_rating: 4.8,
          rating_count: 6,
          is_verified: false,
          is_featured: false,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
      ],
      selectedItem: {
        id: "plugin-item",
        type: "plugin",
        slug: "plugin-item",
        name: "Plugin Item",
        author_id: "author-1",
        author_name: "Author",
        description: "Plugin description",
        category: "tools",
        tags: [],
        license: "MIT",
        extra_metadata: {},
        latest_version: "1.0.0",
        download_count: 10,
        avg_rating: 4.8,
        rating_count: 6,
        is_verified: false,
        is_featured: false,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
      consumptionItems: [
        {
          itemId: "plugin-item",
          itemType: "plugin",
          status: "warning",
          consumerSurface: "plugin-management-panel",
          installed: false,
          used: false,
          warning: "Install needs follow-up",
        },
      ],
    });

    render(<MarketplacePage />);

    expect(screen.getAllByText("Install needs follow-up")).toHaveLength(2);
    expect(screen.getByRole("button", { name: "Verify" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Feature" })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Side-load local plugin" }),
    ).toBeInTheDocument();
  });

  it("hydrates a deep-linked item only once so later selections are not overridden", async () => {
    const deepLinkedItem = createMarketplaceItem({
      id: "react-skill",
      slug: "react-skill",
      name: "React Skill",
      sourceType: "builtin",
      skillPreview: {
        canonicalPath: "skills/react",
        label: "React Skill",
        markdownBody: "# React Skill",
        frontmatterYaml: "name: React Skill",
        requires: [],
        tools: ["code_editor"],
        availableParts: ["agents"],
        agentConfigs: [],
      },
    });
    const otherItem = createMarketplaceItem({
      id: "plugin-item",
      type: "plugin",
      slug: "plugin-item",
      name: "Plugin Item",
      category: "tools",
    });
    window.history.replaceState({}, "", "/marketplace?item=react-skill");
    Object.assign(baseState, {
      builtInItems: [deepLinkedItem],
      items: [otherItem],
      selectedItem: null,
    });

    const { rerender } = render(<MarketplacePage />);

    await waitFor(() => {
      expect(selectItem).toHaveBeenCalledTimes(1);
      expect(selectItem).toHaveBeenLastCalledWith(
        expect.objectContaining({ id: "react-skill" }),
      );
    });

    Object.assign(baseState, {
      selectedItem: otherItem,
    });
    rerender(<MarketplacePage />);

    await waitFor(() => {
      expect(selectItem).toHaveBeenCalledTimes(1);
    });

    window.history.replaceState({}, "", "/marketplace");
  });
});
