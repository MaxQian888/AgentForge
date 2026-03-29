"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";
import {
  buildDashboardSummary,
  type DashboardActivitySource,
  type DashboardAgentSource,
  type DashboardMemberSource,
  type DashboardSummary,
  type DashboardTaskSource,
} from "@/lib/dashboard/summary";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

export interface DashboardProject {
  id: string;
  name: string;
  slug: string;
  description: string;
  repoUrl: string;
  defaultBranch: string;
  createdAt: string;
}

export interface DashboardConfig {
  id: string;
  projectId: string;
  name: string;
  layout: unknown;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  widgets?: DashboardWidget[];
}

export interface DashboardWidget {
  id: string;
  dashboardId: string;
  widgetType: string;
  config: unknown;
  position: unknown;
  createdAt: string;
  updatedAt: string;
}

export interface DashboardWidgetRequestState {
  status: "idle" | "loading" | "success" | "error";
  error: string | null;
}

interface TaskListResponse {
  items: DashboardTaskSource[];
  total: number;
  page: number;
  limit: number;
}

interface DashboardState {
  summary: DashboardSummary | null;
  projects: DashboardProject[];
  selectedProjectId: string | null;
  activeDashboardIdByProject: Record<string, string | null>;
  dashboardsLoadingByProject: Record<string, boolean>;
  dashboardsErrorByProject: Record<string, string | null>;
  dashboardsByProject: Record<string, DashboardConfig[]>;
  widgetsByDashboard: Record<string, DashboardWidget[]>;
  widgetData: Record<string, unknown>;
  widgetRequestStateByKey: Record<string, DashboardWidgetRequestState>;
  tasks: DashboardTaskSource[];
  members: DashboardMemberSource[];
  agents: DashboardAgentSource[];
  activity: DashboardActivitySource[];
  loading: boolean;
  error: string | null;
  sectionErrors: Record<string, string>;
  fetchSummary: (options?: { projectId?: string | null; now?: string }) => Promise<void>;
  fetchDashboards: (projectId: string) => Promise<void>;
  setActiveDashboard: (projectId: string, dashboardId: string | null) => void;
  createDashboard: (projectId: string, input: { name: string; layout?: unknown }) => Promise<void>;
  updateDashboard: (projectId: string, dashboardId: string, input: { name?: string; layout?: unknown }) => Promise<void>;
  deleteDashboard: (projectId: string, dashboardId: string) => Promise<void>;
  fetchWidgetData: (projectId: string, widgetType: string, config?: unknown) => Promise<unknown>;
  saveWidget: (projectId: string, dashboardId: string, input: { id?: string; widgetType: string; config?: unknown; position?: unknown }) => Promise<void>;
  deleteWidget: (projectId: string, dashboardId: string, widgetId: string) => Promise<void>;
  applyTaskUpdate: (task: DashboardTaskSource) => void;
  applyAgentUpdate: (agent: DashboardAgentSource) => void;
  applyActivityNotification: (notification: {
    id: string;
    type: string;
    title: string;
    message?: string;
    body?: string;
    createdAt: string;
    targetId?: string;
  }) => void;
}

function getToken() {
  const authState = useAuthStore.getState() as {
    accessToken?: string | null;
    token?: string | null;
  };
  return authState.accessToken ?? authState.token ?? null;
}

function normalizeActivitySource(activity: {
  id: string;
  type: string;
  title: string;
  message?: string;
  body?: string;
  createdAt: string;
  targetId?: string;
}): DashboardActivitySource {
  return {
    id: activity.id,
    type: activity.type,
    title: activity.title,
    message: activity.message ?? activity.body ?? "",
    createdAt: activity.createdAt,
    targetId: activity.targetId ?? "",
  };
}

function buildSummarySnapshot(state: {
  selectedProjectId: string | null;
  projects: DashboardProject[];
  tasks: DashboardTaskSource[];
  members: DashboardMemberSource[];
  agents: DashboardAgentSource[];
  activity: DashboardActivitySource[];
}): DashboardSummary {
  const selectedProject =
    state.projects.find((project) => project.id === state.selectedProjectId) ?? null;

  return buildDashboardSummary({
    scopeProjectId: state.selectedProjectId,
    scopeProjectName: selectedProject?.name ?? "All Projects",
    projectsCount: state.projects.length,
    tasks: state.tasks,
    agents: state.agents,
    members: state.members,
    activity: state.activity,
  });
}

export const useDashboardStore = create<DashboardState>()((set) => ({
  summary: null,
  projects: [],
  selectedProjectId: null,
  activeDashboardIdByProject: {},
  dashboardsLoadingByProject: {},
  dashboardsErrorByProject: {},
  dashboardsByProject: {},
  widgetsByDashboard: {},
  widgetData: {},
  widgetRequestStateByKey: {},
  tasks: [],
  members: [],
  agents: [],
  activity: [],
  loading: false,
  error: null,
  sectionErrors: {},

  fetchSummary: async (options) => {
    const token = getToken();
    if (!token) return;

    set({ loading: true, error: null, sectionErrors: {} });

    try {
      const api = createApiClient(API_URL);
      const { data: projects } = await api.get<DashboardProject[]>(
        "/api/v1/projects",
        { token }
      );

      const selectedProjectId =
        options?.projectId ?? projects[0]?.id ?? null;
      const selectedProject =
        projects.find((project) => project.id === selectedProjectId) ?? null;

      if (!selectedProjectId || !selectedProject) {
        set({
          projects,
          selectedProjectId: null,
          tasks: [],
          members: [],
          agents: [],
          activity: [],
          summary: buildDashboardSummary({
            scopeProjectId: null,
            scopeProjectName: "All Projects",
            projectsCount: projects.length,
            tasks: [],
            agents: [],
            members: [],
            activity: [],
            now: options?.now,
          }),
        });
        return;
      }

      const results = await Promise.allSettled([
        api.get<TaskListResponse>(`/api/v1/projects/${selectedProjectId}/tasks`, {
          token,
        }),
        api.get<DashboardMemberSource[]>(
          `/api/v1/projects/${selectedProjectId}/members`,
          { token }
        ),
        api.get<DashboardAgentSource[]>("/api/v1/agents", { token }),
        api.get<Array<DashboardActivitySource & { body?: string }>>("/api/v1/notifications", { token }),
      ]);

      const [tasksResult, membersResult, agentsResult, activityResult] = results;
      const sectionErrors: Record<string, string> = {};

      const tasks =
        tasksResult.status === "fulfilled"
          ? tasksResult.value.data.items
          : (sectionErrors.tasks = tasksResult.reason instanceof Error
              ? tasksResult.reason.message
              : "Failed to load tasks",
            []);

      const members =
        membersResult.status === "fulfilled"
          ? membersResult.value.data
          : (sectionErrors.team = membersResult.reason instanceof Error
              ? membersResult.reason.message
              : "Failed to load team members",
            []);

      const agents =
        agentsResult.status === "fulfilled"
          ? agentsResult.value.data.filter((agent) =>
              tasks.some((task) => task.id === agent.taskId)
            )
          : (sectionErrors.agents = agentsResult.reason instanceof Error
              ? agentsResult.reason.message
              : "Failed to load agents",
            []);

      const activity =
        activityResult.status === "fulfilled"
          ? activityResult.value.data.map(normalizeActivitySource)
          : (sectionErrors.activity = activityResult.reason instanceof Error
              ? activityResult.reason.message
              : "Failed to load activity",
            []);

      const summary = buildDashboardSummary({
        scopeProjectId: selectedProjectId,
        scopeProjectName: selectedProject.name,
        projectsCount: projects.length,
        tasks,
        agents,
        members,
        activity,
        now: options?.now,
      });

      set({
        projects,
        selectedProjectId,
        tasks,
        members,
        agents,
        activity,
        summary,
        sectionErrors,
      });
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : "Failed to load dashboard summary",
        tasks: [],
        members: [],
        agents: [],
        activity: [],
      });
    } finally {
      set({ loading: false });
    }
  },

  fetchDashboards: async (projectId) => {
    const token = getToken();
    if (!token) return;
    const api = createApiClient(API_URL);
    set((state) => ({
      dashboardsLoadingByProject: {
        ...state.dashboardsLoadingByProject,
        [projectId]: true,
      },
      dashboardsErrorByProject: {
        ...state.dashboardsErrorByProject,
        [projectId]: null,
      },
    }));
    try {
      const { data } = await api.get<DashboardConfig[]>(`/api/v1/projects/${projectId}/dashboards`, { token });
      set((state) => ({
        activeDashboardIdByProject: {
          ...state.activeDashboardIdByProject,
          [projectId]:
            (state.activeDashboardIdByProject[projectId] &&
            (data ?? []).some((item) => item.id === state.activeDashboardIdByProject[projectId]))
              ? state.activeDashboardIdByProject[projectId]
              : data?.[0]?.id ?? null,
        },
        dashboardsLoadingByProject: {
          ...state.dashboardsLoadingByProject,
          [projectId]: false,
        },
        dashboardsErrorByProject: {
          ...state.dashboardsErrorByProject,
          [projectId]: null,
        },
        dashboardsByProject: { ...state.dashboardsByProject, [projectId]: data ?? [] },
        widgetsByDashboard: {
          ...state.widgetsByDashboard,
          ...(data ?? []).reduce<Record<string, DashboardWidget[]>>((acc, item) => {
            acc[item.id] = item.widgets ?? [];
            return acc;
          }, {}),
        },
      }));
    } catch (error) {
      set((state) => ({
        dashboardsLoadingByProject: {
          ...state.dashboardsLoadingByProject,
          [projectId]: false,
        },
        dashboardsErrorByProject: {
          ...state.dashboardsErrorByProject,
          [projectId]:
            error instanceof Error
              ? error.message
              : "Failed to load dashboards",
        },
      }));
    }
  },

  setActiveDashboard: (projectId, dashboardId) => {
    set((state) => ({
      activeDashboardIdByProject: {
        ...state.activeDashboardIdByProject,
        [projectId]: dashboardId,
      },
    }));
  },

  createDashboard: async (projectId, input) => {
    const token = getToken();
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data } = await api.post<DashboardConfig>(`/api/v1/projects/${projectId}/dashboards`, input, { token });
    set((state) => ({
      activeDashboardIdByProject: {
        ...state.activeDashboardIdByProject,
        [projectId]: data.id,
      },
      dashboardsByProject: {
        ...state.dashboardsByProject,
        [projectId]: [...(state.dashboardsByProject[projectId] ?? []), data],
      },
      widgetsByDashboard: { ...state.widgetsByDashboard, [data.id]: data.widgets ?? [] },
    }));
  },

  updateDashboard: async (projectId, dashboardId, input) => {
    const token = getToken();
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data } = await api.put<DashboardConfig>(`/api/v1/projects/${projectId}/dashboards/${dashboardId}`, input, { token });
    set((state) => ({
      dashboardsByProject: {
        ...state.dashboardsByProject,
        [projectId]: (state.dashboardsByProject[projectId] ?? []).map((item) => (item.id === dashboardId ? data : item)),
      },
    }));
  },

  deleteDashboard: async (projectId, dashboardId) => {
    const token = getToken();
    if (!token) return;
    const api = createApiClient(API_URL);
    await api.delete(`/api/v1/projects/${projectId}/dashboards/${dashboardId}`, { token });
    set((state) => {
      const nextWidgets = { ...state.widgetsByDashboard };
      delete nextWidgets[dashboardId];
      const remainingDashboards = (state.dashboardsByProject[projectId] ?? []).filter(
        (item) => item.id !== dashboardId
      );
      return {
        activeDashboardIdByProject: {
          ...state.activeDashboardIdByProject,
          [projectId]:
            state.activeDashboardIdByProject[projectId] === dashboardId
              ? remainingDashboards[0]?.id ?? null
              : state.activeDashboardIdByProject[projectId] ?? null,
        },
        dashboardsByProject: {
          ...state.dashboardsByProject,
          [projectId]: remainingDashboards,
        },
        widgetsByDashboard: nextWidgets,
      };
    });
  },

  fetchWidgetData: async (projectId, widgetType, config) => {
    const token = getToken();
    if (!token) return null;
    const api = createApiClient(API_URL);
    const query = config == null ? "" : `?config=${encodeURIComponent(JSON.stringify(config))}`;
    const key = `${projectId}:${widgetType}:${JSON.stringify(config ?? {})}`;
    set((state) => ({
      widgetRequestStateByKey: {
        ...state.widgetRequestStateByKey,
        [key]: { status: "loading", error: null },
      },
    }));
    try {
      const { data } = await api.get<unknown>(`/api/v1/projects/${projectId}/dashboard/widgets/${widgetType}${query}`, { token });
      set((state) => ({
        widgetData: { ...state.widgetData, [key]: data },
        widgetRequestStateByKey: {
          ...state.widgetRequestStateByKey,
          [key]: { status: "success", error: null },
        },
      }));
      return data;
    } catch (error) {
      set((state) => ({
        widgetRequestStateByKey: {
          ...state.widgetRequestStateByKey,
          [key]: {
            status: "error",
            error:
              error instanceof Error
                ? error.message
                : "Failed to load widget data",
          },
        },
      }));
      return null;
    }
  },

  saveWidget: async (projectId, dashboardId, input) => {
    const token = getToken();
    if (!token) return;
    const api = createApiClient(API_URL);
    const { data } = await api.post<DashboardWidget>(`/api/v1/projects/${projectId}/dashboards/${dashboardId}/widgets`, input, { token });
    set((state) => ({
      widgetsByDashboard: {
        ...state.widgetsByDashboard,
        [dashboardId]: [
          ...(state.widgetsByDashboard[dashboardId] ?? []).filter((item) => item.id !== data.id),
          data,
        ],
      },
    }));
  },

  deleteWidget: async (projectId, dashboardId, widgetId) => {
    const token = getToken();
    if (!token) return;
    const api = createApiClient(API_URL);
    await api.delete(`/api/v1/projects/${projectId}/dashboards/${dashboardId}/widgets/${widgetId}`, { token });
    set((state) => ({
      widgetsByDashboard: {
        ...state.widgetsByDashboard,
        [dashboardId]: (state.widgetsByDashboard[dashboardId] ?? []).filter((item) => item.id !== widgetId),
      },
    }));
  },

  applyTaskUpdate: (task) => {
    set((state) => {
      if (state.selectedProjectId && task.projectId !== state.selectedProjectId) {
        return state;
      }

      const existingIndex = state.tasks.findIndex((item) => item.id === task.id);
      const tasks =
        existingIndex === -1
          ? [...state.tasks, task]
          : state.tasks.map((item) => (item.id === task.id ? task : item));

      return {
        tasks,
        summary: buildSummarySnapshot({ ...state, tasks }),
      };
    });
  },

  applyAgentUpdate: (agent) => {
    set((state) => {
      const existingIndex = state.agents.findIndex((item) => item.id === agent.id);
      const agents =
        existingIndex === -1
          ? [...state.agents, agent]
          : state.agents.map((item) => (item.id === agent.id ? { ...item, ...agent } : item));

      return {
        agents,
        summary: buildSummarySnapshot({ ...state, agents }),
      };
    });
  },

  applyActivityNotification: (notification) => {
    set((state) => {
      const activity = [
        normalizeActivitySource(notification),
        ...state.activity.filter((item) => item.id !== notification.id),
      ];

      return {
        activity,
        summary: buildSummarySnapshot({ ...state, activity }),
      };
    });
  },
}));
