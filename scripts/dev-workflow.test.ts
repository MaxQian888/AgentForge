/** @jest-environment node */

describe("dev-workflow command detection", () => {
  afterEach(() => {
    jest.resetModules();
    jest.dontMock("node:child_process");
  });

  test("uses `go version` directly for Go availability", () => {
    const spawnSync = jest.fn(() => ({
      status: 0,
    }));

    jest.doMock("node:child_process", () => ({
      spawnSync,
    }));

    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { isCommandAvailable } = require("./dev-workflow.js");

    expect(isCommandAvailable("go")).toBe(true);
    expect(spawnSync).toHaveBeenCalledWith(
      "go",
      ["version"],
      expect.objectContaining({
        shell: false,
        stdio: "ignore",
      }),
    );
  });
});
