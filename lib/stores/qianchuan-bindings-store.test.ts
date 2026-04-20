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
import { useQianchuanBindingsStore } from "./qianchuan-bindings-store";

const sampleWire = {
  id: "b1",
  project_id: "p1",
  advertiser_id: "A1",
  aweme_id: "W1",
  display_name: "店铺A",
  status: "active",
  access_token_secret_ref: "qc.access",
  refresh_token_secret_ref: "qc.refresh",
};

describe("useQianchuanBindingsStore", () => {
  beforeEach(() => {
    useQianchuanBindingsStore.setState({ byProject: {}, loading: {} });
    jest.clearAllMocks();
  });

  it("loads bindings into byProject (camelCase mapping)", async () => {
    const api = {
      get: jest.fn(),
      post: jest.fn(),
      patch: jest.fn(),
      delete: jest.fn(),
    };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({ data: [sampleWire] });

    await useQianchuanBindingsStore.getState().fetchBindings("p1");

    expect(api.get).toHaveBeenCalledWith(
      "/api/v1/projects/p1/qianchuan/bindings",
      { token: "test-token" },
    );
    const rows = useQianchuanBindingsStore.getState().byProject["p1"];
    expect(rows).toHaveLength(1);
    expect(rows[0].advertiserId).toBe("A1");
    expect(rows[0].accessTokenSecretRef).toBe("qc.access");
  });

  it("posts snake_case body on createBinding", async () => {
    const api = {
      get: jest.fn(),
      post: jest.fn(),
      patch: jest.fn(),
      delete: jest.fn(),
    };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.post.mockResolvedValue({ data: sampleWire });
    api.get.mockResolvedValue({ data: [sampleWire] });

    const out = await useQianchuanBindingsStore.getState().createBinding("p1", {
      advertiserId: "A1",
      awemeId: "W1",
      displayName: "店铺A",
      accessTokenSecretRef: "qc.access",
      refreshTokenSecretRef: "qc.refresh",
    });

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/projects/p1/qianchuan/bindings",
      expect.objectContaining({
        advertiser_id: "A1",
        aweme_id: "W1",
        display_name: "店铺A",
        access_token_secret_ref: "qc.access",
        refresh_token_secret_ref: "qc.refresh",
      }),
      { token: "test-token" },
    );
    expect(out?.advertiserId).toBe("A1");
  });

  it("returns ok=true on testBinding success", async () => {
    const api = {
      get: jest.fn(),
      post: jest.fn(),
      patch: jest.fn(),
      delete: jest.fn(),
    };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.post.mockResolvedValue({ data: {} });

    const result = await useQianchuanBindingsStore
      .getState()
      .testBinding("b1");

    expect(result).toEqual({ ok: true });
    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/qianchuan/bindings/b1/test",
      {},
      { token: "test-token" },
    );
  });
});
