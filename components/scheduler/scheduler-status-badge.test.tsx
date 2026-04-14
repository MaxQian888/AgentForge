import { render, screen } from "@testing-library/react";
import { SchedulerStatusBadge } from "./scheduler-status-badge";

describe("SchedulerStatusBadge", () => {
  it("renders known statuses with the mapped styling", () => {
    render(<SchedulerStatusBadge status="failed" className="custom-badge" />);

    const badge = screen.getByText("failed");
    expect(badge).toHaveClass("text-red-700");
    expect(badge).toHaveClass("custom-badge");
  });

  it("supports paused and cancelled lifecycle states", () => {
    const { rerender } = render(<SchedulerStatusBadge status="paused" />);
    expect(screen.getByText("paused")).toHaveClass("bg-muted");

    rerender(<SchedulerStatusBadge status="cancelled" />);
    expect(screen.getByText("cancelled")).toHaveClass("text-orange-700");
  });

  it("falls back to never-run when the status is missing or unknown", () => {
    const { rerender } = render(<SchedulerStatusBadge status={undefined} />);
    expect(screen.getByText("never-run")).toHaveClass("bg-muted");

    rerender(<SchedulerStatusBadge status="mystery" />);
    expect(screen.getByText("mystery")).toHaveClass("bg-muted");
  });
});
