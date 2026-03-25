jest.mock("@/lib/platform-runtime", () => ({
  platformRuntime: {
    defaultBackendUrl: "http://desktop.local:9000",
    resolveBackendUrl: jest.fn(),
  },
}));

import { platformRuntime } from "@/lib/platform-runtime";
import { getDefaultBackendUrl, resolveBackendUrl } from "./backend-url";

describe("backend-url", () => {
  const mockPlatformRuntime = platformRuntime as unknown as {
    defaultBackendUrl: string;
    resolveBackendUrl: jest.Mock;
  };

  beforeEach(() => {
    mockPlatformRuntime.resolveBackendUrl.mockReset();
  });

  it("reads the default backend URL from the shared platform runtime", () => {
    expect(getDefaultBackendUrl()).toBe("http://desktop.local:9000");
  });

  it("delegates backend URL resolution to the shared platform runtime", async () => {
    mockPlatformRuntime.resolveBackendUrl.mockResolvedValueOnce(
      "http://runtime.local:7777",
    );

    await expect(resolveBackendUrl()).resolves.toBe("http://runtime.local:7777");
    expect(mockPlatformRuntime.resolveBackendUrl).toHaveBeenCalledTimes(1);
  });
});
