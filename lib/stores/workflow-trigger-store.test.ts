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
import { useWorkflowTriggerStore, type WorkflowTrigger } from "./workflow-trigger-store";

const sampleTrigger: WorkflowTrigger = {
  id: "trg-1",
  workflowId: "wf-1",
  projectId: "proj-1",
  source: "im",
  targetKind: "dag",
  config: { platform: "feishu", command: "/review" },
  inputMapping: { pr_url: "{{$event.args.0}}" },
  dedupeWindowSeconds: 0,
  enabled: true,
  createdAt: "2026-04-19T00:00:00Z",
  updatedAt: "2026-04-19T00:00:00Z",
};

const pluginTrigger: WorkflowTrigger = {
  id: "trg-2",
  pluginId: "workflow-plugin-review",
  projectId: "proj-1",
  source: "im",
  targetKind: "plugin",
  config: { platform: "feishu", command: "/review" },
  dedupeWindowSeconds: 0,
  enabled: false,
  disabledReason: "plugin_disabled",
  createdAt: "2026-04-19T00:00:00Z",
  updatedAt: "2026-04-19T00:00:00Z",
};

describe("useWorkflowTriggerStore", () => {
  beforeEach(() => {
    useWorkflowTriggerStore.setState({
      triggersByWorkflow: {},
      loading: {},
    });
    jest.clearAllMocks();
  });

  it("fetches triggers for a workflow", async () => {
    const api = { get: jest.fn(), post: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({ data: [sampleTrigger] });

    await useWorkflowTriggerStore.getState().fetchTriggers("wf-1");

    expect(api.get).toHaveBeenCalledWith(
      "/api/v1/workflows/wf-1/triggers",
      { token: "test-token" },
    );
    expect(useWorkflowTriggerStore.getState().triggersByWorkflow["wf-1"]).toEqual([sampleTrigger]);
  });

  it("round-trips both dag and plugin target kinds", async () => {
    const api = { get: jest.fn(), post: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({ data: [sampleTrigger, pluginTrigger] });

    await useWorkflowTriggerStore.getState().fetchTriggers("wf-1");

    const rows = useWorkflowTriggerStore.getState().triggersByWorkflow["wf-1"];
    expect(rows).toHaveLength(2);
    const dag = rows.find((t) => t.targetKind === "dag");
    const plug = rows.find((t) => t.targetKind === "plugin");
    expect(dag?.workflowId).toBe("wf-1");
    expect(plug?.pluginId).toBe("workflow-plugin-review");
    expect(plug?.disabledReason).toBe("plugin_disabled");
    expect(plug?.enabled).toBe(false);
  });

  it("round-trips actingEmployeeId on trigger list responses", async () => {
    const api = { get: jest.fn(), post: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    const triggerWithEmployee: WorkflowTrigger = {
      ...sampleTrigger,
      id: "trg-emp",
      actingEmployeeId: "11111111-2222-3333-4444-555555555555",
    };
    api.get.mockResolvedValue({ data: [triggerWithEmployee] });

    await useWorkflowTriggerStore.getState().fetchTriggers("wf-1");

    const rows = useWorkflowTriggerStore.getState().triggersByWorkflow["wf-1"];
    expect(rows).toHaveLength(1);
    expect(rows[0].actingEmployeeId).toBe("11111111-2222-3333-4444-555555555555");
  });

  it("flips the enabled flag in-memory after setEnabled", async () => {
    const api = { get: jest.fn(), post: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.post.mockResolvedValue({ data: null });
    useWorkflowTriggerStore.setState({
      triggersByWorkflow: { "wf-1": [sampleTrigger] },
      loading: {},
    });

    await useWorkflowTriggerStore.getState().setEnabled("wf-1", sampleTrigger.id, false);

    expect(api.post).toHaveBeenCalledWith(
      `/api/v1/triggers/${sampleTrigger.id}/enabled`,
      { enabled: false },
      { token: "test-token" },
    );
    expect(useWorkflowTriggerStore.getState().triggersByWorkflow["wf-1"][0].enabled).toBe(false);
  });
});
