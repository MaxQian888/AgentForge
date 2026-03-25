import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RoleFormDialog } from "./role-form-dialog";
import type { RoleManifest } from "@/lib/stores/role-store";

const frontendRole: RoleManifest = {
  apiVersion: "agentforge/v1",
  kind: "Role",
  metadata: {
    id: "frontend-developer",
    name: "Frontend Developer",
    version: "1.0.0",
    description: "Frontend specialist",
    author: "AgentForge",
    tags: ["frontend", "react"],
  },
  identity: {
    role: "Senior Frontend Developer",
    goal: "Build polished UI",
    backstory: "Frontend expert",
    systemPrompt: "Use React and Next.js",
    persona: "Helpful",
    goals: ["Build"],
    constraints: ["Stay accessible"],
  },
  capabilities: {
    allowedTools: ["Read", "Edit"],
    skills: [
      { path: "skills/react", autoLoad: true },
      { path: "skills/testing", autoLoad: false },
    ],
    languages: ["TypeScript"],
    frameworks: ["Next.js"],
    maxTurns: 30,
    maxBudgetUsd: 5,
  },
  knowledge: {
    repositories: ["app"],
    documents: ["docs/PRD.md"],
    patterns: ["rsc"],
  },
  security: {
    permissionMode: "bypassPermissions",
    allowedPaths: ["app/", "components/"],
    deniedPaths: ["secrets/"],
    maxBudgetUsd: 5,
    requireReview: true,
  },
  extends: "coding-agent",
};

describe("RoleFormDialog", () => {
  it("prefills from a template role and submits structured inheritance data", async () => {
    const user = userEvent.setup();
    const onSubmit = jest.fn().mockResolvedValue(undefined);

    render(
      <RoleFormDialog
        open
        onOpenChange={jest.fn()}
        onSubmit={onSubmit}
        availableRoles={[frontendRole]}
      />
    );

    await user.selectOptions(screen.getByLabelText("Start from template"), "frontend-developer");
    await user.clear(screen.getByLabelText("Role ID"));
    await user.type(screen.getByLabelText("Role ID"), "custom-frontend");
    await user.selectOptions(screen.getByLabelText("Inherits from"), "frontend-developer");

    await user.click(screen.getByRole("button", { name: "Create" }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        metadata: expect.objectContaining({
          id: "custom-frontend",
          name: "Frontend Developer",
        }),
        extends: "frontend-developer",
        capabilities: expect.objectContaining({
          allowedTools: ["Read", "Edit"],
          skills: [
            { path: "skills/react", autoLoad: true },
            { path: "skills/testing", autoLoad: false },
          ],
        }),
        security: expect.objectContaining({
          permissionMode: "bypassPermissions",
          requireReview: true,
        }),
        knowledge: expect.objectContaining({
          documents: ["docs/PRD.md"],
        }),
      }),
    );
  });

  it("renders structured sections for identity, capabilities, skills, knowledge, and security", async () => {
    const user = userEvent.setup();

    render(
      <RoleFormDialog
        open
        onOpenChange={jest.fn()}
        onSubmit={jest.fn().mockResolvedValue(undefined)}
        availableRoles={[frontendRole]}
      />
    );

    await user.selectOptions(screen.getByLabelText("Start from template"), "frontend-developer");

    expect(screen.getByText("Identity")).toBeInTheDocument();
    expect(screen.getByText("Capabilities")).toBeInTheDocument();
    expect(screen.getByText("Skills")).toBeInTheDocument();
    expect(screen.getByText("Knowledge")).toBeInTheDocument();
    expect(screen.getByText("Security")).toBeInTheDocument();
    expect(screen.getByLabelText("Allowed Tools")).toBeInTheDocument();
    expect(screen.getAllByLabelText("Skill Path").length).toBe(2);
    expect(screen.getByLabelText("Permission Mode")).toBeInTheDocument();
  });

  it("supports edit-mode updates, skill row management, and cancel", async () => {
    const user = userEvent.setup();
    const onSubmit = jest.fn().mockResolvedValue(undefined);
    const onOpenChange = jest.fn();

    render(
      <RoleFormDialog
        open
        role={frontendRole}
        onOpenChange={onOpenChange}
        onSubmit={onSubmit}
        availableRoles={[frontendRole]}
      />
    );

    expect(screen.getByLabelText("Role ID")).toBeDisabled();
    expect(screen.getByLabelText("Start from template")).toBeDisabled();

    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Frontend Lead");
    await user.clear(screen.getByLabelText("Allowed Tools"));
    await user.type(screen.getByLabelText("Allowed Tools"), "Read, Edit, Web");
    await user.click(screen.getByRole("button", { name: "Add Skill" }));
    const skillInputs = screen.getAllByLabelText("Skill Path");
    await user.type(skillInputs[2]!, "skills/accessibility");
    const autoLoadChecks = screen.getAllByLabelText("Auto-load skill");
    await user.click(autoLoadChecks[2]!);
    await user.click(screen.getAllByRole("button", { name: "Remove" })[0]!);
    await user.selectOptions(screen.getByLabelText("Permission Mode"), "acceptEdits");
    await user.click(screen.getByLabelText("Require review before execution"));
    await user.click(screen.getByRole("button", { name: "Update" }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        metadata: expect.objectContaining({ name: "Frontend Lead" }),
        capabilities: expect.objectContaining({
          allowedTools: ["Read", "Edit", "Web"],
          skills: [
            { path: "skills/testing", autoLoad: false },
            { path: "skills/accessibility", autoLoad: true },
          ],
        }),
        security: expect.objectContaining({
          permissionMode: "acceptEdits",
          requireReview: false,
        }),
      }),
    );

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
