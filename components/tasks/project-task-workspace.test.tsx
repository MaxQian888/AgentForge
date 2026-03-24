import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectTaskWorkspace } from "./project-task-workspace";
import type { Task } from "@/lib/stores/task-store";
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

type DndMock = {
  __getLastOnDragEnd: () => ((result: unknown) => void | Promise<void>) | null;
};

const tasks: Task[] = [
  {
    id: "task-1",
    projectId: "project-1",
    title: "Implement timeline view",
    description: "Build the horizontal planning lane.",
    status: "in_progress",
    priority: "high",
    assigneeId: "member-1",
    assigneeType: "human",
    assigneeName: "Alice",
    cost: 2.5,
    spentUsd: 2.5,
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
    title: "Calendar polish",
    description: "Keep unscheduled tasks visible.",
    status: "triaged",
    priority: "medium",
    assigneeId: null,
    assigneeType: null,
    assigneeName: null,
    cost: null,
    spentUsd: 0,
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

describe("ProjectTaskWorkspace", () => {
  beforeEach(() => {
    window.history.replaceState({}, "", "/project?id=project-1");
    useTaskWorkspaceStore.setState({
      viewMode: "board",
      filters: createDefaultTaskWorkspaceFilters(),
      selectedTaskId: null,
    });
  });

  function getLastOnDragEnd() {
    return (jest.requireMock("@hello-pangea/dnd") as DndMock).__getLastOnDragEnd();
  }

  it("keeps filters across views and routes task selection through one workspace", async () => {
    const user = userEvent.setup();
    const onTaskOpen = jest.fn();

    render(
      <ProjectTaskWorkspace
        tasks={tasks}
        loading={false}
        error={null}
        onRetry={jest.fn()}
        onTaskOpen={onTaskOpen}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
      />
    );

    await user.type(screen.getByLabelText("Search tasks"), "calendar");
    await user.click(screen.getByRole("tab", { name: "List" }));

    expect(screen.getByText("Calendar polish")).toBeInTheDocument();
    expect(
      screen.queryByText("Implement timeline view")
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Open Calendar polish" }));
    expect(onTaskOpen).toHaveBeenCalledWith("task-2");

    await user.click(screen.getByRole("tab", { name: "Calendar" }));
    expect(screen.getByText("Calendar polish")).toBeInTheDocument();
    expect(
      screen.queryByText("Implement timeline view")
    ).not.toBeInTheDocument();
  });

  it("shows an explicit empty state when the project has no tasks", () => {
    render(
      <ProjectTaskWorkspace
        tasks={[]}
        loading={false}
        error={null}
        onRetry={jest.fn()}
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
      />
    );

    expect(screen.getByText("No tasks yet")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Create the first task to start using Board, List, Timeline, and Calendar views."
      )
    ).toBeInTheDocument();
  });

  it("persists board drag updates through the shared status callback", async () => {
    const onTaskStatusChange = jest.fn().mockResolvedValue(undefined);

    render(
      <ProjectTaskWorkspace
        tasks={tasks}
        loading={false}
        error={null}
        onRetry={jest.fn()}
        onTaskOpen={jest.fn()}
        onTaskStatusChange={onTaskStatusChange}
        onTaskScheduleChange={jest.fn()}
      />
    );

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

    render(
      <ProjectTaskWorkspace
        tasks={tasks}
        loading={false}
        error={null}
        onRetry={jest.fn()}
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={onTaskScheduleChange}
      />
    );

    await user.click(screen.getByRole("tab", { name: "Timeline" }));

    expect(screen.getByText("Unscheduled")).toBeInTheDocument();
    expect(screen.getByText("Calendar polish")).toBeInTheDocument();

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

    render(
      <ProjectTaskWorkspace
        tasks={tasks}
        loading={false}
        error={null}
        onRetry={jest.fn()}
        onTaskOpen={jest.fn()}
        onTaskStatusChange={jest.fn()}
        onTaskScheduleChange={jest.fn()}
      />
    );

    await user.click(screen.getByRole("tab", { name: "List" }));

    expect(screen.getByText("Stalled")).toBeInTheDocument();
    expect(screen.getByText("No recent update")).toBeInTheDocument();
    expect(screen.getByText("At risk")).toBeInTheDocument();
    expect(screen.getByText("No assignee")).toBeInTheDocument();
  });
});
