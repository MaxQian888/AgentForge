#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require("node:fs");
const path = require("node:path");

function collectRelativePaths(rootDir) {
  const queue = [rootDir];
  const results = new Set();

  while (queue.length > 0) {
    const current = queue.shift();
    for (const entry of fs.readdirSync(current, { withFileTypes: true })) {
      const fullPath = path.join(current, entry.name);
      if (entry.isDirectory()) {
        queue.push(fullPath);
        continue;
      }

      results.add(path.relative(rootDir, fullPath).replace(/\\/gu, "/"));
    }
  }

  return results;
}

function manifestUrlToRelativePath(url) {
  const parsed = new URL(url);
  return parsed.pathname
    .split("/")
    .slice(6)
    .map((segment) => decodeURIComponent(segment))
    .join("/");
}

function findMissingManifestArtifacts({
  availableRelativePaths,
  manifest,
}) {
  const problems = [];

  for (const [platform, entry] of Object.entries(manifest.platforms ?? {})) {
    const relativePath = manifestUrlToRelativePath(entry.url);
    if (!availableRelativePaths.has(relativePath)) {
      problems.push({
        platform,
        reason: "missing-artifact",
        relativePath,
      });
    }
  }

  return problems;
}

function findMissingRequiredPlatforms({ manifest, requiredPlatforms }) {
  const availablePlatforms = new Set(Object.keys(manifest.platforms ?? {}));

  return requiredPlatforms
    .filter((platform) => !availablePlatforms.has(platform))
    .map((platform) => ({
      platform,
      reason: "missing-platform-entry",
    }));
}

function parseArgValue(args, flag) {
  const index = args.indexOf(flag);
  if (index === -1 || index === args.length - 1) {
    return null;
  }

  return args[index + 1];
}

function main(argv = process.argv.slice(2)) {
  const artifactsRoot = parseArgValue(argv, "--artifacts-root");
  const manifestPath = parseArgValue(argv, "--manifest");

  if (!artifactsRoot || !manifestPath) {
    throw new Error(
      "Usage: node scripts/validate-updater-artifacts.js --artifacts-root <dir> --manifest <file>",
    );
  }

  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  const availableRelativePaths = collectRelativePaths(artifactsRoot);
  const problems = findMissingManifestArtifacts({
    availableRelativePaths,
    manifest,
  });
  problems.push(
    ...findMissingRequiredPlatforms({
      manifest,
      requiredPlatforms: [
        "linux-x86_64",
        "windows-x86_64",
        "darwin-x86_64",
        "darwin-aarch64",
      ],
    }),
  );

  if (problems.length > 0) {
    throw new Error(
      `Updater artifact validation failed: ${JSON.stringify(problems)}`,
    );
  }

  console.log("Updater artifacts and manifest are aligned.");
}

if (require.main === module) {
  try {
    main();
  } catch (error) {
    console.error(
      error instanceof Error
        ? error.message
        : "Unknown updater artifact validation error",
    );
    process.exit(1);
  }
}

module.exports = {
  collectRelativePaths,
  findMissingManifestArtifacts,
  findMissingRequiredPlatforms,
  manifestUrlToRelativePath,
};
