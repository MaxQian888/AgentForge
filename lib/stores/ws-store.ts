"use client";
import { create } from "zustand";
import { WSClient } from "@/lib/ws-client";
import { useTaskStore } from "./task-store";
import { useAgentStore } from "./agent-store";
import { useNotificationStore } from "./notification-store";
import { useDashboardStore } from "./dashboard-store";
import { useSprintStore, type Sprint } from "./sprint-store";
import { useTeamStore, normalizeTeam } from "./team-store";
import { useWorkflowStore } from "./workflow-store";
import type { Task } from "./task-store";
import type { AgentPoolSummary, AgentStatus, MemoryStatus } from "./agent-store";

interface WSEventEnvelope<T> {
  type: string;
  projectId?: string;
  payload?: T;
}

function extractPayload<T>(data: unknown): T | null {
  if (!data || typeof data !== "object") {
    return null;
  }

  if ("payload" in data) {
    return ((data as WSEventEnvelope<T>).payload ?? null) as T | null;
  }

  return data as T;
}

function updateTaskRuntime(taskId: string, patch: Partial<Task>) {
  const store = useTaskStore.getState();
  const current = store.tasks.find((task) => task.id === taskId);
  if (!current) {
    return;
  }

  store.upsertTask({
    ...current,
    ...patch,
    updatedAt: typeof patch.updatedAt === "string" ? patch.updatedAt : current.updatedAt,
  });
}

function isActiveAgentStatus(status: string | undefined): boolean {
  return status === "starting" || status === "running";
}

function isPausedAgentStatus(status: string | undefined): boolean {
  return status === "paused";
}

function syncPoolFromAgentRoster() {
  useAgentStore.setState((state) => {
    if (!state.pool) {
      return state;
    }

    const active = state.agents.filter((agent) => isActiveAgentStatus(agent.status)).length;
    const pausedResumable = state.agents.filter((agent) => isPausedAgentStatus(agent.status)).length;

    const nextPool: AgentPoolSummary = {
      ...state.pool,
      active,
      available: Math.max(state.pool.max - active, 0),
      pausedResumable,
    };

    return { ...state, pool: nextPool };
  });
}

interface WSState {
  connected: boolean;
  connect: (url: string, token: string) => void;
  disconnect: () => void;
  subscribe: (channel: string) => void;
  unsubscribe: (channel: string) => void;
}

let client: WSClient | null = null;

export const useWSStore = create<WSState>()((set) => ({
  connected: false,

  connect: (url, token) => {
    if (client) client.close();

    client = new WSClient(url, token);

    client.on("connected", () => set({ connected: true }));
    client.on("disconnected", () => set({ connected: false }));

    client.on("task.updated", (data) => {
      const payload = extractPayload<{ task?: import("./task-store").Task }>(data);
      if (!payload?.task) {
        return;
      }
      useTaskStore.getState().upsertTask(payload.task);
      useDashboardStore.getState().applyTaskUpdate(payload.task);
    });

    client.on("task.created", (data) => {
      const payload = extractPayload<import("./task-store").Task>(data);
      if (!payload) {
        return;
      }
      useTaskStore.getState().upsertTask(payload);
      useDashboardStore.getState().applyTaskUpdate(payload);
    });

    client.on("task.transitioned", (data) => {
      const payload = extractPayload<{ task?: import("./task-store").Task }>(data);
      if (!payload?.task) {
        return;
      }
      useTaskStore.getState().upsertTask(payload.task);
      useDashboardStore.getState().applyTaskUpdate(payload.task);
    });

    client.on("task.assigned", (data) => {
      const payload = extractPayload<{ task?: import("./task-store").Task }>(data);
      if (!payload?.task) {
        return;
      }
      useTaskStore.getState().upsertTask(payload.task);
      useDashboardStore.getState().applyTaskUpdate(payload.task);
    });

    client.on("task.dispatch_blocked", (data) => {
      const payload = extractPayload<{ task?: import("./task-store").Task }>(data);
      if (!payload?.task) {
        return;
      }
      useTaskStore.getState().upsertTask(payload.task);
      useDashboardStore.getState().applyTaskUpdate(payload.task);
    });

    client.on("task.deleted", (data) => {
      const payload = extractPayload<{ id?: string }>(data);
      if (!payload?.id) {
        return;
      }
      useTaskStore.getState().removeTask(payload.id);
    });

    client.on("task.progress.updated", (data) => {
      const payload = extractPayload<{ task?: import("./task-store").Task }>(data);
      if (!payload?.task) {
        return;
      }
      useTaskStore.getState().upsertTask(payload.task);
      useDashboardStore.getState().applyTaskUpdate(payload.task);
    });

    client.on("task.progress.alerted", (data) => {
      const payload = extractPayload<{ task?: import("./task-store").Task }>(data);
      if (!payload?.task) {
        return;
      }
      useTaskStore.getState().upsertTask(payload.task);
      useDashboardStore.getState().applyTaskUpdate(payload.task);
    });

    client.on("task.progress.recovered", (data) => {
      const payload = extractPayload<{ task?: import("./task-store").Task }>(data);
      if (!payload?.task) {
        return;
      }
      useTaskStore.getState().upsertTask(payload.task);
      useDashboardStore.getState().applyTaskUpdate(payload.task);
    });

    const applyAgentEvent = (data: unknown) => {
      const payload = extractPayload<Record<string, unknown>>(data);
      if (!payload || typeof payload.id !== "string") {
        return;
      }

      const agentPayload = {
        id: payload.id,
        taskId: String(payload.taskId ?? ""),
        taskTitle: typeof payload.taskTitle === "string" ? payload.taskTitle : undefined,
        memberId: String(payload.memberId ?? ""),
        roleId: typeof payload.roleId === "string" ? payload.roleId : "",
        roleName: typeof payload.roleName === "string" ? payload.roleName : undefined,
        status: (typeof payload.status === "string" ? payload.status : "running") as AgentStatus,
        provider: typeof payload.provider === "string" ? payload.provider : "",
        model: typeof payload.model === "string" ? payload.model : "",
        inputTokens: Number(payload.inputTokens ?? 0),
        outputTokens: Number(payload.outputTokens ?? 0),
        cacheReadTokens: Number(payload.cacheReadTokens ?? 0),
        costUsd: Number(payload.costUsd ?? 0),
        budgetUsd: Number(payload.budgetUsd ?? 0),
        turnCount: Number(payload.turnCount ?? 0),
        worktreePath: typeof payload.worktreePath === "string" ? payload.worktreePath : undefined,
        branchName: typeof payload.branchName === "string" ? payload.branchName : undefined,
        sessionId: typeof payload.sessionId === "string" ? payload.sessionId : undefined,
        startedAt: typeof payload.startedAt === "string" ? payload.startedAt : new Date().toISOString(),
        createdAt: typeof payload.createdAt === "string" ? payload.createdAt : new Date().toISOString(),
        completedAt: typeof payload.completedAt === "string" ? payload.completedAt : null,
        canResume: Boolean(payload.canResume),
        memoryStatus:
          typeof payload.memoryStatus === "string"
            ? (payload.memoryStatus as MemoryStatus)
            : undefined,
        lastActivityAt:
          typeof payload.lastActivityAt === "string"
            ? payload.lastActivityAt
            : typeof payload.completedAt === "string"
              ? payload.completedAt
              : typeof payload.startedAt === "string"
                ? payload.startedAt
                : new Date().toISOString(),
        updatedAt:
          typeof payload.lastActivityAt === "string"
            ? payload.lastActivityAt
            : typeof payload.completedAt === "string"
              ? payload.completedAt
              : typeof payload.startedAt === "string"
                ? payload.startedAt
                : new Date().toISOString(),
      };

      useAgentStore.getState().upsertAgent(agentPayload);
      useDashboardStore.getState().applyAgentUpdate(agentPayload);
      syncPoolFromAgentRoster();
    };

    client.on("agent.started", applyAgentEvent);
    client.on("agent.progress", applyAgentEvent);
    client.on("agent.completed", applyAgentEvent);
    client.on("agent.failed", applyAgentEvent);
    client.on("agent.cost_update", applyAgentEvent);

    client.on("agent.output", (data) => {
      const payload = extractPayload<{ agent_id?: string; agentId?: string; line?: string }>(data);
      const agentId = payload?.agentId ?? payload?.agent_id;
      const line = payload?.line;
      if (!agentId || typeof line !== "string") {
        return;
      }
      useAgentStore.getState().appendOutput(agentId, line);
    });

    client.on("budget.warning", (data) => {
      const payload = extractPayload<{ taskId?: string; spent?: number; budget?: number }>(data);
      if (!payload?.taskId) {
        return;
      }

      updateTaskRuntime(payload.taskId, {
        spentUsd: typeof payload.spent === "number" ? payload.spent : undefined,
        budgetUsd: typeof payload.budget === "number" ? payload.budget : undefined,
      });
    });

    client.on("budget.exceeded", (data) => {
      const payload = extractPayload<{ taskId?: string; spent?: number; budget?: number }>(data);
      if (!payload?.taskId) {
        return;
      }

      updateTaskRuntime(payload.taskId, {
        status: "budget_exceeded",
        spentUsd: typeof payload.spent === "number" ? payload.spent : undefined,
        budgetUsd: typeof payload.budget === "number" ? payload.budget : undefined,
      });
    });

    client.on("notification", (data) => {
      const payload = extractPayload(data);
      if (!payload) {
        return;
      }
      useNotificationStore.getState().addNotification(payload as import("./notification-store").Notification);
      useDashboardStore.getState().applyActivityNotification(
        payload as import("./notification-store").Notification
      );
    });

    client.on("workflow.trigger_fired", (data) => {
      const envelope =
        data && typeof data === "object"
          ? (data as WSEventEnvelope<Record<string, unknown>>)
          : null;
      const payload = extractPayload<Record<string, unknown>>(data);
      const projectId =
        typeof envelope?.projectId === "string"
          ? envelope.projectId
          : useDashboardStore.getState().selectedProjectId ?? "";

      if (!projectId || !payload) {
        return;
      }

      useWorkflowStore.getState().appendActivity(projectId, {
        taskId: typeof payload.taskId === "string" ? payload.taskId : "",
        action: typeof payload.action === "string" ? payload.action : "unknown",
        from: typeof payload.from === "string" ? payload.from : "",
        to: typeof payload.to === "string" ? payload.to : "",
        config:
          payload.config && typeof payload.config === "object"
            ? (payload.config as Record<string, unknown>)
            : undefined,
      });
    });

    client.on("sprint.updated", (data) => {
      const payload = extractPayload<{ sprint?: Sprint }>(data);
      const sprint = payload?.sprint ?? (payload as unknown as Sprint);
      if (!sprint?.id || !sprint?.projectId) {
        return;
      }
      useSprintStore.getState().upsertSprint(sprint);
    });

    const applyTeamEvent = (data: unknown) => {
      const payload = extractPayload<Record<string, unknown>>(data);
      if (!payload || typeof payload.id !== "string") {
        return;
      }
      const team = normalizeTeam(payload);
      useTeamStore.getState().upsertTeam(team);
    };

    client.on("team.created", applyTeamEvent);
    client.on("team.planning", applyTeamEvent);
    client.on("team.executing", applyTeamEvent);
    client.on("team.reviewing", applyTeamEvent);
    client.on("team.completed", applyTeamEvent);
    client.on("team.failed", applyTeamEvent);
    client.on("team.cancelled", applyTeamEvent);
    client.on("team.cost_update", applyTeamEvent);

    client.connect();
  },

  disconnect: () => {
    client?.close();
    client = null;
    set({ connected: false });
  },

  subscribe: (channel) => {
    client?.subscribe(channel);
  },

  unsubscribe: (channel) => {
    client?.unsubscribe(channel);
  },
}));
