"use client";
import { create } from "zustand";
import { toast } from "sonner";
import { WSClient } from "@/lib/ws-client";
import { useTaskStore } from "./task-store";
import { useAgentStore } from "./agent-store";
import { useNotificationStore } from "./notification-store";
import { useDashboardStore } from "./dashboard-store";
import { useDocsStore } from "./docs-store";
import { useEntityLinkStore } from "./entity-link-store";
import { useReviewStore } from "./review-store";
import { useSprintStore, type Sprint } from "./sprint-store";
import { useSchedulerStore } from "./scheduler-store";
import { useTaskCommentStore } from "./task-comment-store";
import { useTeamStore, normalizeTeam } from "./team-store";
import { useWorkflowStore } from "./workflow-store";
import { emitProjectedDesktopEvent } from "@/lib/platform-runtime";
import { getPreferredLocale } from "./locale-store";
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

    const applyReviewEvent = (data: unknown) => {
      const payload = extractPayload<unknown>(data);
      if (!payload || typeof payload !== "object") {
        return;
      }

      let reviewPayload: unknown = payload;
      if ("review" in payload && typeof (payload as { review?: unknown }).review === "object") {
        reviewPayload = (payload as { review?: unknown }).review;
      }
      if (!reviewPayload || typeof reviewPayload !== "object") {
        return;
      }

      const reviewID = (reviewPayload as { id?: unknown }).id;
      if (typeof reviewID !== "string" || reviewID.trim() === "") {
        return;
      }
      useReviewStore.getState().updateReview(reviewPayload as import("./review-store").ReviewDTO);
    };

    client.on("review.completed", applyReviewEvent);
    client.on("review.pending_human", applyReviewEvent);
    client.on("review.updated", applyReviewEvent);

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
        runtime: typeof payload.runtime === "string" ? payload.runtime : "",
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

    client.on("agent.pool.updated", (data) => {
      const payload = extractPayload<AgentPoolSummary>(data);
      if (!payload) {
        return;
      }
      useAgentStore.setState((state) => ({
        ...state,
        pool: payload,
      }));
    });

    client.on("budget.warning", (data) => {
      const payload = extractPayload<{ taskId?: string; spent?: number; budget?: number; scope?: string; message?: string }>(data);
      if (!payload?.taskId) {
        return;
      }

      updateTaskRuntime(payload.taskId, {
        spentUsd: typeof payload.spent === "number" ? payload.spent : undefined,
        budgetUsd: typeof payload.budget === "number" ? payload.budget : undefined,
      });
      const locale = getPreferredLocale();
      const scopeLabel = formatBudgetScopeLabel(payload.scope, locale);
      toast.warning(
        locale === "zh-CN" ? `预算预警${scopeLabel ? `：${scopeLabel}` : ""}` : `Budget warning${scopeLabel ? `: ${scopeLabel}` : ""}`,
        {
          description:
            typeof payload.message === "string" && payload.message.trim() !== ""
              ? payload.message
              : locale === "zh-CN"
                ? `任务 ${payload.taskId} 的预算接近阈值。`
                : `Task ${payload.taskId} is approaching its budget threshold.`,
        },
      );
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

    client.on("plugin.lifecycle", (data) => {
      const payload = extractPayload<Record<string, unknown>>(data);
      if (!payload) {
        return;
      }

      emitProjectedDesktopEvent({
        type: "plugin.lifecycle",
        source: "plugin",
        timestamp:
          typeof payload.created_at === "string"
            ? payload.created_at
            : typeof payload.createdAt === "string"
              ? payload.createdAt
              : undefined,
        payload,
      });
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

    const applySchedulerEvent = (data: unknown) => {
      const payload = extractPayload<{
        job?: import("./scheduler-store").SchedulerJob;
        run?: import("./scheduler-store").SchedulerJobRun;
      }>(data);
      if (payload?.job) {
        useSchedulerStore.getState().upsertJob(payload.job);
      }
      if (payload?.run) {
        useSchedulerStore.getState().recordRun(payload.run);
      }
    };

    client.on("scheduler.job.updated", applySchedulerEvent);
    client.on("scheduler.run.started", applySchedulerEvent);
    client.on("scheduler.run.completed", applySchedulerEvent);

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

    const refreshDocsTree = () => {
      void useDocsStore.getState().refreshActiveProjectTree();
    };

    const refreshDocsPage = (pageId?: string) => {
      const docsState = useDocsStore.getState();
      if (!docsState.projectId || !docsState.currentPage) {
        refreshDocsTree();
        return;
      }
      void docsState.fetchPage(docsState.projectId, pageId ?? docsState.currentPage.id);
      void docsState.fetchVersions(docsState.projectId, pageId ?? docsState.currentPage.id);
      void docsState.fetchComments(docsState.projectId, pageId ?? docsState.currentPage.id);
    };

    client.on("wiki.page.created", (data) => {
      const payload = extractPayload<{ id?: string }>(data);
      refreshDocsTree();
      if (payload?.id) {
        refreshDocsPage(payload.id);
      }
    });
    client.on("wiki.page.updated", (data) => {
      const payload = extractPayload<{ id?: string }>(data);
      refreshDocsTree();
      refreshDocsPage(payload?.id);
    });
    client.on("wiki.page.moved", () => {
      refreshDocsTree();
    });
    client.on("wiki.page.deleted", () => {
      refreshDocsTree();
    });
    client.on("wiki.comment.created", () => {
      void useDocsStore.getState().refreshActivePageComments();
    });
    client.on("wiki.comment.resolved", () => {
      void useDocsStore.getState().refreshActivePageComments();
    });
    client.on("wiki.version.published", () => {
      const docsState = useDocsStore.getState();
      if (docsState.projectId && docsState.currentPage) {
        void docsState.fetchVersions(docsState.projectId, docsState.currentPage.id);
      }
    });

    client.on("link.created", (data) => {
      const payload = extractPayload<Record<string, unknown>>(data);
      if (!payload) {
        return;
      }
      useEntityLinkStore.getState().upsertLink({
        id: String(payload.id ?? ""),
        projectId: String(payload.projectId ?? ""),
        sourceType: String(payload.sourceType ?? ""),
        sourceId: String(payload.sourceId ?? ""),
        targetType: String(payload.targetType ?? ""),
        targetId: String(payload.targetId ?? ""),
        linkType: String(payload.linkType ?? ""),
        anchorBlockId: typeof payload.anchorBlockId === "string" ? payload.anchorBlockId : null,
        createdBy: String(payload.createdBy ?? ""),
        createdAt: String(payload.createdAt ?? new Date().toISOString()),
        deletedAt: typeof payload.deletedAt === "string" ? payload.deletedAt : null,
      });
    });

    client.on("link.deleted", (data) => {
      const payload = extractPayload<{ id?: string }>(data);
      if (!payload?.id) {
        return;
      }
      useEntityLinkStore.getState().removeLink(payload.id);
    });

    client.on("task_comment.created", (data) => {
      const payload = extractPayload<Record<string, unknown>>(data);
      if (!payload) {
        return;
      }
      useTaskCommentStore.getState().upsertComment({
        id: String(payload.id ?? ""),
        taskId: String(payload.taskId ?? ""),
        parentCommentId: typeof payload.parentCommentId === "string" ? payload.parentCommentId : null,
        body: String(payload.body ?? ""),
        mentions: Array.isArray(payload.mentions) ? payload.mentions.map(String) : [],
        resolvedAt: typeof payload.resolvedAt === "string" ? payload.resolvedAt : null,
        createdBy: String(payload.createdBy ?? ""),
        createdAt: String(payload.createdAt ?? new Date().toISOString()),
        updatedAt: String(payload.updatedAt ?? new Date().toISOString()),
        deletedAt: typeof payload.deletedAt === "string" ? payload.deletedAt : null,
      });
    });

    client.on("task_comment.resolved", (data) => {
      const payload = extractPayload<Record<string, unknown>>(data);
      if (!payload || typeof payload.taskId !== "string" || typeof payload.id !== "string") {
        return;
      }
      const current =
        useTaskCommentStore.getState().commentsByTask[payload.taskId] ?? [];
      const match = current.find((comment) => comment.id === payload.id);
      if (!match) {
        return;
      }
      useTaskCommentStore.getState().upsertComment({
        ...match,
        resolvedAt: payload.resolved === true ? new Date().toISOString() : null,
      });
    });

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

function formatBudgetScopeLabel(scope: string | undefined, locale: "zh-CN" | "en"): string {
  switch (scope) {
    case "task":
      return locale === "zh-CN" ? "任务" : "task";
    case "sprint":
      return locale === "zh-CN" ? "Sprint" : "sprint";
    case "project":
      return locale === "zh-CN" ? "项目" : "project";
    default:
      return "";
  }
}
