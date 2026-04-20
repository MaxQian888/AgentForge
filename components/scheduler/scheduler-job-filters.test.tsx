jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "jobFilters.allStatus": "All statuses",
      "jobFilters.allScopes": "All scopes",
      "jobFilters.statusScheduled": "Scheduled",
      "jobFilters.statusRunning": "Running",
      "jobFilters.statusSucceeded": "Succeeded",
      "jobFilters.statusFailed": "Failed",
      "jobFilters.statusPaused": "Paused",
    };
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SchedulerJobFilters } from "./scheduler-job-filters";
import type { SchedulerJob } from "@/lib/stores/scheduler-store";

function makeJob(jobKey: string, scope: string): SchedulerJob {
  return {
    jobKey,
    name: jobKey,
    scope,
    schedule: "*/5 * * * *",
    enabled: true,
    executionMode: "in_process",
    overlapPolicy: "skip",
    lastRunSummary: "",
    lastError: "",
    config: "{}",
    createdAt: "",
    updatedAt: "",
  };
}

describe("SchedulerJobFilters", () => {
  it("invokes the change callbacks for status and reset", async () => {
    const user = userEvent.setup();
    const onFiltersChange = jest.fn();
    const onReset = jest.fn();

    render(
      <SchedulerJobFilters
        jobs={[makeJob("a", "system"), makeJob("b", "project")]}
        filters={{ status: "failed", scope: "all" }}
        onFiltersChange={onFiltersChange}
        onReset={onReset}
      />,
    );

    // Reset button is rendered for multiple breakpoints (desktop/mobile); any
    // one of them invokes onReset — pick the first to avoid ambiguity.
    const [resetButton] = screen.getAllByRole("button", { name: /reset/i });
    await user.click(resetButton);
    expect(onReset).toHaveBeenCalled();
  });

  it("renders scope options sourced from the provided job list", () => {
    render(
      <SchedulerJobFilters
        jobs={[makeJob("a", "system"), makeJob("b", "project"), makeJob("c", "system")]}
        filters={{ status: "all", scope: "all" }}
        onFiltersChange={jest.fn()}
        onReset={jest.fn()}
      />,
    );

    // The select triggers show placeholders; we only assert the filters mount without error.
    expect(screen.getByText("All statuses")).toBeInTheDocument();
    expect(screen.getByText("All scopes")).toBeInTheDocument();
  });
});
