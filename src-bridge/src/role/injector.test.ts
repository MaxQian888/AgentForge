import { describe, expect, test } from "bun:test";
import { buildSystemPrompt, filterTools } from "./injector.js";
import type { RoleConfig } from "../types.js";

const roleConfig: RoleConfig = {
  role_id: "reviewer",
  name: "Reviewer",
  role: "Senior Reviewer",
  goal: "Find risky changes",
  backstory: "A skeptical but helpful reviewer.",
  system_prompt: "Always explain why a suggestion matters.",
  allowed_tools: ["Read", "Bash"],
  max_budget_usd: 2,
  max_turns: 12,
  permission_mode: "default",
};

describe("role injector", () => {
  test("leaves the base prompt alone when no role is provided", () => {
    expect(buildSystemPrompt("Base prompt")).toBe("Base prompt");
  });

  test("prepends role context and filters tools to the allow-list", () => {
    const prompt = buildSystemPrompt("Base prompt", roleConfig);

    expect(prompt).toContain("# Role: Senior Reviewer");
    expect(prompt).toContain("## Goal");
    expect(prompt).toContain("Always explain why a suggestion matters.");
    expect(prompt).toContain("Base prompt");
    expect(filterTools(["Read", "Edit", "Bash"], roleConfig)).toEqual(["Read", "Bash"]);
    expect(filterTools(["Read", "Edit"], undefined)).toEqual(["Read", "Edit"]);
  });

  test("injects knowledge_context into system prompt when present", () => {
    const roleWithKnowledge = {
      ...roleConfig,
      knowledge_context: "This project uses TypeScript strict mode.",
    };
    const prompt = buildSystemPrompt("Base prompt", roleWithKnowledge);
    expect(prompt).toContain("## Knowledge Context\nThis project uses TypeScript strict mode.");
  });

  test("injects loaded skills and summarizes available on-demand skills", () => {
    const prompt = buildSystemPrompt("Base prompt", {
      ...roleConfig,
      loaded_skills: [
        {
          path: "skills/react",
          label: "React",
          description: "React UI implementation guidance",
          available_parts: ["agents", "references"],
          instructions: "Prefer server-safe React composition.",
          source: "repo-local",
          source_root: "skills",
          origin: "direct",
        },
      ],
      available_skills: [
        {
          path: "skills/testing",
          label: "Testing",
          description: "Regression-oriented test guidance",
          available_parts: ["agents"],
          source: "repo-local",
          source_root: "skills",
          origin: "direct",
        },
      ],
    });

    expect(prompt).toContain("## Loaded Skills");
    expect(prompt).toContain("### React (skills/react)");
    expect(prompt).toContain("Bundled parts: agents, references");
    expect(prompt).toContain("Prefer server-safe React composition.");
    expect(prompt).toContain("## Available On-Demand Skills");
    expect(prompt).toContain("Testing (skills/testing): Regression-oriented test guidance [parts: agents]");
  });

  test("omits knowledge_context section when not set", () => {
    const prompt = buildSystemPrompt("Base prompt", roleConfig);
    expect(prompt).not.toContain("Knowledge Context");
  });
});
