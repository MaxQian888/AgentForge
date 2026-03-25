import { render, screen } from "@testing-library/react";
import { Label } from "./label";

describe("Label", () => {
  it("renders a label bound to an input", () => {
    render(
      <div>
        <Label htmlFor="agent-name">Agent name</Label>
        <input id="agent-name" />
      </div>,
    );

    const input = screen.getByLabelText("Agent name");
    const label = screen.getByText("Agent name");

    expect(input).toHaveAttribute("id", "agent-name");
    expect(label).toHaveAttribute("data-slot", "label");
    expect(label).toHaveClass("font-medium");
  });

  it("merges custom class names", () => {
    render(<Label className="text-primary">Scope</Label>);

    expect(screen.getByText("Scope")).toHaveClass("text-primary");
  });
});
