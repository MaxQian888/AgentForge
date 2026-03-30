jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "runHistory.colStatus": "Status",
      "runHistory.colTrigger": "Trigger",
      "runHistory.colStarted": "Started",
      "runHistory.colDuration": "Duration",
      "runHistory.colSummary": "Summary",
      "runHistory.colMetrics": "Metrics",
      "runHistory.noRuns": "No runs yet.",
    };
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import { SchedulerRunHistory } from "./scheduler-run-history";

describe("SchedulerRunHistory", () => {
  beforeEach(() => {
    jest.spyOn(Date, "now").mockReturnValue(
      new Date("2026-03-30T12:00:00.000Z").getTime(),
    );
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("renders an explicit empty state when there is no run history", () => {
    render(<SchedulerRunHistory runs={[]} />);

    expect(screen.getByText("No runs yet.")).toBeInTheDocument();
  });

  it("renders run rows, metrics, summaries, and error messages", () => {
    render(
      <SchedulerRunHistory
        runs={[
          {
            runId: "run-1",
            jobKey: "scheduler.cleanup",
            triggerSource: "manual",
            status: "failed",
            startedAt: "2026-03-30T11:50:00.000Z",
            durationMs: 61_000,
            summary: "",
            errorMessage: "Bridge unavailable",
            metrics: '{"queued":2,"processed":1}',
            createdAt: "",
            updatedAt: "",
          },
          {
            runId: "run-2",
            jobKey: "scheduler.cleanup",
            triggerSource: "system",
            status: "succeeded",
            startedAt: "2026-03-30T11:40:00.000Z",
            durationMs: 500,
            summary: "Completed maintenance",
            errorMessage: "",
            metrics: "{}",
            createdAt: "",
            updatedAt: "",
          },
        ]}
      />,
    );

    expect(screen.getByText("manual")).toBeInTheDocument();
    expect(screen.getByText("system")).toBeInTheDocument();
    expect(screen.getByText("1m 1s")).toBeInTheDocument();
    expect(screen.getByText("500ms")).toBeInTheDocument();
    expect(screen.getByText("Bridge unavailable")).toBeInTheDocument();
    expect(screen.getByText("Completed maintenance")).toBeInTheDocument();
    expect(screen.getByText("queued: 2")).toBeInTheDocument();
    expect(screen.getByText("processed: 1")).toBeInTheDocument();
  });
});
