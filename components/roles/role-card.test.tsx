import rolesMessages from "../../messages/en/roles.json";

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    const resolved = key
      .split(".")
      .reduce<unknown>((current, part) => {
        if (current && typeof current === "object" && part in current) {
          return (current as Record<string, unknown>)[part];
        }
        return undefined;
      }, rolesMessages as unknown as Record<string, unknown>);

    if (typeof resolved !== "string") {
      return key;
    }
    return Object.entries(values ?? {}).reduce(
      (message, [name, value]) => message.replace(`{${name}}`, String(value)),
      resolved,
    );
  },
}));

import { render, screen } from "@testing-library/react";
import { RoleCard } from "./role-card";
import type { RoleManifest, RoleSkillCatalogEntry } from "@/lib/stores/role-store";

const reviewRole: RoleManifest = {
  apiVersion: "agentforge/v1",
  kind: "Role",
  metadata: {
    id: "reviewer",
    name: "Reviewer",
    version: "1.0.0",
    description: "Reviews pull requests and flags risks.",
    author: "AgentForge",
    tags: ["review", "safety"],
  },
  identity: {
    role: "Code Reviewer",
    goal: "Protect production quality",
    backstory: "Experienced reviewer",
    systemPrompt: "Review thoroughly",
    persona: "Calm",
    goals: ["Review"],
    constraints: ["Do not merge"],
  },
  capabilities: {
    allowedTools: ["Read", "Grep"],
    languages: ["TypeScript"],
    frameworks: ["Next.js"],
    skills: [
      { path: "skills/review", autoLoad: true },
      { path: "skills/security", autoLoad: false },
    ],
    maxTurns: 12,
    maxBudgetUsd: 3,
  },
  knowledge: {
    repositories: ["app"],
    documents: ["docs/PRD.md"],
    patterns: ["review"],
  },
  security: {
    permissionMode: "default",
    allowedPaths: ["app/"],
    deniedPaths: ["secrets/"],
    maxBudgetUsd: 3,
    requireReview: true,
  },
  extends: "base-reviewer",
};

const skillCatalog: RoleSkillCatalogEntry[] = [
  {
    path: "skills/review",
    label: "Review",
    source: "repo-local",
    sourceRoot: "skills",
  },
];

describe("RoleCard", () => {
  it("surfaces execution-relevant summaries and safety cues", () => {
    render(
      <RoleCard role={reviewRole} skillCatalog={skillCatalog} onEdit={jest.fn()} onDelete={jest.fn()} />,
    );

    expect(screen.getByText("Extends base-reviewer")).toBeInTheDocument();
    expect(screen.getByText("1 auto / 1 on-demand")).toBeInTheDocument();
    expect(screen.getByText("Review gate: required before execution")).toBeInTheDocument();
    expect(screen.getByText("1 unresolved")).toBeInTheDocument();
  });
});
