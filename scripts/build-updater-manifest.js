#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require("node:fs");
const path = require("node:path");

const ARTIFACT_SPECS = [
  {
    artifactDir: "tauri-linux-amd64-appimage",
    artifactRegex: /\.AppImage$/u,
    fallbackKey: "linux-x86_64",
    platformKey: "linux-x86_64-appimage",
    signatureRegex: /\.AppImage\.sig$/u,
  },
  {
    artifactDir: "tauri-linux-arm64-appimage",
    artifactRegex: /\.AppImage$/u,
    fallbackKey: "linux-aarch64",
    platformKey: "linux-aarch64-appimage",
    signatureRegex: /\.AppImage\.sig$/u,
  },
  {
    artifactDir: "tauri-macos-x64-updater",
    artifactRegex: /\.app\.tar\.gz$/u,
    fallbackKey: "darwin-x86_64",
    platformKey: "darwin-x86_64-app",
    signatureRegex: /\.app\.tar\.gz\.sig$/u,
  },
  {
    artifactDir: "tauri-macos-arm64-updater",
    artifactRegex: /\.app\.tar\.gz$/u,
    fallbackKey: "darwin-aarch64",
    platformKey: "darwin-aarch64-app",
    signatureRegex: /\.app\.tar\.gz\.sig$/u,
  },
  {
    artifactDir: "tauri-windows-x64-nsis",
    artifactRegex: /\.exe$/u,
    fallbackKey: "windows-x86_64",
    platformKey: "windows-x86_64-nsis",
    signatureRegex: /\.exe\.sig$/u,
  },
  {
    artifactDir: "tauri-windows-x64-msi",
    artifactRegex: /\.msi$/u,
    fallbackKey: null,
    platformKey: "windows-x86_64-msi",
    signatureRegex: /\.msi\.sig$/u,
  },
];

function normalizeReleaseVersion(version) {
  return String(version).replace(/^v/u, "");
}

function collectRelativePaths(rootDir) {
  const queue = [rootDir];
  const results = [];

  while (queue.length > 0) {
    const current = queue.shift();
    for (const entry of fs.readdirSync(current, { withFileTypes: true })) {
      const fullPath = path.join(current, entry.name);
      if (entry.isDirectory()) {
        queue.push(fullPath);
        continue;
      }

      results.push(path.relative(rootDir, fullPath).replace(/\\/gu, "/"));
    }
  }

  return results;
}

function collectUpdaterArtifacts(artifactsRoot) {
  const relativePaths = collectRelativePaths(artifactsRoot);
  const collected = {};

  for (const spec of ARTIFACT_SPECS) {
    const scopedPaths = relativePaths.filter((relativePath) =>
      relativePath.startsWith(`${spec.artifactDir}/`),
    );
    const artifactPath = scopedPaths.find((relativePath) =>
      spec.artifactRegex.test(relativePath),
    );
    const signaturePath = scopedPaths.find((relativePath) =>
      spec.signatureRegex.test(relativePath),
    );

    if (!artifactPath || !signaturePath) {
      continue;
    }

    collected[spec.platformKey] = {
      fallbackKey: spec.fallbackKey,
      path: artifactPath,
      signature: fs
        .readFileSync(path.join(artifactsRoot, signaturePath), "utf8")
        .trim(),
    };
  }

  return collected;
}

function relativePathToDownloadUrl(baseDownloadUrl, relativePath) {
  const safeBase = String(baseDownloadUrl).replace(/\/+$/u, "");
  const encodedPath = relativePath
    .split("/")
    .map((segment) => encodeURIComponent(segment))
    .join("/");

  return `${safeBase}/${encodedPath}`;
}

function inferFallbackKey(platformKey) {
  return platformKey.replace(/-(app|appimage|msi|nsis)$/u, "");
}

function buildUpdaterManifest({
  baseDownloadUrl,
  generatedAt,
  releaseVersion,
  updaterArtifacts,
}) {
  const manifest = {
    platforms: {},
    pub_date: generatedAt,
    version: normalizeReleaseVersion(releaseVersion),
  };

  for (const [platformKey, artifact] of Object.entries(updaterArtifacts)) {
    const entry = {
      signature: artifact.signature,
      url: relativePathToDownloadUrl(baseDownloadUrl, artifact.path),
    };

    manifest.platforms[platformKey] = entry;
    const fallbackKey = artifact.fallbackKey ?? inferFallbackKey(platformKey);
    if (fallbackKey && fallbackKey !== platformKey) {
      manifest.platforms[fallbackKey] = entry;
    }
  }

  return manifest;
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
  const baseDownloadUrl = parseArgValue(argv, "--base-download-url");
  const releaseVersion = parseArgValue(argv, "--release-version");
  const outputPath =
    parseArgValue(argv, "--output") ?? path.join(process.cwd(), "latest.json");

  if (!artifactsRoot || !baseDownloadUrl || !releaseVersion) {
    throw new Error(
      "Usage: node scripts/build-updater-manifest.js --artifacts-root <dir> --base-download-url <url> --release-version <version> [--output <file>]",
    );
  }

  const updaterArtifacts = collectUpdaterArtifacts(artifactsRoot);
  const manifest = buildUpdaterManifest({
    baseDownloadUrl,
    generatedAt: new Date().toISOString(),
    releaseVersion,
    updaterArtifacts,
  });

  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, JSON.stringify(manifest, null, 2));
  console.log(`Updater manifest written to ${outputPath}`);
}

if (require.main === module) {
  try {
    main();
  } catch (error) {
    console.error(
      error instanceof Error
        ? error.message
        : "Unknown updater manifest build error",
    );
    process.exit(1);
  }
}

module.exports = {
  ARTIFACT_SPECS,
  buildUpdaterManifest,
  collectUpdaterArtifacts,
  normalizeReleaseVersion,
  inferFallbackKey,
  relativePathToDownloadUrl,
};
