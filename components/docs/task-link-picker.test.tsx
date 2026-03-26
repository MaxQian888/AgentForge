import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TaskLinkPicker } from "./task-link-picker";

describe("TaskLinkPicker", () => {
  it("renders tasks and supports picking one", async () => {
    const user = userEvent.setup();
    const onPick = jest.fn();

    render(
      <TaskLinkPicker
        open
        onOpenChange={jest.fn()}
        tasks={[{ id: "task-1", title: "Implement parser", status: "triaged" }]}
        onPick={onPick}
      />,
    );

    await user.click(screen.getByRole("button", { name: /Implement parser/i }));
    expect(onPick).toHaveBeenCalledWith("task-1");
  });
});
