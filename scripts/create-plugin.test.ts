/** @jest-environment node */

import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

function readFile(filePath: string) {
  return fs.readFileSync(filePath, "utf8");
}

describe("create-plugin scaffolding", () => {
  test("scaffolds a tool plugin starter with MCP files and starter test", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { scaffoldPlugin } = require("./create-plugin.js");
    const rootDir = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-tool-plugin-"));

    const result = scaffoldPlugin({
      rootDir,
      type: "tool",
      name: "echo-tool",
    });

    expect(result.pluginDir).toBe(path.join(rootDir, "plugins", "tools", "echo-tool"));
    expect(fs.existsSync(path.join(result.pluginDir, "manifest.yaml"))).toBe(true);
    expect(fs.existsSync(path.join(result.pluginDir, "package.json"))).toBe(true);
    expect(fs.existsSync(path.join(result.pluginDir, "src", "index.ts"))).toBe(true);
    expect(fs.existsSync(path.join(result.pluginDir, "src", "index.test.ts"))).toBe(true);

    expect(readFile(path.join(result.pluginDir, "manifest.yaml"))).toContain("kind: ToolPlugin");
    expect(readFile(path.join(result.pluginDir, "src", "index.ts"))).toContain("defineToolPlugin");
  });

  test("scaffolds a review plugin starter with normalized findings helper", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { scaffoldPlugin } = require("./create-plugin.js");
    const rootDir = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-review-plugin-"));

    const result = scaffoldPlugin({
      rootDir,
      type: "review",
      name: "typescript-review",
    });

    expect(result.pluginDir).toBe(path.join(rootDir, "plugins", "reviews", "typescript-review"));
    expect(fs.existsSync(path.join(result.pluginDir, "manifest.yaml"))).toBe(true);
    expect(fs.existsSync(path.join(result.pluginDir, "src", "index.ts"))).toBe(true);
    expect(fs.existsSync(path.join(result.pluginDir, "src", "index.test.ts"))).toBe(true);

    const manifest = readFile(path.join(result.pluginDir, "manifest.yaml"));
    expect(manifest).toContain("kind: ReviewPlugin");
    expect(manifest).toContain("format: findings/v1");

    const entrypoint = readFile(path.join(result.pluginDir, "src", "index.ts"));
    expect(entrypoint).toContain("createReviewResult");
    expect(entrypoint).toContain("defineReviewPlugin");
  });

  test("scaffolds a workflow plugin starter with Go entrypoint and build helper hooks", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { scaffoldPlugin } = require("./create-plugin.js");
    const rootDir = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-workflow-plugin-"));

    const result = scaffoldPlugin({
      rootDir,
      type: "workflow",
      name: "release-train",
    });

    expect(result.pluginDir).toBe(path.join(rootDir, "plugins", "workflows", "release-train"));
    expect(result.goSourceDir).toBe(path.join(rootDir, "src-go", "cmd", "release-train"));
    expect(fs.existsSync(path.join(result.pluginDir, "manifest.yaml"))).toBe(true);
    expect(fs.existsSync(path.join(result.pluginDir, "package.json"))).toBe(true);
    expect(fs.existsSync(path.join(result.goSourceDir, "main.go"))).toBe(true);
    expect(fs.existsSync(path.join(result.goSourceDir, "main_test.go"))).toBe(true);

    const manifest = readFile(path.join(result.pluginDir, "manifest.yaml"));
    expect(manifest).toContain("kind: WorkflowPlugin");
    expect(manifest).toContain("process: sequential");

    const packageJson = readFile(path.join(result.pluginDir, "package.json"));
    expect(packageJson).toContain("plugin:build");
    expect(packageJson).toContain("plugin:verify");
  });
});
