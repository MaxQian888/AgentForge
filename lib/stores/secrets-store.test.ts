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
import { useSecretsStore, type SecretMetadata } from "./secrets-store";

const sample: SecretMetadata = {
  name: "GITHUB_TOKEN",
  description: "review token",
  createdBy: "user-1",
  createdAt: "2026-04-20T00:00:00Z",
  updatedAt: "2026-04-20T00:00:00Z",
};

describe("useSecretsStore", () => {
  beforeEach(() => {
    useSecretsStore.setState({
      secretsByProject: {},
      loadingByProject: {},
      lastRevealedValue: null,
    });
    jest.clearAllMocks();
  });

  it("fetches secrets metadata into the project slot", async () => {
    const api = {
      get: jest.fn(),
      post: jest.fn(),
      patch: jest.fn(),
      delete: jest.fn(),
    };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({ data: [sample] });

    await useSecretsStore.getState().fetchSecrets("proj-1");

    expect(api.get).toHaveBeenCalledWith(
      "/api/v1/projects/proj-1/secrets",
      { token: "test-token" },
    );
    expect(useSecretsStore.getState().secretsByProject["proj-1"]).toEqual([
      sample,
    ]);
  });

  it("captures the one-time value on create and clears it on consumeRevealedValue", async () => {
    const api = {
      get: jest.fn(),
      post: jest.fn(),
      patch: jest.fn(),
      delete: jest.fn(),
    };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.post.mockResolvedValue({ data: { ...sample, value: "ghp_xyz" } });

    const created = await useSecretsStore
      .getState()
      .createSecret("proj-1", "GITHUB_TOKEN", "ghp_xyz", "review token");

    expect(created?.name).toBe("GITHUB_TOKEN");
    expect(useSecretsStore.getState().lastRevealedValue).toEqual({
      projectId: "proj-1",
      name: "GITHUB_TOKEN",
      value: "ghp_xyz",
    });

    useSecretsStore.getState().consumeRevealedValue();
    expect(useSecretsStore.getState().lastRevealedValue).toBeNull();
  });

  it("surfaces rotated value through lastRevealedValue", async () => {
    const api = {
      get: jest.fn(),
      post: jest.fn(),
      patch: jest.fn(),
      delete: jest.fn(),
    };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.patch.mockResolvedValue({
      data: { name: "GITHUB_TOKEN", value: "ghp_new" },
    });
    api.get.mockResolvedValue({ data: [sample] });

    await useSecretsStore
      .getState()
      .rotateSecret("proj-1", "GITHUB_TOKEN", "ghp_new");

    expect(useSecretsStore.getState().lastRevealedValue?.value).toBe("ghp_new");
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
    useSecretsStore.setState({
      secretsByProject: { "proj-1": [sample] },
      loadingByProject: {},
      lastRevealedValue: null,
    });

    await useSecretsStore.getState().deleteSecret("proj-1", "GITHUB_TOKEN");

    expect(useSecretsStore.getState().secretsByProject["proj-1"]).toEqual([]);
  });
});
