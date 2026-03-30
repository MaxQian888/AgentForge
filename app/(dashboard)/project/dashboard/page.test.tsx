import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ProjectDashboardPage from "./page";
import { useDashboardStore } from "@/lib/stores/dashboard-store";

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const translations: Record<string, string> = {
      "projectDashboard.selectProject": "Select a project first.",
      "projectDashboard.createDashboard": "Create Dashboard",
      "projectDashboard.sprintOverview": "Sprint Overview",
      "projectDashboard.selectorLabel": "Dashboard",
      "projectDashboard.rename": "Rename Dashboard",
      "projectDashboard.delete": "Delete Dashboard",
      "projectDashboard.nameLabel": "Dashboard Name",
      "projectDashboard.saveName": "Save Name",
      "projectDashboard.cancelEdit": "Cancel",
      "projectDashboard.loading": "Loading dashboard workspace...",
      "projectDashboard.error": "Dashboard workspace unavailable",
      "projectDashboard.retry": "Retry Dashboard Workspace",
    };

    return translations[key] ?? key;
  },
}));

jest.mock("@/components/dashboard/dashboard-grid", () => ({
  DashboardGrid: ({ dashboard }: { dashboard: { name: string } }) => <div>{dashboard.name}</div>,
}));

const searchParamsState = {
  dashboard: null as string | null,
};

const replace = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
  usePathname: () => "/project/dashboard",
  useSearchParams: () => ({
    get: (key: string) => (key === "dashboard" ? searchParamsState.dashboard : null),
    toString: () =>
      searchParamsState.dashboard
        ? `dashboard=${encodeURIComponent(searchParamsState.dashboard)}`
        : "",
  }),
}));

const fetchDashboards = jest.fn().mockResolvedValue(undefined);
const createDashboard = jest.fn().mockResolvedValue(undefined);
const updateDashboard = jest.fn().mockResolvedValue(undefined);
const deleteDashboard = jest.fn().mockResolvedValue(undefined);

describe("ProjectDashboardPage", () => {
  beforeEach(() => {
    searchParamsState.dashboard = null;
    replace.mockClear();
    fetchDashboards.mockClear();
    createDashboard.mockClear();
    updateDashboard.mockClear();
    deleteDashboard.mockClear();
    useDashboardStore.setState({
      selectedProjectId: "project-1",
      activeDashboardIdByProject: {},
      dashboardsLoadingByProject: {},
      dashboardsErrorByProject: {},
      dashboardsByProject: {},
      fetchDashboards,
      createDashboard,
      updateDashboard,
      deleteDashboard,
    });
  });

  it("renders the create dashboard action when the selected project has no dashboards yet", () => {
    render(<ProjectDashboardPage />);

    expect(screen.getByRole("button", { name: "Create Dashboard" })).toBeInTheDocument();
    expect(fetchDashboards).toHaveBeenCalledWith("project-1");
  });

  it("renders the dashboard selected by the dashboard query param", () => {
    searchParamsState.dashboard = "dashboard-2";
    useDashboardStore.setState({
      dashboardsByProject: {
        "project-1": [
          {
            id: "dashboard-1",
            projectId: "project-1",
            name: "Sprint Overview",
            layout: [],
            createdBy: "user-1",
            createdAt: "2026-03-28T00:00:00.000Z",
            updatedAt: "2026-03-28T00:00:00.000Z",
          },
          {
            id: "dashboard-2",
            projectId: "project-1",
            name: "Review Watch",
            layout: [],
            createdBy: "user-1",
            createdAt: "2026-03-28T00:00:00.000Z",
            updatedAt: "2026-03-28T00:00:00.000Z",
          },
        ],
      },
    });

    render(<ProjectDashboardPage />);

    expect(screen.getByLabelText("Dashboard")).toHaveValue("dashboard-2");
    expect(screen.getAllByText("Review Watch").length).toBeGreaterThan(0);
  });

  it("writes the first accessible dashboard into the route when no dashboard query is present", () => {
    useDashboardStore.setState({
      dashboardsByProject: {
        "project-1": [
          {
            id: "dashboard-1",
            projectId: "project-1",
            name: "Sprint Overview",
            layout: [],
            createdBy: "user-1",
            createdAt: "2026-03-28T00:00:00.000Z",
            updatedAt: "2026-03-28T00:00:00.000Z",
          },
        ],
      },
    });

    render(<ProjectDashboardPage />);

    expect(replace).toHaveBeenCalledWith("/project/dashboard?dashboard=dashboard-1");
  });

  it("creates a dashboard from the empty state", async () => {
    const user = userEvent.setup();

    render(<ProjectDashboardPage />);
    await user.click(screen.getByRole("button", { name: "Create Dashboard" }));

    expect(createDashboard).toHaveBeenCalledWith("project-1", {
      name: "Sprint Overview",
      layout: [],
    });
  });

  it("lets the user switch dashboards from the workspace toolbar", async () => {
    const user = userEvent.setup();

    useDashboardStore.setState({
      dashboardsByProject: {
        "project-1": [
          {
            id: "dashboard-1",
            projectId: "project-1",
            name: "Sprint Overview",
            layout: [],
            createdBy: "user-1",
            createdAt: "2026-03-28T00:00:00.000Z",
            updatedAt: "2026-03-28T00:00:00.000Z",
          },
          {
            id: "dashboard-2",
            projectId: "project-1",
            name: "Review Watch",
            layout: [],
            createdBy: "user-1",
            createdAt: "2026-03-28T00:00:00.000Z",
            updatedAt: "2026-03-28T00:00:00.000Z",
          },
        ],
      },
    });

    render(<ProjectDashboardPage />);
    replace.mockClear();

    await user.selectOptions(screen.getByLabelText("Dashboard"), "dashboard-2");

    expect(replace).toHaveBeenLastCalledWith("/project/dashboard?dashboard=dashboard-2");
  });

  it("renames the active dashboard from the workspace toolbar", async () => {
    const user = userEvent.setup();

    useDashboardStore.setState({
      dashboardsByProject: {
        "project-1": [
          {
            id: "dashboard-1",
            projectId: "project-1",
            name: "Sprint Overview",
            layout: [],
            createdBy: "user-1",
            createdAt: "2026-03-28T00:00:00.000Z",
            updatedAt: "2026-03-28T00:00:00.000Z",
          },
        ],
      },
    });

    render(<ProjectDashboardPage />);
    await user.click(screen.getByRole("button", { name: "Rename Dashboard" }));
    await user.clear(screen.getByLabelText("Dashboard Name"));
    await user.type(screen.getByLabelText("Dashboard Name"), "Exec View");
    await user.click(screen.getByRole("button", { name: "Save Name" }));

    expect(updateDashboard).toHaveBeenCalledWith("project-1", "dashboard-1", {
      name: "Exec View",
    });
  });

  it("deletes the active dashboard from the workspace toolbar", async () => {
    const user = userEvent.setup();

    useDashboardStore.setState({
      dashboardsByProject: {
        "project-1": [
          {
            id: "dashboard-1",
            projectId: "project-1",
            name: "Sprint Overview",
            layout: [],
            createdBy: "user-1",
            createdAt: "2026-03-28T00:00:00.000Z",
            updatedAt: "2026-03-28T00:00:00.000Z",
          },
        ],
      },
    });

    render(<ProjectDashboardPage />);
    await user.click(screen.getByRole("button", { name: "Delete Dashboard" }));

    expect(deleteDashboard).toHaveBeenCalledWith("project-1", "dashboard-1");
  });

  it("renders a loading state while dashboards are being fetched", () => {
    useDashboardStore.setState({
      dashboardsLoadingByProject: { "project-1": true },
    });

    render(<ProjectDashboardPage />);

    expect(
      screen.getByText("Loading dashboard workspace...")
    ).toBeInTheDocument();
  });

  it("renders a retryable error state when dashboards cannot be loaded", async () => {
    const user = userEvent.setup();

    useDashboardStore.setState({
      dashboardsErrorByProject: { "project-1": "dashboard endpoint unavailable" },
    });

    render(<ProjectDashboardPage />);

    expect(
      screen.getByText("Dashboard workspace unavailable: dashboard endpoint unavailable")
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Retry" }));

    expect(fetchDashboards).toHaveBeenLastCalledWith("project-1");
  });
});
