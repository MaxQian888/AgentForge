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
});
