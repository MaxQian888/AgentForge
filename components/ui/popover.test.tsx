import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Popover,
  PopoverAnchor,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from "./popover";

describe("Popover", () => {
  it("opens anchored content with title and description slots", async () => {
    const user = userEvent.setup();
    const { container } = render(
      <Popover>
        <PopoverAnchor data-testid="anchor" />
        <PopoverTrigger>Open popover</PopoverTrigger>
        <PopoverContent align="end" sideOffset={8}>
          <PopoverHeader>
            <PopoverTitle>Recent activity</PopoverTitle>
            <PopoverDescription>Latest task updates across the project.</PopoverDescription>
          </PopoverHeader>
        </PopoverContent>
      </Popover>
    );

    await user.click(screen.getByRole("button", { name: "Open popover" }));

    expect(container.querySelector('[data-slot="popover-anchor"]')).toBeInTheDocument();
    expect(screen.getByText("Recent activity")).toHaveAttribute("data-slot", "popover-title");
    expect(screen.getByText("Latest task updates across the project.")).toHaveAttribute(
      "data-slot",
      "popover-description",
    );
    expect(document.body.querySelector('[data-slot="popover-content"]')).toBeInTheDocument();
  });
});
