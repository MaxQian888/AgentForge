import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "./sheet";

describe("Sheet", () => {
  it("opens a left sheet and closes through the explicit close action", async () => {
    const user = userEvent.setup();
    render(
      <Sheet>
        <SheetTrigger>Open sheet</SheetTrigger>
        <SheetContent side="left">
          <SheetHeader>
            <SheetTitle>Task details</SheetTitle>
            <SheetDescription>Review the latest task context.</SheetDescription>
          </SheetHeader>
          <SheetFooter>
            <SheetClose asChild>
              <button type="button">Close sheet</button>
            </SheetClose>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    );

    await user.click(screen.getByRole("button", { name: "Open sheet" }));

    expect(screen.getByText("Task details")).toHaveAttribute("data-slot", "sheet-title");
    expect(screen.getByText("Review the latest task context.")).toHaveAttribute(
      "data-slot",
      "sheet-description",
    );
    expect(document.body.querySelector('[data-slot="sheet-content"]')).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Close sheet" }));

    await waitFor(() => {
      expect(screen.queryByText("Task details")).not.toBeInTheDocument();
    });
  });
});
