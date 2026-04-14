/* eslint-disable @typescript-eslint/no-require-imports */

const fs = require("node:fs");
const path = require("node:path");

const DEFAULT_GO_WASM_MANIFEST_PATH = path.join(
  "plugins",
  "integrations",
  "feishu-adapter",
  "manifest.yaml",
);

const MAINTAINED_GO_WASM_TARGETS = {
  "feishu-adapter": {
    sourcePath: "./cmd/sample-wasm-plugin",
  },
  "standard-dev-flow": {
    sourcePath: "./cmd/standard-dev-flow",
  },
  "task-delivery-flow": {
    sourcePath: "./cmd/task-delivery-flow",
  },
  "review-escalation-flow": {
    sourcePath: "./cmd/review-escalation-flow",
  },
};

function getRepoRoot() {
  return path.resolve(__dirname, "..");
}

function normalizePath(value, { repoRoot = getRepoRoot() } = {}) {
  if (!value) {
    return "";
  }

  if (path.isAbsolute(value)) {
    return path.normalize(value);
  }

  return path.resolve(repoRoot, value);
}

function extractScalar(source, pattern, label) {
  const match = source.match(pattern);
  if (!match || !match[1]) {
    throw new Error(`manifest is missing required field ${label}`);
  }

  return match[1].trim();
}

function parseManifestFields(source) {
  return {
    pluginId: extractScalar(source, /^\s*id:\s*([^\r\n]+)$/m, "metadata.id"),
    runtime: extractScalar(source, /^\s*runtime:\s*([^\r\n]+)$/m, "spec.runtime"),
    module: extractScalar(source, /^\s*module:\s*([^\r\n]+)$/m, "spec.module"),
  };
}

function resolveMaintainedGoWASMSourcePath(pluginId) {
  return MAINTAINED_GO_WASM_TARGETS[pluginId]?.sourcePath ?? "";
}

function resolveBuildTarget({
  manifestPath,
  sourcePath,
  outputPath,
  repoRoot = getRepoRoot(),
} = {}) {
  const resolvedManifestPath = normalizePath(
    manifestPath || DEFAULT_GO_WASM_MANIFEST_PATH,
    { repoRoot },
  );
  const manifestSource = fs.readFileSync(resolvedManifestPath, "utf8");
  const manifest = parseManifestFields(manifestSource);

  if (manifest.runtime !== "wasm") {
    throw new Error(
      `manifest ${resolvedManifestPath} uses unsupported runtime ${manifest.runtime}`,
    );
  }

  const resolvedSourcePath =
    sourcePath || resolveMaintainedGoWASMSourcePath(manifest.pluginId);

  if (!resolvedSourcePath) {
    throw new Error(
      `no maintained Go WASM source mapping found for plugin ${manifest.pluginId}; pass --source`,
    );
  }

  return {
    manifestPath: resolvedManifestPath,
    pluginId: manifest.pluginId,
    runtime: manifest.runtime,
    modulePath: outputPath
      ? normalizePath(outputPath, { repoRoot })
      : path.resolve(path.dirname(resolvedManifestPath), manifest.module),
    sourcePath: resolvedSourcePath,
  };
}

module.exports = {
  DEFAULT_GO_WASM_MANIFEST_PATH,
  MAINTAINED_GO_WASM_TARGETS,
  getRepoRoot,
  parseManifestFields,
  resolveBuildTarget,
};
