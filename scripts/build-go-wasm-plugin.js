/* eslint-disable @typescript-eslint/no-require-imports */

const { mkdirSync } = require("node:fs");
const { dirname, resolve } = require("node:path");
const { spawnSync } = require("node:child_process");
const {
  getRepoRoot,
  resolveBuildTarget,
} = require("./plugin-dev-targets.js");

function parseArgs(argv = process.argv.slice(2)) {
  const parsed = {};

  for (let index = 0; index < argv.length; index += 1) {
    const value = argv[index];
    if (value === "--manifest") {
      parsed.manifestPath = argv[index + 1];
      index += 1;
      continue;
    }
    if (value === "--source") {
      parsed.sourcePath = argv[index + 1];
      index += 1;
      continue;
    }
    if (value === "--output") {
      parsed.outputPath = argv[index + 1];
      index += 1;
    }
  }

  return parsed;
}

function main(argv = process.argv.slice(2)) {
  const repoRoot = getRepoRoot();
  const target = resolveBuildTarget({
    ...parseArgs(argv),
    repoRoot,
  });

  mkdirSync(dirname(target.modulePath), { recursive: true });

  console.log(
    `-> Building ${target.pluginId} -> ${target.modulePath}`,
  );

  const result = spawnSync(
    "go",
    ["build", "-o", target.modulePath, target.sourcePath],
    {
      cwd: resolve(repoRoot, "src-go"),
      stdio: "inherit",
      env: {
        ...process.env,
        GOOS: "wasip1",
        GOARCH: "wasm",
        CGO_ENABLED: "0",
      },
    },
  );

  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }

  console.log(`   ok ${target.modulePath}`);
}

if (require.main === module) {
  main();
}

module.exports = {
  main,
  parseArgs,
  resolveBuildTarget,
};
