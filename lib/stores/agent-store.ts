"use client";
import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export type AgentStatus = "running" | "paused" | "completed" | "failed" | "killed";

export interface Agent {
  id: string;
  taskId: string;
  taskTitle: string;
  memberId: string;
  roleName: string;
  status: AgentStatus;
  turns: number;
  cost: number;
  budget: number;
  lastActivity: string;
  createdAt: string;
}

interface AgentState {
  agents: Agent[];
  agentOutputs: Map<string, string[]>;
  loading: boolean;
  fetchAgents: () => Promise<void>;
  spawnAgent: (taskId: string, memberId: string) => Promise<void>;
  pauseAgent: (id: string) => Promise<void>;
  killAgent: (id: string) => Promise<void>;
  appendOutput: (id: string, line: string) => void;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export const useAgentStore = create<AgentState>()((set, get) => ({
  agents: [],
  agentOutputs: new Map(),
  loading: false,

  fetchAgents: async () => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    set({ loading: true });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<Agent[]>("/api/v1/agents", { token });
      set({ agents: data });
    } finally {
      set({ loading: false });
    }
  },

  spawnAgent: async (taskId, memberId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data: agent } = await api.post<Agent>(
      "/api/v1/agents",
      { task_id: taskId, member_id: memberId },
      { token }
    );
    set((s) => ({ agents: [...s.agents, agent] }));
  },

  pauseAgent: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    await api.post(`/api/v1/agents/${id}/pause`, {}, { token });
    set((s) => ({
      agents: s.agents.map((a) =>
        a.id === id ? { ...a, status: "paused" as AgentStatus } : a
      ),
    }));
  },

  killAgent: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    const api = createApiClient(API_URL);
    await api.post(`/api/v1/agents/${id}/kill`, {}, { token });
    set((s) => ({
      agents: s.agents.map((a) =>
        a.id === id ? { ...a, status: "killed" as AgentStatus } : a
      ),
    }));
  },

  appendOutput: (id, line) => {
    set((s) => {
      const outputs = new Map(s.agentOutputs);
      const lines = outputs.get(id) ?? [];
      outputs.set(id, [...lines, line]);
      return { agentOutputs: outputs };
    });
  },
}));
