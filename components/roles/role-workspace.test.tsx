import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RoleWorkspace } from "./role-workspace";
import type { RoleManifest } from "@/lib/stores/role-store";

const frontendRole: RoleManifest = {
  apiVersion: "agentforge/v1",
  kind: "Role",
  metadata: {
    id: "frontend-developer",
    name: "Frontend Developer",
    version: "1.2.0",
    description: "Builds polished product UI",
    author: "AgentForge",
    tags: ["frontend", "ui"],
  },
  identity: {
    role: "Senior Frontend Developer",
    goal: "Ship a great dashboard UX",
    backstory: "A patient frontend specialist",
    systemPrompt: "Focus on clarity and accessibility.",
    persona: "Helpful",
    goals: ["Improve workflows"],
    constraints: ["Keep tests green"],
  },
  capabilities: {
    allowedTools: ["Read", "Edit"],
    languages: ["TypeScript"],
    frameworks: ["Next.js"],
    maxTurns: 24,
    maxBudgetUsd: 6,
  },
  knowledge: {
    repositories: ["app", "components"],
    documents: ["docs/PRD.md"],
    patterns: ["responsive-layouts"],
  },
  security: {
    permissionMode: "default",
    allowedPaths: ["app/", "components/"],
    deniedPaths: ["secrets/"],
    maxBudgetUsd: 6,
    requireReview: true,
  },
  extends: "coding-agent",
};

describe("RoleWorkspace", () => {
  it("supports template-based creation with a live execution summary rail", async () => {
    const user = userEvent.setup();
    const onCreateRole = jest.fn().mockResolvedValue(undefined);

    render(
      <RoleWorkspace
        roles={[frontendRole]}
        loading={false}
        error={null}
        onCreateRole={onCreateRole}
        onUpdateRole={jest.fn().mockResolvedValue(undefined)}
        onDeleteRole={jest.fn().mockResolvedValue(undefined)}
      />,
    );

    await user.click(screen.getByRole("button", { name: "New Role" }));
    await user.selectOptions(screen.getByLabelText("Start from template"), "frontend-developer");
    await user.clear(screen.getByLabelText("Role ID"));
    await user.type(screen.getByLabelText("Role ID"), "frontend-developer-copy");
    await user.click(screen.getByRole("button", { name: "Save Role" }));

    expect(onCreateRole).toHaveBeenCalledWith(
      expect.objectContaining({
        metadata: expect.objectContaining({
          id: "frontend-developer-copy",
          name: "Frontend Developer",
          version: "1.2.0",
        }),
      }),
    );

    expect(screen.getByText("Execution Summary")).toBeInTheDocument();
    expect(screen.getByText("Ship a great dashboard UX")).toBeInTheDocument();
    expect(screen.getByText("Read, Edit")).toBeInTheDocument();
    expect(screen.getByText("Review required")).toBeInTheDocument();
  });

  it("loads an existing role into structured workspace sections", async () => {
    const user = userEvent.setup();

    render(
      <RoleWorkspace
        roles={[frontendRole]}
        loading={false}
        error={null}
        onCreateRole={jest.fn().mockResolvedValue(undefined)}
        onUpdateRole={jest.fn().mockResolvedValue(undefined)}
        onDeleteRole={jest.fn().mockResolvedValue(undefined)}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit Frontend Developer" }));

    expect(screen.getByDisplayValue("frontend-developer")).toBeInTheDocument();
    expect(screen.getByDisplayValue("1.2.0")).toBeInTheDocument();
    expect(screen.getByText("Identity")).toBeInTheDocument();
    expect(screen.getByText("Capabilities")).toBeInTheDocument();
    expect(screen.getByText("Knowledge")).toBeInTheDocument();
    expect(screen.getByText("Security")).toBeInTheDocument();
  });
});
