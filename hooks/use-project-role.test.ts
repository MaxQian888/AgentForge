import { renderHook, waitFor } from "@testing-library/react";

const mockStoreState = {
  byProject: {} as Record<
    string,
    { projectId: string; projectRole: string; allowedActions: string[] }
  >,
  loadingByProject: {} as Record<string, boolean>,
  errorByProject: {} as Record<string, string | null>,
  fetchPermissions: jest.fn().mockResolvedValue(null),
  invalidate: jest.fn(),
};

function mockStore(selector?: (s: typeof mockStoreState) => unknown) {
  return selector ? selector(mockStoreState) : mockStoreState;
}
mockStore.getState = () => mockStoreState;

jest.mock("@/lib/stores/project-permissions-store", () => ({
  useProjectPermissionsStore: mockStore,
}));

// The global jest.setup.ts mocks @/hooks/use-project-role with a permissive
// default. We need to unmock it so we can test the real hook.
jest.unmock("@/hooks/use-project-role");

import { useProjectRole } from "./use-project-role";

describe("useProjectRole", () => {
  beforeEach(() => {
    mockStoreState.byProject = {};
    mockStoreState.loadingByProject = {};
    mockStoreState.errorByProject = {};
    mockStoreState.fetchPermissions = jest.fn().mockResolvedValue(null);
    mockStoreState.invalidate = jest.fn();
    jest.clearAllMocks();
  });

  it("returns default values when projectId is null", () => {
    const { result } = renderHook(() => useProjectRole(null));

    expect(result.current.projectRole).toBeNull();
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.can("project.read")).toBe(false);
  });

  it("returns cached permissions without fetching", () => {
    mockStoreState.byProject = {
      p1: {
        projectId: "p1",
        projectRole: "admin",
        allowedActions: ["project.read", "project.update"],
      },
    };

    const { result } = renderHook(() => useProjectRole("p1"));

    expect(result.current.projectRole).toBe("admin");
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.can("project.read")).toBe(true);
    expect(result.current.can("project.delete")).toBe(false);
  });

  it("triggers fetch when permissions are not cached", () => {
    renderHook(() => useProjectRole("p1"));

    expect(mockStoreState.fetchPermissions).toHaveBeenCalledWith("p1");
  });

  it("reports loading state from the store", () => {
    mockStoreState.loadingByProject = { p1: true };

    const { result } = renderHook(() => useProjectRole("p1"));

    expect(result.current.loading).toBe(true);
  });

  it("reports error state from the store", () => {
    mockStoreState.errorByProject = { p1: "network error" };

    const { result } = renderHook(() => useProjectRole("p1"));

    expect(result.current.error).toBe("network error");
    expect(result.current.can("project.read")).toBe(false);
  });

  it("does not refetch when already loading", () => {
    mockStoreState.loadingByProject = { p1: true };

    renderHook(() => useProjectRole("p1"));

    expect(mockStoreState.fetchPermissions).not.toHaveBeenCalled();
  });

  it("does not refetch when an error is already present", () => {
    mockStoreState.errorByProject = { p1: "previous error" };

    renderHook(() => useProjectRole("p1"));

    expect(mockStoreState.fetchPermissions).not.toHaveBeenCalled();
  });

  it("refresh invalidates and re-fetches permissions", async () => {
    mockStoreState.byProject = {
      p1: {
        projectId: "p1",
        projectRole: "editor",
        allowedActions: ["project.read"],
      },
    };

    const { result } = renderHook(() => useProjectRole("p1"));

    await result.current.refresh();

    expect(mockStoreState.invalidate).toHaveBeenCalledWith("p1");
    expect(mockStoreState.fetchPermissions).toHaveBeenCalledWith("p1");
  });

  it("refresh is a no-op when projectId is null", async () => {
    const { result } = renderHook(() => useProjectRole(null));

    await result.current.refresh();

    expect(mockStoreState.invalidate).not.toHaveBeenCalled();
    expect(mockStoreState.fetchPermissions).not.toHaveBeenCalled();
  });

  it("falls back to false for can() when permissions are loading", () => {
    mockStoreState.loadingByProject = { p1: true };

    const { result } = renderHook(() => useProjectRole("p1"));

    // Never grants access optimistically while loading.
    expect(result.current.can("project.read")).toBe(false);
  });
});
