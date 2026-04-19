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
import { useEmployeeStore, type Employee } from "./employee-store";

const sampleEmployee: Employee = {
  id: "emp-1",
  projectId: "project-1",
  name: "default-code-reviewer",
  roleId: "code-reviewer",
  state: "active",
  createdAt: "2026-04-19T00:00:00Z",
  updatedAt: "2026-04-19T00:00:00Z",
};

describe("useEmployeeStore", () => {
  beforeEach(() => {
    useEmployeeStore.setState({
      employeesByProject: {},
      loadingByProject: {},
    });
    jest.clearAllMocks();
  });

  it("fetches employees into the project slot", async () => {
    const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({ data: [sampleEmployee] });

    await useEmployeeStore.getState().fetchEmployees("project-1");

    expect(api.get).toHaveBeenCalledWith(
      "/api/v1/projects/project-1/employees",
      { token: "test-token" },
    );
    expect(useEmployeeStore.getState().employeesByProject["project-1"]).toEqual([sampleEmployee]);
  });

  it("passes state filter as query string", async () => {
    const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({ data: [] });

    await useEmployeeStore.getState().fetchEmployees("project-1", { state: "paused" });

    expect(api.get).toHaveBeenCalledWith(
      "/api/v1/projects/project-1/employees?state=paused",
      { token: "test-token" },
    );
  });

  it("prepends newly created employees to the project slot", async () => {
    const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.post.mockResolvedValue({ data: sampleEmployee });

    const created = await useEmployeeStore.getState().createEmployee("project-1", {
      name: "default-code-reviewer",
      roleId: "code-reviewer",
    });

    expect(created).toEqual(sampleEmployee);
    expect(useEmployeeStore.getState().employeesByProject["project-1"]).toEqual([sampleEmployee]);
  });

  it("updates in-memory state after SetState call", async () => {
    const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.post.mockResolvedValue({ data: null });
    useEmployeeStore.setState({
      employeesByProject: { "project-1": [sampleEmployee] },
      loadingByProject: {},
    });

    await useEmployeeStore.getState().setState("project-1", sampleEmployee.id, "paused");

    expect(api.post).toHaveBeenCalledWith(
      `/api/v1/projects/project-1/employees/${sampleEmployee.id}/state`,
      { state: "paused" },
      { token: "test-token" },
    );
    expect(useEmployeeStore.getState().employeesByProject["project-1"][0].state).toBe("paused");
  });

  it("removes employees from local state after delete", async () => {
    const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.delete.mockResolvedValue({ data: null });
    useEmployeeStore.setState({
      employeesByProject: { "project-1": [sampleEmployee] },
      loadingByProject: {},
    });

    await useEmployeeStore.getState().deleteEmployee("project-1", sampleEmployee.id);

    expect(useEmployeeStore.getState().employeesByProject["project-1"]).toEqual([]);
  });
});
