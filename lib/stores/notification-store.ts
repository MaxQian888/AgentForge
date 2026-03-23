"use client";
import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export interface Notification {
  id: string;
  type: string;
  title: string;
  message: string;
  read: boolean;
  createdAt: string;
}

interface NotificationState {
  notifications: Notification[];
  unreadCount: number;
  fetchNotifications: () => Promise<void>;
  markRead: (id: string) => void;
  addNotification: (n: Notification) => void;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export const useNotificationStore = create<NotificationState>()((set) => ({
  notifications: [],
  unreadCount: 0,

  fetchNotifications: async () => {
    const token = useAuthStore.getState().token;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data } = await api.get<Notification[]>("/api/v1/notifications", {
      token,
    });
    set({
      notifications: data,
      unreadCount: data.filter((n) => !n.read).length,
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
    const token = useAuthStore.getState().token;
    if (!token) return;
    const api = createApiClient(API_URL);
    api.put(`/api/v1/notifications/${id}/read`, {}, { token });
  },

  addNotification: (n) => {
    set((s) => ({
      notifications: [n, ...s.notifications],
      unreadCount: s.unreadCount + (n.read ? 0 : 1),
    }));
  },
}));
