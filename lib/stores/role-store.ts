"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface RoleMetadata {
  id: string;
  name: string;
  version: string;
  description: string;
  author: string;
  tags: string[];
  icon?: string;
}

export interface RoleIdentity {
  role?: string;
  goal?: string;
  backstory?: string;
  systemPrompt: string;
  persona: string;
  goals: string[];
  constraints: string[];
  personality?: string;
  language?: string;
}

export interface RoleCapabilities {
  allowedTools?: string[];
  tools?: string[];
  skills?: RoleSkillReference[];
  languages: string[];
  frameworks: string[];
  maxTurns?: number;
  maxBudgetUsd?: number;
}

export interface RoleSkillReference {
  path: string;
  autoLoad: boolean;
}

export interface RoleSecurity {
  permissionMode?: string;
  allowedPaths: string[];
  deniedPaths: string[];
  maxBudgetUsd: number;
  requireReview: boolean;
}

export interface RoleManifest {
  apiVersion: string;
  kind: string;
  metadata: RoleMetadata;
  identity: RoleIdentity;
  capabilities: RoleCapabilities;
  knowledge: { repositories: string[]; documents: string[]; patterns: string[] };
  security: RoleSecurity;
  extends?: string;
}

interface RoleState {
  roles: RoleManifest[];
  loading: boolean;
  error: string | null;
  fetchRoles: () => Promise<void>;
  createRole: (data: Partial<RoleManifest>) => Promise<RoleManifest>;
  updateRole: (id: string, data: Partial<RoleManifest>) => Promise<RoleManifest>;
  deleteRole: (id: string) => Promise<void>;
}

function getToken() {
  const authState = useAuthStore.getState() as {
    accessToken?: string | null;
    token?: string | null;
  };
  return authState.accessToken ?? authState.token ?? null;
}

export const useRoleStore = create<RoleState>()((set) => ({
  roles: [],
  loading: false,
  error: null,

  fetchRoles: async () => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null });

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<RoleManifest[]>("/api/v1/roles", {
        token,
      });

      set({ roles: data });
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Failed to load roles";
      set({ error: message });
    } finally {
      set({ loading: false });
    }
  },

  createRole: async (data) => {
    const token = getToken();
    if (!token) throw new Error("Not authenticated");

    const api = createApiClient(API_URL);
    const { data: role } = await api.post<RoleManifest>(
      "/api/v1/roles",
      data,
      { token }
    );

    set((state) => ({ roles: [...state.roles, role] }));

    return role;
  },

  updateRole: async (id, data) => {
    const token = getToken();
    if (!token) throw new Error("Not authenticated");

    const api = createApiClient(API_URL);
    const { data: role } = await api.put<RoleManifest>(
      `/api/v1/roles/${id}`,
      data,
      { token }
    );

    set((state) => ({
      roles: state.roles.map((r) =>
        r.metadata.id === id ? role : r
      ),
    }));

    return role;
  },

  deleteRole: async (id) => {
    const token = getToken();
    if (!token) throw new Error("Not authenticated");

    const api = createApiClient(API_URL);
    await api.delete(`/api/v1/roles/${id}`, { token });

    set((state) => ({
      roles: state.roles.filter((r) => r.metadata.id !== id),
    }));
  },
}));
