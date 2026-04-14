#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require("node:fs");
const path = require("node:path");
const YAML = require("yaml");

const { getRepoRoot } = require("./plugin-dev-targets.js");
const { runInternalSkillVerification } = require("./internal-skill-governance.js");

function loadBuiltInSkillBundle({ repoRoot = getRepoRoot() } = {}) {
  const bundlePath = path.join(repoRoot, "skills", "builtin-bundle.yaml");
  const source = fs.readFileSync(bundlePath, "utf8");
  const parsed = YAML.parse(source) || {};
  const entries = Array.isArray(parsed.skills) ? parsed.skills : [];
  return {
    bundlePath,
    entries,
  };
}

function validateSkillBundleEntry(entry, { repoRoot = getRepoRoot() } = {}) {
  const issues = [];
  const skillRoot = String(entry?.root || "").trim().replaceAll("\\", "/").replace(/^skills\//, "");
  const skillDir = path.join(repoRoot, "skills", skillRoot);
  const skillDocPath = path.join(skillDir, "SKILL.md");

  if (!String(entry?.id || "").trim()) {
    issues.push("missing id");
  }
  if (!skillRoot) {
    issues.push("missing root");
  }
  if (!String(entry?.category || "").trim()) {
    issues.push("missing category");
  }
  if (!Array.isArray(entry?.tags) || entry.tags.length === 0) {
    issues.push("missing tags");
  }
  if (!fs.existsSync(skillDocPath)) {
    issues.push(`missing SKILL.md at skills/${skillRoot}`);
    return issues;
  }

  const skillSource = fs.readFileSync(skillDocPath, "utf8");
  const parsedDoc = parseSkillDocument(skillSource);
  if (!String(parsedDoc.frontmatter.name || "").trim()) {
    issues.push("missing SKILL.md frontmatter name");
  }
  if (!String(parsedDoc.frontmatter.description || "").trim()) {
    issues.push("missing SKILL.md frontmatter description");
  }

  const agentsDir = path.join(skillDir, "agents");
  if (fs.existsSync(agentsDir)) {
    const agentEntries = fs.readdirSync(agentsDir).filter((name) => /\.ya?ml$/i.test(name));
    for (const agentFile of agentEntries) {
      const raw = fs.readFileSync(path.join(agentsDir, agentFile), "utf8");
      try {
        YAML.parse(raw);
      } catch (error) {
        issues.push(`invalid agent yaml ${path.posix.join("skills", skillRoot, "agents", agentFile)}: ${error.message}`);
      }
    }
  }

  return issues;
}

function runBuiltInSkillBundleVerification({ repoRoot = getRepoRoot() } = {}) {
  const bundle = loadBuiltInSkillBundle({ repoRoot });
  const failures = [];
  const governance = runInternalSkillVerification({
    repoRoot,
    families: ["built-in-runtime"],
  });

  if (!governance.ok) {
    failures.push(...governance.failures);
  }

  for (const entry of bundle.entries) {
    const issues = validateSkillBundleEntry(entry, { repoRoot });
    if (issues.length > 0) {
      failures.push({
        skillId: entry?.id ?? "",
        issues,
      });
    }
  }

  if (failures.length > 0) {
    return {
      ok: false,
      failures,
    };
  }

  return {
    ok: true,
    skills: bundle.entries.map((entry) => entry.id),
  };
}

function parseSkillDocument(content) {
  const lines = String(content).split(/\r?\n/);
  if (lines[0]?.trim() !== "---") {
    return { frontmatter: {}, body: String(content).trim() };
  }

  const frontmatterLines = [];
  const bodyLines = [];
  let closed = false;
  for (const line of lines.slice(1)) {
    if (!closed && line.trim() === "---") {
      closed = true;
      continue;
    }
    if (!closed) {
      frontmatterLines.push(line);
      continue;
    }
    bodyLines.push(line);
  }

  let frontmatter = {};
  if (frontmatterLines.length > 0) {
    try {
      frontmatter = YAML.parse(frontmatterLines.join("\n")) || {};
    } catch {
      frontmatter = {};
    }
  }

  return {
    frontmatter,
    body: bodyLines.join("\n").trim(),
  };
}

function main() {
  const result = runBuiltInSkillBundleVerification();
  if (!result.ok) {
    for (const failure of result.failures) {
      console.error(`Built-in skill verification failed for ${failure.skillId || "<unknown>"}`);
      for (const issue of failure.issues) {
        console.error(`- ${issue}`);
      }
    }
    process.exit(1);
  }

  for (const skill of result.skills) {
    console.log(skill);
  }
}

if (require.main === module) {
  main();
}

module.exports = {
  loadBuiltInSkillBundle,
  parseSkillDocument,
  runBuiltInSkillBundleVerification,
  validateSkillBundleEntry,
};
