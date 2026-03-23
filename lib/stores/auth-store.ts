"use client";
import { create } from "zustand";
import { persist } from "zustand/middleware";
import { createApiClient } from "@/lib/api-client";

interface User {
  id: string;
  email: string;
  name: string;
}

interface AuthState {
  token: string | null;
  user: User | null;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, name: string) => Promise<void>;
  logout: () => void;
  setToken: (token: string) => void;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      isAuthenticated: false,

      login: async (email, password) => {
        const api = createApiClient(API_URL);
        const { data } = await api.post<{ token: string; user: User }>(
          "/api/v1/auth/login",
          { email, password }
        );
        set({ token: data.token, user: data.user, isAuthenticated: true });
      },

      register: async (email, password, name) => {
        const api = createApiClient(API_URL);
        const { data } = await api.post<{ token: string; user: User }>(
          "/api/v1/auth/register",
          { email, password, name }
        );
        set({ token: data.token, user: data.user, isAuthenticated: true });
      },

      logout: () => set({ token: null, user: null, isAuthenticated: false }),

      setToken: (token) => set({ token, isAuthenticated: true }),
    }),
    { name: "auth-storage" }
  )
);
