import { render, screen } from "@testing-library/react";
import { RoleCard } from "./role-card";
import type { RoleManifest } from "@/lib/stores/role-store";

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

describe("RoleCard", () => {
  it("surfaces execution-relevant summaries and safety cues", () => {
    render(
      <RoleCard role={reviewRole} onEdit={jest.fn()} onDelete={jest.fn()} />,
    );

    expect(screen.getByText("Extends base-reviewer")).toBeInTheDocument();
    expect(screen.getByText("Tools: Read, Grep")).toBeInTheDocument();
    expect(screen.getByText("Max budget: $3.00")).toBeInTheDocument();
    expect(screen.getByText("Skills: 1 auto / 1 on-demand")).toBeInTheDocument();
    expect(screen.getByText("Key skills: skills/review, skills/security")).toBeInTheDocument();
    expect(screen.getByText("Review gate: required before execution")).toBeInTheDocument();
    expect(screen.getByText("Path policy: 1 allow / 1 deny")).toBeInTheDocument();
  });
});
