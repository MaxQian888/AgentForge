jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ accessToken: "test-token" })),
  },
}));

import { createApiClient } from "@/lib/api-client";
import {
  useWorkflowRunStore,
  computeSummary,
  type UnifiedRunRow,
} from "./workflow-run-store";

const dagRow: UnifiedRunRow = {
  engine: "dag",
  runId: "11111111-1111-1111-1111-111111111111",
  workflowRef: { id: "wf-1", name: "Sample DAG" },
  status: "running",
  startedAt: "2026-04-20T10:00:00Z",
  triggeredBy: { kind: "manual" },
};

const pluginRow: UnifiedRunRow = {
  engine: "plugin",
  runId: "22222222-2222-2222-2222-222222222222",
  workflowRef: { id: "plugin-a", name: "plugin-a" },
  status: "completed",
  startedAt: "2026-04-20T09:00:00Z",
  triggeredBy: { kind: "trigger", ref: "trg-1" },
};

describe("useWorkflowRunStore", () => {
  beforeEach(() => {
    useWorkflowRunStore.setState({
      rows: [],
      summary: { running: 0, paused: 0, failed: 0 },
      nextCursor: null,
      filter: {},
      loading: false,
      error: null,
      selectedDetail: null,
      detailLoading: false,
    });
    jest.clearAllMocks();
  });

  it("fetches unified runs with filter round-trip", async () => {
    const api = { get: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({
      data: {
        rows: [dagRow, pluginRow],
        nextCursor: "cur-xyz",
        summary: { running: 1, paused: 0, failed: 0 },
      },
    });

    useWorkflowRunStore
      .getState()
      .setFilter({ engine: "plugin", status: ["running", "paused"] });

    await useWorkflowRunStore.getState().fetchUnifiedRuns("proj-1");

    expect(api.get).toHaveBeenCalledTimes(1);
    const [url] = api.get.mock.calls[0] as [string];
    expect(url).toContain("/api/v1/projects/proj-1/workflow-runs?");
    expect(url).toContain("engine=plugin");
    expect(url).toContain("status=running");
    expect(url).toContain("status=paused");

    const state = useWorkflowRunStore.getState();
    expect(state.rows).toHaveLength(2);
    expect(state.nextCursor).toBe("cur-xyz");
    expect(state.summary.running).toBe(1);
  });

  it("paginates by appending when cursor is provided", async () => {
    const api = { get: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);

    api.get.mockResolvedValueOnce({
      data: { rows: [dagRow], nextCursor: "cur-next", summary: { running: 1, paused: 0, failed: 0 } },
    });
    await useWorkflowRunStore.getState().fetchUnifiedRuns("proj-1");
    expect(useWorkflowRunStore.getState().rows).toHaveLength(1);

    api.get.mockResolvedValueOnce({
      data: { rows: [pluginRow], summary: { running: 1, paused: 0, failed: 0 } },
    });
    await useWorkflowRunStore
      .getState()
      .fetchUnifiedRuns("proj-1", { append: true, cursor: "cur-next" });
    const { rows, nextCursor } = useWorkflowRunStore.getState();
    expect(rows).toHaveLength(2);
    expect(rows[0].engine).toBe("dag");
    expect(rows[1].engine).toBe("plugin");
    expect(nextCursor).toBeNull();
    const secondCallUrl = api.get.mock.calls[1][0] as string;
    expect(secondCallUrl).toContain("cursor=cur-next");
  });

  it("fetches run detail routed by engine", async () => {
    const api = { get: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    const detail = { row: dagRow, body: { execution: { id: dagRow.runId } } };
    api.get.mockResolvedValue({ data: detail });

    const result = await useWorkflowRunStore
      .getState()
      .fetchRunDetail("proj-1", "dag", dagRow.runId);
    expect(result).toEqual(detail);
    expect(api.get).toHaveBeenCalledWith(
      `/api/v1/projects/proj-1/workflow-runs/dag/${dagRow.runId}`,
      { token: "test-token" }
    );
    expect(useWorkflowRunStore.getState().selectedDetail).toEqual(detail);
  });

  it("applies realtime row as upsert keyed on (engine, runId)", () => {
    useWorkflowRunStore.setState({ rows: [dagRow] });
    useWorkflowRunStore
      .getState()
      .applyRealtimeRow({ ...dagRow, status: "completed" }, true);
    const after = useWorkflowRunStore.getState().rows;
    expect(after).toHaveLength(1);
    expect(after[0].status).toBe("completed");
    expect(useWorkflowRunStore.getState().summary.running).toBe(0);

    useWorkflowRunStore.getState().applyRealtimeRow(pluginRow, false);
    const final = useWorkflowRunStore.getState().rows;
    expect(final).toHaveLength(2);
    expect(final[0].engine).toBe("plugin");
  });

  it("computeSummary counts running/paused/failed", () => {
    expect(
      computeSummary([
        { ...dagRow, status: "running" },
        { ...pluginRow, status: "paused" },
        { ...dagRow, runId: "r3", status: "failed" },
        { ...dagRow, runId: "r4", status: "completed" },
      ])
    ).toEqual({ running: 1, paused: 1, failed: 1 });
  });
});
