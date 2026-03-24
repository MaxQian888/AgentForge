import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeamManagement } from "./team-management";
import type { TeamMember } from "@/lib/dashboard/summary";

describe("TeamManagement", () => {
  const projects = [
    { id: "project-1", name: "AgentForge" },
    { id: "project-2", name: "Bridge" },
  ];

  const members: TeamMember[] = [
    {
      id: "member-1",
      projectId: "project-1",
      name: "Alice",
      type: "human",
      typeLabel: "Human",
      role: "frontend-developer",
      email: "alice@example.com",
      avatarUrl: "",
      skills: ["react", "testing"],
      isActive: true,
      status: "active",
      createdAt: "2026-03-20T10:00:00.000Z",
      lastActivityAt: "2026-03-24T09:00:00.000Z",
      workload: {
        assignedTasks: 2,
        inProgressTasks: 1,
        inReviewTasks: 1,
        activeAgentRuns: 0,
      },
    },
    {
      id: "member-2",
      projectId: "project-1",
      name: "Review Bot",
      type: "agent",
      typeLabel: "Agent",
      role: "code-reviewer",
      email: "",
      avatarUrl: "",
      skills: ["review"],
      isActive: true,
      status: "active",
      createdAt: "2026-03-20T10:00:00.000Z",
      lastActivityAt: "2026-03-24T08:30:00.000Z",
      workload: {
        assignedTasks: 1,
        inProgressTasks: 1,
        inReviewTasks: 0,
        activeAgentRuns: 1,
      },
    },
  ];

  it("renders a unified roster and submits create/update actions", async () => {
    const user = userEvent.setup();
    const onCreateMember = jest.fn().mockResolvedValue(undefined);
    const onUpdateMember = jest.fn().mockResolvedValue(undefined);
    const onProjectChange = jest.fn();

    render(
      <TeamManagement
        projects={projects}
        selectedProjectId="project-1"
        members={members}
        loading={false}
        error={null}
        onRetry={jest.fn()}
        onProjectChange={onProjectChange}
        onCreateMember={onCreateMember}
        onUpdateMember={onUpdateMember}
      />
    );

    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("Review Bot")).toBeInTheDocument();
    expect(screen.getByText("Human")).toBeInTheDocument();
    expect(screen.getByText("Agent")).toBeInTheDocument();

    await user.selectOptions(screen.getByLabelText("Project"), "project-2");
    expect(onProjectChange).toHaveBeenCalledWith("project-2");

    await user.click(screen.getByRole("button", { name: "Add Member" }));
    await user.type(screen.getByLabelText("Member Name"), "Bob");
    await user.selectOptions(screen.getByLabelText("Member Type"), "human");
    await user.type(screen.getByLabelText("Role"), "bug-fixer");
    await user.type(screen.getByLabelText("Email"), "bob@example.com");
    await user.click(screen.getByRole("button", { name: "Create Member" }));

    expect(onCreateMember).toHaveBeenCalledWith({
      name: "Bob",
      type: "human",
      role: "bug-fixer",
      email: "bob@example.com",
      skills: [],
    });

    await user.click(screen.getByRole("button", { name: "Edit Alice" }));
    const editRole = screen.getByLabelText("Edit Role");
    await user.clear(editRole);
    await user.type(editRole, "lead-frontend");
    await user.click(screen.getByRole("button", { name: "Save Member" }));

    expect(onUpdateMember).toHaveBeenCalledWith("member-1", {
      name: "Alice",
      role: "lead-frontend",
      email: "alice@example.com",
      isActive: true,
    });
  });

  it("shows a project-scoped empty state", () => {
    render(
      <TeamManagement
        projects={projects}
        selectedProjectId="project-1"
        members={[]}
        loading={false}
        error={null}
        onRetry={jest.fn()}
        onProjectChange={jest.fn()}
        onCreateMember={jest.fn()}
        onUpdateMember={jest.fn()}
      />
    );

    expect(screen.getByText("No team members yet.")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Add the first member" })
    ).toBeInTheDocument();
  });
});
