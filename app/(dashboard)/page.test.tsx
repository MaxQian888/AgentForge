import { Children, isValidElement, type ReactNode, type ReactElement } from "react";

jest.mock("@/components/ui/select", () => {
  function flattenOptions(children: ReactNode): Array<{ value: string; label: string }> {
    const options: Array<{ value: string; label: string }> = [];
    function visit(node: ReactNode) {
      Children.forEach(node, (child) => {
        if (!isValidElement(child)) return;
        const element = child as ReactElement<{ children?: ReactNode; value?: string }>;
        if (element.props.value !== undefined) {
          options.push({
            value: element.props.value,
            label: typeof element.props.children === "string" ? element.props.children : String(element.props.value),
          });
          return;
        }
        visit(element.props.children);
      });
    }
    visit(children);
    return options;
  }

  return {
    Select: ({ value, onValueChange, children }: { value?: string; onValueChange?: (v: string) => void; children?: ReactNode }) => {
      const options = flattenOptions(children);
      let ariaLabel: string | undefined;
      Children.forEach(children, (child) => {
        if (!isValidElement(child)) return;
        const el = child as ReactElement<{ "aria-label"?: string }>;
        if (el.props["aria-label"]) ariaLabel = el.props["aria-label"];
      });
      return (
        <select aria-label={ariaLabel} value={value} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => onValueChange?.(e.target.value)}>
          {options.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
      );
    },
    SelectTrigger: ({ children }: { children?: ReactNode }) => <>{children}</>,
    SelectValue: () => null,
    SelectContent: ({ children }: { children?: ReactNode }) => <>{children}</>,
    SelectItem: ({ children }: { children?: ReactNode }) => <>{children}</>,
  };
});

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import DashboardPage from "./page";

const searchParamsState = {
  projectId: "project-1",
};
const replaceMock = jest.fn();

const dashboardState = {
  summary: {
    progress: {
      inProgress: 2,
      total: 5,
    },
    headline: {
      activeAgents: 3,
      pendingReviews: 1,
      weeklyCost: 42,
    },
    team: {
      totalMembers: 4,
    },
    scope: {
      projectId: "project-1",
    },
    links: {
      agents: "/agents/live",
    },
  },
  projects: [
    {
      id: "project-1",
      name: "AgentForge",
      slug: "agentforge",
      description: "Main project",
      repoUrl: "",
      defaultBranch: "main",
      createdAt: "2026-03-20T10:00:00.000Z",
    },
    {
      id: "project-2",
      name: "Ops",
      slug: "ops",
      description: "Ops project",
      repoUrl: "",
      defaultBranch: "main",
      createdAt: "2026-03-20T10:00:00.000Z",
    },
  ],
  loading: false,
  fetchSummary: jest.fn(),
  activity: [
    {
      id: "evt-1",
      type: "deploy-start",
      title: "Deploy started",
      createdAt: "2026-03-30T00:00:00.000Z",
    },
  ],
  members: [
    {
      id: "member-1",
      name: "Alice",
      role: "Lead",
      type: "human",
      isActive: true,
    },
  ],
  agents: [
    {
      memberId: "member-1",
    },
  ],
};

const agentState = {
  agents: [
    { id: "agent-1", status: "running" },
    { id: "agent-2", status: "paused" },
    { id: "agent-3", status: "offline" },
  ],
  fetchAgents: jest.fn(),
};

const costState = {
  projectCost: {
    budgetSummary: {
      allocated: 100,
      spent: 40,
      remaining: 60,
    },
  },
};

jest.mock("next-intl", () => ({
  useTranslations: (namespace?: string) => (key: string, values?: Record<string, string | number>) => {
    if (namespace === "dashboard" && key === "cards.members") {
      return `${values?.count ?? 0} members`;
    }
    if (namespace === "dashboard" && key === "teamHealth.active") {
      return "Active";
    }
    if (namespace === "dashboard" && key === "teamHealth.idle") {
      return "Idle";
    }
    return namespace ? `${namespace}.${key}` : key;
  },
}));

jest.mock("next/navigation", () => ({
  useRouter: () => ({
    push: jest.fn(),
    replace: replaceMock,
  }),
  usePathname: () => "/",
  useSearchParams: () => ({
    get: (key: string) => (key === "project" ? searchParamsState.projectId : null),
  }),
}));

jest.mock("@/hooks/use-breadcrumbs", () => ({
  useBreadcrumbs: jest.fn(),
}));

jest.mock("@/components/layout/templates", () => ({
  OverviewLayout: ({
    title,
    metrics,
    children,
  }: {
    title: string;
    metrics?: React.ReactNode;
    children: React.ReactNode;
  }) => (
    <div>
      <h1>{title}</h1>
      <div data-testid="overview-metrics">{metrics}</div>
      <div data-testid="overview-children">{children}</div>
    </div>
  ),
}));

jest.mock("@/components/shared/metric-card", () => ({
  MetricCard: ({
    label,
    value,
  }: {
    label: string;
    value: string;
  }) => <div data-testid={`metric-${label}`}>{value}</div>,
}));

jest.mock("@/components/dashboard/activity-feed", () => ({
  ActivityFeed: ({ events }: { events: Array<{ type: string; status: string }> }) => (
    <div data-testid="activity-feed">
      {events.map((event) => `${event.type}:${event.status}`).join(",")}
    </div>
  ),
}));

jest.mock("@/components/dashboard/agent-fleet-widget", () => ({
  AgentFleetWidget: ({ agents }: { agents: Array<{ id: string }> }) => (
    <div data-testid="agent-fleet-widget">{agents.map((agent) => agent.id).join(",")}</div>
  ),
}));

jest.mock("@/components/dashboard/team-health-widget", () => ({
  TeamHealthWidget: ({
    members,
  }: {
    members: Array<{ name: string; status: string; role: string }>;
  }) => (
    <div data-testid="team-health-widget">
      {members.map((member) => `${member.name}:${member.status}:${member.role}`).join(",")}
    </div>
  ),
}));

jest.mock("@/components/dashboard/budget-widget", () => ({
  BudgetWidget: ({
    totalBudget,
    spent,
    remaining,
  }: {
    totalBudget: number;
    spent: number;
    remaining: number;
  }) => (
    <div data-testid="budget-widget">{`${totalBudget}/${spent}/${remaining}`}</div>
  ),
}));

jest.mock("@/components/ui/skeleton", () => ({
  Skeleton: () => <div data-testid="skeleton" />,
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (state: typeof dashboardState) => unknown) => selector(dashboardState),
}));

jest.mock("@/lib/stores/agent-store", () => ({
  useAgentStore: (selector: (state: typeof agentState) => unknown) => selector(agentState),
}));

jest.mock("@/lib/stores/cost-store", () => ({
  useCostStore: (selector: (state: typeof costState) => unknown) => selector(costState),
}));

describe("DashboardPage", () => {
  beforeEach(() => {
    searchParamsState.projectId = "project-1";
    dashboardState.loading = false;
    replaceMock.mockReset();
    dashboardState.fetchSummary.mockReset();
    agentState.fetchAgents.mockReset();
  });

  it("shows dashboard skeletons while the summary is loading", () => {
    dashboardState.loading = true;

    render(<DashboardPage />);

    expect(dashboardState.fetchSummary).toHaveBeenCalledWith({ projectId: "project-1" });
    expect(agentState.fetchAgents).toHaveBeenCalledTimes(1);
    expect(screen.getAllByTestId("skeleton")).toHaveLength(25);
    expect(screen.getAllByTestId("dashboard-widget-skeleton")).toHaveLength(4);
  });

  it("renders metrics, widgets, and quick-action links from the loaded summary", () => {
    render(<DashboardPage />);

    expect(screen.getByRole("heading", { name: "dashboard.pageTitle" })).toBeInTheDocument();
    expect(screen.getByTestId("metric-dashboard.cards.taskProgress")).toHaveTextContent("2/5");
    expect(screen.getByTestId("metric-dashboard.cards.activeAgents")).toHaveTextContent("3");
    expect(screen.getByTestId("activity-feed")).toHaveTextContent("deploy-start:running");
    expect(screen.getByTestId("agent-fleet-widget")).toHaveTextContent("agent-1,agent-2");
    expect(screen.getByTestId("team-health-widget")).toHaveTextContent("Alice:Active:Lead");
    expect(screen.getByTestId("budget-widget")).toHaveTextContent("100/40/60");
    expect(screen.getByRole("link", { name: "dashboard.actions.createTask N" })).toHaveAttribute(
      "href",
      "/project?id=project-1",
    );
    expect(screen.getByRole("link", { name: "dashboard.actions.spawnAgent A" })).toHaveAttribute(
      "href",
      "/agents/live",
    );
    expect(screen.getByRole("link", { name: "dashboard.actions.newSprint S" })).toHaveAttribute(
      "href",
      "/sprints",
    );
    expect(screen.getByRole("link", { name: "dashboard.actions.createTeam T" })).toHaveAttribute(
      "href",
      "/team",
    );
    expect(screen.getByText("N")).toBeInTheDocument();
    expect(screen.getByText("A")).toBeInTheDocument();
    expect(screen.getByText("S")).toBeInTheDocument();
    expect(screen.getByText("T")).toBeInTheDocument();
  });

  it("shows a project selector and updates the query string when the project changes", async () => {
    const user = userEvent.setup();

    render(<DashboardPage />);

    expect(screen.getByLabelText("dashboard.projectFilterLabel")).toHaveValue(
      "project-1",
    );

    await user.selectOptions(
      screen.getByLabelText("dashboard.projectFilterLabel"),
      "project-2",
    );

    expect(replaceMock).toHaveBeenCalledWith("/?project=project-2");

    await user.selectOptions(
      screen.getByLabelText("dashboard.projectFilterLabel"),
      "__all__",
    );

    expect(replaceMock).toHaveBeenCalledWith("/");
  });

  it("maps activity statuses, idle members, and fallback links when budget data is missing", () => {
    dashboardState.summary.scope.projectId = null as unknown as string;
    dashboardState.summary.links.agents = undefined as unknown as string;
    dashboardState.activity = [
      {
        id: "evt-1",
        type: "lint-fail",
        title: "Lint failed",
        createdAt: "2026-03-30T00:00:00.000Z",
      },
      {
        id: "evt-2",
        type: "deploy-complete",
        title: "Deploy complete",
        createdAt: "2026-03-30T00:05:00.000Z",
      },
      {
        id: "evt-3",
        type: "boot-start",
        title: "Boot start",
        createdAt: "2026-03-30T00:06:00.000Z",
      },
      {
        id: "evt-4",
        type: "queued",
        title: "Queued",
        createdAt: "2026-03-30T00:07:00.000Z",
      },
    ];
    dashboardState.members = [
      {
        id: "member-2",
        name: "Bot Worker",
        role: "",
        type: "agent",
        isActive: false,
      },
    ];
    dashboardState.agents = [
      {
        memberId: "member-2",
      },
    ];
    costState.projectCost = null as unknown as typeof costState.projectCost;

    render(<DashboardPage />);

    expect(screen.getByTestId("activity-feed")).toHaveTextContent(
      "lint-fail:failed,deploy-complete:completed,boot-start:running,queued:pending",
    );
    expect(screen.getByTestId("team-health-widget")).toHaveTextContent("Bot Worker:Idle:Agent");
    expect(screen.getByTestId("budget-widget")).toHaveTextContent("0/42/0");
    expect(screen.getByRole("link", { name: "dashboard.actions.createTask N" })).toHaveAttribute(
      "href",
      "/projects",
    );
    expect(screen.getByRole("link", { name: "dashboard.actions.spawnAgent A" })).toHaveAttribute(
      "href",
      "/agents",
    );

    dashboardState.summary.scope.projectId = "project-1";
    dashboardState.summary.links.agents = "/agents/live";
    dashboardState.activity = [
      {
        id: "evt-1",
        type: "deploy-start",
        title: "Deploy started",
        createdAt: "2026-03-30T00:00:00.000Z",
      },
    ];
    dashboardState.members = [
      {
        id: "member-1",
        name: "Alice",
        role: "Lead",
        type: "human",
        isActive: true,
      },
    ];
    dashboardState.agents = [
      {
        memberId: "member-1",
      },
    ];
    costState.projectCost = {
      budgetSummary: {
        allocated: 100,
        spent: 40,
        remaining: 60,
      },
    };
  });
});
