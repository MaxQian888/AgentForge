import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeamManagement } from "./team-management";
import type { TeamMember } from "@/lib/dashboard/summary";
import type { RoleManifest } from "@/lib/stores/role-store";

describe("TeamManagement", () => {
  async function selectOption(
    user: ReturnType<typeof userEvent.setup>,
    label: string,
    option: string
  ) {
    await user.click(screen.getByRole("combobox", { name: label }));
    await user.click(screen.getByRole("option", { name: option }));
  }

  const projects = [
    { id: "project-1", name: "AgentForge" },
    { id: "project-2", name: "Bridge" },
  ];

  const members = [
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
      imPlatform: "feishu",
      imUserId: "ou_review_bot",
      skills: ["review"],
      isActive: false,
      status: "suspended",
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
  ] as unknown as TeamMember[];

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

  it("renders a unified roster and submits create actions", async () => {
    const user = userEvent.setup();
    const onCreateMember = jest.fn().mockResolvedValue(undefined);
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
        onUpdateMember={jest.fn().mockResolvedValue(undefined)}
      />
    );

    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("Review Bot")).toBeInTheDocument();
    expect(screen.getByText("Human")).toBeInTheDocument();
    expect(screen.getByText("Agent")).toBeInTheDocument();
    expect(screen.getByText("Suspended")).toBeInTheDocument();
    expect(screen.getByText("feishu • ou_review_bot")).toBeInTheDocument();
    expect(screen.getByText("frontend-developer")).toBeInTheDocument();
    expect(screen.getByText("Ready")).toBeInTheDocument();
    expect(screen.getByText("Last activity 2026-03-24 09:00 UTC")).toBeInTheDocument();

    await selectOption(user, "Project", "Bridge");
    expect(onProjectChange).toHaveBeenCalledWith("project-2");

    await user.click(screen.getByRole("button", { name: "Add Member" }));
    await user.type(screen.getByLabelText("Member Name"), "Bob");
    await selectOption(user, "Member Type", "Human");
    await user.type(screen.getByLabelText("Role"), "bug-fixer");
    await selectOption(user, "Status", "Suspended");
    await user.type(screen.getByLabelText("Email"), "bob@example.com");
    await user.type(screen.getByLabelText("IM Platform"), "slack");
    await user.type(screen.getByLabelText("IM User ID"), "U-bob");
    await user.click(screen.getByRole("button", { name: "Create Member" }));

    expect(onCreateMember).toHaveBeenCalledWith({
      name: "Bob",
      type: "human",
      role: "bug-fixer",
      status: "suspended",
      email: "bob@example.com",
      imPlatform: "slack",
      imUserId: "U-bob",
      skills: [],
    });
  });

  it("edits canonical status and IM identity for an existing agent member", async () => {
    const user = userEvent.setup();
    const onUpdateMember = jest.fn().mockResolvedValue(undefined);

    render(
      <TeamManagement
        projects={projects}
        selectedProjectId="project-1"
        members={members}
        loading={false}
        error={null}
        availableRoles={availableRoles}
        onRetry={jest.fn()}
        onProjectChange={jest.fn()}
        onCreateMember={jest.fn().mockResolvedValue(undefined)}
        onUpdateMember={onUpdateMember}
      />
    );

    await user.click(screen.getByRole("button", { name: "Edit Review Bot" }));
    const editRole = await screen.findByLabelText("Edit Role");
    await user.clear(editRole);
    await user.type(editRole, "lead-reviewer");
    const editSkills = screen.getByLabelText("Edit Skills");
    await user.clear(editSkills);
    await user.type(editSkills, "review, security, automation");
    await selectOption(user, "Edit Bound Role", "Frontend Developer");
    await selectOption(user, "Edit Status", "Active");
    const editImPlatform = screen.getByLabelText("Edit IM Platform");
    await user.clear(editImPlatform);
    await user.type(editImPlatform, "discord");
    const editImUserId = screen.getByLabelText("Edit IM User ID");
    await user.clear(editImUserId);
    await user.type(editImUserId, "review-bot");
    const editBudget = screen.getByLabelText("Edit Agent Budget USD");
    await user.clear(editBudget);
    await user.type(editBudget, "9");
    await user.click(screen.getByRole("button", { name: "Save Member" }));

    expect(onUpdateMember).toHaveBeenCalledWith("member-2", {
      name: "Review Bot",
      role: "lead-reviewer",
      status: "active",
      email: "",
      imPlatform: "discord",
      imUserId: "review-bot",
      skills: ["review", "security", "automation"],
      agentProfile: {
        roleId: "frontend-developer",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        maxBudgetUsd: "9",
        notes: "keep reviews concise",
      },
    });
  }, 20000);

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

  it("shows loading and error states with retry support", async () => {
    const user = userEvent.setup();
    const onRetry = jest.fn();
    const { rerender } = render(
      <TeamManagement
        projects={projects}
        selectedProjectId="project-1"
        members={members}
        loading
        error={null}
        availableRoles={availableRoles}
        onRetry={onRetry}
        onProjectChange={jest.fn()}
        onCreateMember={jest.fn()}
        onUpdateMember={jest.fn()}
      />
    );

    expect(screen.getByText("Loading team roster...")).toBeInTheDocument();

    rerender(
      <TeamManagement
        projects={projects}
        selectedProjectId="project-1"
        members={members}
        loading={false}
        error="boom"
        availableRoles={availableRoles}
        onRetry={onRetry}
        onProjectChange={jest.fn()}
        onCreateMember={jest.fn()}
        onUpdateMember={jest.fn()}
      />
    );

    expect(screen.getByText("Team roster unavailable")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry Team Load" }));
    expect(onRetry).toHaveBeenCalled();
  });

  it("includes agent profile data when creating an agent member and can cancel edit mode", async () => {
    const user = userEvent.setup();
    const onCreateMember = jest.fn().mockResolvedValue(undefined);

    render(
      <TeamManagement
        projects={projects}
        selectedProjectId="project-1"
        members={members}
        loading={false}
        error={null}
        availableRoles={availableRoles}
        onRetry={jest.fn()}
        onProjectChange={jest.fn()}
        onCreateMember={onCreateMember}
        onUpdateMember={jest.fn().mockResolvedValue(undefined)}
      />
    );

    await user.click(screen.getByRole("button", { name: "Add Member" }));
    await user.type(screen.getByLabelText("Member Name"), "Ops Bot");
    await selectOption(user, "Member Type", "Agent");
    await user.type(screen.getByLabelText("Role"), "ops-bot");
    await user.type(screen.getByLabelText("Skills"), "ops, alerts");
    await selectOption(user, "Bound Role", "Frontend Developer");
    await user.type(screen.getByLabelText("Agent Budget USD"), "12");
    await user.type(screen.getByLabelText("Runtime"), "codex");
    await user.type(screen.getByLabelText("Provider"), "openai");
    await user.type(screen.getByLabelText("Model"), "gpt-5-codex");
    await user.type(screen.getByLabelText("Agent Notes"), "Handle incidents");
    await user.click(screen.getByRole("button", { name: "Create Member" }));

    expect(onCreateMember).toHaveBeenCalledWith({
      name: "Ops Bot",
      type: "agent",
      role: "ops-bot",
      status: "active",
      email: "",
      imPlatform: "",
      imUserId: "",
      skills: ["ops", "alerts"],
      agentProfile: {
        roleId: "frontend-developer",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        maxBudgetUsd: "12",
        notes: "Handle incidents",
      },
    });

    await user.click(screen.getByRole("button", { name: "Edit Review Bot" }));
    await waitFor(() =>
      expect(screen.getByText("Edit Member")).toBeInTheDocument()
    );
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByText("Edit Member")).not.toBeInTheDocument();
  }, 20000);
});
