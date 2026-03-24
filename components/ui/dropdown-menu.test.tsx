import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "./dropdown-menu";

describe("DropdownMenu", () => {
  it("renders grouped, nested, checkbox, and radio menu items", async () => {
    const user = userEvent.setup();
    render(
      <DropdownMenu>
        <DropdownMenuTrigger>Open menu</DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuGroup>
            <DropdownMenuLabel inset>Actions</DropdownMenuLabel>
            <DropdownMenuItem inset>Profile</DropdownMenuItem>
            <DropdownMenuItem variant="destructive">Delete</DropdownMenuItem>
            <DropdownMenuCheckboxItem checked>Subscribed</DropdownMenuCheckboxItem>
            <DropdownMenuSeparator />
            <DropdownMenuRadioGroup value="review">
              <DropdownMenuRadioItem value="review">Review</DropdownMenuRadioItem>
            </DropdownMenuRadioGroup>
            <DropdownMenuSub open>
              <DropdownMenuSubTrigger>More</DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                <DropdownMenuItem>Advanced</DropdownMenuItem>
              </DropdownMenuSubContent>
            </DropdownMenuSub>
          </DropdownMenuGroup>
          <DropdownMenuItem>
            Settings
            <DropdownMenuShortcut>⌘S</DropdownMenuShortcut>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    );

    await user.click(screen.getByRole("button", { name: "Open menu" }));

    expect(screen.getByText("Actions")).toHaveAttribute("data-slot", "dropdown-menu-label");
    expect(screen.getByText("Profile").closest('[data-slot="dropdown-menu-item"]')).toHaveAttribute(
      "data-inset",
      "true",
    );
    expect(screen.getByText("Delete").closest('[data-slot="dropdown-menu-item"]')).toHaveAttribute(
      "data-variant",
      "destructive",
    );
    expect(screen.getByText("Subscribed")).toBeInTheDocument();
    expect(screen.getByText("Review")).toHaveAttribute(
      "data-slot",
      "dropdown-menu-radio-item",
    );
    expect(screen.getByText("More")).toHaveAttribute(
      "data-slot",
      "dropdown-menu-sub-trigger",
    );
    expect(screen.getByText("Advanced")).toHaveAttribute(
      "data-slot",
      "dropdown-menu-item",
    );
    expect(screen.getByText("⌘S")).toHaveAttribute(
      "data-slot",
      "dropdown-menu-shortcut",
    );
    expect(document.body.querySelector('[data-slot="dropdown-menu-separator"]')).toBeInTheDocument();
  });
});
