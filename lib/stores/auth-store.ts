"use client";
import { create } from "zustand";
import { persist } from "zustand/middleware";
import { createApiClient, registerTokenRefresh } from "@/lib/api-client";
import { resolveBackendUrl } from "@/lib/backend-url";
import { withDevtools } from "./_devtools";

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
  /** Refresh the access token using the stored refresh token. Returns the new access token or throws. */
  refreshSession: () => Promise<string>;
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

async function callRefreshEndpoint(refreshToken: string): Promise<AuthSession> {
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
  withDevtools(
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

      refreshSession: async () => {
        const { refreshToken } = get();
        if (!refreshToken) {
          set(unauthenticatedState);
          throw new Error("No refresh token available");
        }
        try {
          const session = await callRefreshEndpoint(refreshToken);
          set({
            ...session,
            status: "authenticated",
          });
          return session.accessToken;
        } catch (error) {
          set(unauthenticatedState);
          throw error;
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
          const newToken = await get().refreshSession();
          const user = await fetchCurrentUser(newToken);
          set({ user, status: "authenticated" });
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
  ),
  { name: "auth-store" },
  )
);

// Register the refresh callback so the API client can transparently refresh
// expired tokens without a circular import on this module.
// Guard: registerTokenRefresh may be undefined in test environments where
// api-client is mocked without this export.
if (typeof registerTokenRefresh === "function") {
  registerTokenRefresh(() => useAuthStore.getState().refreshSession());
}
