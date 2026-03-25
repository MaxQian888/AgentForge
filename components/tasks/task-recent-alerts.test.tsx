import { render, screen } from "@testing-library/react";
import { TaskRecentAlerts } from "./task-recent-alerts";
import type { Notification } from "@/lib/stores/notification-store";

const alerts: Notification[] = [
  {
    id: "alert-1",
    type: "task_progress_alerted",
    title: "Task stalled",
    message: "Calendar polish has stalled.",
    href: "/project?id=project-1#task-task-1",
    read: false,
    createdAt: "2026-03-25T08:00:00.000Z",
  },
  {
    id: "alert-2",
    type: "task_progress_recovered",
    title: "Task recovered",
    message: "Calendar polish is healthy again.",
    href: null,
    read: false,
    createdAt: "2026-03-25T08:10:00.000Z",
  },
];

describe("TaskRecentAlerts", () => {
  it("shows an empty-state message when there are no alerts", () => {
    render(<TaskRecentAlerts alerts={[]} />);

    expect(screen.getByText("No recent task alerts.")).toBeInTheDocument();
  });

  it("renders linked and non-linked alerts", () => {
    render(<TaskRecentAlerts alerts={alerts} />);

    expect(
      screen.getByRole("link", { name: "Task stalled" }),
    ).toHaveAttribute("href", "/project?id=project-1#task-task-1");
    expect(screen.getByText("Calendar polish has stalled.")).toBeInTheDocument();
    expect(screen.getByText("Task recovered")).toBeInTheDocument();
    expect(
      screen.getByText("Calendar polish is healthy again."),
    ).toBeInTheDocument();
  });
});
