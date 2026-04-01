jest.mock("@/components/ui/select", () => {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const React = require("react");

  function flattenOptions(children: React.ReactNode): Array<{ value: string; label: string }> {
    const options: Array<{ value: string; label: string }> = [];
    function visit(node: React.ReactNode) {
      React.Children.forEach(node, (child: unknown) => {
        if (!React.isValidElement(child)) return;
        const element = child as React.ReactElement<{ children?: React.ReactNode; value?: string }>;
        if (element.props.value !== undefined) {
          options.push({
            value: element.props.value,
            label: typeof element.props.children === "string" ? element.props.children : String(element.props.value),
          });
          return;
        }
        visit(element.props.children);
      });
    }
    visit(children);
    return options;
  }

  return {
    Select: ({
      value,
      onValueChange,
      disabled,
      children,
    }: {
      value?: string;
      onValueChange?: (value: string) => void;
      disabled?: boolean;
      children?: React.ReactNode;
    }) => {
      const options = flattenOptions(children);
      let ariaLabel: string | undefined;
      React.Children.forEach(children, (child: unknown) => {
        if (!React.isValidElement(child)) return;
        const el = child as React.ReactElement<{ "aria-label"?: string; children?: React.ReactNode }>;
        if (el.props["aria-label"]) {
          ariaLabel = el.props["aria-label"];
        }
      });

      return (
        <select
          aria-label={ariaLabel}
          disabled={disabled}
          value={value}
          onChange={(e: React.ChangeEvent<HTMLSelectElement>) =>
            onValueChange?.(e.target.value)
          }
        >
          {options.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      );
    },
    SelectTrigger: ({ children, ...props }: { children?: React.ReactNode; "aria-label"?: string; className?: string }) => (
      <span aria-label={props["aria-label"]}>{children}</span>
    ),
    SelectValue: () => null,
    SelectContent: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
    SelectItem: ({
      value,
      children,
    }: {
      value: string;
      children?: React.ReactNode;
    }) => <span data-value={value}>{children}</span>,
  };
});

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
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { TaskWorkspaceMain } from "./task-workspace-main";
import {
  createDefaultTaskWorkspaceFilters,
  useTaskWorkspaceStore,
} from "@/lib/stores/task-workspace-store";
import { useDocsStore } from "@/lib/stores/docs-store";
import { useEntityLinkStore } from "@/lib/stores/entity-link-store";
import { useCustomFieldStore } from "@/lib/stores/custom-field-store";
import type { Sprint, SprintMetrics } from "@/lib/stores/sprint-store";
import type { Task } from "@/lib/stores/task-store";

function makeTask(
  overrides: Partial<Task> & { id: string; title: string; status: Task["status"] },
): Task {
  const { id, title, status, ...rest } = overrides;

  return {
    projectId: "project-1",
    id,
    title,
    description: rest.description ?? `${title} description`,
    status,
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
    ...rest,
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
    useDocsStore.setState({
      tree: [],
      currentPage: null,
      comments: [],
      versions: [],
      templates: [],
      favorites: [],
      recentAccess: [],
      loading: false,
      saving: false,
      error: null,
    });
    useEntityLinkStore.setState({
      linksByEntity: {},
      loading: false,
      error: null,
    });
    useCustomFieldStore.setState({
      definitionsByProject: {},
      valuesByTask: {},
      loadingByProject: {},
      errorByProject: {},
    });
  });

  it("renders board view with sprint metrics and switches to dependency view via store", async () => {
    const tasks = [
      makeTask({ id: "task-1", title: "Build dashboard", status: "inbox" }),
      makeTask({ id: "task-2", title: "Review queues", status: "done", assigneeId: "member-1", assigneeName: "Alice" }),
    ];

    const { rerender } = render(
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

    expect(screen.getByTestId("burndown-chart")).toHaveTextContent("5 planned");
    expect(screen.getByTestId("board-view")).toHaveTextContent("2 board tasks");

    // Simulate view mode change from sidebar
    act(() => {
      useTaskWorkspaceStore.setState((state) => ({ ...state, viewMode: "dependencies" }));
    });
    rerender(
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
    expect(screen.getByTestId("dependency-view")).toHaveTextContent("2 dependency tasks");
  });

  it("renders a quick filter bar and scopes board tasks by assignee, tag, due date, and priority", async () => {
    const user = userEvent.setup();
    const tasks = [
      makeTask({
        id: "task-1",
        title: "Alpha task",
        status: "in_progress",
        assigneeId: "member-1",
        assigneeName: "Alice",
        labels: ["frontend"],
        priority: "high",
        plannedStartAt: "2026-03-25T09:00:00.000Z",
        plannedEndAt: "2026-03-27T18:00:00.000Z",
      }),
      makeTask({
        id: "task-2",
        title: "Beta task",
        status: "triaged",
        assigneeId: "member-2",
        assigneeName: "Bob",
        labels: ["backend"],
        priority: "medium",
        plannedStartAt: "2026-04-02T09:00:00.000Z",
        plannedEndAt: "2026-04-02T18:00:00.000Z",
      }),
      makeTask({
        id: "task-3",
        title: "Gamma task",
        status: "triaged",
        assigneeId: "member-2",
        assigneeName: "Bob",
        labels: ["frontend"],
        priority: "low",
        plannedStartAt: "2026-03-29T09:00:00.000Z",
        plannedEndAt: "2026-03-30T18:00:00.000Z",
      }),
    ];

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
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
      />
    );

    const filterBar = screen.getByTestId("task-quick-filter-bar");

    expect(screen.getByTestId("board-view")).toHaveTextContent("3 board tasks");

    await user.selectOptions(within(filterBar).getByLabelText("Assignee"), "member-1");
    expect(screen.getByTestId("board-view")).toHaveTextContent("1 board tasks");

    await user.selectOptions(within(filterBar).getByLabelText("Assignee"), "all");
    await user.click(within(filterBar).getByRole("button", { name: "frontend" }));
    expect(screen.getByTestId("board-view")).toHaveTextContent("2 board tasks");

    fireEvent.change(within(filterBar).getByLabelText("Due from"), {
      target: { value: "2026-03-28" },
    });
    expect(screen.getByTestId("board-view")).toHaveTextContent("1 board tasks");

    await user.selectOptions(within(filterBar).getByLabelText("Priority"), "high");
    expect(screen.getByText("No tasks match the current filters")).toBeInTheDocument();
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

    await user.click(screen.getByRole("button", { name: "Open Build dashboard" }));
    expect(onTaskOpen).toHaveBeenCalledWith("task-1");
  });

  it("highlights matching search text in list and timeline task results", async () => {
    useTaskWorkspaceStore.setState((state) => ({
      ...state,
      viewMode: "list",
      filters: {
        ...state.filters,
        search: "timeline",
      },
    }));

    render(
      <TaskWorkspaceMain
        projectId="project-1"
        tasks={[
          makeTask({
            id: "task-1",
            title: "Timeline polish",
            description: "Refine the timeline interactions for scheduling.",
            status: "in_progress",
            plannedStartAt: "2026-03-25T09:00:00.000Z",
            plannedEndAt: "2026-03-27T18:00:00.000Z",
          }),
        ]}
        sprints={[]}
        sprintMetrics={null}
        sprintMetricsLoading={false}
        loading={false}
        error={null}
        realtimeConnected={true}
        onRetry={jest.fn()}
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
      />
    );

    expect(screen.getAllByText(/timeline/i, { selector: "mark" }).length).toBeGreaterThanOrEqual(2);

    act(() => {
      useTaskWorkspaceStore.setState((state) => ({ ...state, viewMode: "timeline" }));
    });

    expect(screen.getAllByText(/timeline/i, { selector: "mark" }).length).toBeGreaterThanOrEqual(1);
  });

  it("sorts list rows when the user clicks a column header", async () => {
    const user = userEvent.setup();
    const tasks = [
      makeTask({ id: "task-1", title: "Zulu task", status: "in_progress" }),
      makeTask({ id: "task-2", title: "Alpha task", status: "inbox" }),
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
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
      />
    );

    const openButtons = () =>
      screen
        .getAllByRole("button", { name: /Open .*task/i })
        .map((button) => button.textContent);

    expect(openButtons()).toEqual(["Open Zulu task", "Open Alpha task"]);

    await user.click(screen.getByRole("button", { name: "Sort by Task" }));
    expect(openButtons()).toEqual(["Open Alpha task", "Open Zulu task"]);

    await user.click(screen.getByRole("button", { name: "Sort by Task" }));
    expect(openButtons()).toEqual(["Open Zulu task", "Open Alpha task"]);
  });

  it("supports inline status and priority changes from list rows", async () => {
    const user = userEvent.setup();
    const onTaskStatusChange = jest.fn().mockResolvedValue(undefined);
    const onTaskSave = jest.fn().mockResolvedValue(undefined);
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
        onTaskOpen={jest.fn()}
        onTaskStatusChange={onTaskStatusChange}
        onTaskScheduleChange={jest.fn()}
        onTaskSave={onTaskSave}
      />
    );

    await user.selectOptions(
      screen.getByLabelText("Status for Build dashboard"),
      "done"
    );
    expect(onTaskStatusChange).toHaveBeenCalledWith("task-1", "done");

    await user.selectOptions(
      screen.getByLabelText("Priority for Build dashboard"),
      "high"
    );
    expect(onTaskSave).toHaveBeenCalledWith("task-1", { priority: "high" });
  });

  it("shows an unscheduled indicator in list planning cells when dates are missing", () => {
    useTaskWorkspaceStore.setState((state) => ({
      ...state,
      viewMode: "list",
    }));

    render(
      <TaskWorkspaceMain
        projectId="project-1"
        tasks={[
          makeTask({ id: "task-1", title: "Backlog cleanup", status: "triaged" }),
        ]}
        sprints={[]}
        sprintMetrics={null}
        sprintMetricsLoading={false}
        loading={false}
        error={null}
        realtimeConnected={true}
        onRetry={jest.fn()}
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
      />
    );

    expect(screen.getByText("Unscheduled")).toBeInTheDocument();
  });

  it("renders linked docs as clickable chips in list view", () => {
    useTaskWorkspaceStore.setState((state) => ({
      ...state,
      viewMode: "list",
      displayOptions: {
        ...state.displayOptions,
        showLinkedDocs: true,
      },
    }));
    useDocsStore.setState({
      tree: [
        {
          id: "page-1",
          spaceId: "space-1",
          parentId: null,
          title: "Architecture brief",
          content: "[]",
          contentText: "Doc preview",
          path: "/architecture-brief",
          sortOrder: 0,
          isTemplate: false,
          isSystem: false,
          isPinned: false,
          createdBy: null,
          updatedBy: null,
          createdAt: "2026-03-24T09:00:00.000Z",
          updatedAt: "2026-03-24T09:00:00.000Z",
          deletedAt: null,
          children: [],
        },
      ],
    });
    useEntityLinkStore.setState({
      linksByEntity: {
        "task:task-1": [
          {
            id: "link-1",
            projectId: "project-1",
            sourceType: "task",
            sourceId: "task-1",
            targetType: "wiki_page",
            targetId: "page-1",
            linkType: "design",
            anchorBlockId: null,
            createdBy: "user-1",
            createdAt: "2026-03-24T09:05:00.000Z",
            deletedAt: null,
          },
        ],
      },
    });

    render(
      <TaskWorkspaceMain
        projectId="project-1"
        tasks={[
          makeTask({ id: "task-1", title: "Backlog cleanup", status: "triaged" }),
        ]}
        sprints={[]}
        sprintMetrics={null}
        sprintMetricsLoading={false}
        loading={false}
        error={null}
        realtimeConnected={true}
        onRetry={jest.fn()}
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
      />
    );

    expect(screen.getByRole("link", { name: "Architecture brief" })).toHaveAttribute(
      "href",
      "/docs?pageId=page-1"
    );
  });

  it("lets the user toggle custom field columns in list view", async () => {
    const user = userEvent.setup();

    useTaskWorkspaceStore.setState((state) => ({
      ...state,
      viewMode: "list",
    }));
    useCustomFieldStore.setState({
      definitionsByProject: {
        "project-1": [
          {
            id: "field-1",
            projectId: "project-1",
            name: "Risk Level",
            fieldType: "text",
            options: null,
            sortOrder: 1,
            required: false,
            createdAt: "2026-03-24T09:00:00.000Z",
            updatedAt: "2026-03-24T09:00:00.000Z",
          },
        ],
      },
      valuesByTask: {
        "task-1": [
          {
            id: "value-1",
            taskId: "task-1",
            fieldDefId: "field-1",
            value: "High",
            createdAt: "2026-03-24T09:10:00.000Z",
            updatedAt: "2026-03-24T09:10:00.000Z",
          },
        ],
      },
    });

    render(
      <TaskWorkspaceMain
        projectId="project-1"
        tasks={[
          makeTask({ id: "task-1", title: "Backlog cleanup", status: "triaged" }),
        ]}
        sprints={[]}
        sprintMetrics={null}
        sprintMetricsLoading={false}
        loading={false}
        error={null}
        realtimeConnected={true}
        onRetry={jest.fn()}
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
      />
    );

    expect(screen.getByRole("columnheader", { name: "Risk Level" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Toggle custom column Risk Level" }));

    expect(screen.queryByRole("columnheader", { name: "Risk Level" })).not.toBeInTheDocument();
  });

  it("filters, sorts, and groups the list by custom field values", async () => {
    const user = userEvent.setup();

    useTaskWorkspaceStore.setState((state) => ({
      ...state,
      viewMode: "list",
    }));
    useCustomFieldStore.setState({
      definitionsByProject: {
        "project-1": [
          {
            id: "field-risk",
            projectId: "project-1",
            name: "Risk Level",
            fieldType: "select",
            options: ["High", "Low"],
            sortOrder: 1,
            required: false,
            createdAt: "2026-03-24T09:00:00.000Z",
            updatedAt: "2026-03-24T09:00:00.000Z",
          },
        ],
      },
      valuesByTask: {
        "task-1": [
          {
            id: "value-1",
            taskId: "task-1",
            fieldDefId: "field-risk",
            value: "Low",
            createdAt: "2026-03-24T09:10:00.000Z",
            updatedAt: "2026-03-24T09:10:00.000Z",
          },
        ],
        "task-2": [
          {
            id: "value-2",
            taskId: "task-2",
            fieldDefId: "field-risk",
            value: "High",
            createdAt: "2026-03-24T09:10:00.000Z",
            updatedAt: "2026-03-24T09:10:00.000Z",
          },
        ],
      },
    });

    render(
      <TaskWorkspaceMain
        projectId="project-1"
        tasks={[
          makeTask({ id: "task-1", title: "Backlog cleanup", status: "triaged" }),
          makeTask({ id: "task-2", title: "Calendar polish", status: "triaged" }),
          makeTask({ id: "task-3", title: "Docs sweep", status: "triaged" }),
        ]}
        sprints={[]}
        sprintMetrics={null}
        sprintMetricsLoading={false}
        loading={false}
        error={null}
        realtimeConnected={true}
        onRetry={jest.fn()}
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
      />
    );

    await user.selectOptions(screen.getByLabelText("Custom field filter field"), "field-risk");
    await user.selectOptions(screen.getByLabelText("Custom field filter value"), "High");

    expect(screen.getByRole("button", { name: "Open Calendar polish" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Open Backlog cleanup" })).not.toBeInTheDocument();

    await user.selectOptions(screen.getByLabelText("Custom field filter value"), "all");
    await user.selectOptions(screen.getByLabelText("Sort by custom field"), "field-risk");

    const openButtons = screen
      .getAllByRole("button", { name: /Open .*/ })
      .map((button) => button.textContent);
    expect(openButtons.slice(0, 3)).toEqual([
      "Open Calendar polish",
      "Open Backlog cleanup",
      "Open Docs sweep",
    ]);

    await user.selectOptions(screen.getByLabelText("Group by custom field"), "field-risk");

    expect(screen.getByText("Risk Level: High")).toBeInTheDocument();
    expect(screen.getByText("Risk Level: Low")).toBeInTheDocument();
    expect(screen.getByText("Risk Level: Unset")).toBeInTheDocument();
  });

  it("shows board loading skeletons while the board view is fetching", () => {
    render(
      <TaskWorkspaceMain
        projectId="project-1"
        tasks={[]}
        sprints={[]}
        sprintMetrics={null}
        sprintMetricsLoading={false}
        loading={true}
        error={null}
        realtimeConnected={false}
        onRetry={jest.fn()}
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
      />,
    );

    expect(screen.getByTestId("board-loading-skeleton")).toBeInTheDocument();
  });

  it("shows a create task action when the workspace has no tasks", async () => {
    const user = userEvent.setup();
    const onCreateTask = jest.fn();

    render(
      <TaskWorkspaceMain
        projectId="project-1"
        tasks={[]}
        sprints={[]}
        sprintMetrics={null}
        sprintMetricsLoading={false}
        loading={false}
        error={null}
        realtimeConnected={false}
        onRetry={jest.fn()}
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
        onCreateTask={onCreateTask}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Create Task" }));
    expect(onCreateTask).toHaveBeenCalledTimes(1);
  });

  it("reports individual task failures from bulk operations", async () => {
    const user = userEvent.setup();
    const view = (
      <TaskWorkspaceMain
        projectId="project-1"
        tasks={[
          makeTask({ id: "task-1", title: "Build dashboard", status: "in_progress" }),
          makeTask({ id: "task-2", title: "Review queues", status: "triaged" }),
        ]}
        sprints={[]}
        sprintMetrics={null}
        sprintMetricsLoading={false}
        loading={false}
        error={null}
        realtimeConnected={true}
        onRetry={jest.fn()}
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
        onBulkStatusChange={async () => ({
          failed: [{ taskId: "task-2", message: "Cannot transition to done" }],
        })}
      />
    );

    const rendered = render(view);

    await act(async () => {
      useTaskWorkspaceStore.getState().selectAllVisible(["task-1", "task-2"]);
    });
    rendered.rerender(view);

    const toolbar = screen.getByTestId("bulk-action-toolbar");
    const statusSelect = within(toolbar).getAllByRole("combobox")[0];

    await user.selectOptions(statusSelect, "done");

    expect(
      await screen.findByText("Some bulk actions failed")
    ).toBeInTheDocument();
    expect(screen.getByText("Review queues: Cannot transition to done")).toBeInTheDocument();
  });
});
