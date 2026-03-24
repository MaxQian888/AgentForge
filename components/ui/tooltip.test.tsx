import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "./tooltip";

describe("Tooltip", () => {
  it("shows tooltip content on hover with the default provider delay override", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger>Hover me</TooltipTrigger>
          <TooltipContent sideOffset={6}>Helpful context</TooltipContent>
        </Tooltip>
      </TooltipProvider>
    );

    await user.hover(screen.getByRole("button", { name: "Hover me" }));

    expect(document.body.querySelector('[data-slot="tooltip-content"]')).toHaveAttribute(
      "data-slot",
      "tooltip-content",
    );
  });
});
