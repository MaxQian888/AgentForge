import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import SprintsPage from "./page";

const dashboardState = {
  selectedProjectId: null as string | null,
};

const sprintStoreState = {
  sprintsByProject: {} as Record<string, Array<Record<string, unknown>>>,
  loadingByProject: {} as Record<string, boolean>,
  metricsBySprintId: {} as Record<string, Record<string, unknown>>,
  fetchSprints: jest.fn(),
  fetchSprintMetrics: jest.fn(),
  createSprint: jest.fn().mockResolvedValue(undefined),
  updateSprint: jest.fn().mockResolvedValue(undefined),
};

const milestoneStoreState = {
  milestonesByProject: {} as Record<string, Array<{ id: string; name: string }>>,
  fetchMilestones: jest.fn(),
};

jest.mock("next-intl", () => ({
  useTranslations: (namespace?: string) => (key: string) =>
    namespace ? `${namespace}.${key}` : key,
}));

jest.mock("@/hooks/use-breadcrumbs", () => ({
  useBreadcrumbs: jest.fn(),
}));

jest.mock("@/components/shared/page-header", () => ({
  PageHeader: ({
    title,
    actions,
  }: {
    title: string;
    actions?: React.ReactNode;
  }) => (
    <div>
      <h1>{title}</h1>
      {actions}
    </div>
  ),
}));

jest.mock("@/components/shared/empty-state", () => ({
  EmptyState: ({ title }: { title: string }) => <div data-testid="empty-state">{title}</div>,
}));

jest.mock("@/components/sprint/burndown-chart", () => ({
  BurndownChart: ({ plannedTasks }: { plannedTasks: number }) => (
    <div data-testid="burndown-chart">{plannedTasks}</div>
  ),
}));

jest.mock("@/components/milestones/milestone-editor", () => ({
  MilestoneEditor: ({
    open,
    projectId,
  }: {
    open: boolean;
    projectId: string;
  }) => (open ? <div data-testid="milestone-editor">{projectId}</div> : null),
}));

jest.mock("@/components/ui/dialog", () => ({
  Dialog: ({
    open,
    children,
  }: {
    open: boolean;
    children: React.ReactNode;
  }) => (open ? <div>{children}</div> : null),
  DialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>,
  DialogFooter: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: () => dashboardState,
}));

jest.mock("@/lib/stores/sprint-store", () => ({
  useSprintStore: () => sprintStoreState,
}));

jest.mock("@/lib/stores/milestone-store", () => ({
  useMilestoneStore: (selector: (state: typeof milestoneStoreState) => unknown) =>
    selector(milestoneStoreState),
}));

describe("SprintsPage", () => {
  beforeEach(() => {
    dashboardState.selectedProjectId = null;
    sprintStoreState.sprintsByProject = {};
    sprintStoreState.loadingByProject = {};
    sprintStoreState.metricsBySprintId = {};
    sprintStoreState.fetchSprints.mockReset();
    sprintStoreState.fetchSprintMetrics.mockReset();
    sprintStoreState.createSprint.mockReset().mockResolvedValue(undefined);
    sprintStoreState.updateSprint.mockReset().mockResolvedValue(undefined);
    milestoneStoreState.milestonesByProject = {};
    milestoneStoreState.fetchMilestones.mockReset();
  });

  it("asks the user to select a project before managing sprints", () => {
    render(<SprintsPage />);

    expect(screen.getByRole("heading", { name: "sprints.title" })).toBeInTheDocument();
    expect(screen.getByText("sprints.selectProjectPrompt")).toBeInTheDocument();
    expect(sprintStoreState.fetchSprints).not.toHaveBeenCalled();
  });

  it("loads sprint data for the selected project and shows an empty state when none exist", async () => {
    dashboardState.selectedProjectId = "project-1";

    render(<SprintsPage />);

    await waitFor(() => {
      expect(sprintStoreState.fetchSprints).toHaveBeenCalledWith("project-1");
    });
    expect(milestoneStoreState.fetchMilestones).toHaveBeenCalledWith("project-1");
    expect(screen.getByTestId("empty-state")).toHaveTextContent("sprints.empty.noSprints");
  });

  it("creates a sprint from the page dialog", async () => {
    const user = userEvent.setup();
    dashboardState.selectedProjectId = "project-1";

    render(<SprintsPage />);

    await user.click(screen.getByRole("button", { name: "sprints.newSprint" }));
    const [nameInput, startInput, endInput, budgetInput] = Array.from(
      document.querySelectorAll("input"),
    ) as HTMLInputElement[];
    fireEvent.change(nameInput, { target: { value: "Sprint 12" } });
    fireEvent.change(startInput, { target: { value: "2026-04-01" } });
    fireEvent.change(endInput, { target: { value: "2026-04-14" } });
    fireEvent.change(budgetInput, { target: { value: "125" } });
    await user.click(screen.getByRole("button", { name: "sprints.dialog.create" }));

    await waitFor(() => {
      expect(sprintStoreState.createSprint).toHaveBeenCalledWith("project-1", {
        name: "Sprint 12",
        startDate: "2026-04-01",
        endDate: "2026-04-14",
        totalBudgetUsd: 125,
      });
    });
  });

  it("loads metrics for the active sprint and renders the burndown chart", async () => {
    dashboardState.selectedProjectId = "project-1";
    sprintStoreState.sprintsByProject = {
      "project-1": [
        {
          id: "sprint-1",
          name: "Launch Sprint",
          status: "active",
          startDate: "2026-04-01T00:00:00.000Z",
          endDate: "2026-04-14T00:00:00.000Z",
          totalBudgetUsd: 500,
          spentUsd: 120,
          milestoneId: null,
        },
      ],
    };
    sprintStoreState.metricsBySprintId = {
      "sprint-1": {
        sprint: { name: "Launch Sprint" },
        completedTasks: 4,
        plannedTasks: 10,
        completionRate: 0.4,
        velocityPerWeek: 2.5,
        taskSpentUsd: 120,
        taskBudgetUsd: 500,
        burndown: [],
      },
    };

    render(<SprintsPage />);

    await waitFor(() => {
      expect(sprintStoreState.fetchSprintMetrics).toHaveBeenCalledWith("project-1", "sprint-1");
    });
    expect(screen.getByTestId("burndown-chart")).toHaveTextContent("10");
    expect(screen.getByText("sprints.burndown.title — Launch Sprint")).toBeInTheDocument();
  });

  it("opens the milestone editor and saves sprint edits", async () => {
    const user = userEvent.setup();
    dashboardState.selectedProjectId = "project-1";
    sprintStoreState.sprintsByProject = {
      "project-1": [
        {
          id: "sprint-1",
          name: "Launch Sprint",
          status: "planning",
          startDate: "2026-04-01T00:00:00.000Z",
          endDate: "2026-04-14T00:00:00.000Z",
          totalBudgetUsd: 500,
          spentUsd: 120,
          milestoneId: null,
        },
      ],
    };
    milestoneStoreState.milestonesByProject = {
      "project-1": [{ id: "milestone-1", name: "GA Release" }],
    };

    render(<SprintsPage />);

    await user.click(screen.getByRole("button", { name: "sprints.newMilestone" }));
    expect(screen.getByTestId("milestone-editor")).toHaveTextContent("project-1");

    await user.click(screen.getByRole("button", { name: "sprints.card.edit" }));

    const [nameInput, startInput, endInput, budgetInput] = Array.from(
      document.querySelectorAll("input"),
    ) as HTMLInputElement[];
    const [milestoneSelect, statusSelect] = Array.from(
      document.querySelectorAll("select"),
    ) as HTMLSelectElement[];

    fireEvent.change(nameInput, { target: { value: "Release Sprint" } });
    fireEvent.change(startInput, { target: { value: "2026-04-02" } });
    fireEvent.change(endInput, { target: { value: "2026-04-16" } });
    fireEvent.change(budgetInput, { target: { value: "250" } });
    fireEvent.change(milestoneSelect, { target: { value: "milestone-1" } });
    fireEvent.change(statusSelect, { target: { value: "active" } });

    await user.click(screen.getByRole("button", { name: "sprints.dialog.save" }));

    await waitFor(() => {
      expect(sprintStoreState.updateSprint).toHaveBeenCalledWith("project-1", "sprint-1", {
        name: "Release Sprint",
        startDate: "2026-04-02",
        endDate: "2026-04-16",
        status: "active",
        totalBudgetUsd: 250,
        milestoneId: "milestone-1",
      });
    });
  });
});
