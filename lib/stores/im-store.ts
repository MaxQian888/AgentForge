"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

/* ── Types ── */

export type IMPlatform =
  | "feishu"
  | "dingtalk"
  | "slack"
  | "telegram"
  | "discord"
  | "wecom"
  | "qq"
  | "qqbot";

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
}

export interface IMBridgeStatus {
  registered: boolean;
  lastHeartbeat: string | null;
  providers: string[];
  providerDetails: IMBridgeProviderDetail[];
  health: "healthy" | "degraded" | "disconnected";
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
}

/* ── Store state ── */

interface IMState {
  channels: IMChannel[];
  bridgeStatus: IMBridgeStatus;
  deliveries: IMDelivery[];
  eventTypes: string[];
  loading: boolean;
  error: string | null;

  fetchChannels: () => Promise<void>;
  fetchBridgeStatus: () => Promise<void>;
  fetchDeliveryHistory: () => Promise<void>;
  fetchEventTypes: () => Promise<void>;
  saveChannel: (channel: Omit<IMChannel, "id"> & { id?: string }) => Promise<void>;
  deleteChannel: (id: string) => Promise<void>;
  retryDelivery: (id: string) => Promise<void>;
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
};

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
      ? [...status.providerDetails]
      : [...DEFAULT_BRIDGE_STATUS.providerDetails],
    health: status.health ?? DEFAULT_BRIDGE_STATUS.health,
  };
}

export const useIMStore = create<IMState>()((set, get) => ({
  channels: [],
  bridgeStatus: DEFAULT_BRIDGE_STATUS,
  deliveries: [],
  eventTypes: [],
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
      const { data } = await api.get<IMBridgeStatus>(
        "/api/v1/im/bridge/status",
        { token }
      );
      set({ bridgeStatus: normalizeBridgeStatus(data), error: null });
    } catch {
      set({ bridgeStatus: normalizeBridgeStatus(null), error: null });
    }
  },

  fetchDeliveryHistory: async () => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = getApi();
      const { data } = await api.get<IMDelivery[]>("/api/v1/im/deliveries", {
        token,
      });
      set({ deliveries: data ?? [], error: null });
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
      await get().fetchDeliveryHistory();
    } catch {
      set({ error: "Failed to retry delivery" });
    } finally {
      set({ loading: false });
    }
  },
}));
