"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface DocsPage {
  id: string;
  spaceId: string;
  parentId?: string | null;
  title: string;
  content: string;
  contentText: string;
  path: string;
  sortOrder: number;
  isTemplate: boolean;
  templateCategory?: string;
  isSystem: boolean;
  isPinned: boolean;
  createdBy?: string | null;
  updatedBy?: string | null;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string | null;
  templateSource?: "system" | "custom" | null;
  previewSnippet?: string | null;
  canEdit?: boolean;
  canDelete?: boolean;
  canDuplicate?: boolean;
  canUse?: boolean;
}

export interface DocsTemplateFilters {
  query?: string;
  category?: string;
  source?: "system" | "custom" | "all";
}

export interface DocsPageTreeNode extends DocsPage {
  children: DocsPageTreeNode[];
}

export interface DocsVersion {
  id: string;
  pageId: string;
  versionNumber: number;
  name: string;
  content: string;
  createdBy?: string | null;
  createdAt: string;
}

export interface DocsComment {
  id: string;
  pageId: string;
  anchorBlockId?: string | null;
  parentCommentId?: string | null;
  body: string;
  mentions: string;
  resolvedAt?: string | null;
  createdBy?: string | null;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string | null;
}

export interface DocsFavorite {
  pageId: string;
  userId: string;
  createdAt: string;
}

export interface DocsRecentAccess {
  pageId: string;
  userId: string;
  accessedAt: string;
}

interface DocsState {
  projectId: string | null;
  tree: DocsPageTreeNode[];
  currentPage: DocsPage | null;
  comments: DocsComment[];
  versions: DocsVersion[];
  templates: DocsPage[];
  favorites: DocsFavorite[];
  recentAccess: DocsRecentAccess[];
  loading: boolean;
  saving: boolean;
  error: string | null;
  setProjectId: (projectId: string | null) => void;
  resolvePageContext: (pageId: string) => Promise<string | null>;
  fetchTree: (projectId: string) => Promise<void>;
  refreshActiveProjectTree: () => Promise<void>;
  fetchPage: (projectId: string, pageId: string) => Promise<void>;
  fetchPageWorkspace: (projectId: string, pageId: string) => Promise<void>;
  createPage: (input: {
    projectId: string;
    title: string;
    parentId?: string | null;
    content?: string;
  }) => Promise<DocsPage | null>;
  updatePage: (input: {
    projectId: string;
    pageId: string;
    title: string;
    content: string;
    contentText: string;
    expectedUpdatedAt?: string;
    templateCategory?: string;
  }) => Promise<DocsPage | null>;
  movePage: (input: {
    projectId: string;
    pageId: string;
    parentId?: string | null;
    sortOrder: number;
  }) => Promise<void>;
  fetchVersions: (projectId: string, pageId: string) => Promise<void>;
  createVersion: (input: { projectId: string; pageId: string; name: string }) => Promise<void>;
  restoreVersion: (input: { projectId: string; pageId: string; versionId: string }) => Promise<void>;
  fetchComments: (projectId: string, pageId: string) => Promise<void>;
  refreshActivePageComments: () => Promise<void>;
  createComment: (input: {
    projectId: string;
    pageId: string;
    body: string;
    anchorBlockId?: string | null;
    parentCommentId?: string | null;
    mentions?: string;
  }) => Promise<void>;
  setCommentResolved: (input: {
    projectId: string;
    pageId: string;
    commentId: string;
    resolved: boolean;
  }) => Promise<void>;
  deleteComment: (input: {
    projectId: string;
    pageId: string;
    commentId: string;
  }) => Promise<void>;
  fetchTemplates: (projectId: string, filters?: DocsTemplateFilters) => Promise<void>;
  createPageFromTemplate: (input: {
    projectId: string;
    templateId: string;
    title: string;
    parentId?: string | null;
  }) => Promise<DocsPage | null>;
  createTemplate: (input: {
    projectId: string;
    title: string;
    category: string;
    content?: string;
  }) => Promise<DocsPage | null>;
  createTemplateFromPage: (input: {
    projectId: string;
    pageId: string;
    name: string;
    category: string;
  }) => Promise<DocsPage | null>;
  duplicateTemplate: (input: {
    projectId: string;
    templateId: string;
    name: string;
    category: string;
  }) => Promise<DocsPage | null>;
  updateTemplate: (input: {
    projectId: string;
    templateId: string;
    title: string;
    content: string;
    contentText: string;
    expectedUpdatedAt?: string;
    templateCategory?: string;
  }) => Promise<DocsPage | null>;
  deleteTemplate: (input: {
    projectId: string;
    templateId: string;
  }) => Promise<void>;
  fetchFavorites: (projectId: string) => Promise<void>;
  toggleFavorite: (input: {
    projectId: string;
    pageId: string;
    favorite: boolean;
  }) => Promise<void>;
  fetchRecentAccess: (projectId: string) => Promise<void>;
  togglePinned: (input: {
    projectId: string;
    pageId: string;
    pinned: boolean;
  }) => Promise<void>;
}

function getApi() {
  const token = useAuthStore.getState().accessToken;
  if (!token) {
    throw new Error("missing access token");
  }
  return {
    token,
    api: createApiClient(API_URL),
  };
}

function toDocsPage(raw: Record<string, unknown>): DocsPage {
  return {
    id: String(raw.id ?? ""),
    spaceId: String(raw.spaceId ?? ""),
    parentId: typeof raw.parentId === "string" ? raw.parentId : null,
    title: String(raw.title ?? ""),
    content: String(raw.content ?? "[]"),
    contentText: String(raw.contentText ?? ""),
    path: String(raw.path ?? ""),
    sortOrder: Number(raw.sortOrder ?? 0),
    isTemplate: Boolean(raw.isTemplate),
    templateCategory:
      typeof raw.templateCategory === "string" ? raw.templateCategory : undefined,
    isSystem: Boolean(raw.isSystem),
    isPinned: Boolean(raw.isPinned),
    createdBy: typeof raw.createdBy === "string" ? raw.createdBy : null,
    updatedBy: typeof raw.updatedBy === "string" ? raw.updatedBy : null,
    createdAt: String(raw.createdAt ?? new Date().toISOString()),
    updatedAt: String(raw.updatedAt ?? new Date().toISOString()),
    deletedAt: typeof raw.deletedAt === "string" ? raw.deletedAt : null,
    templateSource:
      raw.templateSource === "system" || raw.templateSource === "custom"
        ? raw.templateSource
        : Boolean(raw.isTemplate)
          ? (Boolean(raw.isSystem) ? "system" : "custom")
          : null,
    previewSnippet:
      typeof raw.previewSnippet === "string" ? raw.previewSnippet : null,
    canEdit:
      typeof raw.canEdit === "boolean"
        ? raw.canEdit
        : Boolean(raw.isTemplate) && !Boolean(raw.isSystem),
    canDelete:
      typeof raw.canDelete === "boolean"
        ? raw.canDelete
        : Boolean(raw.isTemplate) && !Boolean(raw.isSystem),
    canDuplicate:
      typeof raw.canDuplicate === "boolean"
        ? raw.canDuplicate
        : Boolean(raw.isTemplate),
    canUse:
      typeof raw.canUse === "boolean" ? raw.canUse : Boolean(raw.isTemplate),
  };
}

function toDocsTreeNode(raw: Record<string, unknown>): DocsPageTreeNode {
  return {
    ...toDocsPage(raw),
    children: Array.isArray(raw.children)
      ? raw.children
          .filter((child): child is Record<string, unknown> => Boolean(child && typeof child === "object"))
          .map(toDocsTreeNode)
      : [],
  };
}

function toDocsVersion(raw: Record<string, unknown>): DocsVersion {
  return {
    id: String(raw.id ?? ""),
    pageId: String(raw.pageId ?? ""),
    versionNumber: Number(raw.versionNumber ?? 0),
    name: String(raw.name ?? ""),
    content: String(raw.content ?? "[]"),
    createdBy: typeof raw.createdBy === "string" ? raw.createdBy : null,
    createdAt: String(raw.createdAt ?? new Date().toISOString()),
  };
}

function toDocsComment(raw: Record<string, unknown>): DocsComment {
  return {
    id: String(raw.id ?? ""),
    pageId: String(raw.pageId ?? ""),
    anchorBlockId:
      typeof raw.anchorBlockId === "string" ? raw.anchorBlockId : null,
    parentCommentId:
      typeof raw.parentCommentId === "string" ? raw.parentCommentId : null,
    body: String(raw.body ?? ""),
    mentions: String(raw.mentions ?? "[]"),
    resolvedAt: typeof raw.resolvedAt === "string" ? raw.resolvedAt : null,
    createdBy: typeof raw.createdBy === "string" ? raw.createdBy : null,
    createdAt: String(raw.createdAt ?? new Date().toISOString()),
    updatedAt: String(raw.updatedAt ?? new Date().toISOString()),
    deletedAt: typeof raw.deletedAt === "string" ? raw.deletedAt : null,
  };
}

function toDocsFavorite(raw: Record<string, unknown>): DocsFavorite {
  return {
    pageId: String(raw.pageId ?? ""),
    userId: String(raw.userId ?? ""),
    createdAt: String(raw.createdAt ?? new Date().toISOString()),
  };
}

function toDocsRecentAccess(raw: Record<string, unknown>): DocsRecentAccess {
  return {
    pageId: String(raw.pageId ?? ""),
    userId: String(raw.userId ?? ""),
    accessedAt: String(raw.accessedAt ?? new Date().toISOString()),
  };
}

export function flattenDocsTree(nodes: DocsPageTreeNode[]): DocsPage[] {
  return nodes.flatMap((node) => [node, ...flattenDocsTree(node.children)]);
}

export function findDocsPageById(
  nodes: DocsPageTreeNode[],
  pageId: string
): DocsPage | null {
  for (const node of nodes) {
    if (node.id === pageId) {
      return node;
    }
    const child = findDocsPageById(node.children, pageId);
    if (child) {
      return child;
    }
  }
  return null;
}

export const useDocsStore = create<DocsState>()((set, get) => ({
  projectId: null,
  tree: [],
  currentPage: null,
  comments: [],
  versions: [],
  templates: [],
  favorites: [],
  recentAccess: [],
  loading: false,
  saving: false,
  error: null,

  setProjectId: (projectId) => set({ projectId }),

  resolvePageContext: async (pageId) => {
    const { api, token } = getApi();
    set({ loading: true, error: null });
    try {
      const { data } = await api.get<{
        projectId: string;
        page: Record<string, unknown>;
      }>(`/api/v1/wiki/pages/${pageId}`, { token });
      const projectId = String(data.projectId ?? "");
      const page = toDocsPage(data.page ?? {});
      set({ projectId, currentPage: page });
      return projectId || null;
    } catch (error) {
      set({
        error:
          error instanceof Error ? error.message : "Failed to resolve doc page",
      });
      return null;
    } finally {
      set({ loading: false });
    }
  },

  fetchTree: async (projectId) => {
    const { api, token } = getApi();
    set({ loading: true, error: null, projectId });
    try {
      const { data } = await api.get<Record<string, unknown>[]>(
        `/api/v1/projects/${projectId}/wiki/pages`,
        { token }
      );
      set({ tree: data.map(toDocsTreeNode) });
    } catch (error) {
      set({ error: error instanceof Error ? error.message : "Failed to load docs tree" });
    } finally {
      set({ loading: false });
    }
  },

  refreshActiveProjectTree: async () => {
    const { projectId } = get();
    if (!projectId) return;
    await get().fetchTree(projectId);
  },

  fetchPage: async (projectId, pageId) => {
    const { api, token } = getApi();
    set({ loading: true, error: null, projectId });
    try {
      const { data } = await api.get<Record<string, unknown>>(
        `/api/v1/projects/${projectId}/wiki/pages/${pageId}`,
        { token }
      );
      set({ currentPage: toDocsPage(data) });
    } catch (error) {
      set({ error: error instanceof Error ? error.message : "Failed to load doc page" });
    } finally {
      set({ loading: false });
    }
  },

  fetchPageWorkspace: async (projectId, pageId) => {
    await Promise.all([
      get().fetchPage(projectId, pageId),
      get().fetchComments(projectId, pageId),
      get().fetchVersions(projectId, pageId),
      get().fetchTemplates(projectId),
      get().fetchFavorites(projectId),
      get().fetchRecentAccess(projectId),
    ]);
  },

  createPage: async ({ projectId, title, parentId, content }) => {
    const { api, token } = getApi();
    const { data } = await api.post<Record<string, unknown>>(
      `/api/v1/projects/${projectId}/wiki/pages`,
      {
        title,
        parentId: parentId ?? undefined,
        content: content ?? "[]",
      },
      { token }
    );
    const page = toDocsPage(data);
    await get().fetchTree(projectId);
    return page;
  },

  updatePage: async ({
    projectId,
    pageId,
    title,
    content,
    contentText,
    expectedUpdatedAt,
    templateCategory,
  }) => {
    const { api, token } = getApi();
    set({ saving: true });
    try {
      const { data } = await api.put<Record<string, unknown>>(
        `/api/v1/projects/${projectId}/wiki/pages/${pageId}`,
        {
          title,
          content,
          contentText,
          expectedUpdatedAt,
          templateCategory,
        },
        { token }
      );
      const page = toDocsPage(data);
      set({ currentPage: page });
      await get().fetchTree(projectId);
      return page;
    } finally {
      set({ saving: false });
    }
  },

  movePage: async ({ projectId, pageId, parentId, sortOrder }) => {
    const { api, token } = getApi();
    await api.patch(
      `/api/v1/projects/${projectId}/wiki/pages/${pageId}/move`,
      { parentId: parentId ?? undefined, sortOrder },
      { token }
    );
    await get().fetchTree(projectId);
  },

  fetchVersions: async (projectId, pageId) => {
    const { api, token } = getApi();
    const { data } = await api.get<Record<string, unknown>[]>(
      `/api/v1/projects/${projectId}/wiki/pages/${pageId}/versions`,
      { token }
    );
    set({ versions: data.map(toDocsVersion) });
  },

  createVersion: async ({ projectId, pageId, name }) => {
    const { api, token } = getApi();
    await api.post(
      `/api/v1/projects/${projectId}/wiki/pages/${pageId}/versions`,
      { name },
      { token }
    );
    await get().fetchVersions(projectId, pageId);
  },

  restoreVersion: async ({ projectId, pageId, versionId }) => {
    const { api, token } = getApi();
    await api.post(
      `/api/v1/projects/${projectId}/wiki/pages/${pageId}/versions/${versionId}/restore`,
      {},
      { token }
    );
    await get().fetchPageWorkspace(projectId, pageId);
  },

  fetchComments: async (projectId, pageId) => {
    const { api, token } = getApi();
    const { data } = await api.get<Record<string, unknown>[]>(
      `/api/v1/projects/${projectId}/wiki/pages/${pageId}/comments`,
      { token }
    );
    set({ comments: data.map(toDocsComment) });
  },

  refreshActivePageComments: async () => {
    const { currentPage, projectId } = get();
    if (!currentPage || !projectId) return;
    await get().fetchComments(projectId, currentPage.id);
  },

  createComment: async ({
    projectId,
    pageId,
    body,
    anchorBlockId,
    parentCommentId,
    mentions,
  }) => {
    const { api, token } = getApi();
    await api.post(
      `/api/v1/projects/${projectId}/wiki/pages/${pageId}/comments`,
      {
        body,
        anchorBlockId: anchorBlockId ?? undefined,
        parentCommentId: parentCommentId ?? undefined,
        mentions: mentions ?? "[]",
      },
      { token }
    );
    await get().fetchComments(projectId, pageId);
  },

  setCommentResolved: async ({ projectId, pageId, commentId, resolved }) => {
    const { api, token } = getApi();
    await api.patch(
      `/api/v1/projects/${projectId}/wiki/pages/${pageId}/comments/${commentId}`,
      { resolved },
      { token }
    );
    await get().fetchComments(projectId, pageId);
  },

  deleteComment: async ({ projectId, pageId, commentId }) => {
    const { api, token } = getApi();
    await api.delete(
      `/api/v1/projects/${projectId}/wiki/pages/${pageId}/comments/${commentId}`,
      { token }
    );
    await get().fetchComments(projectId, pageId);
  },

  fetchTemplates: async (projectId, filters) => {
    const { api, token } = getApi();
    const params = new URLSearchParams();
    if (filters?.query?.trim()) {
      params.set("q", filters.query.trim());
    }
    if (filters?.category?.trim()) {
      params.set("category", filters.category.trim());
    }
    if (filters?.source && filters.source !== "all") {
      params.set("source", filters.source);
    }
    const { data } = await api.get<Record<string, unknown>[]>(
      `/api/v1/projects/${projectId}/wiki/templates${params.size ? `?${params.toString()}` : ""}`,
      { token }
    );
    set({ templates: data.map(toDocsPage) });
  },

  createTemplate: async ({ projectId, title, category, content }) => {
    const { api, token } = getApi();
    const { data } = await api.post<Record<string, unknown>>(
      `/api/v1/projects/${projectId}/wiki/templates`,
      {
        title,
        category,
        content: content ?? "[]",
      },
      { token }
    );
    const template = toDocsPage(data);
    await Promise.all([get().fetchTemplates(projectId), get().fetchTree(projectId)]);
    return template;
  },

  createPageFromTemplate: async ({ projectId, templateId, title, parentId }) => {
    const { api, token } = getApi();
    const { data } = await api.post<Record<string, unknown>>(
      `/api/v1/projects/${projectId}/wiki/pages/from-template`,
      {
        templateId,
        title,
        parentId: parentId ?? undefined,
      },
      { token }
    );
    const page = toDocsPage(data);
    await get().fetchTree(projectId);
    return page;
  },

  createTemplateFromPage: async ({ projectId, pageId, name, category }) => {
    const { api, token } = getApi();
    const { data } = await api.post<Record<string, unknown>>(
      `/api/v1/projects/${projectId}/wiki/pages/${pageId}/templates`,
      { name, category },
      { token }
    );
    await Promise.all([get().fetchTemplates(projectId), get().fetchTree(projectId)]);
    return toDocsPage(data);
  },

  duplicateTemplate: async ({ projectId, templateId, name, category }) => {
    return get().createTemplateFromPage({
      projectId,
      pageId: templateId,
      name,
      category,
    });
  },

  updateTemplate: async ({
    projectId,
    templateId,
    title,
    content,
    contentText,
    expectedUpdatedAt,
    templateCategory,
  }) => {
    const updated = await get().updatePage({
      projectId,
      pageId: templateId,
      title,
      content,
      contentText,
      expectedUpdatedAt,
      templateCategory,
    });
    await get().fetchTemplates(projectId);
    return updated;
  },

  deleteTemplate: async ({ projectId, templateId }) => {
    const { api, token } = getApi();
    await api.delete(`/api/v1/projects/${projectId}/wiki/pages/${templateId}`, {
      token,
    });
    if (get().currentPage?.id === templateId) {
      set({ currentPage: null });
    }
    await Promise.all([get().fetchTemplates(projectId), get().fetchTree(projectId)]);
  },

  fetchFavorites: async (projectId) => {
    const { api, token } = getApi();
    const { data } = await api.get<Record<string, unknown>[]>(
      `/api/v1/projects/${projectId}/wiki/favorites`,
      { token }
    );
    set({ favorites: data.map(toDocsFavorite) });
  },

  toggleFavorite: async ({ projectId, pageId, favorite }) => {
    const { api, token } = getApi();
    await api.put(
      `/api/v1/projects/${projectId}/wiki/pages/${pageId}/favorite`,
      { favorite },
      { token }
    );
    await get().fetchFavorites(projectId);
  },

  fetchRecentAccess: async (projectId) => {
    const { api, token } = getApi();
    const { data } = await api.get<Record<string, unknown>[]>(
      `/api/v1/projects/${projectId}/wiki/recent`,
      { token }
    );
    set({ recentAccess: data.map(toDocsRecentAccess) });
  },

  togglePinned: async ({ projectId, pageId, pinned }) => {
    const { api, token } = getApi();
    await api.put(
      `/api/v1/projects/${projectId}/wiki/pages/${pageId}/pin`,
      { pinned },
      { token }
    );
    await get().fetchTree(projectId);
  },
}));
