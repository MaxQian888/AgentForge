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
import { useEmployeeTriggerStore } from "./employee-trigger-store";
import type { WorkflowTrigger } from "./workflow-trigger-store";

const sampleTrigger: WorkflowTrigger = {
  id: "trg-1",
  workflowId: "wf-1",
  projectId: "proj-1",
  source: "im",
  targetKind: "dag",
  config: { platform: "feishu", command: "/echo" },
  inputMapping: { text: "{{$event.content}}" },
  dedupeWindowSeconds: 0,
  enabled: true,
  actingEmployeeId: "emp-1",
  createdAt: "2026-04-20T00:00:00Z",
  updatedAt: "2026-04-20T00:00:00Z",
};

const newApi = () => ({
  get: jest.fn(),
  post: jest.fn(),
  patch: jest.fn(),
  delete: jest.fn(),
});

describe("useEmployeeTriggerStore", () => {
  beforeEach(() => {
    useEmployeeTriggerStore.setState({
      triggersByEmployee: {},
      loading: {},
    });
    jest.clearAllMocks();
  });

  it("fetchByEmployee hydrates triggersByEmployee[employeeId]", async () => {
    const api = newApi();
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({ data: [sampleTrigger] });

    await useEmployeeTriggerStore.getState().fetchByEmployee("emp-1");

    expect(api.get).toHaveBeenCalledWith(
      "/api/v1/employees/emp-1/triggers",
      { token: "test-token" },
    );
    expect(useEmployeeTriggerStore.getState().triggersByEmployee["emp-1"]).toEqual([
      sampleTrigger,
    ]);
  });

  it("createTrigger POSTs and prepends locally when actingEmployeeId set", async () => {
    const api = newApi();
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.post.mockResolvedValue({ data: sampleTrigger });

    const created = await useEmployeeTriggerStore.getState().createTrigger({
      workflowId: "wf-1",
      source: "im",
      config: { platform: "feishu", command: "/echo" },
      inputMapping: { text: "{{$event.content}}" },
      actingEmployeeId: "emp-1",
      displayName: "echo",
    });

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/triggers",
      expect.objectContaining({
        workflowId: "wf-1",
        source: "im",
        actingEmployeeId: "emp-1",
      }),
      { token: "test-token" },
    );
    expect(created).toEqual(sampleTrigger);
    expect(useEmployeeTriggerStore.getState().triggersByEmployee["emp-1"]).toEqual([
      sampleTrigger,
    ]);
  });

  it("patchTrigger PATCHes and mutates the row in place across all buckets", async () => {
    const api = newApi();
    (createApiClient as jest.Mock).mockReturnValue(api);
    const patched: WorkflowTrigger = { ...sampleTrigger, enabled: false };
    api.patch.mockResolvedValue({ data: patched });
    useEmployeeTriggerStore.setState({
      triggersByEmployee: { "emp-1": [sampleTrigger] },
      loading: {},
    });

    const got = await useEmployeeTriggerStore
      .getState()
      .patchTrigger("trg-1", { enabled: false });

    expect(api.patch).toHaveBeenCalledWith(
      "/api/v1/triggers/trg-1",
      { enabled: false },
      { token: "test-token" },
    );
    expect(got).toEqual(patched);
    expect(useEmployeeTriggerStore.getState().triggersByEmployee["emp-1"][0].enabled).toBe(false);
  });

  it("deleteTrigger DELETEs and removes the row from the local list", async () => {
    const api = newApi();
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.delete.mockResolvedValue({ data: null });
    useEmployeeTriggerStore.setState({
      triggersByEmployee: { "emp-1": [sampleTrigger] },
      loading: {},
    });

    await useEmployeeTriggerStore.getState().deleteTrigger("trg-1", "emp-1");

    expect(api.delete).toHaveBeenCalledWith(
      "/api/v1/triggers/trg-1",
      { token: "test-token" },
    );
    expect(useEmployeeTriggerStore.getState().triggersByEmployee["emp-1"]).toEqual([]);
  });

  it("testTrigger POSTs to the dry-run endpoint and returns the result without mutating store", async () => {
    const api = newApi();
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.post.mockResolvedValue({
      data: { matched: true, would_dispatch: true, rendered_input: { text: "hi" } },
    });
    useEmployeeTriggerStore.setState({
      triggersByEmployee: { "emp-1": [sampleTrigger] },
      loading: {},
    });

    const res = await useEmployeeTriggerStore.getState().testTrigger("trg-1", {
      platform: "feishu",
      command: "/echo",
      content: "/echo hi",
    });

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/triggers/trg-1/test",
      { event: { platform: "feishu", command: "/echo", content: "/echo hi" } },
      { token: "test-token" },
    );
    expect(res?.matched).toBe(true);
    expect(res?.would_dispatch).toBe(true);
    // Store unchanged.
    expect(useEmployeeTriggerStore.getState().triggersByEmployee["emp-1"]).toEqual([
      sampleTrigger,
    ]);
  });
});
