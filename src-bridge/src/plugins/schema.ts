import { z } from "zod";

export const PluginKindSchema = z.enum([
  "RolePlugin",
  "ToolPlugin",
  "WorkflowPlugin",
  "IntegrationPlugin",
  "ReviewPlugin",
]);

export const PluginRuntimeSchema = z.enum(["declarative", "mcp", "go-plugin", "wasm"]);
export const PluginTrustStateSchema = z.enum(["unknown", "verified", "untrusted"]);
export const PluginApprovalStateSchema = z.enum(["not-required", "pending", "approved", "rejected"]);
export const PluginLifecycleOperationSchema = z.enum([
  "install",
  "enable",
  "activate",
  "deactivate",
  "disable",
  "uninstall",
  "update",
]);
export const PluginLifecycleStateSchema = z.enum([
  "installed",
  "enabled",
  "activating",
  "active",
  "degraded",
  "disabled",
]);

const WorkflowSpecSchema = z.object({
  process: z.enum(["sequential", "hierarchical", "event-driven"]),
  roles: z.array(z.object({ id: z.string().min(1) })).optional(),
  steps: z.array(
    z.object({
      id: z.string().min(1),
      role: z.string().min(1),
      action: z.enum(["agent", "review", "task"]),
      next: z.array(z.string().min(1)).optional(),
    }),
  ).min(1),
  triggers: z.array(
    z.object({
      event: z.string().min(1).optional(),
      profile: z.string().min(1).optional(),
      requiresTask: z.boolean().optional(),
    }).superRefine((trigger, ctx) => {
      if (trigger.event === "task.transition" && !trigger.profile) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: "task.transition workflow triggers must define profile",
          path: ["profile"],
        });
      }
    }),
  ).optional(),
  limits: z.object({ maxRetries: z.number().int().min(0).optional() }).optional(),
});

const ReviewSpecSchema = z.object({
  entrypoint: z.string().min(1).optional(),
  triggers: z.object({
    events: z.array(z.string().min(1)).min(1),
    filePatterns: z.array(z.string().min(1)).optional(),
  }),
  output: z.object({
    format: z.literal("findings/v1"),
  }),
});

const PluginSourceSchema = z.object({
  type: z.enum(["builtin", "local", "git", "npm", "catalog"]),
  path: z.string().optional(),
  repository: z.string().optional(),
  ref: z.string().optional(),
  package: z.string().optional(),
  version: z.string().optional(),
  registry: z.string().optional(),
  catalog: z.string().optional(),
  entry: z.string().optional(),
  digest: z.string().optional(),
  signature: z.string().optional(),
  trust: z.object({
    status: PluginTrustStateSchema,
    approvalState: PluginApprovalStateSchema.optional(),
    source: z.string().optional(),
    verifiedAt: z.string().optional(),
    approvedBy: z.string().optional(),
    approvedAt: z.string().optional(),
    reason: z.string().optional(),
  }).optional(),
  release: z.object({
    version: z.string().optional(),
    channel: z.string().optional(),
    artifact: z.string().optional(),
    notesUrl: z.string().optional(),
    publishedAt: z.string().optional(),
    availableVersion: z.string().optional(),
  }).optional(),
});

export const PluginManifestSchema = z
  .object({
    apiVersion: z.string().min(1),
    kind: PluginKindSchema,
    metadata: z.object({
      id: z.string().min(1),
      name: z.string().min(1),
      version: z.string().min(1),
      description: z.string().optional(),
      tags: z.array(z.string()).optional(),
    }),
    spec: z.object({
      runtime: PluginRuntimeSchema,
      transport: z.string().optional(),
      command: z.string().optional(),
      args: z.array(z.string()).optional(),
      url: z.string().url().optional(),
      binary: z.string().optional(),
      module: z.string().optional(),
      abiVersion: z.string().optional(),
      capabilities: z.array(z.string()).optional(),
      config: z.record(z.string(), z.unknown()).optional(),
      env: z.record(z.string(), z.string()).optional(),
      workflow: WorkflowSpecSchema.optional(),
      review: ReviewSpecSchema.optional(),
    }),
    permissions: z.record(z.string(), z.unknown()).optional().default({}),
    source: PluginSourceSchema.optional(),
  })
  .superRefine((manifest, ctx) => {
    if (manifest.kind === "ToolPlugin" && manifest.spec.runtime !== "mcp") {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "ToolPlugin manifests must use the mcp runtime",
        path: ["spec", "runtime"],
      });
    }
    if (manifest.kind === "IntegrationPlugin" && !["go-plugin", "wasm"].includes(manifest.spec.runtime)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "IntegrationPlugin manifests must use a Go-hosted runtime (go-plugin or wasm)",
        path: ["spec", "runtime"],
      });
    }
    if (manifest.kind === "WorkflowPlugin" && manifest.spec.runtime !== "wasm") {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "WorkflowPlugin manifests must use the wasm runtime",
        path: ["spec", "runtime"],
      });
    }
    if (manifest.kind === "ReviewPlugin" && manifest.spec.runtime !== "mcp") {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "ReviewPlugin manifests must use the mcp runtime",
        path: ["spec", "runtime"],
      });
    }
    if (manifest.kind === "ToolPlugin" && manifest.spec.transport === "stdio" && !manifest.spec.command) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "stdio tool plugins must define spec.command",
        path: ["spec", "command"],
      });
    }
    if (manifest.kind === "WorkflowPlugin") {
      if (!manifest.spec.module) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: "WorkflowPlugin manifests must define spec.module",
          path: ["spec", "module"],
        });
      }
      if (!manifest.spec.abiVersion) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: "WorkflowPlugin manifests must define spec.abiVersion",
          path: ["spec", "abiVersion"],
        });
      }
      if (!manifest.spec.workflow) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: "WorkflowPlugin manifests must define spec.workflow",
          path: ["spec", "workflow"],
        });
      }
    }
    if (manifest.kind === "ReviewPlugin" && !manifest.spec.review) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "ReviewPlugin manifests must define spec.review",
        path: ["spec", "review"],
      });
    }
  });

export const ToolPluginManifestSchema = PluginManifestSchema.superRefine((manifest, ctx) => {
  if (manifest.kind !== "ToolPlugin") {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: "Tool plugin manager only accepts ToolPlugin manifests",
      path: ["kind"],
    });
  }
});

export const PluginRegisterRequestSchema = z.object({
  manifest: PluginManifestSchema,
});
