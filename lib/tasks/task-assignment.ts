import type { TeamMember, TeamMemberWorkload } from "@/lib/dashboard/summary";
import type { Agent } from "@/lib/stores/agent-store";
import type { Task } from "@/lib/stores/task-store";

export interface AssignmentRecommendation {
  member: TeamMember;
  workload: TeamMemberWorkload;
  score: number;
  skillMatches: string[];
  reasons: string[];
}

const ACTIVE_TASK_STATUSES = new Set<Task["status"]>([
  "inbox",
  "triaged",
  "assigned",
  "in_progress",
  "in_review",
  "blocked",
  "changes_requested",
  "budget_exceeded",
]);

const ACTIVE_AGENT_STATUSES = new Set<Agent["status"]>(["starting", "running", "paused"]);

const CAPABILITY_GROUPS = [
  {
    name: "frontend",
    keywords: [
      "frontend",
      "ui",
      "ux",
      "react",
      "next",
      "component",
      "layout",
      "page",
      "board",
      "timeline",
      "calendar",
      "dashboard",
      "tailwind",
      "css",
    ],
  },
  {
    name: "backend",
    keywords: [
      "backend",
      "api",
      "service",
      "handler",
      "route",
      "endpoint",
      "server",
      "bridge",
      "go",
      "database",
      "sql",
      "repository",
      "migration",
      "worker",
    ],
  },
  {
    name: "testing",
    keywords: [
      "test",
      "testing",
      "jest",
      "vitest",
      "coverage",
      "qa",
      "regression",
      "e2e",
      "playwright",
      "assertion",
    ],
  },
  {
    name: "docs",
    keywords: ["doc", "docs", "documentation", "readme", "spec", "prd", "guide"],
  },
  {
    name: "infra",
    keywords: [
      "deploy",
      "deployment",
      "release",
      "docker",
      "compose",
      "ci",
      "workflow",
      "build",
      "infra",
      "ops",
    ],
  },
  {
    name: "security",
    keywords: [
      "auth",
      "security",
      "audit",
      "permission",
      "token",
      "oauth",
      "credential",
      "secret",
    ],
  },
  {
    name: "review",
    keywords: ["review", "lint", "typecheck", "types", "quality", "compliance"],
  },
];

const AGENT_FRIENDLY_KEYWORDS = new Set([
  "test",
  "testing",
  "coverage",
  "docs",
  "documentation",
  "readme",
  "lint",
  "typecheck",
  "refactor",
  "polish",
  "automation",
  "fixture",
  "regression",
  "scaffold",
  "crud",
]);

const HUMAN_FRIENDLY_KEYWORDS = new Set([
  "architecture",
  "design",
  "security",
  "audit",
  "strategy",
  "research",
  "product",
  "stakeholder",
  "planning",
]);

function tokenize(value: string): string[] {
  return value
    .toLowerCase()
    .split(/[^a-z0-9]+/g)
    .map((token) => token.trim())
    .filter(Boolean);
}

function taskTokens(task: Task): string[] {
  return tokenize(`${task.title} ${task.description}`);
}

function memberSearchText(member: TeamMember): string {
  return [member.name, member.role, ...member.skills].join(" ").toLowerCase();
}

function buildMemberWorkloads(
  members: TeamMember[],
  tasks: Task[],
  agents: Agent[]
): Map<string, TeamMemberWorkload> {
  const workloads = new Map<string, TeamMemberWorkload>();

  for (const member of members) {
    workloads.set(member.id, {
      assignedTasks: 0,
      inProgressTasks: 0,
      inReviewTasks: 0,
      activeAgentRuns: 0,
    });
  }

  for (const task of tasks) {
    if (!task.assigneeId || !ACTIVE_TASK_STATUSES.has(task.status)) {
      continue;
    }

    const workload = workloads.get(task.assigneeId);
    if (!workload) {
      continue;
    }

    workload.assignedTasks += 1;
    if (task.status === "in_progress") {
      workload.inProgressTasks += 1;
    }
    if (task.status === "in_review") {
      workload.inReviewTasks += 1;
    }
  }

  for (const agent of agents) {
    if (!ACTIVE_AGENT_STATUSES.has(agent.status)) {
      continue;
    }

    const workload = workloads.get(agent.memberId);
    if (!workload) {
      continue;
    }

    workload.activeAgentRuns += 1;
  }

  return workloads;
}

function capabilityMatches(tokens: string[], member: TeamMember): string[] {
  const haystack = memberSearchText(member);
  const matches = new Set<string>();

  for (const group of CAPABILITY_GROUPS) {
    if (
      group.keywords.some((keyword) => tokens.includes(keyword)) &&
      group.keywords.some((keyword) => haystack.includes(keyword))
    ) {
      matches.add(group.name);
    }
  }

  return [...matches];
}

function assignmentPreference(tokens: string[], member: TeamMember): { bonus: number; reason: string | null } {
  const agentFriendly = tokens.some((token) => AGENT_FRIENDLY_KEYWORDS.has(token));
  const humanFriendly = tokens.some((token) => HUMAN_FRIENDLY_KEYWORDS.has(token));

  if (agentFriendly && !humanFriendly && member.type === "agent") {
    return { bonus: 12, reason: "Fits agent-executable work" };
  }

  if (humanFriendly && !agentFriendly && member.type === "human") {
    return { bonus: 12, reason: "Fits human-led work" };
  }

  return { bonus: 0, reason: null };
}

function workloadPenalty(workload: TeamMemberWorkload): number {
  return (
    workload.assignedTasks * 2 +
    workload.inProgressTasks * 5 +
    workload.inReviewTasks * 3 +
    workload.activeAgentRuns * 6
  );
}

function poolCapacityPenalty(
  member: TeamMember,
  agents: Agent[],
  poolMax: number
): { penalty: number; reason: string | null } {
  if (member.type !== "agent") return { penalty: 0, reason: null };
  const activeRuns = agents.filter(
    (a) =>
      a.memberId === member.id &&
      (a.status === "starting" || a.status === "running")
  ).length;
  if (poolMax > 0 && activeRuns >= poolMax) {
    return { penalty: 20, reason: "At pool capacity" };
  }
  if (poolMax > 0 && activeRuns >= poolMax - 1) {
    return { penalty: 8, reason: "Near pool capacity" };
  }
  return { penalty: 0, reason: null };
}

function memoryWarmthBonus(
  member: TeamMember,
  task: Task,
  tasks: Task[]
): { bonus: number; reason: string | null } {
  const projectTasks = tasks.filter(
    (t) =>
      t.projectId === task.projectId &&
      t.assigneeId === member.id &&
      (t.status === "done" || t.status === "in_progress" || t.status === "in_review")
  );
  if (projectTasks.length >= 3) {
    return { bonus: 10, reason: "Strong project familiarity" };
  }
  if (projectTasks.length >= 1) {
    return { bonus: 5, reason: "Prior project experience" };
  }
  return { bonus: 0, reason: null };
}

function workloadReason(workload: TeamMemberWorkload): string {
  if (
    workload.assignedTasks === 0 &&
    workload.inProgressTasks === 0 &&
    workload.inReviewTasks === 0 &&
    workload.activeAgentRuns === 0
  ) {
    return "Current load is clear";
  }

  const parts: string[] = [];
  if (workload.inProgressTasks > 0) {
    parts.push(`${workload.inProgressTasks} in progress`);
  }
  if (workload.inReviewTasks > 0) {
    parts.push(`${workload.inReviewTasks} in review`);
  }
  if (workload.activeAgentRuns > 0) {
    parts.push(`${workload.activeAgentRuns} active runs`);
  }
  if (parts.length === 0) {
    parts.push(`${workload.assignedTasks} assigned`);
  }

  return `Current load: ${parts.join(", ")}`;
}

export function recommendTaskAssignees(
  task: Task,
  members: TeamMember[],
  tasks: Task[],
  agents: Agent[],
  limit = 3,
  poolMax = 3
): AssignmentRecommendation[] {
  const tokens = taskTokens(task);
  const workloads = buildMemberWorkloads(members, tasks, agents);

  return members
    .filter((member) => member.isActive)
    .map((member) => {
      const workload =
        workloads.get(member.id) ?? {
          assignedTasks: 0,
          inProgressTasks: 0,
          inReviewTasks: 0,
          activeAgentRuns: 0,
        };
      const matches = capabilityMatches(tokens, member);
      const preference = assignmentPreference(tokens, member);
      const sameAssigneeBonus = task.assigneeId === member.id ? 4 : 0;
      const capacity = poolCapacityPenalty(member, agents, poolMax);
      const warmth = memoryWarmthBonus(member, task, tasks);
      const score =
        60 +
        matches.length * 14 +
        preference.bonus +
        sameAssigneeBonus +
        warmth.bonus -
        workloadPenalty(workload) -
        capacity.penalty;

      const reasons = [
        matches.length > 0 ? `Skills match: ${matches.join(", ")}` : "General contributor fit",
        workloadReason(workload),
        preference.reason,
        capacity.reason,
        warmth.reason,
      ].filter((reason): reason is string => Boolean(reason));

      return {
        member,
        workload,
        score,
        skillMatches: matches,
        reasons,
      };
    })
    .sort((left, right) => {
      if (right.score !== left.score) {
        return right.score - left.score;
      }
      return left.member.name.localeCompare(right.member.name);
    })
    .slice(0, limit);
}
