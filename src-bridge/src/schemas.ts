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
  allowed_tools: z.array(z.string()).default([]),
  permission_mode: z.string().default("default"),
  role_config: RoleConfigSchema.optional(),
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
});

export const DecomposeTaskResponseSchema = z.object({
  summary: z.string().min(1),
  subtasks: z.array(DecomposeSubtaskSchema).min(1),
});

export const CancelRequestSchema = z.object({
  task_id: z.string().min(1),
  reason: z.string().optional(),
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
  dimensions: z.array(ReviewDimensionSchema).optional(),
});
