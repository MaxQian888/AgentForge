/** @jest-environment node */

import fs from "node:fs";
import os from "node:os";
import path from "node:path";

export {};

async function loadBundleModule() {
  return import("./verify-built-in-skill-bundle.js");
}

describe("verify-built-in-skill-bundle", () => {
  test("loads the official built-in skill bundle from the repository", async () => {
    const { loadBuiltInSkillBundle } = await loadBundleModule();

    const bundle = loadBuiltInSkillBundle();
    const ids = bundle.entries.map((entry: { id: string }) => entry.id);

    expect(ids).toEqual(
      expect.arrayContaining(["react", "typescript", "testing", "css-animation"]),
    );
  });

  test("flags missing skill package files before the marketplace surface can drift", async () => {
    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-skill-builtins-"));
    fs.mkdirSync(path.join(repoRoot, "skills"), { recursive: true });
    fs.writeFileSync(
      path.join(repoRoot, "skills", "builtin-bundle.yaml"),
      [
        "skills:",
        "  - id: react",
        "    root: react",
        "    category: frontend",
        "    tags: [react]",
      ].join("\n"),
    );

    const { runBuiltInSkillBundleVerification } = await loadBundleModule();

    const result = runBuiltInSkillBundleVerification({ repoRoot });
    expect(result.ok).toBe(false);
    if (result.ok || !result.failures) {
      throw new Error("expected built-in skill bundle verification to fail");
    }
    expect(result.failures[0]?.issues).toEqual(
      expect.arrayContaining(["missing SKILL.md at skills/react"]),
    );
  });

  test("accepts bundled skills with valid SKILL.md and agent yaml inputs", async () => {
    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-skill-builtins-"));
    const skillDir = path.join(repoRoot, "skills", "react");
    fs.mkdirSync(path.join(skillDir, "agents"), { recursive: true });
    fs.writeFileSync(
      path.join(repoRoot, "skills", "builtin-bundle.yaml"),
      [
        "skills:",
        "  - id: react",
        "    root: react",
        "    category: frontend",
        "    tags:",
        "      - react",
        "      - nextjs",
      ].join("\n"),
    );
    fs.writeFileSync(
      path.join(skillDir, "SKILL.md"),
      [
        "---",
        "name: React",
        "description: Build React surfaces.",
        "---",
        "",
        "# React",
      ].join("\n"),
    );
    fs.writeFileSync(
      path.join(skillDir, "agents", "openai.yaml"),
      [
        "interface:",
        '  display_name: "AgentForge React"',
        '  short_description: "Build React safely"',
      ].join("\n"),
    );

    const { runBuiltInSkillBundleVerification } = await loadBundleModule();

    const result = runBuiltInSkillBundleVerification({ repoRoot });
    expect(result).toEqual(
      expect.objectContaining({
        ok: true,
        skills: ["react"],
      }),
    );
  });
});
