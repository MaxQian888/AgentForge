"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

export interface ProjectDocument {
  id: string;
  projectId: string;
  fileName: string;
  fileType: string;
  fileSize: number;
  status: "pending" | "processing" | "ready" | "failed";
  chunkCount: number;
  uploadedAt: string;
}

interface DocumentState {
  documents: ProjectDocument[];
  loading: boolean;
  uploading: boolean;
  error: string | null;
  currentProjectId: string | null;

  loadDocuments: (projectId: string) => Promise<void>;
  uploadDocument: (projectId: string, file: File) => Promise<void>;
  deleteDocument: (projectId: string, documentId: string) => Promise<void>;
  clearError: () => void;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

function extractErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return "Unknown document request failure";
}

function normalizeDocument(raw: Record<string, unknown>): ProjectDocument {
  return {
    id: String(raw.id ?? ""),
    projectId: String(raw.projectId ?? ""),
    fileName: String(raw.fileName ?? ""),
    fileType: String(raw.fileType ?? ""),
    fileSize: Number(raw.fileSize ?? 0),
    status: (["pending", "processing", "ready", "failed"].includes(
      String(raw.status),
    )
      ? String(raw.status)
      : "pending") as ProjectDocument["status"],
    chunkCount: Number(raw.chunkCount ?? 0),
    uploadedAt:
      typeof raw.uploadedAt === "string"
        ? raw.uploadedAt
        : new Date().toISOString(),
  };
}

export const useDocumentStore = create<DocumentState>()((set, get) => ({
  documents: [],
  loading: false,
  uploading: false,
  error: null,
  currentProjectId: null,

  loadDocuments: async (projectId) => {
    const token = useAuthStore.getState().accessToken;
    set({
      currentProjectId: projectId,
      loading: !!token,
      error: null,
    });

    if (!token) {
      return;
    }

    const api = createApiClient(API_URL);
    try {
      const { data } = await api.get<Record<string, unknown>[]>(
        `/api/v1/projects/${projectId}/documents`,
        { token },
      );
      const documents = (data ?? []).map(normalizeDocument);
      set({ documents, loading: false });
    } catch (err) {
      set({ error: extractErrorMessage(err), loading: false });
    }
  },

  uploadDocument: async (projectId, file) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ uploading: true, error: null });

    try {
      const formData = new FormData();
      formData.append("file", file);

      const res = await fetch(
        `${API_URL.replace(/\/$/, "")}/api/v1/projects/${projectId}/documents`,
        {
          method: "POST",
          headers: { Authorization: `Bearer ${token}` },
          body: formData,
        },
      );

      if (!res.ok) {
        const body = await res.json().catch(() => null);
        const message =
          (body as { message?: string })?.message ?? `HTTP ${res.status}`;
        throw new Error(message);
      }

      set({ uploading: false });
      // Reload document list after successful upload
      await get().loadDocuments(projectId);
    } catch (err) {
      set({ uploading: false, error: extractErrorMessage(err) });
      throw err;
    }
  },

  deleteDocument: async (projectId, documentId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    const api = createApiClient(API_URL);
    try {
      await api.delete(
        `/api/v1/projects/${projectId}/documents/${documentId}`,
        { token },
      );
      // Reload document list after successful deletion
      await get().loadDocuments(projectId);
    } catch (err) {
      set({ error: extractErrorMessage(err) });
      throw err;
    }
  },

  clearError: () => {
    set({ error: null });
  },
}));
