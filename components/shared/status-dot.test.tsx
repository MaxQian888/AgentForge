import { render, screen } from "@testing-library/react";
import { StatusDot } from "./status-dot";

describe("StatusDot", () => {
  it("uses the status color map and auto-pulses active states", () => {
    render(<StatusDot status="active" size="md" />);

    const dot = screen.getByRole("status");
    expect(dot).toHaveAttribute("aria-label", "active");
    expect(dot).toHaveClass("size-2.5");
    expect(dot).toHaveClass("bg-emerald-500");
    expect(dot).toHaveClass("animate-pulse-dot");
  });

  it("supports non-pulsing custom states", () => {
    render(<StatusDot status="pending" pulse={false} />);

    const dot = screen.getByRole("status");
    expect(dot).toHaveClass("size-2");
    expect(dot).toHaveClass("bg-zinc-300");
    expect(dot).not.toHaveClass("animate-pulse-dot");
  });
});
