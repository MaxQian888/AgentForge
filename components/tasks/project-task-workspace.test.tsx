import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
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

const fetchDispatchPreflight = jest.fn();
const fetchDispatchHistory = jest.fn();

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

jest.mock("@/lib/stores/agent-store", () => ({
  useAgentStore: (
    selector: (state: {
      agents: Agent[];
      dispatchHistoryByTask: Record<string, unknown[]>;
      fetchDispatchPreflight: typeof fetchDispatchPreflight;
      fetchDispatchHistory: typeof fetchDispatchHistory;
    }) => unknown
  ) =>
    selector({
      agents,
      dispatchHistoryByTask: {},
      fetchDispatchPreflight,
      fetchDispatchHistory,
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
    labels: [],
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
    labels: [],
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
    Object.defineProperty(window, "innerWidth", { writable: true, configurable: true, value: 1400 });
    window.history.replaceState({}, "", "/project?id=project-1");
    fetchDispatchPreflight.mockReset();
    fetchDispatchPreflight.mockResolvedValue(null);
    fetchDispatchHistory.mockReset();
    fetchDispatchHistory.mockResolvedValue(undefined);
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
        projectName="Test Project"
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

    await user.type(screen.getAllByLabelText("Search tasks")[0], "calendar");
    await user.click(screen.getByRole("button", { name: "List" }));

    expect(screen.getAllByText("Calendar polish").length).toBeGreaterThan(0);

    await user.click(screen.getByRole("button", { name: "Open Calendar polish" }));
    expect(onTaskOpen).toHaveBeenCalledWith("task-2");

    await user.click(screen.getByRole("button", { name: "Calendar" }));
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

  it("supports ctrl/cmd multi-select in board view and reveals the bulk action toolbar", async () => {
    const { container } = renderWorkspace();

    const taskCard = container.querySelector('[data-task-id="task-1"]');
    if (!(taskCard instanceof HTMLElement)) {
      throw new Error("Expected task card for task-1");
    }

    fireEvent.click(taskCard, { ctrlKey: true });

    await waitFor(() => {
      expect(useTaskWorkspaceStore.getState().selectedTaskIds).toEqual(["task-1"]);
    });
    expect(screen.getByText("1 selected")).toBeInTheDocument();

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Clear" }));
    expect(screen.queryByText("1 selected")).not.toBeInTheDocument();
  });

  it("keeps unscheduled tasks visible in timeline and reschedules them through the shared callback", async () => {
    const user = userEvent.setup();
    const onTaskScheduleChange = jest.fn().mockResolvedValue(undefined);

    renderWorkspace({ onTaskScheduleChange });

    await user.click(screen.getByRole("button", { name: "Timeline" }));

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

  it("reschedules scheduled timeline tasks while preserving their duration", async () => {
    const user = userEvent.setup();
    const onTaskScheduleChange = jest.fn().mockResolvedValue(undefined);

    renderWorkspace({ onTaskScheduleChange });

    await user.click(screen.getByRole("button", { name: "Timeline" }));

    const onDragEnd = getLastOnDragEnd();

    await act(async () => {
      await onDragEnd?.({
        draggableId: "task-1",
        source: { droppableId: "timeline:2026-03-25", index: 0 },
        destination: { droppableId: "timeline:2026-03-30", index: 0 },
      });
    });

    expect(onTaskScheduleChange).toHaveBeenCalledWith("task-1", {
      plannedStartAt: "2026-03-30T09:00:00.000Z",
      plannedEndAt: "2026-04-01T18:00:00.000Z",
    });
  });

  it("shows inline feedback when a timeline scheduling update fails", async () => {
    const user = userEvent.setup();
    const onTaskScheduleChange = jest
      .fn()
      .mockRejectedValue(new Error("Timeline update failed"));

    renderWorkspace({ onTaskScheduleChange });

    await user.click(screen.getByRole("button", { name: "Timeline" }));

    const onDragEnd = getLastOnDragEnd();

    await act(async () => {
      await onDragEnd?.({
        draggableId: "task-1",
        source: { droppableId: "timeline:2026-03-25", index: 0 },
        destination: { droppableId: "timeline:2026-03-30", index: 0 },
      });
    });

    expect(await screen.findByText("Timeline update failed")).toBeInTheDocument();
    expect(screen.getAllByText("Implement timeline view").length).toBeGreaterThan(0);
  });

  it("supports timeline day, week, and month granularity while keeping scheduled spans visible", async () => {
    const user = userEvent.setup();

    renderWorkspace();

    await user.click(screen.getByRole("button", { name: "Timeline" }));

    const timelineView = screen.getByTestId("timeline-view");
    const dayButton = within(timelineView).getByRole("button", { name: "Day" });
    const weekButton = within(timelineView).getByRole("button", { name: "Week" });
    const monthButton = within(timelineView).getByRole("button", { name: "Month" });

    expect(dayButton).toHaveAttribute("aria-pressed", "true");
    expect(within(timelineView).getByTestId("timeline-bar-task-1")).toHaveAttribute(
      "data-granularity",
      "day"
    );
    expect(within(timelineView).getByTestId("timeline-bar-task-1")).toHaveAttribute(
      "data-span",
      "3"
    );

    await user.click(weekButton);
    expect(weekButton).toHaveAttribute("aria-pressed", "true");
    expect(within(timelineView).getByTestId("timeline-bar-task-1")).toHaveAttribute(
      "data-granularity",
      "week"
    );

    await user.click(monthButton);
    expect(monthButton).toHaveAttribute("aria-pressed", "true");
    expect(within(timelineView).getByTestId("timeline-bar-task-1")).toHaveAttribute(
      "data-granularity",
      "month"
    );
  });

  it("renders timeline dependency connectors and highlights them when a linked task is hovered", async () => {
    const user = userEvent.setup();
    const scheduledTasks = tasks.map((task) =>
      task.id === "task-2"
        ? {
            ...task,
            plannedStartAt: "2026-03-28T09:00:00.000Z",
            plannedEndAt: "2026-03-29T18:00:00.000Z",
          }
        : task
    );

    renderWorkspace({ tasks: scheduledTasks });

    await user.click(screen.getByRole("button", { name: "Timeline" }));

    const connector = screen.getByTestId("timeline-dependency-task-1-task-2");
    expect(connector).toHaveAttribute("data-active", "false");

    await user.hover(screen.getByTestId("timeline-bar-task-1"));
    expect(connector).toHaveAttribute("data-active", "true");

    await user.unhover(screen.getByTestId("timeline-bar-task-1"));
    expect(connector).toHaveAttribute("data-active", "false");
  });

  it("does not render timeline dependency connectors when no visible tasks depend on each other", async () => {
    const user = userEvent.setup();

    renderWorkspace({
      tasks: tasks.map((task) => ({
        ...task,
        blockedBy: [],
      })),
    });

    await user.click(screen.getByRole("button", { name: "Timeline" }));

    expect(screen.queryByTestId("timeline-dependency-task-1-task-2")).not.toBeInTheDocument();
  });

  it("keeps unscheduled tasks visible in calendar view and schedules them through drag", async () => {
    const user = userEvent.setup();
    const onTaskScheduleChange = jest.fn().mockResolvedValue(undefined);

    renderWorkspace({ onTaskScheduleChange });

    await user.click(screen.getByRole("button", { name: "Calendar" }));

    expect(screen.getByText("Unscheduled")).toBeInTheDocument();
    expect(screen.getAllByText("Calendar polish").length).toBeGreaterThan(0);

    const onDragEnd = getLastOnDragEnd();

    await act(async () => {
      await onDragEnd?.({
        draggableId: "task-2",
        source: { droppableId: "calendar:unscheduled", index: 0 },
        destination: { droppableId: "calendar:2026-03-30", index: 0 },
      });
    });

    expect(onTaskScheduleChange).toHaveBeenCalledWith("task-2", {
      plannedStartAt: "2026-03-30T09:00:00.000Z",
      plannedEndAt: "2026-03-30T18:00:00.000Z",
    });
  });

  it("shows inline feedback when a calendar scheduling update fails", async () => {
    const user = userEvent.setup();
    const onTaskScheduleChange = jest
      .fn()
      .mockRejectedValue(new Error("Calendar update failed"));

    renderWorkspace({ onTaskScheduleChange });

    await user.click(screen.getByRole("button", { name: "Calendar" }));

    const onDragEnd = getLastOnDragEnd();

    await act(async () => {
      await onDragEnd?.({
        draggableId: "task-1",
        source: { droppableId: "calendar:2026-03-25", index: 0 },
        destination: { droppableId: "calendar:2026-03-30", index: 0 },
      });
    });

    expect(await screen.findByText("Calendar update failed")).toBeInTheDocument();
    expect(screen.getAllByText("Implement timeline view").length).toBeGreaterThan(0);
  });

  it("supports month and week calendar layouts and renders multi-day task spans", async () => {
    const user = userEvent.setup();

    renderWorkspace();

    await user.click(screen.getByRole("button", { name: "Calendar" }));

    const calendarView = screen.getByTestId("calendar-view");
    const monthButton = within(calendarView).getByRole("button", { name: "Month" });
    const weekButton = within(calendarView).getByRole("button", { name: "Week" });

    expect(monthButton).toHaveAttribute("aria-pressed", "true");
    expect(within(calendarView).getByTestId("calendar-bar-task-1")).toHaveAttribute(
      "data-mode",
      "month"
    );
    expect(within(calendarView).getByTestId("calendar-bar-task-1")).toHaveAttribute(
      "data-span",
      "3"
    );

    await user.click(weekButton);
    expect(weekButton).toHaveAttribute("aria-pressed", "true");
    expect(within(calendarView).getByTestId("calendar-bar-task-1")).toHaveAttribute(
      "data-mode",
      "week"
    );
  });

  it("navigates between calendar months while preserving the visible date grid", async () => {
    const user = userEvent.setup();

    renderWorkspace();

    await user.click(screen.getByRole("button", { name: "Calendar" }));

    const calendarView = screen.getByTestId("calendar-view");
    expect(within(calendarView).getByTestId("calendar-period-label")).toHaveTextContent(
      "2026-03"
    );
    expect(within(calendarView).getByTestId("calendar-bar-task-1")).toBeInTheDocument();

    await user.click(
      within(calendarView).getByRole("button", { name: "Next month" })
    );

    expect(within(calendarView).getByTestId("calendar-period-label")).toHaveTextContent(
      "2026-04"
    );
    expect(
      within(calendarView).queryByTestId("calendar-bar-task-1")
    ).not.toBeInTheDocument();

    await user.click(
      within(calendarView).getByRole("button", { name: "Previous month" })
    );

    expect(within(calendarView).getByTestId("calendar-period-label")).toHaveTextContent(
      "2026-03"
    );
    expect(within(calendarView).getByTestId("calendar-bar-task-1")).toBeInTheDocument();
  });

  it("shows the shared empty state when timeline or calendar have no tasks to render", async () => {
    const user = userEvent.setup();
    useTaskWorkspaceStore.setState({
      viewMode: "timeline",
      filters: createDefaultTaskWorkspaceFilters(),
      selectedTaskId: null,
      contextRailDisplay: "expanded",
      displayOptions: {
        density: "comfortable",
        showDescriptions: true,
        showLinkedDocs: false,
      },
    });

    const { rerender } = renderWorkspace({ tasks: [] });

    expect(screen.getByText("No tasks yet")).toBeInTheDocument();

    await act(async () => {
      useTaskWorkspaceStore.setState((state) => ({ ...state, viewMode: "calendar" }));
    });
    rerender(
      <ProjectTaskWorkspace
        projectId="project-1"
        projectName="Test Project"
        tasks={[]}
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
      />
    );

    await user.click(screen.getByRole("button", { name: "Calendar" }));
    expect(screen.getByText("No tasks yet")).toBeInTheDocument();
  });

  it("renders progress health signals in the list workspace view", async () => {
    const user = userEvent.setup();

    renderWorkspace();

    await user.click(screen.getByRole("button", { name: "List" }));

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

    expect(screen.getByText("Test Project")).toBeInTheDocument();
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

    await user.type(screen.getAllByLabelText("Search tasks")[0], "calendar");

    expect(screen.getByText(/outside the current filters/i)).toBeInTheDocument();
    expect(screen.getByDisplayValue("Implement timeline view")).toBeInTheDocument();
  });

  it("lets shared display options hide descriptions without clearing the current view state", async () => {
    const user = userEvent.setup();

    renderWorkspace();

    await user.click(screen.getByRole("button", { name: "List" }));
    expect(screen.getByText("Build the horizontal planning lane.")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Hide descriptions" }));
    expect(
      screen.queryByText("Build the horizontal planning lane.")
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Board" }));
    await user.click(screen.getByRole("button", { name: "List" }));

    expect(
      screen.queryByText("Build the horizontal planning lane.")
    ).not.toBeInTheDocument();
  });

  it("lets the user type a search filter and clear it to restore all tasks", async () => {
    const user = userEvent.setup();

    renderWorkspace();

    await user.type(screen.getAllByLabelText("Search tasks")[0], "calendar");

    expect(screen.getAllByText("Calendar polish").length).toBeGreaterThan(0);

    await user.click(screen.getByRole("button", { name: "Reset filters" }));

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

    await user.click(screen.getByRole("button", { name: "List" }));
    expect(
      container.querySelector('tr[data-task-id="task-1"][data-selected="true"]')
    ).not.toBeNull();

    await user.click(screen.getByRole("button", { name: "Timeline" }));
    expect(
      container.querySelector('button[data-task-id="task-1"][data-selected="true"]')
    ).not.toBeNull();
  });

  it("shows degraded realtime state in the context rail when websocket is disconnected", () => {
    renderWorkspace({ realtimeConnected: false });

    expect(screen.getByText(/realtime updates unavailable/i)).toBeInTheDocument();
  });

  it("surfaces a degraded realtime indicator in the workspace sidebar", () => {
    renderWorkspace({ realtimeConnected: false });

    expect(screen.getAllByText("Realtime degraded").length).toBeGreaterThan(0);
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
    await user.click(
      await screen.findByRole("button", { name: "Assign and dispatch" }),
    );

    await waitFor(() => {
      expect(onTaskAssign).toHaveBeenCalledWith("task-2", "member-2", "agent");
    });
  });

  it("filters the workspace down to blocked tasks with the shared dependency filter", async () => {
    const user = userEvent.setup();

    renderWorkspace();

    await user.click(screen.getByRole("button", { name: "List" }));
    await user.selectOptions(screen.getAllByLabelText("Dependencies")[0], "blocked");

    expect(screen.getAllByText("Calendar polish").length).toBeGreaterThan(0);
  });

  it("lets the user collapse and expand the context rail without losing workspace state", async () => {
    const user = userEvent.setup();

    renderWorkspace();

    await user.click(screen.getByTitle("Hide Details"));

    expect(
      useTaskWorkspaceStore.getState().contextRailDisplay
    ).toBe("collapsed");
    expect(screen.queryByText("Task details")).not.toBeInTheDocument();

    await user.click(screen.getByTitle("Show Details"));

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

    await user.click(screen.getByRole("button", { name: "List" }));
    await user.selectOptions(screen.getAllByLabelText("Sprint")[0], "sprint-2");

    expect(screen.getAllByText("Calendar polish").length).toBeGreaterThan(0);
  });
});
