import test from "node:test";
import assert from "node:assert/strict";
import { copyFileSync, existsSync, mkdtempSync, readFileSync, rmSync, mkdirSync } from "node:fs";
import { join, resolve } from "node:path";
import { tmpdir } from "node:os";
import { spawnSync } from "node:child_process";

const repoRoot = resolve(import.meta.dirname, "..", "..");

test("build-go-wasm-plugin can derive output from a manifest file", () => {
  const tempDir = mkdtempSync(join(tmpdir(), "agentforge-wasm-build-"));
  const manifestDir = join(tempDir, "fixture-plugin");
  const manifestPath = join(manifestDir, "manifest.yaml");
  const expectedOutput = join(manifestDir, "dist", "fixture.wasm");
  const fixtureManifestPath = resolve(
    repoRoot,
    "scripts",
    "plugin",
    "fixtures",
    "go-wasm-plugin-manifest-without-source.yaml",
  );

  try {
    mkdirSync(join(manifestDir, "dist"), { recursive: true });
    copyFileSync(fixtureManifestPath, manifestPath);

    const result = spawnSync(
        "node",
        [
        "scripts/plugin/build-go-wasm-plugin.js",
        "--manifest",
        manifestPath,
        "--source",
        "./cmd/sample-wasm-plugin",
      ],
      {
        cwd: repoRoot,
        encoding: "utf8",
      },
    );

    assert.equal(result.status, 0, result.stderr || result.stdout);
    assert.equal(existsSync(expectedOutput), true, "manifest-derived wasm output should exist");

    const manifest = readFileSync(manifestPath, "utf8");
    assert.match(manifest, /module:\s+\.\/dist\/fixture\.wasm/);
    assert.match(
      readFileSync(resolve(repoRoot, "docs", "GO_WASM_PLUGIN_RUNTIME.md"), "utf8"),
      /scripts\/plugin\/build-go-wasm-plugin\.js/,
    );
  } finally {
    rmSync(tempDir, { force: true, recursive: true });
  }
});
