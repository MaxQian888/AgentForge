"use client";

import { create } from "zustand";
import { ApiError, createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export type MarketplaceItemType = "plugin" | "skill" | "role" | "all";
export type MarketplaceItemSourceType = "marketplace" | "builtin";
export type MarketplaceServiceStatus =
  | "idle"
  | "loading"
  | "ready"
  | "unavailable";
export type MarketplaceConsumptionStatus = "installed" | "blocked" | "warning";
export type MarketplaceConsumerSurface =
  | "plugin-management-panel"
  | "roles-workspace"
  | "role-skill-catalog";

export interface MarketplaceItem {
  id: string;
  type: "plugin" | "skill" | "role";
  slug: string;
  sourceType?: MarketplaceItemSourceType;
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
  localPath?: string;
  previewError?: string;
  skillPreview?: SkillPackagePreview | null;
}

export interface SkillAgentConfigPreview {
  path: string;
  yaml: string;
  displayName?: string;
  shortDescription?: string;
  defaultPrompt?: string;
}

export interface SkillPackagePreview {
  canonicalPath: string;
  label: string;
  displayName?: string;
  description?: string;
  defaultPrompt?: string;
  markdownBody: string;
  frontmatterYaml: string;
  requires: string[];
  tools: string[];
  availableParts: string[];
  referenceCount?: number;
  scriptCount?: number;
  assetCount?: number;
  agentConfigs: SkillAgentConfigPreview[];
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
  tags: string[];
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

export interface UpdateItemRequest {
  name?: string;
  description?: string;
  category?: string;
  tags?: string[];
  icon_url?: string;
  repository_url?: string;
  license?: string;
  extra_metadata?: Record<string, unknown>;
}

export interface MarketplaceConsumptionProvenance {
  sourceType?: string;
  marketplaceItemId: string;
  selectedVersion?: string;
  recordId?: string;
  localPath?: string;
}

export interface MarketplaceConsumptionRecord {
  itemId: string;
  itemType: Exclude<MarketplaceItemType, "all">;
  version?: string;
  status: MarketplaceConsumptionStatus;
  consumerSurface: MarketplaceConsumerSurface;
  installed: boolean;
  used: boolean;
  recordId?: string;
  localPath?: string;
  provenance?: MarketplaceConsumptionProvenance;
  warning?: string;
  failureReason?: string;
  updatedAt?: string;
}

export interface MarketplaceConsumptionResponse {
  items: MarketplaceConsumptionRecord[];
}

export interface MarketplaceUpdateInfo {
  itemId: string;
  itemType: string;
  installedVersion: string;
  latestVersion: string;
  hasUpdate: boolean;
}

export interface MarketplaceInstallResponse {
  ok: boolean;
  item: MarketplaceConsumptionRecord;
  errorCode?: string;
  message?: string;
}

const MARKETPLACE_URL =
  process.env.NEXT_PUBLIC_MARKETPLACE_URL ?? "http://localhost:7781";
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

function getCurrentUserId(): string | null {
  return useAuthStore.getState().user?.id ?? null;
}

function getErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body as { message?: string } | null;
    return body?.message ?? error.message;
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return fallback;
}

async function postMarketplaceMultipart<T>(
  path: string,
  formData: FormData,
): Promise<T> {
  const token = getToken();
  const response = await fetch(`${MARKETPLACE_URL.replace(/\/$/, "")}${path}`, {
    method: "POST",
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    body: formData,
  });

  const data = await response.json().catch(() => null);
  if (!response.ok) {
    throw new ApiError(
      (data as { message?: string } | null)?.message ?? `HTTP ${response.status}`,
      response.status,
      data,
    );
  }

  return data as T;
}

function consumptionRank(status: MarketplaceConsumptionStatus): number {
  switch (status) {
    case "installed":
      return 3;
    case "warning":
      return 2;
    default:
      return 1;
  }
}

export function resolveMarketplaceConsumptionRecord(
  items: MarketplaceConsumptionRecord[],
  itemId: string,
): MarketplaceConsumptionRecord | null {
  let best: MarketplaceConsumptionRecord | null = null;

  for (const item of items) {
    if (item.itemId !== itemId) {
      continue;
    }
    if (!best) {
      best = item;
      continue;
    }

    const bestRank = consumptionRank(best.status);
    const nextRank = consumptionRank(item.status);
    if (nextRank > bestRank) {
      best = item;
      continue;
    }
    if (nextRank < bestRank) {
      continue;
    }

    const bestTime = best.updatedAt ? Date.parse(best.updatedAt) : 0;
    const nextTime = item.updatedAt ? Date.parse(item.updatedAt) : 0;
    if (nextTime >= bestTime) {
      best = item;
    }
  }

  return best;
}

interface MarketplaceState {
  items: MarketplaceItem[];
  builtInItems: MarketplaceItem[];
  featuredItems: MarketplaceItem[];
  selectedItem: MarketplaceItem | null;
  selectedItemVersions: MarketplaceItemVersion[];
  selectedItemReviews: MarketplaceReview[];
  consumptionItems: MarketplaceConsumptionRecord[];
  updates: MarketplaceUpdateInfo[];
  filters: MarketplaceFilters;
  total: number;
  loading: boolean;
  builtInLoading: boolean;
  consumptionLoading: boolean;
  installLoading: boolean;
  uninstallLoading: boolean;
  sideloadLoading: boolean;
  serviceStatus: MarketplaceServiceStatus;
  serviceMessage: string | null;
  builtInMessage: string | null;
  error: string | null;
  publishDialogOpen: boolean;
  installConfirmItem: MarketplaceItem | null;

  fetchItems: () => Promise<void>;
  fetchBuiltInItems: () => Promise<void>;
  fetchFeatured: () => Promise<void>;
  search: (q: string) => Promise<void>;
  selectItem: (item: MarketplaceItem | null) => void;
  refreshSelectedItem: (itemId: string) => Promise<void>;
  fetchItemVersions: (itemId: string) => Promise<void>;
  fetchItemReviews: (itemId: string) => Promise<void>;
  fetchConsumption: () => Promise<void>;
  installItem: (itemId: string, version: string) => Promise<MarketplaceInstallResponse>;
  uninstallItem: (itemId: string, itemType: string) => Promise<void>;
  sideloadItem: (type: string, file: File) => Promise<MarketplaceInstallResponse>;
  checkUpdates: () => Promise<void>;
  installLocalPlugin: (path: string) => Promise<void>;
  publishItem: (req: CreateItemRequest) => Promise<MarketplaceItem>;
  updateItem: (itemId: string, req: UpdateItemRequest) => Promise<MarketplaceItem>;
  deleteItem: (itemId: string) => Promise<void>;
  uploadVersion: (
    itemId: string,
    payload: { version: string; changelog?: string; artifact: File },
  ) => Promise<MarketplaceItemVersion>;
  yankVersion: (itemId: string, version: string) => Promise<void>;
  verifyItem: (itemId: string) => Promise<void>;
  featureItem: (itemId: string) => Promise<void>;
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
  serviceStatus: "idle",
  serviceMessage: null,
  builtInMessage: null,
  error: null,
  publishDialogOpen: false,
  installConfirmItem: null,

  fetchItems: async () => {
    set({
      loading: true,
      serviceStatus: "loading",
      serviceMessage: null,
      error: null,
    });

    try {
      const { filters } = get();
      const api = getMarketApi();
      const token = getToken();
      const params = new URLSearchParams();
      if (filters.type !== "all") params.set("type", filters.type);
      if (filters.category) params.set("category", filters.category);
      if (filters.tags.length > 0) params.set("tags", filters.tags.join(","));
      if (filters.query) params.set("q", filters.query);
      params.set("sort", filters.sort);
      params.set("page", String(filters.page));

      const { data } = await api.get<MarketplaceListResponse>(
        `/api/v1/items?${params.toString()}`,
        { token },
      );

      set({
        items: (data.items ?? []).map((item) =>
          normalizeMarketplaceItem(item, "marketplace"),
        ),
        total: data.total ?? 0,
        serviceStatus: "ready",
        serviceMessage: null,
      });
    } catch (error) {
      const message = getErrorMessage(error, "Marketplace service is unavailable");
      set({
        items: [],
        total: 0,
        serviceStatus: "unavailable",
        serviceMessage: message,
        error: message,
      });
    } finally {
      set({ loading: false });
    }
  },

  fetchBuiltInItems: async () => {
    set({ builtInLoading: true, builtInMessage: null });
    try {
      const api = getApi();
      const token = getToken();
      const { data } = await api.get<MarketplaceItem[]>(
        "/api/v1/marketplace/built-in-skills",
        { token },
      );
      set({
        builtInItems: (data ?? []).map((item) =>
          normalizeMarketplaceItem(item, "builtin"),
        ),
      });
    } catch (error) {
      set({
        builtInItems: [],
        builtInMessage: getErrorMessage(error, "Failed to load built-in skills"),
      });
    } finally {
      set({ builtInLoading: false });
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
      set({
        featuredItems: (data ?? []).map((item) =>
          normalizeMarketplaceItem(item, "marketplace"),
        ),
      });
    } catch {
      set({ featuredItems: [] });
    }
  },

  search: async (q) => {
    set({
      loading: true,
      serviceStatus: "loading",
      serviceMessage: null,
      error: null,
    });

    try {
      const api = getMarketApi();
      const token = getToken();
      const { data } = await api.get<MarketplaceItem[]>(
        `/api/v1/items/search?q=${encodeURIComponent(q)}`,
        { token },
      );
      set({
        items: (data ?? []).map((item) =>
          normalizeMarketplaceItem(item, "marketplace"),
        ),
        total: data?.length ?? 0,
        serviceStatus: "ready",
        serviceMessage: null,
      });
    } catch (error) {
      const message = getErrorMessage(error, "Marketplace service is unavailable");
      set({
        items: [],
        total: 0,
        serviceStatus: "unavailable",
        serviceMessage: message,
        error: message,
      });
    } finally {
      set({ loading: false });
    }
  },

  selectItem: (item) => {
    set({
      selectedItem: item,
      selectedItemVersions: [],
      selectedItemReviews: [],
      error: null,
    });
    if (item) {
      if (item.sourceType === "builtin") {
        void get().refreshSelectedItem(item.id);
        return;
      }
      void Promise.all([
        get().refreshSelectedItem(item.id),
        get().fetchItemVersions(item.id),
        get().fetchItemReviews(item.id),
      ]);
    }
  },

  refreshSelectedItem: async (itemId) => {
    try {
      const token = getToken();
      const current = get().selectedItem;
      const builtInExisting = get().builtInItems.find((item) => item.id === itemId);
      const useBuiltIn = current?.id === itemId
        ? current.sourceType === "builtin"
        : builtInExisting?.sourceType === "builtin";
      const api = useBuiltIn ? getApi() : getMarketApi();
      const path = useBuiltIn
        ? `/api/v1/marketplace/built-in-skills/${itemId}`
        : `/api/v1/items/${itemId}`;
      const { data } = await api.get<MarketplaceItem>(path, { token });
      const normalized = normalizeMarketplaceItem(
        data,
        useBuiltIn ? "builtin" : "marketplace",
      );
      set((state) => ({
        items: state.items.map((item) => (item.id === itemId ? normalized : item)),
        builtInItems: state.builtInItems.map((item) =>
          item.id === itemId ? normalized : item,
        ),
        featuredItems: state.featuredItems.map((item) =>
          item.id === itemId ? normalized : item,
        ),
        selectedItem:
          state.selectedItem?.id === itemId ? normalized : state.selectedItem,
      }));
    } catch (error) {
      set({ error: getErrorMessage(error, "Failed to refresh marketplace item") });
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
    } catch (error) {
      set({ error: getErrorMessage(error, "Failed to load versions") });
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
    } catch (error) {
      set({ error: getErrorMessage(error, "Failed to load reviews") });
    }
  },

  fetchConsumption: async () => {
    set({ consumptionLoading: true, error: null });
    try {
      const api = getApi();
      const token = getToken();
      const { data } = await api.get<MarketplaceConsumptionResponse>(
        "/api/v1/marketplace/consumption",
        { token },
      );
      set({ consumptionItems: data.items ?? [] });
    } catch (error) {
      set({
        consumptionItems: [],
        error: getErrorMessage(error, "Failed to load marketplace install state"),
      });
    } finally {
      set({ consumptionLoading: false });
    }
  },

  installItem: async (itemId, version) => {
    set({ installLoading: true, error: null });
    try {
      const api = getApi();
      const token = getToken();
      const { data } = await api.post<MarketplaceInstallResponse>(
        "/api/v1/marketplace/install",
        { item_id: itemId, version },
        { token },
      );
      await get().fetchConsumption();
      return data;
    } catch (error) {
      const message = getErrorMessage(error, "Installation failed");
      set({ error: message });
      throw error;
    } finally {
      set({ installLoading: false });
    }
  },

  uninstallItem: async (itemId, itemType) => {
    set({ uninstallLoading: true, error: null });
    try {
      const api = getApi();
      const token = getToken();
      await api.post(
        "/api/v1/marketplace/uninstall",
        { item_id: itemId, item_type: itemType },
        { token },
      );
      await get().fetchConsumption();
    } catch (error) {
      const message = getErrorMessage(error, "Uninstall failed");
      set({ error: message });
      throw error;
    } finally {
      set({ uninstallLoading: false });
    }
  },

  sideloadItem: async (type, file) => {
    set({ sideloadLoading: true, error: null });
    try {
      const token = getToken();
      const formData = new FormData();
      formData.append("type", type);
      formData.append("file", file);

      const response = await fetch(
        `${API_URL.replace(/\/$/, "")}/api/v1/marketplace/sideload`,
        {
          method: "POST",
          headers: token ? { Authorization: `Bearer ${token}` } : undefined,
          body: formData,
        },
      );

      const data = await response.json().catch(() => null);
      if (!response.ok) {
        throw new ApiError(
          (data as { message?: string } | null)?.message ?? `HTTP ${response.status}`,
          response.status,
          data,
        );
      }

      await get().fetchConsumption();
      return data as MarketplaceInstallResponse;
    } catch (error) {
      const message = getErrorMessage(error, "Sideload failed");
      set({ error: message });
      throw error;
    } finally {
      set({ sideloadLoading: false });
    }
  },

  checkUpdates: async () => {
    try {
      const api = getApi();
      const token = getToken();
      const { data } = await api.get<{ items: MarketplaceUpdateInfo[] }>(
        "/api/v1/marketplace/updates",
        { token },
      );
      set({ updates: data.items ?? [] });
    } catch {
      set({ updates: [] });
    }
  },

  installLocalPlugin: async (path) => {
    const api = getApi();
    const token = getToken();
    await api.post("/api/v1/plugins/install", { path }, { token });
  },

  publishItem: async (req) => {
    const api = getMarketApi();
    const token = getToken();
    const { data } = await api.post<MarketplaceItem>("/api/v1/items", req, {
      token,
    });
    await Promise.all([get().fetchItems(), get().fetchFeatured()]);
    return normalizeMarketplaceItem(data, "marketplace");
  },

  updateItem: async (itemId, req) => {
    const api = getMarketApi();
    const token = getToken();
    const { data } = await api.patch<MarketplaceItem>(
      `/api/v1/items/${itemId}`,
      req,
      { token },
    );
    await get().refreshSelectedItem(itemId);
    return normalizeMarketplaceItem(data, "marketplace");
  },

  deleteItem: async (itemId) => {
    const api = getMarketApi();
    const token = getToken();
    await api.delete(`/api/v1/items/${itemId}`, { token });
    set((state) => ({
      selectedItem: state.selectedItem?.id === itemId ? null : state.selectedItem,
      selectedItemVersions:
        state.selectedItem?.id === itemId ? [] : state.selectedItemVersions,
      selectedItemReviews:
        state.selectedItem?.id === itemId ? [] : state.selectedItemReviews,
    }));
    await Promise.all([get().fetchItems(), get().fetchFeatured()]);
  },

  uploadVersion: async (itemId, payload) => {
    const formData = new FormData();
    formData.append("version", payload.version);
    formData.append("changelog", payload.changelog ?? "");
    formData.append("artifact", payload.artifact);

    const data = await postMarketplaceMultipart<MarketplaceItemVersion>(
      `/api/v1/items/${itemId}/versions`,
      formData,
    );
    await Promise.all([
      get().fetchItemVersions(itemId),
      get().refreshSelectedItem(itemId),
    ]);
    return data;
  },

  yankVersion: async (itemId, version) => {
    const api = getMarketApi();
    const token = getToken();
    await api.post(`/api/v1/items/${itemId}/versions/${version}/yank`, {}, { token });
    await get().fetchItemVersions(itemId);
  },

  verifyItem: async (itemId) => {
    const api = getMarketApi();
    const token = getToken();
    await api.post(`/admin/items/${itemId}/verify`, {}, { token });
    await get().refreshSelectedItem(itemId);
  },

  featureItem: async (itemId) => {
    const api = getMarketApi();
    const token = getToken();
    await api.post(`/admin/items/${itemId}/feature`, {}, { token });
    await Promise.all([get().refreshSelectedItem(itemId), get().fetchFeatured()]);
  },

  submitReview: async (itemId, rating, comment) => {
    const api = getMarketApi();
    const token = getToken();
    await api.post(
      `/api/v1/items/${itemId}/reviews`,
      { rating, comment },
      { token },
    );
    await Promise.all([
      get().fetchItemReviews(itemId),
      get().refreshSelectedItem(itemId),
    ]);
  },

  deleteReview: async (itemId) => {
    const api = getMarketApi();
    const token = getToken();
    await api.delete(`/api/v1/items/${itemId}/reviews/me`, { token });
    await Promise.all([
      get().fetchItemReviews(itemId),
      get().refreshSelectedItem(itemId),
    ]);
  },

  setFilters: (f) => {
    set((state) => ({ filters: { ...state.filters, ...f } }));
    void get().fetchItems();
  },

  setPublishDialogOpen: (open) => set({ publishDialogOpen: open }),
  setInstallConfirmItem: (item) => set({ installConfirmItem: item }),
}));

export function useMarketplaceCurrentUserId() {
  return getCurrentUserId();
}

function normalizeMarketplaceItem(
  item: MarketplaceItem,
  fallbackSourceType: MarketplaceItemSourceType,
): MarketplaceItem {
  return {
    ...item,
    sourceType: item.sourceType ?? fallbackSourceType,
    skillPreview: item.skillPreview ?? null,
  };
}
