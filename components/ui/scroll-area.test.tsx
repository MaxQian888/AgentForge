import { render, screen } from "@testing-library/react";
import { ScrollArea } from "./scroll-area";

describe("ScrollArea", () => {
  it("renders viewport content and custom horizontal scrollbar", () => {
    const { container } = render(
      <ScrollArea className="h-40 w-48">
        <div>Scrollable content</div>
      </ScrollArea>
    );

    expect(container.querySelector('[data-slot="scroll-area"]')).toHaveClass("h-40");
    expect(container.querySelector('[data-slot="scroll-area-viewport"]')).toBeInTheDocument();
    expect(screen.getByText("Scrollable content")).toBeInTheDocument();

    expect(container.querySelector('[data-slot="scroll-area-scrollbar"]')).toBeNull();
  });
});
