"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";
import type { TaskStatus } from "./task-store";

export interface WorkflowTrigger {
  fromStatus: string;
  toStatus: string;
  action: string;
  config?: Record<string, unknown>;
}

export interface WorkflowConfig {
  id?: string;
  projectId: string;
  transitions: Record<string, string[]>;
  triggers: WorkflowTrigger[];
  createdAt?: string;
  updatedAt?: string;
}

export interface WorkflowActivityEntry {
  taskId: string;
  action: string;
  from: string;
  to: string;
  timestamp: string;
  config?: Record<string, unknown>;
}

// --- DAG Workflow Types ---

export interface WorkflowNodeData {
  id: string;
  type: string;
  label: string;
  position: { x: number; y: number };
  config?: Record<string, unknown>;
}

export interface WorkflowEdgeData {
  id: string;
  source: string;
  target: string;
  condition?: string;
  label?: string;
}

export interface WorkflowDefinition {
  id: string;
  projectId: string;
  name: string;
  description: string;
  status: string;
  category: string;
  nodes: WorkflowNodeData[];
  edges: WorkflowEdgeData[];
  templateVars?: Record<string, unknown>;
  version: number;
  sourceId?: string;
  createdAt: string;
  updatedAt: string;
}

export interface WorkflowExecution {
  id: string;
  workflowId: string;
  projectId: string;
  taskId?: string;
  status: string;
  currentNodes: string[];
  errorMessage?: string;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
  updatedAt?: string;
}

export interface WorkflowNodeExecution {
  id: string;
  executionId: string;
  nodeId: string;
  status: string;
  result?: unknown;
  errorMessage?: string;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
}

export interface WorkflowPendingReview {
  id: string;
  executionId: string;
  nodeId: string;
  projectId: string;
  reviewerId?: string;
  prompt: string;
  context?: Record<string, unknown>;
  decision: string;
  comment: string;
  createdAt: string;
  resolvedAt?: string;
}

const MAX_WORKFLOW_ACTIVITY_ENTRIES = 10;

interface WorkflowState {
  config: WorkflowConfig | null;
  loading: boolean;
  saving: boolean;
  error: string | null;
  recentActivityByProject: Record<string, WorkflowActivityEntry[]>;
  fetchWorkflow: (projectId: string) => Promise<void>;
  updateWorkflow: (
    projectId: string,
    config: {
      transitions: Record<string, string[]>;
      triggers: WorkflowTrigger[];
    }
  ) => Promise<boolean>;
  appendActivity: (
    projectId: string,
    entry: Omit<WorkflowActivityEntry, "timestamp"> & { timestamp?: string }
  ) => void;
  clearActivity: (projectId: string) => void;

  // DAG Workflow Definitions
  definitions: WorkflowDefinition[];
  definitionsLoading: boolean;
  selectedDefinition: WorkflowDefinition | null;
  fetchDefinitions: (projectId: string) => Promise<void>;
  createDefinition: (
    projectId: string,
    data: {
      name: string;
      description: string;
      nodes: WorkflowNodeData[];
      edges: WorkflowEdgeData[];
    }
  ) => Promise<WorkflowDefinition | null>;
  updateDefinition: (
    id: string,
    data: {
      name?: string;
      description?: string;
      status?: string;
      nodes?: WorkflowNodeData[];
      edges?: WorkflowEdgeData[];
    }
  ) => Promise<boolean>;
  deleteDefinition: (id: string) => Promise<boolean>;
  selectDefinition: (def: WorkflowDefinition | null) => void;
  fetchDefinition: (id: string) => Promise<WorkflowDefinition | null>;

  // DAG Workflow Executions
  executions: WorkflowExecution[];
  executionsLoading: boolean;
  selectedExecution: WorkflowExecution | null;
  nodeExecutions: WorkflowNodeExecution[];
  startExecution: (
    workflowId: string,
    taskId?: string
  ) => Promise<WorkflowExecution | null>;
  fetchExecutions: (workflowId: string) => Promise<void>;
  fetchExecution: (
    id: string
  ) => Promise<{
    execution: WorkflowExecution;
    nodeExecutions: WorkflowNodeExecution[];
  } | null>;
  cancelExecution: (id: string) => Promise<boolean>;
  selectExecution: (exec: WorkflowExecution | null) => void;

  // Workflow Templates
  templates: WorkflowDefinition[];
  templatesLoading: boolean;
  fetchTemplates: (category?: string) => Promise<void>;
  cloneTemplate: (
    templateId: string,
    projectId: string,
    overrides?: Record<string, unknown>
  ) => Promise<WorkflowDefinition | null>;
  executeTemplate: (
    templateId: string,
    projectId: string,
    taskId?: string,
    variables?: Record<string, unknown>
  ) => Promise<WorkflowExecution | null>;

  // Human Review
  resolveReview: (
    executionId: string,
    nodeId: string,
    decision: string,
    comment?: string
  ) => Promise<boolean>;
  sendExternalEvent: (
    executionId: string,
    nodeId: string,
    payload: unknown
  ) => Promise<boolean>;

  pendingReviews: WorkflowPendingReview[];
  pendingReviewsLoading: boolean;
  fetchPendingReviews: (projectId: string) => Promise<void>;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export const ALL_TASK_STATUSES: TaskStatus[] = [
  "inbox",
  "triaged",
  "assigned",
  "in_progress",
  "blocked",
  "in_review",
  "changes_requested",
  "done",
  "cancelled",
  "budget_exceeded",
];

export const TRIGGER_ACTIONS = [
  { value: "auto_assign", label: "Auto-assign agent" },
  { value: "notify", label: "Send notification" },
  { value: "dispatch_agent", label: "Dispatch agent run" },
];

export const useWorkflowStore = create<WorkflowState>()((set) => ({
  config: null,
  loading: false,
  saving: false,
  error: null,
  recentActivityByProject: {},

  // DAG state
  definitions: [],
  definitionsLoading: false,
  selectedDefinition: null,
  executions: [],
  executionsLoading: false,
  selectedExecution: null,
  nodeExecutions: [],
  pendingReviews: [],
  pendingReviewsLoading: false,

  fetchWorkflow: async (projectId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ loading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<WorkflowConfig>(
        `/api/v1/projects/${projectId}/workflow`,
        { token }
      );
      set({ config: data, error: null });
    } catch {
      set({ error: "Unable to load workflow config" });
    } finally {
      set({ loading: false });
    }
  },

  updateWorkflow: async (projectId, payload) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return false;

    set({ saving: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.put<WorkflowConfig>(
        `/api/v1/projects/${projectId}/workflow`,
        payload,
        { token }
      );
      set({ config: data, error: null });
      return true;
    } catch {
      set({ error: "Unable to save workflow config" });
      return false;
    } finally {
      set({ saving: false });
    }
  },

  appendActivity: (projectId, entry) =>
    set((state) => {
      const existing = state.recentActivityByProject[projectId] ?? [];
      const nextEntry: WorkflowActivityEntry = {
        ...entry,
        timestamp: entry.timestamp ?? new Date().toISOString(),
      };

      return {
        recentActivityByProject: {
          ...state.recentActivityByProject,
          [projectId]: [nextEntry, ...existing].slice(
            0,
            MAX_WORKFLOW_ACTIVITY_ENTRIES
          ),
        },
      };
    }),

  clearActivity: (projectId) =>
    set((state) => ({
      recentActivityByProject: {
        ...state.recentActivityByProject,
        [projectId]: [],
      },
    })),

  // --- DAG Workflow Definitions ---

  fetchDefinitions: async (projectId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ definitionsLoading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<WorkflowDefinition[]>(
        `/api/v1/projects/${projectId}/workflows`,
        { token }
      );
      set({ definitions: data ?? [], error: null });
    } catch {
      set({ error: "Unable to load workflow definitions" });
    } finally {
      set({ definitionsLoading: false });
    }
  },

  createDefinition: async (projectId, payload) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;

    set({ saving: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<WorkflowDefinition>(
        `/api/v1/projects/${projectId}/workflows`,
        payload,
        { token }
      );
      set((state) => ({
        definitions: [data, ...state.definitions],
        error: null,
      }));
      return data;
    } catch {
      set({ error: "Unable to create workflow" });
      return null;
    } finally {
      set({ saving: false });
    }
  },

  fetchDefinition: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<WorkflowDefinition>(
        `/api/v1/workflows/${id}`,
        { token }
      );
      set({ selectedDefinition: data });
      return data;
    } catch {
      set({ error: "Unable to load workflow" });
      return null;
    }
  },

  updateDefinition: async (id, payload) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return false;

    set({ saving: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.put<WorkflowDefinition>(
        `/api/v1/workflows/${id}`,
        payload,
        { token }
      );
      set((state) => ({
        definitions: state.definitions.map((d) => (d.id === id ? data : d)),
        selectedDefinition:
          state.selectedDefinition?.id === id
            ? data
            : state.selectedDefinition,
        error: null,
      }));
      return true;
    } catch {
      set({ error: "Unable to update workflow" });
      return false;
    } finally {
      set({ saving: false });
    }
  },

  deleteDefinition: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return false;

    try {
      const api = createApiClient(API_URL);
      await api.delete(`/api/v1/workflows/${id}`, { token });
      set((state) => ({
        definitions: state.definitions.filter((d) => d.id !== id),
        selectedDefinition:
          state.selectedDefinition?.id === id
            ? null
            : state.selectedDefinition,
      }));
      return true;
    } catch {
      set({ error: "Unable to delete workflow" });
      return false;
    }
  },

  selectDefinition: (def) => set({ selectedDefinition: def }),

  // --- DAG Workflow Executions ---

  startExecution: async (workflowId, taskId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;

    set({ saving: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const body: Record<string, unknown> = {};
      if (taskId) body.taskId = taskId;
      const { data } = await api.post<WorkflowExecution>(
        `/api/v1/workflows/${workflowId}/execute`,
        body,
        { token }
      );
      set((state) => ({
        executions: [data, ...state.executions],
        error: null,
      }));
      return data;
    } catch {
      set({ error: "Unable to start execution" });
      return null;
    } finally {
      set({ saving: false });
    }
  },

  fetchExecutions: async (workflowId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    set({ executionsLoading: true, error: null });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<WorkflowExecution[]>(
        `/api/v1/workflows/${workflowId}/executions`,
        { token }
      );
      set({ executions: data ?? [], error: null });
    } catch {
      set({ error: "Unable to load executions" });
    } finally {
      set({ executionsLoading: false });
    }
  },

  fetchExecution: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;

    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<{
        execution: WorkflowExecution;
        nodeExecutions: WorkflowNodeExecution[];
      }>(`/api/v1/executions/${id}`, { token });
      set({
        selectedExecution: data.execution ?? data,
        nodeExecutions: data.nodeExecutions ?? [],
      });
      return data;
    } catch {
      set({ error: "Unable to load execution" });
      return null;
    }
  },

  cancelExecution: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return false;

    try {
      const api = createApiClient(API_URL);
      await api.post(`/api/v1/executions/${id}/cancel`, {}, { token });
      set((state) => ({
        executions: state.executions.map((e) =>
          e.id === id ? { ...e, status: "cancelled" } : e
        ),
      }));
      return true;
    } catch {
      set({ error: "Unable to cancel execution" });
      return false;
    }
  },

  selectExecution: (exec) => set({ selectedExecution: exec }),

  // --- Templates ---
  templates: [],
  templatesLoading: false,

  fetchTemplates: async (category) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    set({ templatesLoading: true });
    try {
      const api = createApiClient(API_URL);
      const params = category ? `?category=${category}` : "";
      const { data } = await api.get<WorkflowDefinition[]>(
        `/api/v1/workflow-templates${params}`,
        { token }
      );
      set({ templates: data, templatesLoading: false });
    } catch {
      set({ templatesLoading: false, error: "Unable to fetch templates" });
    }
  },

  cloneTemplate: async (templateId, projectId, overrides) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<WorkflowDefinition>(
        `/api/v1/workflow-templates/${templateId}/clone`,
        { overrides },
        { token, headers: { "X-Project-ID": projectId } }
      );
      set((state) => ({
        definitions: [data, ...state.definitions],
      }));
      return data;
    } catch {
      set({ error: "Unable to clone template" });
      return null;
    }
  },

  executeTemplate: async (templateId, projectId, taskId, variables) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return null;
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.post<WorkflowExecution>(
        `/api/v1/workflow-templates/${templateId}/execute`,
        { taskId, variables },
        { token, headers: { "X-Project-ID": projectId } }
      );
      set((state) => ({
        executions: [data, ...state.executions],
      }));
      return data;
    } catch {
      set({ error: "Unable to execute template" });
      return null;
    }
  },

  // --- Human Review & Events ---

  resolveReview: async (executionId, nodeId, decision, comment) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return false;
    try {
      const api = createApiClient(API_URL);
      await api.post(
        `/api/v1/executions/${executionId}/review`,
        { nodeId, decision, comment: comment ?? "" },
        { token }
      );
      return true;
    } catch {
      set({ error: "Unable to resolve review" });
      return false;
    }
  },

  sendExternalEvent: async (executionId, nodeId, payload) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return false;
    try {
      const api = createApiClient(API_URL);
      await api.post(
        `/api/v1/executions/${executionId}/events`,
        { nodeId, payload },
        { token }
      );
      return true;
    } catch {
      set({ error: "Unable to send event" });
      return false;
    }
  },

  fetchPendingReviews: async (projectId) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    set({ pendingReviewsLoading: true });
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<WorkflowPendingReview[]>(
        `/api/v1/projects/${projectId}/workflow-reviews`,
        { token }
      );
      set({ pendingReviews: data ?? [], pendingReviewsLoading: false });
    } catch {
      set({ pendingReviewsLoading: false, error: "Unable to fetch reviews" });
    }
  },
}));
