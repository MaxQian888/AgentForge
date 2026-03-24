import { render, screen } from "@testing-library/react";
import CostPage from "./page";

jest.mock("@/lib/stores/agent-store", () => ({
  useAgentStore: (selector: (s: { agents: unknown[] }) => unknown) =>
    selector({ agents: [] }),
}));

jest.mock("@/lib/stores/cost-store", () => ({
  useCostStore: (selector: (s: Record<string, unknown>) => unknown) =>
    selector({
      projectCost: null,
      loading: false,
      error: null,
      fetchProjectCost: jest.fn(),
      velocity: [],
      velocityLoading: false,
      agentPerformance: [],
      performanceLoading: false,
      fetchVelocity: jest.fn(),
      fetchAgentPerformance: jest.fn(),
    }),
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (s: Record<string, unknown>) => unknown) =>
    selector({ selectedProjectId: null }),
}));

jest.mock("@/components/cost/cost-chart", () => ({
  CostChart: () => <div data-testid="cost-chart" />,
}));

jest.mock("@/components/cost/velocity-chart", () => ({
  VelocityChart: () => <div data-testid="velocity-chart" />,
}));

jest.mock("@/components/cost/agent-performance-table", () => ({
  AgentPerformanceTable: () => <div data-testid="agent-performance-table" />,
}));

describe("CostPage", () => {
  it("renders the cost overview heading and summary cards", () => {
    render(<CostPage />);
    expect(screen.getByText("Cost Overview")).toBeInTheDocument();
    expect(screen.getByText("Total Spend")).toBeInTheDocument();
    expect(screen.getByText("Input Tokens")).toBeInTheDocument();
    expect(screen.getByText("Output Tokens")).toBeInTheDocument();
    expect(screen.getByText("Active Agents")).toBeInTheDocument();
  });
});
