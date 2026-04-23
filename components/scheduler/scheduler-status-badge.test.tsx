jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "statusLabels.succeeded": "Succeeded",
      "statusLabels.failed": "Failed",
      "statusLabels.running": "Running",
      "statusLabels.cancel_requested": "Cancel Requested",
      "statusLabels.cancelled": "Cancelled",
      "statusLabels.pending": "Pending",
      "statusLabels.skipped": "Skipped",
      "statusLabels.paused": "Paused",
      "statusLabels.disabled": "Disabled",
      "statusLabels.never-run": "Never Run",
    };
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import { SchedulerStatusBadge } from "./scheduler-status-badge";

describe("SchedulerStatusBadge", () => {
  it("renders known statuses with the mapped styling", () => {
    render(<SchedulerStatusBadge status="failed" className="custom-badge" />);

    const badge = screen.getByText("Failed");
    expect(badge).toHaveClass("text-red-700");
    expect(badge).toHaveClass("custom-badge");
  });

  it("supports paused and cancelled lifecycle states", () => {
    const { rerender } = render(<SchedulerStatusBadge status="paused" />);
    expect(screen.getByText("Paused")).toHaveClass("bg-muted");

    rerender(<SchedulerStatusBadge status="cancelled" />);
    expect(screen.getByText("Cancelled")).toHaveClass("text-orange-700");
  });

  it("falls back to never-run when the status is missing or unknown", () => {
    const { rerender } = render(<SchedulerStatusBadge status={undefined} />);
    expect(screen.getByText("Never Run")).toHaveClass("bg-muted");

    rerender(<SchedulerStatusBadge status="mystery" />);
    expect(screen.getByText("statusLabels.mystery")).toHaveClass("bg-muted");
  });
});
