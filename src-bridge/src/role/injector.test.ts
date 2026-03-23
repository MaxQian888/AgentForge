import { describe, expect, test } from "bun:test";
import { buildSystemPrompt, filterTools } from "./injector.js";
import type { RoleConfig } from "../types.js";

const roleConfig: RoleConfig = {
  name: "Reviewer",
  goal: "Find risky changes",
  backstory: "A skeptical but helpful reviewer.",
  system_prompt: "Always explain why a suggestion matters.",
  allowed_tools: ["Read", "Bash"],
  max_budget_usd: 2,
  permission_mode: "default",
};

describe("role injector", () => {
  test("leaves the base prompt alone when no role is provided", () => {
    expect(buildSystemPrompt("Base prompt")).toBe("Base prompt");
  });

  test("prepends role context and filters tools to the allow-list", () => {
    const prompt = buildSystemPrompt("Base prompt", roleConfig);

    expect(prompt).toContain("# Role: Reviewer");
    expect(prompt).toContain("## Goal");
    expect(prompt).toContain("Always explain why a suggestion matters.");
    expect(prompt).toContain("Base prompt");
    expect(filterTools(["Read", "Edit", "Bash"], roleConfig)).toEqual(["Read", "Bash"]);
    expect(filterTools(["Read", "Edit"], undefined)).toEqual(["Read", "Edit"]);
  });
});
