import type { ReactNode } from "react";

jest.mock("@hello-pangea/dnd", () => ({
  Droppable: ({
    children,
  }: {
    children: (
      provided: {
        innerRef: () => void;
        droppableProps: Record<string, unknown>;
        placeholder: ReactNode;
      },
      snapshot: { isDraggingOver: boolean },
    ) => ReactNode;
  }) =>
    children(
      {
        innerRef: jest.fn(),
        droppableProps: {},
        placeholder: <div data-testid="placeholder" />,
      },
      { isDraggingOver: true },
    ),
}));

jest.mock("./task-card", () => ({
  TaskCard: ({
    task,
    onClick,
  }: {
    task: { id: string; title: string };
    onClick: () => void;
  }) => (
    <button data-testid={`task-card-${task.id}`} onClick={onClick}>
      {task.title}
    </button>
  ),
}));

import userEvent from "@testing-library/user-event";
import { render, screen } from "@testing-library/react";
import { Column } from "./column";
import type { Task } from "@/lib/stores/task-store";

function makeTask(overrides: Partial<Task> & { id: string; title: string }): Task {
  return {
    projectId: "project-1",
    description: "",
    status: "in_progress",
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
    ...overrides,
    id: overrides.id,
    title: overrides.title,
  };
}

describe("Column", () => {
  it("renders the task count and forwards task clicks", async () => {
    const user = userEvent.setup();
    const onTaskClick = jest.fn();
    const tasks = [
      makeTask({ id: "task-1", title: "Build review board" }),
      makeTask({ id: "task-2", title: "Ship notifications" }),
    ];

    const { container } = render(
      <Column
        status="in_progress"
        tasks={tasks}
        selectedTaskId="task-2"
        displayOptions={{ density: "comfortable", showDescriptions: true, showLinkedDocs: false }}
        linkedDocsByTask={{}}
        onTaskClick={onTaskClick}
      />,
    );

    expect(screen.getByText("In Progress")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByTestId("placeholder")).toBeInTheDocument();

    await user.click(screen.getByTestId("task-card-task-1"));
    expect(onTaskClick).toHaveBeenCalledWith(tasks[0]);

    expect(container.querySelector(".bg-accent\\/50")).toBeInTheDocument();
  });

  it("supports quick task creation from the column header with the status pre-set", async () => {
    const user = userEvent.setup();
    const onQuickCreateTask = jest.fn().mockResolvedValue(undefined);

    render(
      <Column
        status="in_progress"
        tasks={[]}
        selectedTaskId={null}
        displayOptions={{ density: "comfortable", showDescriptions: true, showLinkedDocs: false }}
        linkedDocsByTask={{}}
        onTaskClick={jest.fn()}
        onQuickCreateTask={onQuickCreateTask}
      />
    );

    await user.click(screen.getByRole("button", { name: "Quick create in In Progress" }));
    await user.type(screen.getByLabelText("Task title"), "Ship alerts");
    await user.click(screen.getByRole("button", { name: "Add task" }));

    expect(onQuickCreateTask).toHaveBeenCalledWith("in_progress", "Ship alerts");
  });
});
