import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DecomposeTasksDialog } from "./decompose-tasks-dialog";

describe("DecomposeTasksDialog", () => {
  it("selects blocks and confirms decomposition input", async () => {
    const user = userEvent.setup();
    const onConfirm = jest.fn();

    render(
      <DecomposeTasksDialog
        open
        onOpenChange={jest.fn()}
        blocks={[
          { id: "block-a", text: "First block" },
          { id: "block-b", text: "Second block" },
        ]}
        tasks={[{ id: "task-1", title: "Parent task" }]}
        onConfirm={onConfirm}
      />,
    );

    await user.click(screen.getByRole("checkbox", { name: /Select all blocks/i }));
    await user.click(screen.getByRole("combobox"));
    await user.click(screen.getByRole("option", { name: "Parent task" }));
    await user.click(screen.getByRole("button", { name: "Create Tasks" }));

    expect(onConfirm).toHaveBeenCalledWith({
      blockIds: ["block-a", "block-b"],
      parentTaskId: "task-1",
    });
  });
});
