#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { execFileSync, spawnSync } = require("node:child_process");

const ALL_TARGETS = [
  {
    bunTarget: "bun-linux-x64",
    extension: "",
    triple: "x86_64-unknown-linux-gnu",
  },
  {
    bunTarget: "bun-linux-arm64",
    extension: "",
    triple: "aarch64-unknown-linux-gnu",
  },
  {
    bunTarget: "bun-windows-x64",
    extension: ".exe",
    triple: "x86_64-pc-windows-msvc",
  },
  {
    bunTarget: "bun-darwin-x64",
    extension: "",
    triple: "x86_64-apple-darwin",
  },
  {
    bunTarget: "bun-darwin-arm64",
    extension: "",
    triple: "aarch64-apple-darwin",
  },
];

function runAndCapture(command, args, options = {}) {
  try {
    return execFileSync(command, args, {
      cwd: options.cwd,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
  } catch {
    return "";
  }
}

function getFallbackTriple({
  platform = process.platform,
  arch = process.arch,
} = {}) {
  if (platform === "win32" && arch === "x64") {
    return "x86_64-pc-windows-msvc";
  }

  if (platform === "linux" && arch === "x64") {
    return "x86_64-unknown-linux-gnu";
  }

  if (platform === "linux" && arch === "arm64") {
    return "aarch64-unknown-linux-gnu";
  }

  if (platform === "darwin" && arch === "x64") {
    return "x86_64-apple-darwin";
  }

  if (platform === "darwin" && arch === "arm64") {
    return "aarch64-apple-darwin";
  }

  return "x86_64-pc-windows-msvc";
}

function detectHostTriple({ cwd } = {}) {
  const rustcHostTuple = runAndCapture("rustc", ["--print", "host-tuple"], {
    cwd,
  });

  if (rustcHostTuple) {
    return rustcHostTuple;
  }

  const rustcVerbose = runAndCapture("rustc", ["-vV"], { cwd });
  const hostLine = rustcVerbose
    .split(/\r?\n/u)
    .find((line) => line.startsWith("host:"));

  if (hostLine) {
    return hostLine.replace("host:", "").trim();
  }

  return getFallbackTriple({
    platform: os.platform(),
    arch: os.arch(),
  });
}

function resolveTargets({ currentOnly = false, hostTriple } = {}) {
  if (!currentOnly) {
    return [...ALL_TARGETS];
  }

  const resolvedHostTriple = hostTriple ?? detectHostTriple();
  const target = ALL_TARGETS.find(({ triple }) => {
    if (resolvedHostTriple === triple) {
      return true;
    }

    if (
      triple === "x86_64-pc-windows-msvc" &&
      /windows.*amd64|x86_64-pc-windows/u.test(resolvedHostTriple)
    ) {
      return true;
    }

    return false;
  });

  if (target) {
    return [target];
  }

  console.warn(
    `Unknown triple: ${resolvedHostTriple} - defaulting to windows/x64`,
  );

  return [
    {
      bunTarget: "bun-windows-x64",
      extension: ".exe",
      triple: "x86_64-pc-windows-msvc",
    },
  ];
}

function getDirectories() {
  const scriptDir = __dirname;
  const repoRoot = path.resolve(scriptDir, "..");

  return {
    binariesDir: path.join(repoRoot, "src-tauri", "binaries"),
    bridgeDir: path.join(repoRoot, "src-bridge"),
    repoRoot,
  };
}

function getOutputFilename(target) {
  return `bridge-${target.triple}${target.extension}`;
}

function isHostTarget(target, hostTriple) {
  if (target.triple === hostTriple) {
    return true;
  }

  if (
    target.triple === "x86_64-pc-windows-msvc" &&
    /windows.*amd64|x86_64-pc-windows/u.test(hostTriple)
  ) {
    return true;
  }

  return false;
}

function isSkippableCrossTargetError(error) {
  const message = String(error instanceof Error ? error.message : error);

  return (
    /Failed to download/u.test(message) ||
    /UNKNOWN_CERTIFICATE_VERIFICATION_ERROR/u.test(message) ||
    /Failed to extract executable/u.test(message) ||
    /download may be incomplete/u.test(message)
  );
}

function ensureBridgeDependencies(bridgeDir) {
  const result = spawnSync("bun", ["install"], {
    cwd: bridgeDir,
    stdio: "inherit",
  });

  if (result.status !== 0) {
    throw new Error("bun install failed for src-bridge");
  }
}

function buildTarget(target, { bridgeDir, binariesDir }) {
  const outputPath = path.join(binariesDir, getOutputFilename(target));

  console.log(`-> Building ${target.bunTarget} -> ${path.basename(outputPath)}`);

  const result = spawnSync(
    "bun",
    [
      "build",
      "src/server.ts",
      "--compile",
      `--target=${target.bunTarget}`,
      `--outfile=${outputPath}`,
    ],
    {
      cwd: bridgeDir,
      encoding: "utf8",
    },
  );

  if (result.stdout) {
    process.stdout.write(result.stdout);
  }

  if (result.stderr) {
    process.stderr.write(result.stderr);
  }

  if (result.status !== 0) {
    const details = [result.stderr, result.stdout]
      .filter(Boolean)
      .join("\n")
      .trim();
    throw new Error(
      details
        ? `bun build failed for ${target.triple}: ${details}`
        : `bun build failed for ${target.triple}`,
    );
  }

  console.log(`   ok ${path.basename(outputPath)}`);
}

function main(argv = process.argv.slice(2)) {
  const currentOnly = argv.includes("--current-only");
  const directories = getDirectories();
  const hostTriple = detectHostTriple({ cwd: directories.repoRoot });
  const targets = resolveTargets({ currentOnly, hostTriple });
  const ciMode =
    String(process.env.CI || "").toLowerCase() === "true" ||
    process.env.BRIDGE_BUILD_STRICT === "1";
  const skippedTargets = [];
  let hostBuilt = false;

  fs.mkdirSync(directories.binariesDir, { recursive: true });
  ensureBridgeDependencies(directories.bridgeDir);

  if (!currentOnly) {
    console.log("Compiling TS bridge for all supported platforms...");
  }

  for (const target of targets) {
    try {
      buildTarget(target, directories);
      if (isHostTarget(target, hostTriple)) {
        hostBuilt = true;
      }
    } catch (error) {
      const hostTarget = isHostTarget(target, hostTriple);

      if (!ciMode && !hostTarget && isSkippableCrossTargetError(error)) {
        const message =
          error instanceof Error ? error.message : "Unknown bridge build error";
        console.warn(`   skipped ${target.triple}: ${message}`);
        skippedTargets.push(target.triple);
        continue;
      }

      throw error;
    }
  }

  if (!hostBuilt) {
    throw new Error(`Host bridge binary was not built for ${hostTriple}`);
  }

  console.log("");
  if (skippedTargets.length > 0) {
    console.warn(
      `Bridge build skipped cross-target binaries (${skippedTargets.join(", ")}) in non-CI mode.`,
    );
  }
  console.log(
    `Bridge build complete. Binaries in: ${directories.binariesDir}`,
  );
}

if (require.main === module) {
  try {
    main();
  } catch (error) {
    console.error(
      error instanceof Error ? error.message : "Unknown bridge build error",
    );
    process.exit(1);
  }
}

module.exports = {
  ALL_TARGETS,
  detectHostTriple,
  getFallbackTriple,
  getOutputFilename,
  resolveTargets,
};
