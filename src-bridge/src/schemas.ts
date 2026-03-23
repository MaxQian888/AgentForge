import { z } from "zod";

export const RoleConfigSchema = z.object({
  name: z.string().min(1),
  goal: z.string().min(1),
  backstory: z.string(),
  system_prompt: z.string(),
  allowed_tools: z.array(z.string()),
  max_budget_usd: z.number().positive(),
  permission_mode: z.string(),
});

export const ExecuteRequestSchema = z.object({
  task_id: z.string().min(1),
  session_id: z.string().min(1),
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

export const CancelRequestSchema = z.object({
  task_id: z.string().min(1),
  reason: z.string().optional(),
});
