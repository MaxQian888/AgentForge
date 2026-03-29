import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import TeamsPage from "./page";

const replace = jest.fn();
const fetchTeams = jest.fn();
const searchParamsState = {
  project: "project-1",
};

const dashboardState = {
  projects: [
    { id: "project-1", name: "AgentForge" },
    { id: "project-2", name: "Bridge" },
  ],
  selectedProjectId: "project-2",
};

const teamState = {
  teams: [],
  loading: false,
  error: "failed to list teams",
  fetchTeams,
};

jest.mock("next/navigation", () => ({
  usePathname: () => "/teams",
  useRouter: () => ({ replace }),
  useSearchParams: () => ({
    get: (key: string) => (key === "project" ? searchParamsState.project : null),
  }),
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (state: typeof dashboardState) => unknown) => selector(dashboardState),
}));

jest.mock("@/lib/stores/team-store", () => ({
  useTeamStore: (selector?: (state: typeof teamState) => unknown) =>
    selector ? selector(teamState) : teamState,
}));

describe("TeamsPage", () => {
  beforeEach(() => {
    replace.mockReset();
    fetchTeams.mockReset();
    searchParamsState.project = "project-1";
    teamState.error = "failed to list teams";
    teamState.loading = false;
    teamState.teams = [];
  });

  it("loads team runs with an explicit project scope and exposes retry on failure", async () => {
    const user = userEvent.setup();
    render(<TeamsPage />);

    await waitFor(() => expect(fetchTeams).toHaveBeenCalledWith("project-1", undefined));
    expect(screen.getByText("failed to list teams")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /retry/i }));
    expect(fetchTeams).toHaveBeenLastCalledWith("project-1");
  });
});
