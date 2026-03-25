import { renderHook, waitFor } from "@testing-library/react";
import { getDefaultBackendUrl, resolveBackendUrl } from "@/lib/backend-url";
import { useBackendUrl } from "./use-backend-url";

jest.mock("@/lib/backend-url", () => ({
  getDefaultBackendUrl: jest.fn(),
  resolveBackendUrl: jest.fn(),
}));

const mockedGetDefaultBackendUrl = jest.mocked(getDefaultBackendUrl);
const mockedResolveBackendUrl = jest.mocked(resolveBackendUrl);

describe("useBackendUrl", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("returns the default backend url first and then updates with the resolved value", async () => {
    mockedGetDefaultBackendUrl.mockReturnValue("http://localhost:7777");
    mockedResolveBackendUrl.mockResolvedValue("http://127.0.0.1:7788");

    const { result } = renderHook(() => useBackendUrl());

    expect(result.current).toBe("http://localhost:7777");
    expect(mockedGetDefaultBackendUrl).toHaveBeenCalledTimes(1);
    expect(mockedResolveBackendUrl).toHaveBeenCalledTimes(1);

    await waitFor(() => {
      expect(result.current).toBe("http://127.0.0.1:7788");
    });
  });

  it("keeps the default backend url and warns when resolution fails", async () => {
    const error = new Error("desktop bridge offline");
    const warnSpy = jest.spyOn(console, "warn").mockImplementation(() => {});

    mockedGetDefaultBackendUrl.mockReturnValue("http://localhost:7777");
    mockedResolveBackendUrl.mockRejectedValue(error);

    const { result } = renderHook(() => useBackendUrl());

    await waitFor(() => {
      expect(mockedResolveBackendUrl).toHaveBeenCalledTimes(1);
    });

    expect(result.current).toBe("http://localhost:7777");
    expect(warnSpy).toHaveBeenCalledWith(
      "Failed to resolve backend URL:",
      error,
    );

    warnSpy.mockRestore();
  });
});
