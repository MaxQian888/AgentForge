#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require("node:fs");
const path = require("node:path");
const YAML = require("yaml");

const { getRepoRoot } = require("../plugin/plugin-dev-targets.js");

const INTERNAL_SKILL_REGISTRY_FILE = "internal-skills.yaml";
const SKILLS_LOCK_FILE = "skills-lock.json";
const KNOWN_SKILL_SCAN_ROOTS = [
  "skills",
  ".agents/skills",
  ".codex/skills",
  ".claude/skills",
  ".github/skills",
];
const VALID_FAMILIES = new Set([
  "built-in-runtime",
  "repo-assistant",
  "workflow-mirror",
]);
const VALID_SOURCE_TYPES = new Set([
  "repo-authored",
  "upstream-sync",
  "generated-mirror",
]);
const VALID_EXCEPTIONS = new Set([
  "noncanonical-agent-config-extension",
]);

function normalizeRelativePath(value) {
  const normalized = String(value || "").trim().replaceAll("\\", "/");
  if (!normalized) {
    return "";
  }
  return normalized.startsWith("./") ? normalized.slice(2) : normalized;
}

function normalizeDirectoryPath(value) {
  return normalizeRelativePath(value).replace(/\/+$/, "");
}

function normalizeFilePath(value) {
  return normalizeRelativePath(value);
}

function resolveRepoPath(repoRoot, relativePath) {
  return path.resolve(repoRoot, relativePath);
}

function loadYamlFile(filePath) {
  const source = fs.readFileSync(filePath, "utf8");
  return YAML.parse(source) || {};
}

function loadInternalSkillRegistry({ repoRoot = getRepoRoot() } = {}) {
  const registryPath = path.join(repoRoot, INTERNAL_SKILL_REGISTRY_FILE);
  const parsed = loadYamlFile(registryPath);
  const entries = Array.isArray(parsed.skills) ? parsed.skills.map((entry) => normalizeRegistryEntry(entry)) : [];
  return {
    registryPath,
    entries,
  };
}

function normalizeRegistryEntry(entry) {
  return {
    id: String(entry?.id || "").trim(),
    family: String(entry?.family || "").trim(),
    verificationProfile: String(entry?.verificationProfile || "").trim(),
    canonicalRoot: normalizeDirectoryPath(entry?.canonicalRoot),
    sourceType: String(entry?.sourceType || "").trim(),
    lockKey: String(entry?.lockKey || "").trim(),
    mirrorTargets: Array.isArray(entry?.mirrorTargets)
      ? entry.mirrorTargets.map((target) => normalizeFilePath(target)).filter(Boolean)
      : [],
    allowedExceptions: Array.isArray(entry?.allowedExceptions)
      ? entry.allowedExceptions.map((item) => String(item || "").trim()).filter(Boolean)
      : [],
    docsRef: String(entry?.docsRef || "").trim(),
  };
}

function loadSkillsLock({ repoRoot = getRepoRoot() } = {}) {
  const lockPath = path.join(repoRoot, SKILLS_LOCK_FILE);
  if (!fs.existsSync(lockPath)) {
    return { lockPath, skills: {} };
  }

  const parsed = JSON.parse(fs.readFileSync(lockPath, "utf8"));
  return {
    lockPath,
    skills: parsed?.skills && typeof parsed.skills === "object" ? parsed.skills : {},
  };
}

function loadBuiltInSkillBundle({ repoRoot = getRepoRoot() } = {}) {
  const bundlePath = path.join(repoRoot, "skills", "builtin-bundle.yaml");
  if (!fs.existsSync(bundlePath)) {
    return {
      bundlePath,
      entries: [],
    };
  }

  const parsed = loadYamlFile(bundlePath);
  return {
    bundlePath,
    entries: Array.isArray(parsed.skills)
      ? parsed.skills.map((entry) => ({
          id: String(entry?.id || "").trim(),
          root: normalizeDirectoryPath(path.posix.join("skills", String(entry?.root || "").trim().replace(/^skills\//, ""))),
          category: String(entry?.category || "").trim(),
          tags: Array.isArray(entry?.tags) ? entry.tags : [],
        }))
      : [],
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

function readCanonicalSkillPackage(entry, { repoRoot = getRepoRoot() } = {}) {
  const canonicalDir = resolveRepoPath(repoRoot, entry.canonicalRoot);
  const skillDocPath = path.join(canonicalDir, "SKILL.md");
  if (!fs.existsSync(skillDocPath)) {
    return {
      skillDocPath,
      document: null,
      agentFiles: [],
    };
  }

  const document = parseSkillDocument(fs.readFileSync(skillDocPath, "utf8"));
  const agentsDir = path.join(canonicalDir, "agents");
  const agentFiles = fs.existsSync(agentsDir)
    ? fs.readdirSync(agentsDir)
        .filter((name) => /\.ya?ml$/i.test(name))
        .map((name) => ({
          relativePath: normalizeFilePath(path.posix.join(entry.canonicalRoot, "agents", name)),
          absolutePath: path.join(agentsDir, name),
          name,
        }))
    : [];

  return {
    skillDocPath,
    document,
    agentFiles,
  };
}

function inferScanRoot(relativePath) {
  const normalized = normalizeRelativePath(relativePath);
  if (!normalized) {
    return "";
  }

  for (const root of KNOWN_SKILL_SCAN_ROOTS) {
    if (normalized === root || normalized.startsWith(`${root}/`)) {
      return root;
    }
  }

  return "";
}

function collectDiscoveredSkillPackages({ repoRoot = getRepoRoot(), scanRoots = KNOWN_SKILL_SCAN_ROOTS } = {}) {
  const discovered = [];

  for (const root of scanRoots) {
    const absoluteRoot = resolveRepoPath(repoRoot, root);
    if (!fs.existsSync(absoluteRoot)) {
      continue;
    }

    walkFiles(absoluteRoot, (absoluteFilePath) => {
      if (path.basename(absoluteFilePath).toLowerCase() !== "skill.md") {
        return;
      }
      const relativeFilePath = normalizeFilePath(path.relative(repoRoot, absoluteFilePath));
      discovered.push({
        root: normalizeDirectoryPath(path.posix.dirname(relativeFilePath)),
        filePath: relativeFilePath,
      });
    });
  }

  return discovered;
}

function walkFiles(root, visitor) {
  const entries = fs.readdirSync(root, { withFileTypes: true });
  for (const entry of entries) {
    const absolutePath = path.join(root, entry.name);
    if (entry.isDirectory()) {
      walkFiles(absolutePath, visitor);
      continue;
    }
    if (entry.isFile()) {
      visitor(absolutePath);
    }
  }
}

function validateRegistryEntry(entry, context) {
  const issues = [];
  const allowedExceptions = new Set(entry.allowedExceptions || []);
  const bundleEntry = context.builtInBundleById.get(entry.id);
  const canonicalDir = resolveRepoPath(context.repoRoot, entry.canonicalRoot);

  if (!entry.id) {
    issues.push("missing id");
  }
  if (!VALID_FAMILIES.has(entry.family)) {
    issues.push(`invalid family ${entry.family || "<missing>"}`);
  }
  if (!entry.verificationProfile) {
    issues.push("missing verificationProfile");
  }
  if (!entry.canonicalRoot) {
    issues.push("missing canonicalRoot");
  }
  if (!VALID_SOURCE_TYPES.has(entry.sourceType)) {
    issues.push(`invalid sourceType ${entry.sourceType || "<missing>"}`);
  }

  for (const exception of allowedExceptions) {
    if (!VALID_EXCEPTIONS.has(exception)) {
      issues.push(`unknown allowedException ${exception}`);
    }
  }

  if (entry.family === "built-in-runtime" && !entry.canonicalRoot.startsWith("skills/")) {
    issues.push("built-in-runtime canonicalRoot must stay under skills/");
  }
  if (entry.family === "repo-assistant" && !entry.canonicalRoot.startsWith(".agents/skills/")) {
    issues.push("repo-assistant canonicalRoot must stay under .agents/skills/");
  }
  if (entry.family === "workflow-mirror" && !entry.canonicalRoot.startsWith(".codex/skills/")) {
    issues.push("workflow-mirror canonicalRoot must stay under .codex/skills/");
  }

  if (entry.family === "workflow-mirror" && entry.mirrorTargets.length === 0) {
    issues.push("workflow-mirror skills must declare mirrorTargets");
  }

  if (!fs.existsSync(canonicalDir)) {
    issues.push(`missing canonicalRoot at ${entry.canonicalRoot}`);
    return issues;
  }

  const { skillDocPath, document, agentFiles } = readCanonicalSkillPackage(entry, { repoRoot: context.repoRoot });
  if (!fs.existsSync(skillDocPath)) {
    issues.push(`missing SKILL.md at ${entry.canonicalRoot}`);
    return issues;
  }

  if (!String(document?.frontmatter?.name || "").trim()) {
    issues.push("missing SKILL.md frontmatter name");
  }
  if (!String(document?.frontmatter?.description || "").trim()) {
    issues.push("missing SKILL.md frontmatter description");
  }

  if (entry.sourceType === "upstream-sync") {
    if (!entry.lockKey) {
      issues.push("upstream-sync skills must declare lockKey");
    } else if (!context.skillsLock.skills[entry.lockKey]) {
      issues.push(`missing ${SKILLS_LOCK_FILE} entry for lockKey ${entry.lockKey}`);
    }
  }

  if (entry.family === "built-in-runtime") {
    if (!bundleEntry) {
      issues.push(`missing ${INTERNAL_SKILL_REGISTRY_FILE} alignment with skills/builtin-bundle.yaml for ${entry.id}`);
    } else if (bundleEntry.root !== entry.canonicalRoot) {
      issues.push(`built-in bundle root mismatch for ${entry.id}: expected ${bundleEntry.root}`);
    }

    if (agentFiles.length === 0) {
      issues.push("built-in-runtime profile requires at least one agent config");
    }
  }

  for (const agentFile of agentFiles) {
    if (agentFile.name.endsWith(".yml") && !allowedExceptions.has("noncanonical-agent-config-extension")) {
      issues.push(`noncanonical agent config extension at ${agentFile.relativePath}`);
    }

    try {
      YAML.parse(fs.readFileSync(agentFile.absolutePath, "utf8"));
    } catch (error) {
      issues.push(`invalid agent yaml ${agentFile.relativePath}: ${error.message}`);
    }
  }

  if (entry.family === "workflow-mirror") {
    const canonicalSource = fs.readFileSync(skillDocPath, "utf8");
    for (const mirrorTarget of entry.mirrorTargets) {
      const mirrorAbsolutePath = resolveRepoPath(context.repoRoot, mirrorTarget);
      if (!fs.existsSync(mirrorAbsolutePath)) {
        issues.push(`missing mirror target ${mirrorTarget}`);
        continue;
      }

      const mirrorSource = fs.readFileSync(mirrorAbsolutePath, "utf8");
      if (mirrorSource !== canonicalSource) {
        issues.push(`mirror drift detected at ${mirrorTarget}`);
      }
    }
  }

  return issues;
}

function runInternalSkillVerification({
  repoRoot = getRepoRoot(),
  families,
} = {}) {
  const failures = [];
  const registryPath = path.join(repoRoot, INTERNAL_SKILL_REGISTRY_FILE);
  if (!fs.existsSync(registryPath)) {
    return {
      ok: false,
      failures: [
        {
          skillId: "<registry>",
          issues: [`internal skill governance registry missing at ${INTERNAL_SKILL_REGISTRY_FILE}`],
        },
      ],
    };
  }

  const registry = loadInternalSkillRegistry({ repoRoot });
  const bundle = loadBuiltInSkillBundle({ repoRoot });
  const context = {
    repoRoot,
    skillsLock: loadSkillsLock({ repoRoot }),
    builtInBundleById: new Map(bundle.entries.map((entry) => [entry.id, entry])),
  };

  const requestedFamilies = Array.isArray(families) && families.length > 0 ? new Set(families) : null;
  const filteredEntries = registry.entries.filter((entry) => !requestedFamilies || requestedFamilies.has(entry.family));

  const seenIds = new Set();
  const seenRoots = new Set();
  for (const entry of filteredEntries) {
    const issues = [];

    if (seenIds.has(entry.id)) {
      issues.push(`duplicate skill id ${entry.id}`);
    }
    if (seenRoots.has(entry.canonicalRoot)) {
      issues.push(`duplicate canonicalRoot ${entry.canonicalRoot}`);
    }
    seenIds.add(entry.id);
    seenRoots.add(entry.canonicalRoot);

    issues.push(...validateRegistryEntry(entry, context));
    if (issues.length > 0) {
      failures.push({
        skillId: entry.id || "<unknown>",
        issues,
      });
    }
  }

  if (!requestedFamilies || requestedFamilies.has("built-in-runtime")) {
    for (const bundleEntry of bundle.entries) {
      const match = filteredEntries.find((entry) => entry.family === "built-in-runtime" && entry.id === bundleEntry.id);
      if (!match) {
        failures.push({
          skillId: bundleEntry.id || "<unknown>",
          issues: [`missing internal skill governance entry for built-in-runtime skill ${bundleEntry.id}`],
        });
      }
    }
  }

  const relevantScanRoots = new Set(
    filteredEntries.flatMap((entry) => {
      const roots = [inferScanRoot(entry.canonicalRoot)];
      for (const mirrorTarget of entry.mirrorTargets) {
        roots.push(inferScanRoot(mirrorTarget));
      }
      return roots.filter(Boolean);
    }),
  );

  const expectedRoots = new Set();
  for (const entry of filteredEntries) {
    expectedRoots.add(entry.canonicalRoot);
    for (const mirrorTarget of entry.mirrorTargets) {
      expectedRoots.add(normalizeDirectoryPath(path.posix.dirname(mirrorTarget)));
    }
  }

  if (relevantScanRoots.size > 0) {
    const discovered = collectDiscoveredSkillPackages({
      repoRoot,
      scanRoots: Array.from(relevantScanRoots),
    });

    for (const item of discovered) {
      if (!expectedRoots.has(item.root)) {
        failures.push({
          skillId: item.root,
          issues: [`unregistered skill package discovered at ${item.filePath}`],
        });
      }
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
    verifiedSkills: filteredEntries.map((entry) => entry.id),
  };
}

function syncInternalSkillMirrors({
  repoRoot = getRepoRoot(),
  families = ["workflow-mirror"],
} = {}) {
  const registry = loadInternalSkillRegistry({ repoRoot });
  const requestedFamilies = new Set(families);
  const updated = [];

  for (const entry of registry.entries) {
    if (!requestedFamilies.has(entry.family) || entry.mirrorTargets.length === 0) {
      continue;
    }

    const sourcePath = resolveRepoPath(repoRoot, path.posix.join(entry.canonicalRoot, "SKILL.md"));
    const source = fs.readFileSync(sourcePath, "utf8");
    for (const mirrorTarget of entry.mirrorTargets) {
      const absoluteMirrorPath = resolveRepoPath(repoRoot, mirrorTarget);
      fs.mkdirSync(path.dirname(absoluteMirrorPath), { recursive: true });
      const existing = fs.existsSync(absoluteMirrorPath)
        ? fs.readFileSync(absoluteMirrorPath, "utf8")
        : null;
      if (existing !== source) {
        fs.writeFileSync(absoluteMirrorPath, source);
        updated.push({
          skillId: entry.id,
          mirrorTarget,
        });
      }
    }
  }

  return { updated };
}

function parseArgs(argv) {
  const families = [];
  for (let index = 0; index < argv.length; index += 1) {
    if (argv[index] === "--family" && argv[index + 1]) {
      families.push(argv[index + 1]);
      index += 1;
    }
  }
  return { families };
}

function main(argv = process.argv.slice(2)) {
  const { families } = parseArgs(argv);
  const result = runInternalSkillVerification({
    families: families.length > 0 ? families : undefined,
  });

  if (!result.ok) {
    for (const failure of result.failures) {
      console.error(`Internal skill verification failed for ${failure.skillId}`);
      for (const issue of failure.issues) {
        console.error(`- ${issue}`);
      }
    }
    process.exit(1);
  }

  for (const skillId of result.verifiedSkills) {
    console.log(skillId);
  }
}

if (require.main === module) {
  main();
}

module.exports = {
  INTERNAL_SKILL_REGISTRY_FILE,
  loadBuiltInSkillBundle,
  loadInternalSkillRegistry,
  loadSkillsLock,
  main,
  parseSkillDocument,
  runInternalSkillVerification,
  syncInternalSkillMirrors,
};
