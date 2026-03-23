import { z } from "zod";

export const PluginKindSchema = z.enum([
  "RolePlugin",
  "ToolPlugin",
  "WorkflowPlugin",
  "IntegrationPlugin",
  "ReviewPlugin",
]);

export const PluginRuntimeSchema = z.enum(["declarative", "mcp", "go-plugin"]);
export const PluginLifecycleStateSchema = z.enum([
  "installed",
  "enabled",
  "activating",
  "active",
  "degraded",
  "disabled",
]);

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
      config: z.record(z.string(), z.unknown()).optional(),
    }),
    permissions: z.record(z.string(), z.unknown()).optional().default({}),
    source: z
      .object({
        type: z.enum(["builtin", "local"]),
        path: z.string().optional(),
      })
      .optional(),
  })
  .superRefine((manifest, ctx) => {
    if (manifest.kind === "ToolPlugin" && manifest.spec.runtime !== "mcp") {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "ToolPlugin manifests must use the mcp runtime",
        path: ["spec", "runtime"],
      });
    }
    if (manifest.kind === "IntegrationPlugin" && manifest.spec.runtime !== "go-plugin") {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "IntegrationPlugin manifests must use the go-plugin runtime",
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
