#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { execFileSync, spawnSync } = require("node:child_process");

const ALL_TARGETS = [
  { goos: "linux", goarch: "amd64", triple: "x86_64-unknown-linux-gnu" },
  { goos: "linux", goarch: "arm64", triple: "aarch64-unknown-linux-gnu" },
  { goos: "windows", goarch: "amd64", triple: "x86_64-pc-windows-msvc" },
  { goos: "darwin", goarch: "amd64", triple: "x86_64-apple-darwin" },
  { goos: "darwin", goarch: "arm64", triple: "aarch64-apple-darwin" },
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
  const rustcHostTuple = runAndCapture("rustc", ["--print", "host-tuple"], { cwd });

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
    `Unknown triple: ${resolvedHostTriple} - defaulting to windows/amd64`,
  );

  return [
    {
      goos: "windows",
      goarch: "amd64",
      triple: "x86_64-pc-windows-msvc",
    },
  ];
}

function getDirectories() {
  const scriptDir = __dirname;
  const repoRoot = path.resolve(scriptDir, "..", "..");

  return {
    repoRoot,
    goDir: path.join(repoRoot, "src-go"),
    binariesDir: path.join(repoRoot, "src-tauri", "binaries"),
  };
}

function getLdflags(repoRoot) {
  const version =
    process.env.VERSION ||
    runAndCapture("git", ["describe", "--tags", "--always", "--dirty"], {
      cwd: repoRoot,
    }) ||
    "dev";
  const commit =
    runAndCapture("git", ["rev-parse", "--short", "HEAD"], { cwd: repoRoot }) ||
    "unknown";
  const buildDate = new Date().toISOString();

  return [
    "-s",
    "-w",
    `-X github.com/react-go-quick-starter/server/internal/version.Version=${version}`,
    `-X github.com/react-go-quick-starter/server/internal/version.Commit=${commit}`,
    `-X github.com/react-go-quick-starter/server/internal/version.BuildDate=${buildDate}`,
  ].join(" ");
}

function buildTarget(target, { goDir, binariesDir, ldflags }) {
  const extension = target.goos === "windows" ? ".exe" : "";
  const outputPath = path.join(
    binariesDir,
    `server-${target.triple}${extension}`,
  );

  console.log(
    `-> Building ${target.goos}/${target.goarch} -> ${path.basename(outputPath)}`,
  );

  const result = spawnSync(
    "go",
    ["build", `-ldflags=${ldflags}`, "-o", outputPath, "./cmd/server"],
    {
      cwd: goDir,
      env: {
        ...process.env,
        GOOS: target.goos,
        GOARCH: target.goarch,
        CGO_ENABLED: "0",
      },
      stdio: "inherit",
    },
  );

  if (result.status !== 0) {
    throw new Error(`go build failed for ${target.triple}`);
  }

  console.log(`   ok ${path.basename(outputPath)}`);
}

function main(argv = process.argv.slice(2)) {
  const currentOnly = argv.includes("--current-only");
  const directories = getDirectories();
  const hostTriple = detectHostTriple({ cwd: directories.repoRoot });
  const targets = resolveTargets({ currentOnly, hostTriple });
  const ldflags = getLdflags(directories.repoRoot);

  fs.mkdirSync(directories.binariesDir, { recursive: true });

  if (!currentOnly) {
    console.log("Cross-compiling for all supported platforms...");
  }

  for (const target of targets) {
    buildTarget(target, {
      goDir: directories.goDir,
      binariesDir: directories.binariesDir,
      ldflags,
    });
  }

  console.log("");
  console.log(
    `Backend build complete. Binaries in: ${directories.binariesDir}`,
  );
}

if (require.main === module) {
  try {
    main();
  } catch (error) {
    console.error(
      error instanceof Error ? error.message : "Unknown backend build error",
    );
    process.exit(1);
  }
}

module.exports = {
  ALL_TARGETS,
  detectHostTriple,
  getFallbackTriple,
  getLdflags,
  resolveTargets,
};
