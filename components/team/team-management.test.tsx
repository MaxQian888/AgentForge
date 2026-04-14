import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeamManagement } from "./team-management";
import type { TeamMember } from "@/lib/dashboard/summary";
import type { MemberStatus } from "@/lib/team/member-status";
import type { RoleManifest } from "@/lib/stores/role-store";

describe("TeamManagement", () => {
  function setFieldValue(label: string, value: string) {
    fireEvent.change(screen.getByLabelText(label), {
      target: { value },
    });
  }

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
    {
      id: "member-3",
      projectId: "project-1",
      name: "Builder Bot",
      type: "agent",
      typeLabel: "Agent",
      role: "implementer",
      email: "",
      avatarUrl: "",
      skills: ["delivery"],
      isActive: true,
      status: "active",
      createdAt: "2026-03-20T10:00:00.000Z",
      lastActivityAt: "2026-03-24T07:45:00.000Z",
      roleBindingLabel: "frontend-developer",
      readinessState: "incomplete",
      readinessLabel: "Setup Required",
      readinessMissing: ["runtime", "provider", "model"],
      agentSummary: [],
      agentProfile: {
        roleId: "frontend-developer",
        runtime: "",
        provider: "",
        model: "",
        maxBudgetUsd: null,
        notes: "",
      },
      workload: {
        assignedTasks: 0,
        inProgressTasks: 0,
        inReviewTasks: 0,
        activeAgentRuns: 0,
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
    expect(screen.getAllByText("Agent").length).toBeGreaterThan(0);
    expect(screen.getByText("Suspended")).toBeInTheDocument();
    expect(screen.getByText("feishu • ou_review_bot")).toBeInTheDocument();
    expect(screen.getByText("frontend-developer")).toBeInTheDocument();
    expect(screen.getByText("Ready")).toBeInTheDocument();
    expect(screen.getByText("Last activity 2026-03-24 09:00 UTC")).toBeInTheDocument();

    await selectOption(user, "Project", "Bridge");
    expect(onProjectChange).toHaveBeenCalledWith("project-2");

    await user.click(screen.getByRole("button", { name: "Add Member" }));
    setFieldValue("Member Name", "Bob");
    await selectOption(user, "Member Type", "Human");
    setFieldValue("Role", "bug-fixer");
    await selectOption(user, "Status", "Suspended");
    setFieldValue("Email", "bob@example.com");
    setFieldValue("IM Platform", "slack");
    setFieldValue("IM User ID", "U-bob");
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
    fireEvent.change(editRole, { target: { value: "lead-reviewer" } });
    const editSkills = screen.getByLabelText("Edit Skills");
    fireEvent.change(editSkills, {
      target: { value: "review, security, automation" },
    });
    await selectOption(user, "Edit Bound Role", "Frontend Developer");
    await selectOption(user, "Edit Status", "Active");
    const editImPlatform = screen.getByLabelText("Edit IM Platform");
    fireEvent.change(editImPlatform, { target: { value: "discord" } });
    const editImUserId = screen.getByLabelText("Edit IM User ID");
    fireEvent.change(editImUserId, { target: { value: "review-bot" } });
    const editBudget = screen.getByLabelText("Edit Agent Budget USD");
    fireEvent.change(editBudget, { target: { value: "9" } });
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
    setFieldValue("Member Name", "Ops Bot");
    await selectOption(user, "Member Type", "Agent");
    setFieldValue("Role", "ops-bot");
    setFieldValue("Skills", "ops, alerts");
    await selectOption(user, "Bound Role", "Frontend Developer");
    setFieldValue("Agent Budget USD", "12");
    setFieldValue("Runtime", "codex");
    setFieldValue("Provider", "openai");
    setFieldValue("Model", "gpt-5-codex");
    setFieldValue("Agent Notes", "Handle incidents");
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

  it("links roster drill-downs and opens setup-required agents with highlighted missing fields", async () => {
    const user = userEvent.setup();

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
        onUpdateMember={jest.fn().mockResolvedValue(undefined)}
      />
    );

    expect(screen.getByRole("link", { name: "Alice" })).toHaveAttribute(
      "href",
      "/project?id=project-1&member=member-1"
    );
    expect(screen.getByRole("link", { name: "View Alice tasks" })).toHaveAttribute(
      "href",
      "/project?id=project-1&member=member-1"
    );
    expect(
      screen.getByRole("link", { name: "View Review Bot agent activity" })
    ).toHaveAttribute("href", "/agents?member=member-2");

    await user.click(screen.getByRole("button", { name: "Setup Required" }));

    expect(await screen.findByText("Edit Member")).toBeInTheDocument();
    expect(screen.getByLabelText("Edit Runtime")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByLabelText("Edit Provider")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByLabelText("Edit Model")).toHaveAttribute("aria-invalid", "true");
  });

  it("focuses attention categories and shows inline bulk-governance results", async () => {
    const user = userEvent.setup();
    const onBulkUpdateMembers = jest.fn().mockResolvedValue({
      status: "inactive" satisfies MemberStatus,
      results: [
        { memberId: "member-1", success: true, status: "inactive" },
        { memberId: "member-2", success: false, error: "member not found in project" },
      ],
    });

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
        onUpdateMember={jest.fn().mockResolvedValue(undefined)}
        onBulkUpdateMembers={onBulkUpdateMembers}
        bulkUpdatePending={false}
        bulkUpdateResult={null}
        onClearBulkUpdateResult={jest.fn()}
      />
    );

    await user.click(screen.getByRole("button", { name: "Setup Required (1)" }));
    expect(screen.queryByText("Alice")).not.toBeInTheDocument();
    expect(screen.getByText("Builder Bot")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Clear attention filter" }));
    expect(screen.getByText("Alice")).toBeInTheDocument();

    await user.click(screen.getByRole("checkbox", { name: "Select Alice" }));
    await user.click(screen.getByRole("checkbox", { name: "Select Review Bot" }));
    await user.click(screen.getByRole("button", { name: "Mark selected inactive" }));

    expect(onBulkUpdateMembers).toHaveBeenCalledWith(
      ["member-1", "member-2"],
      "inactive",
    );

    expect(
      await screen.findByText("Bulk update complete: 1 updated, 1 failed.")
    ).toBeInTheDocument();
    expect(
      screen.getByText("Review Bot: member not found in project")
    ).toBeInTheDocument();
  });

  it("disables quick lifecycle actions while a member update is in flight", async () => {
    const user = userEvent.setup();
    let resolveUpdate: (() => void) | null = null;
    const onUpdateMember = jest.fn().mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveUpdate = resolve;
        })
    );

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

    const activateButton = screen.getByRole("button", { name: "Activate Review Bot" });
    await user.click(activateButton);

    expect(onUpdateMember).toHaveBeenCalledWith("member-2", {
      status: "active",
    });
    expect(screen.getByRole("button", { name: "Updating Review Bot..." })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Updating Review Bot..." }));
    expect(onUpdateMember).toHaveBeenCalledTimes(1);

    const releaseUpdate = resolveUpdate as (() => void) | null;
    if (!releaseUpdate) {
      throw new Error("expected member update promise resolver to be captured");
    }
    releaseUpdate();
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Activate Review Bot" })).toBeEnabled()
    );
  });

  it("clears attention filters and bulk selection when the project scope changes", async () => {
    const user = userEvent.setup();
    const onClearBulkUpdateResult = jest.fn();
    const { rerender } = render(
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
        onUpdateMember={jest.fn().mockResolvedValue(undefined)}
        onBulkUpdateMembers={jest.fn().mockResolvedValue({
          status: "suspended",
          results: [],
        })}
        bulkUpdatePending={false}
        bulkUpdateResult={null}
        onClearBulkUpdateResult={onClearBulkUpdateResult}
      />
    );

    await user.click(screen.getByRole("button", { name: "Setup Required (1)" }));
    await user.click(screen.getByRole("checkbox", { name: "Select Builder Bot" }));

    expect(screen.queryByText("Alice")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Mark selected suspended" })).toBeInTheDocument();

    rerender(
      <TeamManagement
        projects={projects}
        selectedProjectId="project-2"
        members={members}
        loading={false}
        error={null}
        availableRoles={availableRoles}
        onRetry={jest.fn()}
        onProjectChange={jest.fn()}
        onCreateMember={jest.fn().mockResolvedValue(undefined)}
        onUpdateMember={jest.fn().mockResolvedValue(undefined)}
        onBulkUpdateMembers={jest.fn().mockResolvedValue({
          status: "suspended",
          results: [],
        })}
        bulkUpdatePending={false}
        bulkUpdateResult={null}
        onClearBulkUpdateResult={onClearBulkUpdateResult}
      />
    );

    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Mark selected suspended" })
    ).not.toBeInTheDocument();
    expect(onClearBulkUpdateResult).toHaveBeenCalled();
  });
});
