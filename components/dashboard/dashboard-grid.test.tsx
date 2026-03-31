import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DashboardGrid } from "./dashboard-grid";
import type {
  DashboardConfig,
  DashboardWidgetRequestState,
} from "@/lib/stores/dashboard-store";

jest.mock(
  "react-grid-layout/legacy",
  () => ({
    Responsive: ({
      children,
      onLayoutChange,
    }: {
      children: React.ReactNode;
      onLayoutChange?: (
        layout: Array<{ i: string; x: number; y: number; w: number; h: number }>
      ) => void;
    }) => (
      <div data-testid="dashboard-grid-layout">
        <button
          type="button"
          onClick={() =>
            onLayoutChange?.([{ i: "widget-1", x: 1, y: 0, w: 2, h: 1 }])
          }
        >
          Trigger Layout Change
        </button>
        {children}
      </div>
    ),
    WidthProvider: (Component: unknown) => Component,
  }),
  { virtual: true }
);

const fetchWidgetData = jest.fn().mockResolvedValue(undefined);
const saveWidget = jest.fn().mockResolvedValue(undefined);
const updateDashboard = jest.fn().mockResolvedValue(undefined);
const deleteWidget = jest.fn().mockResolvedValue(undefined);

const widgetDataKey = "project-1:throughput_chart:{}";
const widgetRequestStateByKey: Record<string, DashboardWidgetRequestState> = {
  [widgetDataKey]: { status: "success", error: null },
};
const widgetData: Record<string, Record<string, unknown>> = {
  [widgetDataKey]: {
    widgetType: "throughput_chart",
    points: [{ date: "2026-03-28", count: 3 }],
  },
  "project-1:budget_consumption:{}": {
    spent: 12,
    allocated: 20,
  },
};

const storeState = {
  widgetsByDashboard: {
    "dashboard-1": [
      {
        id: "widget-1",
        dashboardId: "dashboard-1",
        widgetType: "throughput_chart",
        config: {},
        position: { x: 0, y: 0, w: 1, h: 1 },
        createdAt: "2026-03-28T00:00:00.000Z",
        updatedAt: "2026-03-28T00:00:00.000Z",
      },
      {
        id: "widget-2",
        dashboardId: "dashboard-1",
        widgetType: "budget_consumption",
        config: {},
        position: { x: 1, y: 0, w: 1, h: 1 },
        createdAt: "2026-03-28T00:00:00.000Z",
        updatedAt: "2026-03-28T00:00:00.000Z",
      },
    ],
  },
  widgetData,
  widgetRequestStateByKey,
  fetchWidgetData,
  saveWidget,
  updateDashboard,
  deleteWidget,
};

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    const translations: Record<string, string> = {
      "widget.addWidget": "Add Widget",
      "widget.blockedTasks": "Blocked Tasks",
      "widget.budget": "Budget",
      "widget.budgetAllocated": `Allocated $${values?.amount ?? "0.00"}`,
      "widget.agentCostEntries": "Agent Cost Entries",
      "widget.reviewBacklog": "Review Backlog",
      "widget.agingBuckets": "Aging Buckets",
      "widget.slaCompliance": "SLA Compliance",
      "widget.slaTasks": `${values?.compliant ?? 0}/${values?.total ?? 0} tasks`,
      "widget.layoutSaving": "Saving layout...",
      "widget.layoutSaved": "Layout saved",
      "widget.layoutRetry": "Retry Layout Save",
      "widget.resizeWide": "Make Wide",
      "widget.retry": "Retry Widget",
      "widget.errorFallback": "Widget request failed.",
      "widget.filter.timeRange": "Time Range",
      "widget.filter.category": "Category",
      "widget.filter.timeRange.all": "All Time",
      "widget.filter.timeRange.7d": "Last 7 Days",
      "widget.filter.timeRange.30d": "Last 30 Days",
      "widget.filter.timeRange.current_sprint": "Current Sprint",
      "widget.filter.category.all": "All Categories",
      "widget.filter.category.tasks": "Tasks",
      "widget.filter.category.reviews": "Reviews",
      "widget.filter.category.agents": "Agents",
      "widget.filter.category.budget": "Budget",
      "widget.filter.clear": "Clear Filters",
      "widget.filter.applied": "Showing reviews for Last 7 Days",
      "widget.filter.notApplicable": "Filter not applicable",
      "widget.autoRefresh.pause": "Pause Auto Refresh",
      "widget.autoRefresh.resume": "Resume Auto Refresh",
      "widget.autoRefresh.label": "Auto Refresh",
      "widget.autoRefresh.interval.30s": "30s",
      "widget.autoRefresh.interval.60s": "60s",
      "widget.autoRefresh.interval.300s": "5m",
      "widget.autoRefresh.interval.off": "Off",
      "widget.config.chartType": "Chart Type",
      "widget.config.chartType.bar": "Bar",
      "widget.config.chartType.line": "Line",
      "widget.config.invalid": "Widget configuration is invalid.",
      "widget.alert.budget.title": "Budget threshold exceeded",
      "widget.alert.budget.message": "Budget usage is above 90%.",
      "widget.alert.budget.action": "Review budget",
      "widget.alert.blockers.title": "Blocked work requires attention",
      "widget.alert.blockers.message": "There are blocked tasks on this dashboard.",
      "widget.alert.blockers.action": "Open blocked tasks",
      "widget.alert.dismiss": "Dismiss alert",
    };
    if (key === "widget.lastUpdated") {
      return `Last updated ${String(values?.time ?? "")} ago`;
    }

    return translations[key] ?? key;
  },
}));

jest.mock("./widgets", () => ({
  ThroughputChart: ({ data }: { data: Array<{ date: string; count: number }> }) => (
    <div>Throughput points: {data.length}</div>
  ),
  BurndownChartWidget: ({
    data,
  }: {
    data: Array<{ date: string; remainingTasks: number; completedTasks: number }>;
  }) => <div>Burndown points: {data.length}</div>,
  MetricCard: ({
    label,
    value,
    secondary,
  }: {
    label: string;
    value: string;
    secondary?: string;
  }) => (
    <div>
      <span>{label}</span>
      <span>{value}</span>
      {secondary ? <span>{secondary}</span> : null}
    </div>
  ),
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (
    selector: (state: typeof storeState) => unknown
  ) => selector(storeState),
}));

describe("DashboardGrid", () => {
  const dashboard: DashboardConfig = {
    id: "dashboard-1",
    projectId: "project-1",
    name: "Sprint Overview",
    layout: [],
    createdBy: "user-1",
    createdAt: "2026-03-28T00:00:00.000Z",
    updatedAt: "2026-03-28T00:00:00.000Z",
  };

  beforeEach(() => {
    fetchWidgetData.mockClear();
    saveWidget.mockClear();
    updateDashboard.mockClear();
    deleteWidget.mockClear();
    storeState.widgetsByDashboard["dashboard-1"] = [
      {
        id: "widget-1",
        dashboardId: "dashboard-1",
        widgetType: "throughput_chart",
        config: {},
        position: { x: 0, y: 0, w: 1, h: 1 },
        createdAt: "2026-03-28T00:00:00.000Z",
        updatedAt: "2026-03-28T00:00:00.000Z",
      },
      {
        id: "widget-2",
        dashboardId: "dashboard-1",
        widgetType: "budget_consumption",
        config: {},
        position: { x: 1, y: 0, w: 1, h: 1 },
        createdAt: "2026-03-28T00:00:00.000Z",
        updatedAt: "2026-03-28T00:00:00.000Z",
      },
    ];
    storeState.widgetData["project-1:budget_consumption:{}"] = {
      spent: 12,
      allocated: 20,
    };
    delete storeState.widgetData["project-1:blocker_count:{}"];
    delete storeState.widgetRequestStateByKey["project-1:blocker_count:{}"];
    storeState.widgetRequestStateByKey[widgetDataKey] = {
      status: "success",
      error: null,
    };
    storeState.widgetRequestStateByKey["project-1:budget_consumption:{}"] = {
      status: "success",
      error: null,
    };
    storeState.widgetsByDashboard["dashboard-1"][0]!.config = {};
    storeState.widgetsByDashboard["dashboard-1"][1]!.config = {};
  });

  it("persists layout changes and shows save feedback", async () => {
    const user = userEvent.setup();

    render(<DashboardGrid projectId="project-1" dashboard={dashboard} />);
    await user.click(screen.getAllByRole("button", { name: "Make Wide" })[0]!);

    expect(saveWidget).toHaveBeenCalledWith("project-1", "dashboard-1", {
      id: "widget-1",
      widgetType: "throughput_chart",
      config: {},
      position: { x: 0, y: 0, w: 2, h: 1 },
    });
    expect(updateDashboard).toHaveBeenCalledWith("project-1", "dashboard-1", {
      layout: [
        { id: "widget-1", x: 0, y: 0, w: 2, h: 1 },
        { id: "widget-2", x: 1, y: 0, w: 1, h: 1 },
      ],
    });
    await waitFor(() =>
      expect(screen.getByText("Layout saved")).toBeInTheDocument()
    );
  });

  it("persists layout changes coming from the grid layout callback", async () => {
    const user = userEvent.setup();

    render(<DashboardGrid projectId="project-1" dashboard={dashboard} />);

    await user.click(screen.getByRole("button", { name: "Trigger Layout Change" }));

    expect(saveWidget).toHaveBeenCalledWith("project-1", "dashboard-1", {
      id: "widget-1",
      widgetType: "throughput_chart",
      config: {},
      position: { x: 1, y: 0, w: 2, h: 1 },
    });
    expect(updateDashboard).toHaveBeenCalledWith("project-1", "dashboard-1", {
      layout: [
        { id: "widget-1", x: 1, y: 0, w: 2, h: 1 },
        { id: "widget-2", x: 1, y: 0, w: 1, h: 1 },
      ],
    });
  });

  it("shows a retryable widget error state without removing the card", async () => {
    const user = userEvent.setup();
    storeState.widgetRequestStateByKey[widgetDataKey] = {
      status: "error",
      error: "Widget request failed.",
    };

    render(<DashboardGrid projectId="project-1" dashboard={dashboard} />);

    expect(screen.getByText("Widget request failed.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry Widget" }));
    expect(fetchWidgetData).toHaveBeenCalledWith("project-1", "throughput_chart", {});
  });

  it("hydrates persisted global filters from dashboard layout and uses merged widget configs", async () => {
    render(
      <DashboardGrid
        projectId="project-1"
        dashboard={{
          ...dashboard,
          layout: {
            items: [],
            filters: { timeRange: "7d", category: "reviews" },
          },
        }}
      />,
    );

    expect(screen.getByLabelText("Time Range")).toHaveValue("7d");
    expect(screen.getByLabelText("Category")).toHaveValue("reviews");
    await waitFor(() => {
      expect(fetchWidgetData).toHaveBeenCalledWith("project-1", "throughput_chart", {
        range: "7d",
        category: "reviews",
      });
    });
  });

  it("persists global dashboard filters and shows a not-applicable indicator for unsupported widgets", async () => {
    const user = userEvent.setup();

    render(<DashboardGrid projectId="project-1" dashboard={dashboard} />);

    await user.selectOptions(screen.getByLabelText("Time Range"), "7d");
    await user.selectOptions(screen.getByLabelText("Category"), "reviews");

    expect(updateDashboard).toHaveBeenCalledWith("project-1", "dashboard-1", {
      layout: {
        items: [
          { id: "widget-1", x: 0, y: 0, w: 1, h: 1 },
          { id: "widget-2", x: 1, y: 0, w: 1, h: 1 },
        ],
        filters: { timeRange: "7d", category: "reviews" },
      },
    });
    expect(fetchWidgetData).toHaveBeenCalledWith("project-1", "throughput_chart", {
      range: "7d",
      category: "reviews",
    });
    expect(screen.getByText("Showing reviews for Last 7 Days")).toBeInTheDocument();
    expect(screen.getAllByText("Filter not applicable").length).toBeGreaterThan(0);
  });

  it("auto-refreshes widgets on staggered timers and allows pause plus interval changes", async () => {
    jest.useFakeTimers();
    const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
    const consoleErrorSpy = jest.spyOn(console, "error").mockImplementation(() => {});
    storeState.widgetsByDashboard["dashboard-1"][0]!.config = {
      autoRefreshInterval: "30s",
      autoRefreshPaused: false,
    };
    storeState.widgetsByDashboard["dashboard-1"][1]!.config = {
      autoRefreshInterval: "30s",
      autoRefreshPaused: false,
    };

    const { unmount } = render(<DashboardGrid projectId="project-1" dashboard={dashboard} />);

    await act(async () => {
      await Promise.resolve();
    });

    fetchWidgetData.mockClear();

    await act(async () => {
      jest.advanceTimersByTime(30000);
      await Promise.resolve();
    });

    expect(fetchWidgetData).toHaveBeenCalledWith("project-1", "throughput_chart", {});
    const budgetCallsAfter30s = fetchWidgetData.mock.calls.filter(
      (call) => call[1] === "budget_consumption",
    ).length;

    await act(async () => {
      jest.advanceTimersByTime(500);
      await Promise.resolve();
    });

    expect(fetchWidgetData).toHaveBeenCalledWith("project-1", "budget_consumption", {});
    expect(
      fetchWidgetData.mock.calls.filter((call) => call[1] === "budget_consumption").length,
    ).toBeGreaterThan(budgetCallsAfter30s);
    await waitFor(() => {
      expect(screen.getAllByText(/Last updated/i).length).toBeGreaterThan(0);
    });

    await user.click(screen.getAllByRole("button", { name: "Pause Auto Refresh" })[0]!);
    await user.selectOptions(screen.getAllByLabelText("Auto Refresh")[0]!, "60s");

    expect(saveWidget).toHaveBeenNthCalledWith(1, "project-1", "dashboard-1", {
      id: "widget-1",
      widgetType: "throughput_chart",
      config: {
        autoRefreshInterval: "30s",
        autoRefreshPaused: true,
      },
      position: { x: 0, y: 0, w: 1, h: 1 },
    });
    expect(saveWidget).toHaveBeenNthCalledWith(2, "project-1", "dashboard-1", {
      id: "widget-1",
      widgetType: "throughput_chart",
      config: {
        autoRefreshInterval: "60s",
        autoRefreshPaused: false,
      },
      position: { x: 0, y: 0, w: 1, h: 1 },
    });

    await act(async () => {
      unmount();
      await Promise.resolve();
    });

    jest.runOnlyPendingTimers();
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    consoleErrorSpy.mockRestore();
    jest.useRealTimers();
  });

  it("saves throughput chart type changes from the configuration panel", async () => {
    const user = userEvent.setup();

    render(<DashboardGrid projectId="project-1" dashboard={dashboard} />);

    await user.click(screen.getAllByRole("button", { name: "widget.configure" })[0]!);
    await user.selectOptions(screen.getByLabelText("Chart Type"), "line");
    await user.click(screen.getByRole("button", { name: "widget.saveConfig" }));

    expect(saveWidget).toHaveBeenCalledWith("project-1", "dashboard-1", {
      id: "widget-1",
      widgetType: "throughput_chart",
      config: { chartType: "line" },
      position: { x: 0, y: 0, w: 1, h: 1 },
    });
  });

  it("shows inline validation and disables save when widget config is invalid", async () => {
    const user = userEvent.setup();
    storeState.widgetsByDashboard["dashboard-1"][0]!.config = {
      chartType: "scatter",
    };

    render(<DashboardGrid projectId="project-1" dashboard={dashboard} />);

    await user.click(screen.getAllByRole("button", { name: "widget.configure" })[0]!);

    expect(screen.getByText("Widget configuration is invalid.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "widget.saveConfig" })).toBeDisabled();

    await user.selectOptions(screen.getByLabelText("Chart Type"), "bar");

    expect(screen.queryByText("Widget configuration is invalid.")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "widget.saveConfig" })).not.toBeDisabled();
  });

  it("shows a budget alert banner with an action link and allows dismissing it for the session", async () => {
    const user = userEvent.setup();
    storeState.widgetData["project-1:budget_consumption:{}"] = {
      spent: 19,
      allocated: 20,
    };

    render(<DashboardGrid projectId="project-1" dashboard={dashboard} />);

    const reviewBudgetLink = await screen.findByRole("link", { name: "Review budget" });
    expect(reviewBudgetLink).toHaveAttribute("href", "/cost");
    expect(screen.getByText("Budget threshold exceeded")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Dismiss alert" }));

    expect(screen.queryByText("Budget threshold exceeded")).not.toBeInTheDocument();
  });

  it("orders multiple alerts by priority with budget before blocker warnings", async () => {
    storeState.widgetsByDashboard["dashboard-1"].push({
      id: "widget-3",
      dashboardId: "dashboard-1",
      widgetType: "blocker_count",
      config: {},
      position: { x: 2, y: 0, w: 1, h: 1 },
      createdAt: "2026-03-28T00:00:00.000Z",
      updatedAt: "2026-03-28T00:00:00.000Z",
    });
    storeState.widgetData["project-1:blocker_count:{}"] = { count: 4 };
    storeState.widgetRequestStateByKey["project-1:blocker_count:{}"] = {
      status: "success",
      error: null,
    };
    storeState.widgetData["project-1:budget_consumption:{}"] = {
      spent: 19,
      allocated: 20,
    };

    render(<DashboardGrid projectId="project-1" dashboard={dashboard} />);

    const titles = await screen.findAllByRole("heading", { level: 3 });
    expect(titles[0]).toHaveTextContent("Budget threshold exceeded");
    expect(titles[1]).toHaveTextContent("Blocked work requires attention");
  });
});
