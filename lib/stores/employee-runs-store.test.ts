jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ accessToken: "test-token" })),
  },
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

import { createApiClient } from "@/lib/api-client";
import {
  useEmployeeRunsStore,
  type EmployeeRunRow,
} from "./employee-runs-store";

const empID = "00000000-0000-0000-0000-000000000001";

const baseRow: EmployeeRunRow = {
  kind: "workflow",
  id: "11111111-1111-1111-1111-111111111111",
  name: "echo-flow",
  status: "completed",
  startedAt: "2026-04-20T10:00:00Z",
  completedAt: "2026-04-20T10:00:45Z",
  durationMs: 45000,
  refUrl: "/workflow/runs/11111111-1111-1111-1111-111111111111",
};

describe("useEmployeeRunsStore", () => {
  beforeEach(() => {
    useEmployeeRunsStore.setState({
      runsByEmployee: {},
      loadingByEmployee: {},
      pageByEmployee: {},
      hasMoreByEmployee: {},
      kindByEmployee: {},
    });
    jest.clearAllMocks();
  });

  it("fetchRuns populates runsByEmployee and infers hasMore=false when items < page size", async () => {
    const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({
      data: { items: [baseRow], page: 1, size: 20, kind: "all" },
    });

    await useEmployeeRunsStore.getState().fetchRuns(empID, 1);

    expect(api.get).toHaveBeenCalledWith(
      `/api/v1/employees/${empID}/runs?type=all&page=1&size=20`,
      { token: "test-token" },
    );
    const state = useEmployeeRunsStore.getState();
    expect(state.runsByEmployee[empID]).toHaveLength(1);
    expect(state.runsByEmployee[empID][0].name).toBe("echo-flow");
    expect(state.hasMoreByEmployee[empID]).toBe(false);
    expect(state.loadingByEmployee[empID]).toBe(false);
    expect(state.pageByEmployee[empID]).toBe(1);
    expect(state.kindByEmployee[empID]).toBe("all");
  });

  it("fetchRuns merges paginated results when page > 1", async () => {
    const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);

    const page1Items: EmployeeRunRow[] = Array.from({ length: 20 }, (_, i) => ({
      ...baseRow,
      id: `row-1-${i}`,
    }));
    const page2Items: EmployeeRunRow[] = [{ ...baseRow, id: "row-2-0" }];

    api.get.mockResolvedValueOnce({
      data: { items: page1Items, page: 1, size: 20, kind: "all" },
    });
    await useEmployeeRunsStore.getState().fetchRuns(empID, 1);
    expect(useEmployeeRunsStore.getState().hasMoreByEmployee[empID]).toBe(true);

    api.get.mockResolvedValueOnce({
      data: { items: page2Items, page: 2, size: 20, kind: "all" },
    });
    await useEmployeeRunsStore.getState().fetchRuns(empID, 2);

    const state = useEmployeeRunsStore.getState();
    expect(state.runsByEmployee[empID]).toHaveLength(21);
    expect(state.pageByEmployee[empID]).toBe(2);
    expect(state.hasMoreByEmployee[empID]).toBe(false);
  });

  it("ingestWorkflowEvent prepends a new run for the matching employee", () => {
    useEmployeeRunsStore.setState({
      runsByEmployee: { [empID]: [baseRow] },
    });
    useEmployeeRunsStore.getState().ingestWorkflowEvent(empID, {
      kind: "workflow",
      id: "22222222-2222-2222-2222-222222222222",
      name: "card-flow",
      status: "running",
      startedAt: "2026-04-20T10:05:00Z",
      refUrl: "/workflow/runs/22222222-2222-2222-2222-222222222222",
    });
    const list = useEmployeeRunsStore.getState().runsByEmployee[empID];
    expect(list).toHaveLength(2);
    expect(list[0].id).toBe("22222222-2222-2222-2222-222222222222");
  });

  it("ingestWorkflowEvent updates an existing row in place", () => {
    useEmployeeRunsStore.setState({
      runsByEmployee: {
        [empID]: [
          {
            ...baseRow,
            status: "running",
            completedAt: undefined,
            durationMs: undefined,
          },
        ],
      },
    });
    useEmployeeRunsStore.getState().ingestWorkflowEvent(empID, {
      ...baseRow,
      status: "completed",
    });
    const row = useEmployeeRunsStore.getState().runsByEmployee[empID][0];
    expect(row.status).toBe("completed");
    expect(row.completedAt).toBe("2026-04-20T10:00:45Z");
  });

  it("reset clears per-employee slots", () => {
    useEmployeeRunsStore.setState({
      runsByEmployee: { [empID]: [baseRow], other: [baseRow] },
      loadingByEmployee: { [empID]: true },
      pageByEmployee: { [empID]: 3 },
      hasMoreByEmployee: { [empID]: true },
      kindByEmployee: { [empID]: "workflow" },
    });
    useEmployeeRunsStore.getState().reset(empID);
    const state = useEmployeeRunsStore.getState();
    expect(state.runsByEmployee[empID]).toBeUndefined();
    expect(state.runsByEmployee.other).toBeDefined();
    expect(state.loadingByEmployee[empID]).toBeUndefined();
    expect(state.pageByEmployee[empID]).toBeUndefined();
    expect(state.hasMoreByEmployee[empID]).toBeUndefined();
    expect(state.kindByEmployee[empID]).toBeUndefined();
  });
});
