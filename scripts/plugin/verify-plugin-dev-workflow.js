#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const { spawnSync } = require("node:child_process");
const { getRepoRoot, resolveBuildTarget } = require("./plugin-dev-targets.js");

function createVerificationStages({ manifestPath }) {
  return [
    {
      name: "build",
      script: "scripts/plugin/build-go-wasm-plugin.js",
      args: ["--manifest", manifestPath],
    },
    {
      name: "debug-health",
      script: "scripts/plugin/debug-go-wasm-plugin.js",
      args: ["--manifest", manifestPath, "--operation", "health"],
    },
  ];
}

function runVerification({ manifestPath, repoRoot = getRepoRoot() } = {}) {
  const target = resolveBuildTarget({ manifestPath, repoRoot });
  const stages = createVerificationStages({
    manifestPath: target.manifestPath,
  });

  for (const stage of stages) {
    const result = spawnSync("node", [stage.script, ...stage.args], {
      cwd: repoRoot,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "pipe"],
    });

    if (result.status !== 0) {
      return {
        ok: false,
        pluginId: target.pluginId,
        stage: stage.name,
        stdout: result.stdout || "",
        stderr: result.stderr || "",
      };
    }
  }

  return {
    ok: true,
    pluginId: target.pluginId,
    stages: stages.map((stage) => stage.name),
  };
}

function main(argv = process.argv.slice(2)) {
  const manifestFlagIndex = argv.indexOf("--manifest");
  const manifestPath =
    manifestFlagIndex >= 0 ? argv[manifestFlagIndex + 1] : undefined;

  const result = runVerification({ manifestPath });
  if (!result.ok) {
    console.error(
      `Verification failed for ${result.pluginId} at stage ${result.stage}`,
    );
    if (result.stdout) {
      process.stdout.write(result.stdout);
    }
    if (result.stderr) {
      process.stderr.write(result.stderr);
    }
    process.exit(1);
  }

  console.log(
    `Verification passed for ${result.pluginId}: ${result.stages.join(" -> ")}`,
  );
}

if (require.main === module) {
  main();
}

module.exports = {
  createVerificationStages,
  main,
  runVerification,
};
