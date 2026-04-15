import { describe, expect, test } from "bun:test";
import {
  PluginManifestSchema,
  PluginRegisterRequestSchema,
  ToolPluginManifestSchema,
} from "./schema.js";

const validToolPlugin = {
  apiVersion: "agentforge.dev/v1",
  kind: "ToolPlugin",
  metadata: {
    id: "tool.plugin",
    name: "Tool Plugin",
    version: "1.0.0",
  },
  spec: {
    runtime: "mcp",
    transport: "stdio",
    command: "node",
    args: ["plugin.js"],
  },
};

describe("plugin schemas", () => {
  test("parses a valid tool plugin manifest and defaults permissions", () => {
    const parsed = PluginRegisterRequestSchema.parse({
      manifest: validToolPlugin,
    });

    expect(parsed.manifest.permissions).toEqual({});
    expect(parsed.manifest.source).toBeUndefined();
  });

  test("rejects tool plugins that do not use the mcp runtime or omit stdio command", () => {
    const invalidRuntime = ToolPluginManifestSchema.safeParse({
      ...validToolPlugin,
      spec: {
        ...validToolPlugin.spec,
        runtime: "declarative",
      },
    });

    const missingCommand = PluginManifestSchema.safeParse({
      ...validToolPlugin,
      spec: {
        ...validToolPlugin.spec,
        command: undefined,
      },
    });

    expect(invalidRuntime.success).toBe(false);
    expect(JSON.stringify(invalidRuntime.error?.issues)).toContain(
      "ToolPlugin manifests must use the mcp runtime",
    );
    expect(missingCommand.success).toBe(false);
    expect(JSON.stringify(missingCommand.error?.issues)).toContain(
      "stdio tool plugins must define spec.command",
    );
  });

  test("rejects integration plugins that do not use a Go-hosted runtime", () => {
    const invalidIntegration = PluginManifestSchema.safeParse({
      apiVersion: "agentforge.dev/v1",
      kind: "IntegrationPlugin",
      metadata: {
        id: "integration.plugin",
        name: "Integration Plugin",
        version: "1.0.0",
      },
      spec: {
        runtime: "mcp",
      },
    });

    expect(invalidIntegration.success).toBe(false);
    expect(JSON.stringify(invalidIntegration.error?.issues)).toContain(
      "IntegrationPlugin manifests must use a Go-hosted runtime",
    );
  });

  test("accepts workflow manifests with wasm runtime and normalized git source metadata", () => {
    const workflow = PluginManifestSchema.safeParse({
      apiVersion: "agentforge.dev/v1",
      kind: "WorkflowPlugin",
      metadata: {
        id: "workflow.release-train",
        name: "Release Train",
        version: "1.2.0",
      },
      spec: {
        runtime: "wasm",
        module: "./dist/release-train.wasm",
        abiVersion: "v1",
        workflow: {
          process: "sequential",
          roles: [{ id: "coder" }, { id: "reviewer" }],
          steps: [
            { id: "implement", role: "coder", action: "agent", next: ["review"] },
            { id: "review", role: "reviewer", action: "review" },
          ],
          triggers: [{ event: "manual" }],
          limits: { maxRetries: 1 },
        },
      },
      source: {
        type: "git",
        repository: "https://github.com/example/release-train.git",
        ref: "refs/tags/v1.2.0",
        digest: "sha256:workflow",
        trust: {
          status: "verified",
          approvalState: "approved",
          source: "cosign",
        },
        release: {
          version: "1.2.0",
          channel: "stable",
          artifact: "https://github.com/example/release-train/releases/download/v1.2.0/plugin.wasm",
        },
      },
    });

    expect(workflow.success).toBe(true);
  });

  test("accepts workflow manifests with a structured task-driven trigger profile", () => {
    const workflow = PluginManifestSchema.safeParse({
      apiVersion: "agentforge.dev/v1",
      kind: "WorkflowPlugin",
      metadata: {
        id: "workflow.task-delivery",
        name: "Task Delivery",
        version: "1.0.0",
      },
      spec: {
        runtime: "wasm",
        module: "./dist/task-delivery.wasm",
        abiVersion: "v1",
        workflow: {
          process: "sequential",
          roles: [{ id: "planner" }, { id: "coder" }],
          steps: [
            { id: "plan", role: "planner", action: "agent", next: ["implement"] },
            { id: "implement", role: "coder", action: "agent" },
          ],
          triggers: [
            { event: "manual" },
            { event: "task.transition", profile: "task-delivery", requiresTask: true },
          ],
        },
      },
    });

    expect(workflow.success).toBe(true);
  });

  test("rejects task-driven workflow triggers without a profile identifier", () => {
    const workflow = PluginManifestSchema.safeParse({
      apiVersion: "agentforge.dev/v1",
      kind: "WorkflowPlugin",
      metadata: {
        id: "workflow.task-delivery",
        name: "Task Delivery",
        version: "1.0.0",
      },
      spec: {
        runtime: "wasm",
        module: "./dist/task-delivery.wasm",
        abiVersion: "v1",
        workflow: {
          process: "sequential",
          roles: [{ id: "planner" }],
          steps: [{ id: "plan", role: "planner", action: "agent" }],
          triggers: [{ event: "task.transition", requiresTask: true }],
        },
      },
    });

    expect(workflow.success).toBe(false);
    expect(JSON.stringify(workflow.error?.issues)).toContain("profile");
  });

  test("accepts review manifests with trigger contract and npm trust metadata", () => {
    const review = PluginManifestSchema.safeParse({
      apiVersion: "agentforge.dev/v1",
      kind: "ReviewPlugin",
      metadata: {
        id: "review.typescript",
        name: "TypeScript Review",
        version: "1.0.0",
      },
      spec: {
        runtime: "mcp",
        transport: "stdio",
        command: "node",
        args: ["dist/review.js"],
        review: {
          entrypoint: "review:run",
          triggers: {
            events: ["pull_request.updated"],
            filePatterns: ["src/**/*.ts"],
          },
          output: {
            format: "findings/v1",
          },
        },
      },
      source: {
        type: "npm",
        package: "@agentforge/review-typescript",
        version: "1.0.0",
        registry: "https://registry.npmjs.org",
        digest: "sha256:review",
        signature: "sigstore-bundle",
        trust: {
          status: "verified",
          approvalState: "approved",
        },
        release: {
          version: "1.0.0",
          channel: "stable",
        },
      },
    });

    expect(review.success).toBe(true);
  });

  test("rejects review manifests with unsupported output formats", () => {
    const review = PluginManifestSchema.safeParse({
      apiVersion: "agentforge.dev/v1",
      kind: "ReviewPlugin",
      metadata: {
        id: "review.comments",
        name: "Review Comments",
        version: "1.0.0",
      },
      spec: {
        runtime: "mcp",
        transport: "stdio",
        command: "node",
        args: ["dist/review.js"],
        review: {
          entrypoint: "review:run",
          triggers: {
            events: ["pull_request.updated"],
          },
          output: {
            format: "github-review-comments",
          },
        },
      },
    });

    expect(review.success).toBe(false);
  });

  test("rejects workflow manifests that still declare the legacy go-plugin runtime", () => {
    const workflow = PluginManifestSchema.safeParse({
      apiVersion: "agentforge.dev/v1",
      kind: "WorkflowPlugin",
      metadata: {
        id: "workflow.legacy",
        name: "Legacy Workflow",
        version: "0.9.0",
      },
      spec: {
        runtime: "go-plugin",
        binary: "./bin/workflow",
        workflow: {
          process: "sequential",
          roles: [{ id: "coder" }],
          steps: [{ id: "implement", role: "coder", action: "agent" }],
        },
      },
    });

    expect(workflow.success).toBe(false);
    expect(JSON.stringify(workflow.error?.issues)).toContain(
      "WorkflowPlugin manifests must use the wasm runtime",
    );
  });
});
