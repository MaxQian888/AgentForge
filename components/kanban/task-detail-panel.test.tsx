import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TaskDetailPanel } from "./task-detail-panel";
import type { Task } from "@/lib/stores/task-store";

const updateTask = jest.fn();
const transitionTask = jest.fn();

jest.mock("@/lib/stores/task-store", () => ({
  useTaskStore: (selector: (state: {
    updateTask: typeof updateTask;
    transitionTask: typeof transitionTask;
  }) => unknown) =>
    selector({
      updateTask,
      transitionTask,
    }),
}));

const task: Task = {
  id: "task-1",
  projectId: "project-1",
  title: "Implement timeline view",
  description: "Build the horizontal planning lane.",
  status: "in_progress",
  priority: "high",
  assigneeId: "member-1",
  assigneeType: "human",
  assigneeName: "Alice",
  cost: 4.5,
  spentUsd: 4.5,
  plannedStartAt: "2026-03-30T09:00:00.000Z",
  plannedEndAt: "2026-03-31T18:00:00.000Z",
  progress: null,
  createdAt: "2026-03-24T10:00:00.000Z",
  updatedAt: "2026-03-24T12:00:00.000Z",
};

describe("TaskDetailPanel", () => {
  beforeEach(() => {
    updateTask.mockReset();
    updateTask.mockResolvedValue(undefined);
    transitionTask.mockReset();
    transitionTask.mockResolvedValue(undefined);
  });

  it("does not persist invalid planning edits from the detail panel", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();

    render(
      <TaskDetailPanel
        task={task}
        open
        onOpenChange={onOpenChange}
      />
    );

    const dateInputs = document.querySelectorAll<HTMLInputElement>('input[type="date"]');

    expect(dateInputs).toHaveLength(2);

    await user.clear(dateInputs[0]);
    await user.type(dateInputs[0], "2026-04-02");
    await user.clear(dateInputs[1]);
    await user.type(dateInputs[1], "2026-04-01");
    await user.click(screen.getByRole("button", { name: "Save Changes" }));

    expect(updateTask).not.toHaveBeenCalled();
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(
      screen.getByText(/end date cannot be earlier than start date/i)
    ).toBeInTheDocument();
  });
});
