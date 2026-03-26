import { recommendTaskAssignees } from "./task-assignment";
import type { TeamMember } from "@/lib/dashboard/summary";
import type { Agent } from "@/lib/stores/agent-store";
import type { Task } from "@/lib/stores/task-store";

const members: TeamMember[] = [
  {
    id: "member-agent-ui",
    projectId: "project-1",
    name: "UI Bot",
    type: "agent",
    typeLabel: "Agent",
    role: "Frontend automation agent",
    email: "",
    avatarUrl: "",
    skills: ["frontend", "calendar", "automation", "testing"],
    isActive: true,
    status: "active",
    createdAt: "2026-03-24T09:00:00.000Z",
    lastActivityAt: null,
    workload: {
      assignedTasks: 0,
      inProgressTasks: 0,
      inReviewTasks: 0,
      activeAgentRuns: 0,
    },
  },
  {
    id: "member-human-backend",
    projectId: "project-1",
    name: "Backend Bob",
    type: "human",
    typeLabel: "Human",
    role: "Backend engineer",
    email: "",
    avatarUrl: "",
    skills: ["backend", "api", "database"],
    isActive: true,
    status: "active",
    createdAt: "2026-03-24T09:00:00.000Z",
    lastActivityAt: null,
    workload: {
      assignedTasks: 0,
      inProgressTasks: 0,
      inReviewTasks: 0,
      activeAgentRuns: 0,
    },
  },
  {
    id: "member-human-ui-busy",
    projectId: "project-1",
    name: "Alice",
    type: "human",
    typeLabel: "Human",
    role: "Frontend engineer",
    email: "",
    avatarUrl: "",
    skills: ["frontend", "calendar", "timeline"],
    isActive: true,
    status: "active",
    createdAt: "2026-03-24T09:00:00.000Z",
    lastActivityAt: null,
    workload: {
      assignedTasks: 0,
      inProgressTasks: 0,
      inReviewTasks: 0,
      activeAgentRuns: 0,
    },
  },
];

const tasks: Task[] = [
  {
    id: "task-1",
    projectId: "project-1",
    title: "Calendar polish",
    description: "Tighten the frontend calendar experience and add regression tests.",
    status: "triaged",
    priority: "medium",
    assigneeId: null,
    assigneeType: null,
    assigneeName: null,
    cost: null,
    budgetUsd: 0,
    spentUsd: 0,
    agentBranch: "",
    agentWorktree: "",
    agentSessionId: "",
    blockedBy: [],
    plannedStartAt: null,
    plannedEndAt: null,
    progress: null,
    createdAt: "2026-03-24T10:00:00.000Z",
    updatedAt: "2026-03-24T10:00:00.000Z",
  },
  {
    id: "task-2",
    projectId: "project-1",
    title: "Timeline cleanup",
    description: "Fix the current frontend lane rendering.",
    status: "in_progress",
    priority: "high",
    assigneeId: "member-human-ui-busy",
    assigneeType: "human",
    assigneeName: "Alice",
    cost: null,
    budgetUsd: 0,
    spentUsd: 0,
    agentBranch: "",
    agentWorktree: "",
    agentSessionId: "",
    blockedBy: [],
    plannedStartAt: null,
    plannedEndAt: null,
    progress: null,
    createdAt: "2026-03-24T09:00:00.000Z",
    updatedAt: "2026-03-24T11:00:00.000Z",
  },
];

const agents: Agent[] = [
  {
    id: "run-1",
    taskId: "task-2",
    taskTitle: "Timeline cleanup",
    memberId: "member-human-ui-busy",
    roleId: "",
    roleName: "",
    status: "running",
    runtime: "codex",
    provider: "anthropic",
    model: "claude",
    turns: 3,
    cost: 0.3,
    budget: 5,
    worktreePath: "",
    branchName: "",
    sessionId: "",
    lastActivity: "2026-03-24T11:05:00.000Z",
    startedAt: "2026-03-24T10:30:00.000Z",
    createdAt: "2026-03-24T10:30:00.000Z",
    completedAt: null,
    canResume: false,
    memoryStatus: "none",
  },
];

describe("recommendTaskAssignees", () => {
  it("prefers a skill-matched low-load member", () => {
    const recommendations = recommendTaskAssignees(tasks[0], members, tasks, agents);

    expect(recommendations[0]?.member.id).toBe("member-agent-ui");
    expect(recommendations[0]?.skillMatches).toEqual(
      expect.arrayContaining(["frontend", "testing"])
    );
  });

  it("penalizes busy members with similar skills", () => {
    const recommendations = recommendTaskAssignees(tasks[0], members, tasks, agents);

    const busyMember = recommendations.find(
      (recommendation) => recommendation.member.id === "member-human-ui-busy"
    );

    expect(busyMember).toBeDefined();
    expect(busyMember?.reasons.join(" ")).toContain("Current load:");
    expect(recommendations[0].score).toBeGreaterThan(busyMember?.score ?? 0);
  });

  it("penalizes agent members at pool capacity", () => {
    const saturatedAgents: Agent[] = [
      ...agents,
      {
        id: "run-a1",
        taskId: "task-1",
        taskTitle: "Calendar polish",
        memberId: "member-agent-ui",
        roleId: "",
        roleName: "",
        status: "running",
        runtime: "codex",
        provider: "anthropic",
        model: "claude",
        turns: 1,
        cost: 0.1,
        budget: 5,
        worktreePath: "",
        branchName: "",
        sessionId: "",
        lastActivity: "2026-03-24T11:05:00.000Z",
        startedAt: "2026-03-24T10:30:00.000Z",
        createdAt: "2026-03-24T10:30:00.000Z",
        completedAt: null,
        canResume: false,
        memoryStatus: "none",
      },
      {
        id: "run-a2",
        taskId: "task-1",
        taskTitle: "Calendar polish",
        memberId: "member-agent-ui",
        roleId: "",
        roleName: "",
        status: "running",
        runtime: "codex",
        provider: "anthropic",
        model: "claude",
        turns: 1,
        cost: 0.1,
        budget: 5,
        worktreePath: "",
        branchName: "",
        sessionId: "",
        lastActivity: "2026-03-24T11:05:00.000Z",
        startedAt: "2026-03-24T10:30:00.000Z",
        createdAt: "2026-03-24T10:30:00.000Z",
        completedAt: null,
        canResume: false,
        memoryStatus: "none",
      },
    ];

    // poolMax = 2, agent has 2 running → at capacity
    const recommendations = recommendTaskAssignees(
      tasks[0],
      members,
      tasks,
      saturatedAgents,
      3,
      2
    );

    const agentMember = recommendations.find(
      (r) => r.member.id === "member-agent-ui"
    );
    expect(agentMember).toBeDefined();
    expect(agentMember?.reasons.join(" ")).toContain("pool capacity");
  });

  it("gives memory warmth bonus for project familiarity", () => {
    const doneTasks: Task[] = [
      ...tasks,
      {
        id: "task-done-1",
        projectId: "project-1",
        title: "Old task 1",
        description: "",
        status: "done",
        priority: "medium",
        assigneeId: "member-human-backend",
        assigneeType: "human",
        assigneeName: "Backend Bob",
        cost: null,
        budgetUsd: 0,
        spentUsd: 0,
        agentBranch: "",
        agentWorktree: "",
        agentSessionId: "",
        blockedBy: [],
        plannedStartAt: null,
        plannedEndAt: null,
        progress: null,
        createdAt: "2026-03-20T10:00:00.000Z",
        updatedAt: "2026-03-20T10:00:00.000Z",
      },
      {
        id: "task-done-2",
        projectId: "project-1",
        title: "Old task 2",
        description: "",
        status: "done",
        priority: "medium",
        assigneeId: "member-human-backend",
        assigneeType: "human",
        assigneeName: "Backend Bob",
        cost: null,
        budgetUsd: 0,
        spentUsd: 0,
        agentBranch: "",
        agentWorktree: "",
        agentSessionId: "",
        blockedBy: [],
        plannedStartAt: null,
        plannedEndAt: null,
        progress: null,
        createdAt: "2026-03-21T10:00:00.000Z",
        updatedAt: "2026-03-21T10:00:00.000Z",
      },
      {
        id: "task-done-3",
        projectId: "project-1",
        title: "Old task 3",
        description: "",
        status: "done",
        priority: "medium",
        assigneeId: "member-human-backend",
        assigneeType: "human",
        assigneeName: "Backend Bob",
        cost: null,
        budgetUsd: 0,
        spentUsd: 0,
        agentBranch: "",
        agentWorktree: "",
        agentSessionId: "",
        blockedBy: [],
        plannedStartAt: null,
        plannedEndAt: null,
        progress: null,
        createdAt: "2026-03-22T10:00:00.000Z",
        updatedAt: "2026-03-22T10:00:00.000Z",
      },
    ];

    // Backend Bob has 3+ done tasks in project-1 → strong familiarity bonus
    const backendTask: Task = {
      id: "task-backend",
      projectId: "project-1",
      title: "API endpoint handler",
      description: "Build the backend route handler.",
      status: "triaged",
      priority: "medium",
      assigneeId: null,
      assigneeType: null,
      assigneeName: null,
      cost: null,
      budgetUsd: 0,
      spentUsd: 0,
      agentBranch: "",
      agentWorktree: "",
      agentSessionId: "",
      blockedBy: [],
      plannedStartAt: null,
      plannedEndAt: null,
      progress: null,
      createdAt: "2026-03-24T10:00:00.000Z",
      updatedAt: "2026-03-24T10:00:00.000Z",
    };

    const recommendations = recommendTaskAssignees(
      backendTask,
      members,
      doneTasks,
      agents
    );

    const bob = recommendations.find((r) => r.member.id === "member-human-backend");
    expect(bob).toBeDefined();
    expect(bob?.reasons.join(" ")).toContain("project familiarity");
  });
});
