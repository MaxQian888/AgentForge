/** @jest-environment node */

import * as fs from "node:fs";
import * as path from "node:path";

export {};

describe("verify-plugin-dev-workflow stage plan", () => {
  test("builds the maintained sample verification stages", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { createVerificationStages } = require("./verify-plugin-dev-workflow.js");

    expect(
      createVerificationStages({
        manifestPath: "plugins/integrations/sample-integration-plugin/manifest.yaml",
      }).map((stage: { name: string }) => stage.name),
    ).toEqual([
      "build",
      "debug-health",
    ]);

    expect(
      createVerificationStages({
        manifestPath: "plugins/integrations/sample-integration-plugin/manifest.yaml",
      }).map((stage: { script: string }) => stage.script),
    ).toEqual([
      "scripts/plugin/build-go-wasm-plugin.js",
      "scripts/plugin/debug-go-wasm-plugin.js",
    ]);
  });

  test("builds the maintained starter verification stages for new workflow starters", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { createVerificationStages } = require("./verify-plugin-dev-workflow.js");

    expect(
      createVerificationStages({
        manifestPath: "plugins/workflows/task-delivery-flow/manifest.yaml",
      }).map((stage: { name: string }) => stage.name),
    ).toEqual([
      "build",
      "debug-health",
    ]);

    expect(
      createVerificationStages({
        manifestPath: "plugins/workflows/review-escalation-flow/manifest.yaml",
      }).map((stage: { name: string }) => stage.name),
    ).toEqual([
      "build",
      "debug-health",
    ]);
  });

  test("keeps root and plugin-local package commands aligned with the canonical plugin script family", () => {
    const rootPackageJson = JSON.parse(
      fs.readFileSync(path.join(process.cwd(), "package.json"), "utf8"),
    );
    const standardWorkflowPackageJson = JSON.parse(
      fs.readFileSync(
        path.join(process.cwd(), "plugins", "workflows", "standard-dev-flow", "package.json"),
        "utf8",
      ),
    );
    const taskWorkflowPackageJson = JSON.parse(
      fs.readFileSync(
        path.join(process.cwd(), "plugins", "workflows", "task-delivery-flow", "package.json"),
        "utf8",
      ),
    );
    const reviewWorkflowPackageJson = JSON.parse(
      fs.readFileSync(
        path.join(process.cwd(), "plugins", "workflows", "review-escalation-flow", "package.json"),
        "utf8",
      ),
    );

    expect(rootPackageJson.scripts).toMatchObject({
      "build:plugin:wasm": "node scripts/plugin/build-go-wasm-plugin.js",
      "plugin:build": "node scripts/plugin/build-go-wasm-plugin.js",
      "create-plugin": "node scripts/plugin/create-plugin.js",
      "plugin:debug": "node scripts/plugin/debug-go-wasm-plugin.js",
      "plugin:dev": "node scripts/plugin/run-plugin-dev-stack.js",
      "plugin:verify": "node scripts/plugin/verify-plugin-dev-workflow.js",
      "plugin:verify:builtins": "node scripts/plugin/verify-built-in-plugin-bundle.js",
    });
    expect(standardWorkflowPackageJson.scripts).toMatchObject({
      "plugin:build":
        "node ../../../scripts/plugin/build-go-wasm-plugin.js --manifest plugins/workflows/standard-dev-flow/manifest.yaml --source ./cmd/standard-dev-flow",
      "plugin:verify":
        "node ../../../scripts/plugin/verify-plugin-dev-workflow.js --manifest plugins/workflows/standard-dev-flow/manifest.yaml",
    });
    expect(taskWorkflowPackageJson.scripts).toMatchObject({
      "plugin:build":
        "node ../../../scripts/plugin/build-go-wasm-plugin.js --manifest plugins/workflows/task-delivery-flow/manifest.yaml --source ./cmd/task-delivery-flow",
      "plugin:verify":
        "node ../../../scripts/plugin/verify-plugin-dev-workflow.js --manifest plugins/workflows/task-delivery-flow/manifest.yaml",
    });
    expect(reviewWorkflowPackageJson.scripts).toMatchObject({
      "plugin:build":
        "node ../../../scripts/plugin/build-go-wasm-plugin.js --manifest plugins/workflows/review-escalation-flow/manifest.yaml --source ./cmd/review-escalation-flow",
      "plugin:verify":
        "node ../../../scripts/plugin/verify-plugin-dev-workflow.js --manifest plugins/workflows/review-escalation-flow/manifest.yaml",
    });
  });
});
