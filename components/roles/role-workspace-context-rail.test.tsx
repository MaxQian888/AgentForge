jest.mock("next-intl", () => ({
  useTranslations: () => (
    key: string,
    values?: Record<string, string>,
  ) => {
    const map: Record<string, string> = {
      "contextRail.authoringGuide": "Authoring Guide",
      "contextRail.previewAndSandbox": "Preview And Sandbox",
      "contextRail.previewing": "Previewing",
      "contextRail.previewDraft": "Preview Role Draft",
      "contextRail.running": "Running",
      "contextRail.runSandbox": "Run Sandbox Probe",
      "contextRail.sandboxInput": "Sandbox Input",
      "contextRail.readiness": "Readiness",
      "contextRail.readinessNone": "No readiness issues",
      "contextRail.validationIssues": "Validation Issues",
      "contextRail.validationIssuesNone": "No validation issues",
      "contextRail.executionSummary": "Execution Summary",
      "contextRail.allowedTools": "Allowed Tools",
      "contextRail.skills": "Skills",
      "contextRail.budget": "Budget",
      "contextRail.turnLimit": "Turn Limit",
      "contextRail.permissionMode": "Permission Mode",
      "contextRail.promptIntent": "Prompt Intent",
      "contextRail.safetyCues": "Safety Cues",
      "contextRail.skillResolution": "Skill Resolution",
      "contextRail.yamlPreview": "YAML Preview",
      "contextRail.advancedAuthoring": "Advanced Authoring",
      "contextRail.advancedSettings": "Advanced Settings",
      "contextRail.advancedSettingsNone": "No advanced settings",
      "contextRail.storedOnlyFields": "Stored Only Fields",
      "contextRail.storedOnlyFieldsDesc": "Stored fields remain canonical.",
      "contextRail.storedOnlyNone": "No stored-only fields",
      "contextRail.runtimeProjection": "Runtime Projection",
      "contextRail.runtimeProjectionDesc": "Runtime view of the role.",
      "workspace.provenanceSummary": "Inherited {inherited}, Template {template}, Explicit {explicit}",
    };
    let template = map[key] ?? key;
    if (key === "contextRail.guidanceFor") {
      template = "Guidance for {section}";
    }
    if (key === "contextRail.inheritsFrom") {
      template = "Inherited from {name}";
    }
    return Object.entries(values ?? {}).reduce(
      (message, [name, value]) => message.replace(`{${name}}`, String(value)),
      template,
    );
  },
}));

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RoleWorkspaceContextRail } from "./role-workspace-context-rail";

describe("RoleWorkspaceContextRail", () => {
  it("renders preview, sandbox, advanced authoring, and runtime projection details", async () => {
    const user = userEvent.setup();
    const onPreview = jest.fn();
    const onSandbox = jest.fn();
    const onSandboxInputChange = jest.fn();

    render(
      <RoleWorkspaceContextRail
        activeSection="review"
        executionSummary={{
          toolsLabel: "Read, Edit",
          skillsLabel: "1 auto / 1 on-demand",
          budgetLabel: "$6",
          turnsLabel: "24",
          permissionMode: "default",
          promptIntent: "Ship a great dashboard UX",
          keySkillPaths: ["skills/react"],
          safetyCues: ["Review required"],
        }}
        effectiveSkillResolution={[
          {
            label: "React",
            path: "skills/react",
            status: "resolved",
            provenance: "explicit",
          },
        ] as never}
        yamlPreview={"kind: Role\nmetadata:\n  id: frontend"}
        previewLoading={false}
        sandboxLoading={false}
        sandboxInput="Try a sample task"
        onSandboxInputChange={onSandboxInputChange}
        onPreview={onPreview}
        onSandbox={onSandbox}
        previewResult={{
          executionProfile: {
            name: "Frontend Developer",
            role_id: "frontend",
            loaded_skills: [{ label: "React", path: "skills/react" }],
            available_skills: [{ label: "Testing", path: "skills/testing" }],
            skill_diagnostics: [{ code: "missing", message: "Testing unresolved" }],
          },
          effectiveManifest: {
            capabilities: {
              customSettings: { approval_mode: "guided" },
              toolConfig: { mcpServers: [{ name: "design-mcp" }] },
            },
            knowledge: { memory: { shortTerm: { maxTokens: 64000 } } },
            collaboration: { canDelegateTo: ["frontend"] },
            triggers: [{ event: "pr_created" }],
            overrides: { "identity.role": "Frontend Captain" },
          },
          inheritance: { parentRoleId: "coding-agent" },
          validationIssues: [{ field: "overrides", message: "Use explicit override paths only." }],
        } as never}
        sandboxResult={{
          selection: {
            runtime: "claude_code",
            provider: "anthropic",
            model: "claude-sonnet-4-5",
          },
          probe: { text: "A calm frontend specialist." },
          readinessDiagnostics: [{ code: "missing_credentials", message: "Missing runtime credentials" }],
        } as never}
        provenanceMap={{
          customSettings: [{ key: "approval_mode", provenance: "template" }],
          mcpServers: [{ key: "design-mcp", provenance: "inherited" }],
          sharedKnowledge: [],
          privateKnowledge: [],
          triggers: [{ key: "pr_created:auto_review", provenance: "explicit" }],
          collaboration: [{ key: "canDelegateTo", provenance: "explicit" }],
        }}
      />,
    );

    expect(screen.getByText("Authoring Guide")).toBeInTheDocument();
    expect(screen.getByText("Preview And Sandbox")).toBeInTheDocument();
    expect(screen.getByText("Execution Summary")).toBeInTheDocument();
    expect(screen.getByText("Read, Edit")).toBeInTheDocument();
    expect(screen.getByText("Ship a great dashboard UX")).toBeInTheDocument();
    expect(screen.getByText("Review required")).toBeInTheDocument();
    expect(
      screen.getByText((content) =>
        content.includes("Effective role: Frontend Developer (frontend)"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("claude_code / anthropic / claude-sonnet-4-5")).toBeInTheDocument();
    expect(screen.getByText("A calm frontend specialist.")).toBeInTheDocument();
    expect(screen.getByText("Missing runtime credentials")).toBeInTheDocument();
    expect(screen.getByText("overrides: Use explicit override paths only.")).toBeInTheDocument();
    expect(screen.getByText("Inherited from coding-agent")).toBeInTheDocument();
    expect(
      screen.getByText("Inherited 1, Template 1, Explicit 2"),
    ).toBeInTheDocument();
    expect(screen.getAllByText("knowledge.memory").length).toBeGreaterThan(0);
    expect(screen.getByText("React (skills/react)")).toBeInTheDocument();
    expect(screen.getByText("Testing (skills/testing)")).toBeInTheDocument();
    expect(screen.getByText("Testing unresolved")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Preview Role Draft" }));
    await user.click(screen.getByRole("button", { name: "Run Sandbox Probe" }));
    fireEvent.change(screen.getByLabelText("Sandbox Input"), {
      target: { value: "Check this role" },
    });

    expect(onPreview).toHaveBeenCalled();
    expect(onSandbox).toHaveBeenCalled();
    expect(onSandboxInputChange).toHaveBeenLastCalledWith("Check this role");
  });
});
