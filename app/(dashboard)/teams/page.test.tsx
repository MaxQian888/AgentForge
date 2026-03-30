import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import TeamsPage from "./page";
import { Children, isValidElement, type ReactElement, type ReactNode } from "react";

type AgentTeam = import("@/lib/stores/team-store").AgentTeam;

function createMockTeam(
  overrides: Partial<AgentTeam> & Pick<AgentTeam, "id" | "name" | "status">,
): AgentTeam {
  const { id, name, status, ...rest } = overrides;
  return {
    id,
    projectId: "project-1",
    taskId: "task-1",
    taskTitle: "Task",
    name,
    status,
    strategy: "plan-code-review",
    runtime: "codex",
    provider: "openai",
    model: "gpt-5.4",
    plannerRunId: undefined,
    reviewerRunId: undefined,
    coderRunIds: [],
    totalBudget: 24,
    totalSpent: 0,
    errorMessage: "",
    createdAt: "2026-03-25T10:00:00.000Z",
    updatedAt: "2026-03-25T10:00:00.000Z",
    ...rest,
  };
}

const replace = jest.fn();
const fetchTeams = jest.fn();
const deleteTeam = jest.fn();
const searchParamsState = {
  project: "project-1" as string | null,
  action: null as string | null,
};

const dashboardState = {
  projects: [
    { id: "project-1", name: "AgentForge" },
    { id: "project-2", name: "Bridge" },
  ],
  selectedProjectId: "project-2" as string | null,
};

const teamState = {
  teams: [] as AgentTeam[],
  loading: false,
  error: "failed to list teams" as string | null,
  fetchTeams,
  deleteTeam,
};

jest.mock("next/navigation", () => ({
  usePathname: () => "/teams",
  useRouter: () => ({ replace }),
  useSearchParams: () => ({
    get: (key: string) =>
      key === "project"
        ? searchParamsState.project
        : key === "action"
          ? searchParamsState.action
          : null,
    toString: () => {
      const params = new URLSearchParams();
      if (searchParamsState.project) {
        params.set("project", searchParamsState.project);
      }
      if (searchParamsState.action) {
        params.set("action", searchParamsState.action);
      }
      return params.toString();
    },
  }),
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (state: typeof dashboardState) => unknown) => selector(dashboardState),
}));

jest.mock("@/lib/stores/team-store", () => ({
  useTeamStore: (selector?: (state: typeof teamState) => unknown) =>
    selector ? selector(teamState) : teamState,
}));

type SelectMockProps = {
  value?: string;
  onValueChange?: (value: string) => void;
  children?: ReactNode;
};

type SelectItemElement = ReactElement<{ value?: string; children?: ReactNode }>;

function readOptionLabel(node: ReactNode): string {
  if (typeof node === "string") {
    return node;
  }
  if (typeof node === "number") {
    return String(node);
  }
  return "";
}

jest.mock("@/components/ui/select", () => ({
  Select: ({ value, onValueChange, children }: SelectMockProps) => {
    const options: Array<{ value: string; label: string }> = [];
    Children.forEach(children, (child) => {
      if (!isValidElement(child)) return;
      const contentChildren = (child as ReactElement<{ children?: ReactNode }>).props.children;
      Children.forEach(contentChildren, (grandChild) => {
        if (!isValidElement(grandChild)) return;
        const item = grandChild as SelectItemElement;
        if (item.props.value === undefined) return;
        options.push({
          value: item.props.value,
          label: readOptionLabel(item.props.children),
        });
      });
    });

    return (
      <select value={value} onChange={(event) => onValueChange?.(event.target.value)}>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    );
  },
  SelectTrigger: ({ children }: { children?: ReactNode }) => <>{children}</>,
  SelectValue: () => null,
  SelectContent: ({ children }: { children?: ReactNode }) => <>{children}</>,
  SelectItem: ({ children }: { children?: ReactNode }) => <>{children}</>,
}));

jest.mock("@/components/team/team-creation-wizard", () => ({
  TeamCreationWizard: ({ open }: { open: boolean }) =>
    open ? <div data-testid="team-creation-wizard" /> : null,
}));

jest.mock("@/components/team/team-card", () => ({
  TeamCard: ({
    team,
    onDelete,
  }: {
    team: { id: string; name: string };
    onDelete: (id: string) => Promise<void>;
  }) => (
    <div>
      <span>{team.name}</span>
      <button type="button" onClick={() => void onDelete(team.id)}>
        delete-{team.id}
      </button>
    </div>
  ),
}));

jest.mock("@/components/ui/skeleton", () => ({
  Skeleton: () => <div data-testid="skeleton" />,
}));

describe("TeamsPage", () => {
  beforeEach(() => {
    replace.mockReset();
    fetchTeams.mockReset();
    deleteTeam.mockReset().mockResolvedValue(undefined);
    searchParamsState.project = "project-1";
    searchParamsState.action = null;
    teamState.error = "failed to list teams";
    teamState.loading = false;
    teamState.teams = [];
  });

  it("opens the team creation wizard from the current header action", async () => {
    const user = userEvent.setup();
    searchParamsState.action = "create";

    render(<TeamsPage />);

    await user.click(screen.getByRole("button", { name: /create team/i }));

    expect(screen.getByTestId("team-creation-wizard")).toBeInTheDocument();
  });

  it("loads team runs with an explicit project scope and exposes retry on failure", async () => {
    const user = userEvent.setup();
    render(<TeamsPage />);

    await waitFor(() => expect(fetchTeams).toHaveBeenCalledWith("project-1", undefined));
    expect(screen.getByText("failed to list teams")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /retry/i }));
    expect(fetchTeams).toHaveBeenLastCalledWith("project-1");
  });

  it("switches projects, filters active teams, and deletes a team card", async () => {
    const user = userEvent.setup();
    teamState.error = null;
    teamState.teams = [
      createMockTeam({
        id: "team-1",
        name: "Execution Team",
        status: "planning",
        totalSpent: 12,
      }),
      createMockTeam({
        id: "team-2",
        name: "Archive Team",
        status: "completed",
        totalSpent: 8,
      }),
    ];

    render(<TeamsPage />);

    const [projectSelect, statusSelect] = screen.getAllByRole("combobox");
    await user.selectOptions(projectSelect, "project-2");
    expect(replace).toHaveBeenCalledWith("/teams?project=project-2");

    await user.selectOptions(statusSelect, "active");
    expect(screen.getByText("Execution Team")).toBeInTheDocument();
    expect(screen.queryByText("Archive Team")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "delete-team-1" }));
    expect(deleteTeam).toHaveBeenCalledWith("team-1");
  });

  it("shows the select-project empty state when no project scope is available", () => {
    searchParamsState.project = null;
    dashboardState.selectedProjectId = null;
    teamState.error = null;

    render(<TeamsPage />);

    expect(screen.getByText("Select a project to inspect team runs.")).toBeInTheDocument();

    dashboardState.selectedProjectId = "project-2";
  });

  it("renders loading and empty filtered states for team listings", async () => {
    const user = userEvent.setup();
    teamState.error = null;
    teamState.loading = true;

    const { rerender } = render(<TeamsPage />);

    expect(screen.getAllByTestId("skeleton").length).toBeGreaterThan(0);

    teamState.loading = false;
    teamState.teams = [
      createMockTeam({
        id: "team-2",
        name: "Archive Team",
        status: "completed",
        totalSpent: 8,
      }),
    ];
    rerender(<TeamsPage />);

    await user.selectOptions(screen.getAllByRole("combobox")[1], "active");
    expect(screen.getByText("No active teams found.")).toBeInTheDocument();

    teamState.teams = [];
    rerender(<TeamsPage />);
    await user.selectOptions(screen.getAllByRole("combobox")[1], "all");
    expect(screen.getByText("No agent teams yet. Start a team from a task detail page.")).toBeInTheDocument();
  });
});
