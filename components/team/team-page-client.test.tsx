import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeamPageClient } from "./team-page-client";
import type { TeamMember } from "@/lib/dashboard/summary";
import type { CreateMemberInput, UpdateMemberInput } from "@/lib/stores/member-store";

const replace = jest.fn();
const fetchSummary = jest.fn();
const createMember = jest.fn();
const updateMember = jest.fn();
const summarizeMemberRoster = jest.fn();
const teamManagementProjectsRefs: Array<Array<{ id: string; name: string }>> = [];

const roster: TeamMember[] = [
  {
    id: "member-1",
    projectId: "project-1",
    name: "Alice",
    type: "human",
    typeLabel: "Human",
    role: "frontend-developer",
    email: "alice@example.com",
    avatarUrl: "",
    skills: ["react"],
    isActive: true,
    status: "active",
    createdAt: "2026-03-24T08:00:00.000Z",
    lastActivityAt: "2026-03-24T09:00:00.000Z",
    workload: {
      assignedTasks: 1,
      inProgressTasks: 1,
      inReviewTasks: 0,
      activeAgentRuns: 0,
    },
  },
];

const dashboardState = {
  projects: [
    { id: "project-1", name: "AgentForge" },
    { id: "project-2", name: "Bridge" },
  ],
  selectedProjectId: "project-1",
  members: [
    {
      id: "member-1",
      projectId: "project-1",
      name: "Alice",
      type: "human",
      role: "frontend-developer",
      email: "alice@example.com",
      avatarUrl: "",
      skills: ["react"],
      isActive: true,
      createdAt: "2026-03-24T08:00:00.000Z",
    },
  ],
  tasks: [],
  agents: [],
  activity: [],
  loading: false,
  error: null,
  sectionErrors: {},
  fetchSummary,
};

const memberState = {
  createMember,
  updateMember,
};

jest.mock("next/navigation", () => ({
  usePathname: () => "/team",
  useRouter: () => ({ replace }),
  useSearchParams: () => ({
    get: (key: string) => (key === "project" ? "project-1" : null),
  }),
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (state: typeof dashboardState) => unknown) =>
    selector(dashboardState),
}));

jest.mock("@/lib/stores/member-store", () => ({
  useMemberStore: (selector: (state: typeof memberState) => unknown) =>
    selector(memberState),
}));

jest.mock("@/lib/dashboard/summary", () => ({
  summarizeMemberRoster: (...args: unknown[]) => summarizeMemberRoster(...args),
}));

jest.mock("./team-management", () => ({
  TeamManagement: ({
    projects,
    members,
    onProjectChange,
    onCreateMember,
    onUpdateMember,
  }: {
    projects: Array<{ id: string; name: string }>;
    members: TeamMember[];
    onProjectChange: (projectId: string) => void;
    onCreateMember: (input: CreateMemberInput) => Promise<void>;
    onUpdateMember: (memberId: string, input: UpdateMemberInput) => Promise<void>;
  }) => (
    (() => {
      teamManagementProjectsRefs.push(projects);
      return (
        <div>
          <span>{members[0]?.name}</span>
          <button type="button" onClick={() => onProjectChange("project-2")}>
            Switch Project
          </button>
          <button
            type="button"
            onClick={() => onCreateMember({ name: "Bob", type: "human" })}
          >
            Create Member
          </button>
          <button
            type="button"
            onClick={() => onUpdateMember("member-1", { role: "lead-frontend" })}
          >
            Update Member
          </button>
        </div>
      );
    })()
  ),
}));

describe("TeamPageClient", () => {
  beforeEach(() => {
    replace.mockReset();
    fetchSummary.mockReset();
    createMember.mockReset().mockResolvedValue(undefined);
    updateMember.mockReset().mockResolvedValue(undefined);
    summarizeMemberRoster.mockReset().mockReturnValue(roster);
    teamManagementProjectsRefs.length = 0;
  });

  it("keeps project options referentially stable when dashboard state is unchanged", async () => {
    const { rerender } = render(<TeamPageClient />);

    await act(async () => {
      rerender(<TeamPageClient />);
    });

    expect(teamManagementProjectsRefs).toHaveLength(2);
    expect(teamManagementProjectsRefs[1]).toBe(teamManagementProjectsRefs[0]);
  });

  it("reuses dashboard member summary data and refreshes after create or update flows", async () => {
    const user = userEvent.setup();

    await act(async () => {
      render(<TeamPageClient />);
    });

    expect(fetchSummary).toHaveBeenCalledWith({ projectId: "project-1" });
    expect(summarizeMemberRoster).toHaveBeenCalledWith({
      members: dashboardState.members,
      tasks: dashboardState.tasks,
      agents: dashboardState.agents,
      activity: dashboardState.activity,
    });
    expect(screen.getByText("Alice")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Switch Project" }));
    expect(replace).toHaveBeenCalledWith("/team?project=project-2");

    await user.click(screen.getByRole("button", { name: "Create Member" }));
    expect(createMember).toHaveBeenCalledWith("project-1", {
      name: "Bob",
      type: "human",
    });
    expect(fetchSummary).toHaveBeenLastCalledWith({ projectId: "project-1" });

    await user.click(screen.getByRole("button", { name: "Update Member" }));
    expect(updateMember).toHaveBeenCalledWith("member-1", "project-1", {
      role: "lead-frontend",
    });
    expect(fetchSummary).toHaveBeenLastCalledWith({ projectId: "project-1" });
  });
});
