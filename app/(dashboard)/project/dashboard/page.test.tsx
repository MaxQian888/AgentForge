import { render, screen } from "@testing-library/react";
import ProjectDashboardPage from "./page";
import { useDashboardStore } from "@/lib/stores/dashboard-store";

jest.mock("@/components/dashboard/dashboard-grid", () => ({
  DashboardGrid: ({ dashboard }: { dashboard: { name: string } }) => <div>{dashboard.name}</div>,
}));

const fetchDashboards = jest.fn().mockResolvedValue(undefined);
const createDashboard = jest.fn().mockResolvedValue(undefined);

describe("ProjectDashboardPage", () => {
  beforeEach(() => {
    fetchDashboards.mockClear();
    createDashboard.mockClear();
    useDashboardStore.setState({
      selectedProjectId: "project-1",
      dashboardsByProject: {},
      fetchDashboards,
      createDashboard,
    });
  });

  it("renders the create dashboard action when the selected project has no dashboards yet", () => {
    render(<ProjectDashboardPage />);

    expect(screen.getByRole("button", { name: "Create Dashboard" })).toBeInTheDocument();
    expect(fetchDashboards).toHaveBeenCalledWith("project-1");
  });
});
