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
  | "discord";

export interface IMChannel {
  id: string;
  platform: IMPlatform;
  name: string;
  channelId: string;
  webhookUrl: string;
  events: string[];
  active: boolean;
}

export interface IMBridgeStatus {
  registered: boolean;
  lastHeartbeat: string | null;
  providers: string[];
  health: "healthy" | "degraded" | "disconnected";
}

export type IMDeliveryStatus = "delivered" | "suppressed" | "failed";

export interface IMDelivery {
  id: string;
  channelId: string;
  platform: string;
  eventType: string;
  status: IMDeliveryStatus;
  failureReason?: string;
  createdAt: string;
}

/* ── Store state ── */

interface IMState {
  channels: IMChannel[];
  bridgeStatus: IMBridgeStatus;
  deliveries: IMDelivery[];
  loading: boolean;
  error: string | null;

  fetchChannels: () => Promise<void>;
  fetchBridgeStatus: () => Promise<void>;
  fetchDeliveryHistory: () => Promise<void>;
  saveChannel: (channel: Omit<IMChannel, "id"> & { id?: string }) => Promise<void>;
  deleteChannel: (id: string) => Promise<void>;
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
  health: "disconnected",
};

export const useIMStore = create<IMState>()((set, get) => ({
  channels: [],
  bridgeStatus: DEFAULT_BRIDGE_STATUS,
  deliveries: [],
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
      set({ bridgeStatus: data ?? DEFAULT_BRIDGE_STATUS, error: null });
    } catch {
      set({ bridgeStatus: DEFAULT_BRIDGE_STATUS, error: null });
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
}));
