import {
  buildAgentProfileSummary,
  getAgentProfileReadiness,
  parseAgentProfile,
  type AgentProfile,
} from "@/lib/team/agent-profile";
import {
  getMemberStatusLabel,
  isMemberAvailable,
  normalizeMemberStatus,
  type MemberStatus,
} from "@/lib/team/member-status";

export type DashboardTaskStatus =
  | "inbox"
  | "triaged"
  | "assigned"
  | "in_progress"
  | "in_review"
  | "done"
  | "blocked"
  | "changes_requested"
  | "cancelled"
  | "budget_exceeded";

export type DashboardMemberType = "human" | "agent";

export interface DashboardTaskSource {
  id: string;
  projectId: string;
  title: string;
  status: DashboardTaskStatus;
  priority: string;
  assigneeId: string | null;
  assigneeType: DashboardMemberType | null;
  spentUsd: number;
  progress?: {
    healthStatus: "healthy" | "warning" | "stalled";
    riskReason: string;
    lastActivityAt?: string;
    lastRecoveredAt?: string | null;
  } | null;
  updatedAt: string;
  createdAt: string;
}

export interface DashboardAgentSource {
  id: string;
  taskId: string;
  memberId: string;
  status: string;
  costUsd: number;
  turnCount: number;
  updatedAt: string;
  createdAt: string;
  startedAt: string;
}

export interface DashboardMemberSource {
  id: string;
  projectId: string;
  name: string;
  type: DashboardMemberType;
  role: string;
  status?: MemberStatus;
  email: string;
  imPlatform?: string;
  imUserId?: string;
  avatarUrl?: string;
  agentConfig?: string;
  skills: string[];
  isActive: boolean;
  createdAt: string;
}

export interface DashboardActivitySource {
  id: string;
  type: string;
  title: string;
  message: string;
  createdAt: string;
  targetId: string;
}

export interface TeamMemberWorkload {
  assignedTasks: number;
  inProgressTasks: number;
  inReviewTasks: number;
  activeAgentRuns: number;
}

export interface TeamMember {
  id: string;
  projectId: string;
  name: string;
  type: DashboardMemberType;
  typeLabel: "Human" | "Agent";
  role: string;
  statusLabel?: string;
  email: string;
  imPlatform?: string;
  imUserId?: string;
  avatarUrl: string;
  skills: string[];
  isActive: boolean;
  status: MemberStatus;
  createdAt: string;
  lastActivityAt: string | null;
  workload: TeamMemberWorkload;
  agentProfile?: AgentProfile | null;
  roleBindingLabel?: string | null;
  readinessState?: "ready" | "incomplete" | null;
  readinessLabel?: string | null;
  readinessMissing?: string[];
  agentSummary?: string[];
}

export interface DashboardActivityItem {
  id: string;
  type: string;
  title: string;
  message: string;
  href: string;
  createdAt: string;
}

export interface DashboardRiskItem {
  id: string;
  kind: "stalled-task" | "unassigned-work" | "budget-pressure" | "review-pressure";
  title: string;
  description: string;
  href: string;
}

export interface DashboardSummary {
  scope: {
    projectId: string | null;
    projectName: string;
    projectsCount: number;
  };
  headline: {
    activeAgents: number;
    tasksInProgress: number;
    pendingReviews: number;
    weeklyCost: number;
  };
  progress: {
    total: number;
    inbox: number;
    triaged: number;
    assigned: number;
    inProgress: number;
    inReview: number;
    done: number;
  };
  team: {
    totalMembers: number;
    activeHumans: number;
    activeAgents: number;
    activeAgentRuns: number;
    overloadedMembers: number;
  };
  activity: DashboardActivityItem[];
  risks: DashboardRiskItem[];
  links: {
    projects: string;
    team: string;
    agents: string;
    reviews: string;
  };
}

interface BuildDashboardSummaryInput {
  scopeProjectId: string | null;
  scopeProjectName: string;
  projectsCount: number;
  tasks: DashboardTaskSource[];
  agents: DashboardAgentSource[];
  members: DashboardMemberSource[];
  activity: DashboardActivitySource[];
  now?: string;
}

const ACTIVE_AGENT_STATUSES = new Set(["starting", "running", "paused"]);
const WEEK_MS = 7 * 24 * 60 * 60 * 1000;
const STALLED_MS = 3 * 24 * 60 * 60 * 1000;

function toTime(value: string | null | undefined): number {
  return value ? new Date(value).getTime() : 0;
}

function formatProgressReason(reason: string): string {
  switch (reason) {
    case "awaiting_review":
      return "awaiting review";
    case "no_assignee":
      return "no assignee";
    case "no_recent_update":
      return "no recent update";
    default:
      return reason.replaceAll("_", " ");
  }
}

export function normalizeTeamMember(member: DashboardMemberSource): TeamMember {
  const agentProfile =
    member.type === "agent" ? parseAgentProfile(member.agentConfig) : null;
  const readiness =
    member.type === "agent" ? getAgentProfileReadiness(agentProfile) : null;
  const status = normalizeMemberStatus(member.status, member.isActive);

  return {
    id: member.id,
    projectId: member.projectId,
    name: member.name,
    type: member.type,
    typeLabel: member.type === "human" ? "Human" : "Agent",
    role: member.role || (member.type === "human" ? "Contributor" : "Agent"),
    statusLabel: getMemberStatusLabel(status),
    email: member.email,
    imPlatform: member.imPlatform ?? "",
    imUserId: member.imUserId ?? "",
    avatarUrl: member.avatarUrl ?? "",
    skills: member.skills ?? [],
    isActive: isMemberAvailable(status, member.isActive),
    status,
    createdAt: member.createdAt,
    lastActivityAt: null,
    workload: {
      assignedTasks: 0,
      inProgressTasks: 0,
      inReviewTasks: 0,
      activeAgentRuns: 0,
    },
    agentProfile,
    roleBindingLabel:
      member.type === "agent"
        ? agentProfile?.roleId || "Unbound role"
        : null,
    readinessState: readiness?.state ?? null,
    readinessLabel: readiness?.label ?? null,
    readinessMissing: readiness?.missing ?? [],
    agentSummary:
      member.type === "agent" ? buildAgentProfileSummary(agentProfile) : [],
  };
}

export function summarizeMemberRoster(input: {
  members: DashboardMemberSource[];
  tasks: DashboardTaskSource[];
  agents: DashboardAgentSource[];
  activity: DashboardActivitySource[];
}): TeamMember[] {
  const memberMap = new Map<string, TeamMember>();

  for (const member of input.members) {
    memberMap.set(member.id, normalizeTeamMember(member));
  }

  for (const task of input.tasks) {
    if (!task.assigneeId) continue;
    const member = memberMap.get(task.assigneeId);
    if (!member) continue;

    member.workload.assignedTasks += 1;
    if (task.status === "in_progress") {
      member.workload.inProgressTasks += 1;
    }
    if (task.status === "in_review") {
      member.workload.inReviewTasks += 1;
    }
    const taskTime = task.updatedAt || task.createdAt;
    if (!member.lastActivityAt || toTime(taskTime) > toTime(member.lastActivityAt)) {
      member.lastActivityAt = taskTime;
    }
  }

  for (const agent of input.agents) {
    if (!ACTIVE_AGENT_STATUSES.has(agent.status)) continue;
    const member = memberMap.get(agent.memberId);
    if (!member) continue;
    member.workload.activeAgentRuns += 1;
    if (!member.lastActivityAt || toTime(agent.updatedAt) > toTime(member.lastActivityAt)) {
      member.lastActivityAt = agent.updatedAt || agent.startedAt || agent.createdAt;
    }
  }

  for (const event of input.activity) {
    const member = memberMap.get(event.targetId);
    if (!member) continue;
    if (!member.lastActivityAt || toTime(event.createdAt) > toTime(member.lastActivityAt)) {
      member.lastActivityAt = event.createdAt;
    }
  }

  return input.members
    .map((member) => memberMap.get(member.id))
    .filter((member): member is TeamMember => Boolean(member));
}

export function enrichTeamMembers(input: {
  members: TeamMember[];
  tasks: DashboardTaskSource[];
  agents: DashboardAgentSource[];
  activity: DashboardActivitySource[];
}): TeamMember[] {
  const members = input.members.map((member) => ({
    ...member,
    skills: [...member.skills],
    workload: {
      assignedTasks: 0,
      inProgressTasks: 0,
      inReviewTasks: 0,
      activeAgentRuns: 0,
    },
    lastActivityAt: null,
    agentSummary: [...(member.agentSummary ?? [])],
    readinessMissing: [...(member.readinessMissing ?? [])],
  }));
  const memberMap = new Map<string, TeamMember>(
    members.map((member) => [member.id, member] satisfies [string, TeamMember])
  );

  for (const task of input.tasks) {
    if (!task.assigneeId) continue;
    const member = memberMap.get(task.assigneeId);
    if (!member) continue;

    member.workload.assignedTasks += 1;
    if (task.status === "in_progress") {
      member.workload.inProgressTasks += 1;
    }
    if (task.status === "in_review") {
      member.workload.inReviewTasks += 1;
    }
    const taskTime = task.updatedAt || task.createdAt;
    if (!member.lastActivityAt || toTime(taskTime) > toTime(member.lastActivityAt)) {
      member.lastActivityAt = taskTime;
    }
  }

  for (const agent of input.agents) {
    if (!ACTIVE_AGENT_STATUSES.has(agent.status)) continue;
    const member = memberMap.get(agent.memberId);
    if (!member) continue;
    member.workload.activeAgentRuns += 1;
    if (!member.lastActivityAt || toTime(agent.updatedAt) > toTime(member.lastActivityAt)) {
      member.lastActivityAt = agent.updatedAt || agent.startedAt || agent.createdAt;
    }
  }

  for (const event of input.activity) {
    const member = memberMap.get(event.targetId);
    if (!member) continue;
    if (!member.lastActivityAt || toTime(event.createdAt) > toTime(member.lastActivityAt)) {
      member.lastActivityAt = event.createdAt;
    }
  }

  return input.members
    .map((member) => memberMap.get(member.id))
    .filter((member): member is TeamMember => Boolean(member));
}

function buildActivityItems(
  activity: DashboardActivitySource[],
  scopeProjectId: string | null
): DashboardActivityItem[] {
  const teamHref = scopeProjectId ? `/team?project=${scopeProjectId}` : "/team";
  const projectHref = scopeProjectId ? `/project?id=${scopeProjectId}` : "/projects";

  return [...activity]
    .sort((a, b) => toTime(b.createdAt) - toTime(a.createdAt))
    .slice(0, 5)
    .map((event) => ({
      id: event.id,
      type: event.type,
      title: event.title,
      message: event.message,
      href:
        event.type === "agent_started" ||
        event.type === "agent_completed" ||
        event.type === "agent_failed"
          ? "/agents"
          : event.type === "review_completed"
            ? "/agents"
            : event.type === "task_created" || event.type === "task_assigned"
              ? projectHref
              : teamHref,
      createdAt: event.createdAt,
    }));
}

function buildRiskItems(input: {
  tasks: DashboardTaskSource[];
  activity: DashboardActivitySource[];
  scopeProjectId: string | null;
  now: string;
}): DashboardRiskItem[] {
  const risks: DashboardRiskItem[] = [];
  const projectHref = input.scopeProjectId
    ? `/project?id=${input.scopeProjectId}`
    : "/projects";
  const teamHref = input.scopeProjectId
    ? `/team?project=${input.scopeProjectId}`
    : "/team";

  for (const task of input.tasks) {
    if (task.progress?.healthStatus === "stalled") {
      risks.push({
        id: `stalled-${task.id}`,
        kind: "stalled-task",
        title: "Task stalled",
        description: `${task.title} is stalled (${formatProgressReason(task.progress.riskReason || "no_recent_update")}).`,
        href: projectHref,
      });
      continue;
    }

    if (task.progress?.healthStatus === "warning") {
      const isAwaitingReview = task.progress.riskReason === "awaiting_review";
      const isNoAssignee = task.progress.riskReason === "no_assignee";
      risks.push({
        id: `warning-${task.id}`,
        kind: isAwaitingReview ? "review-pressure" : "unassigned-work",
        title:
          isAwaitingReview
            ? "Review attention needed"
            : isNoAssignee
              ? "Unassigned work detected"
              : "Task needs attention",
        description: isNoAssignee
          ? `${task.title} is waiting for an owner.`
          : `${task.title} is at risk (${formatProgressReason(task.progress.riskReason || "warning")}).`,
        href: isAwaitingReview ? "/agents" : teamHref,
      });
    }

    if (task.status === "in_review") {
      const ageMs = toTime(input.now) - toTime(task.updatedAt || task.createdAt);
      if (ageMs >= STALLED_MS) {
        risks.push({
          id: `stalled-${task.id}`,
          kind: "stalled-task",
          title: "Task stalled in review",
          description: `${task.title} has not moved for several days.`,
          href: projectHref,
        });
      }
    }

    if (!task.assigneeId && task.progress?.riskReason !== "no_assignee") {
      risks.push({
        id: `unassigned-${task.id}`,
        kind: "unassigned-work",
        title: "Unassigned work detected",
        description: `${task.title} is ready but has no owner.`,
        href: teamHref,
      });
    }
  }

  if (input.tasks.filter((task) => task.status === "in_review").length >= 3) {
    risks.push({
      id: "review-pressure",
      kind: "review-pressure",
      title: "Review queue is building up",
      description: "Several tasks are waiting in review and may need extra reviewer capacity.",
      href: "/agents",
    });
  }

  if (input.activity.some((event) => event.type === "budget_warning")) {
    risks.push({
      id: "budget-pressure",
      kind: "budget-pressure",
      title: "Budget pressure detected",
      description: "Recent activity includes a budget warning that needs follow-up.",
      href: "/cost",
    });
  }

  return risks.slice(0, 5);
}

export function buildDashboardSummary(
  input: BuildDashboardSummaryInput
): DashboardSummary {
  const now = input.now ?? new Date().toISOString();
  const roster = summarizeMemberRoster({
    members: input.members,
    tasks: input.tasks,
    agents: input.agents,
    activity: input.activity,
  });

  const activeAgents = input.agents.filter((agent) =>
    ACTIVE_AGENT_STATUSES.has(agent.status)
  );
  const weeklyTaskSpend = input.tasks.reduce(
    (sum, task) => sum + (task.spentUsd || 0),
    0
  );
  const weeklyAgentSpend = activeAgents.reduce(
    (sum, agent) => sum + (agent.costUsd || 0),
    0
  );
  const weeklyActivitySpend = input.activity.some(
    (event) => event.type === "budget_warning"
  )
    ? 0
    : 0;

  return {
    scope: {
      projectId: input.scopeProjectId,
      projectName: input.scopeProjectName,
      projectsCount: input.projectsCount,
    },
    headline: {
      activeAgents: activeAgents.length,
      tasksInProgress: input.tasks.filter((task) => task.status === "in_progress").length,
      pendingReviews: input.tasks.filter((task) => task.status === "in_review").length,
      weeklyCost: Number((weeklyTaskSpend + weeklyAgentSpend + weeklyActivitySpend).toFixed(2)),
    },
    progress: {
      total: input.tasks.length,
      inbox: input.tasks.filter((task) => task.status === "inbox").length,
      triaged: input.tasks.filter((task) => task.status === "triaged").length,
      assigned: input.tasks.filter((task) => task.status === "assigned").length,
      inProgress: input.tasks.filter((task) => task.status === "in_progress").length,
      inReview: input.tasks.filter((task) => task.status === "in_review").length,
      done: input.tasks.filter((task) => task.status === "done").length,
    },
    team: {
      totalMembers: roster.length,
      activeHumans: roster.filter(
        (member) => member.type === "human" && member.isActive
      ).length,
      activeAgents: roster.filter(
        (member) => member.type === "agent" && member.isActive
      ).length,
      activeAgentRuns: activeAgents.length,
      overloadedMembers: roster.filter(
        (member) =>
          member.workload.inReviewTasks + member.workload.inProgressTasks >= 3
      ).length,
    },
    activity: buildActivityItems(input.activity, input.scopeProjectId),
    risks: buildRiskItems({
      tasks: input.tasks,
      activity: input.activity,
      scopeProjectId: input.scopeProjectId,
      now,
    }),
    links: {
      projects: "/projects",
      team: input.scopeProjectId ? `/team?project=${input.scopeProjectId}` : "/team",
      agents: "/agents",
      reviews: "/reviews",
    },
  };
}

export function isRecentTimestamp(value: string | null | undefined): boolean {
  if (!value) return false;
  return Date.now() - toTime(value) <= WEEK_MS;
}
