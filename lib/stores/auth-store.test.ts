import { act } from "@testing-library/react";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";

jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(),
}));

jest.mock("@/lib/backend-url", () => ({
  resolveBackendUrl: jest.fn(async () => "http://localhost:7777"),
}));

type MockApiClient = {
  get: jest.Mock;
  post: jest.Mock;
};

function makeApiClient(): MockApiClient {
  return {
    get: jest.fn(),
    post: jest.fn(),
  };
}

function resetAuthStore() {
  localStorage.clear();
  useAuthStore.setState({
    accessToken: null,
    refreshToken: null,
    user: null,
    status: "idle",
  } as never);
}

describe("useAuthStore", () => {
  beforeEach(() => {
    resetAuthStore();
    jest.clearAllMocks();
  });

  it("stores the canonical auth session after login", async () => {
    const api = makeApiClient();
    api.post.mockResolvedValue({
      data: {
        accessToken: "access-123",
        refreshToken: "refresh-123",
        user: {
          id: "user-1",
          email: "test@example.com",
          name: "Test User",
        },
      },
      status: 200,
    });
    (createApiClient as jest.Mock).mockReturnValue(api);

    await act(async () => {
      await useAuthStore.getState().login("test@example.com", "password123");
    });

    expect(useAuthStore.getState()).toMatchObject({
      accessToken: "access-123",
      refreshToken: "refresh-123",
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Test User",
      },
      status: "authenticated",
    });
  });

  it("marks the store unauthenticated when login fails", async () => {
    const api = makeApiClient();
    api.post.mockRejectedValueOnce(new Error("bad credentials"));
    (createApiClient as jest.Mock).mockReturnValue(api);

    await act(async () => {
      await expect(
        useAuthStore.getState().login("test@example.com", "wrong"),
      ).rejects.toThrow("bad credentials");
    });

    expect(useAuthStore.getState().status).toBe("unauthenticated");
  });

  it("stores the canonical auth session after registration", async () => {
    const api = makeApiClient();
    api.post.mockResolvedValueOnce({
      data: {
        accessToken: "access-register",
        refreshToken: "refresh-register",
        user: {
          id: "user-2",
          email: "new@example.com",
          name: "New User",
        },
      },
      status: 200,
    });
    (createApiClient as jest.Mock).mockReturnValue(api);

    await act(async () => {
      await useAuthStore.getState().register(
        "new@example.com",
        "password123",
        "New User",
      );
    });

    expect(useAuthStore.getState()).toMatchObject({
      accessToken: "access-register",
      refreshToken: "refresh-register",
      status: "authenticated",
    });
  });

  it("clears the session when logout completes or fails", async () => {
    const api = makeApiClient();
    api.post.mockRejectedValueOnce(new Error("logout failed"));
    (createApiClient as jest.Mock).mockReturnValue(api);

    useAuthStore.setState({
      accessToken: "access-123",
      refreshToken: "refresh-123",
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Test User",
      },
      status: "authenticated",
    } as never);

    await act(async () => {
      await expect(useAuthStore.getState().logout()).rejects.toThrow(
        "logout failed",
      );
    });

    expect(useAuthStore.getState()).toMatchObject({
      accessToken: null,
      refreshToken: null,
      user: null,
      status: "unauthenticated",
    });
  });

  it("bootstraps a stored session through refresh when identity validation returns unauthorized", async () => {
    const api = makeApiClient();
    api.get
      .mockRejectedValueOnce(Object.assign(new Error("unauthorized"), { status: 401 }))
      .mockResolvedValueOnce({
        data: {
          id: "user-1",
          email: "test@example.com",
          name: "Recovered User",
        },
        status: 200,
      });
    api.post.mockResolvedValueOnce({
      data: {
        accessToken: "access-2",
        refreshToken: "refresh-2",
        user: {
          id: "user-1",
          email: "test@example.com",
          name: "Recovered User",
        },
      },
      status: 200,
    });
    (createApiClient as jest.Mock).mockReturnValue(api);

    useAuthStore.setState({
      accessToken: "expired-access",
      refreshToken: "refresh-1",
      user: null,
      status: "idle",
    } as never);

    await act(async () => {
      await (useAuthStore.getState() as unknown as { bootstrapSession: () => Promise<void> }).bootstrapSession();
    });

    expect(api.post).toHaveBeenCalledWith("/api/v1/auth/refresh", {
      refreshToken: "refresh-1",
    });
    expect(useAuthStore.getState()).toMatchObject({
      accessToken: "access-2",
      refreshToken: "refresh-2",
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Recovered User",
      },
      status: "authenticated",
    });
  });

  it("keeps an already-valid access token without refreshing", async () => {
    const api = makeApiClient();
    api.get.mockResolvedValueOnce({
      data: {
        id: "user-1",
        email: "test@example.com",
        name: "Existing User",
      },
      status: 200,
    });
    (createApiClient as jest.Mock).mockReturnValue(api);

    useAuthStore.setState({
      accessToken: "access-123",
      refreshToken: "refresh-123",
      user: null,
      status: "idle",
    } as never);

    await act(async () => {
      await useAuthStore.getState().bootstrapSession();
    });

    expect(api.post).not.toHaveBeenCalled();
    expect(useAuthStore.getState()).toMatchObject({
      user: {
        id: "user-1",
      },
      status: "authenticated",
    });
  });

  it("clears the session immediately when no tokens are present during bootstrap", async () => {
    const api = makeApiClient();
    (createApiClient as jest.Mock).mockReturnValue(api);

    useAuthStore.setState({
      accessToken: null,
      refreshToken: null,
      user: null,
      status: "idle",
    } as never);

    await act(async () => {
      await useAuthStore.getState().bootstrapSession();
    });

    expect(api.get).not.toHaveBeenCalled();
    expect(api.post).not.toHaveBeenCalled();
    expect(useAuthStore.getState().status).toBe("unauthenticated");
  });

  it("stops bootstrapping when a bootstrap request is already checking", async () => {
    const api = makeApiClient();
    (createApiClient as jest.Mock).mockReturnValue(api);

    useAuthStore.setState({
      accessToken: "access-123",
      refreshToken: "refresh-123",
      user: null,
      status: "checking",
    } as never);

    await act(async () => {
      await useAuthStore.getState().bootstrapSession();
    });

    expect(api.get).not.toHaveBeenCalled();
    expect(api.post).not.toHaveBeenCalled();
  });

  it("clears the session when access validation fails with a non-unauthorized error", async () => {
    const api = makeApiClient();
    api.get.mockRejectedValueOnce(new Error("network failed"));
    (createApiClient as jest.Mock).mockReturnValue(api);

    useAuthStore.setState({
      accessToken: "access-123",
      refreshToken: "refresh-123",
      user: null,
      status: "idle",
    } as never);

    await act(async () => {
      await useAuthStore.getState().bootstrapSession();
    });

    expect(api.post).not.toHaveBeenCalled();
    expect(useAuthStore.getState().status).toBe("unauthenticated");
  });

  it("clears a stale stored session when refresh fails during bootstrap", async () => {
    const api = makeApiClient();
    api.get.mockRejectedValueOnce(
      Object.assign(new Error("unauthorized"), { status: 401 })
    );
    api.post.mockRejectedValueOnce(
      Object.assign(new Error("refresh failed"), { status: 401 })
    );
    (createApiClient as jest.Mock).mockReturnValue(api);

    useAuthStore.setState({
      accessToken: "expired-access",
      refreshToken: "refresh-1",
      user: {
        id: "stale-user",
        email: "stale@example.com",
        name: "Stale User",
      },
      status: "idle",
    } as never);

    await act(async () => {
      await (useAuthStore.getState() as unknown as { bootstrapSession: () => Promise<void> }).bootstrapSession();
    });

    expect(useAuthStore.getState()).toMatchObject({
      accessToken: null,
      refreshToken: null,
      user: null,
      status: "unauthenticated",
    });
  });

  it("returns the access token and clears session state on demand", () => {
    useAuthStore.setState({
      accessToken: "access-123",
      refreshToken: "refresh-123",
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Test User",
      },
      status: "authenticated",
    } as never);

    expect(useAuthStore.getState().getAccessToken()).toBe("access-123");

    act(() => {
      useAuthStore.getState().clearSession();
    });

    expect(useAuthStore.getState()).toMatchObject({
      accessToken: null,
      refreshToken: null,
      user: null,
      status: "unauthenticated",
    });
  });
});
