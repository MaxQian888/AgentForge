/** @jest-environment node */

import fs from "node:fs";
import os from "node:os";
import path from "node:path";

export {};

async function loadBundleModule() {
  return import("./verify-built-in-plugin-bundle.js");
}

describe("verify-built-in-plugin-bundle", () => {
  afterEach(() => {
    jest.resetModules();
    jest.dontMock("node:child_process");
  });

  test("loads the official built-in plugin bundle from the repository", async () => {
    const { loadBuiltInBundle } = await loadBundleModule();

    const bundle = loadBuiltInBundle();
    const ids = bundle.entries.map((entry: { id: string }) => entry.id);

    expect(ids).toEqual(
      expect.arrayContaining([
        "web-search",
        "github-tool",
        "db-query",
        "task-control",
        "review-control",
        "workflow-control",
        "sample-integration-plugin",
        "architecture-check",
        "performance-check",
        "standard-dev-flow",
        "task-delivery-flow",
        "review-escalation-flow",
      ]),
    );
    expect(
      bundle.entries.every(
        (entry: { readiness?: { readyMessage?: string } }) =>
          typeof entry.readiness?.readyMessage === "string" &&
          entry.readiness.readyMessage.length > 0,
      ),
    ).toBe(true);
  });

  test("creates family-specific verification stages for each official built-in", async () => {
    const { createBundleVerificationPlan, loadBuiltInBundle } = await loadBundleModule();

    const plan = createBundleVerificationPlan(loadBuiltInBundle());
    const summary = Object.fromEntries(
      plan.map((item: { pluginId: string; stages: { name: string }[] }) => [
        item.pluginId,
        item.stages.map((stage) => stage.name),
      ]),
    );

    expect(summary["web-search"]).toEqual(["manifest"]);
    expect(summary["task-control"]).toEqual(["manifest", "package-validate"]);
    expect(summary["review-control"]).toEqual(["manifest", "package-validate"]);
    expect(summary["workflow-control"]).toEqual(["manifest", "package-validate"]);
    expect(summary["architecture-check"]).toEqual(["manifest", "package-validate"]);
    expect(summary["standard-dev-flow"]).toEqual(["build", "debug-health"]);
    expect(summary["task-delivery-flow"]).toEqual(["build", "debug-health"]);
    expect(summary["review-escalation-flow"]).toEqual(["build", "debug-health"]);
    expect(summary["sample-integration-plugin"]).toEqual(["build", "debug-health"]);

    const standardDevFlow = plan.find(
      (item: { pluginId: string }) => item.pluginId === "standard-dev-flow",
    );
    expect(standardDevFlow?.stages).toEqual([
      expect.objectContaining({
        name: "build",
        script: "scripts/plugin/build-go-wasm-plugin.js",
      }),
      expect.objectContaining({
        name: "debug-health",
        script: "scripts/plugin/debug-go-wasm-plugin.js",
      }),
    ]);
  });

  test("runs the review plugin validate script during built-in verification", async () => {
    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-builtins-"));
    const pluginDir = path.join(repoRoot, "plugins", "reviews", "architecture-check");
    fs.mkdirSync(pluginDir, { recursive: true });
    fs.writeFileSync(
      path.join(repoRoot, "plugins", "builtin-bundle.yaml"),
      JSON.stringify({
        plugins: [
          {
            id: "architecture-check",
            kind: "ReviewPlugin",
            manifest: "reviews/architecture-check/manifest.yaml",
            verificationProfile: "mcp-review",
            readiness: {
              readyMessage: "Architecture Check is ready for install.",
              blockedMessage: "Requires Bun on the bridge host to execute the bundled review plugin.",
              nextStep: "Install Bun on the bridge host before activation.",
              installable: true,
              prerequisites: [
                {
                  kind: "executable",
                  value: "bun",
                  label: "Bun",
                },
              ],
            },
            availability: {
              status: "ready",
              message: "ready",
            },
          },
        ],
      }),
    );
    fs.writeFileSync(
      path.join(pluginDir, "manifest.yaml"),
      [
        "apiVersion: agentforge/v1",
        "kind: ReviewPlugin",
        "metadata:",
        "  id: architecture-check",
        "  name: Architecture Check",
        "  version: 1.0.0",
        "spec:",
        "  runtime: mcp",
        "  transport: stdio",
        "  command: bun",
        '  args: ["run", "src/index.ts"]',
        "  review:",
        "    entrypoint: review:run",
        "    triggers:",
        '      events: ["pull_request.updated"]',
        "    output:",
        "      format: findings/v1",
        "",
      ].join("\n"),
    );
    fs.writeFileSync(
      path.join(pluginDir, "package.json"),
      JSON.stringify(
        {
          scripts: {
            validate: "bun test src/index.test.ts",
          },
        },
        null,
        2,
      ),
    );

    const spawnSync = jest.fn(() => ({ status: 0, stdout: "", stderr: "" }));

    await jest.isolateModulesAsync(async () => {
      jest.doMock("node:child_process", () => ({ spawnSync }));
      const { runBundleVerification } = await loadBundleModule();

      const result = runBundleVerification({ repoRoot });
      expect(result.ok).toBe(true);
    });

    expect(spawnSync).toHaveBeenCalledWith(
      "bun",
      ["run", "validate"],
      expect.objectContaining({
        cwd: pluginDir,
        encoding: "utf8",
        stdio: ["ignore", "pipe", "pipe"],
      }),
    );
  });

  test("flags malformed readiness contracts before running per-plugin stages", async () => {
    const { validateReadinessContract } = await loadBundleModule();

    expect(
      validateReadinessContract({
        id: "github-tool",
        readiness: {
          readyMessage: "",
          prerequisites: [{ kind: "executable", value: "bun", label: "" }],
        },
      }),
    ).toEqual(
      expect.arrayContaining([
        "missing readiness.readyMessage",
        "missing readiness.blockedMessage",
        "missing readiness.nextStep",
        "missing readiness.prerequisites[0].label",
      ]),
    );
  });

  test("fails tool and workflow built-ins that omit starter catalog metadata", async () => {
    const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentforge-builtins-"));
    fs.mkdirSync(path.join(repoRoot, "plugins", "tools", "web-search"), { recursive: true });
    fs.writeFileSync(
      path.join(repoRoot, "plugins", "builtin-bundle.yaml"),
      JSON.stringify({
        plugins: [
          {
            id: "web-search",
            kind: "ToolPlugin",
            manifest: "tools/web-search/manifest.yaml",
            verificationProfile: "mcp-tool",
            readiness: {
              readyMessage: "Web Search is ready for install.",
              blockedMessage: "Needs local setup before activation.",
              nextStep: "Install the built-in and confirm the bridge runtime is available.",
              installable: true,
            },
            availability: {
              status: "ready",
              message: "ready",
            },
          },
        ],
      }),
    );
    fs.writeFileSync(
      path.join(repoRoot, "plugins", "tools", "web-search", "manifest.yaml"),
      [
        "apiVersion: agentforge/v1",
        "kind: ToolPlugin",
        "metadata:",
        "  id: web-search",
        "  name: Web Search",
        "  version: 1.0.0",
        "spec:",
        "  runtime: mcp",
        "  transport: stdio",
        "  command: node",
        '  args: ["tool.js"]',
        "",
      ].join("\n"),
    );

    const { runBundleVerification } = await loadBundleModule();
    const result = runBundleVerification({ repoRoot });

    expect(result.ok).toBe(false);
    expect(result.stage).toBe("starter-catalog");
    expect(result.stderr).toContain("missing starterFamily");
    expect(result.stderr).toContain("missing coreFlows");
    expect(result.stderr).toContain("missing dependencyRefs");
    expect(result.stderr).toContain("missing workspaceRefs");
  });

  test("evaluates deterministic readiness preflight without requiring live execution", async () => {
    const { evaluateReadiness } = await loadBundleModule();

    const prerequisiteBlocked = evaluateReadiness(
      {
        readiness: {
          readyMessage: "ready",
          blockedMessage: "blocked",
          nextStep: "install helper",
          installable: true,
          prerequisites: [{ kind: "executable", value: "bun", label: "Bun" }],
        },
      },
      { hasExecutable: () => false, env: {} as NodeJS.ProcessEnv, host: "linux" },
    );
    expect(prerequisiteBlocked).toEqual(
      expect.objectContaining({
        status: "requires_prerequisite",
        missingPrerequisites: ["Bun"],
        installable: true,
      }),
    );

    const configurationBlocked = evaluateReadiness(
      {
        readiness: {
          readyMessage: "ready",
          blockedMessage: "blocked",
          nextStep: "set token",
          installable: true,
          configuration: [{ kind: "env", value: "AGENTFORGE_GITHUB_TOKEN", label: "GitHub token" }],
        },
      },
      { hasExecutable: () => true, env: {} as NodeJS.ProcessEnv, host: "linux" },
    );
    expect(configurationBlocked).toEqual(
      expect.objectContaining({
        status: "requires_configuration",
        missingConfiguration: ["GitHub token"],
        installable: true,
      }),
    );
  });
});
