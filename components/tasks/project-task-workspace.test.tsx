import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectTaskWorkspace } from "./project-task-workspace";
import type { TeamMember } from "@/lib/dashboard/summary";
import type { Agent } from "@/lib/stores/agent-store";
import type { EntityLink } from "@/lib/stores/entity-link-store";
import type { Notification } from "@/lib/stores/notification-store";
import type { Task } from "@/lib/stores/task-store";
import type { Sprint, SprintMetrics } from "@/lib/stores/sprint-store";
import {
  createDefaultTaskWorkspaceFilters,
  useTaskWorkspaceStore,
} from "@/lib/stores/task-workspace-store";

jest.mock("@hello-pangea/dnd", () => {
  let lastOnDragEnd: ((result: unknown) => void | Promise<void>) | null = null;

  return {
    DragDropContext: ({
      children,
      onDragEnd,
    }: {
      children: React.ReactNode;
      onDragEnd: (result: unknown) => void | Promise<void>;
    }) => {
      lastOnDragEnd = onDragEnd;
      return <div data-testid="drag-context">{children}</div>;
    },
    Droppable: ({
      children,
      droppableId,
    }: {
      children: (provided: {
        innerRef: () => void;
        droppableProps: Record<string, unknown>;
        placeholder: null;
      }, snapshot: { isDraggingOver: boolean }) => React.ReactNode;
      droppableId: string;
    }) =>
      children(
        {
          innerRef: () => undefined,
          droppableProps: { "data-droppable-id": droppableId },
          placeholder: null,
        },
        { isDraggingOver: false }
      ),
    Draggable: ({
      children,
      draggableId,
    }: {
      children: (provided: {
        innerRef: () => void;
        draggableProps: Record<string, unknown>;
        dragHandleProps: Record<string, unknown>;
      }, snapshot: { isDragging: boolean }) => React.ReactNode;
      draggableId: string;
    }) =>
      children(
        {
          innerRef: () => undefined,
          draggableProps: { "data-draggable-id": draggableId },
          dragHandleProps: {},
        },
        { isDragging: false }
      ),
    __getLastOnDragEnd: () => lastOnDragEnd,
  };
});

jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({ accessToken: "test-token" }),
  },
}));

jest.mock("@/lib/stores/docs-store", () => ({
  useDocsStore: (
    selector: (state: { tree: []; fetchTree: jest.Mock }) => unknown
  ) =>
    selector({
      tree: [],
      fetchTree: jest.fn(),
    }),
  flattenDocsTree: () => [],
}));

type EntityLinkStoreSlice = {
  linksByEntity: Record<string, EntityLink[]>;
  fetchLinks: jest.Mock;
  createLink: jest.Mock;
  deleteLink: jest.Mock;
};

jest.mock("@/lib/stores/entity-link-store", () => ({
  useEntityLinkStore: (selector: (state: EntityLinkStoreSlice) => unknown) =>
    selector({
      linksByEntity: {},
      fetchLinks: jest.fn(),
      createLink: jest.fn(),
      deleteLink: jest.fn(),
    }),
}));

jest.mock("@/lib/stores/task-comment-store", () => ({
  useTaskCommentStore: (
    selector: (state: {
      commentsByTask: Record<string, unknown[]>;
      fetchComments: jest.Mock;
      createComment: jest.Mock;
      setResolved: jest.Mock;
    }) => unknown
  ) =>
    selector({
      commentsByTask: {},
      fetchComments: jest.fn(),
      createComment: jest.fn(),
      setResolved: jest.fn(),
    }),
}));

jest.mock("@/lib/stores/custom-field-store", () => ({
  useCustomFieldStore: (
    selector: (state: {
      definitionsByProject: Record<string, unknown[]>;
      valuesByTask: Record<string, unknown[]>;
      fetchDefinitions: jest.Mock;
      fetchTaskValues: jest.Mock;
    }) => unknown
  ) =>
    selector({
      definitionsByProject: {},
      valuesByTask: {},
      fetchDefinitions: jest.fn(),
      fetchTaskValues: jest.fn(),
    }),
}));

jest.mock("@/lib/stores/saved-view-store", () => ({
  useSavedViewStore: (
    selector: (state: {
      viewsByProject: Record<string, unknown[]>;
      currentViewByProject: Record<string, string | null>;
      fetchViews: jest.Mock;
      selectView: jest.Mock;
      setDefaultView: jest.Mock;
    }) => unknown
  ) =>
    selector({
      viewsByProject: {},
      currentViewByProject: {},
      fetchViews: jest.fn(),
      selectView: jest.fn(),
      setDefaultView: jest.fn(),
    }),
}));

jest.mock("@/lib/stores/milestone-store", () => ({
  useMilestoneStore: (
    selector: (state: {
      milestonesByProject: Record<string, unknown[]>;
      fetchMilestones: jest.Mock;
    }) => unknown
  ) =>
    selector({
      milestonesByProject: {},
      fetchMilestones: jest.fn(),
    }),
}));

type DndMock = {
  __getLastOnDragEnd: () => ((result: unknown) => void | Promise<void>) | null;
};

const tasks: Task[] = [
  {
    id: "task-1",
    projectId: "project-1",
    sprintId: "sprint-1",
    title: "Implement timeline view",
    description: "Build the horizontal planning lane.",
    status: "in_progress",
    priority: "high",
    assigneeId: "member-1",
    assigneeType: "human",
    assigneeName: "Alice",
    cost: 2.5,
    budgetUsd: 6,
    spentUsd: 2.5,
    agentBranch: "",
    agentWorktree: "",
    agentSessionId: "",
    blockedBy: [],
    plannedStartAt: "2026-03-25T09:00:00.000Z",
    plannedEndAt: "2026-03-27T18:00:00.000Z",
    progress: {
      lastActivityAt: "2026-03-24T09:15:00.000Z",
      lastActivitySource: "detector",
      lastTransitionAt: "2026-03-24T09:10:00.000Z",
      healthStatus: "stalled",
      riskReason: "no_recent_update",
      riskSinceAt: "2026-03-24T09:20:00.000Z",
      lastAlertState: "stalled:no_recent_update",
      lastAlertAt: "2026-03-24T09:25:00.000Z",
      lastRecoveredAt: null,
    },
    createdAt: "2026-03-24T09:00:00.000Z",
    updatedAt: "2026-03-24T09:30:00.000Z",
  },
  {
    id: "task-2",
    projectId: "project-1",
    sprintId: "sprint-2",
    title: "Calendar polish",
    description: "Keep unscheduled tasks visible.",
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
    blockedBy: ["task-1"],
    plannedStartAt: null,
    plannedEndAt: null,
    progress: {
      lastActivityAt: "2026-03-24T10:10:00.000Z",
      lastActivitySource: "task_created",
      lastTransitionAt: "2026-03-24T10:00:00.000Z",
      healthStatus: "warning",
      riskReason: "no_assignee",
      riskSinceAt: "2026-03-24T10:05:00.000Z",
      lastAlertState: "warning:no_assignee",
      lastAlertAt: "2026-03-24T10:05:00.000Z",
      lastRecoveredAt: null,
    },
    createdAt: "2026-03-24T10:00:00.000Z",
    updatedAt: "2026-03-24T10:15:00.000Z",
  },
];

const notifications: Notification[] = [
  {
    id: "alert-1",
    type: "task_progress_stalled",
    title: "Task stalled: Implement timeline view",
    message: "Implement timeline view is stalled.",
    href: "/project?id=project-1#task-task-1",
    read: false,
    createdAt: "2026-03-24T12:00:00.000Z",
  },
];

const members: TeamMember[] = [
  {
    id: "member-1",
    projectId: "project-1",
    name: "Alice",
    type: "human",
    typeLabel: "Human",
    role: "Frontend engineer",
    email: "",
    avatarUrl: "",
    skills: ["frontend", "timeline"],
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
    id: "member-2",
    projectId: "project-1",
    name: "Calendar Bot",
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
];

const agents: Agent[] = [];
const sprints: Sprint[] = [
  {
    id: "sprint-1",
    projectId: "project-1",
    name: "Sprint Alpha",
    startDate: "2026-03-24T00:00:00.000Z",
    endDate: "2026-03-30T23:59:59.000Z",
    status: "active",
    totalBudgetUsd: 20,
    spentUsd: 8,
    createdAt: "2026-03-20T10:00:00.000Z",
  },
  {
    id: "sprint-2",
    projectId: "project-1",
    name: "Sprint Beta",
    startDate: "2026-03-31T00:00:00.000Z",
    endDate: "2026-04-06T23:59:59.000Z",
    status: "planning",
    totalBudgetUsd: 12,
    spentUsd: 0,
    createdAt: "2026-03-21T10:00:00.000Z",
  },
];
const sprintMetrics: SprintMetrics = {
  sprint: sprints[0],
  plannedTasks: 3,
  completedTasks: 2,
  remainingTasks: 1,
  completionRate: 66.67,
  velocityPerWeek: 2,
  taskBudgetUsd: 16,
  taskSpentUsd: 9.5,
  burndown: [
    { date: "2026-03-24", remainingTasks: 3, completedTasks: 0 },
    { date: "2026-03-25", remainingTasks: 2, completedTasks: 1 },
    { date: "2026-03-26", remainingTasks: 1, completedTasks: 2 },
  ],
};

describe("ProjectTaskWorkspace", () => {
  beforeEach(() => {
    window.history.replaceState({}, "", "/project?id=project-1");
    useTaskWorkspaceStore.setState({
      viewMode: "board",
      filters: createDefaultTaskWorkspaceFilters(),
      selectedTaskId: null,
      contextRailDisplay: "expanded",
      displayOptions: {
        density: "comfortable",
        showDescriptions: true,
        showLinkedDocs: false,
      },
    });
  });

  function renderWorkspace(
    overrides: Partial<React.ComponentProps<typeof ProjectTaskWorkspace>> = {}
  ) {
    return render(
      <ProjectTaskWorkspace
        projectId="project-1"
        tasks={tasks}
        loading={false}
        error={null}
        realtimeConnected
        notifications={notifications}
        members={members}
        agents={agents}
        sprints={sprints}
        sprintMetrics={sprintMetrics}
        sprintMetricsLoading={false}
        onRetry={jest.fn()}
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
        onTaskSave={jest.fn()}
        onTaskAssign={jest.fn()}
        onSprintFilterChange={jest.fn()}
        {...overrides}
      />
    );
  }

  function getLastOnDragEnd() {
    return (jest.requireMock("@hello-pangea/dnd") as DndMock).__getLastOnDragEnd();
  }

  it("keeps filters across views and routes task selection through one workspace", async () => {
    const user = userEvent.setup();
    const onTaskOpen = jest.fn();

    renderWorkspace({ onTaskOpen });

    await user.type(screen.getByLabelText("Search tasks"), "calendar");
    await user.click(screen.getByRole("tab", { name: "List" }));

    expect(screen.getAllByText("Calendar polish").length).toBeGreaterThan(0);

    await user.click(screen.getByRole("button", { name: "Open Calendar polish" }));
    expect(onTaskOpen).toHaveBeenCalledWith("task-2");

    await user.click(screen.getByRole("tab", { name: "Calendar" }));
    expect(screen.getAllByText("Calendar polish").length).toBeGreaterThan(0);
    expect(
      screen.queryByRole("button", { name: "Open Implement timeline view" })
    ).not.toBeInTheDocument();
  });

  it("shows an explicit empty state when the project has no tasks", () => {
    renderWorkspace({ tasks: [] });

    expect(screen.getByText("No tasks yet")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Create the first task to start using Board, List, Timeline, and Calendar views."
      )
    ).toBeInTheDocument();
  });

  it("persists board drag updates through the shared status callback", async () => {
    const onTaskStatusChange = jest.fn().mockResolvedValue(undefined);

    renderWorkspace({ onTaskStatusChange });

    const onDragEnd = getLastOnDragEnd();

    await act(async () => {
      await onDragEnd?.({
        draggableId: "task-1",
        source: { droppableId: "in_progress", index: 0 },
        destination: { droppableId: "done", index: 0 },
      });
    });

    expect(onTaskStatusChange).toHaveBeenCalledWith("task-1", "done");
  });

  it("keeps unscheduled tasks visible in timeline and reschedules them through the shared callback", async () => {
    const user = userEvent.setup();
    const onTaskScheduleChange = jest.fn().mockResolvedValue(undefined);

    renderWorkspace({ onTaskScheduleChange });

    await user.click(screen.getByRole("tab", { name: "Timeline" }));

    expect(screen.getByText("Unscheduled")).toBeInTheDocument();
    expect(screen.getAllByText("Calendar polish").length).toBeGreaterThan(0);

    const onDragEnd = getLastOnDragEnd();

    await act(async () => {
      await onDragEnd?.({
        draggableId: "task-2",
        source: { droppableId: "timeline:unscheduled", index: 0 },
        destination: { droppableId: "timeline:2026-03-30", index: 0 },
      });
    });

    expect(onTaskScheduleChange).toHaveBeenCalledWith("task-2", {
      plannedStartAt: "2026-03-30T09:00:00.000Z",
      plannedEndAt: "2026-03-30T18:00:00.000Z",
    });
  });

  it("renders progress health signals in the list workspace view", async () => {
    const user = userEvent.setup();

    renderWorkspace();

    await user.click(screen.getByRole("tab", { name: "List" }));

    expect(screen.getAllByText("Stalled").length).toBeGreaterThan(0);
    expect(screen.getAllByText("No recent update").length).toBeGreaterThan(0);
    expect(screen.getAllByText("At risk").length).toBeGreaterThan(0);
    expect(screen.getAllByText("No assignee").length).toBeGreaterThan(0);
  });

  it("shows a retryable load error and keeps the workspace shell mounted", () => {
    renderWorkspace({
      tasks: [],
      error: "Unable to load tasks",
    });

    expect(screen.getByText("Task Workspace")).toBeInTheDocument();
    expect(screen.getAllByText("Unable to load tasks")).not.toHaveLength(0);
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("keeps the selected task in the context rail when current filters hide it", async () => {
    const user = userEvent.setup();

    useTaskWorkspaceStore.setState({
      viewMode: "board",
      filters: createDefaultTaskWorkspaceFilters(),
      selectedTaskId: "task-1",
    });

    renderWorkspace();

    await user.type(screen.getByLabelText("Search tasks"), "calendar");

    expect(screen.getByText(/outside the current filters/i)).toBeInTheDocument();
    expect(screen.getByDisplayValue("Implement timeline view")).toBeInTheDocument();
  });

  it("lets shared display options hide descriptions without clearing the current view state", async () => {
    const user = userEvent.setup();

    renderWorkspace();

    await user.click(screen.getByRole("tab", { name: "List" }));
    expect(screen.getByText("Build the horizontal planning lane.")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Hide descriptions" }));
    expect(
      screen.queryByText("Build the horizontal planning lane.")
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Board" }));
    await user.click(screen.getByRole("tab", { name: "List" }));

    expect(
      screen.queryByText("Build the horizontal planning lane.")
    ).not.toBeInTheDocument();
  });

  it("shows active filter chips and lets the user clear one filter without resetting the others", async () => {
    const user = userEvent.setup();

    renderWorkspace();

    await user.type(screen.getByLabelText("Search tasks"), "calendar");

    expect(
      screen.getByRole("button", { name: 'Clear filter search "calendar"' })
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: 'Clear filter search "calendar"' })
    );

    expect(
      screen.queryByRole("button", { name: 'Clear filter search "calendar"' })
    ).not.toBeInTheDocument();
    expect(screen.getAllByText("Implement timeline view").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Calendar polish").length).toBeGreaterThan(0);
  });

  it("marks the selected task consistently across board, list, and timeline views", async () => {
    const user = userEvent.setup();
    useTaskWorkspaceStore.setState({
      viewMode: "board",
      filters: createDefaultTaskWorkspaceFilters(),
      selectedTaskId: "task-1",
      contextRailDisplay: "expanded",
      displayOptions: {
        density: "comfortable",
        showDescriptions: true,
        showLinkedDocs: false,
      },
    });

    const { container } = renderWorkspace();

    expect(
      container.querySelector('[data-task-id="task-1"][data-selected="true"]')
    ).not.toBeNull();

    await user.click(screen.getByRole("tab", { name: "List" }));
    expect(
      container.querySelector('tr[data-task-id="task-1"][data-selected="true"]')
    ).not.toBeNull();

    await user.click(screen.getByRole("tab", { name: "Timeline" }));
    expect(
      container.querySelector('button[data-task-id="task-1"][data-selected="true"]')
    ).not.toBeNull();
  });

  it("shows degraded realtime state in the context rail when websocket is disconnected", () => {
    renderWorkspace({ realtimeConnected: false });

    expect(screen.getByText(/realtime updates unavailable/i)).toBeInTheDocument();
  });

  it("surfaces a degraded realtime indicator in the workspace header", () => {
    renderWorkspace({ realtimeConnected: false });

    expect(screen.getByText("Live alerts paused")).toBeInTheDocument();
  });

  it("shows smart assignment recommendations for the selected task and forwards one-click assignment", async () => {
    const user = userEvent.setup();
    const onTaskAssign = jest.fn().mockResolvedValue(undefined);

    useTaskWorkspaceStore.setState({
      viewMode: "board",
      filters: createDefaultTaskWorkspaceFilters(),
      selectedTaskId: "task-2",
      contextRailDisplay: "expanded",
      displayOptions: {
        density: "comfortable",
        showDescriptions: true,
        showLinkedDocs: false,
      },
    });

    renderWorkspace({ onTaskAssign });

    expect(screen.getByText("Smart assignment")).toBeInTheDocument();
    expect(screen.getByText("Calendar Bot")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Assign Calendar Bot" }));

    await waitFor(() => {
      expect(onTaskAssign).toHaveBeenCalledWith("task-2", "member-2", "agent");
    });
  });

  it("filters the workspace down to blocked tasks with the shared dependency filter", async () => {
    const user = userEvent.setup();

    renderWorkspace();

    await user.click(screen.getByRole("tab", { name: "List" }));
    await user.selectOptions(screen.getByLabelText("Dependencies"), "blocked");

    expect(screen.getAllByText("Calendar polish").length).toBeGreaterThan(0);
  });

  it("lets the user collapse and expand the context rail without losing workspace state", async () => {
    const user = userEvent.setup();

    renderWorkspace();

    await user.click(screen.getByRole("button", { name: "Collapse context rail" }));

    expect(
      useTaskWorkspaceStore.getState().contextRailDisplay
    ).toBe("collapsed");
    expect(
      screen.getByRole("button", { name: "Expand context rail" })
    ).toBeInTheDocument();
    expect(screen.getAllByText("Realtime live").length).toBeGreaterThan(0);
    expect(screen.getByText("Stalled 1")).toBeInTheDocument();
    expect(screen.queryByText("Task details")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Expand context rail" }));

    expect(
      useTaskWorkspaceStore.getState().contextRailDisplay
    ).toBe("expanded");
    expect(screen.getByText("Task details")).toBeInTheDocument();
  });

  it("clears a stale selected task when it no longer exists in the active task source", async () => {
    useTaskWorkspaceStore.setState({
      viewMode: "board",
      filters: createDefaultTaskWorkspaceFilters(),
      selectedTaskId: "task-missing",
      contextRailDisplay: "expanded",
      displayOptions: {
        density: "comfortable",
        showDescriptions: true,
        showLinkedDocs: false,
      },
    });

    renderWorkspace();

    await waitFor(() => {
      expect(useTaskWorkspaceStore.getState().selectedTaskId).toBeNull();
    });
    expect(screen.getByText("Select a task to inspect its details.")).toBeInTheDocument();
  });

  it("renders sprint metrics and lets the shared sprint filter scope visible tasks", async () => {
    const user = userEvent.setup();

    renderWorkspace();

    expect(screen.getAllByText("Sprint Alpha").length).toBeGreaterThan(0);
    expect(screen.getByText("Velocity 2.00/wk")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "List" }));
    await user.selectOptions(screen.getByLabelText("Sprint"), "sprint-2");

    expect(screen.getAllByText("Calendar polish").length).toBeGreaterThan(0);
  });
});
