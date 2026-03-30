import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConfirmDialog } from "./confirm-dialog";

describe("ConfirmDialog", () => {
  it("renders string descriptions and confirms destructive actions", async () => {
    const user = userEvent.setup();
    const onConfirm = jest.fn();

    render(
      <ConfirmDialog
        open
        title="Delete project"
        description="This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={onConfirm}
        onCancel={jest.fn()}
      />,
    );

    expect(screen.getByText("Delete project")).toBeInTheDocument();
    expect(screen.getByText("This action cannot be undone.")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Delete" }));

    expect(onConfirm).toHaveBeenCalled();
  });

  it("renders rich descriptions and allows cancellation", async () => {
    const user = userEvent.setup();
    const onCancel = jest.fn();

    render(
      <ConfirmDialog
        open
        title="Pause execution"
        description={<span>Wait for approval before resuming.</span>}
        onConfirm={jest.fn()}
        onCancel={onCancel}
      />,
    );

    expect(screen.getByText("Wait for approval before resuming.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onCancel).toHaveBeenCalled();
  });
});
