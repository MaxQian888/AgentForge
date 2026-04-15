#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const { spawnSync } = require("node:child_process");
const { resolve } = require("node:path");
const { getRepoRoot, resolveBuildTarget } = require("./plugin-dev-targets.js");

function parseArgs(argv = process.argv.slice(2)) {
  const parsed = {
    operation: "health",
  };

  for (let index = 0; index < argv.length; index += 1) {
    const value = argv[index];
    if (value === "--manifest") {
      parsed.manifestPath = argv[index + 1];
      index += 1;
      continue;
    }
    if (value === "--operation") {
      parsed.operation = argv[index + 1];
      index += 1;
      continue;
    }
    if (value === "--config") {
      parsed.config = argv[index + 1];
      index += 1;
      continue;
    }
    if (value === "--payload") {
      parsed.payload = argv[index + 1];
      index += 1;
    }
  }

  return parsed;
}

function runDebugCommand({
  manifestPath,
  operation = "health",
  config,
  payload,
  repoRoot = getRepoRoot(),
} = {}) {
  const target = resolveBuildTarget({
    manifestPath,
    repoRoot,
  });
  const args = [
    "run",
    "./cmd/plugin-debugger",
    "--manifest",
    target.manifestPath,
    "--operation",
    operation,
  ];

  if (config) {
    args.push("--config", typeof config === "string" ? config : JSON.stringify(config));
  }

  if (payload) {
    args.push("--payload", typeof payload === "string" ? payload : JSON.stringify(payload));
  }

  const result = spawnSync("go", args, {
    cwd: resolve(repoRoot, "src-go"),
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });

  let output = null;
  const trimmedStdout = (result.stdout || "").trim();
  if (trimmedStdout) {
    try {
      output = JSON.parse(trimmedStdout);
    } catch (error) {
      output = {
        ok: false,
        operation,
        error: `failed to parse debugger output: ${error instanceof Error ? error.message : "unknown error"}`,
      };
    }
  }

  return {
    status: result.status ?? 1,
    stdout: result.stdout || "",
    stderr: result.stderr || "",
    output,
  };
}

function main(argv = process.argv.slice(2)) {
  const result = runDebugCommand(parseArgs(argv));

  if (result.stdout) {
    process.stdout.write(result.stdout);
  }
  if (result.stderr) {
    process.stderr.write(result.stderr);
  }
  if (result.status !== 0) {
    process.exit(result.status);
  }
}

if (require.main === module) {
  main();
}

module.exports = {
  main,
  parseArgs,
  runDebugCommand,
};
