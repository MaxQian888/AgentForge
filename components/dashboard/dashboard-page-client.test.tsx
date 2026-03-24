import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DashboardPageClient } from "./dashboard-page-client";
import type { DashboardSummary } from "@/lib/dashboard/summary";

const fetchSummary = jest.fn();
const dashboardState = {
  summary: {
    scope: {
      projectId: "project-1",
      projectName: "AgentForge",
      projectsCount: 1,
    },
    headline: {
      activeAgents: 1,
      tasksInProgress: 2,
      pendingReviews: 1,
      weeklyCost: 12.5,
    },
    progress: {
      total: 3,
      inbox: 0,
      triaged: 0,
      assigned: 1,
      inProgress: 2,
      inReview: 1,
      done: 0,
    },
    team: {
      totalMembers: 2,
      activeHumans: 1,
      activeAgents: 1,
      activeAgentRuns: 1,
      overloadedMembers: 0,
    },
    activity: [],
    risks: [],
    links: {
      projects: "/projects",
      team: "/team?project=project-1",
      agents: "/agents",
      reviews: "/agents?focus=review",
    },
  } satisfies DashboardSummary,
  loading: false,
  error: null,
  sectionErrors: {},
  fetchSummary,
};

jest.mock("next/navigation", () => ({
  useSearchParams: () => ({
    get: (key: string) => (key === "project" ? "project-1" : null),
  }),
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (state: typeof dashboardState) => unknown) =>
    selector(dashboardState),
}));

jest.mock("./dashboard-overview", () => ({
  DashboardOverview: ({
    summary,
    onRetry,
  }: {
    summary: DashboardSummary;
    onRetry: (section?: string) => void;
  }) => (
    <div>
      <span>{summary.scope.projectName}</span>
      <button type="button" onClick={() => onRetry("activity")}>
        Retry Activity
      </button>
    </div>
  ),
}));

describe("DashboardPageClient", () => {
  beforeEach(() => {
    fetchSummary.mockReset();
  });

  it("loads the selected project scope and retries partial sections with the same scope", async () => {
    const user = userEvent.setup();

    await act(async () => {
      render(<DashboardPageClient />);
    });

    expect(fetchSummary).toHaveBeenCalledWith({ projectId: "project-1" });
    expect(screen.getByText("AgentForge")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Retry Activity" }));

    expect(fetchSummary).toHaveBeenLastCalledWith({ projectId: "project-1" });
  });
});
