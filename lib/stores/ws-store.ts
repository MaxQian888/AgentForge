"use client";
import { create } from "zustand";
import { WSClient } from "@/lib/ws-client";
import { useTaskStore } from "./task-store";
import { useAgentStore } from "./agent-store";
import { useNotificationStore } from "./notification-store";

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
      const payload = data as { task: { id: string; status: string } };
      useTaskStore
        .getState()
        .transitionTask(
          payload.task.id,
          payload.task.status as import("./task-store").TaskStatus
        );
    });

    client.on("agent.output", (data) => {
      const payload = data as { agent_id: string; line: string };
      useAgentStore.getState().appendOutput(payload.agent_id, payload.line);
    });

    client.on("notification", (data) => {
      useNotificationStore
        .getState()
        .addNotification(
          data as import("./notification-store").Notification
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
