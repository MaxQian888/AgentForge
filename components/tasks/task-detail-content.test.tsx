const recommendTaskAssigneesMock = jest.fn();
const getTaskDependencyStateMock = jest.fn();
const normalizePlanningInputMock = jest.fn();

jest.mock("@/components/review/task-review-section", () => ({
  TaskReviewSection: ({ taskId }: { taskId: string }) => (
    <div data-testid="task-review-section">{taskId}</div>
  ),
}));

jest.mock("@/lib/tasks/task-assignment", () => ({
  recommendTaskAssignees: (...args: unknown[]) => recommendTaskAssigneesMock(...args),
}));

jest.mock("@/lib/tasks/task-dependencies", () => ({
  getTaskDependencyState: (...args: unknown[]) => getTaskDependencyStateMock(...args),
}));

jest.mock("@/lib/tasks/task-planning", () => ({
  normalizePlanningInput: (...args: unknown[]) => normalizePlanningInputMock(...args),
}));

import userEvent from "@testing-library/user-event";
import { render, screen, waitFor } from "@testing-library/react";
import { TaskDetailContent } from "./task-detail-content";
import type { TeamMember } from "@/lib/dashboard/summary";
import type { Agent } from "@/lib/stores/agent-store";
import type { Sprint } from "@/lib/stores/sprint-store";
import type { Task } from "@/lib/stores/task-store";

const members: TeamMember[] = [
  {
    id: "member-1",
    projectId: "project-1",
    name: "Alice",
    type: "human",
    typeLabel: "Human",
    role: "Frontend engineer",
    email: "alice@example.com",
    avatarUrl: "",
    skills: ["frontend", "testing"],
    isActive: true,
    status: "active",
    createdAt: "2026-03-25T08:00:00.000Z",
    lastActivityAt: null,
    workload: {
      assignedTasks: 1,
      inProgressTasks: 0,
      inReviewTasks: 0,
      activeAgentRuns: 0,
    },
  },
];

const agents: Agent[] = [
  {
    id: "agent-1",
    taskId: "task-1",
    taskTitle: "Build dashboard",
    memberId: "member-agent",
    roleId: "role-coder",
    roleName: "Coder",
    status: "running",
    runtime: "codex",
    provider: "openai",
    model: "gpt-5.4",
    turns: 3,
    cost: 1.5,
    budget: 3,
    worktreePath: "",
    branchName: "",
    sessionId: "",
    lastActivity: "",
    startedAt: "",
    createdAt: "",
    canResume: false,
    memoryStatus: "none",
  },
];

const sprints: Sprint[] = [
  {
    id: "sprint-1",
    projectId: "project-1",
    name: "Sprint 1",
    startDate: "2026-03-25T00:00:00.000Z",
    endDate: "2026-03-31T00:00:00.000Z",
    status: "active",
    totalBudgetUsd: 20,
    spentUsd: 6,
    createdAt: "2026-03-24T00:00:00.000Z",
  },
];

function makeTask(overrides: Partial<Task> & { id: string; title: string }): Task {
  return {
    id: overrides.id,
    projectId: "project-1",
    title: overrides.title,
    description: "Implement the review dashboard.",
    status: "in_progress",
    priority: "high",
    assigneeId: null,
    assigneeType: null,
    assigneeName: null,
    cost: 1,
    budgetUsd: 5,
    spentUsd: 2.5,
    agentBranch: "agent/task-1",
    agentWorktree: "D:/Project/AgentForge/.worktrees/task-1",
    agentSessionId: "session-1",
    blockedBy: [],
    sprintId: "sprint-1",
    plannedStartAt: "2026-03-25T00:00:00.000Z",
    plannedEndAt: "2026-03-26T00:00:00.000Z",
    progress: {
      lastActivityAt: "2026-03-25T08:00:00.000Z",
      lastActivitySource: "heartbeat",
      lastTransitionAt: "2026-03-25T07:30:00.000Z",
      healthStatus: "warning",
      riskReason: "awaiting_review",
      riskSinceAt: "2026-03-25T07:45:00.000Z",
      lastAlertState: "warning:awaiting_review",
      lastAlertAt: null,
      lastRecoveredAt: null,
    },
    createdAt: "2026-03-25T07:00:00.000Z",
    updatedAt: "2026-03-25T08:00:00.000Z",
    ...overrides,
  };
}

describe("TaskDetailContent", () => {
  beforeEach(() => {
    recommendTaskAssigneesMock.mockReturnValue([
      { member: members[0], reasons: ["frontend fit", "low workload"] },
    ]);
    getTaskDependencyStateMock.mockReturnValue({
      state: "ready_to_unblock",
      blockers: [{ id: "task-2", title: "Design API", status: "done", isComplete: true }],
      blockedTasks: [{ id: "task-3", title: "Ship UI", status: "in_review" }],
    });
    normalizePlanningInputMock.mockReset();
  });

  it("blocks saving when planning input is invalid", async () => {
    const user = userEvent.setup();
    const onTaskSave = jest.fn();
    normalizePlanningInputMock.mockReturnValue({ kind: "invalid" });

    render(
      <TaskDetailContent
        task={makeTask({ id: "task-1", title: "Build dashboard" })}
        tasks={[makeTask({ id: "task-1", title: "Build dashboard" })]}
        members={members}
        agents={agents}
        sprints={sprints}
        onTaskSave={onTaskSave}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Save Changes" }));

    expect(
      screen.getByText("End date cannot be earlier than start date."),
    ).toBeInTheDocument();
    expect(onTaskSave).not.toHaveBeenCalled();
  });

  it("supports assignment, AI decomposition, and saving task edits", async () => {
    const user = userEvent.setup();
    const task = makeTask({ id: "task-1", title: "Build dashboard" });
    const blocker = makeTask({ id: "task-2", title: "Design API", status: "done" });
    const onTaskSave = jest.fn().mockResolvedValue(undefined);
    const onTaskAssign = jest.fn().mockResolvedValue(undefined);
    const onTaskDecompose = jest.fn().mockResolvedValue({
      summary: "Split into two subtasks.",
      subtasks: [
        makeTask({
          id: "task-1-1",
          title: "Implement timeline lane",
          executionMode: "agent",
          status: "inbox",
          description: "Initial scaffold",
        }),
      ],
    });

    normalizePlanningInputMock.mockReturnValue({
      kind: "scheduled",
      plannedStartAt: "2026-03-28T00:00:00.000Z",
      plannedEndAt: "2026-03-29T00:00:00.000Z",
    });

    render(
      <TaskDetailContent
        task={task}
        tasks={[task, blocker]}
        members={members}
        agents={agents}
        sprints={sprints}
        onTaskSave={onTaskSave}
        onTaskAssign={onTaskAssign}
        onTaskDecompose={onTaskDecompose}
      />,
    );

    expect(screen.getByText("Reason: Awaiting review")).toBeInTheDocument();
    expect(screen.getByText("Branch: agent/task-1")).toBeInTheDocument();
    expect(screen.getByTestId("task-review-section")).toHaveTextContent("task-1");

    await user.clear(screen.getByLabelText("Title"));
    await user.type(screen.getByLabelText("Title"), "Build review dashboard");
    await user.click(screen.getByRole("checkbox"));
    await user.click(screen.getByRole("button", { name: "Assign Alice" }));
    expect(onTaskAssign).toHaveBeenCalledWith("task-1", "member-1", "human");

    await user.click(screen.getByRole("button", { name: "AI Decompose Task" }));
    await waitFor(() =>
      expect(onTaskDecompose).toHaveBeenCalledWith("task-1"),
    );
    expect(screen.getByText("Split into two subtasks.")).toBeInTheDocument();
    expect(screen.getByText("Generated subtasks")).toBeInTheDocument();
    expect(screen.getByText("Implement timeline lane")).toBeInTheDocument();
    expect(screen.getByText("Agent-ready")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Save Changes" }));
    await waitFor(() =>
      expect(onTaskSave).toHaveBeenCalledWith("task-1", {
        title: "Build review dashboard",
        description: "Implement the review dashboard.",
        priority: "high",
        sprintId: "sprint-1",
        blockedBy: ["task-2"],
        plannedStartAt: "2026-03-28T00:00:00.000Z",
        plannedEndAt: "2026-03-29T00:00:00.000Z",
      }),
    );
  });
});
