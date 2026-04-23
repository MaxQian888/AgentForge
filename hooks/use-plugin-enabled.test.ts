import { act, renderHook, waitFor } from "@testing-library/react";
import { usePluginEnabled } from "./use-plugin-enabled";

jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({ accessToken: "test-token" }),
  },
}));

jest.mock("@/lib/plugins/enabled", () => ({
  isPluginEnabled: (flag: string | undefined) => {
    if (flag == null) return true;
    return !["0", "false", "no", "off", "disabled"].includes(flag.trim().toLowerCase());
  },
}));

describe("usePluginEnabled", () => {
  let mockFetch: jest.Mock;

  beforeEach(() => {
    mockFetch = jest.fn();
    global.fetch = mockFetch as unknown as typeof fetch;
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("short-circuits to disabled when buildTimeOverride is disabled", () => {
    const { result } = renderHook(() =>
      usePluginEnabled("test-plugin", "disabled"),
    );

    expect(result.current.loading).toBe(false);
    expect(result.current.enabled).toBe(false);
    expect(result.current.lifecycleState).toBeNull();
    expect(result.current.error).toBeNull();
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("short-circuits to disabled when buildTimeOverride is '0'", () => {
    const { result } = renderHook(() =>
      usePluginEnabled("test-plugin", "0"),
    );

    expect(result.current.enabled).toBe(false);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("starts loading and fetches when no buildTimeOverride is provided", () => {
    mockFetch.mockResolvedValue({
      status: 200,
      ok: true,
      json: async () => ({ enabled: true, lifecycle_state: "running" }),
    } as Response);

    const { result } = renderHook(() => usePluginEnabled("test-plugin"));

    expect(result.current.loading).toBe(true);
    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/plugins/test-plugin/status",
      expect.objectContaining({
        headers: expect.objectContaining({
          Accept: "application/json",
          Authorization: "Bearer test-token",
        }),
        credentials: "include",
      }),
    );
  });

  it("resolves to disabled on 404", async () => {
    mockFetch.mockResolvedValue({
      status: 404,
      ok: false,
    } as Response);

    const { result } = renderHook(() => usePluginEnabled("test-plugin"));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.enabled).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("resolves to disabled with error on non-ok non-404 status", async () => {
    mockFetch.mockResolvedValue({
      status: 500,
      ok: false,
    } as Response);

    const { result } = renderHook(() => usePluginEnabled("test-plugin"));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.enabled).toBe(false);
    expect(result.current.error).toBe("status 500");
  });

  it("resolves to enabled when backend returns enabled=true", async () => {
    mockFetch.mockResolvedValue({
      status: 200,
      ok: true,
      json: async () => ({ enabled: true, lifecycle_state: "active" }),
    } as Response);

    const { result } = renderHook(() => usePluginEnabled("test-plugin"));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.enabled).toBe(true);
    expect(result.current.lifecycleState).toBe("active");
    expect(result.current.error).toBeNull();
  });

  it("resolves to disabled when backend returns enabled=false", async () => {
    mockFetch.mockResolvedValue({
      status: 200,
      ok: true,
      json: async () => ({ enabled: false }),
    } as Response);

    const { result } = renderHook(() => usePluginEnabled("test-plugin"));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.enabled).toBe(false);
    expect(result.current.lifecycleState).toBeNull();
  });

  it("handles network errors gracefully", async () => {
    mockFetch.mockRejectedValue(new Error("network failure"));

    const { result } = renderHook(() => usePluginEnabled("test-plugin"));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.enabled).toBe(false);
    expect(result.current.error).toBe("network failure");
  });

  it("does not update state after unmount", async () => {
    mockFetch.mockImplementation(
      () =>
        new Promise((resolve) =>
          setTimeout(
            () =>
              resolve({
                status: 200,
                ok: true,
                json: async () => ({ enabled: true }),
              } as Response),
            100,
          ),
        ),
    );

    const { result, unmount } = renderHook(() => usePluginEnabled("test-plugin"));

    expect(result.current.loading).toBe(true);
    unmount();

    await act(async () => {
      await new Promise((r) => setTimeout(r, 150));
    });

    // After unmount, the state should not have changed (no React state update warning).
    expect(result.current.loading).toBe(true);
  });
});
