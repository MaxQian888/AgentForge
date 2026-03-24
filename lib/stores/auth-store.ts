"use client";
import { create } from "zustand";
import { persist } from "zustand/middleware";
import { createApiClient } from "@/lib/api-client";
import { resolveBackendUrl } from "@/lib/backend-url";

export interface AuthUser {
  id: string;
  email: string;
  name: string;
}

export interface AuthSession {
  accessToken: string;
  refreshToken: string;
  user: AuthUser;
}

export type AuthStatus =
  | "idle"
  | "checking"
  | "authenticated"
  | "unauthenticated";

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  user: AuthUser | null;
  status: AuthStatus;
  hasHydrated: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, name: string) => Promise<void>;
  logout: () => Promise<void>;
  bootstrapSession: () => Promise<void>;
  clearSession: () => void;
  getAccessToken: () => string | null;
}

const unauthenticatedState = {
  accessToken: null,
  refreshToken: null,
  user: null,
  status: "unauthenticated" as AuthStatus,
};

function isUnauthorizedError(error: unknown): boolean {
  return (error as { status?: number })?.status === 401;
}

async function getApiClient() {
  return createApiClient(await resolveBackendUrl());
}

async function fetchCurrentUser(accessToken: string): Promise<AuthUser> {
  const api = await getApiClient();
  const { data } = await api.get<AuthUser>("/api/v1/users/me", {
    token: accessToken,
  });
  return data;
}

async function refreshSession(refreshToken: string): Promise<AuthSession> {
  const api = await getApiClient();
  const { data } = await api.post<AuthSession>("/api/v1/auth/refresh", {
    refreshToken,
  });
  return data;
}

async function authenticate(
  path: "/api/v1/auth/login" | "/api/v1/auth/register",
  body: unknown
): Promise<AuthSession> {
  const api = await getApiClient();
  const { data } = await api.post<AuthSession>(path, body);
  return data;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      accessToken: null,
      refreshToken: null,
      user: null,
      status: "idle",
      hasHydrated: false,

      login: async (email, password) => {
        set({ status: "checking" });
        try {
          const session = await authenticate("/api/v1/auth/login", {
            email,
            password,
          });
          set({
            ...session,
            status: "authenticated",
          });
        } catch (error) {
          set({ status: "unauthenticated" });
          throw error;
        }
      },

      register: async (email, password, name) => {
        set({ status: "checking" });
        try {
          const session = await authenticate("/api/v1/auth/register", {
            email,
            password,
            name,
          });
          set({
            ...session,
            status: "authenticated",
          });
        } catch (error) {
          set({ status: "unauthenticated" });
          throw error;
        }
      },

      logout: async () => {
        const accessToken = get().accessToken;
        try {
          if (accessToken) {
            const api = await getApiClient();
            await api.post("/api/v1/auth/logout", {}, { token: accessToken });
          }
        } finally {
          get().clearSession();
        }
      },

      bootstrapSession: async () => {
        const { accessToken, refreshToken, status } = get();
        if (status === "checking") {
          return;
        }

        if (!accessToken && !refreshToken) {
          set(unauthenticatedState);
          return;
        }

        set({ status: "checking" });

        if (accessToken) {
          try {
            const user = await fetchCurrentUser(accessToken);
            set({ user, status: "authenticated" });
            return;
          } catch (error) {
            if (!refreshToken || !isUnauthorizedError(error)) {
              set(unauthenticatedState);
              return;
            }
          }
        }

        if (!refreshToken) {
          set(unauthenticatedState);
          return;
        }

        try {
          const session = await refreshSession(refreshToken);
          const user = await fetchCurrentUser(session.accessToken);
          set({
            ...session,
            user,
            status: "authenticated",
          });
        } catch {
          set(unauthenticatedState);
        }
      },

      clearSession: () => set(unauthenticatedState),

      getAccessToken: () => get().accessToken,
    }),
    {
      name: "auth-storage",
      partialize: (state) => ({
        accessToken: state.accessToken,
        refreshToken: state.refreshToken,
        user: state.user,
      }),
      onRehydrateStorage: () => (state) => {
        if (!state) {
          return;
        }
        state.hasHydrated = true;
        state.status =
          state.accessToken || state.refreshToken
            ? "idle"
            : "unauthenticated";
      },
    }
  )
);
