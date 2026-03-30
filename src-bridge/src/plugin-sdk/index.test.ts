import { describe, expect, test } from "bun:test";
import {
  connectPluginStdioServer,
  createLocalPluginHarness,
  createPluginMcpServer,
  createReviewFinding,
  createReviewPluginManifest,
  createReviewResult,
  createToolPluginManifest,
  DEFAULT_API_VERSION,
  DEFAULT_REVIEW_ENTRYPOINT,
  defineReviewPlugin,
  defineToolPlugin,
} from "./index.js";
import {
  createReviewPluginManifest as createReviewPluginManifestDirect,
  createToolPluginManifest as createToolPluginManifestDirect,
  DEFAULT_API_VERSION as DEFAULT_API_VERSION_DIRECT,
  DEFAULT_REVIEW_ENTRYPOINT as DEFAULT_REVIEW_ENTRYPOINT_DIRECT,
} from "./manifest.js";
import { createLocalPluginHarness as createLocalPluginHarnessDirect } from "./harness.js";
import {
  createReviewFinding as createReviewFindingDirect,
  createReviewResult as createReviewResultDirect,
} from "./review.js";
import {
  connectPluginStdioServer as connectPluginStdioServerDirect,
  createPluginMcpServer as createPluginMcpServerDirect,
  defineReviewPlugin as defineReviewPluginDirect,
  defineToolPlugin as defineToolPluginDirect,
} from "./server.js";

describe("plugin SDK barrel exports", () => {
  test("re-exports the plugin SDK runtime helpers without drift", () => {
    expect(DEFAULT_API_VERSION).toBe(DEFAULT_API_VERSION_DIRECT);
    expect(DEFAULT_REVIEW_ENTRYPOINT).toBe(DEFAULT_REVIEW_ENTRYPOINT_DIRECT);
    expect(createToolPluginManifest).toBe(createToolPluginManifestDirect);
    expect(createReviewPluginManifest).toBe(createReviewPluginManifestDirect);
    expect(createLocalPluginHarness).toBe(createLocalPluginHarnessDirect);
    expect(createReviewFinding).toBe(createReviewFindingDirect);
    expect(createReviewResult).toBe(createReviewResultDirect);
    expect(createPluginMcpServer).toBe(createPluginMcpServerDirect);
    expect(connectPluginStdioServer).toBe(connectPluginStdioServerDirect);
    expect(defineToolPlugin).toBe(defineToolPluginDirect);
    expect(defineReviewPlugin).toBe(defineReviewPluginDirect);
  });
});
