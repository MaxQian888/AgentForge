/** @jest-environment node */

import fs from "node:fs";
import os from "node:os";
import path from "node:path";

export {};

async function loadGovernanceModule() {
  return import("./internal-skill-governance.js");
}

describe("internal-skill-governance", () => {
  test("loads the repo internal skill registry", async () => {
    const { loadInternalSkillRegistry } = await loadGovernanceModule();

    const registry = loadInternalSkillRegistry();
    const ids = registry.entries.map((entry: { id: string }) => entry.id);

    expect(ids).toEqual(
      expect.arrayContaining(["react", "echo-go-backend", "shadcn", "openspec-propose"]),
    );
  });

  test("flags workflow mirror drift for declared mirrors", async () => {
    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-internal-skills-"));
    fs.mkdirSync(path.join(repoRoot, ".codex", "skills", "openspec-propose"), { recursive: true });
    fs.mkdirSync(path.join(repoRoot, ".claude", "skills", "openspec-propose"), { recursive: true });
    fs.mkdirSync(path.join(repoRoot, ".github", "skills", "openspec-propose"), { recursive: true });
    fs.writeFileSync(
      path.join(repoRoot, "internal-skills.yaml"),
      [
        "version: 1",
        "skills:",
        "  - id: openspec-propose",
        "    family: workflow-mirror",
        "    verificationProfile: workflow-mirror",
        "    canonicalRoot: .codex/skills/openspec-propose",
        "    sourceType: repo-authored",
        "    mirrorTargets:",
        "      - .claude/skills/openspec-propose/SKILL.md",
        "      - .github/skills/openspec-propose/SKILL.md",
      ].join("\n"),
    );
    fs.writeFileSync(
      path.join(repoRoot, ".codex", "skills", "openspec-propose", "SKILL.md"),
      ["---", "name: OpenSpec Propose", "description: Canonical workflow skill", "---", "", "# Canonical"].join("\n"),
    );
    fs.writeFileSync(
      path.join(repoRoot, ".claude", "skills", "openspec-propose", "SKILL.md"),
      ["---", "name: OpenSpec Propose", "description: Drifted workflow skill", "---", "", "# Drifted"].join("\n"),
    );
    fs.writeFileSync(
      path.join(repoRoot, ".github", "skills", "openspec-propose", "SKILL.md"),
      ["---", "name: OpenSpec Propose", "description: Canonical workflow skill", "---", "", "# Canonical"].join("\n"),
    );

    const { runInternalSkillVerification } = await loadGovernanceModule();

    const result = runInternalSkillVerification({ repoRoot });
    expect(result.ok).toBe(false);
    if (result.ok || !result.failures) {
      throw new Error("expected workflow mirror verification to fail");
    }
    expect(result.failures[0]?.issues).toEqual(
      expect.arrayContaining([
        expect.stringContaining(".claude/skills/openspec-propose/SKILL.md"),
      ]),
    );
  });

  test("accepts upstream-synced skills with lockfile-backed exceptions", async () => {
    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-internal-skills-"));
    const skillDir = path.join(repoRoot, ".agents", "skills", "shadcn");
    fs.mkdirSync(path.join(skillDir, "agents"), { recursive: true });
    fs.writeFileSync(
      path.join(repoRoot, "internal-skills.yaml"),
      [
        "version: 1",
        "skills:",
        "  - id: shadcn",
        "    family: repo-assistant",
        "    verificationProfile: repo-assistant",
        "    canonicalRoot: .agents/skills/shadcn",
        "    sourceType: upstream-sync",
        "    lockKey: shadcn",
        "    allowedExceptions:",
        "      - noncanonical-agent-config-extension",
      ].join("\n"),
    );
    fs.writeFileSync(
      path.join(repoRoot, "skills-lock.json"),
      JSON.stringify(
        {
          version: 1,
          skills: {
            shadcn: {
              source: "shadcn/ui",
              sourceType: "github",
              computedHash: "demo-hash",
            },
          },
        },
        null,
        2,
      ),
    );
    fs.writeFileSync(
      path.join(skillDir, "SKILL.md"),
      ["---", "name: shadcn", "description: UI component skill", "user-invocable: false", "---", "", "# shadcn"].join("\n"),
    );
    fs.writeFileSync(
      path.join(skillDir, "agents", "openai.yml"),
      ["interface:", '  display_name: "shadcn/ui"'].join("\n"),
    );

    const { runInternalSkillVerification } = await loadGovernanceModule();

    const result = runInternalSkillVerification({ repoRoot });
    expect(result).toEqual(
      expect.objectContaining({
        ok: true,
        verifiedSkills: ["shadcn"],
      }),
    );
  });

  test("synchronizes workflow mirrors from the canonical source", async () => {
    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-internal-skills-"));
    fs.mkdirSync(path.join(repoRoot, ".codex", "skills", "openspec-apply-change"), { recursive: true });
    fs.mkdirSync(path.join(repoRoot, ".claude", "skills", "openspec-apply-change"), { recursive: true });
    fs.mkdirSync(path.join(repoRoot, ".github", "skills", "openspec-apply-change"), { recursive: true });
    fs.writeFileSync(
      path.join(repoRoot, "internal-skills.yaml"),
      [
        "version: 1",
        "skills:",
        "  - id: openspec-apply-change",
        "    family: workflow-mirror",
        "    verificationProfile: workflow-mirror",
        "    canonicalRoot: .codex/skills/openspec-apply-change",
        "    sourceType: repo-authored",
        "    mirrorTargets:",
        "      - .claude/skills/openspec-apply-change/SKILL.md",
        "      - .github/skills/openspec-apply-change/SKILL.md",
      ].join("\n"),
    );
    const canonical = ["---", "name: OpenSpec Apply", "description: Canonical apply skill", "---", "", "# Apply"].join("\n");
    fs.writeFileSync(path.join(repoRoot, ".codex", "skills", "openspec-apply-change", "SKILL.md"), canonical);
    fs.writeFileSync(path.join(repoRoot, ".claude", "skills", "openspec-apply-change", "SKILL.md"), "# stale");
    fs.writeFileSync(path.join(repoRoot, ".github", "skills", "openspec-apply-change", "SKILL.md"), "# stale");

    const { syncInternalSkillMirrors } = await loadGovernanceModule();

    const result = syncInternalSkillMirrors({ repoRoot });
    expect(result.updated).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          skillId: "openspec-apply-change",
        }),
      ]),
    );
    expect(
      fs.readFileSync(path.join(repoRoot, ".claude", "skills", "openspec-apply-change", "SKILL.md"), "utf8"),
    ).toBe(canonical);
    expect(
      fs.readFileSync(path.join(repoRoot, ".github", "skills", "openspec-apply-change", "SKILL.md"), "utf8"),
    ).toBe(canonical);
  });
});
