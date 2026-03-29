import { render, screen, fireEvent } from "@testing-library/react";
import { TaskDependencyGraph } from "./task-dependency-graph";
import type { Task } from "@/lib/stores/task-store";

function makeTask(overrides: Partial<Task> & { id: string; title: string }): Task {
  return {
    projectId: "proj-1",
    parentId: null,
    sprintId: null,
    executionMode: null,
    description: "",
    status: "inbox",
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
    createdAt: "2025-01-01T00:00:00Z",
    updatedAt: "2025-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("TaskDependencyGraph", () => {
  it("renders empty state when no tasks", () => {
    render(<TaskDependencyGraph tasks={[]} onTaskClick={jest.fn()} />);
    expect(screen.getByText("No tasks to visualize.")).toBeInTheDocument();
  });

  it("renders nodes for tasks with dependency edges", () => {
    const tasks = [
      makeTask({ id: "t1", title: "Task One", status: "done" }),
      makeTask({ id: "t2", title: "Task Two", status: "in_progress", blockedBy: ["t1"] }),
      makeTask({ id: "t3", title: "Task Three", status: "inbox", blockedBy: ["t2"] }),
    ];

    const { container } = render(
      <TaskDependencyGraph tasks={tasks} onTaskClick={jest.fn()} />
    );

    expect(screen.getByText("Task One")).toBeInTheDocument();
    expect(screen.getByText("Task Two")).toBeInTheDocument();
    expect(screen.getByText("Task Three")).toBeInTheDocument();

    // Should have edges (path elements)
    const paths = container.querySelectorAll("path");
    expect(paths.length).toBeGreaterThanOrEqual(2);
  });

  it("calls onTaskClick when a node is clicked", () => {
    const onClick = jest.fn();
    const tasks = [makeTask({ id: "t1", title: "Clickable Task", status: "done" })];

    render(<TaskDependencyGraph tasks={tasks} onTaskClick={onClick} />);
    fireEvent.click(screen.getByText("Clickable Task"));
    expect(onClick).toHaveBeenCalledWith("t1");
  });
});
