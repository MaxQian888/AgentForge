"use client";
import { create } from "zustand";
import { WSClient } from "@/lib/ws-client";
import { useTaskStore } from "./task-store";
import { useAgentStore } from "./agent-store";
import { useNotificationStore } from "./notification-store";
import { useDashboardStore } from "./dashboard-store";

interface WSEventEnvelope<T> {
  type: string;
  projectId?: string;
  payload?: T;
}

function extractPayload<T>(data: unknown): T | null {
  if (!data || typeof data !== "object") {
    return null;
  }

  if ("payload" in data) {
    return ((data as WSEventEnvelope<T>).payload ?? null) as T | null;
  }

  return data as T;
}

interface WSState {
  connected: boolean;
  connect: (url: string, token: string) => void;
  disconnect: () => void;
  subscribe: (channel: string) => void;
  unsubscribe: (channel: string) => void;
}

let client: WSClient | null = null;

export const useWSStore = create<WSState>()((set) => ({
  connected: false,

  connect: (url, token) => {
    if (client) client.close();

    client = new WSClient(url, token);

    client.on("connected", () => set({ connected: true }));
    client.on("disconnected", () => set({ connected: false }));

    client.on("task.updated", (data) => {
      const payload = extractPayload<{ task?: import("./task-store").Task }>(data);
      if (!payload?.task) {
        return;
      }
      useTaskStore.getState().upsertTask(payload.task);
      useDashboardStore.getState().applyTaskUpdate(payload.task);
    });

    client.on("task.created", (data) => {
      const payload = extractPayload<import("./task-store").Task>(data);
      if (!payload) {
        return;
      }
      useTaskStore.getState().upsertTask(payload);
      useDashboardStore.getState().applyTaskUpdate(payload);
    });

    client.on("task.transitioned", (data) => {
      const payload = extractPayload<{ task?: import("./task-store").Task }>(data);
      if (!payload?.task) {
        return;
      }
      useTaskStore.getState().upsertTask(payload.task);
      useDashboardStore.getState().applyTaskUpdate(payload.task);
    });

    client.on("task.assigned", (data) => {
      const payload = extractPayload<{ task?: import("./task-store").Task }>(data);
      if (!payload?.task) {
        return;
      }
      useTaskStore.getState().upsertTask(payload.task);
      useDashboardStore.getState().applyTaskUpdate(payload.task);
    });

    client.on("task.deleted", (data) => {
      const payload = extractPayload<{ id?: string }>(data);
      if (!payload?.id) {
        return;
      }
      useTaskStore.getState().removeTask(payload.id);
    });

    client.on("task.progress.updated", (data) => {
      const payload = extractPayload<{ task?: import("./task-store").Task }>(data);
      if (!payload?.task) {
        return;
      }
      useTaskStore.getState().upsertTask(payload.task);
      useDashboardStore.getState().applyTaskUpdate(payload.task);
    });

    client.on("task.progress.alerted", (data) => {
      const payload = extractPayload<{ task?: import("./task-store").Task }>(data);
      if (!payload?.task) {
        return;
      }
      useTaskStore.getState().upsertTask(payload.task);
      useDashboardStore.getState().applyTaskUpdate(payload.task);
    });

    client.on("task.progress.recovered", (data) => {
      const payload = extractPayload<{ task?: import("./task-store").Task }>(data);
      if (!payload?.task) {
        return;
      }
      useTaskStore.getState().upsertTask(payload.task);
      useDashboardStore.getState().applyTaskUpdate(payload.task);
    });

    client.on("agent.output", (data) => {
      const payload = data as { agent_id: string; line: string };
      useAgentStore.getState().appendOutput(payload.agent_id, payload.line);
    });

    client.on("notification", (data) => {
      const payload = extractPayload(data);
      if (!payload) {
        return;
      }
      useNotificationStore.getState().addNotification(payload as import("./notification-store").Notification);
      useDashboardStore.getState().applyActivityNotification(
        payload as import("./notification-store").Notification
      );
    });

    client.connect();
  },

  disconnect: () => {
    client?.close();
    client = null;
    set({ connected: false });
  },

  subscribe: (channel) => {
    client?.subscribe(channel);
  },

  unsubscribe: (channel) => {
    client?.unsubscribe(channel);
  },
}));
