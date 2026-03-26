export type RoleWorkspaceSectionId =
  | "setup"
  | "identity"
  | "capabilities"
  | "knowledge"
  | "governance"
  | "review";

export interface RoleWorkspaceSectionDefinition {
  id: RoleWorkspaceSectionId;
  label: string;
  description: string;
}

export interface RoleWorkspaceGuidanceDefinition {
  title: string;
  summary: string;
  bullets: string[];
}

export const ROLE_WORKSPACE_SECTIONS: RoleWorkspaceSectionDefinition[] = [
  {
    id: "setup",
    label: "Setup",
    description: "Choose how this role starts, then confirm its reusable identity.",
  },
  {
    id: "identity",
    label: "Identity",
    description: "Define the role job, persona, language, and response style.",
  },
  {
    id: "capabilities",
    label: "Capabilities",
    description: "Describe packages, tools, skills, and execution limits.",
  },
  {
    id: "knowledge",
    label: "Knowledge",
    description: "Bind repos, docs, patterns, and shared knowledge sources.",
  },
  {
    id: "governance",
    label: "Governance",
    description: "Set security, collaboration, and trigger expectations.",
  },
  {
    id: "review",
    label: "Review",
    description: "Inspect YAML, summary, preview, and sandbox before saving.",
  },
];

export const ROLE_WORKSPACE_GUIDANCE: Record<
  RoleWorkspaceSectionId,
  RoleWorkspaceGuidanceDefinition
> = {
  setup: {
    title: "Start From The Right Base",
    summary:
      "Start from a template when most values should carry over. Use inheritance only when the new role is truly a child role.",
    bullets: [
      "Reuse a template to copy the current draft shape into a new role.",
      "Set extends when the role should keep a parent relationship instead of forking the config.",
      "Confirm metadata first so later preview and sandbox results are easy to interpret.",
    ],
  },
  identity: {
    title: "Align Role, Goal, And Prompt",
    summary:
      "Identity fields should agree on what the role does, how it behaves, and how it should answer.",
    bullets: [
      "Fill role title, goal, and system prompt before tuning persona details.",
      "Use advanced identity for language, tone, and response style rather than burying those rules in the prompt.",
      "If role and prompt disagree, sandbox output becomes harder to trust.",
    ],
  },
  capabilities: {
    title: "Model Tools And Skills Explicitly",
    summary:
      "Use packages for reusable capability bundles and skills for explicit knowledge references this role can load.",
    bullets: [
      "Keep allowed tools and external tools readable enough to review before save.",
      "Use skills to declare opt-in knowledge or workflows that belong to this role.",
      "Execution limits such as turns or budget should describe guardrails, not hidden surprises.",
    ],
  },
  knowledge: {
    title: "Bind The Right Context",
    summary:
      "Knowledge fields should make the role's trusted sources obvious to operators and reviewers.",
    bullets: [
      "List repositories, documents, and patterns that matter to this role.",
      "Use shared knowledge rows for sources that the role can cite or reuse across runs.",
      "Prefer concise references over long prompt prose when the knowledge source already exists.",
    ],
  },
  governance: {
    title: "Keep Governance Visible",
    summary:
      "Permission mode, path rules, review requirements, delegation, and triggers should be explicit and reviewable.",
    bullets: [
      "Use permission and path constraints instead of long negative instructions in the prompt.",
      "Add collaboration settings that other operators can explain and maintain.",
      "Only add triggers whose activation rule is clear enough to review later.",
    ],
  },
  review: {
    title: "Preview Before You Save",
    summary:
      "Inspect the execution summary, YAML, and authoritative preview or sandbox results before persisting the draft.",
    bullets: [
      "Preview resolves the effective manifest and execution profile without calling a model.",
      "Sandbox reuses the same draft but adds readiness checks and one bounded prompt probe.",
      "Review the YAML mapping so the structured inputs match the canonical role asset you expect.",
    ],
  },
};
