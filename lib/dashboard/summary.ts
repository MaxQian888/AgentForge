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
import { buildProjectScopedHref } from "@/lib/route-hrefs";

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
  roleBindingState?: "ready" | "missing" | "stale" | null;
  readinessState?: "ready" | "incomplete" | null;
  readinessLabel?: string | null;
  readinessMissing?: string[];
  agentSummary?: string[];
}

export type TeamAttentionCategory = "setup-required" | "inactive" | "suspended";

export interface TeamAttentionGroup {
  id: TeamAttentionCategory;
  label: string;
  count: number;
  memberIds: string[];
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
  bootstrap?: DashboardBootstrapSummary;
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
  projectMeta?: {
    id: string;
    name: string;
    repoUrl?: string;
    settings?: {
      codingAgent?: {
        runtime?: string;
        provider?: string;
        model?: string;
      };
    };
  } | null;
  sprintCount?: number | null;
  docsTemplateCount?: number | null;
  workflowTemplateCount?: number | null;
}

export type DashboardBootstrapPhaseId =
  | "governance"
  | "team"
  | "playbooks"
  | "planning"
  | "delivery";

export type DashboardBootstrapPhaseState = "ready" | "attention" | "blocked";

export interface DashboardBootstrapPhase {
  id: DashboardBootstrapPhaseId;
  title: string;
  state: DashboardBootstrapPhaseState;
  reason: string;
  href: string;
  actionLabel: string;
}

export interface DashboardBootstrapAction {
  id:
    | "configure-governance"
    | "add-member"
    | "open-playbooks"
    | "create-sprint"
    | "open-task-workspace";
  label: string;
  href: string;
}

export interface DashboardBootstrapSummary {
  unresolvedCount: number;
  phases: DashboardBootstrapPhase[];
  nextActions: DashboardBootstrapAction[];
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
    roleBindingState:
      member.type === "agent"
        ? agentProfile?.roleId
          ? "ready"
          : "missing"
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

export function getTeamMemberAttentionCategories(
  member: TeamMember
): TeamAttentionCategory[] {
  const categories: TeamAttentionCategory[] = [];

  if (
    member.type === "agent" &&
    (member.readinessMissing ?? []).some((field) =>
      ["runtime", "provider", "model", "roleId"].includes(field)
    )
  ) {
    categories.push("setup-required");
  }

  if (member.status === "inactive") {
    categories.push("inactive");
  }

  if (member.status === "suspended") {
    categories.push("suspended");
  }

  return categories;
}

export function buildTeamAttentionGroups(
  members: TeamMember[]
): TeamAttentionGroup[] {
  const groups: TeamAttentionGroup[] = [
    {
      id: "setup-required",
      label: "Setup Required",
      count: 0,
      memberIds: [],
    },
    {
      id: "inactive",
      label: "Inactive",
      count: 0,
      memberIds: [],
    },
    {
      id: "suspended",
      label: "Suspended",
      count: 0,
      memberIds: [],
    },
  ];

  const groupMap = new Map(groups.map((group) => [group.id, group]));
  for (const member of members) {
    for (const category of getTeamMemberAttentionCategories(member)) {
      const group = groupMap.get(category);
      if (!group) continue;
      group.count += 1;
      group.memberIds.push(member.id);
    }
  }

  return groups.filter((group) => group.count > 0);
}

export function getQuickLifecycleTargetStatus(
  member: TeamMember
): MemberStatus | null {
  if (member.status === "active") {
    return "suspended";
  }
  if (member.status === "inactive" || member.status === "suspended") {
    return "active";
  }
  return null;
}

export function getQuickLifecycleLabel(member: TeamMember): string | null {
  const status = getQuickLifecycleTargetStatus(member);
  if (!status) return null;
  return status === "active" ? "Activate" : "Suspend";
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
    ? buildProjectScopedHref("/project", {
        projectId: input.scopeProjectId,
        projectParam: "id",
      })
    : "/projects";
  const teamHref = input.scopeProjectId
    ? buildProjectScopedHref("/team", { projectId: input.scopeProjectId })
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

function buildBootstrapSummary(
  input: BuildDashboardSummaryInput,
): DashboardBootstrapSummary | undefined {
  if (!input.scopeProjectId) {
    return undefined;
  }

  const codingAgent = input.projectMeta?.settings?.codingAgent;
  const hasCodingAgentSelection = Boolean(
    codingAgent?.runtime && codingAgent?.provider && codingAgent?.model,
  );
  const hasRepository = Boolean(input.projectMeta?.repoUrl?.trim());

  const governanceHref = buildProjectScopedHref("/settings", {
    projectId: input.scopeProjectId,
    params: {
      section: hasRepository ? "coding-agent" : "repository",
    },
  });
  const teamHref = buildProjectScopedHref("/team", {
    projectId: input.scopeProjectId,
    params: { focus: "add-member" },
  });
  const playbooksHref = buildProjectScopedHref("/workflow", {
    projectId: input.scopeProjectId,
    params: { tab: "templates" },
  });
  const planningHref = buildProjectScopedHref("/sprints", {
    projectId: input.scopeProjectId,
    params: { action: "create-sprint" },
  });
  const deliveryHref = buildProjectScopedHref("/project", {
    projectId: input.scopeProjectId,
    projectParam: "id",
  });

  const phases: DashboardBootstrapPhase[] = [
    {
      id: "governance",
      title: "Governance",
      state: hasRepository && hasCodingAgentSelection ? "ready" : "attention",
      reason:
        hasRepository && hasCodingAgentSelection
          ? "Repository and coding-agent defaults are configured."
          : "Repository or coding-agent defaults still need configuration.",
      href: governanceHref,
      actionLabel: "Configure Governance",
    },
    {
      id: "team",
      title: "Team",
      state: input.members.length > 0 ? "ready" : "attention",
      reason:
        input.members.length > 0
          ? "Human and agent collaborators are available."
          : "Add the first human or agent collaborator.",
      href: teamHref,
      actionLabel: "Add First Member",
    },
    {
      id: "playbooks",
      title: "Playbooks",
      state:
        input.docsTemplateCount == null || input.workflowTemplateCount == null
          ? "blocked"
          : input.docsTemplateCount > 0 && input.workflowTemplateCount > 0
            ? "ready"
            : "attention",
      reason:
        input.docsTemplateCount == null || input.workflowTemplateCount == null
          ? "Template baselines are unavailable right now."
          : input.docsTemplateCount > 0 && input.workflowTemplateCount > 0
            ? "Document and workflow templates are available."
            : "Document or workflow template baselines still need attention.",
      href: playbooksHref,
      actionLabel: "Open Templates",
    },
    {
      id: "planning",
      title: "Planning",
      state:
        (input.sprintCount ?? 0) > 0 || input.tasks.length > 0
          ? "ready"
          : "attention",
      reason:
        (input.sprintCount ?? 0) > 0 || input.tasks.length > 0
          ? "Planning already has an initial sprint or backlog."
          : "Create the first sprint or planning container for the backlog.",
      href: planningHref,
      actionLabel: "Create First Sprint",
    },
    {
      id: "delivery",
      title: "Delivery",
      state: input.tasks.length > 0 ? "ready" : "attention",
      reason:
        input.tasks.length > 0
          ? "The task workspace already has work items to manage."
          : "Open the task workspace and create the first work item.",
      href: deliveryHref,
      actionLabel: "Open Task Workspace",
    },
  ];

  const nextActions = phases
    .filter((phase) => phase.state !== "ready")
    .map((phase): DashboardBootstrapAction => {
      switch (phase.id) {
        case "governance":
          return { id: "configure-governance", label: phase.actionLabel, href: phase.href };
        case "team":
          return { id: "add-member", label: phase.actionLabel, href: phase.href };
        case "playbooks":
          return { id: "open-playbooks", label: phase.actionLabel, href: phase.href };
        case "planning":
          return { id: "create-sprint", label: phase.actionLabel, href: phase.href };
        case "delivery":
        default:
          return { id: "open-task-workspace", label: phase.actionLabel, href: phase.href };
      }
    });

  return {
    unresolvedCount: phases.filter((phase) => phase.state !== "ready").length,
    phases,
    nextActions,
  };
}

export function applyRoleRegistryState(
  members: TeamMember[],
  roles: Array<{ metadata: { id: string } }>,
): TeamMember[] {
  const knownRoleIds = new Set(roles.map((role) => role.metadata.id));

  return members.map((member) => {
    if (member.type !== "agent") {
      return member;
    }

    const boundRoleId = member.agentProfile?.roleId?.trim() ?? "";
    if (!boundRoleId) {
      return {
        ...member,
        roleBindingState: "missing",
        roleBindingLabel: "Unbound role",
      };
    }

    if (!knownRoleIds.has(boundRoleId)) {
      const missing = Array.from(new Set([...(member.readinessMissing ?? []), "roleId"]));
      return {
        ...member,
        roleBindingState: "stale",
        roleBindingLabel: `${boundRoleId} (stale)`,
        readinessState: "incomplete",
        readinessLabel: "Stale role binding",
        readinessMissing: missing,
      };
    }

    return {
      ...member,
      roleBindingState: "ready",
      roleBindingLabel: boundRoleId,
    };
  });
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
    bootstrap: buildBootstrapSummary(input),
    links: {
      projects: "/projects",
      team: input.scopeProjectId
        ? buildProjectScopedHref("/team", { projectId: input.scopeProjectId })
        : "/team",
      agents: input.scopeProjectId
        ? buildProjectScopedHref("/agents", { projectId: input.scopeProjectId })
        : "/agents",
      reviews: input.scopeProjectId
        ? buildProjectScopedHref("/reviews", { projectId: input.scopeProjectId })
        : "/reviews",
    },
  };
}

export function isRecentTimestamp(value: string | null | undefined): boolean {
  if (!value) return false;
  return Date.now() - toTime(value) <= WEEK_MS;
}
