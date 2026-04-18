"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export type InvitationStatus =
  | "pending"
  | "accepted"
  | "declined"
  | "expired"
  | "revoked";

export type InvitationIdentityKind = "email" | "im";

export interface InvitationIdentity {
  kind: InvitationIdentityKind;
  value?: string; // email form
  platform?: string; // IM form
  userId?: string;
  displayName?: string;
}

export interface InvitationDTO {
  id: string;
  projectId: string;
  inviterUserId: string;
  invitedIdentity: InvitationIdentity;
  invitedUserId?: string;
  projectRole: string;
  status: InvitationStatus;
  message?: string;
  expiresAt: string;
  createdAt: string;
  updatedAt: string;
  acceptedAt?: string;
  declineReason?: string;
  revokeReason?: string;
  lastDeliveryStatus?: string;
  lastDeliveryAttemptedAt?: string;
}

export interface InvitationCreateResponse {
  invitation: InvitationDTO;
  acceptToken: string;
  acceptUrl: string;
}

export interface InvitationPublicPreview {
  projectName: string;
  projectRole: string;
  inviterName?: string;
  inviterEmail?: string;
  message?: string;
  expiresAt: string;
  status: InvitationStatus;
  identityKind: InvitationIdentityKind;
  identityHint?: string;
}

export interface CreateInvitationInput {
  invitedIdentity: InvitationIdentity;
  projectRole: string;
  message?: string;
  expiresAt?: string; // RFC3339
}

interface InvitationState {
  invitationsByProject: Record<string, InvitationDTO[]>;
  loadingByProject: Record<string, boolean>;
  errorByProject: Record<string, string | null>;
  lastCreateTokenByProject: Record<string, InvitationCreateResponse | null>;
  fetchInvitations: (projectId: string, status?: InvitationStatus) => Promise<void>;
  createInvitation: (
    projectId: string,
    input: CreateInvitationInput,
  ) => Promise<InvitationCreateResponse>;
  revokeInvitation: (
    projectId: string,
    invitationId: string,
    reason?: string,
  ) => Promise<InvitationDTO>;
  resendInvitation: (
    projectId: string,
    invitationId: string,
  ) => Promise<InvitationDTO>;
  clearLastCreateToken: (projectId: string) => void;
}

function getToken() {
  const authState = useAuthStore.getState() as {
    accessToken?: string | null;
    token?: string | null;
  };
  return authState.accessToken ?? authState.token ?? null;
}

function upsert(list: InvitationDTO[], invitation: InvitationDTO): InvitationDTO[] {
  const idx = list.findIndex((item) => item.id === invitation.id);
  if (idx === -1) return [invitation, ...list];
  const next = list.slice();
  next[idx] = invitation;
  return next;
}

export const useInvitationStore = create<InvitationState>()((set) => ({
  invitationsByProject: {},
  loadingByProject: {},
  errorByProject: {},
  lastCreateTokenByProject: {},

  fetchInvitations: async (projectId, status) => {
    const token = getToken();
    if (!token) return;
    set((state) => ({
      loadingByProject: { ...state.loadingByProject, [projectId]: true },
      errorByProject: { ...state.errorByProject, [projectId]: null },
    }));
    try {
      const api = createApiClient(API_URL);
      const query = status ? `?status=${status}` : "";
      const { data } = await api.get<InvitationDTO[]>(
        `/api/v1/projects/${projectId}/invitations${query}`,
        { token },
      );
      set((state) => ({
        invitationsByProject: {
          ...state.invitationsByProject,
          [projectId]: data ?? [],
        },
      }));
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Failed to load invitations";
      set((state) => ({
        errorByProject: { ...state.errorByProject, [projectId]: message },
      }));
    } finally {
      set((state) => ({
        loadingByProject: { ...state.loadingByProject, [projectId]: false },
      }));
    }
  },

  createInvitation: async (projectId, input) => {
    const token = getToken();
    if (!token) throw new Error("Missing auth token");
    const api = createApiClient(API_URL);
    const { data } = await api.post<InvitationCreateResponse>(
      `/api/v1/projects/${projectId}/invitations`,
      input,
      { token },
    );
    set((state) => ({
      invitationsByProject: {
        ...state.invitationsByProject,
        [projectId]: upsert(
          state.invitationsByProject[projectId] ?? [],
          data.invitation,
        ),
      },
      lastCreateTokenByProject: {
        ...state.lastCreateTokenByProject,
        [projectId]: data,
      },
    }));
    return data;
  },

  revokeInvitation: async (projectId, invitationId, reason) => {
    const token = getToken();
    if (!token) throw new Error("Missing auth token");
    const api = createApiClient(API_URL);
    const { data } = await api.post<InvitationDTO>(
      `/api/v1/projects/${projectId}/invitations/${invitationId}/revoke`,
      { reason: reason ?? "" },
      { token },
    );
    set((state) => ({
      invitationsByProject: {
        ...state.invitationsByProject,
        [projectId]: upsert(
          state.invitationsByProject[projectId] ?? [],
          data,
        ),
      },
    }));
    return data;
  },

  resendInvitation: async (projectId, invitationId) => {
    const token = getToken();
    if (!token) throw new Error("Missing auth token");
    const api = createApiClient(API_URL);
    const { data } = await api.post<InvitationDTO>(
      `/api/v1/projects/${projectId}/invitations/${invitationId}/resend`,
      {},
      { token },
    );
    set((state) => ({
      invitationsByProject: {
        ...state.invitationsByProject,
        [projectId]: upsert(
          state.invitationsByProject[projectId] ?? [],
          data,
        ),
      },
    }));
    return data;
  },

  clearLastCreateToken: (projectId) => {
    set((state) => ({
      lastCreateTokenByProject: {
        ...state.lastCreateTokenByProject,
        [projectId]: null,
      },
    }));
  },
}));

/**
 * Public-preview client used by the accept page. The endpoint is
 * unauthenticated so this function does not require a token.
 */
export async function fetchInvitationPreview(
  token: string,
): Promise<InvitationPublicPreview> {
  const api = createApiClient(API_URL);
  const { data } = await api.get<InvitationPublicPreview>(
    `/api/v1/invitations/by-token/${encodeURIComponent(token)}`,
  );
  return data;
}

/**
 * Accept invitation (requires auth). Callers must already be logged in.
 */
export async function acceptInvitation(token: string): Promise<{
  invitation: InvitationDTO;
}> {
  const authToken = getToken();
  if (!authToken) throw new Error("Not signed in");
  const api = createApiClient(API_URL);
  const { data } = await api.post<{ invitation: InvitationDTO }>(
    `/api/v1/invitations/accept`,
    { token },
    { token: authToken },
  );
  return data;
}

/**
 * Decline invitation (auth optional).
 */
export async function declineInvitation(
  token: string,
  reason?: string,
): Promise<InvitationDTO> {
  const authToken = getToken();
  const api = createApiClient(API_URL);
  const { data } = await api.post<InvitationDTO>(
    `/api/v1/invitations/decline`,
    { token, reason: reason ?? "" },
    authToken ? { token: authToken } : undefined,
  );
  return data;
}
