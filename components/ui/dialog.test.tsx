import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
} from "./dialog";

describe("Dialog", () => {
  it("opens content and closes from the footer action", async () => {
    const user = userEvent.setup();
    render(
      <Dialog>
        <DialogTrigger>Open dialog</DialogTrigger>
        <DialogPortal>
          <DialogOverlay />
        </DialogPortal>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Invite teammate</DialogTitle>
            <DialogDescription>Add someone to the task workspace.</DialogDescription>
          </DialogHeader>
          <DialogFooter showCloseButton>
            <button type="button">Confirm</button>
          </DialogFooter>
          <DialogClose asChild>
            <button type="button">Dismiss</button>
          </DialogClose>
        </DialogContent>
      </Dialog>
    );

    await user.click(screen.getByRole("button", { name: "Open dialog" }));

    expect(screen.getByText("Invite teammate")).toHaveAttribute(
      "data-slot",
      "dialog-title",
    );
    expect(screen.getByText("Add someone to the task workspace.")).toHaveAttribute(
      "data-slot",
      "dialog-description",
    );
    expect(document.body.querySelector('[data-slot="dialog-overlay"]')).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Close" })).toHaveLength(2);
    expect(screen.getByRole("button", { name: "Dismiss" })).toHaveAttribute(
      "data-slot",
      "dialog-close",
    );

    await user.click(screen.getByRole("button", { name: "Dismiss" }));

    await waitFor(() => {
      expect(screen.queryByText("Invite teammate")).not.toBeInTheDocument();
    });
  });
});
