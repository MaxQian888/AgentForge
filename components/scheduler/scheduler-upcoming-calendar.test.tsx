jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, unknown>) => {
    const map: Record<string, string> = {
      "calendar.noUpcoming": "No upcoming runs",
      "calendar.prevMonth": "Previous month",
      "calendar.nextMonth": "Next month",
      "calendar.today": "Today",
      "calendar.gridLabel": "Upcoming runs",
      "calendar.jobsCount": "{count} jobs",
    };
    if (key === "calendar.moreItems") {
      return `+${values?.count ?? ""} more`;
    }
    if (key === "calendar.dayDetail") {
      return `Runs on ${values?.date ?? ""}`;
    }
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SchedulerUpcomingCalendar } from "./scheduler-upcoming-calendar";
import type { SchedulerJob } from "@/lib/stores/scheduler-store";

function job(partial: Partial<SchedulerJob>): SchedulerJob {
  return {
    jobKey: "j",
    name: "Job",
    scope: "system",
    schedule: "*/5 * * * *",
    enabled: true,
    executionMode: "in_process",
    overlapPolicy: "skip",
    lastRunSummary: "",
    lastError: "",
    config: "{}",
    createdAt: "",
    updatedAt: "",
    ...partial,
  };
}

describe("SchedulerUpcomingCalendar", () => {
  beforeEach(() => {
    jest.useFakeTimers();
    jest.setSystemTime(new Date("2026-04-15T09:00:00.000Z"));
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it("renders empty state when no upcoming runs", () => {
    render(<SchedulerUpcomingCalendar jobs={[job({ upcomingRuns: [] })]} />);
    expect(screen.getByText("No upcoming runs")).toBeInTheDocument();
  });

  it("renders a month grid with weekday headers and entries", async () => {
    const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
    render(
      <SchedulerUpcomingCalendar
        jobs={[
          job({
            jobKey: "sync",
            name: "Sync",
            upcomingRuns: [
              { runAt: "2026-04-15T10:00:00.000Z" },
              { runAt: "2026-04-16T11:00:00.000Z" },
            ],
          }),
        ]}
      />,
    );

    expect(screen.getByRole("grid")).toBeInTheDocument();
    // 7 weekday headers + at least one Sync entry
    expect(screen.getAllByRole("columnheader")).toHaveLength(7);
    expect(screen.getAllByText(/Sync/).length).toBeGreaterThan(0);

    // Click Next month navigation
    await user.click(screen.getByLabelText("Next month"));
    // After navigating, the grid is still rendered.
    expect(screen.getByRole("grid")).toBeInTheDocument();
  });
});
