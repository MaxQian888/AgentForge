import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const fetchUnifiedRuns = jest.fn();
const fetchRunDetail = jest.fn();
const setFilter = jest.fn();
const clearDetail = jest.fn();

let runStoreState = {
  rows: [] as any[],
  summary: { running: 0, paused: 0, failed: 0 },
  nextCursor: null as string | null,
  loading: false,
  filter: {} as any,
  selectedDetail: null as any,
  detailLoading: false,
  fetchUnifiedRuns,
  fetchRunDetail,
  setFilter,
  clearDetail,
};

jest.mock("@/lib/stores/workflow-run-store", () => {
  const selector = (selectorFn?: (s: any) => any) =>
    selectorFn ? selectorFn(runStoreState) : runStoreState;
  selector.getState = () => runStoreState;
  return { useWorkflowRunStore: selector };
});

jest.mock("@/lib/stores/workflow-store", () => ({
  useWorkflowStore: (selectorFn?: (s: any) => any) =>
    selectorFn ? selectorFn({ definitions: [] }) : { definitions: [] },
}));

import { WorkflowRunsTab } from "./workflow-runs-tab";

describe("WorkflowRunsTab", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    runStoreState = {
      rows: [],
      summary: { running: 0, paused: 0, failed: 0 },
      nextCursor: null,
      loading: false,
      filter: {},
      selectedDetail: null,
      detailLoading: false,
      fetchUnifiedRuns,
      fetchRunDetail,
      setFilter,
      clearDetail,
    };
  });

  it("fetches runs on mount and renders engine-filter chips", async () => {
    render(<WorkflowRunsTab projectId="proj-1" />);
    await waitFor(() => expect(fetchUnifiedRuns).toHaveBeenCalledWith("proj-1"));
    expect(screen.getByRole("button", { name: "All" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "DAG" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Plugin" })).toBeInTheDocument();
  });

  it("switches the engine filter when a chip is clicked", async () => {
    const user = userEvent.setup();
    render(<WorkflowRunsTab projectId="proj-1" />);
    await user.click(screen.getByRole("button", { name: "Plugin" }));
    expect(setFilter).toHaveBeenCalledWith(expect.objectContaining({ engine: "plugin" }));
  });

  it("renders rows with engine badge, status, and trigger label", () => {
    runStoreState = {
      ...runStoreState,
      rows: [
        {
          engine: "dag",
          runId: "r-dag-1",
          workflowRef: { id: "wf-1", name: "Plan Review" },
          status: "running",
          startedAt: new Date(Date.now() - 120 * 1000).toISOString(),
          triggeredBy: { kind: "manual" },
        },
        {
          engine: "plugin",
          runId: "r-plg-1",
          workflowRef: { id: "plugin-a", name: "plugin-a" },
          status: "completed",
          startedAt: new Date(Date.now() - 3600 * 1000).toISOString(),
          triggeredBy: { kind: "trigger", ref: "trg-1" },
        },
      ],
      summary: { running: 1, paused: 0, failed: 0 },
    };
    render(<WorkflowRunsTab projectId="proj-1" />);
    expect(screen.getByText("Plan Review")).toBeInTheDocument();
    expect(screen.getByText("plugin-a")).toBeInTheDocument();
    // Status badges (visible via text)
    expect(screen.getByText("running")).toBeInTheDocument();
    expect(screen.getByText("completed")).toBeInTheDocument();
    // Trigger labels
    expect(screen.getByText("Manual")).toBeInTheDocument();
    expect(screen.getByText("Triggered")).toBeInTheDocument();
    // Engine badges: both the filter chips and row badges render "DAG"/"Plugin".
    // Each label appears at least twice (filter chip + one row badge each).
    expect(screen.getAllByText("DAG").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("Plugin").length).toBeGreaterThanOrEqual(2);
  });

  it("opens detail when a row is clicked", async () => {
    runStoreState = {
      ...runStoreState,
      rows: [
        {
          engine: "plugin",
          runId: "r-plg-1",
          workflowRef: { id: "plugin-a", name: "plugin-a" },
          status: "running",
          startedAt: new Date().toISOString(),
          triggeredBy: { kind: "manual" },
        },
      ],
    };
    const user = userEvent.setup();
    render(<WorkflowRunsTab projectId="proj-1" />);
    await user.click(screen.getByText("plugin-a"));
    expect(fetchRunDetail).toHaveBeenCalledWith("proj-1", "plugin", "r-plg-1");
  });
});
