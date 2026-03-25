import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeamManagement } from "./team-management";
import type { TeamMember } from "@/lib/dashboard/summary";
import type { RoleManifest } from "@/lib/stores/role-store";

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
      roleBindingLabel: "frontend-developer",
      readinessState: "ready",
      readinessLabel: "Ready",
      agentSummary: ["codex", "openai", "gpt-5-codex"],
      agentProfile: {
        roleId: "frontend-developer",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        maxBudgetUsd: 8,
        notes: "keep reviews concise",
      },
      workload: {
        assignedTasks: 1,
        inProgressTasks: 1,
        inReviewTasks: 0,
        activeAgentRuns: 1,
      },
    },
  ];

  const availableRoles: RoleManifest[] = [
    {
      apiVersion: "agentforge/v1",
      kind: "Role",
      metadata: {
        id: "frontend-developer",
        name: "Frontend Developer",
        version: "1.0.0",
        description: "Builds UI",
        author: "AgentForge",
        tags: ["frontend"],
      },
      identity: {
        role: "Senior Frontend Developer",
        goal: "Ship UI",
        backstory: "Frontend specialist",
        systemPrompt: "Build accessible UI",
        persona: "Helpful",
        goals: ["Ship"],
        constraints: ["Keep tests green"],
      },
      capabilities: {
        allowedTools: ["Read", "Edit"],
        languages: ["TypeScript"],
        frameworks: ["Next.js"],
        maxTurns: 20,
        maxBudgetUsd: 5,
      },
      knowledge: {
        repositories: ["app"],
        documents: ["docs/PRD.md"],
        patterns: ["ui"],
      },
      security: {
        permissionMode: "default",
        allowedPaths: ["app/"],
        deniedPaths: [],
        maxBudgetUsd: 5,
        requireReview: true,
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
        availableRoles={availableRoles}
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
    expect(screen.getByText("frontend-developer")).toBeInTheDocument();
    expect(screen.getByText("Ready")).toBeInTheDocument();

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

    await user.click(screen.getByRole("button", { name: "Edit Review Bot" }));
    const editRole = screen.getByLabelText("Edit Role");
    await user.clear(editRole);
    await user.type(editRole, "lead-reviewer");
    const editSkills = screen.getByLabelText("Edit Skills");
    await user.clear(editSkills);
    await user.type(editSkills, "review, security, automation");
    await user.selectOptions(screen.getByLabelText("Edit Bound Role"), "frontend-developer");
    const editBudget = screen.getByLabelText("Edit Agent Budget USD");
    await user.clear(editBudget);
    await user.type(editBudget, "9");
    await user.click(screen.getByRole("button", { name: "Save Member" }));

    expect(onUpdateMember).toHaveBeenCalledWith("member-2", {
      name: "Review Bot",
      role: "lead-reviewer",
      email: "",
      skills: ["review", "security", "automation"],
      agentProfile: {
        roleId: "frontend-developer",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        maxBudgetUsd: "9",
        notes: "keep reviews concise",
      },
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
        availableRoles={availableRoles}
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
