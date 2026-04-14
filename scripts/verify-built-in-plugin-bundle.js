#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const { getRepoRoot } = require("./plugin-dev-targets.js");
const { createVerificationStages: createGoWasmVerificationStages } = require("./verify-plugin-dev-workflow.js");
const { runBuild } = require("./build-go-wasm-plugin.js");
const { runDebugCommand } = require("./debug-go-wasm-plugin.js");

function loadBuiltInBundle({ repoRoot = getRepoRoot() } = {}) {
  const bundlePath = path.join(repoRoot, "plugins", "builtin-bundle.yaml");
  const source = fs.readFileSync(bundlePath, "utf8");
  const parsed = JSON.parse(source);
  const entries = Array.isArray(parsed.plugins) ? parsed.plugins : [];
  return {
    bundlePath,
    entries,
  };
}

function normalizeHost(value) {
  if (value === "win32") {
    return "windows";
  }
  return String(value || "").trim().toLowerCase();
}

function validateReadinessContract(entry) {
  const readiness = entry?.readiness;
  const issues = [];
  if (!readiness || typeof readiness !== "object") {
    issues.push("missing readiness contract");
    return issues;
  }

  if (!String(readiness.readyMessage || "").trim()) {
    issues.push("missing readiness.readyMessage");
  }

  const hasBlockingChecks =
    (Array.isArray(readiness.prerequisites) && readiness.prerequisites.length > 0) ||
    (Array.isArray(readiness.configuration) && readiness.configuration.length > 0) ||
    (Array.isArray(readiness.supportedHosts) && readiness.supportedHosts.length > 0);
  if (hasBlockingChecks && !String(readiness.blockedMessage || "").trim()) {
    issues.push("missing readiness.blockedMessage");
  }
  if (hasBlockingChecks && !String(readiness.nextStep || "").trim()) {
    issues.push("missing readiness.nextStep");
  }

  for (const [groupName, list] of [
    ["prerequisites", readiness.prerequisites],
    ["configuration", readiness.configuration],
  ]) {
    if (!Array.isArray(list)) {
      continue;
    }
    list.forEach((item, index) => {
      if (!String(item?.kind || "").trim()) {
        issues.push(`missing readiness.${groupName}[${index}].kind`);
      }
      if (!String(item?.value || "").trim()) {
        issues.push(`missing readiness.${groupName}[${index}].value`);
      }
      if (!String(item?.label || "").trim()) {
        issues.push(`missing readiness.${groupName}[${index}].label`);
      }
    });
  }

  return issues;
}

function requiresStarterCatalogMetadata(entry) {
  const kind = String(entry?.kind || "").trim();
  return kind === "ToolPlugin" || kind === "WorkflowPlugin";
}

function validateStarterCatalogMetadata(entry) {
  const issues = [];
  if (!requiresStarterCatalogMetadata(entry)) {
    return issues;
  }

  if (!String(entry?.starterFamily || "").trim()) {
    issues.push("missing starterFamily");
  }

  if (!Array.isArray(entry?.coreFlows) || entry.coreFlows.length === 0) {
    issues.push("missing coreFlows");
  }

  if (!Array.isArray(entry?.dependencyRefs) || entry.dependencyRefs.length === 0) {
    issues.push("missing dependencyRefs");
  }

  if (!Array.isArray(entry?.workspaceRefs) || entry.workspaceRefs.length === 0) {
    issues.push("missing workspaceRefs");
  }

  return issues;
}

function evaluateReadiness(entry, {
  env = process.env,
  hasExecutable = (value) => {
    const pathValue = env.PATH ?? env.Path ?? "";
    const parts = String(pathValue).split(path.delimiter).filter(Boolean);
    const extensions = normalizeHost(process.platform) === "windows"
      ? String(env.PATHEXT || ".EXE;.CMD;.BAT;.COM")
          .split(";")
          .filter(Boolean)
      : [""];
    return parts.some((segment) => {
      return extensions.some((extension) => {
        const candidate = path.join(segment, extension ? `${value}${extension}` : value);
        return fs.existsSync(candidate);
      });
    });
  },
  host = process.platform,
} = {}) {
  const readiness = entry?.readiness;
  if (!readiness || typeof readiness !== "object") {
    return {
      status: entry?.availability?.status ?? "ready",
      message: entry?.availability?.message ?? "",
      nextStep: "",
      blockingReasons: [],
      missingPrerequisites: [],
      missingConfiguration: [],
      installable: true,
    };
  }

  const installable = readiness.installable !== false;
  const normalizedHost = normalizeHost(host);
  const supportedHosts = Array.isArray(readiness.supportedHosts)
    ? readiness.supportedHosts.map(normalizeHost).filter(Boolean)
    : [];
  if (supportedHosts.length > 0 && !supportedHosts.includes(normalizedHost)) {
    return {
      status: "unsupported_host",
      message: readiness.blockedMessage || "Built-in plugin is unsupported on this host.",
      nextStep: readiness.nextStep || "",
      blockingReasons: ["unsupported_host"],
      missingPrerequisites: [],
      missingConfiguration: [],
      installable,
    };
  }

  const missingPrerequisites = Array.isArray(readiness.prerequisites)
    ? readiness.prerequisites
        .filter((item) => String(item?.kind || "").trim().toLowerCase() === "executable")
        .filter((item) => !hasExecutable(String(item.value)))
        .map((item) => item.label)
    : [];
  if (missingPrerequisites.length > 0) {
    return {
      status: "requires_prerequisite",
      message: readiness.blockedMessage || "Built-in plugin requires a local prerequisite before activation can succeed.",
      nextStep: readiness.nextStep || "",
      blockingReasons: ["missing_prerequisite"],
      missingPrerequisites,
      missingConfiguration: [],
      installable,
    };
  }

  const missingConfiguration = Array.isArray(readiness.configuration)
    ? readiness.configuration
        .filter((item) => String(item?.kind || "").trim().toLowerCase() === "env")
        .filter((item) => !String(env[item.value] || "").trim())
        .map((item) => item.label)
    : [];
  if (missingConfiguration.length > 0) {
    return {
      status: "requires_configuration",
      message: readiness.blockedMessage || "Built-in plugin requires configuration before activation can succeed.",
      nextStep: readiness.nextStep || "",
      blockingReasons: ["missing_configuration"],
      missingPrerequisites: [],
      missingConfiguration,
      installable,
    };
  }

  return {
    status: "ready",
    message: readiness.readyMessage || entry?.availability?.message || "Built-in plugin is ready for install.",
    nextStep: readiness.nextStep || "",
    blockingReasons: [],
    missingPrerequisites: [],
    missingConfiguration: [],
    installable,
  };
}

function createBundleVerificationPlan(bundle, { repoRoot = getRepoRoot() } = {}) {
  return bundle.entries.map((entry) => {
    const manifestPath = path.join("plugins", entry.manifest);
    const stages = createStagesForProfile(entry.verificationProfile, manifestPath, { repoRoot });
    return {
      pluginId: entry.id,
      manifestPath,
      verificationProfile: entry.verificationProfile,
      stages,
    };
  });
}

function createStagesForProfile(profile, manifestPath, { repoRoot = getRepoRoot() } = {}) {
  const pluginDir = path.dirname(path.join(repoRoot, manifestPath));
  switch (profile) {
    case "go-wasm":
    case "workflow-wasm":
      return createGoWasmVerificationStages({ manifestPath });
    case "mcp-review": {
      return [
        { name: "manifest" },
        { name: "package-validate", cwd: pluginDir },
      ];
    }
    case "mcp-tool":
      return fs.existsSync(path.join(pluginDir, "package.json"))
        ? [
            { name: "manifest" },
            { name: "package-validate", cwd: pluginDir },
          ]
        : [{ name: "manifest" }];
    default:
      return [{ name: "manifest" }];
  }
}

function verifyManifest(repoRoot, manifestPath) {
  const absolutePath = path.join(repoRoot, manifestPath);
  if (!fs.existsSync(absolutePath)) {
    return {
      ok: false,
      stage: "manifest",
      stderr: `Missing manifest: ${manifestPath}\n`,
    };
  }
  return { ok: true };
}

function runPackageValidate(item, stage, repoRoot) {
  const packageJsonPath = path.join(stage.cwd ?? repoRoot, "package.json");
  if (!fs.existsSync(packageJsonPath)) {
    return {
      ok: false,
      pluginId: item.pluginId,
      stage: stage.name,
      stdout: "",
      stderr: `Missing package.json for ${item.pluginId}\n`,
    };
  }

  const pkg = JSON.parse(fs.readFileSync(packageJsonPath, "utf8"));
  if (!pkg.scripts || typeof pkg.scripts.validate !== "string" || !pkg.scripts.validate.trim()) {
    return {
      ok: false,
      pluginId: item.pluginId,
      stage: stage.name,
      stdout: "",
      stderr: `Missing validate script for ${item.pluginId}\n`,
    };
  }

  const result = spawnSync("bun", ["run", "validate"], {
    cwd: stage.cwd ?? repoRoot,
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });

  if (result.error && result.error.code === "ENOENT") {
    return {
      ok: false,
      pluginId: item.pluginId,
      stage: stage.name,
      stdout: result.stdout || "",
      stderr: `Missing prerequisite executable bun for ${item.pluginId}\n`,
    };
  }

  if (result.status !== 0) {
    return {
      ok: false,
      pluginId: item.pluginId,
      stage: stage.name,
      stdout: result.stdout || "",
      stderr: result.stderr || "",
    };
  }

  return { ok: true };
}

function runBundleVerification({ repoRoot = getRepoRoot() } = {}) {
  const bundle = loadBuiltInBundle({ repoRoot });
  const plan = createBundleVerificationPlan(bundle, { repoRoot });

  for (const entry of bundle.entries) {
    const issues = validateReadinessContract(entry);
    if (issues.length > 0) {
      return {
        ok: false,
        pluginId: entry.id,
        stage: "readiness-contract",
        stdout: "",
        stderr: `${issues.join("\n")}\n`,
      };
    }
  }

  for (const entry of bundle.entries) {
    const issues = validateStarterCatalogMetadata(entry);
    if (issues.length > 0) {
      return {
        ok: false,
        pluginId: entry.id,
        stage: "starter-catalog",
        stdout: "",
        stderr: `${issues.join("\n")}\n`,
      };
    }
  }

  for (const item of plan) {
    for (const stage of item.stages) {
      if (stage.name === "manifest") {
        const manifestResult = verifyManifest(repoRoot, item.manifestPath);
        if (!manifestResult.ok) {
          return {
            ok: false,
            pluginId: item.pluginId,
            stage: stage.name,
            stdout: "",
            stderr: manifestResult.stderr,
          };
        }
        continue;
      }

      if (stage.name === "package-validate") {
        const packageResult = runPackageValidate(item, stage, repoRoot);
        if (!packageResult.ok) {
          return packageResult;
        }
        continue;
      }

      const result = runManagedStage(stage, item.manifestPath, repoRoot);
      if (result.status !== 0) {
        return {
          ok: false,
          pluginId: item.pluginId,
          stage: stage.name,
          stdout: result.stdout || "",
          stderr: result.stderr || "",
        };
      }
    }
  }

  return {
    ok: true,
    stages: plan.map((item) => ({
      pluginId: item.pluginId,
      stages: item.stages.map((stage) => stage.name),
    })),
  };
}

function runManagedStage(stage, manifestPath, repoRoot) {
  if (stage.script === "scripts/build-go-wasm-plugin.js") {
    return runBuild({ manifestPath, repoRoot });
  }
  if (stage.script === "scripts/debug-go-wasm-plugin.js") {
    if (process.env.AGENTFORGE_RUN_WASM_DEBUG_SMOKE !== "1") {
      return {
        status: 0,
        stdout: "",
        stderr: "",
      };
    }
    return runDebugCommand({
      manifestPath,
      operation: "health",
      repoRoot,
    });
  }
  return {
    status: 1,
    stdout: "",
    stderr: `Unsupported verification stage script: ${stage.script}\n`,
  };
}

function main() {
  const result = runBundleVerification();
  if (!result.ok) {
    console.error(`Built-in plugin verification failed for ${result.pluginId} at stage ${result.stage}`);
    if (result.stdout) {
      process.stdout.write(result.stdout);
    }
    if (result.stderr) {
      process.stderr.write(result.stderr);
    }
    process.exit(1);
  }

  for (const item of result.stages) {
    console.log(`${item.pluginId}: ${item.stages.join(" -> ")}`);
  }
}

if (require.main === module) {
  main();
}

module.exports = {
  createBundleVerificationPlan,
  createStagesForProfile,
  evaluateReadiness,
  loadBuiltInBundle,
  main,
  runBundleVerification,
  validateStarterCatalogMetadata,
  validateReadinessContract,
};
