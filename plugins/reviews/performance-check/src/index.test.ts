import { describe, expect, test } from "bun:test";
import { createLocalPluginHarness } from "../../../../src-bridge/src/plugin-sdk/index.js";
import { plugin } from "./index.js";

describe("Performance Check review plugin", () => {
  test("emits normalized findings through the local plugin harness", async () => {
    const harness = createLocalPluginHarness(plugin);
    const result = await harness.callTool("review:run", { review_id: "review-123" });
    expect(result.structuredContent).toEqual(
      expect.objectContaining({
        format: "findings/v1",
      }),
    );
  });
});
