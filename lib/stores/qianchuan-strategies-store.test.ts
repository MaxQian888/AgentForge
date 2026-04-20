jest.mock("@/lib/api-client", () => {
  const actual = jest.requireActual("@/lib/api-client");
  return {
    ...actual,
    createApiClient: jest.fn(),
  };
});

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ accessToken: "test-token" })),
  },
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

import { ApiError, createApiClient } from "@/lib/api-client";
import {
  useQianchuanStrategiesStore,
  type QianchuanStrategy,
} from "./qianchuan-strategies-store";

const projectId = "11111111-1111-1111-1111-111111111111";

const sampleRow: QianchuanStrategy = {
  id: "22222222-2222-2222-2222-222222222222",
  projectId,
  name: "test",
  description: "",
  yamlSource: "name: test",
  parsedSpec: `{"schema_version":1}`,
  version: 1,
  status: "draft",
  createdBy: "00000000-0000-0000-0000-000000000001",
  createdAt: "2026-04-20T00:00:00Z",
  updatedAt: "2026-04-20T00:00:00Z",
  isSystem: false,
};

describe("useQianchuanStrategiesStore", () => {
  beforeEach(() => {
    useQianchuanStrategiesStore.setState({
      strategies: [],
      selected: null,
      loading: false,
      lastError: null,
      lastTestResult: null,
    });
    jest.clearAllMocks();
  });

  it("fetchList populates strategies", async () => {
    const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({ data: [sampleRow] });

    await useQianchuanStrategiesStore.getState().fetchList(projectId);

    expect(api.get).toHaveBeenCalledWith(
      `/api/v1/projects/${projectId}/qianchuan/strategies`,
      { token: "test-token" },
    );
    expect(useQianchuanStrategiesStore.getState().strategies).toEqual([sampleRow]);
  });

  it("create appends to strategies on success", async () => {
    const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.post.mockResolvedValue({ data: sampleRow });

    const created = await useQianchuanStrategiesStore.getState().create(projectId, "name: test");
    expect(created).toEqual(sampleRow);
    expect(useQianchuanStrategiesStore.getState().strategies).toEqual([sampleRow]);
  });

  it("create surfaces structured StrategyParseError on 400", async () => {
    const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.post.mockRejectedValue(
      new ApiError("HTTP 400", 400, {
        error: { line: 3, col: 5, field: "rules[0].condition", msg: "must be non-empty" },
      }),
    );

    const created = await useQianchuanStrategiesStore.getState().create(projectId, "x");
    expect(created).toBeNull();
    const err = useQianchuanStrategiesStore.getState().lastError;
    expect(err).toEqual({
      line: 3,
      col: 5,
      field: "rules[0].condition",
      msg: "must be non-empty",
    });
  });

  it("publish updates the row in place", async () => {
    const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    useQianchuanStrategiesStore.setState({ strategies: [sampleRow] });
    const published: QianchuanStrategy = { ...sampleRow, status: "published" };
    api.post.mockResolvedValue({ data: published });

    const result = await useQianchuanStrategiesStore.getState().publish(sampleRow.id);
    expect(result?.status).toBe("published");
    expect(useQianchuanStrategiesStore.getState().strategies[0].status).toBe("published");
  });

  it("archive flips status to archived", async () => {
    const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    const publishedRow: QianchuanStrategy = { ...sampleRow, status: "published" };
    useQianchuanStrategiesStore.setState({ strategies: [publishedRow] });
    api.post.mockResolvedValue({ data: { ...publishedRow, status: "archived" } });

    const result = await useQianchuanStrategiesStore.getState().archive(sampleRow.id);
    expect(result?.status).toBe("archived");
    expect(useQianchuanStrategiesStore.getState().strategies[0].status).toBe("archived");
  });

  it("testRun stores last result", async () => {
    const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.post.mockResolvedValue({
      data: { fired_rules: ["heartbeat"], actions: [{ rule: "heartbeat", type: "notify_im", params: {} }] },
    });

    const res = await useQianchuanStrategiesStore.getState().testRun(sampleRow.id, { metrics: { cost: 12 } });
    expect(res?.fired_rules).toEqual(["heartbeat"]);
    expect(useQianchuanStrategiesStore.getState().lastTestResult?.actions?.[0].type).toBe("notify_im");
  });

  it("remove drops the row locally on success", async () => {
    const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    useQianchuanStrategiesStore.setState({ strategies: [sampleRow] });
    api.delete.mockResolvedValue({ data: null });

    const ok = await useQianchuanStrategiesStore.getState().remove(sampleRow.id);
    expect(ok).toBe(true);
    expect(useQianchuanStrategiesStore.getState().strategies).toEqual([]);
  });
});
