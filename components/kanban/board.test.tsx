import type { ReactNode } from "react";

let lastOnDragEnd: ((result: unknown) => void | Promise<void>) | null = null;

jest.mock("@hello-pangea/dnd", () => ({
  DragDropContext: ({
    children,
    onDragEnd,
  }: {
    children: ReactNode;
    onDragEnd: (result: unknown) => void | Promise<void>;
  }) => {
    lastOnDragEnd = onDragEnd;
    return <div data-testid="drag-context">{children}</div>;
  },
}));

jest.mock("./column", () => ({
  Column: ({
    status,
    tasks,
    pendingTaskIds = [],
    onTaskClick,
  }: {
    status: string;
    tasks: Array<{ id: string; title: string }>;
    pendingTaskIds?: string[];
    onTaskClick: (task: { id: string; title: string }) => void;
  }) => (
    <div data-testid={`column-${status}`}>
      <div>{status}:{tasks.length}</div>
      {tasks.map((task) => (
        <button
          key={task.id}
          data-testid={`task-${status}-${task.id}`}
          onClick={() => onTaskClick(task)}
        >
          {task.title}
          {pendingTaskIds.includes(task.id) ? " (pending)" : ""}
        </button>
      ))}
    </div>
  ),
}));

import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Board } from "./board";
import type { Task } from "@/lib/stores/task-store";

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  const promise = new Promise<T>((promiseResolve) => {
    resolve = promiseResolve;
  });

  return { promise, resolve };
}

function makeTask(
  overrides: Partial<Task> & { id: string; title: string; status: Task["status"] },
): Task {
  return {
    id: overrides.id,
    projectId: "project-1",
    title: overrides.title,
    description: "",
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

describe("Board", () => {
  it("groups tasks into columns and forwards task clicks", async () => {
    const user = userEvent.setup();
    const onTaskClick = jest.fn();
    const tasks = [
      makeTask({ id: "task-1", title: "Build dashboard", status: "inbox" }),
      makeTask({ id: "task-2", title: "Review queues", status: "done" }),
    ];

    render(
      <Board
        tasks={tasks}
        allTasks={tasks}
        selectedTaskId={null}
        displayOptions={{ density: "comfortable", showDescriptions: true, showLinkedDocs: false }}
        linkedDocsByTask={{}}
        onTaskClick={onTaskClick}
        onTaskStatusChange={jest.fn()}
      />,
    );

    expect(screen.getByTestId("column-inbox")).toHaveTextContent("inbox:1");
    expect(screen.getByTestId("column-done")).toHaveTextContent("done:1");
    expect(screen.getByTestId("column-blocked")).toHaveTextContent("blocked:0");

    await user.click(screen.getByTestId("task-inbox-task-1"));
    expect(onTaskClick).toHaveBeenCalledWith(tasks[0]);
  });

  it("optimistically moves a dragged task and shows a pending state until persistence completes", async () => {
    const statusChange = createDeferred<void>();
    const successChange = jest.fn().mockReturnValue(statusChange.promise);
    const successTasks = [makeTask({ id: "task-1", title: "Build dashboard", status: "inbox" })];
    render(
      <Board
        tasks={successTasks}
        allTasks={successTasks}
        selectedTaskId={null}
        displayOptions={{ density: "comfortable", showDescriptions: true, showLinkedDocs: false }}
        linkedDocsByTask={{}}
        onTaskClick={jest.fn()}
        onTaskStatusChange={successChange}
      />,
    );

    await act(async () => {
      void lastOnDragEnd?.({
        draggableId: "task-1",
        source: { droppableId: "inbox" },
        destination: { droppableId: "done" },
      });
    });

    expect(successChange).toHaveBeenCalledWith("task-1", "done");
    expect(screen.queryByTestId("task-inbox-task-1")).not.toBeInTheDocument();
    expect(screen.getByTestId("task-done-task-1")).toHaveTextContent("Build dashboard (pending)");

    statusChange.resolve();
    await act(async () => {
      await Promise.resolve();
    });

    expect(screen.getByTestId("task-done-task-1")).toHaveTextContent("Build dashboard");
    expect(screen.getByTestId("task-done-task-1")).not.toHaveTextContent("(pending)");
  });

  it("rolls a task back to its original column when a drag update fails", async () => {
    const failingChange = jest.fn().mockRejectedValue(new Error("Cannot transition from inbox to blocked"));
    const failingTasks = [makeTask({ id: "task-2", title: "Review queues", status: "inbox" })];
    render(
      <Board
        tasks={failingTasks}
        allTasks={failingTasks}
        selectedTaskId={null}
        displayOptions={{ density: "comfortable", showDescriptions: true, showLinkedDocs: false }}
        linkedDocsByTask={{}}
        onTaskClick={jest.fn()}
        onTaskStatusChange={failingChange}
      />,
    );

    await act(async () => {
      await lastOnDragEnd?.({
        draggableId: "task-2",
        source: { droppableId: "inbox" },
        destination: { droppableId: "blocked" },
      });
    });

    expect(screen.getByTestId("task-inbox-task-2")).toHaveTextContent("Review queues");
    expect(screen.queryByTestId("task-blocked-task-2")).not.toBeInTheDocument();
    expect(await screen.findByText("Cannot transition from inbox to blocked")).toBeInTheDocument();
  });
});
