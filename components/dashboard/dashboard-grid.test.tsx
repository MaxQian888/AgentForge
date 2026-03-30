import { render, screen, waitFor } from "@testing-library/react";
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
    ],
  },
  widgetData: {
    [widgetDataKey]: {
      widgetType: "throughput_chart",
      points: [{ date: "2026-03-28", count: 3 }],
    },
  },
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
    };

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
    storeState.widgetRequestStateByKey[widgetDataKey] = {
      status: "success",
      error: null,
    };
  });

  it("persists layout changes and shows save feedback", async () => {
    const user = userEvent.setup();

    render(<DashboardGrid projectId="project-1" dashboard={dashboard} />);
    await user.click(screen.getByRole("button", { name: "Make Wide" }));

    expect(saveWidget).toHaveBeenCalledWith("project-1", "dashboard-1", {
      id: "widget-1",
      widgetType: "throughput_chart",
      config: {},
      position: { x: 0, y: 0, w: 2, h: 1 },
    });
    expect(updateDashboard).toHaveBeenCalledWith("project-1", "dashboard-1", {
      layout: [{ id: "widget-1", x: 0, y: 0, w: 2, h: 1 }],
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
      layout: [{ id: "widget-1", x: 1, y: 0, w: 2, h: 1 }],
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
});
