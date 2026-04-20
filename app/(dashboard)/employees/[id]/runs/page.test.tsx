import { render, screen, waitFor } from "@testing-library/react";
import EmployeeRunsPage from "./page";
import { useEmployeeRunsStore } from "@/lib/stores/employee-runs-store";

jest.mock("next/navigation", () => ({
  useParams: () => ({ id: "emp-1" }),
}));

const seededItems = [
  {
    kind: "workflow",
    id: "exec-1",
    name: "echo-flow",
    status: "completed",
    startedAt: "2026-04-20T10:00:00Z",
    completedAt: "2026-04-20T10:00:45Z",
    durationMs: 45000,
    refUrl: "/workflow/runs/exec-1",
  },
  {
    kind: "agent",
    id: "run-1",
    name: "code-reviewer",
    status: "running",
    startedAt: "2026-04-20T10:01:00Z",
    refUrl: "/agents?run=run-1",
  },
];

jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(() => ({
    get: jest.fn().mockResolvedValue({
      data: { items: seededItems, page: 1, size: 20, kind: "all" },
    }),
  })),
}));

jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: { getState: () => ({ accessToken: "test-token" }) },
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

describe("EmployeeRunsPage", () => {
  beforeEach(() => {
    useEmployeeRunsStore.setState({
      runsByEmployee: {
        "emp-1": [
          {
            kind: "workflow",
            id: "exec-1",
            name: "echo-flow",
            status: "completed",
            startedAt: "2026-04-20T10:00:00Z",
            completedAt: "2026-04-20T10:00:45Z",
            durationMs: 45000,
            refUrl: "/workflow/runs/exec-1",
          },
          {
            kind: "agent",
            id: "run-1",
            name: "code-reviewer",
            status: "running",
            startedAt: "2026-04-20T10:01:00Z",
            refUrl: "/agents?run=run-1",
          },
        ],
      },
      loadingByEmployee: { "emp-1": false },
      pageByEmployee: { "emp-1": 1 },
      hasMoreByEmployee: { "emp-1": false },
      kindByEmployee: { "emp-1": "all" },
    });
  });

  it("renders both row kinds with drill-down links", async () => {
    render(<EmployeeRunsPage />);
    await waitFor(() =>
      expect(screen.getByText("echo-flow")).toBeInTheDocument(),
    );
    expect(screen.getByText("code-reviewer")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /echo-flow/ })).toHaveAttribute(
      "href",
      "/workflow/runs/exec-1",
    );
    expect(screen.getByRole("link", { name: /code-reviewer/ })).toHaveAttribute(
      "href",
      "/agents?run=run-1",
    );
  });
});
