jest.mock("@/components/kanban/board", () => ({
  Board: ({ tasks }: { tasks: Array<{ id: string }> }) => (
    <div data-testid="board-view">{tasks.length} board tasks</div>
  ),
}));

jest.mock("@/components/tasks/task-dependency-graph", () => ({
  TaskDependencyGraph: ({ tasks }: { tasks: Array<{ id: string }> }) => (
    <div data-testid="dependency-view">{tasks.length} dependency tasks</div>
  ),
}));

jest.mock("@/components/sprint/burndown-chart", () => ({
  BurndownChart: ({
    plannedTasks,
  }: {
    plannedTasks: number;
  }) => <div data-testid="burndown-chart">{plannedTasks} planned</div>,
}));

import userEvent from "@testing-library/user-event";
import { render, screen } from "@testing-library/react";
import { TaskWorkspaceMain } from "./task-workspace-main";
import {
  createDefaultTaskWorkspaceFilters,
  useTaskWorkspaceStore,
} from "@/lib/stores/task-workspace-store";
import type { Sprint, SprintMetrics } from "@/lib/stores/sprint-store";
import type { Task } from "@/lib/stores/task-store";

function makeTask(
  overrides: Partial<Task> & { id: string; title: string; status: Task["status"] },
): Task {
  return {
    id: overrides.id,
    projectId: "project-1",
    title: overrides.title,
    description: `${overrides.title} description`,
    status: overrides.status,
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
    blockedBy: [],
    plannedStartAt: null,
    plannedEndAt: null,
    progress: null,
    createdAt: "2026-03-25T08:00:00.000Z",
    updatedAt: "2026-03-25T08:00:00.000Z",
  };
}

const sprints: Sprint[] = [
  {
    id: "sprint-1",
    projectId: "project-1",
    name: "Sprint 1",
    startDate: "2026-03-25T00:00:00.000Z",
    endDate: "2026-03-31T00:00:00.000Z",
    status: "active",
    totalBudgetUsd: 20,
    spentUsd: 8,
    createdAt: "2026-03-24T00:00:00.000Z",
  },
];

const sprintMetrics: SprintMetrics = {
  sprint: sprints[0],
  plannedTasks: 5,
  completedTasks: 2,
  remainingTasks: 3,
  completionRate: 40,
  velocityPerWeek: 3.5,
  taskBudgetUsd: 20,
  taskSpentUsd: 8,
  burndown: [],
};

describe("TaskWorkspaceMain", () => {
  beforeEach(() => {
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

  it("filters board tasks, resets filters, and switches to the dependency view", async () => {
    const user = userEvent.setup();
    const tasks = [
      makeTask({ id: "task-1", title: "Build dashboard", status: "inbox" }),
      makeTask({ id: "task-2", title: "Review queues", status: "done", assigneeId: "member-1", assigneeName: "Alice" }),
    ];

    render(
      <TaskWorkspaceMain
        projectId="project-1"
        tasks={tasks}
        sprints={sprints}
        sprintMetrics={sprintMetrics}
        sprintMetricsLoading={false}
        loading={false}
        error={null}
        realtimeConnected={false}
        onRetry={jest.fn()}
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
      />,
    );

    expect(screen.getByText("Live alerts paused")).toBeInTheDocument();
    expect(screen.getByTestId("burndown-chart")).toHaveTextContent("5 planned");
    expect(screen.getByTestId("board-view")).toHaveTextContent("2 board tasks");

    await user.type(screen.getByLabelText("Search tasks"), "Review");
    expect(screen.getByText('search: Review')).toBeInTheDocument();
    expect(screen.getByTestId("board-view")).toHaveTextContent("1 board tasks");

    await user.click(screen.getByRole("button", { name: "Reset filters" }));
    expect(screen.queryByText('search: Review')).not.toBeInTheDocument();
    expect(screen.getByTestId("board-view")).toHaveTextContent("2 board tasks");

    await user.click(screen.getByRole("tab", { name: "Dependencies" }));
    expect(screen.getByTestId("dependency-view")).toHaveTextContent("2 dependency tasks");
  });

  it("renders the list view and forwards task-open interactions", async () => {
    const user = userEvent.setup();
    const onTaskOpen = jest.fn();
    const tasks = [
      makeTask({ id: "task-1", title: "Build dashboard", status: "in_progress" }),
    ];

    useTaskWorkspaceStore.setState((state) => ({
      ...state,
      viewMode: "list",
    }));

    render(
      <TaskWorkspaceMain
        projectId="project-1"
        tasks={tasks}
        sprints={[]}
        sprintMetrics={null}
        sprintMetricsLoading={false}
        loading={false}
        error={null}
        realtimeConnected={true}
        onRetry={jest.fn()}
        onTaskOpen={onTaskOpen}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
      />,
    );

    expect(screen.getByText("Realtime live")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Open Build dashboard" }));
    expect(onTaskOpen).toHaveBeenCalledWith("task-1");
  });
});
