"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

// ── Types ───────────────────────────────────────────────────────────────────

export type MarketplaceItemType = "plugin" | "skill" | "role" | "all";

export interface MarketplaceItem {
  id: string;
  type: "plugin" | "skill" | "role";
  slug: string;
  name: string;
  author_id: string;
  author_name: string;
  description: string;
  category: string;
  tags: string[];
  icon_url?: string;
  repository_url?: string;
  license: string;
  extra_metadata: Record<string, unknown>;
  latest_version?: string;
  download_count: number;
  avg_rating: number;
  rating_count: number;
  is_verified: boolean;
  is_featured: boolean;
  created_at: string;
  updated_at: string;
}

export interface MarketplaceItemVersion {
  id: string;
  item_id: string;
  version: string;
  changelog: string;
  artifact_size_bytes: number;
  artifact_digest: string;
  is_latest: boolean;
  is_yanked: boolean;
  created_at: string;
}

export interface MarketplaceReview {
  id: string;
  item_id: string;
  user_id: string;
  user_name: string;
  rating: number;
  comment: string;
  created_at: string;
  updated_at: string;
}

export interface MarketplaceListResponse {
  items: MarketplaceItem[];
  total: number;
  page: number;
  page_size: number;
}

export interface MarketplaceFilters {
  type: MarketplaceItemType;
  category: string;
  sort: "downloads" | "rating" | "created_at";
  page: number;
  query: string;
}

export interface CreateItemRequest {
  type: "plugin" | "skill" | "role";
  slug: string;
  name: string;
  description: string;
  category: string;
  tags: string[];
  icon_url?: string;
  repository_url?: string;
  license: string;
  extra_metadata?: Record<string, unknown>;
}

// ── Store ───────────────────────────────────────────────────────────────────

const MARKETPLACE_URL =
  process.env.NEXT_PUBLIC_MARKETPLACE_URL ?? "http://localhost:7779";
const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function getMarketApi() {
  return createApiClient(MARKETPLACE_URL);
}

function getApi() {
  return createApiClient(API_URL);
}

function getToken(): string {
  return useAuthStore.getState().accessToken ?? "";
}

interface MarketplaceState {
  items: MarketplaceItem[];
  featuredItems: MarketplaceItem[];
  selectedItem: MarketplaceItem | null;
  selectedItemVersions: MarketplaceItemVersion[];
  selectedItemReviews: MarketplaceReview[];
  installedItemIds: Set<string>;
  filters: MarketplaceFilters;
  total: number;
  loading: boolean;
  error: string | null;
  publishDialogOpen: boolean;
  installConfirmItem: MarketplaceItem | null;

  fetchItems: () => Promise<void>;
  fetchFeatured: () => Promise<void>;
  search: (q: string) => Promise<void>;
  selectItem: (item: MarketplaceItem | null) => void;
  fetchItemVersions: (itemId: string) => Promise<void>;
  fetchItemReviews: (itemId: string) => Promise<void>;
  fetchInstalled: () => Promise<void>;
  installItem: (itemId: string, version: string) => Promise<void>;
  publishItem: (req: CreateItemRequest) => Promise<MarketplaceItem>;
  submitReview: (
    itemId: string,
    rating: number,
    comment: string,
  ) => Promise<void>;
  deleteReview: (itemId: string) => Promise<void>;
  setFilters: (f: Partial<MarketplaceFilters>) => void;
  setPublishDialogOpen: (open: boolean) => void;
  setInstallConfirmItem: (item: MarketplaceItem | null) => void;
}

export const useMarketplaceStore = create<MarketplaceState>()((set, get) => ({
  items: [],
  featuredItems: [],
  selectedItem: null,
  selectedItemVersions: [],
  selectedItemReviews: [],
  installedItemIds: new Set(),
  filters: {
    type: "all",
    category: "",
    sort: "downloads",
    page: 1,
    query: "",
  },
  total: 0,
  loading: false,
  error: null,
  publishDialogOpen: false,
  installConfirmItem: null,

  fetchItems: async () => {
    set({ loading: true, error: null });
    try {
      const { filters } = get();
      const api = getMarketApi();
      const token = getToken();
      const params = new URLSearchParams();
      if (filters.type !== "all") params.set("type", filters.type);
      if (filters.category) params.set("category", filters.category);
      if (filters.query) params.set("q", filters.query);
      params.set("sort", filters.sort);
      params.set("page", String(filters.page));
      const { data } = await api.get<MarketplaceListResponse>(
        `/api/v1/items?${params}`,
        { token },
      );
      set({ items: data.items ?? [], total: data.total ?? 0 });
    } catch (e) {
      set({ error: e instanceof Error ? e.message : "Failed to fetch items" });
    } finally {
      set({ loading: false });
    }
  },

  fetchFeatured: async () => {
    try {
      const api = getMarketApi();
      const token = getToken();
      const { data } = await api.get<MarketplaceItem[]>(
        "/api/v1/items/featured",
        { token },
      );
      set({ featuredItems: data ?? [] });
    } catch {
      /* silent */
    }
  },

  search: async (q: string) => {
    set({ loading: true, error: null });
    try {
      const api = getMarketApi();
      const token = getToken();
      const { data } = await api.get<MarketplaceItem[]>(
        `/api/v1/items/search?q=${encodeURIComponent(q)}`,
        { token },
      );
      set({ items: data ?? [], total: data?.length ?? 0 });
    } catch (e) {
      set({ error: e instanceof Error ? e.message : "Search failed" });
    } finally {
      set({ loading: false });
    }
  },

  selectItem: (item) => {
    set({
      selectedItem: item,
      selectedItemVersions: [],
      selectedItemReviews: [],
    });
    if (item) {
      void get().fetchItemVersions(item.id);
      void get().fetchItemReviews(item.id);
    }
  },

  fetchItemVersions: async (itemId) => {
    try {
      const api = getMarketApi();
      const token = getToken();
      const { data } = await api.get<MarketplaceItemVersion[]>(
        `/api/v1/items/${itemId}/versions`,
        { token },
      );
      set({ selectedItemVersions: data ?? [] });
    } catch {
      /* silent */
    }
  },

  fetchItemReviews: async (itemId) => {
    try {
      const api = getMarketApi();
      const token = getToken();
      const { data } = await api.get<MarketplaceReview[]>(
        `/api/v1/items/${itemId}/reviews`,
        { token },
      );
      set({ selectedItemReviews: data ?? [] });
    } catch {
      /* silent */
    }
  },

  fetchInstalled: async () => {
    try {
      const api = getApi();
      const token = getToken();
      const { data } = await api.get<string[]>(
        "/api/v1/marketplace/installed",
        { token },
      );
      set({ installedItemIds: new Set(data ?? []) });
    } catch {
      /* silent */
    }
  },

  installItem: async (itemId, version) => {
    const api = getApi();
    const token = getToken();
    await api.post(
      "/api/v1/marketplace/install",
      { item_id: itemId, version },
      { token },
    );
    await get().fetchInstalled();
  },

  publishItem: async (req) => {
    const api = getMarketApi();
    const token = getToken();
    const { data } = await api.post<MarketplaceItem>("/api/v1/items", req, {
      token,
    });
    await get().fetchItems();
    return data;
  },

  submitReview: async (itemId, rating, comment) => {
    const api = getMarketApi();
    const token = getToken();
    await api.post(
      `/api/v1/items/${itemId}/reviews`,
      { rating, comment },
      { token },
    );
    await get().fetchItemReviews(itemId);
    const { data: updatedItem } = await api.get<MarketplaceItem>(
      `/api/v1/items/${itemId}`,
      { token },
    );
    set((state) => ({
      items: state.items.map((i) => (i.id === itemId ? updatedItem : i)),
      selectedItem:
        state.selectedItem?.id === itemId ? updatedItem : state.selectedItem,
    }));
  },

  deleteReview: async (itemId) => {
    const api = getMarketApi();
    const token = getToken();
    await api.delete(`/api/v1/items/${itemId}/reviews/me`, { token });
    await get().fetchItemReviews(itemId);
  },

  setFilters: (f) => {
    set((state) => ({ filters: { ...state.filters, ...f } }));
    void get().fetchItems();
  },

  setPublishDialogOpen: (open) => set({ publishDialogOpen: open }),
  setInstallConfirmItem: (item) => set({ installConfirmItem: item }),
}));
