"use client";

import { create } from "zustand";
import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export type IMPlatform =
  | "feishu"
  | "dingtalk"
  | "slack"
  | "telegram"
  | "discord"
  | "wecom"
  | "qq"
  | "qqbot"
  | "email";

export interface IMChannel {
  id: string;
  platform: IMPlatform;
  name: string;
  channelId: string;
  webhookUrl: string;
  platformConfig: Record<string, string>;
  events: string[];
  active: boolean;
}

export interface IMBridgeProviderDetail {
  platform: string;
  capabilityMatrix?: Record<string, unknown>;
  callbackPaths?: string[];
  status?: string;
  transport?: string;
  pendingDeliveries: number;
  recentFailures: number;
  recentDowngrades: number;
  lastDeliveryAt?: string | null;
  diagnostics?: Record<string, string>;
}

export interface IMBridgeStatus {
  registered: boolean;
  lastHeartbeat: string | null;
  providers: string[];
  providerDetails: IMBridgeProviderDetail[];
  health: "healthy" | "degraded" | "disconnected";
  pendingDeliveries: number;
  recentFailures: number;
  recentDowngrades: number;
  averageLatencyMs: number;
}

export type IMDeliveryStatus =
  | "pending"
  | "delivered"
  | "suppressed"
  | "failed"
  | "timeout";

export interface IMDelivery {
  id: string;
  channelId: string;
  platform: string;
  eventType: string;
  status: IMDeliveryStatus;
  failureReason?: string;
  downgradeReason?: string;
  content?: string;
  metadata?: Record<string, string>;
  createdAt: string;
  processedAt?: string;
  latencyMs?: number;
}

export type IMBatchRetryStatus = IMDeliveryStatus | "rejected";

export interface IMBatchRetryItemResult {
  deliveryId: string;
  status: IMBatchRetryStatus;
  message?: string;
}

export interface IMDeliveryHistoryFilters {
  deliveryId?: string;
  status?: string;
  platform?: string;
  eventType?: string;
  kind?: string;
  since?: string;
}

export interface IMTestSendRequest {
  deliveryId?: string;
  platform: string;
  channelId: string;
  projectId?: string;
  bridgeId?: string;
  text: string;
  metadata?: Record<string, string>;
}

export interface IMTestSendResponse {
  deliveryId: string;
  status: IMDeliveryStatus;
  failureReason?: string;
  downgradeReason?: string;
  processedAt?: string;
  latencyMs?: number;
}

interface IMState {
  channels: IMChannel[];
  bridgeStatus: IMBridgeStatus;
  deliveries: IMDelivery[];
  eventTypes: string[];
  historyFilters: IMDeliveryHistoryFilters;
  lastBatchRetryResults: IMBatchRetryItemResult[];
  lastTestSendResult: IMTestSendResponse | null;
  loading: boolean;
  error: string | null;

  fetchChannels: () => Promise<void>;
  fetchBridgeStatus: () => Promise<void>;
  fetchDeliveryHistory: (filters?: IMDeliveryHistoryFilters) => Promise<void>;
  fetchEventTypes: () => Promise<void>;
  saveChannel: (channel: Omit<IMChannel, "id"> & { id?: string }) => Promise<void>;
  deleteChannel: (id: string) => Promise<void>;
  retryDelivery: (id: string) => Promise<void>;
  retryDeliveries: (ids: string[]) => Promise<IMBatchRetryItemResult[]>;
  testSend: (request: IMTestSendRequest) => Promise<IMTestSendResponse | null>;
  setHistoryFilters: (filters: IMDeliveryHistoryFilters) => void;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function getApi() {
  return createApiClient(API_URL);
}

function getToken() {
  return useAuthStore.getState().accessToken;
}

const DEFAULT_BRIDGE_STATUS: IMBridgeStatus = {
  registered: false,
  lastHeartbeat: null,
  providers: [],
  providerDetails: [],
  health: "disconnected",
  pendingDeliveries: 0,
  recentFailures: 0,
  recentDowngrades: 0,
  averageLatencyMs: 0,
};

function normalizeProviderDetail(
  detail: Partial<IMBridgeProviderDetail> | null | undefined,
): IMBridgeProviderDetail | null {
  if (!detail || typeof detail.platform !== "string" || detail.platform.trim() === "") {
    return null;
  }

  return {
    platform: detail.platform,
    capabilityMatrix:
      detail.capabilityMatrix && typeof detail.capabilityMatrix === "object"
        ? { ...detail.capabilityMatrix }
        : undefined,
    callbackPaths: Array.isArray(detail.callbackPaths) ? [...detail.callbackPaths] : undefined,
    status: typeof detail.status === "string" ? detail.status : undefined,
    transport: typeof detail.transport === "string" ? detail.transport : undefined,
    pendingDeliveries:
      typeof detail.pendingDeliveries === "number" ? detail.pendingDeliveries : 0,
    recentFailures:
      typeof detail.recentFailures === "number" ? detail.recentFailures : 0,
    recentDowngrades:
      typeof detail.recentDowngrades === "number" ? detail.recentDowngrades : 0,
    lastDeliveryAt:
      typeof detail.lastDeliveryAt === "string" || detail.lastDeliveryAt === null
        ? detail.lastDeliveryAt
        : undefined,
    diagnostics:
      detail.diagnostics && typeof detail.diagnostics === "object"
        ? { ...(detail.diagnostics as Record<string, string>) }
        : undefined,
  };
}

function normalizeDelivery(delivery: Partial<IMDelivery> | null | undefined): IMDelivery | null {
  if (
    !delivery ||
    typeof delivery.id !== "string" ||
    typeof delivery.channelId !== "string" ||
    typeof delivery.platform !== "string" ||
    typeof delivery.eventType !== "string" ||
    typeof delivery.status !== "string" ||
    typeof delivery.createdAt !== "string"
  ) {
    return null;
  }

  return {
    id: delivery.id,
    channelId: delivery.channelId,
    platform: delivery.platform,
    eventType: delivery.eventType,
    status: delivery.status as IMDeliveryStatus,
    failureReason:
      typeof delivery.failureReason === "string" ? delivery.failureReason : undefined,
    downgradeReason:
      typeof delivery.downgradeReason === "string" ? delivery.downgradeReason : undefined,
    content: typeof delivery.content === "string" ? delivery.content : undefined,
    metadata:
      delivery.metadata && typeof delivery.metadata === "object"
        ? { ...(delivery.metadata as Record<string, string>) }
        : undefined,
    createdAt: delivery.createdAt,
    processedAt: typeof delivery.processedAt === "string" ? delivery.processedAt : undefined,
    latencyMs: typeof delivery.latencyMs === "number" ? delivery.latencyMs : undefined,
  };
}

function buildDeliveryHistoryPath(filters?: IMDeliveryHistoryFilters): string {
  const params = new URLSearchParams();
  if (!filters) {
    return "/api/v1/im/deliveries";
  }
  if (filters.deliveryId) params.set("deliveryId", filters.deliveryId);
  if (filters.status) params.set("status", filters.status);
  if (filters.platform) params.set("platform", filters.platform);
  if (filters.eventType) params.set("eventType", filters.eventType);
  if (filters.kind) params.set("kind", filters.kind);
  if (filters.since) params.set("since", filters.since);
  const query = params.toString();
  return query ? `/api/v1/im/deliveries?${query}` : "/api/v1/im/deliveries";
}

function normalizeBridgeStatus(
  status: Partial<IMBridgeStatus> | null | undefined,
): IMBridgeStatus {
  if (!status) {
    return {
      ...DEFAULT_BRIDGE_STATUS,
      providers: [...DEFAULT_BRIDGE_STATUS.providers],
      providerDetails: [...DEFAULT_BRIDGE_STATUS.providerDetails],
    };
  }

  return {
    registered: status.registered ?? DEFAULT_BRIDGE_STATUS.registered,
    lastHeartbeat:
      typeof status.lastHeartbeat === "string" || status.lastHeartbeat === null
        ? status.lastHeartbeat
        : DEFAULT_BRIDGE_STATUS.lastHeartbeat,
    providers: Array.isArray(status.providers)
      ? [...status.providers]
      : [...DEFAULT_BRIDGE_STATUS.providers],
    providerDetails: Array.isArray(status.providerDetails)
      ? status.providerDetails
          .map((detail) => normalizeProviderDetail(detail))
          .filter((detail): detail is IMBridgeProviderDetail => detail !== null)
      : [...DEFAULT_BRIDGE_STATUS.providerDetails],
    health: status.health ?? DEFAULT_BRIDGE_STATUS.health,
    pendingDeliveries:
      typeof status.pendingDeliveries === "number"
        ? status.pendingDeliveries
        : DEFAULT_BRIDGE_STATUS.pendingDeliveries,
    recentFailures:
      typeof status.recentFailures === "number"
        ? status.recentFailures
        : DEFAULT_BRIDGE_STATUS.recentFailures,
    recentDowngrades:
      typeof status.recentDowngrades === "number"
        ? status.recentDowngrades
        : DEFAULT_BRIDGE_STATUS.recentDowngrades,
    averageLatencyMs:
      typeof status.averageLatencyMs === "number"
        ? status.averageLatencyMs
        : DEFAULT_BRIDGE_STATUS.averageLatencyMs,
  };
}

export const useIMStore = create<IMState>()((set, get) => ({
  channels: [],
  bridgeStatus: DEFAULT_BRIDGE_STATUS,
  deliveries: [],
  eventTypes: [],
  historyFilters: {},
  lastBatchRetryResults: [],
  lastTestSendResult: null,
  loading: false,
  error: null,

  fetchChannels: async () => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = getApi();
      const { data } = await api.get<IMChannel[]>("/api/v1/im/channels", {
        token,
      });
      set({ channels: data ?? [], error: null });
    } catch {
      set({ channels: [], error: "Unable to load IM channels" });
    } finally {
      set({ loading: false });
    }
  },

  fetchBridgeStatus: async () => {
    const token = getToken();
    if (!token) return;

    set({ error: null });
    try {
      const api = getApi();
      const { data } = await api.get<IMBridgeStatus>("/api/v1/im/bridge/status", {
        token,
      });
      set({ bridgeStatus: normalizeBridgeStatus(data), error: null });
    } catch {
      set({ bridgeStatus: normalizeBridgeStatus(null), error: null });
    }
  },

  fetchDeliveryHistory: async (filters) => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = getApi();
      const nextFilters = filters ?? get().historyFilters;
      const { data } = await api.get<IMDelivery[]>(buildDeliveryHistoryPath(nextFilters), {
        token,
      });
      set({
        deliveries: Array.isArray(data)
          ? data
              .map((delivery) => normalizeDelivery(delivery))
              .filter((delivery): delivery is IMDelivery => delivery !== null)
          : [],
        historyFilters: { ...nextFilters },
        error: null,
      });
    } catch {
      set({ deliveries: [], error: "Unable to load delivery history" });
    } finally {
      set({ loading: false });
    }
  },

  fetchEventTypes: async () => {
    const token = getToken();
    if (!token) return;

    try {
      const api = getApi();
      const { data } = await api.get<string[]>("/api/v1/im/event-types", {
        token,
      });
      set({ eventTypes: data ?? [], error: null });
    } catch {
      set({ eventTypes: [], error: "Unable to load IM event types" });
    }
  },

  saveChannel: async (channel) => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = getApi();
      if (channel.id) {
        await api.put(`/api/v1/im/channels/${channel.id}`, channel, { token });
      } else {
        await api.post("/api/v1/im/channels", channel, { token });
      }
      await get().fetchChannels();
    } catch {
      toast.error("Failed to save channel");
      set({ error: "Failed to save channel" });
    } finally {
      set({ loading: false });
    }
  },

  deleteChannel: async (id) => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = getApi();
      await api.delete(`/api/v1/im/channels/${id}`, { token });
      await get().fetchChannels();
    } catch {
      toast.error("Failed to delete channel");
      set({ error: "Failed to delete channel" });
    } finally {
      set({ loading: false });
    }
  },

  retryDelivery: async (id) => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = getApi();
      await api.post(`/api/v1/im/deliveries/${id}/retry`, {}, { token });
      await get().fetchDeliveryHistory(get().historyFilters);
    } catch {
      toast.error("Failed to retry delivery");
      set({ error: "Failed to retry delivery" });
    } finally {
      set({ loading: false });
    }
  },

  retryDeliveries: async (ids) => {
    const token = getToken();
    if (!token || ids.length === 0) return [];

    set({ loading: true, error: null });
    try {
      const api = getApi();
      const { data } = await api.post<{ results?: IMBatchRetryItemResult[] }>(
        "/api/v1/im/deliveries/retry-batch",
        { deliveryIds: ids },
        { token },
      );
      const results = Array.isArray(data?.results) ? data.results : [];
      await get().fetchDeliveryHistory(get().historyFilters);
      set({ lastBatchRetryResults: results, error: null });
      return results;
    } catch {
      toast.error("Failed to retry deliveries");
      set({ error: "Failed to retry deliveries" });
      return [];
    } finally {
      set({ loading: false });
    }
  },

  testSend: async (request) => {
    const token = getToken();
    if (!token) return null;

    set({ loading: true, error: null });
    try {
      const api = getApi();
      const { data } = await api.post<IMTestSendResponse>("/api/v1/im/test-send", request, {
        token,
      });
      const result = data ?? null;
      await get().fetchBridgeStatus();
      await get().fetchDeliveryHistory(get().historyFilters);
      set({ lastTestSendResult: result, error: null });
      return result;
    } catch {
      set({ error: "Failed to send IM test message" });
      return null;
    } finally {
      set({ loading: false });
    }
  },

  setHistoryFilters: (filters) => {
    set({ historyFilters: { ...filters } });
  },
}));
