jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ accessToken: "test-token" })),
  },
}));

jest.mock("sonner", () => ({
  toast: {
    success: jest.fn(),
    error: jest.fn(),
    message: jest.fn(),
  },
}));

import { createApiClient } from "@/lib/api-client";
import {
  useVCSIntegrationsStore,
  type VCSIntegration,
} from "./vcs-integrations-store";

const sample: VCSIntegration = {
  id: "i1",
  projectId: "p1",
  provider: "github",
  host: "github.com",
  owner: "octocat",
  repo: "hello",
  defaultBranch: "main",
  webhookId: "hook-1",
  tokenSecretRef: "vcs.github.demo.pat",
  webhookSecretRef: "vcs.github.demo.webhook",
  status: "active",
  createdAt: "2026-04-20T00:00:00Z",
  updatedAt: "2026-04-20T00:00:00Z",
};

describe("useVCSIntegrationsStore", () => {
  beforeEach(() => {
    useVCSIntegrationsStore.setState({
      integrationsByProject: {},
      loadingByProject: {},
    });
    jest.clearAllMocks();
  });

  it("fetches integrations into the project slot", async () => {
    const api = {
      get: jest.fn(),
      post: jest.fn(),
      patch: jest.fn(),
      delete: jest.fn(),
    };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({ data: [sample] });

    await useVCSIntegrationsStore.getState().fetchIntegrations("p1");

    expect(api.get).toHaveBeenCalledWith(
      "/api/v1/projects/p1/vcs-integrations",
      { token: "test-token" },
    );
    expect(useVCSIntegrationsStore.getState().integrationsByProject["p1"]).toEqual([
      sample,
    ]);
  });

  it("creates an integration and prepends it to the project slot", async () => {
    const api = {
      get: jest.fn(),
      post: jest.fn(),
      patch: jest.fn(),
      delete: jest.fn(),
    };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.post.mockResolvedValue({ data: sample });
    useVCSIntegrationsStore.setState({
      integrationsByProject: { p1: [] },
      loadingByProject: {},
    });

    const created = await useVCSIntegrationsStore.getState().createIntegration("p1", {
      provider: "github",
      host: "github.com",
      owner: "octocat",
      repo: "hello",
      defaultBranch: "main",
      tokenSecretRef: "vcs.github.demo.pat",
      webhookSecretRef: "vcs.github.demo.webhook",
    });

    expect(created?.webhookId).toBe("hook-1");
    expect(
      useVCSIntegrationsStore.getState().integrationsByProject["p1"][0],
    ).toEqual(sample);
  });

  it("patches an integration in-place", async () => {
    const api = {
      get: jest.fn(),
      post: jest.fn(),
      patch: jest.fn(),
      delete: jest.fn(),
    };
    (createApiClient as jest.Mock).mockReturnValue(api);
    const patched = { ...sample, status: "paused" as const };
    api.patch.mockResolvedValue({ data: patched });
    useVCSIntegrationsStore.setState({
      integrationsByProject: { p1: [sample] },
      loadingByProject: {},
    });

    const result = await useVCSIntegrationsStore.getState().patchIntegration("i1", {
      status: "paused",
    });

    expect(result?.status).toBe("paused");
    expect(
      useVCSIntegrationsStore.getState().integrationsByProject["p1"][0].status,
    ).toBe("paused");
  });

  it("removes from the project slot on delete", async () => {
    const api = {
      get: jest.fn(),
      post: jest.fn(),
      patch: jest.fn(),
      delete: jest.fn(),
    };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.delete.mockResolvedValue({ data: null });
    useVCSIntegrationsStore.setState({
      integrationsByProject: { p1: [sample] },
      loadingByProject: {},
    });

    await useVCSIntegrationsStore.getState().deleteIntegration("p1", "i1");

    expect(useVCSIntegrationsStore.getState().integrationsByProject["p1"]).toEqual([]);
  });

  it("sync POSTs to the sync endpoint", async () => {
    const api = {
      get: jest.fn(),
      post: jest.fn(),
      patch: jest.fn(),
      delete: jest.fn(),
    };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.post.mockResolvedValue({ data: { integration: sample, note: "" } });

    await useVCSIntegrationsStore.getState().syncIntegration("i1");

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/vcs-integrations/i1/sync",
      {},
      { token: "test-token" },
    );
  });
});
