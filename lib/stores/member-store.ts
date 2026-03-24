"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";
import {
  normalizeTeamMember,
  type DashboardMemberSource,
  type TeamMember,
} from "@/lib/dashboard/summary";
import {
  serializeAgentProfileDraft,
  type AgentProfileDraft,
} from "@/lib/team/agent-profile";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface CreateMemberInput {
  name: string;
  type: "human" | "agent";
  role?: string;
  email?: string;
  skills?: string[];
  agentProfile?: AgentProfileDraft;
  agentConfig?: string;
}

export interface UpdateMemberInput {
  name?: string;
  role?: string;
  email?: string;
  skills?: string[];
  agentProfile?: AgentProfileDraft;
  agentConfig?: string;
  isActive?: boolean;
}

interface MemberState {
  membersByProject: Record<string, TeamMember[]>;
  loadingByProject: Record<string, boolean>;
  errorByProject: Record<string, string | null>;
  fetchMembers: (projectId: string) => Promise<void>;
  createMember: (projectId: string, input: CreateMemberInput) => Promise<TeamMember>;
  updateMember: (memberId: string, projectId: string, input: UpdateMemberInput) => Promise<TeamMember>;
}

function getToken() {
  const authState = useAuthStore.getState() as {
    accessToken?: string | null;
    token?: string | null;
  };
  return authState.accessToken ?? authState.token ?? null;
}

function normalizeMembers(members: DashboardMemberSource[]): TeamMember[] {
  return members.map(normalizeTeamMember);
}

function resolveAgentConfig(input: {
  agentProfile?: AgentProfileDraft;
  agentConfig?: string;
}) {
  if (input.agentProfile) {
    return serializeAgentProfileDraft(input.agentProfile);
  }
  return input.agentConfig ?? "";
}

export const useMemberStore = create<MemberState>()((set) => ({
  membersByProject: {},
  loadingByProject: {},
  errorByProject: {},

  fetchMembers: async (projectId) => {
    const token = getToken();
    if (!token) return;

    set((state) => ({
      loadingByProject: { ...state.loadingByProject, [projectId]: true },
      errorByProject: { ...state.errorByProject, [projectId]: null },
    }));

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<DashboardMemberSource[]>(
        `/api/v1/projects/${projectId}/members`,
        { token }
      );

      set((state) => ({
        membersByProject: {
          ...state.membersByProject,
          [projectId]: normalizeMembers(data),
        },
      }));
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Failed to load team members";
      set((state) => ({
        errorByProject: { ...state.errorByProject, [projectId]: message },
      }));
    } finally {
      set((state) => ({
        loadingByProject: { ...state.loadingByProject, [projectId]: false },
      }));
    }
  },

  createMember: async (projectId, input) => {
    const token = getToken();
    if (!token) {
      throw new Error("Missing auth token");
    }

    const api = createApiClient(API_URL);
    const { data } = await api.post<DashboardMemberSource>(
      `/api/v1/projects/${projectId}/members`,
      {
        name: input.name,
        type: input.type,
        role: input.role ?? "",
        email: input.email ?? "",
        skills: input.skills ?? [],
        agentConfig: resolveAgentConfig(input),
      },
      { token }
    );

    const member = normalizeTeamMember(data);
    set((state) => ({
      membersByProject: {
        ...state.membersByProject,
        [projectId]: [...(state.membersByProject[projectId] ?? []), member],
      },
    }));
    return member;
  },

  updateMember: async (memberId, projectId, input) => {
    const token = getToken();
    if (!token) {
      throw new Error("Missing auth token");
    }

    const api = createApiClient(API_URL);
    const { data } = await api.put<DashboardMemberSource>(
      `/api/v1/members/${memberId}`,
      {
        name: input.name,
        role: input.role,
        email: input.email,
        skills: input.skills,
        agentConfig: resolveAgentConfig(input),
        isActive: input.isActive,
      },
      { token }
    );

    const member = normalizeTeamMember(data);
    set((state) => ({
      membersByProject: {
        ...state.membersByProject,
        [projectId]: (state.membersByProject[projectId] ?? []).map((item) =>
          item.id === memberId ? member : item
        ),
      },
    }));
    return member;
  },
}));
