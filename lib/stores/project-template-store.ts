"use client";
import { create } from "zustand";
import { toast } from "sonner";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export type ProjectTemplateSource = "system" | "user" | "marketplace";

/**
 * UI-facing projection of a project template row.
 *
 * `snapshot` is populated only by {@link useProjectTemplateStore.fetchDetail};
 * the list endpoint returns entries without a snapshot payload to keep the
 * list response bounded.
 */
export interface ProjectTemplate {
  id: string;
  source: ProjectTemplateSource;
  ownerUserId?: string;
  name: string;
  description?: string;
  snapshotVersion: number;
  snapshot?: unknown;
  createdAt: string;
  updatedAt: string;
}

interface ProjectTemplateListResponse {
  templates: ProjectTemplate[];
}

interface ProjectTemplateState {
  templates: ProjectTemplate[];
  detailsById: Record<string, ProjectTemplate>;
  loading: boolean;
  saving: boolean;
  fetchTemplates: () => Promise<void>;
  fetchDetail: (id: string) => Promise<ProjectTemplate | undefined>;
  saveAsTemplate: (
    projectId: string,
    input: { name: string; description?: string },
  ) => Promise<ProjectTemplate | undefined>;
  updateTemplate: (
    id: string,
    input: { name?: string; description?: string },
  ) => Promise<ProjectTemplate | undefined>;
  deleteTemplate: (id: string) => Promise<void>;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function getToken(): string {
  try {
    return useAuthStore.getState().accessToken ?? "";
  } catch {
    return "";
  }
}

function normalizeTemplate(raw: unknown): ProjectTemplate | null {
  if (!raw || typeof raw !== "object") return null;
  const r = raw as Record<string, unknown>;
  const id = typeof r.id === "string" ? r.id : null;
  const source = typeof r.source === "string" ? (r.source as ProjectTemplateSource) : null;
  if (!id || !source) return null;
  return {
    id,
    source,
    ownerUserId: typeof r.ownerUserId === "string" ? r.ownerUserId : undefined,
    name: typeof r.name === "string" ? r.name : "",
    description: typeof r.description === "string" ? r.description : undefined,
    snapshotVersion: typeof r.snapshotVersion === "number" ? r.snapshotVersion : 1,
    snapshot: r.snapshot,
    createdAt: typeof r.createdAt === "string" ? r.createdAt : "",
    updatedAt: typeof r.updatedAt === "string" ? r.updatedAt : "",
  };
}

export const useProjectTemplateStore = create<ProjectTemplateState>((set, get) => ({
  templates: [],
  detailsById: {},
  loading: false,
  saving: false,

  async fetchTemplates() {
    set({ loading: true });
    try {
      const client = createApiClient(API_URL);
      const { data } = await client.get<ProjectTemplateListResponse>(
        "/api/v1/project-templates",
        { token: getToken() },
      );
      const templates = Array.isArray(data?.templates)
        ? (data.templates.map(normalizeTemplate).filter(Boolean) as ProjectTemplate[])
        : [];
      set({ templates });
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to load project templates";
      toast.error(msg);
    } finally {
      set({ loading: false });
    }
  },

  async fetchDetail(id) {
    try {
      const client = createApiClient(API_URL);
      const { data } = await client.get<ProjectTemplate>(
        `/api/v1/project-templates/${id}`,
        { token: getToken() },
      );
      const normalized = normalizeTemplate(data);
      if (normalized) {
        set((state) => ({
          detailsById: { ...state.detailsById, [id]: normalized },
        }));
      }
      return normalized ?? undefined;
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to load template";
      toast.error(msg);
      return undefined;
    }
  },

  async saveAsTemplate(projectId, input) {
    set({ saving: true });
    try {
      const client = createApiClient(API_URL);
      const { data } = await client.post<ProjectTemplate>(
        `/api/v1/projects/${projectId}/save-as-template`,
        {
          name: input.name,
          description: input.description ?? "",
        },
        { token: getToken() },
      );
      const normalized = normalizeTemplate(data);
      if (normalized) {
        set((state) => ({ templates: [normalized, ...state.templates] }));
        toast.success("Saved project as template");
      }
      return normalized ?? undefined;
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to save template";
      toast.error(msg);
      return undefined;
    } finally {
      set({ saving: false });
    }
  },

  async updateTemplate(id, input) {
    set({ saving: true });
    try {
      const client = createApiClient(API_URL);
      const { data } = await client.put<ProjectTemplate>(
        `/api/v1/project-templates/${id}`,
        input,
        { token: getToken() },
      );
      const normalized = normalizeTemplate(data);
      if (normalized) {
        set((state) => ({
          templates: state.templates.map((t) => (t.id === id ? normalized : t)),
          detailsById: { ...state.detailsById, [id]: normalized },
        }));
        toast.success("Template updated");
      }
      return normalized ?? undefined;
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to update template";
      toast.error(msg);
      return undefined;
    } finally {
      set({ saving: false });
    }
  },

  async deleteTemplate(id) {
    try {
      const client = createApiClient(API_URL);
      await client.delete<unknown>(
        `/api/v1/project-templates/${id}`,
        { token: getToken() },
      );
      set((state) => ({
        templates: state.templates.filter((t) => t.id !== id),
        detailsById: Object.fromEntries(
          Object.entries(state.detailsById).filter(([k]) => k !== id),
        ),
      }));
      toast.success("Template deleted");
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to delete template";
      toast.error(msg);
    }
  },
}));
