"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";
import { getPreferredLocale } from "./locale-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type KnowledgeAssetKind = "wiki_page" | "ingested_file" | "template";
export type IngestStatus = "pending" | "processing" | "ready" | "failed";

export interface KnowledgeAsset {
  id: string;
  projectId: string;
  kind: KnowledgeAssetKind;
  // wiki_page + template fields
  spaceId?: string | null;
  parentId?: string | null;
  path?: string | null;
  sortOrder?: number;
  contentJson?: string | null; // BlockNote JSON
  contentText?: string | null;
  templateCategory?: string | null;
  isSystemTemplate?: boolean;
  // ingested_file fields
  fileRef?: string | null;
  fileSize?: number | null;
  mimeType?: string | null;
  ingestStatus?: IngestStatus | null;
  ingestChunkCount?: number | null;
  // shared
  title: string;
  isPinned: boolean;
  ownerId?: string | null;
  createdBy?: string | null;
  updatedBy?: string | null;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string | null;
  version: number;
  // computed
  templateSource?: "system" | "custom" | null;
  previewSnippet?: string | null;
  canEdit?: boolean;
  canDelete?: boolean;
  canDuplicate?: boolean;
  canUse?: boolean;
  // materialize relationship
  materializedFromId?: string | null;
  sourceUpdatedSinceMaterialize?: boolean;
}

export interface KnowledgeAssetTreeNode extends KnowledgeAsset {
  children: KnowledgeAssetTreeNode[];
}

export interface AssetVersion {
  id: string;
  assetId: string;
  versionNumber: number;
  name: string;
  kindSnapshot: string;
  contentJson?: string | null;
  fileRef?: string | null;
  createdBy?: string | null;
  createdAt: string;
}

export interface AssetComment {
  id: string;
  assetId: string;
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

export interface KnowledgeSearchResult {
  id: string;
  kind: KnowledgeAssetKind;
  title: string;
  snippet: string;
  updatedAt: string;
  score: number;
}

export interface KnowledgeSearchResponse {
  items: KnowledgeSearchResult[];
  nextCursor?: string | null;
}

// ---------------------------------------------------------------------------
// Legacy type aliases (for backward-compat during migration)
// ---------------------------------------------------------------------------

/** @deprecated Use KnowledgeAsset */
export type DocsPage = KnowledgeAsset;
/** @deprecated Use KnowledgeAssetTreeNode */
export type DocsPageTreeNode = KnowledgeAssetTreeNode;
/** @deprecated Use AssetVersion */
export type DocsVersion = AssetVersion;
/** @deprecated Use AssetComment */
export type DocsComment = AssetComment;

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

export interface DocsTemplateFilters {
  query?: string;
  category?: string;
  source?: "system" | "custom" | "all";
}

// ---------------------------------------------------------------------------
// Store state interface
// ---------------------------------------------------------------------------

export interface KnowledgeState {
  projectId: string | null;
  tree: KnowledgeAssetTreeNode[];
  ingestedFiles: KnowledgeAsset[];
  templates: KnowledgeAsset[];
  currentAsset: KnowledgeAsset | null;
  comments: AssetComment[];
  versions: AssetVersion[];
  favorites: { assetId: string; userId: string; createdAt: string }[];
  recentAccess: { assetId: string; userId: string; accessedAt: string }[];
  searchResults: KnowledgeSearchResponse | null;
  loading: boolean;
  saving: boolean;
  uploading: boolean;
  error: string | null;

  setProjectId: (projectId: string | null) => void;

  // Wiki page actions
  resolvePageContext: (pageId: string) => Promise<string | null>;
  fetchTree: (projectId: string) => Promise<void>;
  refreshActiveProjectTree: () => Promise<void>;
  fetchAsset: (projectId: string, assetId: string) => Promise<void>;
  fetchPageWorkspace: (projectId: string, pageId: string) => Promise<void>;
  createPage: (input: {
    projectId: string;
    title: string;
    parentId?: string | null;
    content?: string;
  }) => Promise<KnowledgeAsset | null>;
  updatePage: (input: {
    projectId: string;
    pageId: string;
    title: string;
    content: string;
    contentText: string;
    expectedUpdatedAt?: string;
    templateCategory?: string;
  }) => Promise<KnowledgeAsset | null>;
  movePage: (input: {
    projectId: string;
    pageId: string;
    parentId?: string | null;
    sortOrder: number;
  }) => Promise<void>;
  deletePage: (input: { projectId: string; pageId: string }) => Promise<void>;
  restoreAsset: (input: { projectId: string; assetId: string }) => Promise<void>;

  // Versions
  fetchVersions: (projectId: string, assetId: string) => Promise<void>;
  createVersion: (input: { projectId: string; assetId: string; name: string }) => Promise<void>;
  restoreVersion: (input: {
    projectId: string;
    assetId: string;
    versionId: string;
  }) => Promise<void>;

  // Comments
  fetchComments: (projectId: string, assetId: string) => Promise<void>;
  refreshActiveAssetComments: () => Promise<void>;
  createComment: (input: {
    projectId: string;
    assetId: string;
    body: string;
    anchorBlockId?: string | null;
    parentCommentId?: string | null;
    mentions?: string;
  }) => Promise<void>;
  setCommentResolved: (input: {
    projectId: string;
    assetId: string;
    commentId: string;
    resolved: boolean;
  }) => Promise<void>;
  deleteComment: (input: {
    projectId: string;
    assetId: string;
    commentId: string;
  }) => Promise<void>;

  // Templates
  fetchTemplates: (projectId: string, filters?: DocsTemplateFilters) => Promise<void>;
  createPageFromTemplate: (input: {
    projectId: string;
    templateId: string;
    title: string;
    parentId?: string | null;
  }) => Promise<KnowledgeAsset | null>;
  createTemplate: (input: {
    projectId: string;
    title: string;
    category: string;
    content?: string;
  }) => Promise<KnowledgeAsset | null>;
  createTemplateFromPage: (input: {
    projectId: string;
    pageId: string;
    name: string;
    category: string;
  }) => Promise<KnowledgeAsset | null>;
  duplicateTemplate: (input: {
    projectId: string;
    templateId: string;
    name: string;
    category: string;
  }) => Promise<KnowledgeAsset | null>;
  updateTemplate: (input: {
    projectId: string;
    templateId: string;
    title: string;
    content: string;
    contentText: string;
    expectedUpdatedAt?: string;
    templateCategory?: string;
  }) => Promise<KnowledgeAsset | null>;
  deleteTemplate: (input: { projectId: string; templateId: string }) => Promise<void>;

  // Ingested files
  fetchIngestedFiles: (projectId: string) => Promise<void>;
  uploadFile: (projectId: string, file: File) => Promise<void>;
  reuploadFile: (projectId: string, assetId: string, file: File) => Promise<void>;
  deleteIngestedFile: (input: { projectId: string; assetId: string }) => Promise<void>;
  materializeAsWiki: (projectId: string, assetId: string) => Promise<KnowledgeAsset | null>;

  // Favorites & pinned
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

  // Search
  searchKnowledge: (
    projectId: string,
    query: string,
    opts?: { kind?: KnowledgeAssetKind; updatedAfter?: string; cursor?: string },
  ) => Promise<void>;
  clearSearch: () => void;
}

// ---------------------------------------------------------------------------
// Selectors
// ---------------------------------------------------------------------------

export function selectWikiPageTree(state: KnowledgeState): KnowledgeAssetTreeNode[] {
  return state.tree;
}

export function selectIngestedFiles(state: KnowledgeState): KnowledgeAsset[] {
  return state.ingestedFiles;
}

export function selectTemplatesByCategory(
  state: KnowledgeState,
  category?: string,
): KnowledgeAsset[] {
  if (!category) return state.templates;
  return state.templates.filter((t) => t.templateCategory === category);
}

// ---------------------------------------------------------------------------
// Helpers mapping old concepts to new
// ---------------------------------------------------------------------------

export function flattenKnowledgeTree(nodes: KnowledgeAssetTreeNode[]): KnowledgeAsset[] {
  return nodes.flatMap((node) => [node, ...flattenKnowledgeTree(node.children)]);
}

export function findKnowledgeAssetById(
  nodes: KnowledgeAssetTreeNode[],
  id: string,
): KnowledgeAsset | null {
  for (const node of nodes) {
    if (node.id === id) return node;
    const child = findKnowledgeAssetById(node.children, id);
    if (child) return child;
  }
  return null;
}

/** @deprecated Use flattenKnowledgeTree */
export const flattenDocsTree = flattenKnowledgeTree;
/** @deprecated Use findKnowledgeAssetById */
export const findDocsPageById = findKnowledgeAssetById;

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

function getApi() {
  const token = useAuthStore.getState().accessToken;
  if (!token) throw new Error("missing access token");
  return { token, api: createApiClient(API_URL) };
}

function toKnowledgeAsset(raw: Record<string, unknown>): KnowledgeAsset {
  const rawKindIsValid = ["wiki_page", "ingested_file", "template"].includes(
    String(raw.kind),
  );
  const isSystem = Boolean(raw.isSystem ?? raw.isSystemTemplate);
  const isTemplate = Boolean(raw.isTemplate);
  const kind = (rawKindIsValid
    ? String(raw.kind)
    : isTemplate
      ? "template"
      : "wiki_page") as KnowledgeAssetKind;

  return {
    id: String(raw.id ?? ""),
    projectId: String(raw.projectId ?? ""),
    kind,
    spaceId: typeof raw.spaceId === "string" ? raw.spaceId : null,
    parentId: typeof raw.parentId === "string" ? raw.parentId : null,
    path: typeof raw.path === "string" ? raw.path : null,
    sortOrder: Number(raw.sortOrder ?? 0),
    // contentJson maps both new contentJson and legacy content field
    contentJson: typeof raw.contentJson === "string"
      ? raw.contentJson
      : typeof raw.content === "string"
        ? raw.content
        : null,
    contentText: typeof raw.contentText === "string" ? raw.contentText : null,
    templateCategory: typeof raw.templateCategory === "string" ? raw.templateCategory : null,
    isSystemTemplate: isSystem,
    fileRef: typeof raw.fileRef === "string" ? raw.fileRef : null,
    fileSize: typeof raw.fileSize === "number" ? raw.fileSize : null,
    mimeType: typeof raw.mimeType === "string" ? raw.mimeType : null,
    ingestStatus: (["pending", "processing", "ready", "failed"].includes(String(raw.ingestStatus))
      ? String(raw.ingestStatus)
      : null) as IngestStatus | null,
    ingestChunkCount: typeof raw.ingestChunkCount === "number" ? raw.ingestChunkCount : null,
    title: String(raw.title ?? ""),
    isPinned: Boolean(raw.isPinned),
    ownerId: typeof raw.ownerId === "string" ? raw.ownerId : null,
    createdBy: typeof raw.createdBy === "string" ? raw.createdBy : null,
    updatedBy: typeof raw.updatedBy === "string" ? raw.updatedBy : null,
    createdAt: String(raw.createdAt ?? new Date().toISOString()),
    updatedAt: String(raw.updatedAt ?? new Date().toISOString()),
    deletedAt: typeof raw.deletedAt === "string" ? raw.deletedAt : null,
    version: Number(raw.version ?? 1),
    templateSource:
      raw.templateSource === "system" || raw.templateSource === "custom"
        ? raw.templateSource
        : isTemplate
          ? isSystem
            ? "system"
            : "custom"
          : null,
    previewSnippet: typeof raw.previewSnippet === "string" ? raw.previewSnippet : null,
    canEdit: typeof raw.canEdit === "boolean" ? raw.canEdit : isTemplate && !isSystem,
    canDelete: typeof raw.canDelete === "boolean" ? raw.canDelete : isTemplate && !isSystem,
    canDuplicate: typeof raw.canDuplicate === "boolean" ? raw.canDuplicate : isTemplate,
    canUse: typeof raw.canUse === "boolean" ? raw.canUse : isTemplate,
    materializedFromId:
      typeof raw.materializedFromId === "string" ? raw.materializedFromId : null,
    sourceUpdatedSinceMaterialize:
      typeof raw.sourceUpdatedSinceMaterialize === "boolean"
        ? raw.sourceUpdatedSinceMaterialize
        : false,
  };
}

function toTreeNode(raw: Record<string, unknown>): KnowledgeAssetTreeNode {
  return {
    ...toKnowledgeAsset(raw),
    children: Array.isArray(raw.children)
      ? raw.children
          .filter((child): child is Record<string, unknown> => Boolean(child && typeof child === "object"))
          .map(toTreeNode)
      : [],
  };
}

function toAssetVersion(raw: Record<string, unknown>): AssetVersion {
  return {
    id: String(raw.id ?? ""),
    assetId: String(raw.assetId ?? raw.pageId ?? ""),
    versionNumber: Number(raw.versionNumber ?? 0),
    name: String(raw.name ?? ""),
    kindSnapshot: String(raw.kindSnapshot ?? "wiki_page"),
    contentJson: typeof raw.contentJson === "string"
      ? raw.contentJson
      : typeof raw.content === "string"
        ? raw.content
        : null,
    fileRef: typeof raw.fileRef === "string" ? raw.fileRef : null,
    createdBy: typeof raw.createdBy === "string" ? raw.createdBy : null,
    createdAt: String(raw.createdAt ?? new Date().toISOString()),
  };
}

function toAssetComment(raw: Record<string, unknown>): AssetComment {
  return {
    id: String(raw.id ?? ""),
    assetId: String(raw.assetId ?? raw.pageId ?? ""),
    anchorBlockId: typeof raw.anchorBlockId === "string" ? raw.anchorBlockId : null,
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

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

export const useKnowledgeStore = create<KnowledgeState>()((set, get) => ({
  projectId: null,
  tree: [],
  ingestedFiles: [],
  templates: [],
  currentAsset: null,
  comments: [],
  versions: [],
  favorites: [],
  recentAccess: [],
  searchResults: null,
  loading: false,
  saving: false,
  uploading: false,
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
      const asset = toKnowledgeAsset(data.page ?? {});
      set({ projectId, currentAsset: asset });
      return projectId || null;
    } catch (error) {
      const locale = getPreferredLocale();
      set({
        error:
          error instanceof Error ? error.message : (locale === "zh-CN" ? "解析文档页面失败" : "Failed to resolve doc page"),
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
        `/api/v1/projects/${projectId}/knowledge/assets/tree`,
        { token },
      );
      set({ tree: data.map(toTreeNode) });
    } catch (error) {
      const locale = getPreferredLocale();
      set({ error: error instanceof Error ? error.message : (locale === "zh-CN" ? "加载知识树失败" : "Failed to load knowledge tree") });
    } finally {
      set({ loading: false });
    }
  },

  refreshActiveProjectTree: async () => {
    const { projectId } = get();
    if (!projectId) return;
    await get().fetchTree(projectId);
  },

  fetchAsset: async (projectId, assetId) => {
    const { api, token } = getApi();
    set({ loading: true, error: null, projectId });
    try {
      const { data } = await api.get<Record<string, unknown>>(
        `/api/v1/projects/${projectId}/knowledge/assets/${assetId}`,
        { token },
      );
      set({ currentAsset: toKnowledgeAsset(data) });
    } catch (error) {
      const locale = getPreferredLocale();
      set({ error: error instanceof Error ? error.message : (locale === "zh-CN" ? "加载资产失败" : "Failed to load asset") });
    } finally {
      set({ loading: false });
    }
  },

  fetchPageWorkspace: async (projectId, pageId) => {
    await Promise.all([
      get().fetchAsset(projectId, pageId),
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
      `/api/v1/projects/${projectId}/knowledge/assets`,
      {
        kind: "wiki_page",
        title,
        parentId: parentId ?? undefined,
        contentJson: content ?? "[]",
      },
      { token },
    );
    const asset = toKnowledgeAsset(data);
    await get().fetchTree(projectId);
    return asset;
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
        `/api/v1/projects/${projectId}/knowledge/assets/${pageId}`,
        {
          title,
          contentJson: content,
          contentText,
          expectedUpdatedAt,
          templateCategory,
        },
        { token },
      );
      const asset = toKnowledgeAsset(data);
      set({ currentAsset: asset });
      await get().fetchTree(projectId);
      return asset;
    } finally {
      set({ saving: false });
    }
  },

  movePage: async ({ projectId, pageId, parentId, sortOrder }) => {
    const { api, token } = getApi();
    await api.patch(
      `/api/v1/projects/${projectId}/knowledge/assets/${pageId}/move`,
      { parentId: parentId ?? undefined, sortOrder },
      { token },
    );
    await get().fetchTree(projectId);
  },

  deletePage: async ({ projectId, pageId }) => {
    const { api, token } = getApi();
    await api.delete(`/api/v1/projects/${projectId}/knowledge/assets/${pageId}`, { token });
    if (get().currentAsset?.id === pageId) {
      set({ currentAsset: null });
    }
    await get().fetchTree(projectId);
  },

  restoreAsset: async ({ projectId, assetId }) => {
    const { api, token } = getApi();
    await api.post(
      `/api/v1/projects/${projectId}/knowledge/assets/${assetId}/restore`,
      {},
      { token },
    );
    await get().fetchTree(projectId);
  },

  fetchVersions: async (projectId, assetId) => {
    const { api, token } = getApi();
    const { data } = await api.get<Record<string, unknown>[]>(
      `/api/v1/projects/${projectId}/knowledge/assets/${assetId}/versions`,
      { token },
    );
    set({ versions: data.map(toAssetVersion) });
  },

  createVersion: async ({ projectId, assetId, name }) => {
    const { api, token } = getApi();
    await api.post(
      `/api/v1/projects/${projectId}/knowledge/assets/${assetId}/versions`,
      { name },
      { token },
    );
    await get().fetchVersions(projectId, assetId);
  },

  restoreVersion: async ({ projectId, assetId, versionId }) => {
    const { api, token } = getApi();
    await api.post(
      `/api/v1/projects/${projectId}/knowledge/assets/${assetId}/versions/${versionId}/restore`,
      {},
      { token },
    );
    await get().fetchPageWorkspace(projectId, assetId);
  },

  fetchComments: async (projectId, assetId) => {
    const { api, token } = getApi();
    const { data } = await api.get<Record<string, unknown>[]>(
      `/api/v1/projects/${projectId}/knowledge/assets/${assetId}/comments`,
      { token },
    );
    set({ comments: data.map(toAssetComment) });
  },

  refreshActiveAssetComments: async () => {
    const { currentAsset, projectId } = get();
    if (!currentAsset || !projectId) return;
    await get().fetchComments(projectId, currentAsset.id);
  },

  createComment: async ({
    projectId,
    assetId,
    body,
    anchorBlockId,
    parentCommentId,
    mentions,
  }) => {
    const { api, token } = getApi();
    await api.post(
      `/api/v1/projects/${projectId}/knowledge/assets/${assetId}/comments`,
      {
        body,
        anchorBlockId: anchorBlockId ?? undefined,
        parentCommentId: parentCommentId ?? undefined,
        mentions: mentions ?? "[]",
      },
      { token },
    );
    await get().fetchComments(projectId, assetId);
  },

  setCommentResolved: async ({ projectId, assetId, commentId, resolved }) => {
    const { api, token } = getApi();
    await api.patch(
      `/api/v1/projects/${projectId}/knowledge/assets/${assetId}/comments/${commentId}`,
      { resolved },
      { token },
    );
    await get().fetchComments(projectId, assetId);
  },

  deleteComment: async ({ projectId, assetId, commentId }) => {
    const { api, token } = getApi();
    await api.delete(
      `/api/v1/projects/${projectId}/knowledge/assets/${assetId}/comments/${commentId}`,
      { token },
    );
    await get().fetchComments(projectId, assetId);
  },

  fetchTemplates: async (projectId, filters) => {
    const { api, token } = getApi();
    const params = new URLSearchParams();
    params.set("kind", "template");
    if (filters?.query?.trim()) params.set("q", filters.query.trim());
    if (filters?.category?.trim()) params.set("category", filters.category.trim());
    if (filters?.source && filters.source !== "all") params.set("source", filters.source);
    const { data } = await api.get<Record<string, unknown>[]>(
      `/api/v1/projects/${projectId}/knowledge/assets?${params.toString()}`,
      { token },
    );
    set({ templates: data.map(toKnowledgeAsset) });
  },

  createTemplate: async ({ projectId, title, category, content }) => {
    const { api, token } = getApi();
    const { data } = await api.post<Record<string, unknown>>(
      `/api/v1/projects/${projectId}/knowledge/assets`,
      {
        kind: "template",
        title,
        templateCategory: category,
        contentJson: content ?? "[]",
      },
      { token },
    );
    const asset = toKnowledgeAsset(data);
    await Promise.all([get().fetchTemplates(projectId), get().fetchTree(projectId)]);
    return asset;
  },

  createPageFromTemplate: async ({ projectId, templateId, title, parentId }) => {
    const { api, token } = getApi();
    const { data } = await api.post<Record<string, unknown>>(
      `/api/v1/projects/${projectId}/knowledge/assets`,
      {
        kind: "wiki_page",
        title,
        parentId: parentId ?? undefined,
        templateId,
      },
      { token },
    );
    const asset = toKnowledgeAsset(data);
    await get().fetchTree(projectId);
    return asset;
  },

  createTemplateFromPage: async ({ projectId, pageId, name, category }) => {
    const { api, token } = getApi();
    const { data } = await api.post<Record<string, unknown>>(
      `/api/v1/projects/${projectId}/knowledge/assets/${pageId}/versions`,
      { name, templateCategory: category, saveAsTemplate: true },
      { token },
    );
    await Promise.all([get().fetchTemplates(projectId), get().fetchTree(projectId)]);
    return toKnowledgeAsset(data);
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
    await api.delete(
      `/api/v1/projects/${projectId}/knowledge/assets/${templateId}`,
      { token },
    );
    if (get().currentAsset?.id === templateId) {
      set({ currentAsset: null });
    }
    await Promise.all([get().fetchTemplates(projectId), get().fetchTree(projectId)]);
  },

  fetchIngestedFiles: async (projectId) => {
    const { api, token } = getApi();
    set({ loading: true, error: null });
    try {
      const { data } = await api.get<Record<string, unknown>[]>(
        `/api/v1/projects/${projectId}/knowledge/assets?kind=ingested_file`,
        { token },
      );
      set({ ingestedFiles: data.map(toKnowledgeAsset) });
    } catch (error) {
      const locale = getPreferredLocale();
      set({ error: error instanceof Error ? error.message : (locale === "zh-CN" ? "加载文件失败" : "Failed to load files") });
    } finally {
      set({ loading: false });
    }
  },

  uploadFile: async (projectId, file) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    set({ uploading: true, error: null });
    try {
      const formData = new FormData();
      formData.append("file", file);
      formData.append("kind", "ingested_file");
      const res = await fetch(
        `${API_URL.replace(/\/$/, "")}/api/v1/projects/${projectId}/knowledge/assets`,
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
      await get().fetchIngestedFiles(projectId);
    } catch (error) {
      set({ error: error instanceof Error ? error.message : "Upload failed" });
      throw error;
    } finally {
      set({ uploading: false });
    }
  },

  reuploadFile: async (projectId, assetId, file) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    set({ uploading: true, error: null });
    try {
      const formData = new FormData();
      formData.append("file", file);
      const res = await fetch(
        `${API_URL.replace(/\/$/, "")}/api/v1/projects/${projectId}/knowledge/assets/${assetId}/reupload`,
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
      await get().fetchIngestedFiles(projectId);
    } catch (error) {
      set({ error: error instanceof Error ? error.message : "Re-upload failed" });
      throw error;
    } finally {
      set({ uploading: false });
    }
  },

  deleteIngestedFile: async ({ projectId, assetId }) => {
    const { api, token } = getApi();
    await api.delete(
      `/api/v1/projects/${projectId}/knowledge/assets/${assetId}`,
      { token },
    );
    await get().fetchIngestedFiles(projectId);
  },

  materializeAsWiki: async (projectId, assetId) => {
    const { api, token } = getApi();
    set({ saving: true, error: null });
    try {
      const { data } = await api.post<Record<string, unknown>>(
        `/api/v1/projects/${projectId}/knowledge/assets/${assetId}/materialize-as-wiki`,
        {},
        { token },
      );
      const asset = toKnowledgeAsset(data);
      await get().fetchTree(projectId);
      return asset;
    } catch (error) {
      set({ error: error instanceof Error ? error.message : "Materialize failed" });
      return null;
    } finally {
      set({ saving: false });
    }
  },

  fetchFavorites: async (projectId) => {
    const { api, token } = getApi();
    const { data } = await api.get<Record<string, unknown>[]>(
      `/api/v1/projects/${projectId}/wiki/favorites`,
      { token },
    );
    set({
      favorites: data.map((raw) => ({
        assetId: String(raw.assetId ?? raw.pageId ?? ""),
        userId: String(raw.userId ?? ""),
        createdAt: String(raw.createdAt ?? new Date().toISOString()),
      })),
    });
  },

  toggleFavorite: async ({ projectId, pageId, favorite }) => {
    const { api, token } = getApi();
    await api.put(
      `/api/v1/projects/${projectId}/knowledge/assets/${pageId}/favorite`,
      { favorite },
      { token },
    );
    await get().fetchFavorites(projectId);
  },

  fetchRecentAccess: async (projectId) => {
    const { api, token } = getApi();
    const { data } = await api.get<Record<string, unknown>[]>(
      `/api/v1/projects/${projectId}/wiki/recent`,
      { token },
    );
    set({
      recentAccess: data.map((raw) => ({
        assetId: String(raw.assetId ?? raw.pageId ?? ""),
        userId: String(raw.userId ?? ""),
        accessedAt: String(raw.accessedAt ?? new Date().toISOString()),
      })),
    });
  },

  togglePinned: async ({ projectId, pageId, pinned }) => {
    const { api, token } = getApi();
    await api.put(
      `/api/v1/projects/${projectId}/knowledge/assets/${pageId}/pin`,
      { pinned },
      { token },
    );
    await get().fetchTree(projectId);
  },

  searchKnowledge: async (projectId, query, opts) => {
    const { api, token } = getApi();
    set({ loading: true, error: null });
    try {
      const params = new URLSearchParams({ q: query });
      if (opts?.kind) params.set("kind", opts.kind);
      if (opts?.updatedAfter) params.set("updated_after", opts.updatedAfter);
      if (opts?.cursor) params.set("cursor", opts.cursor);
      const { data } = await api.get<KnowledgeSearchResponse>(
        `/api/v1/projects/${projectId}/knowledge/search?${params.toString()}`,
        { token },
      );
      set({ searchResults: data });
    } catch (error) {
      set({ error: error instanceof Error ? error.message : "Search failed" });
    } finally {
      set({ loading: false });
    }
  },

  clearSearch: () => set({ searchResults: null }),
}));

// ---------------------------------------------------------------------------
// Backward-compat alias
// ---------------------------------------------------------------------------

/** @deprecated Use useKnowledgeStore */
export const useDocsStore = useKnowledgeStore;
