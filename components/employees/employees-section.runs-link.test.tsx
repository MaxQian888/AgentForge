const seededEmployee = {
  id: "emp-1",
  projectId: "proj-1",
  name: "ping",
  roleId: "code-reviewer",
  state: "active" as const,
  createdAt: "2026-04-20T00:00:00Z",
  updatedAt: "2026-04-20T00:00:00Z",
};

jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(() => ({
    get: jest.fn().mockResolvedValue({ data: [seededEmployee] }),
    post: jest.fn(),
    patch: jest.fn(),
    delete: jest.fn(),
  })),
}));

jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: { getState: () => ({ accessToken: "t" }) },
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

import { render, screen, waitFor } from "@testing-library/react";
import { EmployeesSection } from "./employees-section";
import { useEmployeeStore } from "@/lib/stores/employee-store";

describe("EmployeesSection runs link", () => {
  beforeEach(() => {
    useEmployeeStore.setState({
      employeesByProject: {
        "proj-1": [
          {
            id: "emp-1",
            projectId: "proj-1",
            name: "ping",
            roleId: "code-reviewer",
            state: "active",
            createdAt: "2026-04-20T00:00:00Z",
            updatedAt: "2026-04-20T00:00:00Z",
          },
        ],
      },
      loadingByProject: { "proj-1": false },
    } as never);
  });

  it("renders a Runs link on each employee row", async () => {
    render(<EmployeesSection projectId="proj-1" />);
    const link = await waitFor(() =>
      screen.getByRole("link", { name: /Runs/ }),
    );
    expect(link).toHaveAttribute("href", "/employees/emp-1/runs");
  });
});
