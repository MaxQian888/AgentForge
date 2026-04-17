const recommendTaskAssigneesMock = jest.fn();
const getTaskDependencyStateMock = jest.fn();
const normalizePlanningInputMock = jest.fn();
type DocsStoreMockState = {
  tree: never[];
  fetchTree: jest.Mock;
};
type EntityLinkStoreMockState = {
  linksByEntity: Record<string, unknown>;
  fetchLinks: jest.Mock;
  createLink: jest.Mock;
  deleteLink: jest.Mock;
};
type TaskCommentStoreMockState = {
  commentsByTask: Record<string, unknown>;
  fetchComments: jest.Mock;
  createComment: jest.Mock;
  setResolved: jest.Mock;
};
type CustomFieldStoreMockState = {
  definitionsByProject: Record<string, unknown>;
  valuesByTask: Record<string, unknown>;
  fetchDefinitions: jest.Mock;
  fetchTaskValues: jest.Mock;
};
type MilestoneStoreMockState = {
  milestonesByProject: Record<string, unknown>;
  fetchMilestones: jest.Mock;
};
type AgentStoreMockState = {
  fetchDispatchPreflight: jest.Mock;
  fetchDispatchHistory: jest.Mock;
  dispatchHistoryByTask: Record<string, unknown>;
};

const fetchDispatchPreflightMock = jest.fn();
const fetchDispatchHistoryMock = jest.fn();

jest.mock("@/components/review/task-review-section", () => ({
  TaskReviewSection: ({ taskId }: { taskId: string }) => (
    <div data-testid="task-review-section">{taskId}</div>
  ),
}));

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    if (!values) {
      return key;
    }
    return Object.entries(values).reduce(
      (message, [token, value]) => message.replace(`{${token}}`, String(value)),
      key,
    );
  },
}));

jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({ accessToken: "test-token" }),
  },
}));

jest.mock("@/lib/stores/agent-store", () => ({
  useAgentStore: (selector: (state: AgentStoreMockState) => unknown) =>
    selector({
      fetchDispatchPreflight: fetchDispatchPreflightMock,
      fetchDispatchHistory: fetchDispatchHistoryMock,
      dispatchHistoryByTask: {
        "task-1": [
          {
            id: "attempt-1",
            projectId: "project-1",
            taskId: "task-1",
            memberId: "member-agent",
            outcome: "queued",
            triggerSource: "manual",
            reason: "agent pool is at capacity",
            createdAt: "2026-03-28T10:00:00.000Z",
          },
        ],
      },
    }),
}));

jest.mock("@/lib/stores/knowledge-store", () => ({
  useKnowledgeStore: (selector: (state: DocsStoreMockState) => unknown) =>
    selector({
      tree: [],
      fetchTree: jest.fn(),
    }),
  flattenKnowledgeTree: () => [],
}));

jest.mock("@/lib/stores/entity-link-store", () => ({
  useEntityLinkStore: (selector: (state: EntityLinkStoreMockState) => unknown) =>
    selector({
      linksByEntity: {},
      fetchLinks: jest.fn(),
      createLink: jest.fn(),
      deleteLink: jest.fn(),
    }),
}));

jest.mock("@/lib/stores/task-comment-store", () => ({
  useTaskCommentStore: (selector: (state: TaskCommentStoreMockState) => unknown) =>
    selector({
      commentsByTask: {},
      fetchComments: jest.fn(),
      createComment: jest.fn(),
      setResolved: jest.fn(),
    }),
}));

jest.mock("@/lib/stores/custom-field-store", () => ({
  useCustomFieldStore: (selector: (state: CustomFieldStoreMockState) => unknown) =>
    selector({
      definitionsByProject: {},
      valuesByTask: {},
      fetchDefinitions: jest.fn(),
      fetchTaskValues: jest.fn(),
    }),
}));

jest.mock("@/lib/stores/milestone-store", () => ({
  useMilestoneStore: (selector: (state: MilestoneStoreMockState) => unknown) =>
    selector({
      milestonesByProject: {},
      fetchMilestones: jest.fn(),
    }),
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
  {
    id: "member-agent",
    projectId: "project-1",
    name: "Agent Smith",
    type: "agent",
    typeLabel: "Agent",
    role: "Automation agent",
    email: "agent@example.com",
    avatarUrl: "",
    skills: ["frontend", "testing"],
    isActive: true,
    status: "active",
    createdAt: "2026-03-25T08:00:00.000Z",
    lastActivityAt: null,
    workload: {
      assignedTasks: 1,
      inProgressTasks: 1,
      inReviewTasks: 0,
      activeAgentRuns: 1,
    },
  },
  {
    id: "member-suspended",
    projectId: "project-1",
    name: "Suspended Sam",
    type: "human",
    typeLabel: "Human",
    role: "Frontend backup",
    statusLabel: "Suspended",
    email: "sam@example.com",
    avatarUrl: "",
    skills: ["frontend", "testing"],
    isActive: true,
    status: "suspended",
    createdAt: "2026-03-25T08:00:00.000Z",
    lastActivityAt: null,
    workload: {
      assignedTasks: 0,
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
    projectId: "project-1",
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
    labels: [],
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
    id: overrides.id,
    title: overrides.title,
  };
}

describe("TaskDetailContent", () => {
  beforeEach(() => {
    fetchDispatchPreflightMock.mockReset();
    fetchDispatchPreflightMock.mockResolvedValue({
      admissionLikely: true,
      dispatchOutcomeHint: "started",
      poolActive: 1,
      poolAvailable: 1,
      poolQueued: 0,
    });
    fetchDispatchHistoryMock.mockReset();
    fetchDispatchHistoryMock.mockResolvedValue([]);
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

    await user.click(screen.getByRole("button", { name: "detail.saveChanges" }));

    expect(
      screen.getByText("detail.endBeforeStart"),
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

    expect(screen.getByText("detail.reason")).toBeInTheDocument();
    expect(screen.getByText("detail.branch")).toBeInTheDocument();
    expect(screen.getByText("detail.dispatchHistoryTitle")).toBeInTheDocument();
    expect(fetchDispatchHistoryMock).toHaveBeenCalledWith("task-1");
    expect(screen.getByTestId("task-review-section")).toHaveTextContent("task-1");
    expect(screen.getByTestId("linked-docs-panel")).toBeInTheDocument();
    expect(screen.getByTestId("task-comments")).toBeInTheDocument();

    await user.clear(screen.getByLabelText("detail.titleLabel"));
    await user.type(screen.getByLabelText("detail.titleLabel"), "Build review dashboard");
    await user.click(screen.getByRole("checkbox"));
    await user.click(screen.getByRole("button", { name: "detail.assignMember" }));
    expect(onTaskAssign).toHaveBeenCalledWith("task-1", "member-1", "human");

    await user.click(screen.getByRole("button", { name: "detail.aiDecomposeTask" }));
    await waitFor(() =>
      expect(onTaskDecompose).toHaveBeenCalledWith("task-1"),
    );
    expect(screen.getByText("Split into two subtasks.")).toBeInTheDocument();
    expect(screen.getByText("detail.generatedSubtasks")).toBeInTheDocument();
    expect(screen.getByText("Implement timeline lane")).toBeInTheDocument();
    expect(screen.getByText("execution.agentReady")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "detail.saveChanges" }));
    await waitFor(() =>
      expect(onTaskSave).toHaveBeenCalledWith("task-1", {
        title: "Build review dashboard",
        description: "Implement the review dashboard.",
        priority: "high",
        sprintId: "sprint-1",
        milestoneId: null,
        blockedBy: ["task-2"],
        labels: [],
        plannedStartAt: "2026-03-28T00:00:00.000Z",
        plannedEndAt: "2026-03-29T00:00:00.000Z",
      }),
    );
  });

  it("shows dispatch preflight before assigning an agent member", async () => {
    const user = userEvent.setup();
    const task = makeTask({ id: "task-1", title: "Build dashboard" });
    const onTaskAssign = jest.fn().mockResolvedValue(undefined);
    normalizePlanningInputMock.mockReturnValue({
      kind: "scheduled",
      plannedStartAt: "2026-03-28T00:00:00.000Z",
      plannedEndAt: "2026-03-29T00:00:00.000Z",
    });
    recommendTaskAssigneesMock.mockReturnValue([
      { member: members[1], reasons: ["agent ready", "frontend fit"] },
    ]);

    render(
      <TaskDetailContent
        task={task}
        tasks={[task]}
        members={members}
        agents={agents}
        sprints={sprints}
        onTaskAssign={onTaskAssign}
      />,
    );

    await user.click(screen.getByRole("button", { name: "detail.assignMember" }));

    expect(fetchDispatchPreflightMock).toHaveBeenCalledWith("project-1", "task-1", "member-agent");
    expect(screen.getByText("detail.dispatchPreflightTitle")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "detail.dispatchPreflightConfirm" }));

    await waitFor(() =>
      expect(onTaskAssign).toHaveBeenCalledWith("task-1", "member-agent", "agent"),
    );
  });

  it("does not expose suspended members in the manual assignment list", () => {
    const task = makeTask({ id: "task-1", title: "Build dashboard" });

    render(
      <TaskDetailContent
        task={task}
        tasks={[task]}
        members={members}
        agents={agents}
        sprints={sprints}
        onTaskAssign={jest.fn()}
      />
    );

    expect(screen.getByRole("option", { name: "Alice (Human)" })).toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "Suspended Sam (Human)" })
    ).not.toBeInTheDocument();
  });
});
jest.mock("./linked-docs-panel", () => ({
  LinkedDocsPanel: () => <div data-testid="linked-docs-panel">linked-docs-panel</div>,
}));

jest.mock("./task-comments", () => ({
  TaskComments: () => <div data-testid="task-comments">task-comments</div>,
}));
