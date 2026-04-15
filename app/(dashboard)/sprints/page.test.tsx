import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import SprintsPage from "./page";

const push = jest.fn();
const dashboardState = {
  selectedProjectId: null as string | null,
  fetchSummary: jest.fn().mockResolvedValue(undefined),
};
const searchParamsState = {
  project: null as string | null,
  action: null as string | null,
};

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
  useSearchParams: () => ({
    get: (key: string) =>
      key === "project"
        ? searchParamsState.project
        : key === "action"
          ? searchParamsState.action
          : null,
  }),
}));

const sprintStoreState = {
  sprintsByProject: {} as Record<string, Array<Record<string, unknown>>>,
  loadingByProject: {} as Record<string, boolean>,
  metricsBySprintId: {} as Record<string, Record<string, unknown>>,
  budgetDetailBySprintId: {} as Record<string, Record<string, unknown>>,
  budgetLoadingBySprintId: {} as Record<string, boolean>,
  budgetErrorBySprintId: {} as Record<string, string | null>,
  fetchSprints: jest.fn(),
  fetchSprintMetrics: jest.fn(),
  fetchSprintBudgetDetail: jest.fn(),
  createSprint: jest.fn().mockResolvedValue({ id: "sprint-created" }),
  updateSprint: jest.fn().mockResolvedValue({ id: "sprint-1" }),
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

jest.mock("@/components/ui/select", () => {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const React = require("react");

  function flattenOptions(children: React.ReactNode): Array<{ value: string; label: string }> {
    const options: Array<{ value: string; label: string }> = [];
    function visit(node: React.ReactNode) {
      React.Children.forEach(node, (child: unknown) => {
        if (!React.isValidElement(child)) return;
        const element = child as React.ReactElement<{ children?: React.ReactNode; value?: string }>;
        if (element.props.value !== undefined) {
          options.push({
            value: element.props.value,
            label: typeof element.props.children === "string" ? element.props.children : String(element.props.value),
          });
          return;
        }
        visit(element.props.children);
      });
    }
    visit(children);
    return options;
  }

  return {
    Select: ({ value, onValueChange, children }: { value: string; onValueChange: (v: string) => void; children: React.ReactNode }) => {
      const options = flattenOptions(children);
      return (
        <select value={value} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => onValueChange(e.target.value)}>
          {options.map((item: { value: string; label: string }) => (
            <option key={item.value} value={item.value}>{item.label}</option>
          ))}
        </select>
      );
    },
    SelectTrigger: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    SelectValue: () => null,
    SelectContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    SelectItem: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  };
});

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector?: (state: typeof dashboardState) => unknown) =>
    selector ? selector(dashboardState) : dashboardState,
}));

jest.mock("@/lib/stores/sprint-store", () => ({
  useSprintStore: (selector?: (state: typeof sprintStoreState) => unknown) =>
    selector ? selector(sprintStoreState) : sprintStoreState,
  normalizeSprintDateInput: (value: string) =>
    /^\d{4}-\d{2}-\d{2}$/.test(value) ? `${value}T00:00:00.000Z` : value,
}));

jest.mock("@/lib/stores/milestone-store", () => ({
  useMilestoneStore: (selector: (state: typeof milestoneStoreState) => unknown) =>
    selector(milestoneStoreState),
}));

describe("SprintsPage", () => {
  beforeEach(() => {
    push.mockReset();
    dashboardState.selectedProjectId = null;
    dashboardState.fetchSummary.mockReset().mockResolvedValue(undefined);
    searchParamsState.project = null;
    searchParamsState.action = null;
    sprintStoreState.sprintsByProject = {};
    sprintStoreState.loadingByProject = {};
    sprintStoreState.metricsBySprintId = {};
    sprintStoreState.budgetDetailBySprintId = {};
    sprintStoreState.budgetLoadingBySprintId = {};
    sprintStoreState.budgetErrorBySprintId = {};
    sprintStoreState.fetchSprints.mockReset();
    sprintStoreState.fetchSprintMetrics.mockReset();
    sprintStoreState.fetchSprintBudgetDetail.mockReset();
    sprintStoreState.createSprint.mockReset().mockResolvedValue({ id: "sprint-created" });
    sprintStoreState.updateSprint.mockReset().mockResolvedValue({ id: "sprint-1" });
    milestoneStoreState.milestonesByProject = {};
    milestoneStoreState.fetchMilestones.mockReset();
  });

  it("asks the user to select a project before managing sprints", () => {
    render(<SprintsPage />);

    expect(screen.getByRole("heading", { name: "sprints.title" })).toBeInTheDocument();
    expect(screen.getByText("sprints.selectProjectPrompt")).toBeInTheDocument();
    expect(sprintStoreState.fetchSprints).not.toHaveBeenCalled();
  });

  it("uses explicit project scope and opens the create dialog for bootstrap handoff actions", async () => {
    searchParamsState.project = "project-1";
    searchParamsState.action = "create-sprint";

    render(<SprintsPage />);

    await waitFor(() => {
      expect(sprintStoreState.fetchSprints).toHaveBeenCalledWith("project-1");
    });
    expect(screen.getByRole("heading", { name: "sprints.dialog.createTitle" })).toBeInTheDocument();
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
    milestoneStoreState.milestonesByProject = {
      "project-1": [{ id: "milestone-1", name: "GA Release" }],
    };

    render(<SprintsPage />);

    await user.click(screen.getByRole("button", { name: "sprints.newSprint" }));
    const [nameInput, startInput, endInput, budgetInput] = Array.from(
      document.querySelectorAll("input"),
    ) as HTMLInputElement[];
    const [milestoneSelect] = Array.from(document.querySelectorAll("select")) as HTMLSelectElement[];
    fireEvent.change(nameInput, { target: { value: "Sprint 12" } });
    fireEvent.change(startInput, { target: { value: "2026-04-01" } });
    fireEvent.change(endInput, { target: { value: "2026-04-14" } });
    fireEvent.change(budgetInput, { target: { value: "125" } });
    fireEvent.change(milestoneSelect, { target: { value: "milestone-1" } });
    await user.click(screen.getByRole("button", { name: "sprints.dialog.create" }));

    await waitFor(() => {
      expect(sprintStoreState.createSprint).toHaveBeenCalledWith("project-1", {
        name: "Sprint 12",
        startDate: "2026-04-01T00:00:00.000Z",
        endDate: "2026-04-14T00:00:00.000Z",
        totalBudgetUsd: 125,
        milestoneId: "milestone-1",
      });
    });
  });

  it("loads metrics and budget detail for the active sprint and renders the selected sprint detail", async () => {
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
        completionRate: 40,
        velocityPerWeek: 2.5,
        taskSpentUsd: 120,
        taskBudgetUsd: 500,
        burndown: [],
      },
    };
    sprintStoreState.budgetDetailBySprintId = {
      "sprint-1": {
        sprintId: "sprint-1",
        sprintName: "Launch Sprint",
        allocated: 500,
        spent: 120,
        remaining: 380,
        thresholdStatus: "warning",
        tasks: [
          {
            taskId: "task-1",
            title: "Polish burndown",
            allocated: 200,
            spent: 120,
            remaining: 80,
            thresholdStatus: "warning",
          },
        ],
      },
    };

    render(<SprintsPage />);

    await waitFor(() => {
      expect(sprintStoreState.fetchSprintMetrics).toHaveBeenCalledWith("project-1", "sprint-1");
    });
    expect(sprintStoreState.fetchSprintBudgetDetail).toHaveBeenCalledWith("sprint-1");
    expect(screen.getByTestId("burndown-chart")).toHaveTextContent("10");
    expect(screen.getByText("sprints.burndown.title — Launch Sprint")).toBeInTheDocument();
    expect(screen.getByText("Polish burndown")).toBeInTheDocument();
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
        startDate: "2026-04-02T00:00:00.000Z",
        endDate: "2026-04-16T00:00:00.000Z",
        status: "active",
        totalBudgetUsd: 250,
        milestoneId: "milestone-1",
      });
    });
  });

  it("shows inline save errors instead of silently closing the create dialog", async () => {
    const user = userEvent.setup();
    dashboardState.selectedProjectId = "project-1";
    sprintStoreState.createSprint.mockRejectedValueOnce(new Error("invalid milestone scope"));

    render(<SprintsPage />);

    await user.click(screen.getByRole("button", { name: "sprints.newSprint" }));
    const [nameInput, startInput, endInput] = Array.from(
      document.querySelectorAll("input"),
    ) as HTMLInputElement[];
    fireEvent.change(nameInput, { target: { value: "Sprint 12" } });
    fireEvent.change(startInput, { target: { value: "2026-04-01" } });
    fireEvent.change(endInput, { target: { value: "2026-04-14" } });
    await user.click(screen.getByRole("button", { name: "sprints.dialog.create" }));

    expect(await screen.findByText("invalid milestone scope")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "sprints.dialog.createTitle" })).toBeInTheDocument();
  });

  it("opens sprint-scoped execution work from the selected sprint detail", async () => {
    const user = userEvent.setup();
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
        completionRate: 40,
        velocityPerWeek: 2.5,
        taskSpentUsd: 120,
        taskBudgetUsd: 500,
        burndown: [],
      },
    };
    sprintStoreState.budgetDetailBySprintId = {
      "sprint-1": {
        sprintId: "sprint-1",
        sprintName: "Launch Sprint",
        allocated: 500,
        spent: 120,
        remaining: 380,
        thresholdStatus: "warning",
        tasks: [],
      },
    };

    render(<SprintsPage />);

    await user.click(screen.getByText("Launch Sprint"));
    await user.click(screen.getByRole("button", { name: "sprints.actions.openTasks" }));

    expect(push).toHaveBeenCalledWith("/project?id=project-1&sprint=sprint-1");
  });
});
