/* eslint-disable @typescript-eslint/no-require-imports */

const { mkdirSync } = require("node:fs");
const { dirname, resolve } = require("node:path");
const { spawnSync } = require("node:child_process");

const repoRoot = resolve(__dirname, "..");
const outputPath = resolve(repoRoot, "plugins", "integrations", "feishu-adapter", "dist", "feishu.wasm");

mkdirSync(dirname(outputPath), { recursive: true });

const result = spawnSync(
  "go",
  ["build", "-o", outputPath, "./cmd/sample-wasm-plugin"],
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
