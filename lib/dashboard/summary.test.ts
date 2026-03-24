import {
  buildDashboardSummary,
  summarizeMemberRoster,
  type DashboardActivitySource,
  type DashboardTaskSource,
  type DashboardAgentSource,
  type DashboardMemberSource,
} from "./summary";

describe("dashboard summary helpers", () => {
  const projectId = "project-1";

  const tasks: DashboardTaskSource[] = [
    {
      id: "task-1",
      projectId,
      title: "Stalled review item",
      status: "in_review",
      priority: "high",
      assigneeId: "member-human-1",
      assigneeType: "human",
      spentUsd: 12,
      progress: {
        healthStatus: "stalled",
        riskReason: "awaiting_review",
        lastActivityAt: "2026-03-10T10:00:00.000Z",
      },
      updatedAt: "2026-03-10T10:00:00.000Z",
      createdAt: "2026-03-01T10:00:00.000Z",
    },
    {
      id: "task-2",
      projectId,
      title: "Agent implementation",
      status: "in_progress",
      priority: "medium",
      assigneeId: "member-agent-1",
      assigneeType: "agent",
      spentUsd: 7.5,
      updatedAt: "2026-03-24T08:00:00.000Z",
      createdAt: "2026-03-24T07:30:00.000Z",
    },
    {
      id: "task-3",
      projectId,
      title: "Unassigned backlog",
      status: "assigned",
      priority: "urgent",
      assigneeId: null,
      assigneeType: null,
      spentUsd: 0,
      progress: {
        healthStatus: "warning",
        riskReason: "no_assignee",
        lastActivityAt: "2026-03-23T08:00:00.000Z",
      },
      updatedAt: "2026-03-23T08:00:00.000Z",
      createdAt: "2026-03-22T08:00:00.000Z",
    },
  ];

  const agents: DashboardAgentSource[] = [
    {
      id: "run-1",
      taskId: "task-2",
      memberId: "member-agent-1",
      status: "running",
      costUsd: 15.25,
      turnCount: 8,
      updatedAt: "2026-03-24T09:00:00.000Z",
      createdAt: "2026-03-24T07:35:00.000Z",
      startedAt: "2026-03-24T07:35:00.000Z",
    },
  ];

  const members: DashboardMemberSource[] = [
    {
      id: "member-human-1",
      projectId,
      name: "Alice",
      type: "human",
      role: "frontend-developer",
      email: "alice@example.com",
      skills: ["react", "testing"],
      isActive: true,
      createdAt: "2026-03-01T08:00:00.000Z",
    },
    {
      id: "member-agent-1",
      projectId,
      name: "Review Bot",
      type: "agent",
      role: "code-reviewer",
      email: "",
      skills: ["review", "security"],
      isActive: true,
      createdAt: "2026-03-01T08:00:00.000Z",
    },
  ];

  const activity: DashboardActivitySource[] = [
    {
      id: "notification-1",
      type: "review_completed",
      title: "Deep review completed",
      message: "Task task-1 is waiting for a reviewer.",
      createdAt: "2026-03-24T09:30:00.000Z",
      targetId: "member-human-1",
    },
    {
      id: "notification-2",
      type: "budget_warning",
      title: "Budget warning",
      message: "Weekly spend reached 80%.",
      createdAt: "2026-03-24T06:30:00.000Z",
      targetId: "member-human-1",
    },
  ];

  it("builds progress, cost, activity, and risk insights for the selected project scope", () => {
    const summary = buildDashboardSummary({
      scopeProjectId: projectId,
      scopeProjectName: "AgentForge",
      projectsCount: 1,
      tasks,
      agents,
      members,
      activity,
      now: "2026-03-24T12:00:00.000Z",
    });

    expect(summary.scope.projectId).toBe(projectId);
    expect(summary.headline.activeAgents).toBe(1);
    expect(summary.headline.tasksInProgress).toBe(1);
    expect(summary.headline.pendingReviews).toBe(1);
    expect(summary.headline.weeklyCost).toBe(34.75);
    expect(summary.progress.total).toBe(3);
    expect(summary.progress.assigned).toBe(1);
    expect(summary.progress.inReview).toBe(1);
    expect(summary.team.totalMembers).toBe(2);
    expect(summary.team.activeHumans).toBe(1);
    expect(summary.team.activeAgents).toBe(1);
    expect(summary.activity[0]).toMatchObject({
      id: "notification-1",
      title: "Deep review completed",
    });
    expect(summary.risks.map((risk) => risk.kind)).toEqual(
      expect.arrayContaining(["stalled-task", "unassigned-work", "budget-pressure"])
    );
    expect(summary.risks).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          kind: "stalled-task",
          description: expect.stringContaining("awaiting review"),
        }),
      ])
    );
  });

  it("summarizes member workload with shared member identities", () => {
    const roster = summarizeMemberRoster({
      members,
      tasks,
      agents,
      activity,
    });

    expect(roster).toHaveLength(2);
    expect(roster[0]).toMatchObject({
      id: "member-human-1",
      status: "active",
      workload: {
        assignedTasks: 1,
        inProgressTasks: 0,
        inReviewTasks: 1,
        activeAgentRuns: 0,
      },
    });
    expect(roster[1]).toMatchObject({
      id: "member-agent-1",
      type: "agent",
      workload: {
        assignedTasks: 1,
        inProgressTasks: 1,
        inReviewTasks: 0,
        activeAgentRuns: 1,
      },
    });
  });
});
