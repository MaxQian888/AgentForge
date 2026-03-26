"use client";
import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export interface Notification {
  id: string;
  targetId?: string;
  type: string;
  title: string;
  message: string;
  data?: string;
  href?: string | null;
  read: boolean;
  createdAt: string;
}

interface NotificationApiShape {
  id: string;
  targetId?: string;
  type: string;
  title: string;
  body?: string;
  message?: string;
  data?: string;
  isRead?: boolean;
  read?: boolean;
  createdAt: string;
}

interface NotificationState {
  notifications: Notification[];
  unreadCount: number;
  fetchNotifications: () => Promise<void>;
  markRead: (id: string) => void;
  markAllRead: () => void;
  addNotification: (n: NotificationApiShape | Notification) => void;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function extractHref(data?: string): string | null {
  if (!data) {
    return null;
  }

  try {
    const parsed = JSON.parse(data) as { href?: string };
    return typeof parsed.href === "string" ? parsed.href : null;
  } catch {
    return null;
  }
}

function normalizeNotification(
  notification: NotificationApiShape | Notification
): Notification {
  return {
    id: notification.id,
    targetId: "targetId" in notification ? notification.targetId : undefined,
    type: notification.type,
    title: notification.title,
    message:
      "message" in notification && typeof notification.message === "string"
        ? notification.message
        : "body" in notification && typeof notification.body === "string"
          ? notification.body
          : "",
    data: "data" in notification ? notification.data : undefined,
    href:
      "href" in notification && typeof notification.href !== "undefined"
        ? notification.href
        : extractHref("data" in notification ? notification.data : undefined),
    read:
      "read" in notification && typeof notification.read === "boolean"
        ? notification.read
        : Boolean(notification.isRead),
    createdAt: notification.createdAt,
  };
}

function dedupeNotifications(notifications: Notification[]): Notification[] {
  const deduped: Notification[] = [];
  const seen = new Set<string>();

  for (const notification of notifications) {
    if (seen.has(notification.id)) {
      continue;
    }

    seen.add(notification.id);
    deduped.push(notification);
  }

  return deduped;
}

function upsertNotification(
  notifications: Notification[],
  incoming: Notification
): Notification[] {
  const existingIndex = notifications.findIndex((entry) => entry.id === incoming.id);

  if (existingIndex === -1) {
    return [incoming, ...notifications];
  }

  const next = [...notifications];
  next[existingIndex] = incoming;
  return next;
}

export const useNotificationStore = create<NotificationState>()((set) => ({
  notifications: [],
  unreadCount: 0,

  fetchNotifications: async () => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data } = await api.get<NotificationApiShape[]>(
      "/api/v1/notifications",
      {
        token,
      }
    );
    const notifications = dedupeNotifications(data.map(normalizeNotification));
    set({
      notifications,
      unreadCount: notifications.filter((n) => !n.read).length,
    });
  },

  markRead: (id) => {
    set((s) => {
      const notifications = s.notifications.map((n) =>
        n.id === id ? { ...n, read: true } : n
      );
      return {
        notifications,
        unreadCount: notifications.filter((n) => !n.read).length,
      };
    });
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    api.put(`/api/v1/notifications/${id}/read`, {}, { token });
  },

  markAllRead: () => {
    set((s) => ({
      notifications: s.notifications.map((n) => ({ ...n, read: true })),
      unreadCount: 0,
    }));
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    api.put("/api/v1/notifications/read-all", {}, { token });
  },

  addNotification: (n) => {
    const normalized = normalizeNotification(n);
    set((s) => {
      const notifications = upsertNotification(s.notifications, normalized);
      return {
        notifications,
        unreadCount: notifications.filter((entry) => !entry.read).length,
      };
    });
  },
}));
