/** @jest-environment node */

export {};

describe("verify-plugin-dev-workflow stage plan", () => {
  test("builds the maintained sample verification stages", () => {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { createVerificationStages } = require("./verify-plugin-dev-workflow.js");

    expect(
      createVerificationStages({
        manifestPath: "plugins/integrations/feishu-adapter/manifest.yaml",
      }).map((stage: { name: string }) => stage.name),
    ).toEqual([
      "build",
      "debug-health",
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
});
