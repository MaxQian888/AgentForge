import { z } from "zod";

export const RoleConfigSchema = z.object({
  role_id: z.string().min(1),
  name: z.string().min(1),
  role: z.string().min(1),
  goal: z.string().min(1),
  backstory: z.string(),
  system_prompt: z.string(),
  allowed_tools: z.array(z.string()),
  max_budget_usd: z.number().positive(),
  max_turns: z.number().int().positive(),
  permission_mode: z.string(),
  tools: z.array(z.string()).optional(),
  knowledge_context: z.string().optional(),
  output_filters: z.array(z.string()).optional(),
}).strict();

export const ExecuteRequestSchema = z.object({
  task_id: z.string().min(1),
  session_id: z.string().min(1),
  runtime: z.enum(["claude_code", "codex", "opencode"]).optional(),
  provider: z.string().min(1).optional(),
  model: z.string().min(1).optional(),
  prompt: z.string().min(1),
  worktree_path: z.string(),
  branch_name: z.string(),
  system_prompt: z.string().default(""),
  max_turns: z.number().int().positive().default(50),
  budget_usd: z.number().positive().default(1.0),
  warn_threshold: z.number().min(0).max(1).optional(),
  allowed_tools: z.array(z.string()).default([]),
  permission_mode: z.string().default("default"),
  role_config: RoleConfigSchema.optional(),
  team_id: z.string().optional(),
  team_role: z.string().optional(),
});

export const DecomposeTaskRequestSchema = z.object({
  task_id: z.string().min(1),
  title: z.string().min(1),
  description: z.string().min(1),
  priority: z.enum(["critical", "high", "medium", "low"]),
  provider: z.string().min(1).optional(),
  model: z.string().min(1).optional(),
});

export const DecomposeSubtaskSchema = z.object({
  title: z.string().min(1),
  description: z.string().min(1),
  priority: z.enum(["critical", "high", "medium", "low"]),
  executionMode: z.enum(["human", "agent"]),
});

export const DecomposeTaskResponseSchema = z.object({
  summary: z.string().min(1),
  subtasks: z.array(DecomposeSubtaskSchema).min(1),
});

export const CancelRequestSchema = z.object({
  task_id: z.string().min(1),
  reason: z.string().optional(),
});

export const ResumeRequestSchema = z.object({
  task_id: z.string().min(1),
});

export const ReviewDimensionSchema = z.enum([
  "logic",
  "security",
  "performance",
  "compliance",
]);

export const DeepReviewRequestSchema = z.object({
  review_id: z.string().min(1),
  task_id: z.string().min(1),
  pr_url: z.string().min(1),
  pr_number: z.number().int().nonnegative().optional(),
  title: z.string().optional(),
  description: z.string().optional(),
  diff: z.string().optional(),
  trigger_event: z.string().min(1).optional(),
  changed_files: z.array(z.string().min(1)).optional(),
  dimensions: z.array(ReviewDimensionSchema).optional(),
  review_plugins: z.array(
    z.object({
      plugin_id: z.string().min(1),
      name: z.string().min(1),
      entrypoint: z.string().min(1).optional(),
      source_type: z.string().min(1).optional(),
      transport: z.enum(["stdio", "http"]).optional(),
      command: z.string().min(1).optional(),
      args: z.array(z.string()).optional(),
      url: z.string().url().optional(),
      events: z.array(z.string().min(1)).optional(),
      file_patterns: z.array(z.string().min(1)).optional(),
      output_format: z.string().min(1).optional(),
    }),
  ).optional(),
});
