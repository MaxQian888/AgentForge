import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RelatedTasksPanel } from "./related-tasks-panel";

describe("RelatedTasksPanel", () => {
  it("renders linked tasks and add/remove actions", async () => {
    const user = userEvent.setup();
    const onAddTask = jest.fn();
    const onRemoveTask = jest.fn();

    render(
      <RelatedTasksPanel
        tasks={[
          {
            linkId: "link-1",
            taskId: "task-1",
            title: "Implement parser",
            status: "in_progress",
            assigneeName: "Alice",
            dueDate: "2026-03-30",
          },
        ]}
        onAddTask={onAddTask}
        onRemoveTask={onRemoveTask}
      />,
    );

    expect(screen.getByText("Related Tasks")).toBeInTheDocument();
    expect(screen.getByText("Implement parser")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Link Task" }));
    expect(onAddTask).toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Remove Implement parser" }));
    expect(onRemoveTask).toHaveBeenCalledWith("link-1");
  });
});
